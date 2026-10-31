package main

// The notebook's pens: fine, medium or bold; black, gray, or a marker (a
// highlighter, wide, light, under the ink). Tapping "pen" while it's in
// hand opens them under it; the choice is kept (hellobro.json). In the
// .xopp a gray pen is a pen of that color, the marker Xournal++'s
// highlighter (yellow there); a stroke of another color is drawn in the
// gray it is, dark enough to see.

import (
	"encoding/xml"
	"image"
	"slices"
	"strconv"
	"strings"
	"time"

	"hellobro/kobo"
)

const (
	noteGray   = 0x80 // the gray pen
	noteMarker = 0xC8 // the marker, at its darkest
	noteMarkR  = 160  // the marker's radius, tenths of a unit: 3.5 mm wide
)

var (
	noteWidths = []string{"fine", "medium", "bold"}
	noteInks   = []string{"black", "gray", "marker"}
	noteWidthK = map[string]float64{"fine": 0.55, "medium": 1, "bold": 1.8}
)

// penWidth and penInk are the pen as last chosen.
func (a *bro) penWidth() string {
	if _, ok := noteWidthK[a.mem.NoteWidth]; ok {
		return a.mem.NoteWidth
	}
	return "medium"
}

func (a *bro) penInk() string {
	if strings.Contains(strings.Join(noteInks, " "), a.mem.NoteInk) && a.mem.NoteInk != "" {
		return a.mem.NoteInk
	}
	return "black"
}

// drawPenSample draws a line as the pen writes now, at the foot of its
// button r: its width and its gray.
func (w *noteWin) drawPenSample(a *bro, r image.Rectangle, ink, bg uint8) {
	half := map[string]float64{"fine": 0.5, "medium": 0.9, "bold": 1.5}[a.penWidth()]
	g := ink
	switch a.penInk() {
	case "gray":
		g = noteGray
	case "marker":
		g, half = noteMarker, 2.2
	}
	drawIcon(a.C, []iconPart{iconLine(half, 4, 23, 20, 23)}, r, g, bg)
}

// newStroke is a stroke as the pen writes it now.
func (a *bro) newStroke() *noteStroke {
	s := &noteStroke{}
	at := func(n, v string) xml.Attr { return xml.Attr{Name: xml.Name{Local: n}, Value: v} }
	switch a.penInk() {
	case "gray":
		s.gray = noteGray
		s.attrs = []xml.Attr{at("tool", "pen"), at("color", "#808080ff"), at("capStyle", "round")}
	case "marker":
		s.gray, s.marker = noteMarker, true
		s.attrs = []xml.Attr{at("tool", "highlighter"), at("color", "#ffff0080"), at("capStyle", "round")}
	}
	return s
}

// penRadius is the pen's radius (tenths of a unit) pressing frac of its
// hardest; a marker's doesn't change with it.
func (a *bro) penRadius(frac float64) int16 {
	k := noteWidthK[a.penWidth()]
	if a.penInk() == "marker" {
		return int16(noteMarkR * k)
	}
	return int16(max(5, (15+25*frac)*k)) // 1.5 to 4 units, medium
}

// colorGray is the gray a .xopp's color (#rrggbbaa) is drawn in: a pen's
// dark enough to read, a marker's light, under the ink.
func colorGray(c string, marker bool) uint8 {
	h := strings.TrimPrefix(c, "#")
	v, err := strconv.ParseUint(h[:min(6, len(h))], 16, 32)
	if err != nil || len(h) < 6 {
		if marker {
			return noteMarker
		}
		return kobo.Black
	}
	r, g, b := v>>16&0xff, v>>8&0xff, v&0xff
	lum := uint8((299*r + 587*g + 114*b) / 1000)
	switch {
	case marker:
		return max(0x90, min(lum, noteMarker))
	case lum < 0x30:
		return kobo.Black
	}
	return min(lum, 0xA0)
}

// optGroups are a tool's choices, in groups: the pen's widths and inks,
// the papers.
func optGroups(tool string) [][]string {
	if tool == "paper" {
		return [][]string{strings.Fields(notePapers)}
	}
	return [][]string{noteWidths, noteInks}
}

