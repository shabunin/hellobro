// Package kobo is what hellobro's apps share: an app that takes over the
// Kobo's screen from Nickel and draws its own UI, with tabs, through FBInk.
//
// The App does the screen (orientation, the update queue, e-ink
// waveforms), touch (calibration, taps, swipes along the left edge for the
// frontlight), keyboards and buttons, the header, the tab bar and the
// footer. A Program fills in the pages.
//
// The UI follows the orientation the Kobo is held in at launch, and the
// "rotate" button cycles through the others. Each orientation gets a
// two-tap touch calibration the first time it's used.
//
// Esc (but with Config.OwnKeys), "exit", or a while without input quits.
//
// Apps are launched through launch.sh, which keeps Nickel from drawing over them.
package kobo

import (
	"fmt"
	"image"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	// IdleTimeout is how long without input before an app quits, unless
	// its Program asks for longer.
	IdleTimeout = 3 * time.Minute
	tapSlop     = 30 // px a tap may wander
	tapTime     = 700 * time.Millisecond
)

// A Program is an app's pages and what they do. The App calls it from its
// loop, one call at a time; goroutines get back onto the loop with Do.
type Program interface {
	// Layout places the pages' panels in L, for a new orientation or
	// fullscreen.
	Layout(L Layout)
	// Draw draws the current tab's page, which has just been cleared.
	Draw()
	// Enter is called when tab becomes the current one (the first tab too,
	// at the start), before it's drawn.
	Enter(tab int)
	// Key sees every key event first, but Ctrl+Alt+1…9 (tabs), and true
	// takes it: the App doesn't quit on Esc then. It also hears of
	// keyboards coming and going (m.Connected, m.Gone).
	Key(m KeyMsg) bool
	// Touch sees a touch event at p: a contact starting, moving (g.Last is
	// still the point before) or the pen hovering (g is nil). On a
	// TouchDown the Program can take g over, setting g.Owner: it then
	// gets no tap, and it's no light swipe.
	Touch(ev TouchEv, p image.Point, g *Gesture)
	// Tap is a tap on the page.
	Tap(p image.Point)
	// Lift is a contact ending (g.Tap says if it was a tap), unless it was
	// a light swipe.
	Lift(g *Gesture)
	// Tick comes every 30 ms.
	Tick()
	// IdleTimeout is how long without input before the app quits.
	IdleTimeout() time.Duration
	// Exit is called when the app quits.
	Exit()
}

// Config names an app.
type Config struct {
	Name  string   // the binary's, for the log: Name.log next to it
	Title string   // in the header
	About []string // at the foot of the cards, a line each: the version, where it's from
	Tabs  []string // Ctrl+Alt+1, 2, …
	// Bar puts the header and the tabs in one bar at the top: no title,
	// the tabs with what's in them (SetTabNames' info), and the status.
	// No buttons: the Program has a menu of its own.
	Bar bool
	// OwnKeys leaves Ctrl+Alt+1–9 (tabs) and Ctrl+Alt+S (sleep) to the
	// Program's Key, for an app with keys of its own making, and Esc
	// doesn't quit: quitting is the Program's to do (App.Quit).
	OwnKeys bool
}

// Gesture is one contact, from down to up.
type Gesture struct {
	Start, Last image.Point
	At          time.Time
	Pen         bool
	Points      int
	Moved       float64 // the farthest it got from Start
	MaxPressure int32
	Ignored     string // why it does nothing, for the log; no tap then
	Owner       int    // nonzero: the Program took it (for what is up to it)
	Tap         bool   // once it's over: whether it was a tap
	Dur         time.Duration

	lightZone bool // started on the left edge: may become a light swipe
	light     bool // is a light swipe
	lightFrom int  // light level when the swipe started
	unfull    int  // fullscreen, started on the top (1) or bottom (-1) edge: in from it leaves fullscreen
	unfulled  bool // it did
}

