package main

// Settings' windows made with ink (ink/, the gallery's components): the
// window's strip is still the window manager's, its body an ink pane
// (inkkobo.Pane) laid out from the state each frame. The pane takes the
// contacts that come down on it (ownPane), and the keys while the window
// is in focus: Tab moves ink's own focus there, not focus.go's.

import (
	"image"
	"slices"

	"hellobro/ink"
	"hellobro/ink/inkkobo"
	"hellobro/ink/text"
	"hellobro/ink/theme"
	"hellobro/ink/widget"
	"hellobro/kobo"
)

const ownPane = 20 // a contact on an ink window's body

// inkUI is a settings' window's body in ink, and a word for its strip.
type inkUI interface {
	Layout(gtx ink.Ctx) ink.Dims
	Status() string
}

// inkUIs make the windows that are ink's, by key.
var inkUIs = map[string]func(a *bro) inkUI{
	"battery":   func(a *bro) inkUI { return &batteryUI{a: a} },
	"usage":     func(a *bro) inkUI { return &usageUI{a: a} },
	"sharing":   func(a *bro) inkUI { return &shareUI{a: a} },
	"wifi":      func(a *bro) inkUI { return &radioUI{a: a, panel: panelWiFi} },
	"bluetooth": func(a *bro) inkUI { return &radioUI{a: a, panel: panelBT} },
	"settings": func(a *bro) inkUI {
		return &settingsUI{a: a, sel: make([]widget.Select, len(settingList)), act: make([]widget.Clickable, len(settingList))}
	},
}

// inkWin is a guiWin's ink: its pane, and its strip as drawn.
type inkWin struct {
	ui     inkUI
	pane   *inkkobo.Pane
	p      kobo.Panel // placed
	status string     // the strip's, as drawn
}

// theme is the look of hellobro's ink windows: one for all.
func (a *bro) theme() *theme.Theme {
	if a.th == nil {
		a.th = theme.New()
	}
	return a.th
}

// inkOf is g's ink, made the first time; nil if g isn't ink's.
func (a *bro) inkOf(g *guiWin) *inkWin {
	if g.ink == nil {
		mk, ok := inkUIs[g.key]
		if !ok {
			return nil
		}
		ui := mk(a)
		g.ink = &inkWin{ui: ui, pane: inkkobo.NewPane(ui.Layout)}
	}
	return g.ink
}

func (w *inkWin) place(p kobo.Panel) {
	w.p = p
	w.pane.Place(p.Body)
}

// draw draws it all: the strip, the body.
func (w *inkWin) draw(a *bro, focused bool) {
	if w.p.Outer.Empty() {
		return
	}
	w.status = w.ui.Status()
	a.DrawPanel(a.winPanel(w.p, focused), w.status)
	w.pane.Draw(a.C.Gray)
	a.Mark(w.p.Outer, true)
}

// drawTitle draws the strip again: the focus came or went.
func (w *inkWin) drawTitle(a *bro, focused bool) {
	if w.p.Outer.Empty() {
		return
	}
	w.status = w.ui.Status()
	pn := a.winPanel(w.p, focused)
	a.DrawPanelTitle(pn, w.status)
	a.Mark(pn.Title, true)
}

// flush puts what changed on the panel: the body's, the strip's word.
func (w *inkWin) flush(a *bro, focused bool) {
	if w.p.Outer.Empty() {
		return
	}
	if s := w.ui.Status(); s != w.status {
		w.drawTitle(a, focused)
	}
	a.flushPane(w.pane)
}

// inkMods are the modifiers held, as ink has them.
func inkMods(m termMods) ink.Mods {
	var md ink.Mods
	if m.shift {
		md |= ink.Shift
	}
	if m.ctrl {
		md |= ink.Ctrl
	}
	if m.alt {
		md |= ink.Alt
	}
	if m.meta {
		md |= ink.Super
	}
	return md
}

// inkWins are the ink windows that show.
func (a *bro) inkWins() []*guiWin {
	var gs []*guiWin
	for _, w := range a.wm.wins {
		if g, ok := w.(*guiWin); ok && g.ink != nil && !g.ink.p.Outer.Empty() {
			gs = append(gs, g)
		}
	}
	return gs
}

// inkTick lays out what's due and puts it on the panel.
func (a *bro) inkTick() {
	if a.Calibrating() {
		return
	}
	a.overTick()
	if a.overlay() {
		return
	}
	for _, g := range a.inkWins() {
		g.ink.flush(a, a.focusedWin() == window(g))
	}
	for _, b := range a.bodies() {
		a.flushPane(b.inkPane())
		b.take()
	}
}

// inkBody is an ink pane in a window that isn't a settings' window: the
// launcher's tiles, a list of books or notebooks.
type inkBody interface {
	inkPane() *inkkobo.Pane
	take() // what the frame picked, done
}

// bodied is a window that may show some.
type bodied interface {
	inkBodies(a *bro) []inkBody // those that show
}

// bodies are the ink bodies that show.
func (a *bro) bodies() []inkBody {
	var bs []inkBody
	for _, w := range a.wm.wins {
		if bw, ok := w.(bodied); ok {
			bs = append(bs, bw.inkBodies(a)...)
		}
	}
	return bs
}

func (t *termSession) inkBodies(a *bro) []inkBody {
	if u := t.pane.launch; u != nil && u.shows(a) {
		return []inkBody{u}
	}
	return nil
}

func (u *launchUI) inkPane() *inkkobo.Pane { return u.pane }

// inkRedraw asks key's window, if it's ink's and shows, for a frame.
func (a *bro) inkRedraw(key string) {
	if g := a.gui(key); g != nil && g.ink != nil {
		g.ink.pane.Win.Invalidate()
	}
}

