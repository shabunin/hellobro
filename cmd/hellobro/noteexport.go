package main

// Exporting a notebook: "export" (or Ctrl+E) writes it as a PDF, NAME.pdf
// by its .xopp in Notebooks/, a picture of each page (200 dpi, gray: for
// a computer, a printer, or an LLM to read); of pages on a PDF (from
// Xournal++), the pages written on. And the page shown as a PNG in Clips/, as NAME pN.png
// and latest.png, as a PDF's cut is (bookcut.go): for a program in Alpine
// to take. It's drawn away from the screen's loop, from what the pages
// were when it was asked; a note says when it's written.

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"slices"

	xdraw "golang.org/x/image/draw"

	"hellobro/kobo"
	"hellobro/pdf"
)

const (
	noteExportDPI  = 200
	noteExportRule = 0xE6 // the paper's lines, printed: lighter than on screen
)

// exportPage is a page to export, as it was.
type exportPage struct {
	n       int // which (from 0)
	size    image.Point
	pt      [2]float64 // its size in points
	strokes []*noteStroke
	pdfPage int // the PDF's page under it, -1 none
}

// exportPages are the pages to export: those written on, of a notebook
// on a PDF; else all to the last written on.
func (w *noteWin) exportPages() []exportPage {
	onPDF := w.nb.pdfPath() != ""
	last := 0
	for n, pg := range w.nb.pages {
		if len(pg) > 0 {
			last = n
		}
	}
	var out []exportPage
	for n := range w.nb.pages[:last+1] {
		if onPDF && len(w.nb.pages[n]) == 0 {
			continue
		}
		out = append(out, w.exportPage(n))
	}
	return out
}

func (w *noteWin) exportPage(n int) exportPage {
	size := w.pageSize(n)
	return exportPage{n: n, size: size, pt: [2]float64{float64(size.X) * notePt72, float64(size.Y) * notePt72},
		strokes: slices.Clone(w.strokes(n)), pdfPage: w.pdfPage(n)}
}

// export writes the notebook as a PDF, and the page shown as a PNG.
func (w *noteWin) export(a *bro) {
	if w.exporting {
		return
	}
	pages, shown := w.exportPages(), w.exportPage(min(w.cur, len(w.nb.pages)-1))
	if len(pages) == 0 {
		a.Notify("nothing written on it to export")
		return
	}
	w.exporting = true
	name, paper, pdfPath := w.nb.name, w.nb.paper, w.nb.pdfPath()
	count := fmt.Sprintf("%d pages", len(pages))
	if len(pages) == 1 {
		count = "a page"
	}
	a.Notify("exporting " + count + "…")
	go func() {
		msg, err := exportNotebook(name, paper, pdfPath, pages, shown)
		a.Do(func() {
			w.exporting = false
			if err != nil {
				log.Printf("notebook %s: export: %v", name, err)
				a.Notify("not exported: " + err.Error())
				return
			}
			log.Printf("notebook %s: exported, %s", name, count)
			a.Notify(msg)
		})
	}()
}