const (
	EdgeWidth  = 110 // px from the left (light) or right edge where edge swipes start
	SwipeStart = 40  // px of vertical travel before an edge swipe counts
	unfullEdge = 80  // px from the top or bottom where a swipe in leaves fullscreen
	unfullMove = 150 // px it takes
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

// App is the app shell; a Program embeds it, and uses what's exported.
type App struct {
	C          Canvas // what's on screen, or about to be
	F          Fonts
	L          Layout
	Tab        int
	Full       bool           // the page has the whole screen
	Keyboard   string         // the keyboard connected, if any
	kbds       map[string]int // the keyboards connected, by name: how many of each
	LightIdle  time.Duration  // the light goes off after this long untouched (0: never)
	LightStart *int           // the light's level at start; nil: Nickel's
	LightKept  func(int)      // the reader set the light to this

	prog  Program
	cfg   Config
	scr   *screen
	stats loopStats

	touch     *touchDevice
	tmap      touchMap
	cal       map[int]axes
	calib     *calibState // non-nil while calibrating
	calibNote string
	gestures  map[int32]*Gesture
	devs      []inputDev
	entered   bool // the first tab got its Enter

	light        *frontlight
	lightOverlay *image.Gray // what the light gauge covers, while it's shown
	lightHideAt  time.Time   // when the gauge SetLight put up goes
	notice       string      // Notify's, in the header for a moment
	noticeUntil  time.Time
	order        []int       // the tabs shown, in order; nil: all
	hints        []string    // in the tabs' corners, by place; nil: numbers
	tabInfo      []string    // Config.Bar's: what's in each tab, by place
	footerTabs   string      // the footer's word on the tabs; "": Ctrl+Alt+N
	footerSleep  string      // and on sleep's key; "": Ctrl+Alt+S
	footerQuit   string      // and on quit's; "": Esc
	gaugeOverlay *image.Gray // the same for the Program's gauge

	inputAt time.Time   // last touch or key
	call    chan func() // work from goroutines, run on the UI loop

	status     string          // the header's status text, as drawn
	note       string          // the Program's, left of it
	network    string          // the Program's, first: the Wi-Fi and the address
	dirty      []dirtyRect     // pending screen updates, blitted one at a time
	flash      bool            // the next full-screen update should flash
	flashR     image.Rectangle // an area whose next update should flash (GC16), to clear ghosting
	blitting   bool
	ink        bool // Ink's: black & white updates go in pen mode
	jobs       chan blitJob
	done       chan error
	quit       bool
	alive      atomic.Int64    // last loop tick, for the watchdog
	blitAt     atomic.Int64    // start of the blit in flight, or 0
	noPower    bool            // the power button doesn't reach us
	wokeAt     time.Time       // from sleep: the power press that woke us comes after
	asleep     bool            // the sleep screen is up: nothing else gets drawn
	sleeping   atomic.Bool     // asleep, for the watchdog: the clock jumps
	locked     bool            // touch does nothing, keys only what the Program lets through (lock.go)
	lockHint   string          // how to unlock
	eatTouch   bool            // a contact that began while locked isn't over
	lockLines  []string        // what else the lock screen shows
	lockC      Canvas          // the lock screen, on the panel while locked
	lockLight  int             // the light before locking
	lockSwipes []LockSwipe     // the lock screen's swipes, if any
	lockSlot   int32           // the contact that may be one, or -1
	lockFrom   image.Point     // where it came down
	lockTo     image.Point     // and where it is
	restart    string          // a binary to run in our place once we've quit
	ctrl       [2]bool         // left and right Ctrl held, for Ctrl+Alt+N
	alt        [2]bool         // and Alt
	ring       image.Rectangle // the focus ring's outer edge, if it shows (ring.go)
	touchMarks bool            // where touches land, shown (touchmark.go)
	marks      []touchMark

	blinks     []blinking // notice.go
	toast      *toast
	lightOffAt time.Time // FlashLight's light goes off
	lightIdled int       // the light's level before idling turned it off; 0: it didn't
}

// New opens the log and the screen. The Program, which embeds the App, is
// told the pages' layout once Run starts.
func New(cfg Config, prog Program) *App {
	if f, err := openLog(cfg.Name); err == nil {
		log.SetOutput(io.MultiWriter(os.Stderr, f))
		logFile = f
		go func() { // vfat + a hard reset loses unsynced writes, i.e. the interesting end of the log
			for range time.Tick(2 * time.Second) {
				f.Sync()
			}
		}()
	}
	log.Printf("%s starting", cfg.Name)

	scr, err := openScreen()
	if err != nil {
		log.Fatal(err)
	}
	return &App{
		prog: prog, cfg: cfg, scr: scr, F: LoadFonts(), cal: loadCalibrations(), gestures: map[int32]*Gesture{},
		jobs: make(chan blitJob), done: make(chan error), call: make(chan func(), 64),
		inputAt: time.Now(), // idle from the start, not since 1970: the light, sleep and auto-exit count from here
	}
}

// Run runs the app until it quits, then gives the screen back.
func (a *App) Run() {
	var err error
	go func() {
		for j := range a.jobs {
			a.blitAt.Store(time.Now().UnixNano())
			err := a.scr.blit(j.img, j.img.Bounds(), j.wf, j.flash)
			a.blitAt.Store(0)
			a.done <- err
		}
	}()
	// Hello first, while the rest gets going; restarted, the one before
	// said it already.
	if os.Getenv(restartedEnv) == "" {
		s := a.scr.info
		a.C = Canvas{image.NewGray(image.Rect(0, 0, s.ViewW, s.ViewH))}
		a.drawCard(a.cfg.Title, "starting…")
		a.flush()
	}
	os.Unsetenv(restartedEnv)
	start := -1
	if a.LightStart != nil {
		start = *a.LightStart
	}
	a.light = openFrontlight(start)
	a.devs = inputDevices()
	touches := make(chan TouchEv, 4096)
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
	keys := make(chan KeyMsg, 256)
	go watchKeyboards(keys)
	for _, d := range a.devs {
		if d.role() == "buttons" {
			go watchButtons(d, keys)
		}
	}
	a.WaitBlit()
	a.relayout()
	a.loop(touches, keys)

	a.prog.Exit()
	a.WaitBlit()
	a.dirty = nil
	if a.restart != "" {
		a.drawRestart()
	} else {
		a.drawBye()
	}
	a.flush()
	a.WaitBlit()
	a.scr.close()
	if a.light != nil {
		if !a.lightHideAt.IsZero() { // set by a key just now
			a.keepLight()
		}
		if a.restart == "" { // a new build sets the kept level: no blink of Nickel's between
			a.light.close()
		}
	}
	if a.restart != "" {
		a.exec()
		if a.light != nil {
			a.light.close()
		}
		return // it didn't: launch.sh gives Kobo back
	}
	log.Print("bye")
	if logFile != nil {
		logFile.Sync()
	}
}

// Restart quits, then runs exe in our place: the same process, so Nickel
// stays frozen and launch.sh goes on waiting for us.
func (a *App) Restart(exe string) {
	log.Printf("restart into %s", exe)
	a.restart, a.quit = exe, true
}

// Restarting says whether we're quitting to run a new build in our place.
func (a *App) Restarting() bool { return a.restart != "" }

// restartedEnv tells a new build it's taking over from the one before.
const restartedEnv = "HELLOBRO_RESTARTED"

func (a *App) exec() {
	log.Printf("restarting: exec %s", a.restart)
	if logFile != nil {
		logFile.Sync()
	}
	err := syscall.Exec(a.restart, []string{a.restart}, append(os.Environ(), restartedEnv+"=1"))
	log.Printf("restart: %v", err)
}

func openLog(name string) (*os.File, error) {
	return os.OpenFile(AppFile(name+".log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
}

// AppFile is a file next to the binary.
func AppFile(name string) string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), name)
	}
	return name
}

