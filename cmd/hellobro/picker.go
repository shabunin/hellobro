package main

// The switcher, a fuzzy menu in the manner of rofi's combi mode: Mod+D
// or Mod+Space (dmenu's key in i3) lists the windows, the workspaces,
// the saved SSH hosts (a new terminal to one), new shells, the settings'
// windows and the commands (commands.go). Typing narrows the list, Up/Down (or Tab,
// Shift+Tab) move, Enter goes; with nothing typed, 1-9 go straight to
// that row. Esc, its key again or a tap outside close it. The page under it holds still meanwhile.

import (
	"image"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"hellobro/kobo"
	"hellobro/wm"
)

const pickRowsMax = 10

var pickRowH = 58 // sizes.go: taller for fingers

type pickEntry struct {
	kind, title, detail string
	icon                string                                 // appIcon's, for a menu of tiles
	on                  bool                                   // a tile for what's on now: dark
	stay                bool                                   // picked, the menu stays (a tick)
	hold                func()                                 // a finger held on its tile: its settings, say
	close               func()                                 // its tile's × tapped (tileClose): a workspace's
	paint               func(c kobo.Canvas, r image.Rectangle) // draws its tile, for one that isn't an icon's
	changed             func() bool                            // its tile would be drawn otherwise now (paint's)
	current             bool
	do                  func()
	score               int
}

type picker struct {
	title, hint string
	all, shown  []pickEntry
	extra       func(query string) *pickEntry // an entry made of what's typed, or nil
	query       string
	sel         int
	box         image.Rectangle    // fixed while it's up, so narrowing leaves nothing behind
	tiles       int                // a menu of tiles, this many across (0: rows)
	tileH       int                // their height (0: pickTileH's)
	wide        bool               // the whole screen, tiles as it is (wideTiles): the workspaces'
	keysOnly    bool               // no line to type in: a key typed is key's
	key         func(ch string)    // what a key typed does, keysOnly
	refresh     func()             // before the tiles are asked whether they changed
	entries     func() []pickEntry // all anew, when what they show may have changed (againPicker)
	lookedAt    time.Time          // the tiles last asked whether they changed
	ui          *pickerUI
	over        *inkOver
	drawn       bool // all of it drawn once: then only what changes
}

// tileHeight is a tile's height in p.
func (p *picker) tileHeight() int {
	if p.tileH > 0 {
		return p.tileH
	}
	return pickTileH()
}

func (a *bro) openPicker() {
	p := &picker{}
	t := a.wm.tree
	focus := a.focusedWin()
	for _, n := range t.Windows() {
		w := a.wm.wins[n]
		detail := "workspace " + wm.WorkspaceOf(n).Name
		if ws := wm.WorkspaceOf(n); ws == t.Scratchpad() {
			detail = "put away"
		}
		if s, ok := w.(*termSession); ok {
			detail += " · " + s.status(&a.tv)
		}
		p.all = append(p.all, pickEntry{kind: "window", title: w.Title(), detail: detail, current: w == focus,
			do: func() { a.wm.tree.Focus(n); a.wmApply() }})
	}
	for _, ws := range t.Workspaces() {
		p.all = append(p.all, pickEntry{kind: "workspace", title: "Workspace " + ws.Name, detail: a.keyLabels("workspace number " + ws.Name),
			current: ws == t.Current(), do: func() { a.wmRun("workspace " + strconv.Quote(ws.Name)) }})
	}
	for _, h := range a.tv.hosts {
		cmd := ""
		if h.cmd != "" {
			cmd = " · " + h.cmd
		}
		p.all = append(p.all, pickEntry{kind: "ssh", title: h.name, detail: h.String() + cmd, do: func() { a.goHost(h) }})
	}
	p.all = append(p.all,
		pickEntry{kind: "new", title: "Alpine", detail: "a shell in Alpine, in a jail", do: func() { a.goNewShell(true) }},
		pickEntry{kind: "new", title: "Kobo shell", detail: "/bin/sh on this Kobo", do: func() { a.goNewShell(false) }})
	for _, k := range guiKeys {
		p.all = append(p.all, pickEntry{kind: "window", title: guiName(k), detail: "settings", do: func() { a.openGUI(k) }})
	}
	p.all = append(p.all, pickEntry{kind: "new", title: "Notebook", detail: "a new one, to write in with the pen", do: func() { a.openNotebook("") }})
	for _, n := range noteNames() {
		p.all = append(p.all, pickEntry{kind: "notebook", title: n, detail: "written with the pen", do: func() { a.openNotebook(n) }})
	}
	for _, c := range commands {
		if c.hidden || c.when != nil && !c.when(a) {
			continue
		}
		detail := a.keyLabels(c.id)
		if c.detail != nil {
			detail = strings.TrimSuffix(c.detail(a)+" · "+detail, " · ")
		}
		p.all = append(p.all, pickEntry{kind: "command", title: c.title, detail: detail, do: func() { c.run(a) }})
	}
	p.title, p.hint = "Go to", "windows, workspaces, hosts, commands…"
	a.showPicker(p)
}

