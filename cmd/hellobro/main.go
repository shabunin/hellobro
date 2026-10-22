// hellobro is a terminal for the Kobo Elipsa, for a Bluetooth keyboard.
//
// Tabs:
//   - SSH: terminals (xterm emulation, PTY) over SSH, several at once;
//   - Shell: terminals with a shell on the Kobo itself;
//   - Settings: Kobo's Wi-Fi and Bluetooth, and hellobro's sleep, sharing
//     and status bar;
//   - Files: sharing with a computer (SFTP, ssh, mDNS), and the Kobo's
//     storage.
//
// A swipe along the right edge sets the font size, Ctrl+Shift+F gives
// the session in focus the whole screen. Which tabs show, and in what order, is up
// to the user (tabs.go), and so are the keys (commands.go). The music is
// a Linux player's, in an Alpine session (mpris.go). The rest
// (the screen, touch, the frontlight on the left edge, quitting) is the
// kobo package's.
//
// Launch it through launch.sh, which keeps Nickel from drawing over us.
package main

import (
	"fmt"
	"image"
	"log"
	"os"
	"time"

	"hellobro/kobo"
	"hellobro/radio"
)

const (
	tabSSH = iota
	tabShell
	tabRadio
	tabFiles
	tabPower
)

var tabNames = []string{"SSH", "Shell", "Settings", "Files", "Power"}

func isTermTab(t int) bool { return t == tabSSH || t == tabShell }

const termIdleTimeout = 20 * time.Minute // with a session up: reading a remote screen takes longer
const relayIdle = 24 * time.Hour         // with the relay on: as good as never

// bro is the app: the kobo.App, the terminals, the radios, and the
// Linux players' remote control.
type bro struct {
	*kobo.App
	tf     termFonts
	termP  kobo.Panel
	tv     termView
	font   *fontSwipe // a contact along the right edge, which may become a font swipe
	media  mediaState
	stat   statBar     // the terminal tabs' status bar
	picker *picker     // the switcher, while it's up
	asking *askBox     // a question, while it's up
	info   *infoBox    // facts to read, while they're up
	tabsEd *tabsEditor // the Tabs dialog, while it's up
	tabs   tabsConf
	keys   keyMap
	keyset *keySet // Settings' Keys: a key being asked for
	rv     [2]radioPanelView
	rs     radioState
	slp    sleepState
	sv     settingsView
	fs     focusState // the keys' focus: focus.go
	set    settings
	mem    remembered // hellobro.json
	mnt    mountState // the hosts' files, mounted (mount.go)
	doze   dozeState
	share  shareState
	relay  relayState
	fv     filesView
	upd    updateState
	folds  map[string]bool // the panels folded, by key (fold.go)
	net    netState        // the header's network
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
	a.tabs = loadTabs()
	a.folds = loadFolds()
	a.App = kobo.New(kobo.Config{Name: "hellobro", Title: "hello bro", Tabs: tabNames, OwnKeys: true}, a)
	profileOn.Store(a.setting("profile") == "on")
	alpineBus.Store(a.setting("alpine-bus") == "on")
	a.applyLightIdle()
	a.applyLightStart()
	log.Printf("hellobro %s", version)
	a.keys = loadKeys()
	a.applyTabs()
	a.tf.setSize(termFontDefault)
	a.shareApply()
	go a.watchPlayers() // the buses may be slow: never in the way of starting
	a.Run()
}

func (a *bro) Layout(L kobo.Layout) {
	a.lay = L
	// The terminals' area: the sessions' panels go in it, the status bar
	// under them. Fullscreen, no frames: the cells get it all.
	if L.Full {
		a.termP = kobo.Panel{Name: "Terminal", Outer: L.Page, Body: L.Page}
	} else {
		a.termP = kobo.Panel{Name: "Terminal", Outer: L.Body, Body: L.Body}
	}
	a.layoutStat()
	if a.tv.cw != 0 {
		a.termGeometry() // the terminal follows the new size
	}
	a.layoutRadio(L)
	a.layoutFiles(L)
	a.layoutPower(L)
}

