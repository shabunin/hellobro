// hellobro is an input playground for the Kobo Elipsa.
//
// Tabs:
//   - Keyboard: type with a USB or Bluetooth keyboard;
//   - Finger: draw with a finger (the pen is ignored there);
//   - Stylus: draw with the pen, pressure sets the width (fingers are
//     ignored on the canvas, so a resting palm doesn't draw);
//   - Network: Wi-Fi status, a scan (access points, hosts on the local
//     network) and an HTTPS request;
//   - Sound: Bluetooth audio devices, and a player (test tone, MP3, WAV);
//   - SSH: terminals (xterm emulation, PTY) over SSH, several at once;
//   - Shell: terminals with a shell on the Kobo itself;
//   - System: the device, kernel, memory, storage, battery, tools;
//   - Info: input devices, screen and touch mapping details.
//
// The UI follows the orientation the Kobo is held in at launch, and the
// "rotate" button cycles through the others. Each orientation gets a
// two-tap touch calibration the first time it's used.
//
// Swiping a finger up or down along the left edge sets the frontlight
// (except on the finger canvas, where it draws).
//
// Esc, "exit", the power button, or 3 minutes without input quits.
//
// Launch it through hellobro.sh, which keeps Nickel from drawing over us.
package main

import (
	"fmt"
	"image"
	"image/draw"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	idleTimeout    = 3 * time.Minute
	sshIdleTimeout = 20 * time.Minute
	tapSlop        = 30 // px a tap may wander
	tapTime        = 700 * time.Millisecond
	maxLog         = 200
	keyF1          = 59
)

// gesture is one contact, from down to up.
type gesture struct {
	start, last image.Point
	at          time.Time
	pen         bool
	points      int
	moved       float64
	maxPressure int32
	canvas      tabID // the canvas it draws on, or -1
	ignored     string

	lightZone bool // started on the left edge: may become a light swipe
	light     bool // is a light swipe
	lightFrom int  // light level when the swipe started

	fontZone bool // started on the right edge of the terminal: may become a font swipe
	font     bool // is a font swipe
	fontFrom int  // the terminal's font size when it started
	fontTo   int  // the size it will get when the finger lifts
}

const (
	lightEdge  = 110 // px from the left (light) and right (terminal font) edges where swipes start
	lightStart = 40  // px of vertical travel before a swipe counts
)

type calibState struct {
	raw  []image.Point
	note string
}

// dirtyRect is an area to send to the panel. Ink-only areas use the
// fast black & white waveform, anything else a grayscale one.
type dirtyRect struct {
	r  image.Rectangle
	ui bool
}

type blitJob struct {
	img   *image.Gray
	wf    string
	flash bool
}

type netPanelState struct {
	status string
	lines  []string
	busy   bool
}

type app struct {
	scr   *screen
	f     fonts
	c     canvas
	L     layout
	tab   tabID
	full  bool // the terminal has the whole screen (F11)
	stats loopStats

	touch     *touchDevice
	tmap      touchMap
	cal       map[int]axes
	calib     *calibState // non-nil while calibrating
	calibNote string
	gestures  map[int32]*gesture

	devs     []inputDev
	keyboard string
	ed       editor
	shown    []string // typing rows currently on screen
	lastKey  string

	light        *frontlight
	lightOverlay *image.Gray // what the light gauge covers, while it's shown
	fontOverlay  *image.Gray // the same for the terminal font gauge

	fingerImg, stylusImg *image.Gray
	fingerLog, stylusLog []string
	penStatus, penShown  string
	penAt                time.Time

	netState [numNetPanels]netPanelState
	netc     chan netMsg
	snd      soundState
	tv       termView
	sys      sysState
	inputAt  time.Time   // last touch or key
	call     chan func() // work from goroutines, run on the UI loop

	status   string
	dirty    []dirtyRect     // pending screen updates, blitted one at a time
	flash    bool            // the next full-screen update should flash
	flashR   image.Rectangle // an area whose next update should flash (GC16), to clear ghosting
	blitting bool
	jobs     chan blitJob
	done     chan error
	quit     bool
	alive    atomic.Int64 // last loop tick, for the watchdog
	blitAt   atomic.Int64 // start of the blit in flight, or 0
	noPower  bool         // the power button doesn't reach us
}