// openMenu puts up the menu with other entries: title over them, hint in
// the empty query, and extra (if not nil) making one of what's typed.
func (a *bro) openMenu(title, hint string, entries []pickEntry, extra func(string) *pickEntry) {
	a.showPicker(&picker{title: title, hint: hint, all: entries, extra: extra})
}

// openTiles puts up a menu of tiles, an icon and words each, for
// fingers: the quick menu.
func (a *bro) openTiles(title, hint string, entries []pickEntry) {
	a.showPicker(&picker{title: title, hint: hint, all: entries, tiles: 2})
}

// pickTileH is a tile's height in a menu of tiles.
func pickTileH() int { return pickRowH * 7 / 4 }

func (a *bro) showPicker(p *picker) {
	if a.picker != nil && a.picker.wide && !p.wide { // the bar back first: this one leaves it be
		a.unpick()
	}
	if a.picker != nil && a.picker.over != nil { // in its place
		a.picker.over.gone()
	}
	b := a.screenFree()
	w := min(1100, b.Dx()-80)
	n := len(p.all)
	if p.extra != nil {
		n++
	}
	h := 136 + min(max(1, n), pickRowsMax)*pickRowH + 16
	top := b.Min.Y + max(40, b.Dy()/8)
	switch {
	case p.wide: // the whole screen: as many across as makes them biggest (wideTiles)
		p.box = b
		a.Covered = true // the bar's not drawn over it
	case p.tiles > 0:
		p.tiles = max(2, min(3, w/340))
		h = 136 + (n+p.tiles-1)/p.tiles*(p.tileHeight()+12) + 16
	}
	if !p.wide {
		h = min(h, b.Max.Y-40-top)
		p.box = image.Rect((b.Dx()-w)/2, top, (b.Dx()+w)/2, top+h)
	}
	p.ui = newPickerUI(a, p)
	p.over = newOver(p.ui.Layout, w)
	p.over.at = p.box
	p.over.take = func() {
		if a.picker == p {
			p.ui.take()
		}
	}
	a.picker = p
	a.filterPicker()
	a.drawPicker()
}

// wideTiles is how many tiles across, how wide and how high n of them
// are, fit biggest in w×h: each the shape of area, a header over it.
// The width or the height may be what holds them back.
func wideTiles(n, w, h int, area image.Rectangle) (cols, tileW, tileH int) {
	aw, ah := max(1, area.Dx()), max(1, area.Dy())
	for c := 1; c <= 6; c++ {
		rows := (n + c - 1) / c
		tw := min((w-(c-1)*12)/c, ((h-rows*12)/rows-wsHeadH)*aw/ah)
		if tw > tileW {
			cols, tileW = c, tw
		}
	}
	if tileW < 100 { // too many: six across, scrolled
		cols, tileW = 6, (w-60)/6
	}
	return cols, tileW, wsHeadH + tileW*ah/aw
}