// inkDown gives a contact coming down on an ink window's body to it, and
// focuses the window.
func (a *bro) inkDown(ev kobo.TouchEv, p image.Point, g *kobo.Gesture) bool {
	var panes []*inkkobo.Pane
	w := a.windowAt(p)
	switch w := w.(type) {
	case *guiWin:
		if w.ink != nil {
			panes = append(panes, w.ink.pane)
		}
	case bodied:
		for _, b := range w.inkBodies(a) {
			panes = append(panes, b.inkPane())
		}
	}
	if !slices.ContainsFunc(panes, func(pn *inkkobo.Pane) bool { return pn.Down(ev, p, g, ownPane) }) {
		return false
	}
	if a.focusedWin() != w {
		a.wm.tree.Focus(a.nodeOf(w))
		a.wmApply()
	}
	a.inkTick()
	return true
}

func (a *bro) inkMove(ev kobo.TouchEv, p image.Point, g *kobo.Gesture) bool {
	if g.Owner != ownPane {
		return false
	}
	if o := a.over(); o != nil && o.pane.Move(ev, p, g) {
		return true
	}
	for _, gw := range a.inkWins() {
		if gw.ink.pane.Move(ev, p, g) {
			return true
		}
	}
	for _, b := range a.bodies() {
		if b.inkPane().Move(ev, p, g) {
			break
		}
	}
	return true
}

func (a *bro) inkLift(g *kobo.Gesture) bool {
	if g.Owner != ownPane {
		return false
	}
	if o := a.over(); o != nil && o.pane.Lift(g) {
		a.inkTick()
		return true
	}
	for _, w := range a.wm.wins {
		if gw, ok := w.(*guiWin); ok && gw.ink != nil && gw.ink.pane.Lift(g) {
			break
		}
		if bw, ok := w.(bodied); ok && slices.ContainsFunc(bw.inkBodies(a), func(b inkBody) bool { return b.inkPane().Lift(g) }) {
			break
		}
	}
	a.inkTick()
	return true
}

// inkCancel lets g go from whichever pane has it, no click.
func (a *bro) inkCancel(g *kobo.Gesture) {
	if o := a.over(); o != nil && o.pane.Cancel(g) {
		return
	}
	for _, gw := range a.inkWins() {
		if gw.ink.pane.Cancel(g) {
			return
		}
	}
	for _, b := range a.bodies() {
		if b.inkPane().Cancel(g) {
			return
		}
	}
}

// updateRing puts the focus ring where the ink window in focus has it,
// if it shows: Tab moved the focus, and no touch since.
func (a *bro) updateRing() {
	var r image.Rectangle
	switch g, ok := a.focusedWin().(*guiWin); {
	case a.Calibrating():
	case a.over() != nil && !a.wm.offscreen:
		r = a.over().pane.Ring()
	case ok && g.ink != nil && !a.overlay():
		r = g.ink.pane.Ring()
	case !a.overlay():
		if bw, ok := a.focusedWin().(bodied); ok {
			for _, b := range bw.inkBodies(a) {
				if rr := b.inkPane().Ring(); !rr.Empty() {
					r = rr
				}
			}
		}
	}
	a.SetRing(r)
}

// flushPane puts what changed of p on the panel: a window's, so the
// floating windows over it are drawn again where it changed.
func (a *bro) flushPane(p *inkkobo.Pane) { p.FlushTo(a.C.Gray, a.Mark) }

// inkPaint paints ui at r on the canvas, once, for the caller to mark:
// what's only shown in ink, its taps the window's own (an empty
// workspace, the ":" line, a cut's buttons). *p keeps the pane between
// paints.
func (a *bro) inkPaint(p **inkkobo.Pane, r image.Rectangle, ui ink.Widget) {
	if *p == nil {
		*p = inkkobo.NewPane(ui)
	}
	(*p).Win.UI = ui
	(*p).Place(r)
	(*p).Draw(a.C.Gray)
}

// inkButton is a button's look as a picture, as big as it's given: its
// label in the middle, inverted while it's pressed.
func (a *bro) inkButton(label string, pressed bool) ink.Widget {
	return func(gtx ink.Ctx) ink.Dims {
		th := a.theme()
		size := gtx.Constraints.Max
		r := image.Rectangle{Max: size}
		fg, bg := th.Fg, th.Bg
		if pressed {
			fg, bg = bg, fg
		}
		ink.Fill(gtx.Ops, r, bg)
		ink.Frame(gtx.Ops, r, max(gtx.Dp(th.Border), 2), th.Fg)
		f := th.Face(gtx, th.TextSize, text.Regular)
		s := text.Fit(f, label, size.X-gtx.Dp(th.Pad))
		ink.TextAt(gtx.Ops, f, (size.X-text.Width(f, s))/2, (size.Y-text.Height(f))/2+text.Ascent(f), s, fg)
		return ink.Dims{Size: size}
	}
}

// inkIcon is an icon's button as a picture, as big as it's given;
// inverted while it's pressed.
func (a *bro) inkIcon(icon []iconPart, pressed bool) ink.Widget {
	return func(gtx ink.Ctx) ink.Dims {
		th := a.theme()
		size := gtx.Constraints.Max
		r := image.Rectangle{Max: size}
		fg, bg := th.Fg, th.Bg
		if pressed {
			fg, bg = bg, fg
		}
		ink.Fill(gtx.Ops, r, bg)
		ink.Frame(gtx.Ops, r, max(gtx.Dp(th.Border), 2), th.Fg)
		ink.Draw(gtx.Ops, r, func(dst *image.Gray, dr image.Rectangle) {
			drawIcon(kobo.Canvas{Gray: dst}, icon, dr, fg, bg)
		})
		return ink.Dims{Size: size}
	}
}
