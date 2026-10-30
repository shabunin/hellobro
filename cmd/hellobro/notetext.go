package main

// Text on a notebook's page, typed from a keyboard. The text tool (T)
// puts a cursor where the page is tapped, or in the text tapped; what's
// typed goes in there (Enter: a new line; the arrows, Home and End move
// along it), and Esc, another tool, a tap elsewhere or the page turned
// leaves it. A text is a stroke of a kind (noteStroke.txt), as a
// picture is: taken, moved, sized and deleted with the select tool,
// undone as one; the eraser leaves it, as in Xournal++, where it's a
// <text> with its font, its size in points and its top-left corner.

import (
	"encoding/xml"
	"image"
	"log"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"

	"hellobro/kobo"
)

// noteText is a text on a page: in its stroke's box, from its top left.
type noteText struct {
	s     string
	size  float64    // points
	attrs []xml.Attr // from its file: the font, the color… (nil: ours)
}

const noteTextSize = 14.0 // points, ours: Xournal++'s 12 is small on the Kobo

// noteTyping is a text being typed: its stroke on page, shown there
// while it is, and the one it was (nil: a new one).
type noteTyping struct {
	page int
	s    *noteStroke
	was  *noteStroke
	pos  int // the cursor: a byte in the text
}

// noteTextFaces are the text's font at each size it's drawn (px).
var noteTextFaces = map[int]font.Face{}

func noteTextFace(px int) font.Face {
	px = max(4, px)
	f, ok := noteTextFaces[px]
	if !ok {
		f = kobo.Face(goregular.TTF, float64(px))
		noteTextFaces[px] = f
	}
	return f
}

// textStroke is t with its top left at (page units).
func textStroke(t *noteText, at image.Point) *noteStroke {
	s := &noteStroke{txt: t, box: image.Rectangle{Min: at, Max: at}}
	s.bound()
	return s
}

// px is the text's size in page units.
func (t *noteText) px() float64 { return t.size / notePt72 }

// textBox is how much room t takes from at (page units): its widest line
// by its lines (a cursor's width, empty).
func (t *noteText) box(at image.Point) image.Rectangle {
	f := noteTextFace(int(t.px() + 0.5))
	lines := strings.Split(t.s, "\n")
	wd := 2
	for _, l := range lines {
		wd = max(wd, font.MeasureString(f, l).Ceil())
	}
	return image.Rect(at.X, at.Y, at.X+wd, at.Y+len(lines)*f.Metrics().Height.Ceil())
}

// drawNoteText draws s's text where its box is, k screen px a page
// unit from org; the cursor at pos, if it's >= 0.
func drawNoteText(c kobo.Canvas, s *noteStroke, org image.Point, k float64, pos int, clip image.Rectangle) {
	t := s.txt
	f := noteTextFace(int(t.px()*k + 0.5))
	m := f.Metrics()
	lh := m.Height.Ceil()
	x := org.X + int(float64(s.box.Min.X)*k+0.5)
	y := org.Y + int(float64(s.box.Min.Y)*k+0.5)
	at := 0 // the byte each line starts at
	for i, l := range strings.Split(t.s, "\n") {
		top := y + i*lh
		c.TextAt(f, x, top+m.Ascent.Ceil(), l, s.gray, clip)
		if pos >= at && pos <= at+len(l) {
			cx := x + font.MeasureString(f, l[:pos-at]).Round()
			c.Fill(image.Rect(cx, top, cx+max(2, lh/12), top+lh).Intersect(clip), kobo.Black)
		}
		at += len(l) + 1
	}
}

// textAt starts typing at p (page units) on page: in the text there, or
// a new one from there.
func (w *noteWin) textAt(a *bro, page int, p image.Point) {
	w.endTyping(a)
	var hit *noteStroke
	for _, s := range w.strokes(page) {
		if s.txt != nil && p.In(s.box.Inset(-8)) {
			hit = s
		}
	}
	k := &noteTyping{page: page, was: hit}
	if hit != nil { // a copy, typed in; the old one comes back if nothing changes
		k.s = textStroke(&noteText{s: hit.txt.s, size: hit.txt.size, attrs: hit.txt.attrs}, hit.box.Min)
		k.s.gray = hit.gray
		k.pos = len(hit.txt.s)
		pg := w.nb.pages[page]
		for i, s := range pg {
			if s == hit {
				pg[i] = k.s
			}
		}
	} else {
		for len(w.nb.pages) <= page {
			w.nb.pages = append(w.nb.pages, nil)
		}
		t := &noteText{size: noteTextSize}
		k.s = textStroke(t, p.Sub(image.Pt(0, int(t.px())/2))) // the line's middle where it's tapped
		w.nb.pages[page] = append(w.nb.pages[page], k.s)
	}
	w.typing = k
	a.exAsk(true) // the on-screen keyboard, if there's no other
	w.typed(a, k.s.box)
}

// endTyping leaves the text being typed: kept as a change to undo, or,
// emptied, gone; unchanged, as it was.
func (w *noteWin) endTyping(a *bro) {
	k := w.typing
	if k == nil {
		return
	}
	w.typing = nil
	a.exAsk(false)
	pg := w.nb.pages[k.page]
	put := func(s *noteStroke) { // in k.s's place
		for i := range pg {
			if pg[i] == k.s {
				pg[i] = s
			}
		}
	}
	switch {
	case k.was != nil && k.s.txt.s == k.was.txt.s:
		put(k.was)
	case strings.TrimSpace(k.s.txt.s) == "":
		w.nb.pages[k.page] = slices.DeleteFunc(pg, func(s *noteStroke) bool { return s == k.s })
		if k.was != nil {
			w.edited(noteOp{page: k.page, del: []*noteStroke{k.was}})
		}
	default:
		op := noteOp{page: k.page, add: []*noteStroke{k.s}}
		if k.was != nil {
			op.del = []*noteStroke{k.was}
		}
		w.edited(op)
		log.Printf("notebook %s: a text on page %d", w.nb.name, k.page+1)
	}
	w.typed(a, k.s.box)
}

