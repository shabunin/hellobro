package main

// The launcher, a new window's (Meta+Enter): what runs here, as tiles
// for a finger, in groups: the terminals (Alpine, the Kobo's sh, the
// saved hosts, the nearby hellobros, by number as they always were),
// books and notebooks, the settings' windows, and the Kobo itself.
// Under them, the find> line, which the keys type into: it narrows the
// tiles to what matches, best first (every notebook and every book read
// too, not only the last ones), and Enter opens the first. With an @ in
// it, or nothing matching, it's ssh> again: Enter logs in to the host
// typed. A tap on it brings the keyboard on the screen if there's no
// other. The arrows go from tile to tile, Enter opens the one lit. While
// SSH logs in, or a host's line is edited, the launcher is lines of
// text again (drawConnect).

import (
	"fmt"
	"image"
	"math"
	"os"
	"path"
	"slices"
	"strings"
	"time"

	"hellobro/kobo"
)

// launchTile is an app on the launcher.
type launchTile struct {
	name, note string
	icon       string // appIcon's
	num        int    // the number key that opens it; 0 none
	do         func()
}

type launchGroup struct {
	name  string
	tiles []launchTile
}

const (
	launchTileMin   = 290             // a tile's least width; the rows take as many as fit
	launchRecentFor = 5 * time.Second // the books and notebooks last opened are looked for again after that
)

// launchMenu says whether t's launcher shows its tiles: it's waiting
// for a choice, not logging in or editing a host's line.
func launchMenu(t *termSession) bool {
	return t.state == termIdle && t.edit == nil && t.prompt == nil
}

// launchGroups are the tiles, as they are now.
func (a *bro) launchGroups(t *termSession) []launchGroup {
	v := &a.tv
	alpine := "Linux " + alpineVersion + ", in a jail"
	if _, err := os.Stat(alpineImg); err != nil {
		alpine = "set up the first time: 3 MB"
	}
	terms := []launchTile{
		{"Alpine", alpine, "alpine", 1, func() { a.termStartShell(t, true) }},
		{"Kobo shell", "/bin/sh, as " + whoami(), "shell", 2, func() { a.termStartShell(t, false) }},
	}
	num := func(n int) int {
		if n > 9 {
			return 0
		}
		return n
	}
	for i, h := range v.hosts {
		note := h.String()
		if h.cmd != "" {
			note += " ▸ " + h.cmd
		}
		if m := a.mounted(h); m != nil && m.cmd != nil {
			note = "mounted, " + mountMode(m.rw)
		}
		terms = append(terms, launchTile{h.name, note, "host", num(i + 3), func() { a.hostMenu(t, h) }})
	}
	a.nearbyLook()
	for i, p := range v.nearby.peers {
		terms = append(terms, launchTile{p.Name, a.peerNote(p), "nearby", num(len(v.hosts) + i + 3), func() { a.typeInto(t, p) }})
	}
	terms = append(terms, launchTile{"SSH key", "copy this Kobo's", "key", 0, a.copyKoboKey})

	read := []launchTile{
		{"Books", "EPUBs and PDFs", "book", 0, func() { a.replaceWin(t, a.newBookWin("")) }},
		{"Notebooks", "all, in folders", "notebooks", 0, func() { a.replaceWin(t, a.newNotesWin("")) }},
		{"New notebook", "to write in", "new", 0, func() { a.launchNotebook(t, "") }},
	}
	m := &t.pane.menu
	if m.recent == nil || time.Since(m.recentAt) > launchRecentFor { // the card's looked at, not at every key
		m.recent, m.recentAt = []launchTile{}, time.Now()
		for _, b := range a.lastBooks(2) {
			m.recent = append(m.recent, launchTile{b.name, b.note, "reading", 0, func() { a.replaceWin(t, a.newBookWin(b.key)) }})
		}
		names := noteNames()
		for _, n := range names[:min(2, len(names))] {
			note := "notebook"
			if d := path.Dir(n); d != "." {
				note = "in " + d
			}
			m.recent = append(m.recent, launchTile{path.Base(n), note, "notebook", 0, func() { a.launchNotebook(t, n) }})
		}
	}
	read = append(read, m.recent...)

	var set []launchTile
	for _, k := range guiKeys {
		set = append(set, launchTile{guiName(k), guiNotes[k], k, 0, func() { a.replaceWin(t, &guiWin{key: k}) }})
	}
	set = append(set, launchTile{"Keys", "a command's keys", "keys", 0, func() { a.openKeys(nil) }})

	self := []launchTile{
		{"Rotate", "the screen", "rotate", 0, func() { a.Rotate() }},
		{"Keyboard", "on the screen: show / hide", "keyboard", 0, a.vkToggle},
		{"Light", "on / off", "light", 0, a.toggleLight},
		{"Sleep", "", "sleep", 0, func() { a.App.Sleep() }},
		{"Lock", "the screen", "lock", 0, a.lock},
		{"Touch", "calibrate", "touch", 0, func() { a.Calibrate() }},
		{"Nickel", "quit, back to it", "quit", 0, func() {
			a.ask("Quit?", "hellobro closes, Nickel comes back", "quit", func() { a.Quit() })
		}},
	}
	gs := []launchGroup{{"Terminals", terms}, {"Read and write", read}, {"Settings", set}, {"This Kobo", self}}
	if q := launchQuery(t); q != "" {
		return a.launchFound(t, gs, q)
	}
	return gs
}

