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

var pickRowH = 58 // HELLOBRO_TOUCH=big: taller, for fingers

type pickEntry struct {
	kind, title, detail string
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

func (a *bro) showPicker(p *picker) {
	b := a.screenFree()
	w := min(1100, b.Dx()-80)
	n := len(p.all)
	if p.extra != nil {
		n++
	}
	h := 136 + min(max(1, n), pickRowsMax)*pickRowH + 16
	top := b.Min.Y + max(40, b.Dy()/8)
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
	if p.sel < p.top {
		p.top = p.sel
	}
	if p.sel >= p.top+rows {
		p.top = p.sel - rows + 1
	}
	c.Fill(p.box, kobo.White)
	c.Frame(p.box, 4, kobo.Black)
	head := image.Rect(p.box.Min.X+24, p.box.Min.Y+10, p.box.Max.X-24, p.box.Min.Y+60)
	c.Text(a.F.Panel, head, p.title, kobo.Black, kobo.AlignLeft)
	c.Text(a.F.Small, head, "type to narrow · ↑ ↓ Enter · Esc closes", kobo.Dark, kobo.AlignRight)
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
	case code == keyUp || code == kobo.KeyTab && m.shift:
		p.sel = (p.sel + len(p.shown) - 1) % max(1, len(p.shown))
	case code == keyDown || code == kobo.KeyTab:
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
		p.query += ch
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
