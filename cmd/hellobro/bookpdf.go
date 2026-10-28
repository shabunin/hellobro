package main

// PDFs in the reader: a PDF is a book whose chapters are its pages, so
// the shelf, the contents (its outline), bookmarks, the strip under the
// page and turning pages are the books' own. Here: a page drawn by MuPDF
// (package pdf), gray, its white margins trimmed, as big as the window
// takes it whole, or as wide as the window and then in screenfuls; the
// next page drawn ahead while the reader reads, so turning to it is at
// once. Its style panel has the fit and the trimming.

import (
	"fmt"
	"image"
	"log"
	"path/filepath"
	"strings"
	"time"

	"hellobro/book"
	"hellobro/kobo"
	"hellobro/pdf"
)

const (
	pdfInkPx   = 300 // a page is drawn this wide first, to find its ink
	pdfOverlap = 20  // of a page in screenfuls, 1/n of a screenful shows again on the next
	pdfKept    = 3   // pages kept drawn: the one shown, the one before, the one ahead
)

// pdfShown is a page drawn for a layout.
type pdfShown struct {
	spine int
	laid  bookLaid
	img   *image.Gray
}

func isPDF(path string) bool { return strings.HasSuffix(strings.ToLower(path), ".pdf") }

// openPDF opens w's PDF and makes its book: a chapter a page, the
// outline its contents.
func (a *bro) openPDF(w *bookWin) {
	d, err := pdf.Open(w.path)
	if err != nil {
		w.err = err.Error()
		log.Printf("pdf: %s: %v", w.path, err)
		return
	}
	n := d.Pages()
	b := &book.Book{Title: d.Title(), Spine: make([]string, n)}
	if b.Title == "" {
		b.Title = strings.TrimSuffix(filepath.Base(w.path), filepath.Ext(w.path))
	}
	for _, e := range d.Outline() {
		if e.Page < n {
			b.TOC = append(b.TOC, book.TOCEntry{Title: e.Title, Spine: e.Page, Depth: e.Depth})
		}
	}
	w.pdf, w.b = d, b
	if pl, ok := a.mem.Books[w.path]; ok && pl.Spine < n {
		w.spine, w.at = pl.Spine, book.Pos{Block: pl.Block, Off: pl.Off}
	}
	log.Printf("pdf: %s: %d pages, %d in the outline", filepath.Base(w.path), n, len(b.TOC))
}

// pdfDraw draws page spine of d for a layout: away from the main loop,
// it touches nothing of the window's.
func pdfDraw(d *pdf.Doc, spine int, laid bookLaid) (*image.Gray, error) {
	size, err := d.Size(spine)
	if err != nil {
		return nil, err
	}
	// where its ink is, in points
	x0, y0, x1, y1 := 0.0, 0.0, size.W, size.H
	if !laid.st.pdfNoTrim && size.W > 0 {
		z := pdfInkPx / size.W
		m, err := d.Render(spine, z, image.Rectangle{Max: size.Scaled(z)})
		if err != nil {
			return nil, err
		}
		if ink := pdf.Ink(m); !ink.Empty() {
			pad := 4.0 // px of the small drawing, round the ink
			x0, y0 = max(0, (float64(ink.Min.X)-pad)/z), max(0, (float64(ink.Min.Y)-pad)/z)
			x1, y1 = min(size.W, (float64(ink.Max.X)+pad)/z), min(size.H, (float64(ink.Max.Y)+pad)/z)
		}
	}
	z := float64(laid.w) / (x1 - x0)
	if !laid.st.pdfWidth {
		z = min(z, float64(laid.h)/(y1-y0))
	}
	clip := image.Rect(int(x0*z), int(y0*z), int(x1*z+0.5), int(y1*z+0.5))
	return d.Render(spine, z, clip)
}

// pdfPage is the page spine drawn for the layout, drawn now if it hasn't
// been yet.
func (w *bookWin) pdfPage(spine int, laid bookLaid) (*image.Gray, error) {
	for _, s := range w.shown {
		if s.spine == spine && s.laid == laid {
			return s.img, nil
		}
	}
	t := time.Now()
	img, err := pdfDraw(w.pdf, spine, laid)
	if err != nil {
		return nil, err
	}
	log.Printf("pdf: page %d drawn in %v", spine+1, time.Since(t).Round(time.Millisecond))
	w.keep(pdfShown{spine, laid, img})
	return img, nil
}

// keep keeps a page drawn, and the ones near the page shown.
func (w *bookWin) keep(s pdfShown) {
	kept := []pdfShown{s}
	for _, o := range w.shown {
		if o.laid == s.laid && o.spine != s.spine && o.spine >= w.spine-1 && o.spine <= w.spine+1 && len(kept) < pdfKept {
			kept = append(kept, o)
		}
	}
	w.shown = kept
}

// layoutPDF draws the page and cuts it in screenfuls, if it's taller
// than the window: a screen page each, starting at a thousandth of its
// height (Pos.Off), so a place and a bookmark hold at any size.
func (w *bookWin) layoutPDF(a *bro, laid bookLaid) {
	w.ch, w.laid = &book.Chapter{}, laid // laid out: the chapter's there
	w.pages = []book.Page{{}}
	w.page = 0
	img, err := w.pdfPage(w.spine, laid)
	if err != nil {
		w.err = err.Error()
		log.Printf("pdf: page %d: %v", w.spine+1, err)
		return
	}
	w.err = ""
	h, screen := img.Bounds().Dy(), laid.h
	if h > screen {
		step := screen - screen/pdfOverlap
		w.pages = w.pages[:0]
		for top := 0; ; top += step {
			top = min(top, h-screen)
			w.pages = append(w.pages, book.Page{Start: book.Pos{Off: top * 1000 / h}})
			if top+screen >= h {
				break
			}
		}
	}
	w.page = book.PageAt(w.pages, w.at)
}

