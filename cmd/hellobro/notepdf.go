package main

// Notebooks on PDFs: a PDF's pages as the paper, written over. "write"
// under a PDF's page in the reader (bookmarks.go) opens its notebook
// (named as the PDF) at that page, made the first time: a page for each
// of the PDF's, as big as it is. In the .xopp each page's background is
// that page of the PDF, as Xournal++ has it (type="pdf", pageno from 1,
// the PDF's path on the first page only), so Xournal++ opens it too,
// asking where the PDF is if it's not where the Kobo has it.
//
// A page of a PDF is drawn by MuPDF, away from the screen's loop (it
// takes most of a second on the Elipsa), blank till it's done; each is
// drawn once at a size and kept, a few of them.

import (
	"encoding/xml"
	"image"
	"log"
	"os"
	"path/filepath"
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
	zoom := float64(width) / sz.W
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
	case !lines:
	case w.nb.paper == "squares" || w.nb.paper == "lines":
		if w.nb.paper == "squares" {
			for u := noteRule; u < size.X; u += noteRule {
				x := pg.Min.X + int(float64(u)*w.scale)
				c.Fill(image.Rect(x, pg.Min.Y, x+1, pg.Max.Y).Intersect(cl), kobo.Light)
			}
		}
		for u := 2 * noteRule; u < size.Y; u += noteRule {
			y := pg.Min.Y + int(float64(u)*w.scale)
			c.Fill(image.Rect(pg.Min.X, y, pg.Max.X, y+1).Intersect(cl), kobo.Light)
		}
	}
	for l := range 3 {
		for _, s := range w.strokes(n) {
			if strokeLayer(s) == l && w.toScreenRect(s.box, pg).Overlaps(cl) {
				w.drawStroke(a, s, pg, cl)
			}
		}
	}
}

// pdfNotebook is a new notebook on the PDF at path: a page for each of
// its pages.
func pdfNotebook(name, path string, doc *pdf.Doc) (*notebook, error) {
	nb := &notebook{name: name, paper: "plain"}
	at := func(n, v string) xml.Attr { return xml.Attr{Name: xml.Name{Local: n}, Value: v} }
	for i := range doc.Pages() {
		sz, err := doc.Size(i)
		if err != nil {
			return nil, err
		}
		bg := &xoppAny{XMLName: xml.Name{Local: "background"}, Attrs: []xml.Attr{at("type", "pdf")}}
		if i == 0 {
			bg.Attrs = append(bg.Attrs, at("domain", "absolute"), at("filename", path))
		}
		bg.Attrs = append(bg.Attrs, at("pageno", strconv.Itoa(i+1)))
		nb.pages = append(nb.pages, nil)
		nb.meta = append(nb.meta, notePage{w: sz.W, h: sz.H, bg: bg})
	}
	if len(nb.pages) == 0 {
		return nil, os.ErrInvalid
	}
	return nb, nil
}

// writePDF opens the notebook on the PDF at path at its page (from 0):
// made the first time, named as the PDF (or "NAME (2)"… if a notebook
// of that name is on something else).
func (a *bro) writePDF(path string, page int) {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	for i := 1; ; i++ {
		name := base
		if i > 1 {
			name = base + " (" + strconv.Itoa(i) + ")"
		}
		if w := a.noteWindow(name); w != nil {
			if w.nb.pdfPath() != path {
				continue
			}
			w.goPDFPage(a, page)
			a.wm.tree.Focus(a.nodeOf(w))
			a.wmApply()
			return
		}
		if _, err := os.Stat(notePath(name)); err == nil {
			w := a.noteOpen(name)
			if w == nil || w.nb.pdfPath() != path {
				continue
			}
			w.goPDFPage(nil, page)
			a.openWin(w)
			return
		}
		doc, err := pdf.Open(path)
		if err == nil {
			var nb *notebook
			if nb, err = pdfNotebook(name, path, doc); err == nil {
				w := &noteWin{nb: nb, changed: time.Now()}
				w.pdf = &notePDF{path: path, doc: doc, drawn: map[pdfKey]*image.Gray{}, pending: map[pdfKey]bool{}}
				w.goPDFPage(nil, page)
				log.Printf("notebook %s: on %s, %d pages", name, path, len(nb.pages))
				a.openWin(w)
				return
			}
			doc.Close()
		}
		log.Printf("notebook on %s: %v", path, err)
		a.Notify("can't write on it: " + err.Error())
		return
	}
}

// goPDFPage shows the page that's the PDF's page i (a nil a: not
// shown yet).
func (w *noteWin) goPDFPage(a *bro, i int) {
	for n := range w.nb.pages {
		if w.pdfPage(n) == i {
			w.cur, w.overview = n, false
			break
		}
	}
	if a != nil {
		w.show(a)
	}
}
