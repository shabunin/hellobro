package main

// Cutting a part of a PDF's page out: the pen drawn across the page, or
// a finger held still a moment then moved (as in the terminals), makes a
// rectangle, and by it its buttons: copy its text (into the clipboard,
// which the terminals paste), save it (a picture at 300 dpi and its
// text, in Clips/ on the Kobo's storage, and as Clips/latest.png and
// latest.txt too, for a program in Alpine to take: an LLM's, say), or
// ×. A tap elsewhere or Esc takes it away, as does turning the page.

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"hellobro/kobo"
	"hellobro/pdf"
)

const (
	cutDPI   = 300  // a cut saved: as sharp as a print
	cutMaxPx = 3000 // but no bigger, along its longer side
	cutMin   = 24   // px: a rectangle smaller isn't one
)

// bookCut is a gesture cutting a part out, while it's on.
type bookCut struct {
	w        *bookWin
	g        *kobo.Gesture
	early    float64 // how far the finger got in its first moment
	on       bool    // cutting: the gesture is ours
	from, to image.Point
	shown    image.Rectangle // the frame on the screen now
	drawn    time.Time
}

// cutStart notes a contact on a PDF's page: it may become a cut.
func (a *bro) cutStart(p image.Point, g *kobo.Gesture) bool {
	w, ok := a.windowAt(p).(*bookWin)
	if !ok || w.pdf == nil || w.list || w.styling || !p.In(w.view.dst) {
		return false
	}
	a.bk.cut = &bookCut{w: w, g: g, from: p, to: p}
	return true
}

// cutMove follows it: the pen cuts as soon as it moves, a finger once
// it's been held still a moment. The frame follows in the fast black
// and white updates, now and then.
func (a *bro) cutMove(p image.Point, g *kobo.Gesture) {
	s, w := a.bk.cut, a.bk.cut.w
	if !s.on {
		switch {
		case g.Owner != 0:
			a.bk.cut = nil
			return
		case g.Pen && g.Moved > selStill:
		case g.Pen:
			return
		case time.Since(g.At) < selHold:
			s.early = g.Moved
			return
		case s.early > selStill: // it was moving from the start: a swipe
			a.bk.cut = nil
			return
		}
		s.on, g.Owner = true, ownSelect
		if !w.cut.Empty() {
			w.cut = image.Rectangle{}
			w.draw(a)
		}
		log.Print("pdf: cutting")
	}
	s.to = p
	if time.Since(s.drawn) < 150*time.Millisecond {
		return
	}
	s.drawn = time.Now()
	r := image.Rectangle{s.from, s.to}.Canon().Intersect(w.view.dst)
	if sh, err := w.pdfPage(w.spine, w.laid); err == nil && !s.shown.Empty() { // the page back under the old frame
		old := s.shown.Intersect(w.view.dst)
		kobo.CopyGray(a.C.Gray, old, sh.img, old.Min.Sub(w.view.dst.Min).Add(w.view.sp))
	}
	a.C.Frame(r, 3, kobo.Black)
	a.Mark(r.Union(s.shown), false)
	s.shown = r
}

// cutLift ends it: the rectangle stays, with its buttons.
func (a *bro) cutLift(g *kobo.Gesture) bool {
	s := a.bk.cut
	if s == nil || s.g != g {
		return false
	}
	a.bk.cut = nil
	if !s.on {
		return false
	}
	w := s.w
	if r := (image.Rectangle{s.from, s.to}).Canon().Intersect(w.view.dst); r.Dx() >= cutMin && r.Dy() >= cutMin {
		w.cut = r
	}
	w.draw(a)
	return true
}

