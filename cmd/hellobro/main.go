// hellobro is a terminal for the Kobo Elipsa, for a Bluetooth keyboard,
// and a tiling window manager after i3 (wm.go): workspaces of windows,
// side by side, tabbed, stacked or floating. A window is a terminal
// (SSH, Alpine in a jail, the Kobo's own sh; a launcher till it runs
// one: term.go) or one of the settings' (gui.go): Kobo's Wi-Fi and
// Bluetooth, sharing with a computer (SFTP, ssh, mDNS), hellobro's
// settings, the battery and what uses it.
//
// A swipe along the right edge opens the menu. The keys are i3's,
// with Mod, and the user's to change (commands.go); i3's commands come
// in on a socket too (ipc.go). The music is a Linux player's, in an
// Alpine session (mpris.go). The rest (the screen, touch, the frontlight
// on the left edge, quitting) is the kobo package's.
//
// Launch it through launch.sh, which keeps Nickel from drawing over us.
package main

import (
	"fmt"
	"image"
	"log"
	"os"
	"path/filepath"
	"time"

	"hellobro/ink/theme"
	"hellobro/kobo"
	"hellobro/radio"
	"hellobro/share"
)

// bro is the app: the kobo.App, the windows, the radios, and the Linux
// players' remote control.
type bro struct {
	*kobo.App
	wm      wmState
	ipc     ipcState
	tv      termView
	edge    *kobo.Gesture    // a contact along the right edge, which may become a menu swipe
	inkLive *kobo.Gesture    // the pen (or a finger) writing on an ink window's sheet: drawn as it comes
	pair    [2]*kobo.Gesture // two fingers together: may become a workspace swipe
	pinch   *pinch           // or a PDF zoomed (bookzoom.go)
	media   mediaState
	stat    statBar  // the terminal tabs' status bar
	picker  *picker  // the switcher, while it's up
	exUI    *exUI    // the ":" line, drawn in ink (excmd.go)
	asking  *askBox  // a question, while it's up
	info    *infoBox // facts to read, while they're up
	keys    keyMap
	keyset  *keySet // Settings' Keys: a key being asked for
	rs      radioState
	slp     sleepState
	set     settings
	mem     remembered // hellobro.json
	mnt     mountState // the hosts' files, mounted (mount.go)
	share   shareState
	relay   relayState
	upd     updateState
	net     netState // the header's network
	pw      powerState
	kbd     kbdState
	vk      vkbdState        // the on-screen keyboard (vkbd.go)
	bk      bookState        // the reader's fonts (bookwin.go)
	nt      noteState        // the notebooks' pen (notebook.go)
	lay     kobo.Layout      // the last layout, to lay out again
	th      *theme.Theme     // the ink windows' look (inkwin.go)
	shot    *shot            // a screenshot being taken (shot.go)
	trio    [3]*kobo.Gesture // three fingers together: may become the workspaces' swipe
	cal     calState         // the calendar's store and its syncing (calsync.go)
}

const (
	ownMenu   = 1   // a gesture that is a menu swipe
	ownTrio   = 22  // one of three fingers together: maybe the workspaces' swipe
	ownPair   = 4   // one of two fingers together: maybe a workspace swipe
	ownEdge   = 5   // one in from an edge: a workspace swipe
	menuSwipe = 100 // px along the right edge it takes
	pairSwipe = 120 // px across both fingers go, for a workspace
)