// typeKey types a key into the text being typed; false if it isn't one
// for it (Ctrl's go on as they would, after the typing's left).
func (w *noteWin) typeKey(a *bro, code uint16, ch string) bool {
	k := w.typing
	if a.tv.mods.ctrl || a.tv.mods.alt {
		w.endTyping(a)
		return false
	}
	t := k.s.txt
	line := func() (start, end int) {
		start = strings.LastIndexByte(t.s[:k.pos], '\n') + 1
		end = len(t.s)
		if i := strings.IndexByte(t.s[k.pos:], '\n'); i >= 0 {
			end = k.pos + i
		}
		return
	}
	switch code {
	case kobo.KeyEsc:
		w.endTyping(a)
		return true
	case kobo.KeyEnter:
		ch = "\n"
	case kobo.KeyTab:
		ch = "    "
	case kobo.KeyBackspace:
		if k.pos > 0 {
			_, n := utf8.DecodeLastRuneInString(t.s[:k.pos])
			t.s = t.s[:k.pos-n] + t.s[k.pos:]
			k.pos -= n
		}
		ch = ""
	case keyDelete:
		if k.pos < len(t.s) {
			_, n := utf8.DecodeRuneInString(t.s[k.pos:])
			t.s = t.s[:k.pos] + t.s[k.pos+n:]
		}
		ch = ""
	case keyLeft:
		if k.pos > 0 {
			_, n := utf8.DecodeLastRuneInString(t.s[:k.pos])
			k.pos -= n
		}
		ch = ""
	case keyRight:
		if k.pos < len(t.s) {
			_, n := utf8.DecodeRuneInString(t.s[k.pos:])
			k.pos += n
		}
		ch = ""
	case keyHome:
		k.pos, _ = line()
		ch = ""
	case keyEnd:
		_, k.pos = line()
		ch = ""
	case keyUp, keyDown: // the line before or after, as far along as it goes
		start, end := line()
		col := utf8.RuneCountInString(t.s[start:k.pos])
		var from, to int
		if code == keyUp {
			if start == 0 {
				return true
			}
			to = start - 1
			from = strings.LastIndexByte(t.s[:to], '\n') + 1
		} else {
			if end == len(t.s) {
				return true
			}
			from = end + 1
			to = len(t.s)
			if i := strings.IndexByte(t.s[from:], '\n'); i >= 0 {
				to = from + i
			}
		}
		k.pos = from
		for i := 0; i < col && k.pos < to; i++ {
			_, n := utf8.DecodeRuneInString(t.s[k.pos:])
			k.pos += n
		}
		ch = ""
	default:
		if ch == "" {
			return true // a key that types nothing: not the page's while typing
		}
	}
	if ch != "" {
		t.s = t.s[:k.pos] + ch + t.s[k.pos:]
		k.pos += len(ch)
	}
	before := k.s.box
	k.s.bound()
	w.typed(a, before.Union(k.s.box))
	return true
}

// typed draws the page again where the text was or is (page units).
func (w *noteWin) typed(a *bro, box image.Rectangle) {
	if !a.shows(w) || w.overview {
		return
	}
	k := w.typing
	page := -1
	if k != nil {
		page = k.page
	}
	for i, slot := range w.slots {
		if page >= 0 && w.cur+i != page {
			continue
		}
		r := w.toScreenRect(box.Inset(-12), slot).Intersect(slot)
		if r.Empty() {
			continue
		}
		w.drawPages(a, r)
		a.Mark(r, false)
	}
}

// xoppText is s as a .xopp's text.
func (s *noteStroke) xoppText() xoppTextEl {
	t := s.txt
	as := t.attrs
	if as == nil {
		as = []xml.Attr{{Name: xml.Name{Local: "font"}, Value: "Sans"}, {Name: xml.Name{Local: "color"}, Value: "#000000ff"}}
	}
	as = append([]xml.Attr(nil), as...)
	as = setAttr(as, "size", fmtPt(t.size))
	as = setAttr(as, "x", fmtPt(float64(s.box.Min.X)*notePt72))
	as = setAttr(as, "y", fmtPt(float64(s.box.Min.Y)*notePt72))
	return xoppTextEl{Attrs: as, Data: t.s}
}

// xoppToText is a .xopp's text as a stroke: where it is, its size.
func xoppToText(x xoppTextEl) *noteStroke {
	num := func(name string, def float64) float64 {
		v, err := strconv.ParseFloat(attr(x.Attrs, name), 64)
		if err != nil {
			return def
		}
		return v
	}
	var as []xml.Attr
	for _, a := range x.Attrs {
		if n := a.Name.Local; n != "x" && n != "y" && n != "size" {
			as = append(as, a)
		}
	}
	t := &noteText{s: x.Data, size: max(1, num("size", 12)), attrs: as}
	return textStroke(t, image.Pt(int(num("x", 0)/notePt72+0.5), int(num("y", 0)/notePt72+0.5)))
}