func main() {
	if f, err := openLog(); err == nil {
		defer f.Close()
		log.SetOutput(io.MultiWriter(os.Stderr, f))
		logFile = f
		go func() { // vfat + a hard reset loses unsynced writes, i.e. the interesting end of the log
			for range time.Tick(2 * time.Second) {
				f.Sync()
			}
		}()
	}
	log.Print("hellobro starting")

	scr, err := openScreen()
	if err != nil {
		log.Fatal(err)
	}
	a := &app{
		scr: scr, f: loadFonts(), cal: loadCalibrations(), gestures: map[int32]*gesture{},
		jobs: make(chan blitJob), done: make(chan error), penStatus: "pen away",
		netc: make(chan netMsg, 16), call: make(chan func(), 64),
	}
	a.snd.gain = 40

	a.light = openFrontlight()
	a.devs = inputDevices()
	touches := make(chan touchEv, 4096)
	for _, d := range a.devs {
		log.Printf("input device %s: %q %s", d.handler, d.name, d.role())
		if d.isTouch() && a.touch == nil {
			if a.touch, err = openTouch(d); err != nil {
				log.Printf("touch: %v", err)
				a.touch = nil
				continue
			}
			go a.touch.run(touches)
		}
	}
	keys := make(chan kbdMsg, 256)
	go watchKeyboards(keys)
	for _, d := range a.devs {
		if d.role() == "buttons" {
			go watchButtons(d, keys)
		}
	}
	go func() {
		for j := range a.jobs {
			a.blitAt.Store(time.Now().UnixNano())
			err := a.scr.blit(j.img, j.img.Bounds(), j.wf, j.flash)
			a.blitAt.Store(0)
			a.done <- err
		}
	}()

	a.relayout()
	a.loop(touches, keys)

	a.stopPlayback("")
	a.termCloseAll()
	a.waitBlit()
	a.dirty = nil
	a.drawBye()
	a.flush()
	a.waitBlit()
	scr.close()
	if a.light != nil {
		a.light.close()
	}
	log.Print("bye")
}

func openLog() (*os.File, error) {
	path := "hellobro.log"
	if exe, err := os.Executable(); err == nil {
		path = filepath.Join(filepath.Dir(exe), path)
	}
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
}

func (a *app) loop(touches <-chan touchEv, keys <-chan kbdMsg) {
	go a.watchdog()
	idle := time.NewTimer(idleTimeout)
	tick := time.NewTicker(30 * time.Millisecond)
	clock := time.NewTicker(20 * time.Second)
	for !a.quit {
		a.stats.start()
		select {
		case ev := <-touches:
			a.stats.event("touch", time.Since(ev.at))
			a.onTouch(ev)
			idle.Reset(idleTimeout)
			a.inputAt = time.Now()
		case m := <-keys:
			if m.at.IsZero() {
				a.stats.event("keyboard", 0) // a keyboard came or went
			} else {
				a.stats.event("key", time.Since(m.at))
			}
			a.onKey(m)
			if m.connected == "" && m.gone == "" {
				idle.Reset(idleTimeout)
				a.inputAt = time.Now()
			}
		case err := <-a.done:
			a.stats.event("blit done", 0)
			a.blitting = false
			if err != nil {
				log.Print(err)
			}
			a.flush() // keep the panel busy while there's something to show
		case m := <-a.netc:
			a.stats.event("net", 0)
			a.onNet(m)
		case f := <-a.call:
			a.stats.event("call", 0)
			f()
		case <-tick.C:
			a.stats.event("tick", 0)
			a.alive.Store(time.Now().UnixNano())
			a.updatePenStatus()
			a.termTick()
			a.flush()
		case <-clock.C:
			a.stats.event("clock", 0)
			a.drawStatus()
		case <-idle.C:
			if a.termLive() && time.Since(a.inputAt) < sshIdleTimeout { // reading a remote screen takes longer
				idle.Reset(sshIdleTimeout - time.Since(a.inputAt))
				continue
			}
			log.Print("idle timeout")
			a.quit = true
		}
		a.stats.done()
	}
}

