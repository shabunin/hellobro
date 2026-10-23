// hellobro is a terminal for the Kobo Elipsa, for a Bluetooth keyboard,
// and a tiling window manager after i3 (wm.go): workspaces of windows,
// side by side, tabbed, stacked or floating. A window is a terminal
// (SSH, Alpine in a jail, the Kobo's own sh; a launcher till it runs
// one: term.go) or one of the settings' (gui.go): Kobo's Wi-Fi and
// Bluetooth, sharing with a computer (SFTP, ssh, mDNS), hellobro's
// settings, the battery and what uses it.
//
// A swipe along the right edge sets the font size. The keys are i3's,
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
)

const termIdleTimeout = 20 * time.Minute // with a session up: reading a remote screen takes longer
const relayIdle = 24 * time.Hour         // with the relay on: as good as never

// bro is the app: the kobo.App, the windows, the radios, and the Linux
// players' remote control.
type bro struct {
	*kobo.App
	wm     wmState
	ipc    ipcState
	tf     termFonts
	tv     termView
	font   *fontSwipe // a contact along the right edge, which may become a font swipe
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
	doze   dozeState
	share  shareState
	relay  relayState
	upd    updateState
	net    netState // the header's network
	pv     powerView
	pw     powerState
	lay    kobo.Layout // the last layout, to lay out again
}

// fontSwipe is a contact that started on the right edge.
type fontSwipe struct {
	g        *kobo.Gesture
	from, to int // font sizes: when it started, and when the finger lifts
}

const ownFont = 1 // a gesture that is a font swipe

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
	updateTrial() // before anything that might hang
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
	a.keys = loadKeys()
	a.applyFooter()
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
	if a.Dozing() {
		a.dozeTick()
		return
	}
	a.dozeWhenIdle()
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

func (a *bro) IdleTimeout() time.Duration {
	if a.Dozing() || a.musicPlaying() { // dozeTick sleeps when the music's over
		return playingIdle
	}
	if a.relayOn() { // a computer may come in from afar any time
		return relayIdle
	}
	if a.termLive() || a.shareBusy() { // a computer sending music takes a while
		return termIdleTimeout
	}
	return kobo.IdleTimeout
}

func (a *bro) Key(m kobo.KeyMsg) bool {
	switch {
	case m.Connected != "" || m.Gone != "":
		if m.Gone != "" { // what it held won't come up now
			a.tv.mods = termMods{caps: a.tv.mods.caps}
		}
		return true
	case kobo.IsMediaKey(m.Code): // the music's, dozing too
		if m.Value == 1 || m.Value == 2 && (m.Code == kobo.KeyVolumeUp || m.Code == kobo.KeyVolumeDown) {
			log.Printf("media key %d from %s", m.Code, m.Dev)
			a.mediaKey(m.Code)
		}
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
		return true
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
	switch {
	case g == nil:
	case ev.Kind == kobo.TouchDown:
		a.focusTouched(p)
		switch {
		case a.overlay():
		case !ev.Pen && p.X >= a.C.Bounds().Dx()-kobo.EdgeWidth:
			a.font = &fontSwipe{g: g}
		case a.wmTouch(p, g):
		default:
			a.selStart(p, g)
		}
	case a.tv.sel != nil && a.tv.sel.g == g:
		a.selMove(p, g)
	case a.font != nil && a.font.g == g:
		dy := g.Start.Y - p.Y
		if g.Owner == 0 && kobo.Abs(dy) > kobo.SwipeStart && kobo.Abs(dy) > 2*kobo.Abs(p.X-g.Start.X) {
			g.Owner, a.font.from = ownFont, a.tv.fontPx
			log.Printf("font swipe from %d px", a.font.from)
		}
		if g.Owner == ownFont {
			a.termFontSwipe(a.font, dy)
		}
	}
}

func (a *bro) Lift(g *kobo.Gesture) {
	if a.selLift(g) || a.wmDragLift(g) {
		return
	}
	if f := a.font; f != nil && f.g == g {
		a.font = nil
		if g.Owner == ownFont {
			a.HideGauge()
			a.setTermFont(f.to)
			return
		}
	}
	if !g.Tap && !g.Pen && !a.overlay() {
		a.termSwipe(g.Start, g.Last)
	}
}