// launchQuery is what's typed in the launcher's line to look for; ""
// if nothing is, or it's a host (an @ in it).
func launchQuery(t *termSession) string {
	q := strings.TrimSpace(t.input)
	if strings.Contains(q, "@") {
		return ""
	}
	return q
}

// launchFound is the tiles that match q, best first, with every notebook
// and every book read (the groups have the last two of each).
func (a *bro) launchFound(t *termSession, gs []launchGroup, q string) []launchGroup {
	type scored struct {
		tl    launchTile
		score int
	}
	var found []scored
	seen := map[string]bool{}
	add := func(tl launchTile) {
		if s := fuzzy(q, tl.name+" "+tl.note); s >= 0 && !seen[tl.name+"\x00"+tl.note] {
			seen[tl.name+"\x00"+tl.note] = true
			found = append(found, scored{tl, s})
		}
	}
	for _, g := range gs {
		for _, tl := range g.tiles {
			if tl.icon != "reading" && tl.icon != "notebook" { // the last ones: all of them come below
				add(tl)
			}
		}
	}
	for _, n := range noteNames() {
		note := "notebook"
		if d := path.Dir(n); d != "." {
			note = "notebook in " + d
		}
		add(launchTile{path.Base(n), note, "notebook", 0, func() { a.launchNotebook(t, n) }})
	}
	for _, b := range a.lastBooks(len(a.mem.Books)) {
		add(launchTile{b.name, b.note, "reading", 0, func() { a.replaceWin(t, a.newBookWin(b.key)) }})
	}
	slices.SortStableFunc(found, func(x, y scored) int { return y.score - x.score })
	g := launchGroup{name: "nothing matches: Enter logs in to " + q + " (ssh)"}
	if len(found) > 0 {
		g.name = fmt.Sprintf("%d matching %q", len(found), q)
	}
	for _, f := range found {
		g.tiles = append(g.tiles, f.tl)
	}
	return []launchGroup{g}
}

// guiNotes say what's in each settings' window, on its tile.
var guiNotes = map[string]string{
	"wifi": "networks", "bluetooth": "keyboards, headphones", "sharing": "SFTP and SSH",
	"settings": "the rest", "battery": "how long it lasts", "usage": "what drains it",
}

// lastBooks are the n books read last, if they're still there.
func (a *bro) lastBooks(n int) []libEntry {
	var es []libEntry
	for p, pl := range a.mem.Books {
		if pl.Read == 0 {
			continue
		}
		es = append(es, libEntry{key: p, name: placeName(p, pl), note: fmt.Sprintf("%d%% read", pl.Pct), at: pl.Read})
	}
	slices.SortFunc(es, func(x, y libEntry) int { return int(y.at - x.at) })
	var out []libEntry
	for _, e := range es {
		if len(out) == n {
			break
		}
		if _, err := os.Stat(e.key); err == nil {
			out = append(out, e)
		}
	}
	return out
}

