package main

// Cutting a part of a PDF's page out: the pen drawn across the page, or
// a finger held still a moment then moved (as in the terminals), makes a
// rectangle. So does the select tool (its button under the page), by
// taps alone: one corner, then the other; a tap on a corner's dot picks
// it, the next tap puts it there. By the rectangle are its buttons: copy it (its text into the
// clipboard, which the terminals paste; and its picture, which a
// notebook pastes: notepic.go), save it (a picture at 300 dpi and its
// text, in Clips/ on the Kobo's storage, and as Clips/latest.png and
// latest.txt too, for a program in Alpine to take: an LLM's, say), or
// ×. A tap elsewhere or Esc takes it away, as does turning the page.
// With the select tool in hand and nothing cut, "copy page" copies the
// whole page so: to paste it in a notebook, to write on.

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
	cutCopyDPI = 227  // a cut copied: a notebook's page unit a pixel, the Elipsa's screen
	cutDPI     = 300  // a cut saved: as sharp as a print
	cutMaxPx   = 3000 // but no bigger, along its longer side
	cutMin     = 24   // px: a rectangle smaller isn't one
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
		case (g.Pen || w.selecting) && g.Moved > selStill: // the select tool's finger draws at once, as the pen
		case g.Pen || w.selecting:
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
		w.dropCut()
		w.cut = r
	}
	w.draw(a)
	return true
}

const cornerHit = 56 // px from a corner's dot a tap picks it

// dropCut takes the cut away, and what the select tool had placed.
func (w *bookWin) dropCut() { w.cut, w.corner, w.grab = image.Rectangle{}, nil, 0 }

// corners are the cut's, clockwise from the top left.
func (w *bookWin) corners() [4]image.Point {
	r := w.cut
	return [4]image.Point{r.Min, {r.Max.X - 1, r.Min.Y}, r.Max.Sub(image.Pt(1, 1)), {r.Min.X, r.Max.Y - 1}}
}

// selectTap is a tap on the page with the select tool: the cut's
// buttons; a picked corner put there; a corner's dot picked; else a
// corner placed, the first or the second (and the cut is made).
func (w *bookWin) selectTap(a *bro, p image.Point) {
	for _, r := range w.cutRows {
		if p.In(r.r) {
			r.do()
			return
		}
	}
	in := w.view.dst
	p = image.Pt(max(in.Min.X, min(in.Max.X-1, p.X)), max(in.Min.Y, min(in.Max.Y-1, p.Y)))
	if !w.cut.Empty() {
		cs := w.corners()
		if w.grab > 0 { // the far corner stays, this one goes to p
			r := image.Rectangle{cs[(w.grab+1)%4], p}.Canon()
			r.Max = r.Max.Add(image.Pt(1, 1))
			w.grab = 0
			if r.Dx() >= cutMin && r.Dy() >= cutMin {
				w.cut = r
			}
			w.draw(a)
			return
		}
		for i, c := range cs {
			if d := p.Sub(c); d.X*d.X+d.Y*d.Y <= cornerHit*cornerHit {
				w.grab = i + 1
				w.draw(a)
				return
			}
		}
		w.dropCut() // elsewhere: another cut, from here
	}
	if w.corner == nil {
		w.corner = &p
		w.draw(a)
		return
	}
	r := image.Rectangle{*w.corner, p}.Canon()
	r.Max = r.Max.Add(image.Pt(1, 1))
	w.corner = nil
	if r.Dx() >= cutMin && r.Dy() >= cutMin {
		w.cut = r
		log.Print("pdf: cut by taps")
	}
	w.draw(a)
}

// drawCorner draws a corner's dot: a ring, filled if it's picked.
func drawCorner(c kobo.Canvas, p image.Point, picked bool, clip image.Rectangle) {
	c.Disc(p, 17, kobo.White, clip)
	c.Disc(p, 14, kobo.Black, clip)
	if !picked {
		c.Disc(p, 9, kobo.White, clip)
	}
}

