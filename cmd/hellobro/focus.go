package main

// The keys' focus within a settings' window, as on a desktop: Tab moves
// it through the window's controls (the buttons, the lists), Shift+Tab
// back, and a dashed ring shows where it is. Enter or Space presses the
// button in focus (it's a tap on it); a list takes the arrows, PgUp/PgDn
// and the window's letters, as before. The ring shows once Tab is used;
// a touch takes it away (and the focus goes where it touched). The
// terminals keep Tab: it's the shell's. Between the windows the focus
// moves with Meta and the arrows (wm.go).

import (
	"image"
	"slices"
	"strconv"

	"hellobro/kobo"
)

// control is a stop for Tab.
type control struct {
	id   string          // the same from one draw to the next: "wifi.list", "wifi.scan"
	r    image.Rectangle // as drawn
	list bool            // its keys are the window's: the arrows move in it, Enter picks
}

type focusState struct {
	at   string // the control in focus
	win  window // whose it is
	ring bool   // shown: Tab was used, and no touch since
}

// controls are w's, in Tab's order: left to right and down.
func (a *bro) controls(w window) []control {
	g, ok := w.(*guiWin)
	if !ok || g.panel(a).Outer.Empty() {
		return nil
	}
	var cs []control
	add := func(id string, r image.Rectangle, list bool) {
		if !r.Empty() {
			cs = append(cs, control{g.key + "." + id, r, list})
		}
	}
	buttons := func(bs []radioButton) {
		for _, b := range bs {
			add(b.label, b.r, false)
		}
	}
	switch g.key {
	case "wifi", "bluetooth":
		i := radioPanelOf(g.key)
		pv := &a.rv[i]
		if p := a.rs.prompt; p != nil && p.panel == i { // a question: its box and buttons, till it's answered
			add("input", pv.input, true)
			bs := slices.Clone(pv.buttons) // drawn from the right
			slices.Reverse(bs)
			buttons(bs)
			break
		}
		for j, r := range pv.ctl {
			add("ctl"+strconv.Itoa(j), r, false)
		}
		add("list", pv.list, true)
		buttons(pv.buttons)
	case "sharing":
		buttons(a.shp.btns)
	case "settings":
		if len(a.sv.rows) > 0 {
			add("list", a.sv.rows[0].Union(a.sv.rows[len(a.sv.rows)-1]), true)
		}
	}
	return cs
}

// focused is the control in focus among cs, w's: the one Tab got to, or
// else w's list (or its first control). -1: none.
func (a *bro) focused(w window, cs []control) int {
	if a.fs.win == w {
		if i := slices.IndexFunc(cs, func(c control) bool { return c.id == a.fs.at }); i >= 0 {
			return i
		}
	}
	if i := slices.IndexFunc(cs, func(c control) bool { return c.list }); i >= 0 {
		return i
	}
	if len(cs) > 0 {
		return 0
	}
	return -1
}

// focusKeyed takes the keys that move the focus or press what's in it;
// false leaves a key to the window.
func (a *bro) focusKeyed(code uint16, value int32) bool {
	w := a.focusedWin()
	cs := a.controls(w)
	if value == 0 || len(cs) == 0 {
		return false
	}
	m := &a.tv.mods
	i := a.focused(w, cs)
	// Only a control moved to (or touched) takes Enter: the one in focus by
	// default may be a button nobody meant to press.
	c, chosen := cs[i], a.fs.win == w && cs[i].id == a.fs.at
	switch {
	case code == kobo.KeyTab && !m.ctrl && !m.alt && !m.meta:
		d := 1
		if m.shift {
			d = -1
		}
		a.fs.ring = true
		a.fs.at, a.fs.win = cs[(i+d+len(cs))%len(cs)].id, w
		a.updateRing()
		return true
	case !chosen:
		return false
	case !c.list && (code == kobo.KeyEnter || code == kobo.KeySpace):
		if value == 1 {
			a.Tap(c.r.Min.Add(c.r.Size().Div(2)))
		}
		return true
	}
	return false
}

// focusTouched moves the focus to what was touched, and hides the ring.
func (a *bro) focusTouched(p image.Point) {
	a.fs.ring = false
	if a.overlay() {
		return
	}
	w := a.windowAt(p)
	for _, c := range a.controls(w) {
		if p.In(c.r) {
			a.fs.at, a.fs.win = c.id, w
			return
		}
	}
}

// updateRing puts the ring where the focus is now: things move, come and
// go. It's looked at every tick.
func (a *bro) updateRing() {
	var r image.Rectangle
	if a.fs.ring && !a.overlay() && !a.Calibrating() {
		w := a.focusedWin()
		cs := a.controls(w)
		if i := a.focused(w, cs); i >= 0 {
			r = cs[i].r.Inset(-kobo.RingOut)
		}
	}
	a.SetRing(r)
}