// menuLayout places groups of ns[i] tiles in rows w wide, as many to a
// row as take minW each, a heading headH high over each group: the
// headings' places, the tiles', from y 0, and how high it all is.
func menuLayout(w int, ns []int, minW, tileH, headH, gap int) (heads []image.Rectangle, tiles [][]image.Rectangle, h int) {
	cols := max(1, (w+gap)/(minW+gap))
	tw := (w - (cols-1)*gap) / cols
	y := 0
	for i, n := range ns {
		if i > 0 {
			y += 2 * gap
		}
		heads = append(heads, image.Rect(0, y, w, y+headH))
		y += headH
		var rs []image.Rectangle
		for k := range n {
			x, ty := k%cols*(tw+gap), y+k/cols*(tileH+gap)
			rs = append(rs, image.Rect(x, ty, x+tw, ty+tileH))
		}
		if rows := (n + cols - 1) / cols; rows > 0 {
			y += rows*tileH + (rows-1)*gap
		}
		tiles = append(tiles, rs)
	}
	return heads, tiles, y
}

// menuStep is the tile the arrows go to from sel (-1: none lit yet):
// dx along the tiles, dy to the nearest row above or below, the tile
// in it nearest across.
func menuStep(rs []image.Rectangle, sel, dx, dy int) int {
	switch {
	case len(rs) == 0:
		return -1
	case sel < 0 || sel >= len(rs):
		return 0
	case dx != 0:
		return min(max(sel+dx, 0), len(rs)-1)
	}
	mid := func(r image.Rectangle) image.Point { return r.Min.Add(r.Max).Div(2) }
	at := mid(rs[sel])
	best, bestY, bestX := sel, math.MaxInt, math.MaxInt
	for i, r := range rs {
		m := mid(r)
		ddy := (m.Y - at.Y) * dy
		if ddy <= 0 {
			continue
		}
		if ddx := kobo.Abs(m.X - at.X); ddy < bestY || ddy == bestY && ddx < bestX {
			best, bestY, bestX = i, ddy, ddx
		}
	}
	return best
}

// menuPlaced is how the launcher was laid out, for the keys and a
// finger: each tile, where it is in the list (not scrolled), the lit
// one, and how far it's scrolled.
type menuPlaced struct {
	sig      string // what was drawn: drawn again only when that changes
	tiles    []launchTile
	rects    []image.Rectangle
	area     image.Rectangle // the tiles' part of the panel
	h        int             // how high the tiles are, all told
	lit      int             // the tile lit (Enter opens it): the first, new; -1 none, scrolled
	drawnLit int             // the one drawn lit
	top      int             // how far down the tiles are scrolled
	asked    bool            // the ssh> line asked for the keyboard on the screen
	recent   []launchTile    // the books and notebooks last opened, as looked for
	recentAt time.Time
}

