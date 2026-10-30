package main

// The switcher, a fuzzy menu in the manner of rofi's combi mode: Meta+D
// or Meta+Space (dmenu's key in i3) lists the windows, the workspaces,
// the saved SSH hosts (a new terminal to one), new shells, the settings'
// windows and the commands (commands.go). Typing narrows the list, Up/Down (or Tab,
// Shift+Tab) move, Enter goes; with nothing typed, 1-9 go straight to
// that row. Esc, its key again or a tap outside close it. The page under it holds still meanwhile.

import (
	"image"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"hellobro/kobo"
	"hellobro/wm"
)

const pickRowsMax = 10

var pickRowH = 58 // sizes.go: taller for fingers

type pickEntry struct {
	kind, title, detail string
	icon                string                  // appIcon's, for a menu of tiles
	on                  bool                    // a tile for what's on now: dark
	stay                bool                    // picked, the menu stays (a tick)
	paint               func(r image.Rectangle) // draws its tile, for one that isn't an icon's
	current             bool
	do                  func()
	score               int
}

type picker struct {
	title, hint string
	all, shown  []pickEntry
	extra       func(query string) *pickEntry // an entry made of what's typed, or nil
	query       string
	sel, top    int
	box         image.Rectangle // fixed while it's up, so narrowing leaves nothing behind
	rows        []image.Rectangle
	tiles       int  // a menu of tiles, this many across (0: rows)
	tileH       int  // their height (0: pickTileH's)
	wide        bool // the screen's width, tiles as it is: the workspaces'
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
	b := a.screenFree()
	w := min(1100, b.Dx()-80)
	n := len(p.all)
	if p.extra != nil {
		n++
	}
	h := 136 + min(max(1, n), pickRowsMax)*pickRowH + 16
	top := b.Min.Y + max(40, b.Dy()/8)
	if p.wide { // tiles the screen's shape, a header over each
		w, top = b.Dx()-48, max(b.Min.Y, a.lay.Page.Min.Y)+16 // under the bar
		p.tiles = 3
		if b.Dx() > b.Dy() {
			p.tiles = 4
		}
		tw := (w - 40 - (p.tiles-1)*12) / p.tiles
		ar := a.wm.area
		p.tileH = wsHeadH + tw*ar.Dy()/max(1, ar.Dx())
	}
	if p.tiles > 0 {
		if !p.wide {
			p.tiles = max(2, min(3, w/340))
		}
		h = 136 + (n+p.tiles-1)/p.tiles*(p.tileHeight()+12) + 16
	}
	h = min(h, b.Max.Y-40-top)
	p.box = image.Rect((b.Dx()-w)/2, top, (b.Dx()+w)/2, top+h)
	a.picker = p
	a.filterPicker()
	a.drawPicker()
}