// closePicker takes the menu down; the page comes back.
func (a *bro) closePicker() {
	a.unpick()
}

// pick runs the entry shown at n.
func (a *bro) pick(n int) {
	p := a.picker
	if n < 0 || n >= len(p.shown) {
		return
	}
	e := p.shown[n]
	if e.current {
		a.closePicker()
		return
	}
	if e.stay {
		e.do()
		if a.picker == p { // its entries anew, the same lit
			if p.entries != nil {
				p.all = p.entries()
			}
			a.filterPicker()
			p.sel = n
			a.drawPicker()
		}
		return
	}
	a.unpick() // what's under the menu, whatever comes next draws over it
	e.do()
}

// unpick takes the menu down and draws what was under it: the page, or
// for one over the whole screen (the workspaces') the bar too.
func (a *bro) unpick() {
	wide := a.picker != nil && a.picker.wide
	if a.picker != nil && a.picker.over != nil {
		a.picker.over.gone()
	}
	a.picker = nil
	if wide {
		a.Covered = false
		a.DrawAll()
		return
	}
	a.DrawPage()
}

// goHost connects to h: in the launcher in focus, else in a new window.
func (a *bro) goHost(h sshHost) {
	t := a.term()
	if t == nil || !t.launcher() {
		t = &termSession{}
		a.openWin(t)
		if a.nodeOf(t) == nil { // too many
			return
		}
	}
	a.termConnect(t, h)
}

// goNewShell starts a shell (Alpine's, or the Kobo's): in the launcher in
// focus, else in a new window.
func (a *bro) goNewShell(alpine bool) {
	t := a.term()
	if t == nil || !t.launcher() {
		t = &termSession{}
		a.openWin(t)
		if a.nodeOf(t) == nil {
			return
		}
	}
	a.termStartShell(t, alpine)
}

// filterPicker narrows the list to what matches the query, best first.
func (a *bro) filterPicker() {
	p := a.picker
	p.shown = p.shown[:0]
	for _, e := range p.all {
		if e.score = fuzzy(p.query, e.title+" "+e.detail+" "+e.kind); e.score >= 0 {
			p.shown = append(p.shown, e)
		}
	}
	if p.query != "" {
		slices.SortStableFunc(p.shown, func(x, y pickEntry) int { return y.score - x.score })
	}
	if p.extra != nil {
		e := p.extra(p.query)
		// After what matches: "road" picks Road trip, ↓ makes "road".
		if !slices.ContainsFunc(p.all, func(x pickEntry) bool { return strings.EqualFold(x.title, strings.TrimSpace(p.query)) }) {
			p.shown = append(p.shown, *e)
		}
	}
	p.sel = 0
	if p.query == "" {
		for i, e := range p.shown {
			if e.current && e.kind == "session" {
				p.sel = i
			}
		}
	}
}

// fuzzy scores how well text matches q, its letters in order with gaps
// allowed; -1 if it doesn't. Runs and word starts count for more.
func fuzzy(q, text string) int {
	if q == "" {
		return 0
	}
	t := []rune(strings.ToLower(text))
	score, run, j := 0, 0, 0
	for _, c := range strings.ToLower(q) {
		if unicode.IsSpace(c) {
			continue
		}
		found := false
		for ; j < len(t); j++ {
			if t[j] != c {
				run = 0
				score--
				continue
			}
			bonus := 1
			if j == 0 || !unicode.IsLetter(t[j-1]) && !unicode.IsDigit(t[j-1]) {
				bonus += 8 // a word's start
			}
			run++
			score += bonus + 4*(run-1)
			j++
			found = true
			break
		}
		if !found {
			return -1
		}
	}
	return max(0, score+100)
}

