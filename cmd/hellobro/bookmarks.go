package main

// Bookmarks, and the bar over a book's page (toolbar.go, as a
// notebook's): its buttons for the shelf, the contents (☰), the style
// panel (Aa), a bookmark (a ribbon, filled when the page has one), and
// the pages. A bookmark is a place in a chapter, as the
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

// --- the bar along the top (toolbar.go) ---

// barTools are the bar's: All Books, back from a link, the contents,
// the style, a bookmark; a PDF's select tool (bookcut.go) and zoom
// (one page at a time). Then those that go first where there's no room.
func (w *bookWin) barTools() (tools []barBtn, drops []string) {
	tools = []barBtn{{id: "all", label: "‹ All Books"}}
	if w.from != nil {
		tools = append(tools, barBtn{id: "back", label: "‹ back"})
	}
	tools = append(tools, barBtn{id: "toc"}, barBtn{id: "style"}, barBtn{id: "mark"})
	if w.pdf != nil {
		tools = append(tools, barBtn{id: "select"})
		if !w.laid.two {
			tools = append(tools, barBtn{id: "zoom out"}, barBtn{id: "fit", label: "100%"}, barBtn{id: "zoom in"})
		}
	}
	return tools, []string{"fit", "style", "zoom out", "zoom in"}
}

// barKind is the book's, for its bar fullscreen (fullBar).
func (w *bookWin) barKind() string {
	if w.pdf != nil {
		return "pdf"
	}
	return "epub"
}

// flipBar shows the bar fullscreen, or not, for books of this one's kind
// (the style panel's row), and lays this one out again.
func (w *bookWin) flipBar(a *bro) {
	a.flipFullBar(w.barKind())
	w.place(a, w.p)
	w.draw(a)
}

// counter is which pages show: a PDF's of all, an EPUB's of the chapter.
func (w *bookWin) counter() string {
	n, of := w.page, len(w.pages)
	if w.pdf != nil {
		n, of = w.spine, len(w.b.Spine)
	}
	if w.laid.two && n+1 < of {
		return fmt.Sprintf("%d–%d / %d", n+1, n+2, of)
	}
	return fmt.Sprintf("%d / %d", n+1, of)
}

// atEnd says the page shown is the book's first (d < 0) or its last.
func (w *bookWin) atEnd(d int) bool {
	if d < 0 {
		return w.spine == 0 && w.page == 0
	}
	last, step := len(w.pages)-1, 1
	if w.laid.two {
		last -= last % 2
		if w.pdf != nil { // its pages two at a time: the last two, or the last
			step = 2
		}
	}
	return w.spine+step >= len(w.b.Spine) && w.page >= last
}

// drawBar lays the bar out and draws it: the select tool black while
// it's in hand, what can't be done now gray.
func (w *bookWin) drawBar(a *bro) {
	tools, drops := w.barTools()
	cw := kobo.TextWidth(a.F.Small, "999 / 999")
	if w.laid.two {
		cw = kobo.TextWidth(a.F.Small, "999–999 / 999")
	}
	if w.bar.Empty() { // fullscreen, without it (barOff)
		w.btns = nil
		return
	}
	w.btns = layTop(a, w.bar, tools, drops, cw)
	c := a.C
	c.Fill(w.bar, kobo.White)
	for _, b := range w.btns {
		on := b.id == "select" && w.selecting
		off := b.id == "prev" && w.atEnd(-1) || b.id == "next" && w.atEnd(1) ||
			(b.id == "fit" || b.id == "zoom out") && w.zoom <= pdfZoomMin
		drawBarBtn(a, b, on, off, func(r image.Rectangle, ink, bg uint8) bool {
			m, s := r.Min.Add(r.Max).Div(2), r.Dy()/4
			switch b.id {
			case "toc": // ☰: three lines
				for k := -1; k <= 1; k++ {
					c.Fill(image.Rect(m.X-s, m.Y+k*s*2/3-1, m.X+s, m.Y+k*s*2/3+2), ink)
				}
			case "style":
				c.Text(a.F.Body, r, "Aa", ink, kobo.AlignCenter)
			case "mark": // filled while the page has one
				ribbon(c, image.Rect(m.X-s*2/3, m.Y-s, m.X+s*2/3, m.Y+s), len(w.pageMarks()) > 0, ink)
			case "select":
				drawIcon(c, noteIcon("select"), r, ink, bg)
			case "zoom out", "zoom in": // −, +: bars, as ☰'s
				c.Fill(image.Rect(m.X-s, m.Y-1, m.X+s, m.Y+2), ink)
				if b.id == "zoom in" {
					c.Fill(image.Rect(m.X-1, m.Y-s, m.X+2, m.Y+s), ink)
				}
			default:
				return false
			}
			return true
		})
	}
	drawBarCounter(a, barCounter(w.btns, w.bar, false), w.counter(), false)
	if n := len(w.btns); n > 2 { // between the tools and the arrows, if there's room: the chapter, how far in
		gap := image.Rect(w.btns[n-3].r.Max.X+2*barGap, w.bar.Min.Y, w.btns[n-1].r.Min.X-2*barGap, w.bar.Max.Y)
		text := fmt.Sprintf("%d%%", w.pct())
		if name := w.chapterName(); name != "" {
			text = name + " · " + text
		}
		if gap.Dx() >= 120 {
			c.Text(a.F.Small, gap, kobo.Fit(a.F.Small, text, gap.Dx()), kobo.Mid, kobo.AlignCenter)
		}
	}
}

// barTap does what a tap at p on the bar asks: true if it was on it.
func (w *bookWin) barTap(a *bro, p image.Point) bool {
	if p.In(barCounter(w.btns, w.bar, false)) { // the pages, small; an EPUB's contents
		if w.pdf != nil {
			w.showPages(a)
		} else {
			w.openContents(a)
		}
		return true
	}
	for _, b := range w.btns {
		if !p.In(b.r) {
			continue
		}
		switch b.id {
		case "all":
			w.toShelf(a)
		case "back":
			w.goBack(a)
		case "toc":
			w.openContents(a)
		case "style":
			w.openStyle(a)
		case "mark":
			w.toggleMark(a)
		case "select":
			w.selecting = !w.selecting
			w.dropCut()
			log.Printf("pdf: select tool %v", w.selecting)
			w.draw(a)
		case "zoom out":
			w.zoomStep(a, -1)
		case "zoom in":
			w.zoomStep(a, 1)
		case "fit":
			w.setZoom(a, pdfZoomMin, w.area.Min.Add(w.area.Max).Div(2))
		case "prev":
			w.turn(a, -1)
		case "next":
			w.turn(a, 1)
		}
		return true
	}
	return p.In(w.bar) // between the buttons: nothing
}

// drawPageMark hangs a ribbon in the top margin at the right, if the
// page is bookmarked.
func (w *bookWin) drawPageMark(a *bro) {
	if len(w.pageMarks()) == 0 {
		return
	}
	top := w.bar.Max.Y
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
	return image.Rect(w.area.Max.X-120, w.bar.Max.Y, body.Max.X, w.area.Min.Y+100)
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
