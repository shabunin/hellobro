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
	next := w.page + 1
	if w.laid.two && w.pdf == nil { // two side by side: the right one's too
		next++
	}
	if next < len(w.pages) {
		to = w.pages[next].Start
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

const (
	footButton = 100 // a button's width
	zoomButton = 84  // a PDF's zoom buttons': − 100% +
	zoomGap    = 24  // between them and the rest
)

// footButtons are the strip's buttons: the contents, the style, a
// bookmark, and in a PDF the select tool (bookcut.go), side by side in
// its middle (with the zoom buttons, if they show).
func (w *bookWin) footButtons() (toc, style, mark, sel image.Rectangle) {
	foot := image.Rect(w.area.Min.X, w.area.Max.Y, w.area.Max.X, w.p.Body.Max.Y)
	n, width := 3, 3*footButton
	if w.pdf != nil {
		n, width = 4, 4*footButton
	}
	if w.zoomShows() {
		width += zoomGap + 3*zoomButton
	}
	x := (foot.Min.X+foot.Max.X)/2 - width/2
	bs := make([]image.Rectangle, 4)
	for i := range n {
		bs[i] = image.Rect(x+i*footButton, foot.Min.Y, x+(i+1)*footButton, foot.Max.Y)
	}
	return bs[0], bs[1], bs[2], bs[3]
}

// zoomShows is whether the strip has a PDF's zoom buttons: it zooms
// (one page at a time), and there's room for them.
func (w *bookWin) zoomShows() bool {
	return w.pdf != nil && !w.laid.two && w.area.Dx() >= 4*footButton+zoomGap+3*zoomButton+2*160
}

// zoomButtons are a PDF's zoom buttons, right of the select tool:
// out, back to the fit (100%), in. Empty if they don't show.
func (w *bookWin) zoomButtons() (out, fit, in image.Rectangle) {
	if !w.zoomShows() {
		return
	}
	_, _, _, sel := w.footButtons()
	r := image.Rect(sel.Max.X+zoomGap, sel.Min.Y, sel.Max.X+zoomGap+zoomButton, sel.Max.Y)
	return r, r.Add(image.Pt(zoomButton, 0)), r.Add(image.Pt(2*zoomButton, 0))
}

// drawFoot draws the strip: the chapter, the buttons, how far in.
func (w *bookWin) drawFoot(a *bro) {
	c := a.C
	toc, style, mark, sel := w.footButtons()
	out, fit, in := w.zoomButtons()
	foot := image.Rect(w.area.Min.X, w.area.Max.Y, w.area.Max.X, w.p.Body.Max.Y-4)
	last := mark.Union(sel).Union(in)
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
	if !in.Empty() {
		m = out.Min.Add(out.Max).Div(2) // −, + : bars, as ☰'s
		c.Fill(image.Rect(m.X-14, m.Y-2, m.X+14, m.Y+2), kobo.Dark)
		m = in.Min.Add(in.Max).Div(2)
		c.Fill(image.Rect(m.X-14, m.Y-2, m.X+14, m.Y+2), kobo.Dark)
		c.Fill(image.Rect(m.X-2, m.Y-14, m.X+2, m.Y+14), kobo.Dark)
		ink := uint8(kobo.Dark)
		if w.zoom == pdfZoomMin { // there already
			ink = kobo.Mid
		}
		c.Text(a.F.Small, fit, "100%", ink, kobo.AlignCenter)
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
	toc, style, mark, sel := w.footButtons()
	out, fit, in := w.zoomButtons()
	switch {
	case p.In(out):
		w.zoomStep(a, -1)
	case p.In(in):
		w.zoomStep(a, 1)
	case p.In(fit):
		w.setZoom(a, pdfZoomMin, w.area.Min.Add(w.area.Max).Div(2))
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
	w.list, w.listMks, w.listPages, w.sel = true, marks, false, 0
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

// drawTabs draws the list's header in r (library.go): All Books, the
// tabs, the contents, the bookmarks, a PDF's pages (bookpages.go); and
// at their right the way back to the page.
func (w *bookWin) drawTabs(a *bro, r image.Rectangle) {
	names := []string{"Contents", fmt.Sprintf("Bookmarks (%d)", len(w.marks))}
	if w.pdf != nil {
		names = append(names, "Pages")
	}
	on := 0
	switch {
	case w.listPages:
		on = 2
	case w.listMks:
		on = 1
	}
	var tabs []libTab
	for k, name := range names {
		show := func() { w.showList(a, k == 1) }
		if k == 2 {
			show = func() { w.showPages(a) }
		}
		tabs = append(tabs, libTab{name, k == on, show})
	}
	h := a.drawLibHeader(r, "All Books", false, func() { w.toShelf(a) }, "back to the book", func() { w.list = false; w.draw(a) }, tabs)
	w.rows = append(w.rows, h.rows...)
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
		if w.listPages { // a screenful of pages
			n = w.listN
		}
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