// Do runs f on the UI loop; it's for goroutines, not the loop itself.
func (a *App) Do(f func()) { a.call <- f }

// Screen is how the screen is set up now.
func (a *App) Screen() ScreenInfo { return a.scr.info }

// Calibrating says whether the touch calibration has the screen.
func (a *App) Calibrating() bool { return a.calib != nil }

// Blitting says whether an update is on its way to the panel.
func (a *App) Blitting() bool { return a.blitting }

// TabName is the current tab's.
func (a *App) TabName() string { return a.cfg.Tabs[a.Tab] }

// Tabs are the tabs shown, in their order (indices into Config.Tabs).
func (a *App) Tabs() []int {
	if a.order == nil {
		order := make([]int, len(a.cfg.Tabs))
		for i := range order {
			order[i] = i
		}
		return order
	}
	return a.order
}

// SetTabs shows the tabs in order, with hints in their corners (as
// many; nil: their numbers, for Ctrl+Alt+N). The tab up, if hidden,
// gives way to the first.
func (a *App) SetTabs(order []int, hints []string) {
	order = slices.DeleteFunc(slices.Clone(order), func(t int) bool { return t < 0 || t >= len(a.cfg.Tabs) })
	if len(order) == 0 {
		return
	}
	a.order, a.hints = order, hints
	if a.C.Gray == nil { // not up yet
		if !slices.Contains(order, a.Tab) {
			a.Tab = order[0]
		}
		return
	}
	a.L = a.layout()
	a.prog.Layout(a.L)
	if !slices.Contains(order, a.Tab) {
		a.Tab = order[0]
		a.prog.Enter(a.Tab)
	}
	if a.calib == nil {
		a.drawAll(false)
	}
}

