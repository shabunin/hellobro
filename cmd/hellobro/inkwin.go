package main

// Settings' windows made with ink (ink/, the gallery's components): the
// window's strip is still the window manager's, its body an ink pane
// (inkkobo.Pane) laid out from the state each frame. The pane takes the
// contacts that come down on it (ownPane), and the keys while the window
// is in focus: Tab moves ink's own focus there, not focus.go's.

import (
	"image"

	"hellobro/ink"
	"hellobro/ink/inkkobo"
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
	w.pane.Flush(a.App)
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
	a.askTick()
	if a.overlay() {
		return
	}
	for _, g := range a.inkWins() {
		g.ink.flush(a, a.focusedWin() == window(g))
	}
}

// inkRedraw asks key's window, if it's ink's and shows, for a frame.
func (a *bro) inkRedraw(key string) {
	if g := a.gui(key); g != nil && g.ink != nil {
		g.ink.pane.Win.Invalidate()
	}
}

// inkDown gives a contact coming down on an ink window's body to it, and
// focuses the window.
func (a *bro) inkDown(ev kobo.TouchEv, p image.Point, g *kobo.Gesture) bool {
	gw, ok := a.windowAt(p).(*guiWin)
	if !ok || gw.ink == nil || !gw.ink.pane.Down(ev, p, g, ownPane) {
		return false
	}
	if a.focusedWin() != window(gw) {
		a.wm.tree.Focus(a.nodeOf(gw))
		a.wmApply()
	}
	a.inkTick()
	return true
}

func (a *bro) inkMove(ev kobo.TouchEv, p image.Point, g *kobo.Gesture) bool {
	if g.Owner != ownPane {
		return false
	}
	if q := a.asking; q != nil && q.pane.Move(ev, p, g) {
		return true
	}
	for _, gw := range a.inkWins() {
		if gw.ink.pane.Move(ev, p, g) {
			break
		}
	}
	return true
}

func (a *bro) inkLift(g *kobo.Gesture) bool {
	if g.Owner != ownPane {
		return false
	}
	if q := a.asking; q != nil && q.pane.Lift(g) {
		a.inkTick()
		return true
	}
	for _, w := range a.wm.wins {
		if gw, ok := w.(*guiWin); ok && gw.ink != nil && gw.ink.pane.Lift(g) {
			break
		}
	}
	a.inkTick()
	return true
}

// updateRing puts the focus ring where the ink window in focus has it,
// if it shows: Tab moved the focus, and no touch since.
func (a *bro) updateRing() {
	var r image.Rectangle
	switch g, ok := a.focusedWin().(*guiWin); {
	case a.Calibrating():
	case a.asking != nil && !a.wm.offscreen:
		r = a.asking.pane.Ring()
	case ok && g.ink != nil && !a.overlay():
		r = g.ink.pane.Ring()
	}
	a.SetRing(r)
}