// drawMenu draws t's launcher as tiles; only what changed, if it's
// drawn already.
func (a *bro) drawMenu(t *termSession) {
	pn, c := &t.pane, a.C
	m := &pn.menu
	body := a.termPanel(t).Body
	in := body.Inset(16)
	gs := a.launchGroups(t)

	// At the bottom, what can't scroll: how the last thing ended, the
	// ssh> line, the keys.
	small := a.F.Small.Metrics().Height.Ceil() + 6
	fieldH := a.libRowH()
	notes := t.notes
	if len(notes)*small > in.Dy()/3 { // a small window: the notes are in the log
		notes = nil
	}
	bottomH := len(notes)*small + 8 + fieldH + 8 + small
	area := image.Rect(in.Min.X, in.Min.Y, in.Max.X, max(in.Min.Y, in.Max.Y-bottomH-12))
	bottom := image.Rect(in.Min.X, area.Max.Y+12, in.Max.X, max(area.Max.Y+12, in.Max.Y))

	var ns []int
	var tiles []launchTile
	for _, g := range gs {
		ns = append(ns, len(g.tiles))
		tiles = append(tiles, g.tiles...)
	}
	bodyH := a.F.Body.Metrics().Height.Ceil()
	headH := small + 10
	compact := false
	var heads []image.Rectangle
	var rects [][]image.Rectangle
	var h int
	for tileH := 150; ; tileH -= 8 { // as tall as there's room for, down to a finger's and the notes'
		heads, rects, h = menuLayout(area.Dx(), ns, launchTileMin, max(tileH, bodyH+small+34), headH, 14)
		if h <= area.Dy() || tileH <= 104 {
			break
		}
	}
	if h > area.Dy() { // no room for the notes: the names alone
		compact = true
		heads, rects, h = menuLayout(area.Dx(), ns, launchTileMin, max(72, bodyH+28), headH, 12)
	}
	more := h > area.Dy() && area.Dy() > 4*small // it scrolls: room for "more", if there's room
	if more {
		area.Min.Y += small
		area.Max.Y -= small
	}
	var flat []image.Rectangle
	for _, rs := range rects {
		flat = append(flat, rs...)
	}
	if m.lit >= len(flat) {
		m.lit = len(flat) - 1
	}
	m.top = min(max(m.top, 0), max(0, h-area.Dy()))
	if m.lit >= 0 { // the tile lit shows
		r := flat[m.lit]
		if r.Min.Y < m.top {
			m.top = r.Min.Y
		}
		if r.Max.Y > m.top+area.Dy() {
			m.top = r.Max.Y - area.Dy()
		}
	}
	var sig strings.Builder
	fmt.Fprint(&sig, body, area, compact, m.top, flat)
	for _, g := range gs {
		fmt.Fprint(&sig, "|", g.name)
		for _, tl := range g.tiles {
			fmt.Fprint(&sig, ";", tl.name, ",", tl.note, ",", tl.num)
		}
	}
	at := area.Min.Sub(image.Pt(0, m.top))
	shows := func(r image.Rectangle) bool { return r.Add(at).In(area) }

	full := pn.picks == nil || m.sig != sig.String()
	if full { // all of it
		c.Fill(body, kobo.White)
		a.Mark(body, true)
		for i, g := range gs {
			if hr := heads[i].Add(at); shows(heads[i]) {
				c.Text(a.F.Small, hr, g.name, kobo.Mid, kobo.AlignLeft)
				c.Fill(image.Rect(hr.Min.X, hr.Max.Y-4, hr.Max.X, hr.Max.Y-2), kobo.Light)
			}
		}
		if more && m.top > 0 {
			c.Text(a.F.Small, image.Rect(area.Min.X, area.Min.Y-small, area.Max.X, area.Min.Y), "↑ more: swipe down", kobo.Mid, kobo.AlignRight)
		}
		if more && h-m.top > area.Dy() {
			c.Text(a.F.Small, image.Rect(area.Min.X, area.Max.Y, area.Max.X, area.Max.Y+small), "↓ more: swipe up", kobo.Mid, kobo.AlignRight)
		}
	} else {
		c.Fill(bottom, kobo.White)
		a.Mark(bottom, true)
	}
	pn.picks = nil
	for i, r := range flat {
		if !shows(r) {
			continue
		}
		r = r.Add(at)
		if full || i != m.drawnLit != (i != m.lit) { // the lit one moved off it, or on
			if !full {
				a.Mark(r, true)
			}
			a.drawTile(r, tiles[i], i == m.lit, compact)
		}
		pn.picks = append(pn.picks, launchPick{r, a.launchDo(t, tiles[i].do)})
	}
	m.sig, m.tiles, m.rects, m.area, m.h, m.drawnLit = sig.String(), tiles, flat, area, h, m.lit

	y := bottom.Min.Y
	for _, n := range notes {
		c.Text(a.F.Small, image.Rect(bottom.Min.X, y, bottom.Max.X, y+small), n, kobo.Black, kobo.AlignLeft)
		y += small
	}
	field := image.Rect(bottom.Min.X, y+8, bottom.Max.X, y+8+fieldH)
	c.Frame(field, 2, kobo.Black)
	prompt := "find>"
	if t.input != "" && launchQuery(t) == "" || t.input != "" && len(tiles) == 0 {
		prompt = "ssh>"
	}
	label := image.Rect(field.Min.X+14, field.Min.Y, field.Min.X+14+kobo.TextWidth(a.F.Small, "find>")+12, field.Max.Y)
	c.Text(a.F.Small, label, prompt, kobo.Mid, kobo.AlignLeft)
	text := image.Rect(label.Max.X, field.Min.Y, field.Max.X-14, field.Max.Y)
	if t.input == "" {
		c.Text(a.F.Small, text, "type to find an app, a book, a notebook; user@host[:port] [command]: log in", kobo.Mid, kobo.AlignLeft)
	} else {
		c.Text(a.F.Body, text, t.input+"█", kobo.Black, kobo.AlignLeft)
	}
	pn.picks = append(pn.picks, launchPick{field, func() {
		m.asked = true
		a.vkAsk(true)
	}})
	keys := "1–9, or ↑ ↓ ← → and Enter   " + a.keyHints("launcher", "another window", "palette", "the switcher", "kill", "close it") + "   Esc: close this window"
	c.Text(a.F.Small, image.Rect(bottom.Min.X, field.Max.Y+8, bottom.Max.X, field.Max.Y+8+small), keys, kobo.Mid, kobo.AlignLeft)
}