// SetTabNames names the tabs anew, for a Program whose tabs come and go
// (hellobro's workspaces): all of them shown, in order, cur up, hints in
// their corners, and with Config.Bar info after each name: what's in it.
// A Program that does this takes the taps on them (TabTapper) and draws
// its page itself: this redraws the tab bar only.
func (a *App) SetTabNames(names, hints, info []string, cur int) {
	if len(names) == 0 {
		return
	}
	a.cfg.Tabs, a.order, a.hints, a.tabInfo = slices.Clone(names), nil, slices.Clone(hints), slices.Clone(info)
	a.Tab = max(0, min(cur, len(names)-1))
	if a.C.Gray == nil || a.calib != nil {
		return
	}
	full := a.L.Full
	a.L = a.layout()
	a.L.Full = full
	a.drawTabs()
}

// TabTapper is a Program that takes the taps on the tabs itself: i is
// the tab's place in the bar.
type TabTapper interface {
	TabTap(i int)
}

// SetFooter has the footer say tabs on the tabs' keys ("tabs: tap or
// Ctrl+Alt+1–4"), and sleep and quit for their keys ("-": there's none);
// "" for the defaults, which are the keys without Config.OwnKeys.
func (a *App) SetFooter(tabs, sleep, quit string) {
	a.footerTabs, a.footerSleep, a.footerQuit = tabs, sleep, quit
	if a.C.Gray != nil && a.calib == nil {
		a.drawFooter()
	}
}