// loopStats times the main loop, so the log shows where a slow UI spends
// its time: in our handlers (busy, slowest) or waiting to get to the
// input at all (lag).
type loopStats struct {
	since, began  time.Time
	what          string
	events        int
	busy, slowest time.Duration
	slowestWhat   string
	lag, maxLag   time.Duration
	inputs        int
}

func (s *loopStats) start() {
	if s.since.IsZero() {
		s.since = time.Now()
	}
}

func (s *loopStats) event(what string, lag time.Duration) {
	s.began, s.what = time.Now(), what
	if what == "key" || what == "touch" {
		s.inputs++
		s.lag += lag
		s.maxLag = max(s.maxLag, lag)
	}
}

func (s *loopStats) done() {
	if s.began.IsZero() {
		return
	}
	d := time.Since(s.began)
	s.began = time.Time{}
	s.events++
	s.busy += d
	if d > s.slowest {
		s.slowest, s.slowestWhat = d, s.what
	}
	if time.Since(s.since) < 10*time.Second || s.inputs == 0 && s.slowest < 100*time.Millisecond {
		return
	}
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	avgLag := time.Duration(0)
	if s.inputs > 0 {
		avgLag = s.lag / time.Duration(s.inputs)
	}
	log.Printf("loop: %d events in %.0fs, busy %v, slowest %v (%s); %d inputs, lag avg %v max %v; heap %d MB, %d goroutines",
		s.events, time.Since(s.since).Seconds(), s.busy.Round(time.Millisecond), s.slowest.Round(time.Millisecond), s.slowestWhat,
		s.inputs, avgLag.Round(time.Millisecond), s.maxLag.Round(time.Millisecond), ms.HeapAlloc>>20, runtime.NumGoroutine())
	*s = loopStats{since: time.Now()}
}

// watchdog quits if the loop gets stuck: Nickel stays frozen while we run.
// A blit stuck in the panel driver can't be interrupted, and would keep
// us (and the frozen Nickel) hanging forever: give Nickel back and leave.
func (a *app) watchdog() {
	a.alive.Store(time.Now().UnixNano())
	for range time.Tick(2 * time.Second) {
		if stuck := time.Since(time.Unix(0, a.alive.Load())); stuck > 30*time.Second {
			bail(fmt.Sprintf("main loop stuck for %v", stuck.Round(time.Second)))
		}
		if at := a.blitAt.Load(); at != 0 {
			if stuck := time.Since(time.Unix(0, at)); stuck > 10*time.Second {
				bail(fmt.Sprintf("screen update stuck in the driver for %v", stuck.Round(time.Second)))
			}
		}
	}
}

var logFile *os.File

// bail thaws Nickel itself (hellobro.sh may never get to) and exits.
func bail(why string) {
	log.Printf("%s, bailing out", why)
	for _, pid := range strings.Fields(os.Getenv("HELLOBRO_FROZEN")) {
		if n, err := strconv.Atoi(pid); err == nil {
			syscall.Kill(n, syscall.SIGCONT)
		}
	}
	if logFile != nil {
		logFile.Sync()
	}
	os.Exit(3)
}

// relayout rebuilds everything for the current screen orientation.
func (a *app) relayout() {
	s := a.scr.info
	a.c = canvas{image.NewGray(image.Rect(0, 0, s.viewW, s.viewH))}
	a.L = makeLayout(s.viewW, s.viewH, a.f, a.full)
	if a.tv.cw != 0 {
		a.termGeometry() // the terminal follows the new size
	}
	// New canvases when their size changes; F11 and back keeps the drawings.
	if sz := a.L.fingerCanvas.body.Size(); !a.full && (a.fingerImg == nil || a.fingerImg.Bounds().Size() != sz) {
		a.fingerImg = blank(sz)
	}
	if sz := a.L.stylusCanvas.body.Size(); !a.full && (a.stylusImg == nil || a.stylusImg.Bounds().Size() != sz) {
		a.stylusImg = blank(sz)
	}
	a.shown = nil
	a.dirty = nil
	a.gestures = map[int32]*gesture{}

	a.tmap = touchMap{w: s.viewW, h: s.viewH, ox: s.originX, oy: s.originY}
	if a.touch != nil {
		a.tmap.rx, a.tmap.ry = a.touch.rx, a.touch.ry
	}
	// No range from the kernel: assume raw units are panel pixels.
	pw, ph := min(s.screenW, s.screenH), max(s.screenW, s.screenH)
	if s.swap {
		pw, ph = ph, pw
	}
	if a.tmap.rx.max <= a.tmap.rx.min {
		a.tmap.rx = absRange{0, int32(pw - 1)}
	}
	if a.tmap.ry.max <= a.tmap.ry.min {
		a.tmap.ry = absRange{0, int32(ph - 1)}
	}
	if c, ok := a.cal[s.rota]; ok {
		a.tmap.axes = c
		log.Printf("touch mapping for rota %d: %s (calibrated; FBInk default: %s)", s.rota, c, defaultAxes(s))
	} else {
		a.tmap.axes = defaultAxes(s)
		log.Printf("touch mapping for rota %d: %s (FBInk default, not calibrated yet)", s.rota, a.tmap.axes)
		if a.touch != nil {
			a.startCalib("")
			return
		}
	}
	a.drawAll(true)
}