func (a *bro) drawPicker() {
	p := a.picker
	if !p.drawn { // all of it the first time, then what changed
		p.drawn = true
		a.drawOver(p.over)
		return
	}
	p.over.pane.Win.Invalidate()
	a.overTick()
}

// pickerKey takes every key while the menu is up.
func (a *bro) pickerKey(code uint16, value int32) {
	p, m := a.picker, &a.tv.mods
	if a.termMod(code, value) || value == 0 { // releases too: a Ctrl let go of
		return
	}
	ch, printable := kobo.KeyChar(code, m.shift, m.caps)
	switch {
	case code == kobo.KeyEsc || a.keyCommand(code) == commandByID("palette"):
		a.closePicker()
		return
	case code == kobo.KeyEnter:
		a.pick(p.sel)
		return
	case p.keysOnly && printable && code != kobo.KeyTab && !m.ctrl && !m.alt:
		if p.key != nil {
			p.key(ch)
		}
		return
	case p.keysOnly && code == kobo.KeyBackspace:
		return
	case p.query == "" && p.extra == nil && ch >= "1" && ch <= "9" && !m.ctrl: // a name typed can start with one
		a.pick(int(ch[0] - '1'))
		return
	case p.tiles > 0 && (code == keyUp || code == keyDown): // tiles: a row up or down, if there's one
		to := p.sel + p.tiles
		if code == keyUp {
			to = p.sel - p.tiles
		}
		if to >= 0 && to < len(p.shown) {
			p.sel = to
		}
	case code == keyUp || code == kobo.KeyTab && m.shift || p.tiles > 0 && code == keyLeft:
		p.sel = (p.sel + len(p.shown) - 1) % max(1, len(p.shown))
	case code == keyDown || code == kobo.KeyTab || p.tiles > 0 && code == keyRight:
		p.sel = (p.sel + 1) % max(1, len(p.shown))
	case code == kobo.KeyBackspace && m.ctrl, m.ctrl && ch == "u":
		p.query = ""
		a.filterPicker()
	case code == kobo.KeyBackspace:
		if r := []rune(p.query); len(r) > 0 {
			p.query = string(r[:len(r)-1])
			a.filterPicker()
		}
	case printable && !m.ctrl && !m.alt:
		p.query += kobo.Type(ch)
		a.filterPicker()
	default:
		return
	}
	a.drawPicker()
}

// pickerTap closes the menu, tapped outside it: the card takes its
// own taps, only one outside comes here.
func (a *bro) pickerTap(pt image.Point) {
	if !pt.In(a.picker.box) {
		a.closePicker()
	}
}

// pickerTick draws again the tiles that changed under it, a few seconds
// apart at most: the workspaces' terminals as their output comes.
// againPicker makes the menu's entries anew, if it can, and draws it
// again if any reads otherwise now.
func (a *bro) againPicker() {
	p := a.picker
	if p == nil || p.entries == nil {
		return
	}
	es := p.entries()
	if slices.EqualFunc(es, p.all, func(x, y pickEntry) bool {
		return x.title == y.title && x.detail == y.detail && x.on == y.on
	}) {
		return
	}
	sel := p.sel
	p.all = es
	a.filterPicker()
	p.sel = min(sel, max(0, len(p.shown)-1))
	a.drawPicker()
}

func (a *bro) pickerTick() {
	p := a.picker
	if p == nil || p.tiles == 0 || time.Since(p.lookedAt) < 3*time.Second || a.Blitting() {
		return
	}
	p.lookedAt = time.Now()
	if p.refresh != nil {
		p.refresh()
	}
	a.repaintTiles()
}

// repaintTiles draws again the tiles whose paint would differ now.
func (a *bro) repaintTiles() {
	p := a.picker
	again := false
	for _, e := range p.shown {
		if e.changed != nil && e.paint != nil && e.changed() {
			delete(p.ui.painted, e.title)
			again = true
		}
	}
	if again {
		p.over.pane.Win.Invalidate()
	}
}
