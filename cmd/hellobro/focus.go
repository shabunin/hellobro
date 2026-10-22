package main

// The keys' focus, as on a desktop: Tab moves it through the page's
// controls (the tab strip, the buttons, the lists), Shift+Tab back, and
// a dashed ring shows where it is. Ctrl+Tab goes to the next panel.
// Enter or Space presses the button in focus (it's a tap on it); a list
// takes the arrows, PgUp/PgDn and the page's letters, as before; on the
// tab strip ← → change tabs. The ring shows once Tab is used; a touch
// takes it away (and the focus goes where it touched). The terminals keep
// Tab: it's the shell's.

import (
	"image"
	"slices"
	"strconv"

	"hellobro/kobo"
)

// control is a stop for Tab.
type control struct {
	id    string          // the same from one draw to the next: "wifi.list", "files.open"
	r     image.Rectangle // as drawn
	panel string          // the foldable it's in; "" for the tab strip
	list  bool            // its keys are the page's: the arrows move in it, Enter picks
}

type focusState struct {
	at   map[int]string // the control in focus, by tab
	ring bool           // shown: Tab was used, and no touch since
}

// controls are the current page's, in Tab's order: the tab strip, then
// each panel's, left to right and down. A folded panel is its strip, and
// Enter opens it.
func (a *bro) controls() []control {
	var cs []control
	add := func(panel, id string, r image.Rectangle, list bool) {
		if !r.Empty() {
			cs = append(cs, control{id, r, panel, list})
		}
	}
	add("", "tabs", a.TabRect(a.Tab), false)
	// panel adds a panel's title button, or its strip if folded: true if
	// that's all of it.
	panel := func(key string) bool {
		p := a.foldPanel(key)
		if p.Folded {
			add(key, key+".strip", p.Title, false)
			return true
		}
		add(key, key+".button", p.Button, false)
		return false
	}
	buttons := func(key string, bs []radioButton) {
		for _, b := range bs {
			add(key, key+"."+b.label, b.r, false)
		}
	}
	switch a.Tab {
	case tabRadio:
		if p := a.rs.prompt; p != nil { // a question: its box and buttons, till it's answered
			cs = cs[:0]
			key := []string{"wifi", "bluetooth"}[p.panel]
			pv := &a.rv[p.panel]
			add(key, key+".input", pv.input, true)
			bs := slices.Clone(pv.buttons) // drawn from the right
			slices.Reverse(bs)
			buttons(key, bs)
			return cs
		}
		for i, key := range []string{"wifi", "bluetooth"} {
			if panel(key) {
				continue
			}
			pv := &a.rv[i]
			for j, r := range pv.ctl {
				add(key, key+".ctl"+strconv.Itoa(j), r, false)
			}
			add(key, key+".list", pv.list, true)
			buttons(key, pv.buttons)
		}
		if !panel("sharing") {
			buttons("sharing", a.shp.btns)
		}
		if !panel("settings") && len(a.sv.rows) > 0 {
			add("settings", "settings.list", a.sv.rows[0].Union(a.sv.rows[len(a.sv.rows)-1]), true)
		}
	case tabPower:
		panel("battery")
		panel("usage")
	}
	return cs
}

// focused is the control in focus among cs: the one Tab got to, or else
// the panel in focus's list (or its first control). -1: none.
func (a *bro) focused(cs []control) int {
	if i := slices.IndexFunc(cs, func(c control) bool { return c.id == a.fs.at[a.Tab] }); i >= 0 {
		return i
	}
	k, first := a.focusKey(), -1
	for i, c := range cs {
		if c.panel == k && c.list {
			return i
		}
		if c.panel == k && first < 0 {
			first = i
		}
	}
	if first < 0 && len(cs) > 0 {
		first = 0
	}
	return first
}

// focusTo moves the focus to c, and the panel focus with it.
func (a *bro) focusTo(c control) {
	if a.fs.at == nil {
		a.fs.at = map[int]string{}
	}
	a.fs.at[a.Tab] = c.id
	if c.panel == "" || c.panel == a.focusKey() {
		return
	}
	switch c.panel {
	case "battery", "usage":
		a.pv.useFocus = c.panel == "usage"
	default:
		if a.rs.prompt != nil { // the question's panel keeps it
			return
		}
		a.rs.focus = radioPanelOf(c.panel)
	}
	a.focusMoved()
}

// focusKeyed takes the keys that move the focus or press what's in it;
// false leaves a key to the page.
func (a *bro) focusKeyed(code uint16, value int32) bool {
	if isTermTab(a.Tab) || value == 0 {
		return false
	}
	m := &a.tv.mods
	cs := a.controls()
	i := a.focused(cs)
	if i < 0 {
		return false
	}
	// Only a control moved to (or touched) takes Enter or the arrows: the
	// one in focus by default may be a button nobody meant to press.
	c, chosen := cs[i], cs[i].id == a.fs.at[a.Tab]
	switch {
	case code == kobo.KeyTab && !m.ctrl && !m.alt && !m.meta:
		d := 1
		if m.shift {
			d = -1
		}
		a.fs.ring = true
		a.focusTo(cs[(i+d+len(cs))%len(cs)])
		a.updateRing()
		return true
	case !chosen:
		return false
	case c.id == "tabs":
		tabs := a.Tabs()
		at := slices.Index(tabs, a.Tab)
		to := at
		switch code {
		case keyLeft:
			to = max(0, at-1)
		case keyRight:
			to = min(len(tabs)-1, at+1)
		case keyHome:
			to = 0
		case keyEnd:
			to = len(tabs) - 1
		default:
			return false
		}
		if to != at {
			a.SwitchTab(tabs[to])
			if !isTermTab(a.Tab) { // the strip keeps the focus; a terminal takes the keys
				a.focusTo(control{id: "tabs"})
			}
		}
		return true
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
	if isTermTab(a.Tab) || a.overlay() {
		return
	}
	for _, c := range a.controls() {
		if p.In(c.r) {
			a.focusTo(c)
			return
		}
	}
}

// updateRing puts the ring where the focus is now: things move, come and
// go. It's looked at every tick.
func (a *bro) updateRing() {
	var r image.Rectangle
	if a.fs.ring && !isTermTab(a.Tab) && !a.overlay() && !a.Calibrating() && !a.Dozing() {
		cs := a.controls()
		if i := a.focused(cs); i >= 0 {
			r = cs[i].r.Inset(-kobo.RingOut)
			if cs[i].id == "tabs" { // tabs touch: inside it
				r = cs[i].r.Inset(kobo.RingOut)
			}
		}
	}
	a.SetRing(r)
}
