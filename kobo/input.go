package kobo

// evdev plumbing: device discovery, touch contacts, keyboards.

import (
	"bufio"
	"encoding/binary"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// linux/input-event-codes.h
const (
	evSyn = 0x00
	evKey = 0x01
	evAbs = 0x03

	synReport  = 0x00
	synDropped = 0x03

	btnTouch = 0x14a
	keyPower = 116

	absX            = 0x00
	absY            = 0x01
	absMTSlot       = 0x2f
	absMTPositionX  = 0x35
	absMTPositionY  = 0x36
	absMTToolType   = 0x37
	absMTTrackingID = 0x39
	absMTPressure   = 0x3a

	mtToolPen = 1 // on the Elipsa's Elan panel: 0 finger, 1 pen

	eviocgrab = 0x40044590 // _IOW('E', 0x90, int)
)

var inputDir = EnvOr("HELLOBRO_INPUT_DIR", "/dev/input") // overridable for testing

type inputDev struct {
	name, handler    string
	keyBits, absBits []string
}

func (d inputDev) path() string { return filepath.Join(inputDir, d.handler) }

func (d inputDev) isKeyboard() bool {
	return hasBit(d.keyBits, KeyA) && hasBit(d.keyBits, KeyZ) && hasBit(d.keyBits, KeySpace)
}

func (d inputDev) isTouch() bool { return hasBit(d.absBits, absMTPositionX) }

// hasMediaKeys: e.g. Bluetooth headphones' play/pause buttons (AVRCP).
func (d inputDev) hasMediaKeys() bool {
	return hasBit(d.keyBits, KeyPlayPause) || hasBit(d.keyBits, KeyPlayCD)
}

// role is a short label for the device list.
func (d inputDev) role() string {
	switch {
	case d.isTouch():
		return "touch + pen"
	case d.isKeyboard():
		return "keyboard"
	case d.hasMediaKeys():
		return "media buttons"
	case strings.Contains(strings.ToLower(d.name), "accel"):
		return "tilt sensor"
	case hasBit(d.keyBits, keyPower):
		return "buttons"
	}
	return ""
}

// inputDevices parses /proc/bus/input/devices, keeping devices with an event handler.
func inputDevices() []inputDev {
	f, err := os.Open(EnvOr("HELLOBRO_DEVICES", "/proc/bus/input/devices"))
	if err != nil {
		log.Printf("input devices: %v", err)
		return nil
	}
	defer f.Close()

	var devs []inputDev
	var d inputDev
	flush := func() {
		if d.handler != "" {
			devs = append(devs, d)
		}
		d = inputDev{}
	}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "":
			flush()
		case strings.HasPrefix(line, "N: Name="):
			d.name = strings.Trim(strings.TrimPrefix(line, "N: Name="), `"`)
		case strings.HasPrefix(line, "H: Handlers="):
			for _, h := range strings.Fields(strings.TrimPrefix(line, "H: Handlers=")) {
				if strings.HasPrefix(h, "event") {
					d.handler = h
				}
			}
		case strings.HasPrefix(line, "B: KEY="):
			d.keyBits = strings.Fields(strings.TrimPrefix(line, "B: KEY="))
		case strings.HasPrefix(line, "B: ABS="):
			d.absBits = strings.Fields(strings.TrimPrefix(line, "B: ABS="))
		}
	}
	flush()
	return devs
}

// hasBit checks a /proc/bus/input/devices bitmask: hex words of
// unsigned-long size, most significant first.
func hasBit(words []string, bit int) bool {
	const wordBits = strconv.IntSize
	idx := len(words) - 1 - bit/wordBits
	if idx < 0 {
		return false
	}
	w, err := strconv.ParseUint(words[idx], 16, 64)
	return err == nil && w&(1<<(bit%wordBits)) != 0
}

// --- touch ---

const (
	TouchDown = iota
	TouchMove
	TouchUp
	TouchHover // pen in range but not touching
)

