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

// penLabel is the pen's button: what it writes, if not the plain pen.
func (a *bro) penLabel() string {
	ink, width := a.penInk(), a.penWidth()
	word := map[string]string{"black": "pen", "gray": "gray", "marker": "marker"}[ink]
	if width != "medium" {
		word = width + " " + word
	}
	return word
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

// layPenMenu lays the pen's choices under its button.
func (w *noteWin) layPenMenu(a *bro) {
	w.penOpts = nil
	i := slices.IndexFunc(w.btns, func(b noteBtn) bool { return b.id == "pen" })
	if i < 0 {
		return
	}
	x, y := w.btns[i].r.Min.X+8, w.bar.Max.Y+10
	h := w.bar.Dy() - 8
	for gi, group := range [][]string{noteWidths, noteInks} {
		if gi > 0 {
			x += 24
		}
		for _, o := range group {
			r := image.Rect(x, y, x+max(kobo.TextWidth(a.F.Small, o)+36, h+8), y+h)
			w.penOpts = append(w.penOpts, noteBtn{id: o, label: o, r: r})
			x = r.Max.X + 8
		}
	}
}

// penMenuRect is where the pen's choices are, with their frame.
func (w *noteWin) penMenuRect() image.Rectangle {
	if !w.penMenu || len(w.penOpts) == 0 {
		return image.Rectangle{}
	}
	r := w.penOpts[0].r
	for _, o := range w.penOpts {
		r = r.Union(o.r)
	}
	return r.Inset(-8).Intersect(w.paperArea())
}

// drawPenMenu draws them, the chosen black, if it's open and in clip.
func (w *noteWin) drawPenMenu(a *bro, clip image.Rectangle) {
	m := w.penMenuRect()
	if m.Empty() || !m.Overlaps(clip) {
		return
	}
	c := a.C
	c.Fill(m, kobo.White)
	c.Frame(m, 2, kobo.Black)
	for _, o := range w.penOpts {
		bg, ink := uint8(kobo.White), uint8(kobo.Black)
		if o.id == a.penWidth() || o.id == a.penInk() {
			bg, ink = kobo.Black, kobo.White
		}
		c.Fill(o.r, bg)
		c.Frame(o.r, 2, kobo.Black)
		c.Text(a.F.Small, o.r, o.label, ink, kobo.AlignCenter)
	}
}

// penMenuTap takes a tap on the pen's choices; false if it's not on them.
func (w *noteWin) penMenuTap(a *bro, p image.Point) bool {
	if !p.In(w.penMenuRect()) {
		return false
	}
	for _, o := range w.penOpts {
		if !kobo.Hit(p, o.r) {
			continue
		}
		if _, ok := noteWidthK[o.id]; ok {
			a.mem.NoteWidth = o.id
		} else {
			a.mem.NoteInk = o.id
		}
		a.mem.save()
		w.place(a, w.p) // the pen's button says it
		w.show(a)
	}
	return true
}

// closePenMenu puts the pen's choices away, if they're out.
func (w *noteWin) closePenMenu(a *bro) {
	if !w.penMenu {
		return
	}
	r := w.penMenuRect()
	w.penMenu = false
	w.redraw(a, r)
}