func main() {
	if filepath.Base(os.Args[0]) == "hellobro-msg" { // in a jail (ipc.go)
		msgMain(os.Args[1:])
		return
	}
	if filepath.Base(os.Args[0]) == "read-kitty" { // in a jail (readkitty.go)
		os.Exit(readMain(os.Args[1:]))
	}
	if len(os.Args) >= 2 && os.Args[1] == "--msg" {
		msgMain(os.Args[2:])
		return
	}
	if len(os.Args) == 2 && os.Args[1] == "--version" { // hellobro-update asks a new build this, screen untouched
		fmt.Println("hellobro", version)
		return
	}
	if len(os.Args) == 4 && os.Args[1] == "--mount" { // a host's files (sftpfs.go), screen untouched
		mountMain(os.Args[2], os.Args[3])
		return
	}
	if len(os.Args) == 3 && os.Args[1] == "--alpine-init" { // a jail's first process (alpineinit.go), screen untouched
		alpineInit(os.Args[2])
		return
	}
	updateTrial() // before anything that might hang
	a := &bro{media: mediaState{kick: make(chan struct{}, 1)}}
	a.vk = vkbdState{held: map[*kobo.Gesture]vkHeld{}, latched: map[uint16]bool{}, since: time.Now()}
	a.set = loadSettings()
	a.applySizes() // before anything is laid out
	a.applyTextSize()
	a.vk.on = a.setting("vkbd") == "on"
	kobo.SetTyping(a.langs()[0])
	a.mem = loadRemembered(kobo.AppFile(""))
	a.tv.hosts = a.mem.hosts()
	radio.StateDir = kobo.AppFile("")
	a.App = kobo.New(kobo.Config{Name: "hellobro", Title: "hello bro", About: []string{version, "github.com/shabunin/hellobro"}, Tabs: []string{"1"}, OwnKeys: true, Bar: true, Plus: true}, a)
	a.loadBar() // the App's, before it's up
	a.App.LightAt = a.lightAt
	a.App.TopSwipe = a.topSwipe
	profileOn.Store(a.setting("profile") == "on")
	alpineBus.Store(a.setting("alpine-bus") == "on")
	a.applyLightIdle()
	a.applyLightStart()
	log.Printf("hellobro %s", version)
	deviceLabel = a.deviceName()
	sshClientVersion = "SSH-2.0-hellobro " + share.PeerName("SSH-2.0-hellobro "+a.deviceName()) // who's asking to type, for another hellobro
	a.keys = loadKeys()
	a.applyMod()
	a.SetTouchMarks(a.setting("touch-marks") == "on")
	a.SetAutoRotate(a.setting("auto-rotate") == "on")
	a.wmInit()
	a.ipcStart()
	a.termInit()
	a.shareApply()
	go a.watchPlayers() // the buses may be slow: never in the way of starting
	a.Run()
}

func (a *bro) Layout(L kobo.Layout) {
	a.lay = L
	// The windows' area: the page, less the on-screen keyboard, the gaps
	// at its edges and the status bar under it. Full screen, the window
	// has it all, but the keyboard.
	a.wm.area = a.vkPlace(L.Page)
	if !L.Full {
		a.wm.area = a.wm.area.Inset(wmGap / 2)
	}
	if a.tv.fontPx != 0 {
		a.termGeometry() // the cells' size
	}
	if a.wm.tree == nil {
		return
	}
	if !L.Full && a.wm.tree.Fullscreen() != nil { // a swipe in from the edge left full screen
		a.wm.tree.SetFullscreen(a.wm.tree.Fullscreen(), false)
	}
	if tall := L.Page.Dy() > L.Page.Dx(); !a.wm.oriented || tall != a.wm.tall { // first, or turned
		a.wm.oriented, a.wm.tall = true, tall
		a.wm.tree.SetRect(a.wm.area)
		a.wm.tree.Orient()
	}
	a.wm.laid = ""
	a.wmPlace()
	a.wmFocusMoved()
	a.wmStarts()
}

func (a *bro) Enter(tab int) {}

func (a *bro) Tap(p image.Point) {
	switch {
	case a.asking != nil:
		a.askTap(p)
	case a.info != nil:
		a.infoTap(p)
	case a.keyset != nil:
		a.keySetTap(p)
	case a.picker != nil:
		a.pickerTap(p)
	case a.topTap(p):
	case !a.lay.Full && !a.BarOff && a.nowPlaying() != "" && p.In(a.StatusNote()): // what's playing, in the bar
		a.mediaKey(kobo.KeyPlayPause)
	case !a.lay.Full && p.Y < a.lay.Page.Min.Y: // the bar, not a workspace's name in it
		a.openWorkspaces()
	default:
		a.wmTap(p)
	}
}

func (a *bro) Tick() {
	a.sleepWhenIdle()
	a.vkTick()
	a.keysTick()
	a.netTick()
	a.termTick()
	a.statTick()
	a.radioTick()
	a.shareTick()
	a.powerTick()
	a.updateTick()
	a.trialTick()
	a.ipcTick()
	a.noteTick()
	a.calTick()
	a.holdTick()
	a.inkTick()
	a.wmSettle(false)
	a.wmKeep(false)
	a.updateRing()
}

func (a *bro) Exit() {
	a.noteExit()
	a.bookExit()
	a.wmKeep(true)
	a.ipcExit()
	a.termCloseAll()
	a.radioExit()
	a.shareExit()
	a.unmountAll(false)
	mountsExit()
	alpineExit()
}