// TouchEv is one change of one contact (finger or pen).
type TouchEv struct {
	Kind     int
	slot     int32
	x, y     int32 // raw panel units
	Pen      bool
	Pressure int32
	At       time.Time
}

type absRange struct{ min, max int32 }

type touchDevice struct {
	f              *os.File
	name           string
	rx, ry, rpress absRange
}

// openTouch grabs the touch panel and asks it for its coordinate ranges.
func openTouch(d inputDev) (*touchDevice, error) {
	f, err := openGrabbed(d.path())
	if err != nil {
		return nil, err
	}
	t := &touchDevice{f: f, name: d.name}
	t.rx = absInfo(f, absMTPositionX)
	t.ry = absInfo(f, absMTPositionY)
	t.rpress = absInfo(f, absMTPressure)
	log.Printf("touch %q: x %d..%d, y %d..%d, pressure %d..%d",
		d.name, t.rx.min, t.rx.max, t.ry.min, t.ry.max, t.rpress.min, t.rpress.max)
	return t, nil
}

// absInfo wraps EVIOCGABS. A zero range means the query failed.
func absInfo(f *os.File, code int) absRange {
	var info [6]int32                                                           // value, minimum, maximum, fuzz, flat, resolution
	req := uintptr(2<<30|'E'<<8) | unsafe.Sizeof(info)<<16 | uintptr(0x40+code) // _IOR('E', 0x40+abs, struct input_absinfo)
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), req, uintptr(unsafe.Pointer(&info))); errno != 0 {
		log.Printf("EVIOCGABS %#x: %v", code, errno)
		return absRange{}
	}
	return absRange{info[1], info[2]}
}

type contact struct {
	id       int32 // tracking ID, -1 = no contact
	x, y     int32
	tool     int32
	pressure int32
}

// down reports whether the contact is really touching: a pen with zero
// pressure is only hovering (KOReader drops those too).
func (c *contact) down() bool { return c.id >= 0 && !(c.tool == mtToolPen && c.pressure == 0) }

// run turns the multitouch slot protocol into per-contact down/move/up
// (plus hover for the pen).
func (t *touchDevice) run(out chan<- TouchEv) {
	defer t.f.Close()
	slots := map[int32]*contact{}
	slot := func(n int32) *contact {
		c, ok := slots[n]
		if !ok {
			c = &contact{id: -1}
			slots[n] = c
		}
		return c
	}
	prev := map[int32]contact{} // slot states as of the previous frame
	cur := int32(0)
	dropping, drops := false, 0

	err := readEvents(t.f, func(typ, code uint16, val int32) {
		if dropping { // the kernel's buffer overflowed: skip to the next complete frame
			dropping = !(typ == evSyn && code == synReport)
			return
		}
		switch {
		case typ == evAbs && code == absMTSlot:
			cur = val
		case typ == evAbs && code == absMTTrackingID:
			slot(cur).id = val
		case typ == evAbs && code == absMTToolType:
			slot(cur).tool = val
		case typ == evAbs && code == absMTPressure:
			slot(cur).pressure = val
		case typ == evAbs && (code == absMTPositionX || code == absX):
			slot(cur).x = val
		case typ == evAbs && (code == absMTPositionY || code == absY):
			slot(cur).y = val
		case typ == evSyn && code == synDropped:
			dropping = true
			if drops++; drops <= 10 || drops%100 == 0 {
				log.Printf("touch: SYN_DROPPED #%d, events were lost", drops)
			}
		case typ == evSyn && code == synReport:
			now := time.Now()
			for n, c := range slots {
				p, seen := prev[n]
				if !seen {
					p = contact{id: -1}
				}
				moved := c.x != p.x || c.y != p.y
				kind := -1
				switch {
				case c.down() && !p.down():
					kind = TouchDown
				case c.down() && (moved || c.pressure != p.pressure):
					kind = TouchMove
				case !c.down() && p.down():
					kind = TouchUp
				case c.id >= 0 && c.tool == mtToolPen && moved:
					kind = TouchHover
				}
				if kind >= 0 {
					out <- TouchEv{kind, n, c.x, c.y, c.tool == mtToolPen || (kind == TouchUp && p.tool == mtToolPen), c.pressure, now}
				}
				prev[n] = *c
			}
		}
	})
	log.Printf("touch closed: %v", err)
}

