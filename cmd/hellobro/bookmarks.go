package main

// Bookmarks, and the strip under a book's page: its buttons for the
// contents (☰), the style panel (Aa) and a bookmark (a ribbon, filled
// when the page has one). A bookmark is a place in a chapter, as the
// place a book was left is; the contents' second tab lists them.

import (
	"cmp"
	"fmt"
	"image"
	"log"
	"slices"
	"strings"
	"time"

	"hellobro/book"
	"hellobro/kobo"
)

// bookMark is a bookmark: where, and what the page starts with.
type bookMark struct {
	Spine int    `json:"spine"`
	Block int    `json:"block"`
	Off   int    `json:"off"`
	Pct   int    `json:"pct"`
	Text  string `json:"text,omitempty"`
	At    int64  `json:"at"` // when it was set, Unix seconds
}

func (m bookMark) pos() book.Pos { return book.Pos{Block: m.Block, Off: m.Off} }

// pageSpan is where the page shown starts, and where the next one does.
func (w *bookWin) pageSpan() (from, to book.Pos) {
	from = w.pages[w.page].Start
	to = book.Pos{Block: 1 << 30}
	if w.page+1 < len(w.pages) {
		to = w.pages[w.page+1].Start
	}
	return from, to
}

// pageMarks are the bookmarks on the page shown, by index.
func (w *bookWin) pageMarks() []int {
	if w.b == nil || w.page >= len(w.pages) {
		return nil
	}
	from, to := w.pageSpan()
	var on []int
	for i, m := range w.marks {
		if p := m.pos(); m.Spine == w.spine && !p.Before(from) && p.Before(to) {
			on = append(on, i)
		}
	}
	return on
}

// toggleMark bookmarks the page shown, or takes its bookmarks away.
func (w *bookWin) toggleMark(a *bro) {
	if w.b == nil || w.page >= len(w.pages) {
		return
	}
	if on := w.pageMarks(); len(on) > 0 {
		for _, i := range slices.Backward(on) {
			w.marks = slices.Delete(w.marks, i, i+1)
		}
		a.Notify("bookmark taken away")
	} else {
		from, _ := w.pageSpan()
		w.marks = append(w.marks, bookMark{Spine: w.spine, Block: from.Block, Off: from.Off, Pct: w.pct(), Text: w.pageWords(80), At: time.Now().Unix()})
		slices.SortFunc(w.marks, func(x, y bookMark) int {
			return cmp.Or(x.Spine-y.Spine, x.Block-y.Block, x.Off-y.Off)
		})
		a.Notify("bookmarked")
	}
	w.save(a)
	w.draw(a)
}

// pageWords is how the page starts, n bytes or so of it: the chapter's
// text from there, not the page's words, which hyphens and styles cut.
func (w *bookWin) pageWords(n int) string {
	if w.pdf != nil {
		return w.pdfWords(n)
	}
	var b strings.Builder
	at := w.pages[w.page].Start
	for bi := at.Block; bi < len(w.ch.Blocks) && b.Len() < 2*n; bi++ {
		var t strings.Builder
		for _, sp := range w.ch.Blocks[bi].Spans {
			t.WriteString(sp.Text)
		}
		s := t.String()
		if bi == at.Block && at.Off <= len(s) {
			s = s[at.Off:]
		}
		b.WriteString(s + " ")
	}
	s := strings.Join(strings.Fields(strings.ReplaceAll(b.String(), "\u00ad", "")), " ")
	if len(s) > n {
		s = strings.ToValidUTF8(s[:n], "") + "…"
	}
	return s
}

// markLines are the bookmarks as the list shows them.
func (w *bookWin) markLines() []string {
	var ls []string
	for _, m := range w.marks {
		s, text := fmt.Sprintf("%d%%", m.Pct), m.Text
		if t := w.tocName(m.Spine); t != "" {
			s += "  ·  " + t
			if len(text) >= len(t) && strings.EqualFold(text[:len(t)], t) { // a page that starts with the heading
				text = strings.TrimSpace(text[len(t):])
			}
		}
		ls = append(ls, s+"  ·  "+text)
	}
	return ls
}

// tocName is the contents' line for chapter spine: the first of its
// lines, or the last chapter's before it that has one.
func (w *bookWin) tocName(spine int) string {
	name, at := "", -1
	for _, e := range w.b.TOC {
		if e.Spine > spine {
			break
		}
		if e.Spine != at {
			name, at = e.Title, e.Spine
		}
	}
	return name
}

// goToMark shows bookmark i's page.
func (w *bookWin) goToMark(a *bro, i int) {
	m := w.marks[i]
	w.list = false
	w.goTo(m.Spine, m.pos())
	w.draw(a)
}

// deleteMark takes bookmark i away, from the list.
func (w *bookWin) deleteMark(a *bro, i int) {
	if i < 0 || i >= len(w.marks) {
		return
	}
	w.marks = slices.Delete(w.marks, i, i+1)
	w.save(a)
	w.draw(a)
}