// IdleTimeout is never: sleeping when idle (sleepWhenIdle) takes the
// auto-exit's place, and keeps the windows.
func (a *bro) IdleTimeout() time.Duration { return noAutoExit }

const noAutoExit = 100 * 365 * 24 * time.Hour

func (a *bro) Key(m kobo.KeyMsg) bool {
	if !a.wm.moving.IsZero() && m.Value == 1 && m.Code != 0 && !kobo.IsModifier(m.Code) {
		if c := a.keyCommand(m.Code); c == nil || !wmMove(c.id) { // anything but another move: in place now
			a.wmSettle(true)
		}
	}
	switch {
	case m.Connected != "" || m.Gone != "":
		if m.Gone != "" { // what it held won't come up now
			a.tv.mods = termMods{caps: a.tv.mods.caps}
			a.SetLockSwipes(a.lockSwipes()) // locked, touch's way out
			a.kbdGone()
		} else {
			if a.keyLabel("lock") != "" {
				a.SetLockSwipes(nil) // its keys unlock again
			}
			if a.vk.said > 0 { // shown by hand: a keyboard's here now
				a.vk.said = 0
			}
			a.kbdBack()
		}
		return true
	case kobo.IsMediaKey(m.Code): // the music's, locked too
		if m.Value == 1 || m.Value == 2 && (m.Code == kobo.KeyVolumeUp || m.Code == kobo.KeyVolumeDown) {
			log.Printf("media key %d from %s", m.Code, m.Dev)
			a.mediaKey(m.Code)
		}
		return true
	case a.Locked():
		a.lockedKey(m.Code, m.Value)
		return true
	case a.Calibrating():
		return false
	case a.shot != nil && m.Code == kobo.KeyEsc:
		if m.Value == 1 {
			a.shotCancel()
		}
		return true
	case a.asking != nil:
		a.askKey(m.Code, m.Value)
		return true
	case a.info != nil:
		a.infoKey(m.Code, m.Value)
		return true
	case a.keyset != nil:
		a.keySetKey(m.Code, m.Value)
		return true
	case a.picker != nil:
		a.pickerKey(m.Code, m.Value)
		return true
	case a.termMod(m.Code, m.Value): // kept track of always, for the terminals
		if t := a.term(); t != nil && a.focusedWin() == window(t) && a.wm.mode == "" {
			t.kittySend(m.Code, m.Value, a.tv.mods) // a program that wants every key has them too
		}
		return true
	case a.wm.mode == "" && a.grabbed(m.Code, m.Value): // every key the window's, but grab's own; a mode's keys are its own
		return a.wmKey(m.Code, m.Value)
	case a.runKey(m.Code, m.Value): // ours, never the remote's
		return true
	case a.wm.mode != "": // a mode has its keys, and only those
		return true
	}
	return a.wmKey(m.Code, m.Value)
}

func (a *bro) Touch(ev kobo.TouchEv, p image.Point, g *kobo.Gesture) {
	if ev.Kind == kobo.TouchDown {
		a.wmSettle(true) // the windows where they are now, before the touch finds one
	}
	switch {
	case g == nil: // the pen in the air
		if ev.Kind == kobo.TouchHover {
			a.nt.hoverAt = time.Now()
		}
	case ev.Kind == kobo.TouchDown:
		if a.shotDown(p, g) {
			return
		}
		if a.vkTouch(p, g) {
			return
		}
		if !ev.Pen && (a.trioUp(g) || a.pairUp(g)) {
			return
		}
		a.holdStart(p, g)
		if !ev.Pen && !a.overlay() && (!a.Full && p.X < kobo.EdgeWidth || p.X >= a.C.Bounds().Dx()-kobo.EdgeWidth) { // the right edge's menu, full screen too
			a.edge = g
		}
		switch {
		case a.overDown(ev, p, g):
		case a.overlay():
		case a.edge == g && p.X >= kobo.EdgeWidth: // the right edge is ours
		case a.fullTop(ev, p): // the top's swipes', full screen; a tap there is the bar's (topTap)
		case a.wmTouch(p, g):
		case a.inkDown(ev, p, g):
		case a.noteTouch(ev, p, g):
		case a.cutStart(p, g):
		default:
			a.selStart(p, g)
		}
	case a.shotMove(p, g):
	case g.Owner == ownTrio:
		a.trioMove()
	case a.inkMove(ev, p, g):
	case a.noteMove(ev, p, g):
	case a.vkMove(p, g):
	case a.edge == g && a.edgeSwipe(p, g):
	case a.tv.sel != nil && a.tv.sel.g == g:
		a.selMove(p, g)
	case a.bk.cut != nil && a.bk.cut.g == g:
		a.cutMove(p, g)
	case g.Owner == ownPair:
		a.pairMove()
	}
}

