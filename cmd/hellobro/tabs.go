package main

// The tabs: which are shown, and in what order. The tabs file keeps it,
// "ssh -shell settings files power", a - hiding one. Shell starts
// hidden; Settings can't be, it's where they're set. The Tabs dialog
// (a row in Settings, or the switcher) changes them: a tap or Space
// shows or hides one, Shift+↑ ↓ or the arrows move it.

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"slices"
	"strconv"
	"strings"

	"hellobro/kobo"
)

const (
	tabsTxt     = "tabs"
	tabsDefault = "ssh -shell settings files power"
	tabsRowH    = 76
)

type tabsConf struct {
	order  []int // all of them
	hidden map[int]bool
}

func tabKey(t int) string { return strings.ToLower(tabNames[t]) }

func loadTabs() tabsConf {
	b, err := os.ReadFile(kobo.AppFile(tabsTxt))
	if err != nil {
		b = []byte(tabsDefault)
	}
	return parseTabs(string(b))
}

func parseTabs(s string) tabsConf {
	c := tabsConf{hidden: map[int]bool{}}
	for _, w := range strings.Fields(s) {
		name, hide := strings.CutPrefix(w, "-")
		t := slices.IndexFunc(tabNames, func(n string) bool { return strings.EqualFold(n, name) })
		if t < 0 || slices.Contains(c.order, t) {
			continue
		}
		c.order = append(c.order, t)
		c.hidden[t] = hide && t != tabRadio
	}
	def := parseTabsOrder(tabsDefault)
	for t := range tabNames { // one the file doesn't know of: as by default
		if !slices.Contains(c.order, t) {
			c.order = append(c.order, t)
			c.hidden[t] = def[t]
		}
	}
	return c
}

// parseTabsOrder is which tabs s hides, the defaults' way.
func parseTabsOrder(s string) map[int]bool {
	h := map[int]bool{}
	for _, w := range strings.Fields(s) {
		if name, ok := strings.CutPrefix(w, "-"); ok {
			h[slices.IndexFunc(tabNames, func(n string) bool { return strings.EqualFold(n, name) })] = true
		}
	}
	return h
}

func (c tabsConf) String() string {
	var ws []string
	for _, t := range c.order {
		w := tabKey(t)
		if c.hidden[t] {
			w = "-" + w
		}
		ws = append(ws, w)
	}
	return strings.Join(ws, " ")
}

// shown are the tabs shown, in order.
func (c tabsConf) shown() []int {
	return slices.DeleteFunc(slices.Clone(c.order), func(t int) bool { return c.hidden[t] })
}

func (a *bro) saveTabs() {
	if err := os.WriteFile(kobo.AppFile(tabsTxt), []byte(a.tabs.String()+"\n"), 0o644); err != nil {
		log.Printf("tabs: %v", err)
	}
	log.Printf("tabs: %s", a.tabs)
}

// applyTabs shows the tabs as set, with their keys in the corners and
// the footer.
func (a *bro) applyTabs() {
	hints, footer := a.tabHints()
	a.App.SetTabs(a.tabs.shown(), hints)
	sleep := a.keyLabel("sleep")
	if sleep == "" {
		sleep = "-"
	}
	quit := a.keyLabel("quit")
	if quit == "" {
		quit = "-"
	}
	a.SetFooter(footer, sleep, quit)
}

// tabHints are the keys to the tabs shown, by place, and the footer's
// word on them. A tab's key is its own (tab.shell) or its place's
// (tab.3). The corners have little room: keys with the modifiers most
// of them have show without, which the footer gives.
func (a *bro) tabHints() ([]string, string) {
	order := a.tabs.shown()
	keys := make([]*combo, len(order))
	count := map[uint8]int{}
	for i, t := range order {
		for _, id := range []string{"tab." + tabKey(t), "tab." + strconv.Itoa(i+1)} {
			if ks := a.keys.keys[id]; len(ks) > 0 {
				keys[i] = &ks[0]
				count[ks[0].mods]++
				break
			}
		}
	}
	if len(count) == 0 {
		return make([]string, len(order)), "tabs: tap"
	}
	var mods uint8
	for m, n := range count {
		if n > count[mods] || n == count[mods] && m < mods {
			mods = m
		}
	}
	hints := make([]string, len(order))
	numbered := len(count) == 1
	for i, k := range keys {
		switch {
		case k == nil:
			numbered = false
		case k.mods == mods:
			hints[i] = combo{code: k.code}.label()
		default:
			hints[i] = k.label()
		}
		if k != nil && k.code != uint16(kobo.Key1+i) {
			numbered = false
		}
	}
	switch {
	case numbered:
		return hints, fmt.Sprintf("tabs: tap or %s1–%d", modsLabel(mods), len(order))
	case mods != 0:
		return hints, "tabs: tap or " + modsLabel(mods) + "corner key"
	default:
		return hints, "tabs: tap or the corner key"
	}
}