func (a *app) rotate() {
	a.waitBlit()
	if err := a.scr.setRotation((a.scr.info.rota + 1) % 4); err != nil {
		log.Printf("rotate: %v", err)
	}
	a.relayout()
}

func (a *app) switchTab(t tabID) {
	if t == a.tab || a.calib != nil {
		return
	}
	a.tab = t
	if t == tabNet && a.netState[netWifi].lines == nil {
		a.startNet(netWifi)
	}
	if t == tabSound {
		a.enterSound()
	}
	if isTermTab(t) {
		a.enterTerm()
	}
	if t == tabSystem && a.sys.at.IsZero() {
		a.startSys()
	}
	if a.full {
		a.setFull(false)
		return
	}
	a.drawTabs()
	a.drawPage()
}

// setFull gives the terminal the whole screen, or puts the app back around it.
func (a *app) setFull(on bool) {
	if on == a.full {
		return
	}
	log.Printf("fullscreen terminal: %v", on)
	a.full = on
	a.waitBlit()
	a.relayout()
}

// --- touch ---

func (a *app) onTouch(ev touchEv) {
	p := a.tmap.toView(ev.x, ev.y)
	if ev.pen {
		a.penAt = ev.at
	}
	if a.calib != nil {
		if ev.kind == touchUp {
			a.calibTap(ev)
		}
		return
	}

	switch ev.kind {
	case touchHover:
		a.penStatus = fmt.Sprintf("pen hovering at %d, %d", p.X, p.Y)

	case touchDown:
		g := &gesture{start: p, last: p, at: ev.at, pen: ev.pen, points: 1, canvas: -1, maxPressure: ev.pressure}
		switch {
		case a.tab == tabFinger && p.In(a.L.fingerCanvas.body):
			if ev.pen {
				g.ignored = "pen ignored on the Finger tab"
			} else {
				g.canvas = tabFinger
			}
		case a.tab == tabStylus && p.In(a.L.stylusCanvas.body):
			if ev.pen {
				g.canvas = tabStylus
			} else {
				g.ignored = "finger ignored on the Stylus tab"
			}
		}
		g.lightZone = a.light != nil && !ev.pen && p.X < lightEdge && g.canvas < 0 &&
			!(a.tab == tabFinger && p.In(a.L.fingerCanvas.body))
		g.fontZone = isTermTab(a.tab) && !ev.pen && p.X >= a.c.Bounds().Dx()-lightEdge
		a.gestures[ev.slot] = g
		if g.canvas >= 0 {
			a.ink(g.canvas, p, p, ev.pressure)
		}
		if ev.pen {
			a.penStatus = fmt.Sprintf("pen down, pressure %d", ev.pressure)
		}

	case touchMove:
		g := a.gestures[ev.slot]
		if g == nil {
			return
		}
		g.points++
		g.moved = math.Max(g.moved, math.Hypot(float64(p.X-g.start.X), float64(p.Y-g.start.Y)))
		g.maxPressure = max(g.maxPressure, ev.pressure)
		if dy := g.start.Y - p.Y; g.lightZone && !g.light && abs(dy) > lightStart && abs(dy) > 2*abs(p.X-g.start.X) {
			g.light, g.lightFrom = true, a.light.level
			log.Printf("light swipe from %d%%", g.lightFrom)
		}
		if g.light {
			// Half the screen height covers the whole range.
			a.setLight(g.lightFrom + (g.start.Y-p.Y)*100/(a.c.Bounds().Dy()/2))
		}
		if dy := g.start.Y - p.Y; g.fontZone && !g.font && abs(dy) > lightStart && abs(dy) > 2*abs(p.X-g.start.X) {
			g.font, g.fontFrom = true, a.tv.fontPx
			log.Printf("font swipe from %d px", g.fontFrom)
		}
		if g.font {
			a.termFontSwipe(g, g.start.Y-p.Y)
		}
		if g.canvas >= 0 {
			a.ink(g.canvas, g.last, p, ev.pressure)
		}
		g.last = p
		if ev.pen {
			a.penStatus = fmt.Sprintf("pen down, pressure %d", ev.pressure)
		}

	case touchUp:
		g := a.gestures[ev.slot]
		delete(a.gestures, ev.slot)
		if g == nil {
			return
		}
		if g.light {
			log.Printf("light swipe: %d%% → %d%%", g.lightFrom, a.light.level)
			a.hideLightGauge()
			return
		}
		if g.font {
			a.hideGauge(&a.fontOverlay)
			a.setTermFont(g.fontTo)
			return
		}
		dur := ev.at.Sub(g.at)
		tap := g.moved < tapSlop && dur < tapTime
		if tap && g.canvas < 0 && g.ignored == "" {
			a.tapAt(g.start)
		} else if !tap && !g.pen && isTermTab(a.tab) && a.calib == nil {
			a.termSwipe(g.start, g.last)
		}
		a.logGesture(g, tap, dur)
		if ev.pen {
			a.penStatus = "pen lifted"
		}
	}
}