// exportNotebook draws pages and writes them; it says where.
func exportNotebook(name, paper, pdfPath string, pages []exportPage, shown exportPage) (string, error) {
	var doc *pdf.Doc
	if pdfPath != "" {
		var err error
		if doc, err = pdf.Open(pdfPath); err != nil {
			return "", err
		}
		defer doc.Close()
	}
	var imgs []pdfImagePage
	var shownImg *image.Gray
	for _, pg := range pages {
		m, err := renderNotePage(pg, paper, doc, noteExportDPI)
		if err != nil {
			return "", fmt.Errorf("page %d: %w", pg.n+1, err)
		}
		imgs = append(imgs, pdfImagePage{img: m, w: pg.pt[0], h: pg.pt[1]})
		if pg.n == shown.n {
			shownImg = m
		}
	}
	if shownImg == nil {
		var err error
		if shownImg, err = renderNotePage(shown, paper, doc, noteExportDPI); err != nil {
			return "", err
		}
	}
	out := filepath.Join(notesDir(), name+".pdf")
	if err := writeAtomic(out, imagesPDF(imgs)); err != nil {
		return "", err
	}
	var b bytes.Buffer
	png.Encode(&b, shownImg) // into memory: it can't fail
	clips := filepath.Join(booksRoot(), "Clips")
	if err := os.MkdirAll(clips, 0o755); err != nil {
		return "", err
	}
	pngName := fmt.Sprintf("%s p%d.png", name, shown.n+1)
	for _, f := range []string{pngName, "latest.png"} {
		if err := writeAtomic(filepath.Join(clips, f), b.Bytes()); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("exported: Notebooks/%s.pdf; page %d as Clips/latest.png", name, shown.n+1), nil
}

// writeAtomic writes b to path by way of a new file: never half of it.
func writeAtomic(path string, b []byte) error {
	tmp := path + ".new"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// renderNotePage draws pg at dpi: its paper (the PDF's page, or the
// lines) and its strokes, in their layers.
func renderNotePage(pg exportPage, paper string, doc *pdf.Doc, dpi float64) (*image.Gray, error) {
	k := dpi / 227 // px a page unit
	px := image.Pt(int(float64(pg.size.X)*k+0.5), int(float64(pg.size.Y)*k+0.5))
	m := kobo.Blank(px)
	c := kobo.Canvas{Gray: m}
	switch {
	case pg.pdfPage >= 0 && doc != nil:
		bg, err := renderPDFPage(doc, pg.pdfPage, px.X)
		if err != nil {
			return nil, err
		}
		kobo.CopyGray(m, bg.Bounds().Intersect(m.Bounds()), bg, image.Point{})
	case paper == "squares" || paper == "lines":
		if paper == "squares" {
			for u := noteRule; u < pg.size.X; u += noteRule {
				x := int(float64(u) * k)
				c.Fill(image.Rect(x, 0, x+1, px.Y), noteExportRule)
			}
		}
		for u := 2 * noteRule; u < pg.size.Y; u += noteRule {
			y := int(float64(u) * k)
			c.Fill(image.Rect(0, y, px.X, y+1), noteExportRule)
		}
	}
	at := func(x, y int16) image.Point { return image.Pt(int(float64(x)*k+0.5), int(float64(y)*k+0.5)) }
	for l := range 3 {
		for _, s := range pg.strokes {
			switch {
			case strokeLayer(s) != l:
			case s.pic != nil:
				r := image.Rect(int(float64(s.box.Min.X)*k), int(float64(s.box.Min.Y)*k), int(float64(s.box.Max.X)*k), int(float64(s.box.Max.Y)*k))
				xdraw.ApproxBiLinear.Scale(m, r, s.pic.gray, s.pic.gray.Bounds(), xdraw.Src, nil)
			default:
				radius := func(p notePt) int { return max(1, int(float64(p.r)/10*k+0.5)) }
				prev := at(s.pts[0].x, s.pts[0].y)
				c.Disc(prev, radius(s.pts[0]), s.gray, m.Bounds())
				for _, p := range s.pts[1:] {
					q := at(p.x, p.y)
					c.Line(prev, q, radius(p), s.gray, m.Bounds())
					prev = q
				}
			}
		}
	}
	return m, nil
}

// pdfImagePage is a page of a PDF that's a picture: it, and the page's
// size in points.
type pdfImagePage struct {
	img  *image.Gray
	w, h float64
}

// imagesPDF is a PDF of pages, each a gray picture filling it.
func imagesPDF(pages []pdfImagePage) []byte {
	var b bytes.Buffer
	var offs []int
	obj := func(body string, stream []byte) {
		offs = append(offs, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s", len(offs), body)
		if stream != nil {
			b.WriteString("\nstream\n")
			b.Write(stream)
			b.WriteString("\nendstream")
		}
		b.WriteString("\nendobj\n")
	}
	b.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
	kids := ""
	for i := range pages {
		kids += fmt.Sprintf("%d 0 R ", 3+3*i)
	}
	obj("<< /Type /Catalog /Pages 2 0 R >>", nil)
	obj(fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", kids, len(pages)), nil)
	for i, pg := range pages {
		m := pg.img
		var z bytes.Buffer
		zw := zlib.NewWriter(&z)
		for y := range m.Rect.Dy() {
			zw.Write(m.Pix[y*m.Stride : y*m.Stride+m.Rect.Dx()])
		}
		zw.Close()
		content := fmt.Sprintf("q %.2f 0 0 %.2f 0 0 cm /Im0 Do Q", pg.w, pg.h)
		obj(fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.2f %.2f] /Resources << /XObject << /Im0 %d 0 R >> >> /Contents %d 0 R >>",
			pg.w, pg.h, 5+3*i, 4+3*i), nil)
		obj(fmt.Sprintf("<< /Length %d >>", len(content)), []byte(content))
		obj(fmt.Sprintf("<< /Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace /DeviceGray /BitsPerComponent 8 /Filter /FlateDecode /Length %d >>",
			m.Rect.Dx(), m.Rect.Dy(), z.Len()), z.Bytes())
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(offs)+1)
	for _, o := range offs {
		fmt.Fprintf(&b, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offs)+1, xref)
	return b.Bytes()
}
