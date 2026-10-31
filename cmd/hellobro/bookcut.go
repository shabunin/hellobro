package main

// Cutting a part of a PDF's page out: the pen drawn across the page, or
// a finger held still a moment then moved (as in the terminals), makes a
// rectangle. So does the select tool (its button on the bar): a
// finger dragged across at once, or taps, one corner, then the other.
// With it the rectangle changes by fingers too: a corner's dot dragged
// moves that corner, a side dragged that side, the inside the whole of
// it; or a dot tapped picks it, and the next tap puts it there. By the
// rectangle are its buttons: zoom (the page so the rectangle fills the
// window: bookzoom.go), copy it (its text into the
// clipboard, which the terminals paste; and its picture, which a
// notebook pastes: notepic.go), save it (a picture at 300 dpi and its
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
	"slices"
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
	edgeHit    = 40   // px from a side a drag moves it
)

// What of the cut a drag with the select tool moves (bookCut.resize):
// sides, two of them a corner; or all of it.
const (
	cutL = 1 << iota
	cutT
	cutR
	cutB
	cutAll
)

// bookCut is a gesture cutting a part out, while it's on.
type bookCut struct {
	w        *bookWin
	g        *kobo.Gesture
	v        pdfView         // the page it's on
	pan      bool            // a finger moving the page (bookzoom.go), not a cut
	resize   int             // the select tool's drag changing the cut: what of it (cutL...)
	orig     image.Rectangle // the cut, as it was
	panFrom  image.Point     // where the page was, its point at the window's top left
	early    float64         // how far the finger got in its first moment
	on       bool            // cutting: the gesture is ours
	from, to image.Point
	shown    image.Rectangle // the frame on the screen now
	drawn    time.Time
}

// cutStart notes a contact on a PDF's page: it may become a cut.
func (a *bro) cutStart(p image.Point, g *kobo.Gesture) bool {
	w, ok := a.windowAt(p).(*bookWin)
	if !ok || w.pdf == nil || w.list || w.styling {
		return false
	}
	v, on := w.viewAt(p)
	if !on {
		return false
	}
	s := &bookCut{w: w, v: v, g: g, from: p, to: p}
	if w.selecting && !w.cut.Empty() {
		if s.resize = w.cutHit(p); s.resize != 0 {
			s.v, s.orig = w.view, w.cut
		}
	}
	a.bk.cut = s
	return true
}

// cutHit is what of the cut a drag from p moves: a corner, a side, the
// whole (from inside), or nothing (0).
func (w *bookWin) cutHit(p image.Point) int {
	for i, c := range w.corners() {
		if d := p.Sub(c); d.X*d.X+d.Y*d.Y <= cornerHit*cornerHit {
			return [4]int{cutL | cutT, cutT | cutR, cutR | cutB, cutB | cutL}[i]
		}
	}
	r := w.cut
	near := func(a, b int) bool { return max(a-b, b-a) <= edgeHit }
	switch {
	case !p.In(r.Inset(-edgeHit)):
		return 0
	case near(p.X, r.Min.X):
		return cutL
	case near(p.X, r.Max.X):
		return cutR
	case near(p.Y, r.Min.Y):
		return cutT
	case near(p.Y, r.Max.Y):
		return cutB
	case p.In(r):
		return cutAll
	}
	return 0
}

// rect is the rectangle the gesture makes now, on its page.
func (s *bookCut) rect() image.Rectangle {
	in := s.w.view.dst
	if s.resize == 0 {
		return image.Rectangle{s.from, s.to}.Canon().Intersect(in)
	}
	d, r := s.to.Sub(s.from), s.orig
	if s.resize == cutAll { // moved whole, kept on the page
		d.X = max(in.Min.X-r.Min.X, min(in.Max.X-r.Max.X, d.X))
		d.Y = max(in.Min.Y-r.Min.Y, min(in.Max.Y-r.Max.Y, d.Y))
		return r.Add(d)
	}
	if s.resize&cutL != 0 {
		r.Min.X += d.X
	}
	if s.resize&cutT != 0 {
		r.Min.Y += d.Y
	}
	if s.resize&cutR != 0 {
		r.Max.X += d.X
	}
	if s.resize&cutB != 0 {
		r.Max.Y += d.Y
	}
	return r.Canon().Intersect(in)
}

