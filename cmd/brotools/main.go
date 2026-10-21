// brotools is a set of diagnostics for the Kobo Elipsa: input, network,
// sound and what the device is.
//
// Tabs:
//   - Keys: type with a USB or Bluetooth keyboard;
//   - Finger: draw with a finger (the pen is ignored there);
//   - Stylus: draw with the pen, pressure sets the width (fingers are
//     ignored on the canvas, so a resting palm doesn't draw);
//   - Net: Wi-Fi status, a scan (access points, hosts on the local
//     network), an mDNS browser and an HTTPS request;
//   - Sound: Bluetooth audio devices, and a player (test tone, MP3, WAV);
//   - System: the device, kernel, memory, storage, battery, tools;
//   - Info: input devices, screen and touch mapping details.
//
// The rest (the screen, touch, the frontlight on the left edge, quitting)
// is the kobo package's.
//
// Launch it through launch.sh, which keeps Nickel from drawing over us.
package main

import (
	"fmt"
	"image"
	"image/draw"
	"log"
	"math"
	"strings"
	"time"

	"hellobro/kobo"
)

const (
	tabKeys = iota
	tabFinger
	tabStylus
	tabNet
	tabSound
	tabSystem
	tabInfo
)

var tabNames = []string{"Keys", "Finger", "Stylus", "Net", "Sound", "System", "Info"}

const maxLog = 200

// Gesture owners: the canvas a contact draws on.
const (
	ownFinger = 1 + iota
	ownStylus
)

type netPanelState struct {
	status string
	lines  []string
	busy   bool
}

// tools is the app: the kobo.App, and what its pages show.
type tools struct {
	*kobo.App
	L layout

	ed      editor
	shown   []string // typing rows currently on screen
	lastKey string

	fingerImg, stylusImg *image.Gray
	fingerLog, stylusLog []string
	penStatus, penShown  string
	penAt                time.Time

	netState [numNetPanels]netPanelState
	netc     chan netMsg
	snd      soundState
	sys      sysState
}

// layout is where the pages' panels go.
type layout struct {
	typing, lastKey          kobo.Panel
	fingerCanvas, fingerLog  kobo.Panel
	stylusCanvas, stylusLog  kobo.Panel
	devices, screenP, touchP kobo.Panel
	net                      [numNetPanels]kobo.Panel
	btP, playerP             kobo.Panel
	sys                      [numSysPanels]kobo.Panel
}

func main() {
	a := &tools{penStatus: "pen away", netc: make(chan netMsg, 16)}
	a.snd.gain = 40
	a.App = kobo.New(kobo.Config{Name: "brotools", Title: "bro tools", Tabs: tabNames}, a)
	go func() {
		for m := range a.netc {
			a.Do(func() { a.onNet(m) })
		}
	}()
	a.Run()
}