// layOpts lays the choices of the tool w.optsOf says (the pen's, the
// paper's) under its button, or right of it down the side; a row too
// long for the window goes on under.
func (w *noteWin) layOpts(a *bro) {
	w.opts = nil
	tool := w.optsOf
	if tool == "" {
		tool = "pen" // laid out anyway: the tests', and a tap's
	}
	i := slices.IndexFunc(w.btns, func(b barBtn) bool { return b.id == tool })
	if i < 0 {
		return
	}
	pen := w.btns[i].r
	x0, y, h := pen.Min.X+8, w.bar.Max.Y+10, pen.Dy()
	if w.side {
		x0, y = w.bar.Max.X+10, pen.Min.Y
	}
	end := w.paperArea().Max.X - 8
	width := func(o string) int {
		if tool == "paper" { // its picture, then its words
			return h + kobo.TextWidth(a.F.Small, paperWords[o]) + 20
		}
		return max(kobo.TextWidth(a.F.Small, o)+36, h+8)
	}
	all := 0
	for gi, group := range optGroups(tool) {
		all += 24 * min(gi, 1)
		for _, o := range group {
			all += width(o) + 8
		}
	}
	if x0+all > end && !w.side { // further left, to be one row if the window's wide enough
		x0 = max(w.paperArea().Min.X+8, end-all)
	}
	x := x0
	for gi, group := range optGroups(tool) {
		if gi > 0 {
			x += 24
		}
		for _, o := range group {
			label := o
			if tool == "paper" {
				label = paperWords[o]
			}
			wd := width(o)
			if x+wd > end && x > x0 {
				x, y = x0, y+h+8
			}
			r := image.Rect(x, y, x+wd, y+h)
			w.opts = append(w.opts, barBtn{id: o, label: label, r: r})
			x = r.Max.X + 8
		}
	}
}

// optsRect is where the choices are, with their frame, if they're out.
func (w *noteWin) optsRect() image.Rectangle {
	if w.optsOf == "" || len(w.opts) == 0 {
		return image.Rectangle{}
	}
	r := w.opts[0].r
	for _, o := range w.opts {
		r = r.Union(o.r)
	}
	return r.Inset(-8).Intersect(w.paperArea())
}

// drawOpts draws them, the chosen black, if they're out and in clip; a
// paper's on itself.
func (w *noteWin) drawOpts(a *bro, clip image.Rectangle) {
	m := w.optsRect()
	if m.Empty() || !m.Overlaps(clip) {
		return
	}
	c := a.C
	c.Fill(m, kobo.White)
	c.Frame(m, 2, kobo.Black)
	for _, o := range w.opts {
		bg, ink := uint8(kobo.White), uint8(kobo.Black)
		if o.id == a.penWidth() || o.id == a.penInk() || o.id == w.nb.paper {
			bg, ink = kobo.Black, kobo.White
		}
		c.Fill(o.r, bg)
		c.Frame(o.r, 2, kobo.Black)
		if w.optsOf == "paper" { // its picture, then its words
			pic := image.Rect(o.r.Min.X, o.r.Min.Y, o.r.Min.X+o.r.Dy(), o.r.Max.Y)
			drawIcon(c, paperIcon(o.id), pic, ink, bg)
			c.Text(a.F.Small, image.Rect(pic.Max.X, o.r.Min.Y, o.r.Max.X-8, o.r.Max.Y), o.label, ink, kobo.AlignLeft)
			continue
		}
		c.Text(a.F.Small, o.r, o.label, ink, kobo.AlignCenter)
	}
}

// optsTap takes a tap on the choices; false if it's not on them.
func (w *noteWin) optsTap(a *bro, p image.Point) bool {
	if !p.In(w.optsRect()) {
		return false
	}
	for _, o := range w.opts {
		if !kobo.Hit(p, o.r) {
			continue
		}
		switch {
		case w.optsOf == "paper":
			w.nb.paper = o.id
			w.changed = time.Now()
		case noteWidthK[o.id] > 0:
			a.mem.NoteWidth = o.id
		default:
			a.mem.NoteInk = o.id
		}
		a.mem.save()
		w.place(a, w.p) // the tool's button says it
		w.show(a)
	}
	return true
}

// toggleOpts puts tool's choices out, or away if they're out.
func (w *noteWin) toggleOpts(a *bro, tool string) {
	if w.optsOf == tool {
		w.closeOpts(a)
		return
	}
	w.closeOpts(a)
	w.optsOf = tool
	w.layOpts(a)
	w.show(a)
}

// closeOpts puts the choices away, if they're out.
func (w *noteWin) closeOpts(a *bro) {
	if w.optsOf == "" {
		return
	}
	r := w.optsRect()
	w.optsOf = ""
	w.redraw(a, r)
}