func (a *app) tapAt(p image.Point) {
	if a.quit {
		return
	}
	for i, r := range a.L.tabs {
		if p.In(r) {
			a.switchTab(tabID(i))
			return
		}
	}
	switch {
	case hit(p, a.L.btnExit):
		log.Print("exit tapped")
		a.quit = true
	case hit(p, a.L.btnRotate):
		a.rotate()
	case hit(p, a.L.btnCalib):
		a.startCalib("")
	case a.tab == tabFinger && hit(p, a.L.fingerCanvas.button):
		a.fingerImg = blank(a.fingerImg.Bounds().Size())
		a.drawPage()
	case a.tab == tabStylus && hit(p, a.L.stylusCanvas.button):
		a.stylusImg = blank(a.stylusImg.Bounds().Size())
		a.drawPage()
	case a.tab == tabNet:
		for i, np := range a.L.net {
			if hit(p, np.button) {
				a.startNet(i)
			}
		}
	case a.tab == tabSound:
		a.tapSound(p)
	case isTermTab(a.tab):
		a.termTap(p)
	case a.tab == tabSystem && hit(p, a.L.sys[sysDevice].button):
		a.startSys()
	}
}

// --- network ---

var netProbes = [numNetPanels]func(chan<- netMsg){netWifi: wifiStatus, netScan: scan, netMDNS: mdnsBrowse, netHTTP: httpTest}

func (a *app) startNet(i int) {
	st := &a.netState[i]
	if st.busy {
		return
	}
	*st = netPanelState{busy: true, lines: st.lines}
	log.Printf("network: starting probe %d", i)
	go netProbes[i](a.netc)
	if a.tab == tabNet && a.calib == nil {
		a.drawNetPanel(i)
	}
}

func (a *app) onNet(m netMsg) {
	a.netState[m.panel] = netPanelState{status: m.status, lines: m.lines, busy: !m.done}
	if m.done {
		log.Printf("network %d: %s\n\t%s", m.panel, m.status, joinLines(m.lines))
	}
	if a.tab == tabNet && a.calib == nil {
		a.drawNetPanel(m.panel)
	}
}