func (a *tools) Layout(K kobo.Layout) {
	f, body, L := a.F, K.Body, &a.L

	// Keyboard: a big text box and a "last key" strip.
	L.typing = kobo.NewPanel(f, "Typing", image.Rect(body.Min.X, body.Min.Y, body.Max.X, body.Max.Y-140-kobo.Pad), "")
	L.lastKey = kobo.NewPanel(f, "Last key", image.Rect(body.Min.X, body.Max.Y-140, body.Max.X, body.Max.Y), "")

	cr, lr := K.Split(body, 0.66)
	L.fingerCanvas = kobo.NewPanel(f, "Finger canvas", cr, "clear")
	L.fingerLog = kobo.NewPanel(f, "Finger log", lr, "")
	L.stylusCanvas = kobo.NewPanel(f, "Stylus canvas", cr, "clear")
	L.stylusLog = kobo.NewPanel(f, "Stylus log", lr, "")
	// New canvases when their size changes.
	if sz := L.fingerCanvas.Body.Size(); a.fingerImg == nil || a.fingerImg.Bounds().Size() != sz {
		a.fingerImg = kobo.Blank(sz)
	}
	if sz := L.stylusCanvas.Body.Size(); a.stylusImg == nil || a.stylusImg.Bounds().Size() != sz {
		a.stylusImg = kobo.Blank(sz)
	}
	a.shown = nil

	// Info: devices on the left, screen above touch on the right (in portrait: devices on top).
	dr, rest := K.Split(body, 0.45)
	at := rest.Min.Y + rest.Dy()*40/100
	if K.Landscape {
		at = rest.Min.Y + rest.Dy()*45/100
	}
	L.screenP = kobo.NewPanel(f, "Screen", image.Rect(rest.Min.X, rest.Min.Y, rest.Max.X, at-kobo.Pad/2), "")
	L.touchP = kobo.NewPanel(f, "Touch", image.Rect(rest.Min.X, at+kobo.Pad/2, rest.Max.X, rest.Max.Y), "")
	L.devices = kobo.NewPanel(f, "Input devices", dr, "")

	// Network: Wi-Fi status, scan, mDNS, HTTP test.
	var nr []image.Rectangle // wifi, scan, mdns, http
	if K.Landscape {         // Wi-Fi above HTTPS on the left, scan above mDNS on the right
		left, right := K.Split(body, 0.5)
		l, r := K.Stack(left, 45), K.Stack(right, 50)
		nr = []image.Rectangle{l[0], r[0], r[1], l[1]}
	} else {
		nr = K.Stack(body, 24, 50, 76)
	}
	L.net[netWifi] = kobo.NewPanel(f, "Wi-Fi", nr[0], "refresh")
	L.net[netScan] = kobo.NewPanel(f, "Scan", nr[1], "scan")
	L.net[netMDNS] = kobo.NewPanel(f, "mDNS", nr[2], "browse")
	L.net[netHTTP] = kobo.NewPanel(f, "HTTPS", nr[3], "GET")

	// Sound: Bluetooth devices & PCMs, and the player.
	br, pr := K.Split(body, 0.4)
	if K.Landscape {
		br, pr = K.Split(body, 0.5)
	}
	L.btP = kobo.NewPanel(f, "Bluetooth", br, "refresh")
	L.playerP = kobo.NewPanel(f, "Player", pr, "stop")

	// System: device, system, storage & power, software & tools.
	var sr []image.Rectangle
	if K.Landscape {
		left, right := K.Split(body, 0.5)
		l, r := K.Stack(left, 30), K.Stack(right, 30)
		sr = []image.Rectangle{l[0], l[1], r[0], r[1]}
	} else {
		sr = K.Stack(body, 15, 39, 56)
	}
	L.sys[sysDevice] = kobo.NewPanel(f, "Device", sr[0], "refresh")
	L.sys[sysSystem] = kobo.NewPanel(f, "System", sr[1], "")
	L.sys[sysStorage] = kobo.NewPanel(f, "Storage & power", sr[2], "")
	L.sys[sysSoftware] = kobo.NewPanel(f, "Software & tools", sr[3], "")
}

func (a *tools) Enter(tab int) {
	switch {
	case tab == tabNet && a.netState[netWifi].lines == nil:
		a.startNet(netWifi)
	case tab == tabSound:
		a.enterSound()
	case tab == tabSystem && a.sys.at.IsZero():
		a.startSys()
	}
}

func (a *tools) Draw() {
	c, L := a.C, a.L
	switch a.Tab {
	case tabKeys:
		a.DrawPanel(L.typing, a.keyboardStatus())
		a.shown = nil
		a.drawTyping()
		a.DrawPanel(L.lastKey, "")
		a.drawLastKey()
	case tabFinger:
		a.DrawPanel(L.fingerCanvas, "pen is ignored here")
		draw.Draw(c.Gray, L.fingerCanvas.Body, a.fingerImg, image.Point{}, draw.Src)
		a.DrawPanel(L.fingerLog, fmt.Sprintf("%d", len(a.fingerLog)))
		a.DrawLines(L.fingerLog, a.fingerLog, "Draw or tap with a finger on the canvas.")
	case tabStylus:
		a.DrawPanel(L.stylusCanvas, a.penStatus)
		a.penShown = a.penStatus
		draw.Draw(c.Gray, L.stylusCanvas.Body, a.stylusImg, image.Point{}, draw.Src)
		a.DrawPanel(L.stylusLog, "fingers are ignored on the canvas")
		a.DrawLines(L.stylusLog, a.stylusLog, "Draw with the stylus. Pressure sets the line width.")
	case tabNet:
		for i := range a.L.net {
			a.drawNetPanel(i)
		}
	case tabSound:
		a.drawBluetooth()
		a.drawPlayer()
	case tabSystem:
		a.drawSystem()
	case tabInfo:
		a.drawInfo()
	}
}

func (a *tools) Tap(p image.Point) {
	switch {
	case a.Tab == tabFinger && kobo.Hit(p, a.L.fingerCanvas.Button):
		a.fingerImg = kobo.Blank(a.fingerImg.Bounds().Size())
		a.DrawPage()
	case a.Tab == tabStylus && kobo.Hit(p, a.L.stylusCanvas.Button):
		a.stylusImg = kobo.Blank(a.stylusImg.Bounds().Size())
		a.DrawPage()
	case a.Tab == tabNet:
		for i, np := range a.L.net {
			if kobo.Hit(p, np.Button) {
				a.startNet(i)
			}
		}
	case a.Tab == tabSound:
		a.tapSound(p)
	case a.Tab == tabSystem && kobo.Hit(p, a.L.sys[sysDevice].Button):
		a.startSys()
	}
}