// cutMove follows it: the pen cuts as soon as it moves, a finger once
// it's been held still a moment. The frame follows in the fast black
// and white updates, now and then.
func (a *bro) cutMove(p image.Point, g *kobo.Gesture) {
	s, w := a.bk.cut, a.bk.cut.w
	if !s.on {
		switch {
		case s.pan: // ours (ownPan)
			s.to = p
			if time.Since(s.drawn) >= 150*time.Millisecond {
				s.drawn = time.Now()
				w.panTo(a, s.panFrom.Sub(p.Sub(s.from)), false)
			}
			return
		case g.Owner != 0:
			a.bk.cut = nil
			return
		case !g.Pen && !w.selecting && g.Moved > selStill && time.Since(g.At) < selHold && w.canPan(): // moving from the start: the page moves
			s.pan, s.panFrom, s.to, g.Owner = true, s.v.sp, p, ownPan
			w.dropCut()
			log.Print("pdf: panning")
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
		if !w.cut.Empty() || w.corner != nil {
			w.cut, w.corner, w.grab = image.Rectangle{}, nil, 0
			w.draw(a)
		}
		w.view = s.v
		if s.resize != 0 {
			log.Printf("pdf: the cut dragged (%d)", s.resize)
		} else {
			log.Print("pdf: cutting")
		}
	}
	s.to = p
	if time.Since(s.drawn) < 150*time.Millisecond {
		return
	}
	s.drawn = time.Now()
	r := s.rect()
	if sh, err := w.pdfPage(w.view.spine, w.laid); err == nil && !s.shown.Empty() { // the page back under the old frame
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
	if s.pan {
		s.w.panTo(a, s.panFrom.Sub(s.to.Sub(s.from)), true)
		return true
	}
	if !s.on {
		return false
	}
	w := s.w
	if r := s.rect(); r.Dx() >= cutMin && r.Dy() >= cutMin {
		w.dropCut()
		w.cut = r
	} else if s.resize != 0 { // too small: as it was
		w.cut = s.orig
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
	for i, r := range w.cutRows {
		if p.In(r.r) {
			w.pressCut(a, i)
			return
		}
	}
	if v, ok := w.viewAt(p); ok && w.cut.Empty() && w.corner == nil { // a new cut: on this page
		w.view = v
	}
	in := w.view.dst
	at := p
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
		if v, ok := w.viewAt(at); ok {
			w.view = v
			in = v.dst
			p = image.Pt(max(in.Min.X, min(in.Max.X-1, at.X)), max(in.Min.Y, min(in.Max.Y-1, at.Y)))
		}
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
	w.cutRows, w.cutLabels = nil, nil
	r, c := w.cut, a.C
	if w.selecting && w.corner != nil {
		drawCorner(c, *w.corner, true, w.view.dst)
	}
	if r.Empty() {
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
	labels := []string{"copy", "save", "zoom", "×"}
	widths := []int{140, 140, 140, 64}
	do := []func(){func() { w.cutCopy(a) }, func() { w.cutSave(a) }, func() { w.zoomCut(a) }, func() { w.dropCut(); w.draw(a) }}
	if !w.canZoom() { // two side by side: whole, each
		labels, widths, do = slices.Delete(labels, 2, 3), slices.Delete(widths, 2, 3), slices.Delete(do, 2, 3)
	}
	total := 0
	for _, bw := range widths {
		total += bw + gap
	}
	y := r.Max.Y + gap
	if y+h > w.area.Max.Y {
		y = max(w.area.Min.Y, r.Min.Y-gap-h)
	}
	x := max(w.area.Min.X, min(r.Max.X, w.area.Max.X)-total)
	for i, label := range labels {
		b := image.Rect(x, y, x+widths[i], y+h)
		x += widths[i] + gap
		drawCutButton(a, b, label, false)
		w.cutRows, w.cutLabels = append(w.cutRows, bookRow{b, do[i]}), append(w.cutLabels, label)
	}
}

// drawCutButton draws a cut's button, dark while it's pressed.
func drawCutButton(a *bro, b image.Rectangle, label string, pressed bool) {
	bg, fg := uint8(kobo.White), uint8(kobo.Black)
	if pressed {
		bg, fg = kobo.Black, kobo.White
	}
	a.C.Fill(b, bg)
	a.C.Frame(b, 3, kobo.Black)
	a.C.Text(a.F.Panel, b, label, fg, kobo.AlignCenter)
}

// pressCut runs a cut's button r, shown pressed meanwhile: on the panel
// at once, as what it does may take a while (a copy, a save: the page
// rendered again), and a button that doesn't change looks missed. Then
// it's drawn as it was, if it's still there (zoom and × draw anew).
func (w *bookWin) pressCut(a *bro, i int) {
	r, label := w.cutRows[i], w.cutLabels[i]
	drawCutButton(a, r.r, label, true)
	a.Mark(r.r, false) // black and white: the fast waveform
	a.Show()
	r.do()
	if i < len(w.cutRows) && w.cutRows[i].r == r.r && w.cutLabels[i] == label && !w.cut.Empty() {
		drawCutButton(a, r.r, label, false)
		a.Mark(r.r, false)
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

// cutCopy copies the cut: its text, for a terminal, and its picture,
// for a notebook.
func (w *bookWin) cutCopy(a *bro) {
	r := w.cutRect()
	text, err := w.pdf.TextIn(w.view.spine, r)
	if err != nil {
		a.Notify("can't read it: " + err.Error())
		return
	}
	z := pdf.CapZoom(r.X1-r.X0, r.Y1-r.Y0, min(cutCopyDPI/72.0, cutMaxPx/max(r.X1-r.X0, r.Y1-r.Y0)))
	m, err := w.pdf.Render(w.view.spine, z, image.Rect(int(r.X0*z), int(r.Y0*z), int(r.X1*z+0.5), int(r.Y1*z+0.5)))
	if err == nil {
		err = copyPicture(m)
	}
	if err != nil {
		log.Printf("pdf: copying a cut's picture: %v", err)
		a.Notify("can't copy its picture: " + err.Error())
		return
	}
	if strings.TrimSpace(text) == "" { // a scan's: the picture only
		a.Notify("copied its picture (no text in it): paste it in a notebook")
		return
	}
	a.copyText(text, "its text and picture")
}

// cutSave saves the cut, a picture and its text, away from the main
// loop: drawn big, it takes a moment.
func (w *bookWin) cutSave(a *bro) {
	d, spine, r := w.pdf, w.view.spine, w.cutRect()
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
	z := pdf.CapZoom(r.X1-r.X0, r.Y1-r.Y0, min(cutDPI/72.0, cutMaxPx/max(r.X1-r.X0, r.Y1-r.Y0)))
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
