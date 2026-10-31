package main

// Notebooks on PDFs: a page whose background is a PDF's page, as
// Xournal++ has it (type="pdf", pageno from 1, the PDF's path on the
// first page that has it), shows that page under the ink: a notebook
// written on a PDF in Xournal++ and copied here, with its PDF where the
// .xopp says it is.
//
// A page of a PDF is drawn by MuPDF, away from the screen's loop (it
// takes most of a second on the Elipsa), blank till it's done; each is
// drawn once at a size and kept, a few of them.

import (
	"image"
	"log"
	"strconv"
	"strings"
	"time"

	"hellobro/kobo"
	"hellobro/pdf"
)

const notePDFKeep = 24 << 20 // bytes of drawn PDF pages kept

// notePDF is a notebook's PDF, open, and its pages as drawn.
type notePDF struct {
	path    string
	doc     *pdf.Doc
	err     error
	drawn   map[pdfKey]*image.Gray
	order   []pdfKey // the oldest first
	pending map[pdfKey]bool
}

type pdfKey struct{ page, width int }

// bgPDFPage is which of the PDF's pages bg is (from 0); -1 if it's not.
func bgPDFPage(bg *xoppAny) int {
	if bg == nil || attr(bg.Attrs, "type") != "pdf" {
		return -1
	}
	no := attr(bg.Attrs, "pageno")
	no = no[:len(no)-len(strings.TrimLeft(no, "0123456789"))] // Xournal++ writes "1ll"
	n, err := strconv.Atoi(no)
	if err != nil || n < 1 {
		return -1
	}
	return n - 1
}

// pdfPath is the PDF the notebook's pages are on, "" if none.
func (nb *notebook) pdfPath() string {
	for _, m := range nb.meta {
		if bgPDFPage(m.bg) >= 0 {
			if f := attr(m.bg.Attrs, "filename"); f != "" {
				return f
			}
		}
	}
	return ""
}

// pdfPage is which of the PDF's pages page n is on; -1 if none.
func (w *noteWin) pdfPage(n int) int {
	if n < 0 || n >= len(w.nb.meta) {
		return -1
	}
	return bgPDFPage(w.nb.meta[n].bg)
}

// pageSize is page n's size in page units: as its file says (a PDF's
// page's), or the Elipsa's screen.
func (w *noteWin) pageSize(n int) image.Point {
	if n >= 0 && n < len(w.nb.meta) && w.nb.meta[n].w > 0 && w.nb.meta[n].h > 0 {
		m := w.nb.meta[n]
		return image.Pt(int(m.w/notePt72+0.5), int(m.h/notePt72+0.5))
	}
	return image.Pt(notePageW, notePageH)
}

// pageBounds is page n, in page units.
func (w *noteWin) pageBounds(n int) image.Rectangle { return image.Rectangle{Max: w.pageSize(n)} }

// pdfImage is page n's PDF page drawn width px wide; nil till it's
// drawn (then where it shows is drawn again), or if it can't be.
func (w *noteWin) pdfImage(a *bro, n, width int) *image.Gray {
	i := w.pdfPage(n)
	if i < 0 || width <= 0 {
		return nil
	}
	if w.pdf == nil {
		path := w.nb.pdfPath()
		w.pdf = &notePDF{path: path, drawn: map[pdfKey]*image.Gray{}, pending: map[pdfKey]bool{}}
		w.pdf.doc, w.pdf.err = pdf.Open(path)
		if w.pdf.err != nil {
			log.Printf("notebook %s: %v", w.nb.name, w.pdf.err)
			a.Notify("the notebook's PDF: " + w.pdf.err.Error())
		}
	}
	np := w.pdf
	k := pdfKey{i, width}
	if m := np.drawn[k]; m != nil || np.err != nil {
		return m
	}
	if !np.pending[k] {
		np.pending[k] = true
		go func() {
			start := time.Now()
			m, err := renderPDFPage(np.doc, i, width)
			a.Do(func() {
				delete(np.pending, k)
				if err != nil {
					log.Printf("notebook %s: PDF page %d: %v", w.nb.name, i+1, err)
					return
				}
				log.Printf("notebook %s: PDF page %d drawn %d px wide in %v", w.nb.name, i+1, width, time.Since(start).Round(time.Millisecond))
				np.keep(k, m)
				if w.pdf == np {
					w.redraw(a, w.screenOf(n))
				}
			})
		}()
	}
	return nil
}

// renderPDFPage draws doc's page i width px wide.
func renderPDFPage(doc *pdf.Doc, i, width int) (*image.Gray, error) {
	sz, err := doc.Size(i)
	if err != nil {
		return nil, err
	}
	zoom := pdf.CapZoom(sz.W, sz.H, min(float64(width)/sz.W, 4*float64(width)/sz.H)) // a long page: no longer than 4 widths
	return doc.Render(i, zoom, image.Rectangle{Max: sz.Scaled(zoom)})
}

// keep keeps m, the oldest going past notePDFKeep bytes.
func (np *notePDF) keep(k pdfKey, m *image.Gray) {
	np.drawn[k], np.order = m, append(np.order, k)
	total := 0
	for _, o := range np.order {
		total += len(np.drawn[o].Pix)
	}
	for total > notePDFKeep && len(np.order) > 1 {
		total -= len(np.drawn[np.order[0]].Pix)
		delete(np.drawn, np.order[0])
		np.order = np.order[1:]
	}
}

// closePDF lets the PDF go (MuPDF keeps a lot for it).
func (w *noteWin) closePDF() {
	if w.pdf != nil && w.pdf.doc != nil {
		w.pdf.doc.Close()
	}
	w.pdf = nil
}

// screenOf is where page n shows: its slot, or its tile in the
// overview; empty if it doesn't.
func (w *noteWin) screenOf(n int) image.Rectangle {
	if w.overview {
		tiles, _ := w.ovLayout()
		if i := n - w.ovFirst; i >= 0 && i < len(tiles) {
			return tiles[i]
		}
		return image.Rectangle{}
	}
	r, _ := w.slotOf(n)
	return r
}

// drawPage draws page n at pg on screen, where it's in cl: its paper
// (a PDF's page, or the lines) and its strokes.
func (w *noteWin) drawPage(a *bro, n int, pg, cl image.Rectangle, lines bool) {
	c := a.C
	size := w.pageSize(n)
	switch {
	case w.pdfPage(n) >= 0:
		if m := w.pdfImage(a, n, pg.Dx()); m != nil {
			c.Fill(cl, kobo.White)
			kobo.CopyGray(c.Gray, cl.Intersect(m.Bounds().Add(pg.Min)), m, cl.Min.Sub(pg.Min))
		}
	case lines:
		drawPaper(c, w.nb.paper, size, pg.Min, w.scale, cl, kobo.Light) // notepaper.go
	}
	for l := range 3 {
		for _, s := range w.strokes(n) {
			if strokeLayer(s) == l && w.toScreenRect(s.box, pg).Overlaps(cl) {
				w.drawStroke(a, s, pg, cl)
			}
		}
	}
}
