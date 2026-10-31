package main

// Exporting a notebook: "export" (or Ctrl+E) asks which, as Settings'
// "Notebook export" has it: a PDF, "NAME DATE TIME.pdf" by its .xopp in
// Notebooks/ (each export its own), a picture of each page (200 dpi, gray: for a computer, a printer, or
// an LLM to read); of pages on a PDF (from Xournal++), the pages written
// on. Or the page shown as a PNG in Clips/, as "NAME pN DATE TIME.png"
// (each export its own) and latest.png, as a PDF's cut is (bookcut.go):
// for a program in Alpine to take. It's drawn away from the screen's
// loop, from what the pages were when it was asked; a note on the page
// says when it's written, and where.

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"image"
	"image/png"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

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

// export exports the notebook as the setting has it: asking (the PDF
// first, Enter's), or at once.
func (w *noteWin) export(a *bro) {
	w.endTyping(a)
	if w.exporting {
		return
	}
	switch a.setting("note-export") {
	case "pdf":
		w.exportAs(a, "pdf")
	case "png":
		w.exportAs(a, "png")
	default:
		n := len(w.exportPages())
		all := fmt.Sprintf("PDF: the %d pages", n)
		if n == 1 {
			all = "PDF: the page"
		}
		a.choose("Export "+filepath.Base(w.nb.name), "as a PDF in Notebooks/, or this page as a PNG in Clips/",
			askOpt{all, "p", func() { w.exportAs(a, "pdf") }},
			askOpt{fmt.Sprintf("PNG: page %d", w.cur+1), "n", func() { w.exportAs(a, "png") }},
			askOpt{"cancel", "c", nil})
	}
}

// exportAs writes the notebook as a PDF ("pdf"), or the page shown as a
// PNG ("png").
func (w *noteWin) exportAs(a *bro, as string) {
	if w.exporting {
		return
	}
	pages, shown := w.exportPages(), w.exportPage(min(w.cur, len(w.nb.pages)-1))
	if as == "png" {
		pages = []exportPage{shown}
	}
	if as == "pdf" && len(pages) == 0 {
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
	at := time.Now()
	go func() {
		var msg string
		var err error
		if as == "png" {
			msg, err = exportPNG(name, paper, pdfPath, shown, at)
		} else {
			msg, err = exportPDF(name, paper, pdfPath, pages, at)
		}
		a.Do(func() {
			w.exporting = false
			if err != nil {
				log.Printf("notebook %s: export: %v", name, err)
				a.Notify("not exported: " + err.Error())
				a.Toast("not exported: "+err.Error(), 8*time.Second)
				return
			}
			log.Printf("notebook %s: exported, %s as %s", name, count, as)
			a.Notify(msg)
			a.Toast(msg, 6*time.Second)
		})
	}()
}

// exportDoc is the PDF under a notebook, if it's on one (to close).
func exportDoc(pdfPath string) (*pdf.Doc, error) {
	if pdfPath == "" {
		return nil, nil
	}
	return pdf.Open(pdfPath)
}

// exportPDF draws pages and writes them as the notebook's PDF, named for
// when it was asked (at); it says where.
func exportPDF(name, paper, pdfPath string, pages []exportPage, at time.Time) (string, error) {
	doc, err := exportDoc(pdfPath)
	if err != nil {
		return "", err
	}
	if doc != nil {
		defer doc.Close()
	}
	var imgs []pdfImagePage
	for _, pg := range pages {
		m, err := renderNotePage(pg, paper, doc, noteExportDPI)
		if err != nil {
			return "", fmt.Errorf("page %d: %w", pg.n+1, err)
		}
		imgs = append(imgs, pdfImagePage{img: m, w: pg.pt[0], h: pg.pt[1]})
	}
	out := filepath.Join(notesDir(), filepath.FromSlash(name)+exportStamp(at)+".pdf")
	if err := writeAtomic(out, imagesPDF(imgs)); err != nil {
		return "", err
	}
	rel, _ := filepath.Rel(notesDir(), out)
	return "exported: Notebooks/" + filepath.ToSlash(rel), nil
}

// exportPNG draws the page shown and writes it as a PNG in Clips/, named
// for when it was asked (at), and as latest.png; it says where.
func exportPNG(name, paper, pdfPath string, shown exportPage, at time.Time) (string, error) {
	doc, err := exportDoc(pdfPath)
	if err != nil {
		return "", err
	}
	if doc != nil {
		defer doc.Close()
	}
	m, err := renderNotePage(shown, paper, doc, noteExportDPI)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	png.Encode(&b, m) // into memory: it can't fail
	clips := filepath.Join(booksRoot(), "Clips")
	if err := os.MkdirAll(clips, 0o755); err != nil {
		return "", err
	}
	pngName := exportPNGName(name, shown.n, at)
	for _, f := range []string{pngName, "latest.png"} {
		if err := writeAtomic(filepath.Join(clips, f), b.Bytes()); err != nil {
			return "", err
		}
	}
	return "exported: Clips/" + pngName, nil
}

// exportPNGName is the page n's PNG, exported at: the notebook's own name
// (in a folder too), the page and when.
func exportPNGName(name string, n int, at time.Time) string {
	return fmt.Sprintf("%s p%d%s.png", filepath.Base(name), n+1, exportStamp(at))
}

// exportStamp is when an export was asked, after its name: as the card
// takes it (no colons).
func exportStamp(at time.Time) string { return at.Format(" 2006-01-02 15.04.05") }

// exportMark is in the PDFs exports write: what's written over, or
// moved with a notebook renamed, is only ever one of those.
const exportMark = "%hellobro notebook export\n"

// exportsOf are the notebook name's exports, by what's after its name in
// theirs: " DATE TIME.pdf", and as they were once, ".pdf" and
// " (notebook).pdf". Only ours: a PDF of the same name may be someone
// else's (Xournal++'s lecture.pdf beside lecture.xopp).
func exportsOf(name string) map[string]string {
	p := filepath.Join(notesDir(), filepath.FromSlash(name))
	ps, _ := filepath.Glob(globEscape(p) + " ????-??-?? ??.??.??.pdf")
	ps = append(ps, p+".pdf", p+" (notebook).pdf")
	out := map[string]string{}
	for _, f := range ps {
		if isExport(f) {
			out[strings.TrimPrefix(f, p)] = f
		}
	}
	return out
}

// globEscape is p as a glob matching only itself.
func globEscape(p string) string {
	var b strings.Builder
	for _, r := range p {
		if strings.ContainsRune(`*?[\`, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// isExport says whether the file at p is one an export wrote.
func isExport(p string) bool {
	f, err := os.Open(p)
	if err != nil {
		return false
	}
	defer f.Close()
	head := make([]byte, 64)
	n, _ := io.ReadFull(f, head)
	return bytes.Contains(head[:n], []byte(exportMark))
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
	default:
		drawPaper(c, paper, pg.size, image.Point{}, k, m.Bounds(), noteExportRule)
	}
	at := func(x, y int16) image.Point { return image.Pt(int(float64(x)*k+0.5), int(float64(y)*k+0.5)) }
	for l := range 3 {
		for _, s := range pg.strokes {
			switch {
			case strokeLayer(s) != l:
			case s.txt != nil:
				drawNoteText(c, s, image.Point{}, k, -1, m.Bounds())
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
	b.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n" + exportMark)
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