// showTab shows or hides tab t.
func (a *bro) showTab(t int, on bool) {
	if t == tabRadio {
		return
	}
	a.tabs.hidden[t] = !on
	a.saveTabs()
	a.applyTabs()
}

// tabsSummary is the tabs shown, for the Settings row.
func (a *bro) tabsSummary() string {
	var ns []string
	for _, t := range a.tabs.shown() {
		ns = append(ns, tabNames[t])
	}
	return strings.Join(ns, " · ")
}

// --- the dialog ---

type tabsEditor struct {
	sel              int // a place in a.tabs.order
	box, done        image.Rectangle
	rows, ups, downs []image.Rectangle
}

func (a *bro) openTabs() {
	b := a.C.Bounds()
	w := min(900, b.Dx()-80)
	n := len(a.tabs.order)
	h := 32 + 70 + n*tabsRowH + 20 + 68 + 32
	top := b.Min.Y + max(40, (b.Dy()-h)/2)
	e := &tabsEditor{sel: max(0, slices.Index(a.tabs.order, a.Tab))}
	e.box = image.Rect((b.Dx()-w)/2, top, (b.Dx()+w)/2, top+h)
	in := e.box.Inset(32)
	y := in.Min.Y + 70
	for range n {
		r := image.Rect(in.Min.X, y, in.Max.X, y+tabsRowH-8)
		bw := 84
		e.rows = append(e.rows, r)
		e.downs = append(e.downs, image.Rect(r.Max.X-8-bw, r.Min.Y+6, r.Max.X-8, r.Max.Y-6))
		e.ups = append(e.ups, image.Rect(r.Max.X-8-2*bw-12, r.Min.Y+6, r.Max.X-8-bw-12, r.Max.Y-6))
		y += tabsRowH
	}
	e.done = image.Rect(in.Max.X-240, in.Max.Y-68, in.Max.X, in.Max.Y)
	a.tabsEd = e
	a.drawTabsEdit()
}

func (a *bro) closeTabs() {
	a.tabsEd = nil
	a.DrawPage()
	a.drawStat(true)
}

func (a *bro) drawTabsEdit() {
	e, c := a.tabsEd, a.C
	c.Fill(e.box, kobo.White)
	c.Frame(e.box, 4, kobo.Black)
	in := e.box.Inset(32)
	head := image.Rect(in.Min.X, in.Min.Y, in.Max.X, in.Min.Y+56)
	c.Text(a.F.Panel, head, "Tabs", kobo.Black, kobo.AlignLeft)
	c.Text(a.F.Small, head, "the ticked ones, in this order", kobo.Dark, kobo.AlignRight)
	n := len(a.tabs.order)
	for i, t := range a.tabs.order {
		r := e.rows[i]
		if i == e.sel {
			c.Frame(r, 3, kobo.Black)
		}
		box := image.Rect(r.Min.X+20, r.Min.Y+(r.Dy()-36)/2, r.Min.X+56, r.Min.Y+(r.Dy()-36)/2+36)
		shown := !a.tabs.hidden[t]
		ink := uint8(kobo.Black)
		if t == tabRadio {
			ink = kobo.Mid // can't be unticked
		}
		c.Frame(box, 3, ink)
		if shown {
			c.Fill(box.Inset(8), ink)
		}
		text := image.Rect(box.Max.X+24, r.Min.Y, e.ups[i].Min.X-16, r.Max.Y)
		fg := uint8(kobo.Black)
		if !shown {
			fg = kobo.Dark
		}
		c.Text(a.F.Body, text, tabNames[t], fg, kobo.AlignLeft)
		note := ""
		switch {
		case t == tabRadio:
			note = "always: tabs are set here"
		case !shown:
			note = "hidden"
		}
		text.Min.X += kobo.TextWidth(a.F.Body, tabNames[t]) + 24
		c.Text(a.F.Small, text, note, kobo.Dark, kobo.AlignLeft)
		for _, btn := range []struct {
			r  image.Rectangle
			on bool
			up bool
		}{{e.ups[i], i > 0, true}, {e.downs[i], i < n-1, false}} {
			if !btn.on {
				continue
			}
			c.Frame(btn.r, 2, kobo.Black)
			m := image.Pt((btn.r.Min.X+btn.r.Max.X)/2, (btn.r.Min.Y+btn.r.Max.Y)/2)
			if btn.up {
				triangle(c, image.Pt(m.X, m.Y-12), image.Pt(m.X-16, m.Y+10), image.Pt(m.X+16, m.Y+10), kobo.Black)
			} else {
				triangle(c, image.Pt(m.X, m.Y+12), image.Pt(m.X-16, m.Y-10), image.Pt(m.X+16, m.Y-10), kobo.Black)
			}
		}
	}
	hint := image.Rect(in.Min.X, e.done.Min.Y, e.done.Min.X-20, e.done.Max.Y)
	c.Text(a.F.Small, hint, "Space: show or hide  ·  Shift+↑ ↓: move", kobo.Dark, kobo.AlignLeft)
	c.Fill(e.done, kobo.Black)
	c.Text(a.F.Small, e.done, "done  (Esc)", kobo.White, kobo.AlignCenter)
	a.Mark(e.box, true)
}