// drawCut draws the rectangle cut out, if there's one, and its buttons:
// under it, or over it if there's no room under it.
func (w *bookWin) drawCut(a *bro) {
	w.cutRows = nil
	r, c := w.cut, a.C
	if w.selecting && w.corner != nil {
		drawCorner(c, *w.corner, true, w.view.dst)
	}
	if r.Empty() {
		if w.selecting {
			w.pageButton(a)
		}
		return
	}
	c.Frame(r.Inset(-3), 3, kobo.White)
	c.Frame(r, 3, kobo.Black)
	if w.selecting {
		for i, p := range w.corners() {
			drawCorner(c, p, w.grab == i+1, w.p.Body)
		}
	}
	const h, gap = 64, 10
	labels := []string{"copy", "save", "×"}
	widths := []int{140, 140, 64}
	total := 0
	for _, bw := range widths {
		total += bw + gap
	}
	y := r.Max.Y + gap
	if y+h > w.area.Max.Y {
		y = max(w.area.Min.Y, r.Min.Y-gap-h)
	}
	x := max(w.area.Min.X, min(r.Max.X, w.area.Max.X)-total)
	do := []func(){func() { w.cutCopy(a) }, func() { w.cutSave(a) }, func() { w.dropCut(); w.draw(a) }}
	for i, label := range labels {
		b := image.Rect(x, y, x+widths[i], y+h)
		x += widths[i] + gap
		c.Fill(b, kobo.White)
		c.Frame(b, 3, kobo.Black)
		c.Text(a.F.Panel, b, label, kobo.Black, kobo.AlignCenter)
		w.cutRows = append(w.cutRows, bookRow{b, do[i]})
	}
}

// pageButton is "copy page", at the page's foot on the right.
func (w *bookWin) pageButton(a *bro) {
	const h, gap = 64, 10
	bw := kobo.TextWidth(a.F.Panel, "copy page") + 48
	b := image.Rect(w.area.Max.X-gap-bw, w.area.Max.Y-gap-h, w.area.Max.X-gap, w.area.Max.Y-gap)
	c := a.C
	c.Fill(b.Inset(-3), kobo.White)
	c.Frame(b, 3, kobo.Black)
	c.Text(a.F.Panel, b, "copy page", kobo.Black, kobo.AlignCenter)
	w.cutRows = append(w.cutRows, bookRow{b, func() { w.copyPage(a) }})
}

// copyPage copies the whole page, as a cut of it all.
func (w *bookWin) copyPage(a *bro) {
	sz, err := w.pdf.Size(w.spine)
	if err != nil {
		a.Notify("can't copy it: " + err.Error())
		return
	}
	log.Printf("pdf: page %d copied", w.spine+1)
	w.copyPart(a, pdf.Rect{X1: sz.W, Y1: sz.H}, "the page's")
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

// cutCopy copies the cut: its text, for a terminal, and its picture,
// for a notebook.
func (w *bookWin) cutCopy(a *bro) { w.copyPart(a, w.cutRect(), "its") }

// copyPart copies r of the page (in points): its text and its picture.
func (w *bookWin) copyPart(a *bro, r pdf.Rect, what string) {
	text, err := w.pdf.TextIn(w.spine, r)
	if err != nil {
		a.Notify("can't read it: " + err.Error())
		return
	}
	z := cutCopyDPI / 72.0
	m, err := w.pdf.Render(w.spine, z, image.Rect(int(r.X0*z), int(r.Y0*z), int(r.X1*z+0.5), int(r.Y1*z+0.5)))
	if err == nil {
		err = copyPicture(m)
	}
	if err != nil {
		log.Printf("pdf: copying a cut's picture: %v", err)
		a.Notify("can't copy its picture: " + err.Error())
		return
	}
	if strings.TrimSpace(text) == "" { // a scan's: the picture only
		a.Notify("copied " + what + " picture (no text in it): paste it in a notebook")
		return
	}
	a.copyText(text, what+" text and picture")
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
