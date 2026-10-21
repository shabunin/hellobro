// hellobro is a terminal for the Kobo Elipsa, for a Bluetooth keyboard.
//
// Tabs:
//   - SSH: terminals (xterm emulation, PTY) over SSH, several at once;
//   - Shell: terminals with a shell on the Kobo itself;
//   - Music: a player for the albums on the Kobo, over Bluetooth; it
//     goes on playing on the other tabs.
//
// A swipe along the right edge sets the font size, F11 gives the
// terminal the whole screen. The rest (the screen, touch, the frontlight
// on the left edge, quitting) is the kobo package's.
//
// Launch it through launch.sh, which keeps Nickel from drawing over us.
package main

import (
	"image"
	"log"
	"strconv"
	"time"

	"hellobro/kobo"
)

const (
	tabSSH = iota
	tabShell
	tabMusic
)

var tabNames = []string{"SSH", "Shell", "Music"}

func isTermTab(t int) bool { return t == tabSSH || t == tabShell }

const termIdleTimeout = 20 * time.Minute // with a session up: reading a remote screen takes longer

// bro is the app: the kobo.App, the terminal and the music player.
type bro struct {
	*kobo.App
	tf    termFonts
	termP kobo.Panel
	tv    termView
	font  *fontSwipe // a contact along the right edge, which may become a font swipe
	mv    musicView
	mus   musicState
	stat  statBar     // the terminal tabs' status bar
	lay   kobo.Layout // the last layout, to lay out again
}

// fontSwipe is a contact that started on the right edge.
type fontSwipe struct {
	g        *kobo.Gesture
	from, to int // font sizes: when it started, and when the finger lifts
}

const ownFont = 1 // a gesture that is a font swipe

func main() {
	a := &bro{mus: musicState{open: -1, album: -1, track: -1, gain: 40}}
	if g, err := strconv.Atoi(readMusicState()["gain"]); err == nil {
		a.mus.gain = max(0, min(gainMax, g))
	}
	a.loadStatBar()
	a.App = kobo.New(kobo.Config{Name: "hellobro", Title: "hello bro", Tabs: tabNames}, a)
	a.tf.setSize(termFontDefault)
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
}

func (a *bro) Draw() {
	if a.Tab == tabMusic {
		a.drawMusic()
	} else {
		a.drawTerm()
	}
}

func (a *bro) Enter(tab int) {
	if tab == tabMusic {
		a.enterMusic()
	} else {
		a.enterTerm()
	}
}

func (a *bro) Tap(p image.Point) {
	if isTermTab(a.Tab) && p.In(a.stat.r) {
		a.hotkey(kobo.KeySpace)
		return
	}
	if a.Tab == tabMusic {
		a.musicTap(p)
	} else {
		a.termTap(p)
	}
}

func (a *bro) Tick() {
	a.termTick()
	a.statTick()
}

func (a *bro) Exit() {
	a.termCloseAll()
	a.saveMusicState()
	a.musicStop()
}

func (a *bro) IdleTimeout() time.Duration {
	if d, ok := a.musicIdleTimeout(); ok {
		return d
	}
	if a.termLive() {
		return termIdleTimeout
	}
	return kobo.IdleTimeout
}

func (a *bro) Key(m kobo.KeyMsg) bool {
	switch {
	case m.Connected != "" || m.Gone != "":
		return true
	case kobo.IsMediaKey(m.Code): // the music's, on any tab
		if m.Value == 1 {
			a.musicMediaKey(m.Code)
		}
		return true
	case a.Calibrating():
		return false
	case a.tv.mods.ctrl && a.tv.mods.shift && hotkeys[m.Code]: // ours on every tab, never the remote's
		if m.Value == 1 {
			a.hotkey(m.Code)
		}
		return true
	case a.Tab == tabMusic:
		if a.termMod(m.Code, m.Value) { // kept track of on every tab, for the terminals
			return true
		}
		return a.musicKey(m.Code, m.Value)
	}
	return a.termKey(m.Code, m.Value)
}

const (
	keyB     = 48
	keyComma = 51
	keyDot   = 52
)

// hotkeys are Ctrl+Shift+key, the same on every tab: the music's
// (Space: play/pause, < and >: the previous and next track) and the
// status bar's (B: show or hide).
var hotkeys = map[uint16]bool{kobo.KeySpace: true, keyComma: true, keyDot: true, keyB: true}

func (a *bro) hotkey(code uint16) {
	if code == keyB {
		a.toggleStat()
		return
	}
	media := map[uint16]uint16{kobo.KeySpace: kobo.KeyPlayPause, keyComma: kobo.KeyPreviousSong, keyDot: kobo.KeyNextSong}[code]
	if !a.mus.scanned { // the Music tab hasn't been up yet: find the albums first
		a.scanMusic(func() { a.musicMediaKey(media) })
		return
	}
	a.musicMediaKey(media)
}

func (a *bro) Touch(ev kobo.TouchEv, p image.Point, g *kobo.Gesture) {
	switch {
	case g == nil:
	case ev.Kind == kobo.TouchDown:
		if isTermTab(a.Tab) && !ev.Pen && p.X >= a.C.Bounds().Dx()-kobo.EdgeWidth {
			a.font = &fontSwipe{g: g}
		}
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
	if f := a.font; f != nil && f.g == g {
		a.font = nil
		if g.Owner == ownFont {
			a.HideGauge()
			a.setTermFont(f.to)
			return
		}
	}
	switch {
	case g.Tap || g.Pen:
	case a.Tab == tabMusic:
		a.musicSwipe(g.Start, g.Last)
	default:
		a.termSwipe(g.Start, g.Last)
	}
}