// --- keyboards ---

// KeyMsg is what keyboard goroutines send to the UI loop: a key event,
// or a keyboard connecting or going away.
type KeyMsg struct {
	Code      uint16
	Value     int32 // 1 press, 0 release, 2 autorepeat
	Connected string
	Gone      string
	button    bool // from the device's own buttons (power, sleep cover), not a keyboard
	ungrabbed bool // the buttons are someone else's: we won't see the power button
	at        time.Time
}

// watchKeyboards polls for keyboards and starts a reader for each new one.
// Bluetooth keyboards appear as ordinary input devices once paired.
func watchKeyboards(out chan<- KeyMsg) {
	var mu sync.Mutex
	open := map[string]bool{}
	for {
		for _, d := range inputDevices() {
			media := !d.isKeyboard()
			if media && !d.hasMediaKeys() {
				continue
			}
			path := d.path()
			mu.Lock()
			isOpen := open[path]
			open[path] = true
			mu.Unlock()
			if isOpen {
				continue
			}
			go func(name string) {
				defer func() {
					mu.Lock()
					delete(open, path)
					mu.Unlock()
					if !media {
						out <- KeyMsg{Gone: name}
					}
				}()
				f, err := openGrabbed(path)
				if err != nil {
					log.Printf("open keyboard %s: %v", path, err)
					return
				}
				defer f.Close()
				if media {
					log.Printf("media buttons: %s", name)
				} else {
					out <- KeyMsg{Connected: name}
				}
				err = readEvents(f, func(typ, code uint16, val int32) {
					if typ == evKey {
						out <- KeyMsg{Code: code, Value: val, at: time.Now()}
					}
				})
				log.Printf("keyboard %s: %v", path, err)
			}(d.name)
		}
		time.Sleep(2 * time.Second)
	}
}

// watchButtons forwards the power button and sleep cover. Grabbing them
// also keeps Nickel from finding a queued power press when it wakes up.
func watchButtons(d inputDev, out chan<- KeyMsg) {
	f, err := os.Open(d.path())
	if err != nil {
		log.Printf("open buttons %s: %v", d.path(), err)
		return
	}
	defer f.Close()
	if err := grab(f); err != nil {
		// Someone (Nickel?) holds them, so their events never reach us.
		log.Printf("EVIOCGRAB %s: %v: no power button for us", d.path(), err)
		out <- KeyMsg{button: true, ungrabbed: true}
		return
	}
	err = readEvents(f, func(typ, code uint16, val int32) {
		if typ == evKey {
			out <- KeyMsg{Code: code, Value: val, button: true}
		}
	})
	log.Printf("buttons %s: %v", d.path(), err)
}

// --- evdev helpers ---

// openGrabbed opens an evdev device and grabs it, so Nickel doesn't react
// to our input. The grab is released when the fd closes, even if we crash.
func openGrabbed(path string) (*os.File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	if err := grab(f); err != nil {
		log.Printf("EVIOCGRAB %s: %v (Nickel will see this input too)", path, err)
	}
	return f, nil
}

func grab(f *os.File) error {
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), eviocgrab, 1); errno != 0 {
		return errno
	}
	return nil
}

// readEvents calls fn for every event until the device closes.
// It returns nil on a clean EOF.
func readEvents(f *os.File, fn func(typ, code uint16, val int32)) error {
	// struct input_event on 32-bit ARM: timeval (8) + type (2) + code (2) + value (4).
	var buf [16]byte
	for {
		if _, err := io.ReadFull(f, buf[:]); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		fn(binary.LittleEndian.Uint16(buf[8:]),
			binary.LittleEndian.Uint16(buf[10:]),
			int32(binary.LittleEndian.Uint32(buf[12:])))
	}
}

// EnvOr is the environment variable key, or def if it's unset.
func EnvOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