// fullTop says a finger coming down at p is the top's, full screen over
// a notebook: its swipes (one finger: the bar, topSwipe; two: out of
// full screen) go before the bar's buttons there, which have its tap.
func (a *bro) fullTop(ev kobo.TouchEv, p image.Point) bool {
	if !a.Full || ev.Pen || p.Y >= kobo.TopEdge || a.overlay() {
		return false
	}
	_, ok := a.windowAt(p).(*noteWin)
	return ok
}

// topTap hands a tap there to what's there in ink (the bar's buttons, the
// settings' header), as if it had been its all along.
func (a *bro) topTap(p image.Point) bool {
	if !a.fullTop(kobo.TouchEv{}, p) {
		return false
	}
	w := a.windowAt(p).(*noteWin)
	for _, b := range w.inkBodies(a) {
		g := &kobo.Gesture{Start: p, Last: p, Tap: true}
		if b.inkPane().Down(kobo.TouchEv{Kind: kobo.TouchDown}, p, g, ownPane) {
			a.inkTick()
			b.inkPane().Lift(g)
			a.inkTick()
			break
		}
	}
	return true
}

// trioUp makes g and the two fingers already down three together, if
// they all came down in the screen's top third and aren't writing: three
// fingers swiped down from there bring the workspaces up, anywhere.
func (a *bro) trioUp(g *kobo.Gesture) bool {
	fs := a.Fingers()
	top := a.C.Bounds().Min.Y + a.C.Bounds().Dy()/3
	if len(fs) != 3 || a.Locked() || a.picker != nil {
		return false
	}
	for _, f := range fs {
		if f.Start.Y >= top || f.Owner == ownInk || f.Owner == ownNoteSel || f.Owner == ownShot {
			return false
		}
	}
	for i, f := range fs {
		if f.Owner == ownPane {
			a.inkCancel(f)
		}
		if a.tv.sel != nil && a.tv.sel.g == f {
			a.tv.sel = nil
		}
		f.Owner = ownTrio
		a.trio[i] = f
	}
	a.pair, a.edge = [2]*kobo.Gesture{}, nil
	return true
}

// trioMove brings the workspaces up once the three have gone down far
// enough together.
func (a *bro) trioMove() {
	for _, f := range a.trio {
		if f == nil { // done, or one lifted
			return
		}
		if d := f.Last.Sub(f.Start); d.Y < pairSwipe || d.Y < 2*kobo.Abs(d.X) {
			return
		}
	}
	a.trio = [3]*kobo.Gesture{} // once: they stay three till they lift
	log.Print("three-finger swipe: the workspaces")
	a.openWorkspaces()
}

// topSwipe takes a finger swiped down from the top, full screen, over a
// book or a notebook: its bar shown, or hidden (for all of its kind, as
// the style panel's or the settings' row). Two fingers leave full screen
// (wsSwipe); elsewhere one does too.
func (a *bro) topSwipe(p image.Point) bool {
	if a.overlay() {
		return false
	}
	switch w := a.windowAt(p).(type) {
	case *bookWin:
		w.flipBar(a)
		log.Printf("book: swiped down from the top: the bar %s full screen", fullBarWord(a.fullBar(w.barKind())))
		return true
	case *noteWin:
		if w.overview || w.settings { // its buttons are all the overview has; the settings, a page of their own
			return false
		}
		a.flipFullBar("note")
		w.place(a, w.p)
		w.draw(a)
		log.Printf("notebook: swiped down from the top: the bar %s full screen", fullBarWord(a.fullBar("note")))
		return true
	}
	return false
}

// lightAt says if a finger down at p, by the left edge, may set the
// light by swiping: anywhere, full screen too, but in a notebook only if
// its settings say so (a finger by the edge may be writing).
func (a *bro) lightAt(p image.Point) bool {
	if _, ok := a.windowAt(p).(*noteWin); ok {
		return a.setting("note-light") != "off"
	}
	return true
}