// ink draws a stroke segment on a canvas page (offscreen and on screen).
func (a *app) ink(t tabID, from, to image.Point, pressure int32) {
	p, img, radius := a.L.fingerCanvas, a.fingerImg, 4
	if t == tabStylus {
		p, img = a.L.stylusCanvas, a.stylusImg
		radius = 1 + int(7*math.Min(1, float64(pressure)/float64(a.pressureMax())))
	}
	o := p.body.Min
	canvas{img}.line(from.Sub(o), to.Sub(o), radius, black, img.Bounds())
	r := image.Rectangle{from, to}.Canon().Inset(-radius - 1).Intersect(p.body)
	draw.Draw(a.c.Gray, r, img, r.Min.Sub(o), draw.Src)
	a.mark(r, false)
}

func (a *app) pressureMax() int32 {
	if a.touch != nil && a.touch.rpress.max > 0 {
		return a.touch.rpress.max
	}
	return 1023
}

func (a *app) logGesture(g *gesture, tap bool, dur time.Duration) {
	what := fmt.Sprintf("stroke  %d pts  %.2fs  %d,%d → %d,%d", g.points, dur.Seconds(), g.start.X, g.start.Y, g.last.X, g.last.Y)
	if tap {
		what = fmt.Sprintf("tap  %d,%d", g.start.X, g.start.Y)
	}
	if g.pen {
		what += fmt.Sprintf("  max pressure %d", g.maxPressure)
	}
	if g.ignored != "" {
		what += "  (" + g.ignored + ")"
	}
	tool, page, lines := "finger", tabFinger, &a.fingerLog
	if g.pen {
		tool, page, lines = "pen", tabStylus, &a.stylusLog
	}
	log.Printf("%s %s", tool, what)
	*lines = append(*lines, what)
	if len(*lines) > maxLog {
		*lines = (*lines)[len(*lines)-maxLog:]
	}
	if a.tab == page && a.calib == nil {
		if page == tabFinger {
			a.drawPanelTitle(a.L.fingerLog, fmt.Sprintf("%d", len(*lines)))
			a.drawLines(a.L.fingerLog, *lines, "")
		} else {
			a.drawLines(a.L.stylusLog, *lines, "")
		}
	}
}

// updatePenStatus refreshes the stylus canvas title when the pen state
// changed, without holding up ink that is still being drawn.
func (a *app) updatePenStatus() {
	if a.penStatus != "pen away" && time.Since(a.penAt) > 1500*time.Millisecond {
		a.penStatus = "pen away"
	}
	if a.tab != tabStylus || a.calib != nil || a.penStatus == a.penShown || a.blitting {
		return
	}
	a.penShown = a.penStatus
	a.drawPanelTitle(a.L.stylusCanvas, a.penStatus)
}

// --- calibration ---

func (a *app) startCalib(note string) {
	a.lightOverlay, a.fontOverlay = nil, nil
	a.calib = &calibState{note: note}
	a.gestures = map[int32]*gesture{}
	a.dirty = nil
	a.drawCalib()
}

func (a *app) calibTap(ev touchEv) {
	a.calib.raw = append(a.calib.raw, image.Pt(int(ev.x), int(ev.y)))
	if len(a.calib.raw) < len(calibrationTargets) {
		a.drawCalib()
		return
	}
	s := a.scr.info
	got, miss := calibrate(a.tmap, a.calib.raw)
	def := defaultAxes(s)
	log.Printf("calibration for rota %d: %s, mean miss %.0f px, raw taps %v (FBInk default: %s)", s.rota, got, miss, a.calib.raw, def)
	if miss > 150 {
		a.startCalib(fmt.Sprintf("Missed by %.0f px on average, let's try again.", miss))
		return
	}
	a.cal[s.rota] = got
	saveCalibrations(a.cal)
	a.tmap.axes = got
	verdict := "matches"
	if got != def {
		verdict = "differs from"
	}
	a.calibNote = fmt.Sprintf("calibrated (miss %.0f px), %s FBInk's default", miss, verdict)
	a.calib = nil
	a.drawAll(true)
}

// --- keyboard ---