// launchDo is do, the keyboard on the screen let go first if the ssh>
// line asked for it.
func (a *bro) launchDo(t *termSession, do func()) func() {
	return func() {
		a.launchUnask(t)
		t.input = ""
		do()
	}
}

// launchUnask lets the keyboard on the screen go, if t's ssh> line
// asked for it.
func (a *bro) launchUnask(t *termSession) {
	if t.pane.menu.asked {
		t.pane.menu.asked = false
		a.vkAsk(false)
	}
}

// drawTile draws tile tl in r, lit (black) or not; compact, its name
// alone.
func (a *bro) drawTile(r image.Rectangle, tl launchTile, lit, compact bool) {
	c := a.C
	bg, ink, gray := uint8(kobo.White), uint8(kobo.Black), uint8(kobo.Mid)
	if lit {
		bg, ink, gray = kobo.Black, kobo.White, kobo.Light
	}
	c.Fill(r, bg)
	c.Frame(r, 2, kobo.Black)
	side := min(r.Dy()-16, 80)
	ir := image.Rect(r.Min.X+8, r.Min.Y+(r.Dy()-side)/2, r.Min.X+8+side, r.Min.Y+(r.Dy()+side)/2)
	drawIcon(c, appIcon(tl.icon), ir, ink, bg)
	text := image.Rect(ir.Max.X+8, r.Min.Y+6, r.Max.X-10, r.Max.Y-6)
	name := text
	if tl.num > 0 {
		nw := kobo.TextWidth(a.F.Small, "9")
		c.Text(a.F.Small, image.Rect(r.Max.X-10-nw, r.Min.Y+6, r.Max.X-10, r.Min.Y+6+a.F.Small.Metrics().Height.Ceil()), fmt.Sprint(tl.num), gray, kobo.AlignRight)
		name.Max.X -= nw + 8
	}
	if compact || tl.note == "" {
		c.Text(a.F.Body, name, tl.name, ink, kobo.AlignLeft)
		return
	}
	mid := text.Min.Y + text.Dy()*11/20
	name.Max.Y = mid
	c.Text(a.F.Body, name, tl.name, ink, kobo.AlignLeft)
	c.Text(a.F.Small, image.Rect(text.Min.X, mid, text.Max.X, text.Max.Y-4), tl.note, gray, kobo.AlignLeft)
}

// menuKey moves the lit tile by the arrows, or opens it (Enter); false
// if code is neither, or nothing's lit for Enter.
func (a *bro) menuKey(t *termSession, code uint16) bool {
	m := &t.pane.menu
	switch code {
	case kobo.KeyEnter:
		if m.lit < 0 || m.lit >= len(m.tiles) {
			return false
		}
		a.launchDo(t, m.tiles[m.lit].do)()
		return true
	case 105: // ←
		m.lit = menuStep(m.rects, m.lit, -1, 0)
	case 106: // →
		m.lit = menuStep(m.rects, m.lit, 1, 0)
	case 103: // ↑
		m.lit = menuStep(m.rects, m.lit, 0, -1)
	case 108: // ↓
		m.lit = menuStep(m.rects, m.lit, 0, 1)
	default:
		return false
	}
	a.updateTerm(t)
	return true
}