// edgeSwipe takes a swipe from an edge, once it's gone far enough: up
// or down along the right edge, the menu (up or down the left edge is
// the light's: kobo's); if the settings say so, in from the right the
// next workspace, in from the left the one before (as pages turn).
func (a *bro) edgeSwipe(p image.Point, g *kobo.Gesture) bool {
	if g.Owner != 0 {
		return false
	}
	dx, dy := p.X-g.Start.X, p.Y-g.Start.Y
	right := g.Start.X >= kobo.EdgeWidth
	switch {
	case right && kobo.Abs(dy) > menuSwipe && kobo.Abs(dy) > 2*kobo.Abs(dx):
		g.Owner, a.edge = ownMenu, nil
		log.Print("menu swipe")
		a.menu()
	case kobo.Abs(dx) > menuSwipe && kobo.Abs(dx) > 2*kobo.Abs(dy) && (dx < 0) == right && a.setting("edge-swipe") == "edge": // inward
		g.Owner, a.edge = ownEdge, nil
		if a.tv.sel != nil && a.tv.sel.g == g {
			a.tv.sel = nil
		}
		to := "next"
		if !right {
			to = "prev"
		}
		log.Printf("edge swipe: workspace %s", to)
		a.wmRun("workspace " + to)
	default:
		return false
	}
	return true
}

// pairUp makes g and the finger already down a pair, if that one isn't
// anything yet: two fingers are a workspace swipe, never a tap, a scroll
// or a selection.
func (a *bro) pairUp(g *kobo.Gesture) bool {
	fs := a.Fingers()
	if len(fs) != 2 || a.overlay() {
		return false
	}
	first := fs[0]
	if first == g {
		first = fs[1]
	}
	// A notebook's finger on its page (or its pages, small) does nothing
	// till it lifts: a second finger may make it half of a pair.
	if first.Pen {
		return false
	}
	// A PDF's finger that has begun a cut, or moved the page, a moment
	// before the second came down (the select tool's draws at once) lets go.
	if first.Owner != 0 && first.Owner != ownLeaf && first.Owner != ownNoteOv && !a.cutLetGo(first) {
		return false
	}
	if a.tv.sel != nil && a.tv.sel.g == first {
		a.tv.sel = nil
	}
	if a.edge == first {
		a.edge = nil
	}
	first.Owner, g.Owner = ownPair, ownPair
	a.pair = [2]*kobo.Gesture{first, g}
	return true
}

// pairSwipe is the next workspace (both fingers to the left) or the one
// before (to the right), once both have gone far enough across.
func (a *bro) pairSwipe() {
	f, g := a.pair[0], a.pair[1]
	if f == nil || g == nil {
		return
	}
	if a.wsSwipe(f, g) || a.vkSwipe(f, g) {
		a.pair = [2]*kobo.Gesture{}
		return
	}
	dir := pairDir(f) + pairDir(g) // full screen too: two fingers aren't the page's
	if dir != -2 && dir != 2 {
		return
	}
	a.pair = [2]*kobo.Gesture{} // once: the fingers stay a pair till they lift
	to := "next"
	if dir > 0 {
		to = "prev"
	}
	log.Printf("two-finger swipe: workspace %s", to)
	a.wmRun("workspace " + to)
}

// pairDir is which way a finger went across: -1 left, 1 right, 0 not
// far enough, or more up or down.
func pairDir(g *kobo.Gesture) int {
	d := g.Last.Sub(g.Start)
	switch {
	case kobo.Abs(d.X) < pairSwipe || kobo.Abs(d.X) < 2*kobo.Abs(d.Y):
		return 0
	case d.X < 0:
		return -1
	}
	return 1
}

func (a *bro) Lift(g *kobo.Gesture) {
	if a.bk.hold == g {
		a.bk.hold = nil
	}
	if a.shotLift(g) || a.inkLift(g) || a.vkLift(g) || a.selLift(g) || a.cutLift(g) || a.wmDragLift(g) || a.noteLift(g) {
		return
	}
	if a.edge == g {
		a.edge = nil
	}
	if g.Owner == ownTrio {
		a.trio = [3]*kobo.Gesture{}
		return
	}
	if a.pair[0] == g || a.pair[1] == g {
		if a.pinch != nil {
			a.pinchEnd()
		}
		if a.nt.pinch != nil {
			a.notePinchEnd()
		}
		a.pair = [2]*kobo.Gesture{}
	}
	if g.Owner == ownMenu || g.Owner == ownPair || g.Owner == ownEdge || g.Owner == ownKey || g.Owner == ownHold || g.Owner == ownTop {
		return
	}
	if !g.Tap && !g.Pen && !a.overlay() {
		switch w := a.windowAt(g.Start).(type) {
		case *bookWin:
			w.swipe(a, g.Start, g.Last)
			return
		}
		a.termSwipe(g.Start, g.Last)
	}
}