// drawPDF shows the screenful of the page the window's on.
func (w *bookWin) drawPDF(a *bro) {
	if w.err != "" {
		a.C.Text(a.F.Body, image.Rect(w.area.Min.X, w.area.Min.Y, w.area.Max.X, w.area.Min.Y+40), "can't draw this page: "+w.err, kobo.Dark, kobo.AlignLeft)
	} else if img, err := w.pdfPage(w.spine, w.laid); err == nil {
		b := img.Bounds()
		top := w.pages[w.page].Start.Off * b.Dy() / 1000
		r := image.Rect(0, 0, b.Dx(), min(b.Dy(), w.area.Dy())).Add(w.area.Min)
		r = r.Add(image.Pt((w.area.Dx()-r.Dx())/2, (w.area.Dy()-r.Dy())/2)) // in the middle
		kobo.CopyGray(a.C.Gray, r.Intersect(w.area), img, b.Min.Add(image.Pt(0, top)))
	}
	w.drawPageMark(a)
	w.drawFoot(a)
	w.drawAhead(a)
}

// drawAhead draws the next page while this one's read, if it's the last
// screenful of this one.
func (w *bookWin) drawAhead(a *bro) {
	next, laid, d := w.spine+1, w.laid, w.pdf
	if w.ahead || next >= len(w.b.Spine) || w.page+1 < len(w.pages) {
		return
	}
	for _, s := range w.shown {
		if s.spine == next && s.laid == laid {
			return
		}
	}
	w.ahead = true
	go func() {
		t := time.Now()
		img, err := pdfDraw(d, next, laid)
		a.Do(func() {
			w.ahead = false
			if err != nil || w.pdf != d {
				return
			}
			log.Printf("pdf: page %d drawn ahead in %v", next+1, time.Since(t).Round(time.Millisecond))
			w.keep(pdfShown{next, laid, img})
		})
	}()
}

// pdfWords is how the page shown starts, n bytes or so of it.
func (w *bookWin) pdfWords(n int) string {
	text, err := w.pdf.Text(w.spine)
	if err != nil {
		return ""
	}
	s := strings.Join(strings.Fields(text), " ")
	if len(s) > n {
		s = strings.ToValidUTF8(s[:n], "") + "…"
	}
	return s
}

// pdfFootName is the strip's left: the page, and the outline's line.
func (w *bookWin) pdfFootName() string {
	s := fmt.Sprintf("%d / %d", w.spine+1, len(w.b.Spine))
	if name := w.chapterName(); name != "" {
		s += "  ·  " + name
	}
	return s
}

// --- its style panel: the fit, the trimming ---

func (w *bookWin) drawPDFStyle(a *bro) {
	st := a.bookStyle()
	area := w.p.Body.Inset(kobo.Pad)
	c := a.C
	w.rows = nil
	fit := map[bool]string{false: "whole page", true: "page width"}
	onOff := map[bool]string{true: "on", false: "off"}
	rows := []struct {
		label, val string
		do         func()
	}{
		{"Fit", fit[st.pdfWidth], func() { st.pdfWidth = !st.pdfWidth; a.setBookStyle(st) }},
		{"Trim white margins", onOff[!st.pdfNoTrim], func() { st.pdfNoTrim = !st.pdfNoTrim; a.setBookStyle(st) }},
		{"", "back to the book", func() { w.closeStyle(a) }},
	}
	w.sel = max(0, min(w.sel, len(rows)-1))
	y, rowH := area.Min.Y+4, 72
	for i, row := range rows {
		r := image.Rect(area.Min.X+6, y, area.Max.X-6, y+rowH-8)
		y += rowH
		if i == w.sel {
			c.Frame(r.Inset(-4), 3, kobo.Dark)
		}
		c.Text(a.F.Body, image.Rect(r.Min.X+12, r.Min.Y, r.Max.X, r.Max.Y), row.label, kobo.Black, kobo.AlignLeft)
		b := image.Rect(r.Max.X-260, r.Min.Y, r.Max.X, r.Max.Y)
		c.Frame(b, 2, kobo.Black)
		c.Text(a.F.Panel, b, row.val, kobo.Black, kobo.AlignCenter)
		do := row.do
		w.rows = append(w.rows, bookRow{b, func() { w.sel = i; do() }})
	}
	c.Text(a.F.Small, image.Rect(area.Min.X+18, y+8, area.Max.X, y+48), "a page as wide as the screen is read in screenfuls, top to bottom", kobo.Mid, kobo.AlignLeft)
}

func (w *bookWin) pdfStyleKey(a *bro, code uint16, ch string) bool {
	switch {
	case code == 103 || ch == "k": // up
		w.sel = max(0, w.sel-1)
		w.draw(a)
	case code == 108 || ch == "j" || code == kobo.KeyTab:
		w.sel = min(2, w.sel+1)
		w.draw(a)
	case code == kobo.KeyEnter || code == kobo.KeySpace || code == 105 || code == 106:
		if w.sel < len(w.rows) {
			w.rows[w.sel].do()
		}
	case code == kobo.KeyEsc || ch == "s" || ch == "q":
		w.closeStyle(a)
	default:
		return false
	}
	return true
}