// menuSwipe scrolls the tiles, if they don't all fit: a swipe up, on.
func (a *bro) menuSwipe(t *termSession, from, to image.Point) bool {
	m := &t.pane.menu
	d := to.Sub(from)
	if !launchMenu(t) || !from.In(m.area) || kobo.Abs(d.Y) < 60 || kobo.Abs(d.Y) < 2*kobo.Abs(d.X) {
		return false
	}
	if m.h <= m.area.Dy() {
		return true
	}
	step := max(m.area.Dy()*3/4, 1)
	if d.Y > 0 {
		step = -step
	}
	m.top += step
	m.lit = -1 // the lit one would pull it back
	a.updateTerm(t)
	return true
}

// appIcon is the picture of an app's tile, on the grid of the notebook's
// buttons (noteicons.go).
func appIcon(id string) []iconPart {
	ring := func(x, y, r, w float64) iconPart { return iconPart{kind: 'o', pts: []float64{x, y, r}, w: w} }
	dot := func(x, y, r float64) iconPart { return iconPart{kind: 'd', pts: []float64{x, y, r}} }
	box := func(x0, y0, x1, y1 float64) iconPart { return iconPart{kind: 'f', pts: []float64{x0, y0, x1, y1}} }
	switch id {
	case "alpine":
		return []iconPart{iconShut(0.9, 2, 20, 9, 7, 13, 14, 16, 10, 22, 20), iconLine(0.6, 7, 10.5, 9, 12, 11, 10.5)}
	case "shell":
		return []iconPart{iconShut(0.8, 3, 4, 21, 4, 21, 20, 3, 20), iconLine(1, 7, 9, 10, 12, 7, 15), iconLine(0.8, 12, 15.5, 17, 15.5)}
	case "host":
		return []iconPart{iconShut(0.8, 3, 4, 21, 4, 21, 16, 3, 16), iconLine(0.8, 12, 16, 12, 20), iconLine(0.8, 7, 20.5, 17, 20.5)}
	case "nearby":
		return []iconPart{iconShut(0.8, 3, 6, 12, 6, 12, 20, 3, 20), iconLine(0.6, iconArc(13, 9, 4, -60, 60)...), iconLine(0.6, iconArc(13, 9, 8, -60, 60)...), dot(7.5, 17.5, 1)}
	case "key":
		return []iconPart{ring(7.5, 12, 4, 0.8), iconLine(0.9, 11.5, 12, 21, 12), iconLine(0.8, 18, 12, 18, 16), iconLine(0.8, 15, 12, 15, 15)}
	case "book":
		return []iconPart{iconShut(0.7, 12, 6, 3, 4, 3, 18, 12, 20), iconShut(0.7, 12, 6, 21, 4, 21, 18, 12, 20)}
	case "reading":
		return []iconPart{iconShut(0.8, 5, 3, 19, 3, 19, 21, 5, 21), iconLine(0.8, 8, 3, 8, 21), iconLine(0.7, 13, 3, 13, 11, 15, 9, 17, 11, 17, 3)}
	case "notebooks":
		return []iconPart{iconShut(0.8, 6, 3, 20, 3, 20, 21, 6, 21), iconLine(0.6, 3, 7, 8, 7), iconLine(0.6, 3, 12, 8, 12), iconLine(0.6, 3, 17, 8, 17), iconLine(0.4, 11, 9, 17, 9), iconLine(0.4, 11, 13, 17, 13)}
	case "notebook":
		return []iconPart{iconShut(0.8, 5, 3, 19, 3, 19, 21, 5, 21), iconLine(0.3, 8, 8, 16, 8), iconLine(0.3, 8, 12, 16, 12), iconLine(0.3, 8, 16, 13, 16)}
	case "new":
		return []iconPart{iconShut(0.8, 5, 3, 19, 3, 19, 21, 5, 21), iconLine(1, 12, 8, 12, 16), iconLine(1, 8, 12, 16, 12)}
	case "wifi":
		return []iconPart{iconLine(0.9, iconArc(12, 19, 14, -135, -45)...), iconLine(0.9, iconArc(12, 19, 9, -135, -45)...), iconLine(0.9, iconArc(12, 19, 4, -135, -45)...), dot(12, 19, 1.6)}
	case "bluetooth":
		return []iconPart{iconLine(0.9, 7, 8, 17, 16, 12, 21, 12, 3, 17, 8, 7, 16)}
	case "sharing":
		return []iconPart{iconLine(0.9, 4, 8, 19, 8), iconHead(20, 8, 1, 0), iconLine(0.9, 20, 16, 5, 16), iconHead(4, 16, -1, 0)}
	case "settings":
		return []iconPart{iconLine(0.4, 3, 6, 21, 6), iconLine(0.4, 3, 12, 21, 12), iconLine(0.4, 3, 18, 21, 18), dot(8, 6, 2.4), dot(16, 12, 2.4), dot(11, 18, 2.4)}
	case "battery":
		return []iconPart{iconShut(0.8, 3, 7, 19, 7, 19, 17, 3, 17), box(19.5, 10, 21.5, 14), box(5, 9, 13, 15)}
	case "usage":
		return []iconPart{box(4, 14, 8, 20), box(10, 7, 14, 20), box(16, 11, 20, 20), iconLine(0.5, 2, 21, 22, 21)}
	case "keys":
		return []iconPart{ring(7, 7, 2.5, 0.7), ring(17, 7, 2.5, 0.7), ring(7, 17, 2.5, 0.7), ring(17, 17, 2.5, 0.7), iconShut(0.7, 9.5, 9.5, 14.5, 9.5, 14.5, 14.5, 9.5, 14.5)}
	case "keyboard":
		ps := []iconPart{iconShut(0.8, 2, 6, 22, 6, 22, 18, 2, 18), iconLine(0.8, 8, 15, 16, 15)}
		for x := 5.0; x < 20; x += 3.5 {
			ps = append(ps, box(x, 9, x+1.6, 10.6))
		}
		return ps
	case "rotate":
		return noteIcon("turn")
	case "light":
		ps := []iconPart{ring(12, 12, 4, 0.8)}
		for k := range 8 {
			t := float64(k) * math.Pi / 4
			ps = append(ps, iconLine(0.7, 12+7*math.Cos(t), 12+7*math.Sin(t), 12+10*math.Cos(t), 12+10*math.Sin(t)))
		}
		return ps
	case "sleep":
		return []iconPart{dot(11, 13, 8), {kind: 'b', pts: []float64{16, 9, 7}}}
	case "lock":
		return []iconPart{iconLine(0.9, iconArc(12, 11, 5, 180, 360)...), box(5, 11, 19, 21), {kind: 'b', pts: []float64{12, 15.5, 1.6}}}
	case "touch":
		return []iconPart{ring(12, 12, 6, 0.6), iconLine(0.6, 12, 2, 12, 22), iconLine(0.6, 2, 12, 22, 12)}
	case "airplane": // from above, its nose up
		return []iconPart{iconLine(1.2, 12, 2.5, 12, 19), iconShut(0.7, 12, 8.5, 21.5, 14, 21.5, 16, 12, 12.5, 2.5, 16, 2.5, 14), iconLine(0.8, 8, 21.5, 12, 18.5, 16, 21.5)}
	case "ticked":
		return []iconPart{iconShut(0.8, 3, 3, 21, 3, 21, 21, 3, 21), iconLine(1.4, 6.5, 12.5, 10.5, 16.5, 17.5, 7.5)}
	case "unticked":
		return []iconPart{iconShut(0.8, 3, 3, 21, 3, 21, 21, 3, 21)}
	case "workspaces": // four screens, one with windows
		return []iconPart{iconShut(0.7, 2, 3, 11, 3, 11, 11, 2, 11), iconShut(0.7, 13, 3, 22, 3, 22, 11, 13, 11),
			iconShut(0.7, 2, 13, 11, 13, 11, 21, 2, 21), iconShut(0.7, 13, 13, 22, 13, 22, 21, 13, 21), box(4, 5, 9, 9)}
	case "switcher": // a list, one picked
		return []iconPart{iconShut(0.8, 3, 4, 21, 4, 21, 20, 3, 20), box(5, 7, 19, 11), iconLine(0.5, 6, 14, 18, 14), iconLine(0.5, 6, 17, 18, 17)}
	case "quit":
		return []iconPart{iconLine(0.9, iconArc(12, 13, 8, -55, 235)...), iconLine(1, 12, 3, 12, 12)}
	}
	return nil
}
