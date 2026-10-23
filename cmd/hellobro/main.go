// hellobro is a terminal for the Kobo Elipsa, for a Bluetooth keyboard,
// and a tiling window manager after i3 (wm.go): workspaces of windows,
// side by side, tabbed, stacked or floating. A window is a terminal
// (SSH, Alpine in a jail, the Kobo's own sh; a launcher till it runs
// one: term.go) or one of the settings' (gui.go): Kobo's Wi-Fi and
// Bluetooth, sharing with a computer (SFTP, ssh, mDNS), hellobro's
// settings, the battery and what uses it.
//
// A swipe along the right edge opens the menu. The keys are i3's,
// with Meta, and the user's to change (commands.go); i3's commands come
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

	"hellobro/kobo"
	"hellobro/radio"
	"hellobro/share"
)

// bro is the app: the kobo.App, the windows, the radios, and the Linux
// players' remote control.
type bro struct {
	*kobo.App
	wm     wmState
	ipc    ipcState
	tf     termFonts
	tv     termView
	edge   *kobo.Gesture    // a contact along the right edge, which may become a menu swipe
	pair   [2]*kobo.Gesture // two fingers together: may become a workspace swipe
	media  mediaState
	stat   statBar  // the terminal tabs' status bar
	picker *picker  // the switcher, while it's up
	asking *askBox  // a question, while it's up
	info   *infoBox // facts to read, while they're up
	keys   keyMap
	keyset *keySet // Settings' Keys: a key being asked for
	rv     [2]radioPanelView
	rs     radioState
	slp    sleepState
	sv     settingsView
	shp    sharePanel // the Settings tab's Sharing
	fs     focusState // the keys' focus: focus.go
	set    settings
	mem    remembered // hellobro.json
	mnt    mountState // the hosts' files, mounted (mount.go)
	share  shareState
	relay  relayState
	upd    updateState
	net    netState // the header's network
	pv     powerView
	pw     powerState
	kbd    kbdState
	lay    kobo.Layout // the last layout, to lay out again
}

const (
	ownMenu   = 1   // a gesture that is a menu swipe
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
	updateTrial()                                  // before anything that might hang
	if kobo.EnvOr("HELLOBRO_TOUCH", "") == "big" { // a small sharp screen, fingers on it
		kobo.Big()
		pickRowH, statH = 92, 56
	}
	a := &bro{media: mediaState{kick: make(chan struct{}, 1)}}
	a.loadStatBar()
	a.set = loadSettings()
	a.mem = loadRemembered(kobo.AppFile(""))
	a.tv.hosts = a.mem.hosts()
	radio.StateDir = kobo.AppFile("")
	a.App = kobo.New(kobo.Config{Name: "hellobro", Title: "hello bro", About: []string{version, "github.com/shabunin/hellobro"}, Tabs: []string{"1"}, OwnKeys: true, Bar: true}, a)
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
	a.tf.setSize(termFontDefault)
	a.wmInit()
	a.ipcStart()
	a.termInit()
	a.shareApply()
	go a.watchPlayers() // the buses may be slow: never in the way of starting
	a.Run()
}

func (a *bro) Layout(L kobo.Layout) {
	a.lay = L
	// The windows' area: the page, less the gaps at its edges and the
	// status bar under it. Full screen, the window has it all.
	a.wm.area = L.Page.Inset(wmGap / 2)
	if L.Full {
		a.wm.area = L.Page
	}
	a.layoutStat()
	if a.tv.cw != 0 {
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
	case p.In(a.stat.r):
		a.mediaKey(kobo.KeyPlayPause)
	default:
		a.wmTap(p)
	}
}

func (a *bro) Tick() {
	a.sleepWhenIdle()
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
	a.wmSettle(false)
	a.wmKeep(false)
	a.updateRing()
}

func (a *bro) Exit() {
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
	case a.focusKeyed(m.Code, m.Value): // Tab, and Enter on a button
		return true
	}
	return a.wmKey(m.Code, m.Value)
}

func (a *bro) Touch(ev kobo.TouchEv, p image.Point, g *kobo.Gesture) {
	if ev.Kind == kobo.TouchDown {
		a.wmSettle(true) // the windows where they are now, before the touch finds one
	}
	switch {
	case g == nil:
	case ev.Kind == kobo.TouchDown:
		if !ev.Pen && a.pairUp(g) {
			return
		}
		a.focusTouched(p)
		if !ev.Pen && !a.overlay() && (p.X < kobo.EdgeWidth || p.X >= a.C.Bounds().Dx()-kobo.EdgeWidth) {
			a.edge = g
		}
		switch {
		case a.overlay():
		case a.edge == g && p.X >= kobo.EdgeWidth: // the right edge is ours
		case a.wmTouch(p, g):
		default:
			a.selStart(p, g)
		}
	case a.edge == g && a.edgeSwipe(p, g):
	case a.tv.sel != nil && a.tv.sel.g == g:
		a.selMove(p, g)
	case g.Owner == ownPair:
		a.pairSwipe()
	}
}

// edgeSwipe takes a swipe from an edge, once it's gone far enough: in
// from the right, the next workspace, in from the left the one before
// (as pages turn); up or down along the right edge, the menu. (Up or
// down the left edge is the light's: kobo's.)
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
	case kobo.Abs(dx) > menuSwipe && kobo.Abs(dx) > 2*kobo.Abs(dy) && (dx < 0) == right: // inward
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
	if first.Owner != 0 || first.Pen {
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
	dir := pairDir(f) + pairDir(g)
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
	if a.selLift(g) || a.wmDragLift(g) {
		return
	}
	if a.edge == g {
		a.edge = nil
	}
	if a.pair[0] == g || a.pair[1] == g {
		a.pair = [2]*kobo.Gesture{}
	}
	if g.Owner == ownMenu || g.Owner == ownPair || g.Owner == ownEdge {
		return
	}
	if !g.Tap && !g.Pen && !a.overlay() {
		a.termSwipe(g.Start, g.Last)
	}
}