// --- the strip under the page ---

const footButton = 100 // a button's width

// footButtons are the strip's buttons: the contents, the style, a
// bookmark, and in a PDF the select tool (bookcut.go) and the pen, to
// write on it (notepdf.go), side by side in its middle; narrower in a
// narrow window, room left for the chapter and how far in.
func (w *bookWin) footButtons() (toc, style, mark, sel, write image.Rectangle) {
	foot := image.Rect(w.area.Min.X, w.area.Max.Y, w.area.Max.X, w.p.Body.Max.Y)
	n := 3
	if w.pdf != nil {
		n = 5
	}
	bw := max(56, min(footButton, (foot.Dx()-2*90)/n))
	x := (foot.Min.X+foot.Max.X)/2 - n*bw/2
	bs := make([]image.Rectangle, 5)
	for i := range n {
		bs[i] = image.Rect(x+i*bw, foot.Min.Y, x+(i+1)*bw, foot.Max.Y)
	}
	return bs[0], bs[1], bs[2], bs[3], bs[4]
}

// drawFoot draws the strip: the chapter, the buttons, how far in.
func (w *bookWin) drawFoot(a *bro) {
	c := a.C
	toc, style, mark, sel, write := w.footButtons()
	foot := image.Rect(w.area.Min.X, w.area.Max.Y, w.area.Max.X, w.p.Body.Max.Y-4)
	last := mark.Union(sel).Union(write)
	name := w.chapterName()
	if w.pdf != nil {
		name = w.pdfFootName()
	}
	c.Text(a.F.Small, image.Rect(foot.Min.X, foot.Min.Y, toc.Min.X-12, foot.Max.Y), name, kobo.Mid, kobo.AlignLeft)
	c.Text(a.F.Small, image.Rect(last.Max.X+12, foot.Min.Y, foot.Max.X, foot.Max.Y), fmt.Sprintf("%d%%", w.pct()), kobo.Mid, kobo.AlignRight)
	// ☰: three lines
	m := toc.Min.Add(toc.Max).Div(2)
	for k := -1; k <= 1; k++ {
		c.Fill(image.Rect(m.X-14, m.Y+k*9-2, m.X+14, m.Y+k*9+1), kobo.Dark)
	}
	c.Text(a.F.Body, style, "Aa", kobo.Dark, kobo.AlignCenter)
	m = mark.Min.Add(mark.Max).Div(2)
	ribbon(c, image.Rect(m.X-10, m.Y-14, m.X+10, m.Y+14), len(w.pageMarks()) > 0, kobo.Dark)
	if !sel.Empty() { // the select tool: a rectangle with its corners' dots, black while it's in hand
		ink := uint8(kobo.Dark)
		if w.selecting {
			c.Fill(sel.Inset(8), kobo.Black)
			ink = kobo.White
		}
		m = sel.Min.Add(sel.Max).Div(2)
		r := image.Rect(m.X-18, m.Y-12, m.X+18, m.Y+12)
		c.Frame(r, 2, ink)
		for _, p := range []image.Point{r.Min, {r.Max.X - 1, r.Min.Y}, r.Max.Sub(image.Pt(1, 1)), {r.Min.X, r.Max.Y - 1}} {
			c.Disc(p, 4, ink, sel)
		}
	}
	if !write.Empty() { // a pen, slanted: to write on the page
		m = write.Min.Add(write.Max).Div(2)
		c.Line(image.Pt(m.X-9, m.Y+9), image.Pt(m.X+11, m.Y-11), 4, kobo.Dark, write)
		c.Line(image.Pt(m.X-15, m.Y+15), image.Pt(m.X-9, m.Y+9), 1, kobo.Dark, write)
		c.Fill(image.Rect(m.X-16, m.Y+17, m.X+6, m.Y+19), kobo.Dark)
	}
}

// drawPageMark hangs a ribbon in the top margin at the right, if the
// page is bookmarked.
func (w *bookWin) drawPageMark(a *bro) {
	if len(w.pageMarks()) == 0 {
		return
	}
	top := w.p.Body.Min.Y
	ribbon(a.C, image.Rect(w.area.Max.X-22, top, w.area.Max.X, max(top+16, w.area.Min.Y-6)), true, kobo.Black)
}

// ribbon draws a bookmark's ribbon in r, a notch cut in its foot; filled
// or an outline.
func ribbon(c kobo.Canvas, r image.Rectangle, filled bool, g uint8) {
	shape := func(r image.Rectangle, g uint8) {
		notch := r.Dy() / 3
		cx := (r.Min.X + r.Max.X) / 2
		for y := r.Min.Y; y < r.Max.Y; y++ {
			cut := 0
			if d := y - (r.Max.Y - notch); d > 0 {
				cut = d * r.Dx() / 2 / notch
			}
			if cut == 0 {
				c.Fill(image.Rect(r.Min.X, y, r.Max.X, y+1), g)
				continue
			}
			c.Fill(image.Rect(r.Min.X, y, cx-cut, y+1), g)
			c.Fill(image.Rect(cx+cut, y, r.Max.X, y+1), g)
		}
	}
	shape(r, g)
	if !filled {
		in := image.Rect(r.Min.X+3, r.Min.Y+3, r.Max.X-3, r.Max.Y-4)
		shape(in, kobo.White)
	}
}