func (a *tools) Tick() { a.updatePenStatus() }

func (a *tools) IdleTimeout() time.Duration { return kobo.IdleTimeout }

func (a *tools) Exit() { a.stopPlayback("") }

// --- touch ---

func (a *tools) Touch(ev kobo.TouchEv, p image.Point, g *kobo.Gesture) {
	if ev.Pen {
		a.penAt = ev.At
	}
	switch {
	case g == nil:
		a.penStatus = fmt.Sprintf("pen hovering at %d, %d", p.X, p.Y)
		return
	case ev.Kind == kobo.TouchDown:
		switch {
		case a.Tab == tabFinger && p.In(a.L.fingerCanvas.Body):
			if ev.Pen {
				g.Ignored = "pen ignored on the Finger tab"
			} else {
				g.Owner = ownFinger
			}
		case a.Tab == tabStylus && p.In(a.L.stylusCanvas.Body):
			if ev.Pen {
				g.Owner = ownStylus
			} else {
				g.Ignored = "finger ignored on the Stylus tab"
			}
		}
		if g.Owner != 0 {
			a.ink(g.Owner, p, p, ev.Pressure)
		}
	case g.Owner != 0:
		a.ink(g.Owner, g.Last, p, ev.Pressure)
	}
	if ev.Pen {
		a.penStatus = fmt.Sprintf("pen down, pressure %d", ev.Pressure)
	}
}

func (a *tools) Lift(g *kobo.Gesture) {
	what := g.String()
	page, lines := tabFinger, &a.fingerLog
	if g.Pen {
		page, lines = tabStylus, &a.stylusLog
		a.penStatus = "pen lifted"
	}
	*lines = append(*lines, what)
	if len(*lines) > maxLog {
		*lines = (*lines)[len(*lines)-maxLog:]
	}
	if a.Tab == page && !a.Calibrating() {
		if page == tabFinger {
			a.DrawPanelTitle(a.L.fingerLog, fmt.Sprintf("%d", len(*lines)))
			a.DrawLines(a.L.fingerLog, *lines, "")
		} else {
			a.DrawLines(a.L.stylusLog, *lines, "")
		}
	}
}

// ink draws a stroke segment on a canvas page (offscreen and on screen).
func (a *tools) ink(owner int, from, to image.Point, pressure int32) {
	p, img, radius := a.L.fingerCanvas, a.fingerImg, 4
	if owner == ownStylus {
		p, img = a.L.stylusCanvas, a.stylusImg
		radius = 1 + int(7*math.Min(1, float64(pressure)/float64(a.PressureMax())))
	}
	o := p.Body.Min
	kobo.Canvas{Gray: img}.Line(from.Sub(o), to.Sub(o), radius, kobo.Black, img.Bounds())
	r := image.Rectangle{from, to}.Canon().Inset(-radius - 1).Intersect(p.Body)
	draw.Draw(a.C.Gray, r, img, r.Min.Sub(o), draw.Src)
	a.Mark(r, false)
}

// updatePenStatus refreshes the stylus canvas title when the pen state
// changed, without holding up ink that is still being drawn.
func (a *tools) updatePenStatus() {
	if a.penStatus != "pen away" && time.Since(a.penAt) > 1500*time.Millisecond {
		a.penStatus = "pen away"
	}
	if a.Tab != tabStylus || a.Calibrating() || a.penStatus == a.penShown || a.Blitting() {
		return
	}
	a.penShown = a.penStatus
	a.DrawPanelTitle(a.L.stylusCanvas, a.penStatus)
}

// --- keyboard ---

func (a *tools) Key(m kobo.KeyMsg) bool {
	switch {
	case m.Connected != "" || m.Gone != "":
		if !a.Calibrating() && a.Tab == tabKeys {
			a.DrawPanelTitle(a.L.typing, a.keyboardStatus())
		} else if !a.Calibrating() && a.Tab == tabInfo {
			a.DrawPage()
		}
		return true
	case kobo.IsMediaKey(m.Code):
		if m.Value == 1 {
			a.mediaKey(m.Code)
		}
		return true
	case m.Value == 1 && m.Code == kobo.KeyEsc:
		return false // quitting
	}
	desc, changed := a.ed.key(m.Code, m.Value)
	if desc != "" {
		log.Printf("key %s", desc)
		a.lastKey = desc
	}
	if a.Calibrating() || a.Tab != tabKeys {
		return true
	}
	if desc != "" {
		a.drawLastKey()
	}
	if changed {
		a.drawTyping()
	}
	return true
}