// drawCut draws the rectangle cut out, if there's one, and its buttons:
// under it, or over it if there's no room under it.
func (w *bookWin) drawCut(a *bro) {
	w.cutRows = nil
	r := w.cut
	if r.Empty() {
		return
	}
	c := a.C
	c.Frame(r.Inset(-3), 3, kobo.White)
	c.Frame(r, 3, kobo.Black)
	const h, gap = 64, 10
	labels := []string{"copy text", "save", "×"}
	widths := []int{200, 140, 64}
	total := 0
	for _, bw := range widths {
		total += bw + gap
	}
	y := r.Max.Y + gap
	if y+h > w.area.Max.Y {
		y = max(w.area.Min.Y, r.Min.Y-gap-h)
	}
	x := max(w.area.Min.X, min(r.Max.X, w.area.Max.X)-total)
	do := []func(){func() { w.cutCopy(a) }, func() { w.cutSave(a) }, func() { w.cut = image.Rectangle{}; w.draw(a) }}
	for i, label := range labels {
		b := image.Rect(x, y, x+widths[i], y+h)
		x += widths[i] + gap
		c.Fill(b, kobo.White)
		c.Frame(b, 3, kobo.Black)
		c.Text(a.F.Panel, b, label, kobo.Black, kobo.AlignCenter)
		w.cutRows = append(w.cutRows, bookRow{b, do[i]})
	}
}

// cutRect is the cut in the page's points.
func (w *bookWin) cutRect() pdf.Rect {
	v := w.view
	at := func(p image.Point) (float64, float64) {
		q := p.Sub(v.dst.Min).Add(v.sp)
		return float64(q.X) / v.zoom, float64(q.Y) / v.zoom
	}
	x0, y0 := at(w.cut.Min)
	x1, y1 := at(w.cut.Max)
	return pdf.Rect{X0: x0, Y0: y0, X1: x1, Y1: y1}
}

// cutCopy copies the cut's text.
func (w *bookWin) cutCopy(a *bro) {
	text, err := w.pdf.TextIn(w.spine, w.cutRect())
	switch {
	case err != nil:
		a.Notify("can't read it: " + err.Error())
	case strings.TrimSpace(text) == "":
		a.Notify("no text in it (a scan?): save keeps it as a picture")
	default:
		a.copyText(text, "")
	}
}

// cutSave saves the cut, a picture and its text, away from the main
// loop: drawn big, it takes a moment.
func (w *bookWin) cutSave(a *bro) {
	d, spine, r := w.pdf, w.spine, w.cutRect()
	dir := filepath.Join(booksRoot(), "Clips")
	name := fmt.Sprintf("%s, p%d, %s", fileName(w.Title(), 40), spine+1, time.Now().Format("2006-01-02 150405"))
	a.Notify("saving…")
	go func() {
		err := saveCut(d, spine, r, dir, name)
		a.Do(func() {
			if err != nil {
				log.Printf("pdf: saving a cut: %v", err)
				a.Notify("can't save it: " + err.Error())
				return
			}
			log.Printf("pdf: saved %s", name)
			a.Notify("saved: Clips/" + name + ".png")
		})
	}()
}

func saveCut(d *pdf.Doc, spine int, r pdf.Rect, dir, name string) error {
	z := min(cutDPI/72.0, cutMaxPx/max(r.X1-r.X0, r.Y1-r.Y0))
	m, err := d.Render(spine, z, image.Rect(int(r.X0*z), int(r.Y0*z), int(r.X1*z+0.5), int(r.Y1*z+0.5)))
	if err != nil {
		return err
	}
	var pic bytes.Buffer
	if err := png.Encode(&pic, m); err != nil {
		return err
	}
	text, _ := d.TextIn(spine, r)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, n := range []string{name, "latest"} {
		if err := os.WriteFile(filepath.Join(dir, n+".png"), pic.Bytes(), 0o644); err != nil {
			return err
		}
		txt := filepath.Join(dir, n+".txt")
		if strings.TrimSpace(text) == "" {
			os.Remove(txt) // a scan's: none, rather than the last one's
			continue
		}
		if err := os.WriteFile(txt, []byte(text), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// fileName is s made a file's name, at most n letters: what FAT and
// shells take without fuss.
func fileName(s string, n int) string {
	var b strings.Builder
	k := 0
	for _, r := range s {
		if k >= n {
			break
		}
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_':
		case unicode.IsSpace(r) || r == '.' || r == ',':
			r = ' '
		default:
			continue
		}
		b.WriteRune(r)
		k++
	}
	return strings.Join(strings.Fields(b.String()), " ")
}
