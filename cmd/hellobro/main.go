// hellobro is a terminal for the Kobo Elipsa, for a Bluetooth keyboard.
//
// Tabs:
//   - SSH: terminals (xterm emulation, PTY) over SSH, several at once;
//   - Shell: terminals with a shell on the Kobo itself;
//   - Music: a player for the albums on the Kobo, over Bluetooth; it
//     goes on playing on the other tabs;
//   - Settings: Kobo's Wi-Fi and Bluetooth, and hellobro's sleep, sharing
//     and status bar;
//   - Files: sharing with a computer (SFTP, ssh, mDNS), and the Kobo's
//     storage.
//
// A swipe along the right edge sets the font size, F11 gives the
// terminal the whole screen. Which tabs show, and in what order, is up
// to the user (tabs.go), and so are the keys (commands.go). The rest
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
	"strconv"
	"time"

	"hellobro/kobo"
	"hellobro/radio"
)

const (
	tabSSH = iota
	tabShell
	tabMusic
	tabRadio
	tabFiles
)

var tabNames = []string{"SSH", "Shell", "Music", "Settings", "Files"}

func isTermTab(t int) bool { return t == tabSSH || t == tabShell }

const termIdleTimeout = 20 * time.Minute // with a session up: reading a remote screen takes longer
const relayIdle = 24 * time.Hour         // with the relay on: as good as never

// bro is the app: the kobo.App, the terminal, the music player and the
// radios.
type bro struct {
	*kobo.App
	tf       termFonts
	termP    kobo.Panel
	tv       termView
	font     *fontSwipe // a contact along the right edge, which may become a font swipe
	mv       musicView
	mus      musicState
	stat     statBar     // the terminal tabs' status bar
	picker   *picker     // the switcher, while it's up
	asking   *askBox     // a question, while it's up
	tabsEd   *tabsEditor // the Tabs dialog, while it's up
	tabs     tabsConf
	keys     keyMap
	keyset   *keySet // Settings' Keys: a key being asked for
	lightWas int     // the light's level before light.toggle turned it off
	rv       [2]radioPanelView
	rs       radioState
	slp      sleepState
	sv       settingsView
	set      settings
	doze     dozeState
	share    shareState
	relay    relayState
	fv       filesView
	upd      updateState
	folds    map[string]bool // the panels folded, by key (fold.go)
	net      netState        // the header's network
	lay      kobo.Layout     // the last layout, to lay out again
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
	updateTrial() // before anything that might hang
	a := &bro{mus: musicState{lib: &library{}, gain: 40}}
	if g, err := strconv.Atoi(readMusicState()["gain"]); err == nil {
		a.mus.gain = max(0, min(gainMax, g))
	}
	a.loadStatBar()
	a.set = loadSettings()
	radio.StateDir = kobo.AppFile("")
	a.tabs = loadTabs()
	a.folds = loadFolds()
	a.App = kobo.New(kobo.Config{Name: "hellobro", Title: "hello bro", Tabs: tabNames, OwnKeys: true}, a)
	log.Printf("hellobro %s", version)
	a.keys = loadKeys()
	a.applyTabs()
	a.tf.setSize(termFontDefault)
	a.shareApply()
	go a.watchHeadphones() // the bus may be slow: never in the way of starting
	a.Run()
}

func (a *bro) Layout(L kobo.Layout) {
	a.lay = L
	if L.Full { // no frame: the cells get it all
		a.termP = kobo.Panel{Name: "Terminal", Outer: L.Page, Body: L.Page}
	} else {
		a.termP = kobo.NewPanel(a.F, "Terminal", L.Body, "")
	}
	a.layoutStat()
	if a.tv.cw != 0 {
		a.termGeometry() // the terminal follows the new size
	}
	a.layoutMusic(L)
	a.layoutRadio(L)
	a.layoutFiles(L)
}

func (a *bro) Draw() {
	switch a.Tab {
	case tabMusic:
		a.drawMusic()
	case tabRadio:
		a.rv[panelWiFi].shown, a.rv[panelBT].shown, a.sv.shown = "", "", ""
		a.drawRadio()
	case tabFiles:
		a.drawFiles()
	default:
		a.drawTerm()
	}
}

func (a *bro) Enter(tab int) {
	switch tab {
	case tabMusic:
		a.enterMusic()
	case tabRadio:
		a.enterRadio()
	case tabFiles:
		a.enterFiles()
	default:
		a.enterTerm()
	}
}

func (a *bro) Tap(p image.Point) {
	if a.asking != nil {
		a.askTap(p)
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
	case tabMusic:
		a.musicTap(p)
	case tabRadio:
		a.radioTap(p)
	case tabFiles:
		a.filesTap(p)
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
	a.updateTick()
	a.trialTick()
}

func (a *bro) Exit() {
	a.termCloseAll()
	a.saveMusicState()
	a.musicStop()
	a.radioExit()
	a.shareExit()
}

func (a *bro) IdleTimeout() time.Duration {
	if a.Dozing() { // dozeTick sleeps when the music's over
		return playingIdle
	}
	if d, ok := a.musicIdleTimeout(); ok {
		return d
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
	case a.Tab == tabMusic:
		return a.musicKey(m.Code, m.Value)
	case a.Tab == tabRadio:
		return a.radioKey(m.Code, m.Value)
	case a.Tab == tabFiles:
		return a.filesKey(m.Code, m.Value)
	}
	return a.termKey(m.Code, m.Value)
}

func (a *bro) mediaKey(code uint16) {
	if !a.mus.scanned { // the Music tab hasn't been up yet: find the albums first
		a.scanMusic(func() { a.musicMediaKey(code) })
		return
	}
	a.musicMediaKey(code)
}

func (a *bro) Touch(ev kobo.TouchEv, p image.Point, g *kobo.Gesture) {
	switch {
	case g == nil:
	case ev.Kind == kobo.TouchDown:
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
	case g.Tap || g.Pen || a.overlay() || a.Tab == tabRadio || a.Tab == tabFiles:
	case a.Tab == tabMusic:
		a.musicSwipe(g.Start, g.Last)
	default:
		a.termSwipe(g.Start, g.Last)
	}
}