// drawTyping repaints only the text rows that changed.
func (a *tools) drawTyping() {
	b := a.L.typing.Body.Inset(kobo.Pad)
	lineH := a.F.Mono.Metrics().Height.Ceil() + 6
	adv := kobo.TextWidth(a.F.Mono, "M")
	cols, rows := max(1, b.Dx()/adv-1), max(1, b.Dy()/lineH)
	lines := a.ed.wrapped(cols)
	if len(lines) > rows {
		lines = lines[len(lines)-rows:]
	}
	if len(a.shown) != rows {
		a.shown = make([]string, rows)
		for i := range a.shown {
			a.shown[i] = "\x00" // never matches, so everything is drawn once
		}
	}
	for i := 0; i < rows; i++ {
		s, cursor := "", i == len(lines)-1
		if i < len(lines) {
			s = lines[i]
		}
		key := s
		if cursor {
			key += "\x01"
		}
		if key == a.shown[i] {
			continue
		}
		a.shown[i] = key
		lr := image.Rect(b.Min.X, b.Min.Y+i*lineH, b.Max.X, b.Min.Y+(i+1)*lineH)
		a.C.Fill(lr, kobo.White)
		if i == 0 && len(a.ed.lines) == 0 {
			a.C.Text(a.F.Body, image.Rect(lr.Min.X+adv+12, lr.Min.Y, lr.Max.X, lr.Max.Y), "type something, bro", kobo.Mid, kobo.AlignLeft)
		} else {
			a.C.Text(a.F.Mono, lr, s, kobo.Black, kobo.AlignLeft)
		}
		if cursor {
			x := lr.Min.X + kobo.TextWidth(a.F.Mono, s)
			a.C.Fill(image.Rect(x+2, lr.Min.Y+4, x+2+adv, lr.Max.Y-4), kobo.Black)
		}
		a.Mark(lr, true)
	}
}

func (a *tools) drawLastKey() {
	b := a.L.lastKey.Body
	a.C.Fill(b, kobo.White)
	s := a.lastKey
	if s == "" {
		s = "-"
	}
	a.C.Text(a.F.Mono, b.Inset(kobo.Pad), s, kobo.Black, kobo.AlignLeft)
	a.Mark(b, true)
}

func (a *tools) keyboardStatus() string {
	if a.Keyboard == "" {
		return "no keyboard: pair one in Kobo Settings > Bluetooth"
	}
	return "connected: " + a.Keyboard
}

// --- network ---

var netProbes = [numNetPanels]func(chan<- netMsg){netWifi: wifiStatus, netScan: scan, netMDNS: mdnsBrowse, netHTTP: httpTest}

func (a *tools) startNet(i int) {
	st := &a.netState[i]
	if st.busy {
		return
	}
	*st = netPanelState{busy: true, lines: st.lines}
	log.Printf("network: starting probe %d", i)
	go netProbes[i](a.netc)
	if a.Tab == tabNet && !a.Calibrating() {
		a.drawNetPanel(i)
	}
}

func (a *tools) onNet(m netMsg) {
	a.netState[m.panel] = netPanelState{status: m.status, lines: m.lines, busy: !m.done}
	if m.done {
		log.Printf("network %d: %s\n\t%s", m.panel, m.status, joinLines(m.lines))
	}
	if a.Tab == tabNet && !a.Calibrating() {
		a.drawNetPanel(m.panel)
	}
}

var netHints = [numNetPanels]string{
	netWifi: "Tap refresh.",
	netScan: "Tap scan: nearby Wi-Fi networks, then hosts on this network.",
	netMDNS: "Tap browse: services announced over multicast DNS (printers, TVs, AirPlay, …).",
	netHTTP: "Tap GET to fetch " + httpTestURL + ".",
}

func (a *tools) drawNetPanel(i int) {
	p, st := a.L.net[i], a.netState[i]
	status := st.status
	if st.busy && status == "" {
		status = "working…"
	}
	a.DrawPanel(p, status)
	f := a.F.Small
	if i == netWifi && len(st.lines) <= 7 {
		f = a.F.Body
	}
	a.DrawText(p, f, st.lines, netHints[i], true)
}

// --- info ---

func (a *tools) drawInfo() {
	L := a.L
	devs, screen, touch := a.InfoLines()
	a.DrawPanel(L.devices, fmt.Sprintf("%d", len(devs)))
	a.DrawLines(L.devices, devs, "none found")
	a.DrawPanel(L.screenP, "")
	a.DrawLines(L.screenP, screen, "")
	a.DrawPanel(L.touchP, "")
	a.DrawLines(L.touchP, touch, "")
}

func joinLines(lines []string) string { return strings.Join(lines, "\n\t") }