// closePicker takes the menu down; the page comes back.
func (a *bro) closePicker() {
	a.picker = nil
	a.DrawPage()
	a.drawStat(true)
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
			top := p.top
			a.filterPicker()
			p.sel, p.top = n, top
			a.drawPicker()
		}
		return
	}
	a.picker = nil
	a.DrawPage() // what's under the menu, whatever comes next draws over it
	e.do()
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
	p.sel, p.top = 0, 0
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
	p, c := a.picker, a.C
	rows := (p.box.Dy() - 152) / pickRowH
	if p.tiles > 0 {
		rows = (p.box.Dy() - 152) / (p.tileHeight() + 12) * p.tiles
	}
	step := max(1, p.tiles) // tiles scroll by rows of them
	rows = max(step, rows-rows%step)
	if p.sel < p.top {
		p.top = p.sel - p.sel%step
	}
	if p.sel >= p.top+rows {
		p.top = p.sel - p.sel%step - rows + step
	}
	c.Fill(p.box, kobo.White)
	c.Frame(p.box, 4, kobo.Black)
	head := image.Rect(p.box.Min.X+24, p.box.Min.Y+10, p.box.Max.X-24, p.box.Min.Y+60)
	c.Text(a.F.Panel, head, p.title, kobo.Black, kobo.AlignLeft)
	keys := "type to narrow · ↑ ↓ Enter · Esc closes"
	if p.tiles > 0 {
		keys = "tap one, or the arrows and Enter · Esc closes"
	}
	c.Text(a.F.Small, head, keys, kobo.Dark, kobo.AlignRight)
	in := image.Rect(p.box.Min.X+20, head.Max.Y+6, p.box.Max.X-20, head.Max.Y+64)
	c.Frame(in, 2, kobo.Black)
	q := "› " + p.query
	tr := in.Inset(4)
	tr.Min.X += 12
	c.Text(a.F.Body, tr, q, kobo.Black, kobo.AlignLeft)
	cx := tr.Min.X + kobo.TextWidth(a.F.Body, q) + 3
	c.Fill(image.Rect(cx, in.Min.Y+12, cx+3, in.Max.Y-12), kobo.Black) // the cursor
	if p.query == "" {
		c.Text(a.F.Small, tr, p.hint, kobo.Mid, kobo.AlignRight)
	}

	p.rows = p.rows[:0]
	y := in.Max.Y + 10
	if len(p.shown) == 0 {
		c.Text(a.F.Body, image.Rect(in.Min.X+16, y, in.Max.X, y+pickRowH), "nothing matches", kobo.Dark, kobo.AlignLeft)
	}
	if p.tiles > 0 {
		tw := (in.Dx() - (p.tiles-1)*12) / p.tiles
		for n := p.top; n < min(len(p.shown), p.top+rows); n++ {
			e, i := p.shown[n], n-p.top
			x, ty := in.Min.X+i%p.tiles*(tw+12), y+i/p.tiles*(p.tileHeight()+12)
			r := image.Rect(x, ty, x+tw, ty+p.tileHeight())
			p.rows = append(p.rows, r)
			num := 0
			if p.query == "" && n < 9 {
				num = n + 1
			}
			if e.paint != nil {
				e.paint(r)
			} else {
				a.drawTile(r, launchTile{name: e.title, note: e.detail, icon: e.icon, num: num}, e.on, false)
			}
			if n == p.sel { // the keys' choice: ringed, as dark is on
				a.C.Frame(r.Inset(-6), 4, kobo.Black)
			}
		}
		a.Mark(p.box, true)
		return
	}
	for n := p.top; n < min(len(p.shown), p.top+rows); n++ {
		e := p.shown[n]
		r := image.Rect(p.box.Min.X+12, y, p.box.Max.X-12, y+pickRowH)
		y += pickRowH
		p.rows = append(p.rows, r)
		fg, dim := uint8(kobo.Black), uint8(kobo.Dark)
		if n == p.sel {
			c.Fill(r, kobo.Black)
			fg, dim = kobo.White, kobo.Light
		}
		text := r.Inset(4)
		text.Min.X += 12
		if p.query == "" && p.extra == nil && n < 9 {
			c.Text(a.F.Panel, image.Rect(text.Min.X, text.Min.Y, text.Min.X+40, text.Max.Y), strconv.Itoa(n+1), fg, kobo.AlignLeft)
		}
		text.Min.X += 48
		kw := 110
		c.Text(a.F.Small, image.Rect(text.Max.X-kw, text.Min.Y, text.Max.X, text.Max.Y), e.kind, dim, kobo.AlignRight)
		text.Max.X -= kw + 16
		title := e.title
		if e.current {
			title += "  ●"
		}
		f := a.F.Body
		if kobo.IsBig() {
			f = a.F.Tab
		}
		c.Text(f, text, title, fg, kobo.AlignLeft)
		text.Min.X += kobo.TextWidth(f, title) + 28
		c.Text(a.F.Small, text, e.detail, dim, kobo.AlignLeft)
	}
	a.Mark(p.box, true)
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

// pickerTap goes to the row tapped; a tap outside closes.
func (a *bro) pickerTap(pt image.Point) {
	for n, r := range a.picker.rows {
		if pt.In(r) {
			a.pick(a.picker.top + n)
			return
		}
	}
	if !pt.In(a.picker.box) {
		a.closePicker()
	}
}