func (a *bro) Draw() {
	switch a.Tab {
	case tabRadio:
		a.rv[panelWiFi].shown, a.rv[panelBT].shown, a.sv.shown = "", "", ""
		a.drawRadio()
	case tabFiles:
		a.drawFiles()
	case tabPower:
		a.pv.shownBat, a.pv.shownUse = "", ""
		a.drawPower()
	default:
		a.drawTerm()
	}
}

func (a *bro) Enter(tab int) {
	switch tab {
	case tabRadio:
		a.enterRadio()
	case tabFiles:
		a.enterFiles()
	case tabPower:
		a.enterPower()
	default:
		a.enterTerm()
	}
}

func (a *bro) Tap(p image.Point) {
	if a.asking != nil {
		a.askTap(p)
		return
	}
	if a.info != nil {
		a.infoTap(p)
		return
	}
	if a.tabsEd != nil {
		a.tabsTap(p)
		return
	}
	if a.keyset != nil {
		a.keySetTap(p)
		return
	}
	if a.picker != nil {
		a.pickerTap(p)
		return
	}
	if a.foldTap(p) {
		return
	}
	if isTermTab(a.Tab) && p.In(a.stat.r) {
		a.mediaKey(kobo.KeyPlayPause)
		return
	}
	switch a.Tab {
	case tabRadio:
		a.radioTap(p)
	case tabFiles:
		a.filesTap(p)
	case tabPower:
		a.powerTap(p)
	default:
		a.termTap(p)
	}
}

func (a *bro) Tick() {
	if a.Dozing() {
		a.dozeTick()
		return
	}
	a.dozeWhenIdle()
	a.keysTick()
	a.netTick()
	a.termTick()
	a.statTick()
	a.radioTick()
	a.shareTick()
	a.filesTick()
	a.powerTick()
	a.updateTick()
	a.trialTick()
	a.updateRing()
}

func (a *bro) Exit() {
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
	case kobo.IsMediaKey(m.Code): // the music's, on any tab, dozing too
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
	case a.tabsEd != nil:
		a.tabsKey(m.Code, m.Value)
		return true
	case a.keyset != nil:
		a.keySetKey(m.Code, m.Value)
		return true
	case a.picker != nil:
		a.pickerKey(m.Code, m.Value)
		return true
	case a.termMod(m.Code, m.Value): // kept track of on every tab, for the terminals
		return true
	case a.runKey(m.Code, m.Value): // ours, never the remote's
		return true
	case a.focusKeyed(m.Code, m.Value): // Tab, and Enter on a button
		return true
	case a.focusFolded(): // a panel folded to its strip: Enter opens it, the rest waits
		if m.Value == 1 && m.Code == kobo.KeyEnter {
			a.foldFocused()
		}
		return true
	case a.Tab == tabRadio:
		return a.radioKey(m.Code, m.Value)
	case a.Tab == tabFiles:
		return a.filesKey(m.Code, m.Value)
	case a.Tab == tabPower:
		return false
	}
	return a.termKey(m.Code, m.Value)
}

func (a *bro) Touch(ev kobo.TouchEv, p image.Point, g *kobo.Gesture) {
	switch {
	case g == nil:
	case ev.Kind == kobo.TouchDown:
		a.focusTouched(p)
		switch {
		case !isTermTab(a.Tab):
		case !ev.Pen && p.X >= a.C.Bounds().Dx()-kobo.EdgeWidth:
			a.font = &fontSwipe{g: g}
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
	if a.selLift(g) {
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
	switch {
	case g.Tap || g.Pen || a.overlay() || a.Tab == tabRadio || a.Tab == tabFiles || a.Tab == tabPower:
	default:
		a.termSwipe(g.Start, g.Last)
	}
}