func (a *App) loop(touches <-chan TouchEv, keys <-chan KeyMsg) {
	go a.watchdog()
	idle := time.NewTimer(IdleTimeout)
	tick := time.NewTicker(30 * time.Millisecond)
	clock := time.NewTicker(20 * time.Second)
	for !a.quit {
		a.stats.start()
		select {
		case ev := <-touches:
			a.stats.event("touch", time.Since(ev.At))
			if a.asleep {
				continue
			}
			if a.locked || a.eatTouch {
				a.lockTouch(ev)
				continue
			}
			a.lightBack()
			a.onTouch(ev)
			if a.ink {
				a.flush() // now, not at the next tick: ink follows the hand
			}
			idle.Reset(IdleTimeout)
			a.inputAt = time.Now()
		case m := <-keys:
			key := !m.at.IsZero() && m.Connected == "" && m.Gone == ""
			if m.at.IsZero() {
				a.stats.event("keyboard", 0) // a keyboard came or went
			} else {
				a.stats.event("key", time.Since(m.at))
			}
			locked := a.locked
			if key && !locked {
				a.lightBack()
			}
			a.onKey(m)
			if m.Connected == "" && m.Gone == "" && !a.locked { // locked, keys aren't use: in a bag they'd keep it awake
				if key && locked { // the one that unlocked
					a.lightBack()
				}
				idle.Reset(IdleTimeout)
				a.inputAt = time.Now()
			}
		case err := <-a.done:
			a.stats.event("blit done", 0)
			a.blitting = false
			if err != nil {
				log.Print(err)
			}
			a.flush() // keep the panel busy while there's something to show
		case f := <-a.call:
			a.stats.event("call", 0)
			f()
		case <-tick.C:
			a.stats.event("tick", 0)
			a.alive.Store(time.Now().UnixNano())
			if !a.noticeUntil.IsZero() && time.Now().After(a.noticeUntil) {
				a.notice, a.noticeUntil = "", time.Time{}
				a.drawStatus()
			}
			if !a.lightHideAt.IsZero() && time.Now().After(a.lightHideAt) {
				a.lightHideAt = time.Time{}
				a.hideGauge(&a.lightOverlay)
				a.keepLight()
			}
			if !a.asleep {
				a.prog.Tick()
				a.noticeTick()
				a.markTick()
				a.lightIdle()
			}
			a.flush()
		case <-clock.C:
			a.stats.event("clock", 0)
			a.drawStatus()
		case <-idle.C:
			if d := a.prog.IdleTimeout(); a.asleep || time.Since(a.inputAt) < d { // reading a remote screen takes longer
				idle.Reset(min(d-time.Since(a.inputAt), IdleTimeout)) // and ask again: it may be shorter by then
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
func (a *App) watchdog() {
	a.alive.Store(time.Now().UnixNano())
	for range time.Tick(2 * time.Second) {
		if a.sleeping.Load() { // the clock leaps over the sleep
			a.alive.Store(time.Now().UnixNano())
			continue
		}
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

// bail thaws Nickel itself (launch.sh may never get to) and exits.
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
func (a *App) relayout() {
	s := a.scr.info
	a.C = Canvas{image.NewGray(image.Rect(0, 0, s.ViewW, s.ViewH))}
	a.L = a.layout()
	a.prog.Layout(a.L)
	if !a.entered {
		a.entered = true
		a.prog.Enter(a.Tab)
	}
	a.dirty = nil
	a.gestures = map[int32]*Gesture{}

	a.tmap = touchMap{w: s.ViewW, h: s.ViewH, ox: s.OriginX, oy: s.OriginY}
	if a.touch != nil {
		a.tmap.rx, a.tmap.ry = a.touch.rx, a.touch.ry
	}
	// No range from the kernel: assume raw units are panel pixels.
	pw, ph := min(s.ScreenW, s.ScreenH), max(s.ScreenW, s.ScreenH)
	if s.Swap {
		pw, ph = ph, pw
	}
	if a.tmap.rx.max <= a.tmap.rx.min {
		a.tmap.rx = absRange{0, int32(pw - 1)}
	}
	if a.tmap.ry.max <= a.tmap.ry.min {
		a.tmap.ry = absRange{0, int32(ph - 1)}
	}
	if c, ok := a.cal[s.Rota]; ok {
		a.tmap.axes = c
		log.Printf("touch mapping for rota %d: %s (calibrated; FBInk default: %s)", s.Rota, c, defaultAxes(s))
	} else {
		a.tmap.axes = defaultAxes(s)
		log.Printf("touch mapping for rota %d: %s (FBInk default, not calibrated yet)", s.Rota, a.tmap.axes)
		if a.touch != nil {
			a.startCalib("")
			return
		}
	}
	a.drawAll(true)
}

// Rotate turns the screen a quarter, as the header's button does.
func (a *App) Rotate() { a.rotate() }

// Calibrate starts the touch calibration, as the header's button does.
func (a *App) Calibrate() { a.startCalib("") }

// Quit leaves, back to Nickel.
func (a *App) Quit() {
	log.Print("quit")
	a.quit = true
}

func (a *App) rotate() { a.SetRotation((a.scr.info.Rota + 1) % 4) }

// Rotation is the screen's, as FBInk counts: 0 and 2 portrait (2 upside
// down), 1 and 3 landscape.
func (a *App) Rotation() int { return a.scr.info.Rota }

// SetRotation turns the screen to r.
func (a *App) SetRotation(r int) {
	if r = (r%4 + 4) % 4; r == a.scr.info.Rota {
		return
	}
	a.WaitBlit()
	if err := a.scr.setRotation(r); err != nil {
		log.Printf("rotate: %v", err)
	}
	a.relayout()
}

// SwitchTab shows tab t, if it's shown.
func (a *App) SwitchTab(t int) {
	if t == a.Tab || !slices.Contains(a.Tabs(), t) || a.calib != nil {
		return
	}
	a.Tab = t
	a.prog.Enter(t)
	if a.Full {
		a.SetFull(false)
		return
	}
	a.drawTabs()
	a.DrawPage()
}

// SetFull gives the page the whole screen, or puts the app back around it.
func (a *App) SetFull(on bool) {
	if on == a.Full {
		return
	}
	log.Printf("fullscreen: %v", on)
	a.Full = on
	a.WaitBlit()
	a.relayout()
}

// --- touch ---

func (a *App) onTouch(ev TouchEv) {
	p := a.tmap.toView(ev.x, ev.y)
	if a.calib != nil {
		if ev.Kind == TouchUp {
			a.calibTap(ev)
		}
		return
	}

	switch ev.Kind {
	case TouchHover:
		a.prog.Touch(ev, p, nil)

	case TouchDown:
		g := &Gesture{Start: p, Last: p, At: ev.At, Pen: ev.Pen, Points: 1, MaxPressure: ev.Pressure}
		a.gestures[ev.slot] = g
		if a.touchMarks { // to see what the panel says, to the ms
			log.Printf("touch: down #%d at %d,%d (%s)", ev.slot, p.X, p.Y, ev.At.Format("05.000"))
		}
		a.addMark(p, false)
		a.prog.Touch(ev, p, g)
		g.lightZone = a.light != nil && !ev.Pen && p.X < EdgeWidth && g.Owner == 0
		if a.Full && !ev.Pen && !g.lightZone && g.Owner == 0 { // no fullscreen key without a keyboard: a way out by touch
			switch {
			case p.Y < unfullEdge:
				g.unfull = 1
			case p.Y >= a.C.Bounds().Dy()-unfullEdge:
				g.unfull = -1
			}
		}

	case TouchMove:
		g := a.gestures[ev.slot]
		if g == nil {
			return
		}
		g.Points++
		g.Moved = math.Max(g.Moved, math.Hypot(float64(p.X-g.Start.X), float64(p.Y-g.Start.Y)))
		g.MaxPressure = max(g.MaxPressure, ev.Pressure)
		if dy := g.Start.Y - p.Y; g.lightZone && !g.light && Abs(dy) > SwipeStart && Abs(dy) > 2*Abs(p.X-g.Start.X) {
			g.light, g.lightFrom = true, a.light.level
			log.Printf("light swipe from %d%%", g.lightFrom)
		}
		if dy := (p.Y - g.Start.Y) * g.unfull; g.unfull != 0 && !g.unfulled && dy > unfullMove && dy > 2*Abs(p.X-g.Start.X) {
			g.unfulled = true
			log.Printf("fullscreen: swiped in from the %s edge", map[int]string{1: "top", -1: "bottom"}[g.unfull])
			a.SetFull(false)
		}
		if g.unfulled {
			g.Last = p
			return
		}
		if g.light {
			// Half the screen height covers the whole range.
			a.setLight(g.lightFrom + (g.Start.Y-p.Y)*100/(a.C.Bounds().Dy()/2))
		} else {
			a.prog.Touch(ev, p, g)
		}
		g.Last = p

	case TouchUp:
		g := a.gestures[ev.slot]
		delete(a.gestures, ev.slot)
		if g == nil {
			return
		}
		if a.touchMarks {
			log.Printf("touch: up #%d at %d,%d (%s)", ev.slot, g.Last.X, g.Last.Y, ev.At.Format("05.000"))
		}
		if g.light {
			log.Printf("light swipe: %d%% → %d%%", g.lightFrom, a.light.level)
			a.hideGauge(&a.lightOverlay)
			a.keepLight()
			return
		}
		if g.unfulled {
			return
		}
		g.Dur = ev.At.Sub(g.At)
		g.Tap = g.Moved < tapSlop && g.Dur < tapTime
		if g.Moved >= tapSlop {
			a.addMark(g.Last, true)
		}
		if g.Tap && g.Owner == 0 && g.Ignored == "" {
			a.tapAt(g.Start)
		}
		tool := "finger"
		if g.Pen {
			tool = "pen"
		}
		log.Printf("%s %s", tool, g)
		a.prog.Lift(g)
	}
}

// Fingers are the fingers on the screen now (not the pen).
func (a *App) Fingers() []*Gesture {
	var fs []*Gesture
	for _, g := range a.gestures {
		if !g.Pen {
			fs = append(fs, g)
		}
	}
	return fs
}

// String describes g, once it's over.
func (g *Gesture) String() string {
	s := fmt.Sprintf("stroke  %d pts  %.2fs  %d,%d → %d,%d", g.Points, g.Dur.Seconds(), g.Start.X, g.Start.Y, g.Last.X, g.Last.Y)
	if g.Tap {
		s = fmt.Sprintf("tap  %d,%d", g.Start.X, g.Start.Y)
	}
	if g.Pen {
		s += fmt.Sprintf("  max pressure %d", g.MaxPressure)
	}
	if g.Ignored != "" {
		s += "  (" + g.Ignored + ")"
	}
	return s
}

func (a *App) tapAt(p image.Point) {
	if a.quit {
		return
	}
	for i, r := range a.L.tabs {
		if p.In(r) {
			if t, ok := a.prog.(TabTapper); ok {
				t.TabTap(i)
			} else {
				a.SwitchTab(a.Tabs()[i])
			}
			return
		}
	}
	switch {
	case Hit(p, a.L.btnExit):
		log.Print("exit tapped")
		a.quit = true
	case Hit(p, a.L.btnSleep):
		a.Sleep()
	case Hit(p, a.L.btnRotate):
		a.rotate()
	case Hit(p, a.L.btnCalib):
		a.startCalib("")
	default:
		a.prog.Tap(p)
	}
}

// PressureMax is the most pressure the pen reports.
func (a *App) PressureMax() int32 {
	if a.touch != nil && a.touch.rpress.max > 0 {
		return a.touch.rpress.max
	}
	return 1023
}

// --- calibration ---

func (a *App) startCalib(note string) {
	a.lightOverlay, a.gaugeOverlay = nil, nil
	a.calib = &calibState{note: note}
	a.gestures = map[int32]*Gesture{}
	a.dirty = nil
	a.drawCalib()
}

func (a *App) calibTap(ev TouchEv) {
	a.calib.raw = append(a.calib.raw, image.Pt(int(ev.x), int(ev.y)))
	if len(a.calib.raw) < len(calibrationTargets) {
		a.drawCalib()
		return
	}
	s := a.scr.info
	got, miss := calibrate(a.tmap, a.calib.raw)
	def := defaultAxes(s)
	log.Printf("calibration for rota %d: %s, mean miss %.0f px, raw taps %v (FBInk default: %s)", s.Rota, got, miss, a.calib.raw, def)
	if miss > 150 {
		a.startCalib(fmt.Sprintf("Missed by %.0f px on average, let's try again.", miss))
		return
	}
	a.cal[s.Rota] = got
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

func (a *App) onKey(m KeyMsg) {
	switch {
	case m.button && m.ungrabbed:
		a.noPower = true
		if a.calib == nil {
			a.drawFooter()
		}
		return
	case m.button:
		log.Printf("button %d value %d", m.Code, m.Value)
		if m.Code == keyPower && m.Value == 0 { // on release, so Nickel doesn't get a stray one
			a.powerPressed()
		}
		return
	case m.Connected != "" || m.Gone != "":
		if a.kbds == nil {
			a.kbds = map[string]int{}
		}
		if m.Connected != "" {
			log.Printf("keyboard connected: %s", m.Connected)
			a.kbds[m.Connected]++
			a.Keyboard = m.Connected
		} else {
			log.Printf("keyboard gone: %s", m.Gone)
			if a.kbds[m.Gone]--; a.kbds[m.Gone] <= 0 {
				delete(a.kbds, m.Gone)
			}
			a.devs = inputDevices()
			for k := range a.kbds { // another's still here (a typing device's went, say): no keyboard's gone
				a.Keyboard = k
				return
			}
			a.Keyboard = ""
			// What it held won't come up now.
			a.ctrl, a.alt = [2]bool{}, [2]bool{}
			a.SetFull(false) // the keys are the way back, so don't strand the user there
		}
		a.devs = inputDevices()
		a.prog.Key(m)
		return
	}
	switch m.Code {
	case KeyLeftCtrl, KeyRightCtrl:
		a.ctrl[b2i(m.Code == KeyRightCtrl)] = m.Value != 0
	case keyLeftAlt, keyRightAlt:
		a.alt[b2i(m.Code == keyRightAlt)] = m.Value != 0
	}
	// Asleep, the modifiers still go to the Program, so it knows what's
	// held: Ctrl+Alt+S's Ctrl and Alt come up once it's asleep, and
	// missed, they'd stay down after.
	if a.asleep {
		if IsModifier(m.Code) {
			a.prog.Key(m)
		}
		return
	}
	if a.locked { // the Program lets through what it will
		a.prog.Key(m)
		return
	}
	// Ctrl+Alt+N: tab N, whatever the page does with keys (a terminal
	// would send it on), fullscreen too.
	if n := int(m.Code) - Key1; !a.cfg.OwnKeys && (a.ctrl[0] || a.ctrl[1]) && (a.alt[0] || a.alt[1]) && n >= 0 && n < min(9, len(a.Tabs())) {
		if m.Value == 1 {
			a.SwitchTab(a.Tabs()[n])
		}
		return
	}
	if m.Code == keyS && !a.cfg.OwnKeys && (a.ctrl[0] || a.ctrl[1]) && (a.alt[0] || a.alt[1]) { // Ctrl+Alt+S: sleep
		if m.Value == 1 {
			a.Sleep()
		}
		return
	}
	if a.prog.Key(m) {
		return
	}
	if m.Code == KeyEsc && m.Value == 1 && !a.cfg.OwnKeys {
		log.Print("Esc pressed")
		a.quit = true
	}
}

// --- screen updates ---

// Area is r's, in pixels.
func Area(r image.Rectangle) int { return r.Dx() * r.Dy() }

// compact says whether one update covering a and b wastes little.
func compact(a, b image.Rectangle) bool { return Area(a.Union(b)) <= (Area(a)+Area(b))*3/2 }

// Mark queues r for the panel, merged into a pending area of the same kind
// when that doesn't make it much bigger: small updates stay small. ui
// areas get a grayscale waveform, others (black & white ink only) the
// fast one.
func (a *App) Mark(r image.Rectangle, ui bool) {
	if a.C.Gray != nil {
		r = r.Intersect(a.C.Bounds()) // a touch mark at the edge, say: a framebuffer written straight has nothing past it
	}
	if r.Empty() || a.locked { // the page is off the panel
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

// MarkFlash redraws r with a flash, which clears the ghosting that
// non-flashing updates leave behind.
func (a *App) MarkFlash(r image.Rectangle) {
	a.Mark(r, true)
	a.flashR = r
}

func (a *App) markAll(flash bool) {
	a.ring = image.Rectangle{} // a card, or a new layout: the Program puts it back if it's still wanted
	a.blinks, a.toast = nil, nil
	a.dirty = []dirtyRect{{a.C.Bounds(), true}}
	a.flash = a.flash || flash
}

// flush sends the oldest pending area, with those of its kind close
// enough to share an update, unless a blit is still in flight: updates
// pile up meanwhile, which coalesces them. Far-apart areas go one after
// the other, since the panel's time grows with the area: a typed row and
// the bottom of the screen mustn't become a refresh of all in between.
// If they pile up anyway, everything of a kind goes at once, so the
// screen can't fall further and further behind.
func (a *App) flush() {
	if a.blitting || a.asleep || len(a.dirty) == 0 {
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
	CopyGray(img, d.r, a.panel().Gray, d.r.Min)
	if !a.locked {
		a.stampNotices(img)
		if d.r.Overlaps(a.ring) {
			stampRing(img, a.ring)
		}
		stampMarks(img, a.marks)
	}
	flash := a.flash && d.r == a.C.Bounds()
	if !flash && d.ui && !a.flashR.Empty() && a.flashR.In(d.r) {
		flash, a.flashR = true, image.Rectangle{}
	}
	wf := "DU"
	switch {
	case flash:
		wf = "GC16"
		if d.r == a.C.Bounds() {
			a.flash = false
		}
	case d.ui:
		wf = "GL16"
	default:
		blackWhite(img)
		if a.ink {
			wf = "PEN"
		}
	}
	a.jobs <- blitJob{img, wf, flash}
	a.blitting = true
}

// Ink says ink is being written (or not now): the fast updates go in
// the panel's pen mode then, for the least latency. A grayscale update
// after (Mark's ui) ends it.
func (a *App) Ink(on bool) { a.ink = on }

// blackWhite rounds img's grays (text's smoothed edges) to black or
// white, for the fast waveform. DU can't show gray: sent one, it leaves
// the pixel faint, while the panel takes it as drawn, so the grayscale
// cleanup after, with the same gray, wouldn't touch it again. Rounded,
// the edges differ from what the cleanup sends, and it draws them.
func blackWhite(img *image.Gray) {
	for i, v := range img.Pix {
		if v < 128 {
			img.Pix[i] = 0
		} else {
			img.Pix[i] = 255
		}
	}
}

// WaitBlit waits for the update in flight, if any.
func (a *App) WaitBlit() {
	if a.blitting {
		if err := <-a.done; err != nil {
			log.Print(err)
		}
		a.blitting = false
	}
}

// Abs is |n|.
func Abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