// footTap does what a tap at p on the strip under the page asks, if
// it's on a button.
func (w *bookWin) footTap(a *bro, p image.Point) bool {
	toc, style, mark, sel, write := w.footButtons()
	switch {
	case p.In(write):
		log.Printf("pdf: write on page %d", w.spine+1)
		a.writePDF(w.path, w.spine)
	case p.In(sel):
		w.selecting = !w.selecting
		w.dropCut()
		log.Printf("pdf: select tool %v", w.selecting)
		w.draw(a)
	case p.In(toc):
		w.openContents(a)
	case p.In(style):
		w.openStyle(a)
	case p.In(mark):
		w.toggleMark(a)
	default:
		return false
	}
	return true
}

// --- the list's tabs ---

// showList shows the book's contents, or its bookmarks: the place shown
// picked in either.
func (w *bookWin) showList(a *bro, marks bool) {
	w.list, w.listMks, w.sel = true, marks, 0
	if marks {
		for i, m := range w.marks {
			if m.Spine < w.spine || m.Spine == w.spine && !w.at.Before(m.pos()) {
				w.sel = i
			}
		}
	} else {
		for i, e := range w.b.TOC {
			if e.Spine <= w.spine {
				w.sel = i
			}
		}
	}
	w.top = w.sel - 3
	w.draw(a)
}

// drawTabs draws the list's tabs in r: the contents, the bookmarks;
// and at their right the way back to the page, for a finger.
func (w *bookWin) drawTabs(a *bro, r image.Rectangle) {
	c := a.C
	button := func(b image.Rectangle, label string, do func()) {
		c.Frame(b, 2, kobo.Black)
		c.Text(a.F.Panel, b, label, kobo.Black, kobo.AlignCenter)
		w.rows = append(w.rows, bookRow{b, do})
	}
	shelf := image.Rect(r.Min.X, r.Min.Y, r.Min.X+min(150, r.Dx()/6), r.Max.Y-6)
	button(shelf, "‹ books", func() { w.save(a); a.replaceWin(w, a.newBookWin("")) })
	back := image.Rect(r.Max.X-min(260, r.Dx()/4), r.Min.Y, r.Max.X, r.Max.Y-6)
	button(back, "back to the book", func() { w.list = false; w.draw(a) })
	r.Min.X, r.Max.X = shelf.Max.X+16, back.Min.X-16
	c.Fill(image.Rect(r.Min.X, r.Max.Y-2, r.Max.X, r.Max.Y), kobo.Light)
	half := r.Dx() / 2
	for k, name := range []string{"Contents", fmt.Sprintf("Bookmarks (%d)", len(w.marks))} {
		t := image.Rect(r.Min.X+k*half, r.Min.Y, r.Min.X+(k+1)*half, r.Max.Y)
		ink := uint8(kobo.Mid)
		if marks := k == 1; marks == w.listMks {
			ink = kobo.Black
			c.Fill(image.Rect(t.Min.X, t.Max.Y-5, t.Max.X, t.Max.Y), kobo.Black)
		}
		c.Text(a.F.Body, t, name, ink, kobo.AlignCenter)
		marks := k == 1
		w.rows = append(w.rows, bookRow{t, func() { w.showList(a, marks) }})
	}
}

// markCorner is the page's top right, where its ribbon hangs: a tap
// there bookmarks it, or takes its bookmark away.
func (w *bookWin) markCorner() image.Rectangle {
	body := w.p.Body
	return image.Rect(w.area.Max.X-120, body.Min.Y, body.Max.X, w.area.Min.Y+100)
}

// swipe: on a page, across it turns it (to the left on, to the right
// back); on a list, up or down goes a screenful on or back.
func (w *bookWin) swipe(a *bro, from, to image.Point) {
	d := to.Sub(from)
	switch {
	case w.styling:
	case w.list:
		if kobo.Abs(d.Y) < 60 || kobo.Abs(d.Y) < 2*kobo.Abs(d.X) {
			return
		}
		n := max(1, w.listN-1)
		if d.Y > 0 { // the finger down: back
			n = -n
		}
		w.sel += n
		w.top += n
		w.draw(a)
	case kobo.Abs(d.X) >= 60 && kobo.Abs(d.X) > 2*kobo.Abs(d.Y):
		if d.X < 0 {
			w.turn(a, 1)
		} else {
			w.turn(a, -1)
		}
	}
}
