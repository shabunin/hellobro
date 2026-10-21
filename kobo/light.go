package kobo

// The frontlight, the way KOReader drives the Elipsa's: an ioctl on
// /dev/ntx_io taking 0-100. It can't be read back, so we start from the
// level Nickel saved, and put that back when we leave, since Nickel would
// otherwise believe its own level is still on.

import (
	"bufio"
	"fmt"
	"image"
	"image/draw"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const ntxSetLight = 241 // ioctl on /dev/ntx_io: set the frontlight, 0-100

var nickelConf = EnvOr("HELLOBRO_NICKEL_CONF", "/mnt/onboard/.kobo/Kobo/Kobo eReader.conf")

type frontlight struct {
	f       *os.File
	level   int // what we set last
	initial int // Nickel's, restored on close
	from    string
}

func openFrontlight() *frontlight {
	f, err := os.OpenFile(EnvOr("HELLOBRO_NTX_IO", "/dev/ntx_io"), os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		log.Printf("frontlight: %v", err)
		return nil
	}
	l := &frontlight{f: f, level: 50, from: "a guess"}
	if lvl, ok := nickelLightLevel(); ok {
		l.level, l.from = lvl, "Nickel's settings"
	} else if lvl, ok := sysfsBrightness(); ok {
		l.level, l.from = lvl, "sysfs"
	}
	l.initial = l.level
	log.Printf("frontlight: %d%% (from %s)", l.level, l.from)
	return l
}

// set changes the light, 0 (off) to 100.
func (l *frontlight) set(level int) error {
	level = max(0, min(100, level))
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, l.f.Fd(), ntxSetLight, uintptr(level))
	if errno == syscall.ENOTTY && os.Getenv("HELLOBRO_NTX_IO") != "" {
		errno = 0 // a stand-in file, for testing
	}
	if errno != 0 {
		return fmt.Errorf("frontlight ioctl: %w", errno)
	}
	l.level = level
	return nil
}

func (l *frontlight) close() {
	if l.level != l.initial {
		log.Printf("frontlight: back to Nickel's %d%%", l.initial)
		l.set(l.initial)
	}
	l.f.Close()
}

// nickelLightLevel reads [PowerOptions] FrontLightLevel (and FrontLightState).
func nickelLightLevel() (int, bool) {
	f, err := os.Open(nickelConf)
	if err != nil {
		return 0, false
	}
	defer f.Close()
	section, level, on, found := "", 0, true, false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "[") {
			section = line
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok || section != "[PowerOptions]" {
			continue
		}
		switch k {
		case "FrontLightLevel":
			if n, err := strconv.Atoi(v); err == nil {
				level, found = n, true
			}
		case "FrontLightState":
			on = v != "false"
		}
	}
	if !on {
		level = 0
	}
	return level, found
}

// sysfsBrightness reads a backlight's level as a percentage, if the kernel offers one.
func sysfsBrightness() (int, bool) {
	dirs, _ := filepath.Glob("/sys/class/backlight/*")
	for _, d := range dirs {
		read := func(name string) int {
			b, _ := os.ReadFile(filepath.Join(d, name))
			n, _ := strconv.Atoi(strings.TrimSpace(string(b)))
			return n
		}
		cur, maxB := read("actual_brightness"), read("max_brightness")
		if cur == 0 {
			cur = read("brightness")
		}
		log.Printf("frontlight: %s: %d of %d", d, cur, maxB)
		if maxB > 0 {
			return cur * 100 / maxB, true
		}
	}
	return 0, false
}

// --- the swipe's label ---

// lightLabelRect sits right of the finger, centered vertically: the
// percentage, no bar.
func (a *App) lightLabelRect() image.Rectangle {
	cy := a.C.Bounds().Dy() / 2
	return image.Rect(EdgeWidth+20, cy-40, EdgeWidth+20+250, cy+40)
}

func (a *App) setLight(level int) {
	level = max(0, min(100, level))
	if level == a.light.level && a.lightOverlay != nil {
		return
	}
	if err := a.light.set(level); err != nil {
		log.Print(err)
	}
	r := a.lightLabelRect()
	if a.lightOverlay == nil {
		a.lightOverlay = image.NewGray(r)
		draw.Draw(a.lightOverlay, r, a.C.Gray, r.Min, draw.Src)
	}
	a.C.Fill(r, White)
	a.C.Frame(r, 3, Black)
	a.C.Text(a.F.Panel, r, fmt.Sprintf("light %d%%", a.light.level), Black, AlignCenter)
	a.Mark(r, false) // black & white only, so the fast waveform is fine
}

// DrawGauge shows a gauge like the light's: a bar in r, filled n/of,
// labeled; HideGauge takes it away again.
func (a *App) DrawGauge(r image.Rectangle, n, of int, label string) {
	a.drawGauge(&a.gaugeOverlay, r, n, of, label)
}

func (a *App) HideGauge() { a.hideGauge(&a.gaugeOverlay) }

// GaugeUp says whether a gauge is on screen: drawing under it would be
// undone when it goes.
func (a *App) GaugeUp() bool { return a.lightOverlay != nil || a.gaugeOverlay != nil }

// drawGauge draws the gauge, saving what it covers in *under the first time.
func (a *App) drawGauge(under **image.Gray, r image.Rectangle, n, of int, label string) {
	if *under == nil {
		*under = image.NewGray(r)
		draw.Draw(*under, r, a.C.Gray, r.Min, draw.Src)
	}
	c := a.C
	c.Fill(r, White)
	c.Frame(r, 3, Black)
	bar := image.Rect(r.Min.X+20, r.Min.Y+20, r.Max.X-20, r.Max.Y-70)
	c.Frame(bar, 2, Black)
	in := bar.Inset(6)
	c.Fill(image.Rect(in.Min.X, in.Max.Y-in.Dy()*n/of, in.Max.X, in.Max.Y), Black)
	c.Text(a.F.Panel, image.Rect(r.Min.X, r.Max.Y-64, r.Max.X, r.Max.Y-10), label, Black, AlignCenter)
	a.Mark(r, false) // black & white only, so the fast waveform is fine
}

// hideGauge puts back what the gauge covered.
func (a *App) hideGauge(under **image.Gray) {
	if *under == nil {
		return
	}
	r := (*under).Bounds()
	draw.Draw(a.C.Gray, r, *under, r.Min, draw.Src)
	*under = nil
	a.Mark(r, true)
}

func (a *App) lightInfo() string {
	if a.light == nil {
		return "frontlight: not available"
	}
	return fmt.Sprintf("frontlight: %d%% (started at %d%%, from %s)", a.light.level, a.light.initial, a.light.from)
}
