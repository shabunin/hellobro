// hellobro is a terminal for the Kobo Elipsa, for a Bluetooth keyboard.
//
// Tabs:
//   - SSH: terminals (xterm emulation, PTY) over SSH, several at once;
//   - Shell: terminals with a shell on the Kobo itself.
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
	"time"

	"hellobro/kobo"
)

const (
	tabSSH = iota
	tabShell
)

var tabNames = []string{"SSH", "Shell"}

const termIdleTimeout = 20 * time.Minute // with a session up: reading a remote screen takes longer

// bro is the app: the kobo.App, and the terminal on its pages.
type bro struct {
	*kobo.App
	tf    termFonts
	termP kobo.Panel
	tv    termView
	font  *fontSwipe // a contact along the right edge, which may become a font swipe
}

// fontSwipe is a contact that started on the right edge.
type fontSwipe struct {
	g        *kobo.Gesture
	from, to int // font sizes: when it started, and when the finger lifts
}

const ownFont = 1 // a gesture that is a font swipe

func main() {
	a := &bro{}
	a.App = kobo.New(kobo.Config{Name: "hellobro", Title: "hello bro", Tabs: tabNames}, a)
	a.tf.setSize(termFontDefault)
	a.Run()
}

func (a *bro) Layout(L kobo.Layout) {
	if L.Full { // no frame: the cells get it all
		a.termP = kobo.Panel{Name: "Terminal", Outer: L.Page, Body: L.Page}
	} else {
		a.termP = kobo.NewPanel(a.F, "Terminal", L.Body, "")
	}
	if a.tv.cw != 0 {
		a.termGeometry() // the terminal follows the new size
	}
}

func (a *bro) Draw()             { a.drawTerm() }
func (a *bro) Enter(tab int)     { a.enterTerm() }
func (a *bro) Tap(p image.Point) { a.termTap(p) }
func (a *bro) Tick()             { a.termTick() }
func (a *bro) Exit()             { a.termCloseAll() }

func (a *bro) IdleTimeout() time.Duration {
	if a.termLive() {
		return termIdleTimeout
	}
	return kobo.IdleTimeout
}

func (a *bro) Key(m kobo.KeyMsg) bool {
	if m.Connected != "" || m.Gone != "" {
		return true
	}
	return !a.Calibrating() && a.termKey(m.Code, m.Value)
}

func (a *bro) Touch(ev kobo.TouchEv, p image.Point, g *kobo.Gesture) {
	switch {
	case g == nil:
	case ev.Kind == kobo.TouchDown:
		if !ev.Pen && p.X >= a.C.Bounds().Dx()-kobo.EdgeWidth {
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
	if !g.Tap && !g.Pen {
		a.termSwipe(g.Start, g.Last)
	}
}