func (a *app) onKey(m kbdMsg) {
	if isTermTab(a.tab) && a.calib == nil && !m.button && m.connected == "" && m.gone == "" && a.termKey(m.code, m.value) {
		return
	}
	switch {
	case m.button && m.ungrabbed:
		a.noPower = true
		if a.calib == nil {
			a.drawFooter()
		}
		return
	case isMediaKey(m.code):
		if m.value == 1 {
			a.mediaKey(m.code)
		}
		return
	case m.button:
		log.Printf("button %d value %d", m.code, m.value)
		if m.code == keyPower && m.value == 0 { // on release, so Nickel doesn't get a stray one
			log.Print("power button pressed")
			a.quit = true
		}
		return
	case m.connected != "" || m.gone != "":
		if m.connected != "" {
			log.Printf("keyboard connected: %s", m.connected)
			a.keyboard = m.connected
		} else {
			log.Printf("keyboard gone: %s", m.gone)
			a.keyboard = ""
			a.setFull(false) // F11 is the only way back, so don't strand the user there
		}
		a.devs = inputDevices()
		if a.calib == nil && a.tab == tabKeys {
			a.drawPanelTitle(a.L.typing, a.keyboardStatus())
		} else if a.calib == nil && a.tab == tabInfo {
			a.drawPage()
		}
		return
	case m.code == keyEsc && m.value == 1:
		log.Print("Esc pressed")
		a.quit = true
		return
	case m.code >= keyF1 && m.code < keyF1+uint16(numTabs) && m.value == 1:
		a.switchTab(tabID(m.code - keyF1))
		return
	}
	desc, changed := a.ed.key(m.code, m.value)
	if desc != "" {
		log.Printf("key %s", desc)
		a.lastKey = desc
	}
	if a.calib != nil || a.tab != tabKeys {
		return
	}
	if desc != "" {
		a.drawLastKey()
	}
	if changed {
		a.drawTyping()
	}
}

// --- screen updates ---

func area(r image.Rectangle) int { return r.Dx() * r.Dy() }

// compact says whether one update covering a and b wastes little.
func compact(a, b image.Rectangle) bool { return area(a.Union(b)) <= (area(a)+area(b))*3/2 }

// mark queues r for the panel, merged into a pending area of the same kind
// when that doesn't make it much bigger: small updates stay small.
func (a *app) mark(r image.Rectangle, ui bool) {
	if r.Empty() {
		return
	}
	for i, d := range a.dirty {
		if d.ui == ui && compact(d.r, r) {
			a.dirty[i].r = d.r.Union(r)
			return
		}
	}
	a.dirty = append(a.dirty, dirtyRect{r, ui})
}

// markFlash redraws r with a flash, which clears the ghosting that
// non-flashing updates leave behind.
func (a *app) markFlash(r image.Rectangle) {
	a.mark(r, true)
	a.flashR = r
}

func (a *app) markAll(flash bool) {
	a.dirty = []dirtyRect{{a.c.Bounds(), true}}
	a.flash = a.flash || flash
}

// flush sends the oldest pending area, with those of its kind close
// enough to share an update, unless a blit is still in flight: updates
// pile up meanwhile, which coalesces them. Far-apart areas go one after
// the other, since the panel's time grows with the area: a typed row and
// the bottom of the screen mustn't become a refresh of all in between.
// If they pile up anyway, everything of a kind goes at once, so the
// screen can't fall further and further behind.
func (a *app) flush() {
	if a.blitting || len(a.dirty) == 0 {
		return
	}
	d := a.dirty[0]
	rest := a.dirty[:0]
	all := len(a.dirty) > 4
	for _, o := range a.dirty[1:] {
		if o.ui == d.ui && (all || compact(d.r, o.r)) {
			d.r = d.r.Union(o.r)
		} else {
			rest = append(rest, o)
		}
	}
	a.dirty = rest
	img := image.NewGray(d.r)
	draw.Draw(img, d.r, a.c.Gray, d.r.Min, draw.Src)
	flash := a.flash && d.r == a.c.Bounds()
	if !flash && d.ui && !a.flashR.Empty() && a.flashR.In(d.r) {
		flash, a.flashR = true, image.Rectangle{}
	}
	wf := "DU"
	switch {
	case flash:
		wf = "GC16"
		if d.r == a.c.Bounds() {
			a.flash = false
		}
	case d.ui:
		wf = "GL16"
	}
	a.jobs <- blitJob{img, wf, flash}
	a.blitting = true
}

func (a *app) waitBlit() {
	if a.blitting {
		if err := <-a.done; err != nil {
			log.Print(err)
		}
		a.blitting = false
	}
}

func joinLines(lines []string) string { return strings.Join(lines, "\n\t") }
