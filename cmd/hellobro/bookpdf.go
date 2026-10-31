package main

// PDFs in the reader: a PDF is a book whose chapters are its pages, so
// the shelf, the contents (its outline), bookmarks, the bar over the
// page and turning pages are the books' own. Here: a page drawn by MuPDF
// (package pdf), gray, its white margins trimmed, as big as the window
// takes it whole, or as wide as the window, then zoomed if asked; one
// bigger than the window is read in screenfuls, across then down. The
// next page is drawn while the reader reads, so turning to it is at
// once. Its style panel has the fit, the zoom, the trimming and the
// pages: one, or two side by side, each whole, meeting in the middle as
// a book's do, turned two at a time. Cutting a part of a page out is
// bookcut.go's.

import (
	"errors"
	"fmt"
	"image"
	"log"
	"path/filepath"
	"slices"
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

// pdfZooms are the zoom's steps, % of the fit (a pinch goes between).
var pdfZooms = []int{100, 125, 150, 175, 200, 250, 300, 400}

// pdfShown is a page drawn for a layout.
type pdfShown struct {
	spine int
	laid  bookLaid
	img   *image.Gray // its bounds: where it is in the page drawn whole
	zoom  float64     // px a point
}

// pdfAhead is a page being drawn ahead, away from the main loop.
type pdfAhead struct {
	spine int
	laid  bookLaid
	done  chan struct{} // closed when it's drawn
	s     pdfShown
	err   error
}

// pdfView is where a page shown is on the screen: its part from sp
// at dst, zoom px a point.
type pdfView struct {
	dst   image.Rectangle
	sp    image.Point
	zoom  float64
	spine int
}

// viewAt is the page shown at p, if one is.
func (w *bookWin) viewAt(p image.Point) (pdfView, bool) {
	for _, v := range w.views {
		if p.In(v.dst) {
			return v, true
		}
	}
	return pdfView{}, false
}

func isPDF(path string) bool { return strings.HasSuffix(strings.ToLower(path), ".pdf") }

// openPDF opens the PDF at path and makes its book: a chapter a page,
// the outline its contents. Away from the main loop: it touches nothing
// of a window's.
func openPDF(path string) (*pdf.Doc, *book.Book, error) {
	d, err := pdf.Open(path)
	if err != nil {
		return nil, nil, err
	}
	n := d.Pages()
	b := &book.Book{Title: d.Title(), Spine: make([]string, n)}
	if b.Title == "" {
		b.Title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	for _, e := range d.Outline() {
		if e.Page < n {
			b.TOC = append(b.TOC, book.TOCEntry{Title: e.Title, Spine: e.Page, Depth: e.Depth})
		}
	}
	log.Printf("pdf: %s: %d pages, %d in the outline", filepath.Base(path), n, len(b.TOC))
	return d, b, nil
}

// pdfDraw draws page spine of d for a layout: away from the main loop,
// it touches nothing of the window's.
func pdfDraw(d *pdf.Doc, spine int, laid bookLaid) (pdfShown, error) {
	size, err := d.Size(spine)
	if err != nil {
		return pdfShown{}, err
	}
	if pdf.CapZoom(size.W, size.H, 1) == 0 {
		return pdfShown{}, errors.New("the page has no size")
	}
	// where its ink is, in points
	x0, y0, x1, y1 := 0.0, 0.0, size.W, size.H
	if !laid.st.pdfNoTrim {
		z := pdf.CapZoom(size.W, size.H, min(pdfInkPx/size.W, 4*pdfInkPx/size.H)) // a long page: no longer than 4 widths' worth
		m, err := d.Render(spine, z, image.Rectangle{Max: size.Scaled(z)})
		if err != nil {
			return pdfShown{}, err
		}
		if ink := pdf.Ink(m); !ink.Empty() {
			pad := 4.0 // px of the small drawing, round the ink
			x0, y0 = max(0, (float64(ink.Min.X)-pad)/z), max(0, (float64(ink.Min.Y)-pad)/z)
			x1, y1 = min(size.W, (float64(ink.Max.X)+pad)/z), min(size.H, (float64(ink.Max.Y)+pad)/z)
		}
	}
	z, whole := float64(laid.w)/(x1-x0), float64(laid.w)/size.W
	if !laid.st.pdfWidth {
		z, whole = min(z, float64(laid.h)/(y1-y0)), min(whole, float64(laid.h)/size.H)
	}
	z = min(z, 3*whole) // trimming takes the margins off: a thin strip of ink isn't blown up to the window
	z = pdf.CapZoom(x1-x0, y1-y0, z*float64(laid.st.pdfZoom)/100)
	if z == 0 {
		return pdfShown{}, errors.New("no room to draw the page")
	}
	clip := image.Rect(int(x0*z), int(y0*z), int(x1*z+0.5), int(y1*z+0.5))
	img, err := d.Render(spine, z, clip)
	return pdfShown{spine, laid, img, z}, err
}

// pdfPage is the page spine drawn for the layout, drawn now if it hasn't
// been yet; if it's being drawn ahead, it's waited for.
func (w *bookWin) pdfPage(spine int, laid bookLaid) (pdfShown, error) {
	for _, s := range w.shown {
		if s.spine == spine && s.laid == laid {
			return s, nil
		}
	}
	if ah := w.ahead; ah != nil && ah.spine == spine && ah.laid == laid {
		<-ah.done // nearly there: rather than draw it twice
		if ah.err == nil {
			w.keep(ah.s)
			return ah.s, nil
		}
	}
	t := time.Now()
	s, err := pdfDraw(w.pdf, spine, laid)
	if err != nil {
		return pdfShown{}, err
	}
	log.Printf("pdf: page %d drawn in %v", spine+1, time.Since(t).Round(time.Millisecond))
	w.keep(s)
	return s, nil
}

// keep keeps a page drawn, and the ones near the page shown: the one
// before, the one after; of two side by side, the two before and after.
func (w *bookWin) keep(s pdfShown) {
	lo, hi, most := w.spine-1, w.spine+1, pdfKept
	if s.laid.two {
		lo, hi, most = w.spine-2, w.spine+3, 2*pdfKept
	}
	kept := []pdfShown{s}
	for _, o := range w.shown {
		if o.laid == s.laid && o.spine != s.spine && o.spine >= lo && o.spine <= hi && len(kept) < most {
			kept = append(kept, o)
		}
	}
	w.shown = kept
}

// layoutPDF draws the page and cuts it in screenfuls, if it's bigger
// than the window: a screen page each, across then down, starting at
// thousandths of its height (Pos.Block) and width (Pos.Off), so a place
// and a bookmark hold at any size.
func (w *bookWin) layoutPDF(a *bro, laid bookLaid) {
	w.ch, w.laid = &book.Chapter{}, laid // laid out: the chapter's there
	w.pages, w.tiles, w.page = []book.Page{{}}, []image.Point{{}}, 0
	s, err := w.pdfPage(w.spine, laid)
	if err != nil {
		w.err = err.Error()
		log.Printf("pdf: page %d: %v", w.spine+1, err)
		return
	}
	b := s.img.Bounds()
	if b.Empty() {
		w.err = "the page drew empty"
		return
	}
	w.err = ""
	w.pages, w.tiles = nil, nil
	for _, y := range pdfSteps(b.Dy(), laid.h) {
		for _, x := range pdfSteps(b.Dx(), laid.w) {
			w.pages = append(w.pages, book.Page{Start: book.Pos{Block: y * 1000 / b.Dy(), Off: x * 1000 / b.Dx()}})
			w.tiles = append(w.tiles, image.Pt(x, y))
		}
	}
	w.page = book.PageAt(w.pages, w.at)
}

// pdfSteps are where the screenfuls start along n px, screen px a
// screenful, a little of each showing again on the next.
func pdfSteps(n, screen int) []int {
	if n <= screen {
		return []int{0}
	}
	step := screen - screen/pdfOverlap
	var at []int
	for o := 0; ; o += step {
		o = min(o, n-screen)
		at = append(at, o)
		if o+screen >= n {
			return at
		}
	}
}

// drawPDF shows the screenful of the page the window's on; of two side
// by side, the pages, the left one's right edge and the right one's
// left meeting at the gutter.
func (w *bookWin) drawPDF(a *bro) {
	w.views = nil
	area := w.area
	if w.err != "" {
		a.C.Text(a.F.Body, image.Rect(area.Min.X, area.Min.Y, area.Max.X, area.Min.Y+40), "can't draw this page: "+w.err, kobo.Dark, kobo.AlignLeft)
	} else if w.page < len(w.tiles) {
		switch {
		case w.laid.two:
			w.drawPDFIn(a, w.spine, image.Rect(area.Min.X, area.Min.Y, area.Min.X+w.laid.w, area.Max.Y), image.Point{}, 1)
			if w.spine+1 < len(w.b.Spine) {
				w.drawPDFIn(a, w.spine+1, image.Rect(area.Max.X-w.laid.w, area.Min.Y, area.Max.X, area.Max.Y), image.Point{}, -1)
			}
		default:
			tile := w.tiles[w.page]
			if s, err := w.pdfPage(w.spine, w.laid); err == nil && w.panned {
				tile = w.panTile(s.img.Bounds())
			}
			w.drawPDFIn(a, w.spine, area, tile, 0)
		}
	}
	if (!w.cut.Empty() || w.corner != nil) && slices.ContainsFunc(w.views, func(v pdfView) bool { return v.spine == w.view.spine }) {
		for _, v := range w.views { // a cut's page: where it is now
			if v.spine == w.view.spine {
				w.view = v
			}
		}
	} else if len(w.views) > 0 {
		w.view = w.views[0]
	} else {
		w.view = pdfView{}
	}
	w.drawCut(a)
	w.drawPageMark(a)
	w.drawBar(a)
	w.drawAhead(a)
}

// drawPDFIn draws the screenful at tile of page spine in r: in the
// middle of it; its right edge at r's (lean 1), or its left (-1).
func (w *bookWin) drawPDFIn(a *bro, spine int, r image.Rectangle, tile image.Point, lean int) {
	s, err := w.pdfPage(spine, w.laid)
	if err != nil {
		log.Printf("pdf: page %d: %v", spine+1, err)
		return
	}
	b := s.img.Bounds()
	size := image.Pt(min(b.Dx(), r.Dx()), min(b.Dy(), r.Dy()))
	dst := image.Rectangle{Max: size}.Add(r.Min).Add(r.Size().Sub(size).Div(2))
	switch lean {
	case 1:
		dst = dst.Add(image.Pt(r.Max.X-dst.Max.X, 0))
	case -1:
		dst = dst.Add(image.Pt(r.Min.X-dst.Min.X, 0))
	}
	sp := b.Min.Add(tile)
	kobo.CopyGray(a.C.Gray, dst, s.img, sp)
	w.views = append(w.views, pdfView{dst, sp, s.zoom, spine})
}

// drawAhead draws the next page while this one's read, if it's the last
// screenful of this one; of two side by side, the next two, one by one.
func (w *bookWin) drawAhead(a *bro) {
	laid, d := w.laid, w.pdf
	if w.ahead != nil || w.page+1 < len(w.pages) {
		return
	}
	next := -1
	for _, n := range map[bool][]int{false: {w.spine + 1}, true: {w.spine + 2, w.spine + 3}}[laid.two] {
		if n < len(w.b.Spine) && !slices.ContainsFunc(w.shown, func(s pdfShown) bool { return s.spine == n && s.laid == laid }) {
			next = n
			break
		}
	}
	if next < 0 {
		return
	}
	ah := &pdfAhead{spine: next, laid: laid, done: make(chan struct{})}
	w.ahead = ah
	go func() {
		t := time.Now()
		ah.s, ah.err = pdfDraw(d, next, laid)
		close(ah.done)
		a.Do(func() {
			if w.ahead == ah {
				w.ahead = nil
			}
			if ah.err != nil || w.pdf != d || ah.laid != w.laid { // drawn for a zoom or a size gone: it'd push out the page shown
				return
			}
			log.Printf("pdf: page %d drawn ahead in %v", next+1, time.Since(t).Round(time.Millisecond))
			w.keep(ah.s)
			w.drawAhead(a)
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

// --- its style panel: the fit, the trimming ---

const (
	pdfStyleFit = iota
	pdfStyleZoom
	pdfStyleTrim
	pdfStylePages
	pdfStyleDone
	pdfStyleN
)

// pdfStyleStep changes row i: d -1 or 1 a step down or up, 0 picks it.
func (w *bookWin) pdfStyleStep(a *bro, i, d int) {
	st := a.bookStyle()
	switch i {
	case pdfStyleFit:
		st.pdfWidth = !st.pdfWidth
	case pdfStyleZoom:
		if d != 0 {
			w.zoomStep(a, d)
		}
		return
	case pdfStyleTrim:
		st.pdfNoTrim = !st.pdfNoTrim
	case pdfStylePages:
		w.setPageMode(a, nextPages(w.pageMode, d))
		return
	case pdfStyleDone:
		w.closeStyle(a)
		return
	}
	a.setBookStyle(st)
}

func (w *bookWin) drawPDFStyle(a *bro) {
	st := a.bookStyle()
	area := w.p.Body.Inset(kobo.Pad)
	c := a.C
	w.rows = nil
	head := w.styleHeader(a, &area, func() { w.pdfStyleStep(a, pdfStyleDone, 0) })
	fit := map[bool]string{false: "whole page", true: "page width"}
	onOff := map[bool]string{true: "on", false: "off"}
	labels := [pdfStyleN]string{"Fit", "Zoom", "Trim white margins", "Pages", ""}
	w.sel = max(0, min(w.sel, pdfStyleN-1))
	y, rowH, boxW := area.Min.Y+4, 72, 72
	for i := range pdfStyleN {
		if i == pdfStyleDone { // in the header, at its right
			if i == w.sel {
				c.Frame(head.Inset(-4), 3, kobo.Dark)
			}
			continue
		}
		r := image.Rect(area.Min.X+6, y, area.Max.X-6, y+rowH-8)
		y += rowH
		if i == w.sel {
			c.Frame(r.Inset(-4), 3, kobo.Dark)
		}
		c.Text(a.F.Body, image.Rect(r.Min.X+12, r.Min.Y, r.Max.X, r.Max.Y), labels[i], kobo.Black, kobo.AlignLeft)
		box := func(b image.Rectangle, label string, d int) {
			c.Frame(b, 2, kobo.Black)
			c.Text(a.F.Panel, b, label, kobo.Black, kobo.AlignCenter)
			w.rows = append(w.rows, bookRow{b, func() { w.sel = i; w.pdfStyleStep(a, i, d) }})
		}
		switch i {
		case pdfStyleZoom:
			box(image.Rect(r.Max.X-boxW, r.Min.Y, r.Max.X, r.Max.Y), "+", 1)
			vr := image.Rect(r.Max.X-boxW-160, r.Min.Y, r.Max.X-boxW, r.Max.Y)
			c.Text(a.F.Body, vr, fmt.Sprintf("%d%%", w.zoom), kobo.Black, kobo.AlignCenter)
			box(image.Rect(vr.Min.X-boxW, r.Min.Y, vr.Min.X, r.Max.Y), "−", -1)
		case pdfStyleFit:
			box(image.Rect(r.Max.X-260, r.Min.Y, r.Max.X, r.Max.Y), fit[st.pdfWidth], 0)
		case pdfStyleTrim:
			box(image.Rect(r.Max.X-260, r.Min.Y, r.Max.X, r.Max.Y), onOff[!st.pdfNoTrim], 0)
		case pdfStylePages:
			box(image.Rect(r.Max.X-260, r.Min.Y, r.Max.X, r.Max.Y), bookPageWords[w.pageMode], 0)
		}
	}
	c.Text(a.F.Small, image.Rect(area.Min.X+18, y+8, area.Max.X, y+48), "a page bigger than the screen is read in screenfuls, across then down; + and - zoom too; two side by side are whole", kobo.Mid, kobo.AlignLeft)
}

func (w *bookWin) pdfStyleKey(a *bro, code uint16, ch string) bool {
	switch {
	case code == 103 || ch == "k": // up
		w.sel = max(0, w.sel-1)
		w.draw(a)
	case code == 108 || ch == "j" || code == kobo.KeyTab:
		w.sel = min(pdfStyleN-1, w.sel+1)
		w.draw(a)
	case code == 105 || ch == "h": // left
		w.pdfStyleStep(a, w.sel, -1)
	case code == 106 || ch == "l":
		w.pdfStyleStep(a, w.sel, 1)
	case code == kobo.KeyEnter || code == kobo.KeySpace:
		w.pdfStyleStep(a, w.sel, 0)
	case ch == "+" || ch == "=":
		w.zoomStep(a, 1)
	case ch == "-":
		w.zoomStep(a, -1)
	case code == kobo.KeyEsc || ch == "s" || ch == "q":
		w.closeStyle(a)
	default:
		return false
	}
	return true
}