// tabsChanged keeps the change and shows it, the dialog staying up.
func (a *bro) tabsChanged() {
	a.saveTabs()
	e := a.tabsEd
	a.tabsEd = nil // the page under it is drawn as it is
	a.applyTabs()
	a.tabsEd = e
	a.drawTabsEdit()
}

func (a *bro) toggleTab(i int) {
	if t := a.tabs.order[i]; t != tabRadio {
		a.tabs.hidden[t] = !a.tabs.hidden[t]
		a.tabsChanged()
	}
}

func (a *bro) moveTab(i, d int) {
	o, j := a.tabs.order, i+d
	if j < 0 || j >= len(o) {
		return
	}
	o[i], o[j] = o[j], o[i]
	a.tabsEd.sel = j
	a.tabsChanged()
}

func (a *bro) tabsKey(code uint16, value int32) {
	if a.termMod(code, value) || value == 0 {
		return
	}
	e, m := a.tabsEd, &a.tv.mods
	switch {
	case code == kobo.KeyEsc:
		a.closeTabs()
		return
	case (code == keyUp || code == keyDown) && (m.shift || m.alt):
		a.moveTab(e.sel, map[bool]int{true: -1, false: 1}[code == keyUp])
		return
	case code == kobo.KeySpace || code == kobo.KeyEnter:
		a.toggleTab(e.sel)
		return
	case code == keyUp:
		e.sel = max(0, e.sel-1)
	case code == keyDown, code == kobo.KeyTab:
		e.sel = min(len(e.rows)-1, e.sel+1)
	default:
		return
	}
	a.drawTabsEdit()
}

func (a *bro) tabsTap(p image.Point) {
	e := a.tabsEd
	if kobo.Hit(p, e.done) || !p.In(e.box) {
		a.closeTabs()
		return
	}
	n := len(e.rows)
	for i, r := range e.rows {
		switch {
		case i > 0 && kobo.Hit(p, e.ups[i]):
			a.moveTab(i, -1)
		case i < n-1 && kobo.Hit(p, e.downs[i]):
			a.moveTab(i, 1)
		case p.In(r):
			e.sel = i
			a.toggleTab(i)
		default:
			continue
		}
		return
	}
}

// triangle fills the triangle p, q, s.
func triangle(c kobo.Canvas, p, q, s image.Point, g uint8) {
	b := image.Rectangle{p, p.Add(image.Pt(1, 1))}.Union(image.Rectangle{q, q.Add(image.Pt(1, 1))}).Union(image.Rectangle{s, s.Add(image.Pt(1, 1))})
	edge := func(a, b, x image.Point) int { return (b.X-a.X)*(x.Y-a.Y) - (b.Y-a.Y)*(x.X-a.X) }
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			pt := image.Pt(x, y)
			e1, e2, e3 := edge(p, q, pt), edge(q, s, pt), edge(s, p, pt)
			if (e1 >= 0 && e2 >= 0 && e3 >= 0) || (e1 <= 0 && e2 <= 0 && e3 <= 0) {
				c.SetGray(x, y, color.Gray{g})
			}
		}
	}
}
