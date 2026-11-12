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
//
// An EPUB's page is cut the same way, by the pen or the select tool (a
// finger held there looks a word up: bookdict.go): its picture is the
// page as the screen has it, its text the words in the rectangle. No
// zoom.

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
	"unicode/utf8"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"

	"hellobro/book"
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
	if !ok || w.list || w.styling || w.b == nil || w.pdf == nil && !w.selecting && !g.Pen {
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
		case !g.Pen && !w.selecting && g.Moved > selStill && time.Since(g.At) < selHold && w.canPan() && !g.Unfulling(p): // moving from the start: the page moves (down from the top, full screen, the bar's: topSwipe)
			s.pan, s.panFrom, s.to, g.Owner = true, s.v.sp, p, ownPan
			w.dropCut()
			log.Print("pdf: panning")
			return
		case !g.Pen && w.selecting && g.Unfulling(p): // down from the top, full screen: it may be the bar's yet (topSwipe)
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
			log.Printf("book: the cut dragged (%d)", s.resize)
		} else {
			log.Print("book: cutting")
		}
	}
	s.to = p
	if time.Since(s.drawn) < 150*time.Millisecond {
		return
	}
	s.drawn = time.Now()
	r := s.rect()
	if !s.shown.Empty() { // the page back under the old frame
		w.pageUnder(a, s.shown.Intersect(w.view.dst))
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

// cutLetGo gives up g, a finger cutting or moving the page, as a second
// finger makes it half of a pair: the page as it was before it.
func (a *bro) cutLetGo(g *kobo.Gesture) bool {
	s := a.bk.cut
	if s == nil || s.g != g {
		return false
	}
	a.bk.cut = nil
	w := s.w
	switch {
	case s.pan:
		w.panTo(a, s.panFrom, true)
	case s.on:
		if s.resize != 0 {
			w.cut = s.orig
		}
		w.draw(a) // its frame away
	}
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
		log.Print("book: cut by taps")
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
	a.inkPaint(&a.bk.cutPane, b, a.inkButton(label, pressed))
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
	if w.pdf == nil {
		m, text := w.epubCut(a)
		if err := copyPicture(m); err != nil {
			log.Printf("book: copying a cut's picture: %v", err)
			a.Notify("can't copy its picture: " + err.Error())
			return
		}
		if strings.TrimSpace(text) == "" { // a picture's
			a.Notify("copied its picture: paste it in a notebook")
			return
		}
		a.copyText(text, "its text and picture")
		return
	}
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
	dir := filepath.Join(booksRoot(), "Clips")
	stamp := time.Now().Format("2006-01-02 150405")
	var name string
	var save func() error
	if w.pdf == nil { // as the screen has it: there's nothing to draw bigger
		m, text := w.epubCut(a)
		name = fmt.Sprintf("%s, %d%%, %s", fileName(w.Title(), 40), w.pct(), stamp)
		save = func() error { return writeClip(dir, name, m, text) }
	} else {
		d, spine, r := w.pdf, w.view.spine, w.cutRect()
		name = fmt.Sprintf("%s, p%d, %s", fileName(w.Title(), 40), spine+1, stamp)
		save = func() error { return saveCut(d, spine, r, dir, name) }
	}
	a.Notify("saving…")
	go func() {
		err := save()
		a.Do(func() {
			if err != nil {
				log.Printf("book: saving a cut: %v", err)
				a.Notify("can't save it: " + err.Error())
				return
			}
			log.Printf("book: saved %s", name)
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
	text, _ := d.TextIn(spine, r)
	return writeClip(dir, name, m, text)
}

// writeClip writes a cut's picture and text, as name and as latest.
func writeClip(dir, name string, m image.Image, text string) error {
	var pic bytes.Buffer
	if err := png.Encode(&pic, m); err != nil {
		return err
	}
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

// --- an EPUB's ---

// bookShotOf is what an EPUB's shot is of: the pages, as laid out, where.
type bookShotOf struct {
	ch   *book.Chapter
	page int
	laid bookLaid
	body image.Rectangle
}

// epubShot is the EPUB's pages shown, drawn again aside: what a cut
// takes, without its frame and dots, a note's box or the bar.
func (w *bookWin) epubShot(a *bro) *image.Gray {
	of := bookShotOf{w.ch, w.page, w.laid, w.p.Body}
	if w.shot != nil && w.shotOf == of {
		return w.shot
	}
	m := image.NewGray(w.p.Body)
	c := kobo.Canvas{Gray: m}
	c.Fill(m.Rect, kobo.White)
	pics := map[string]*image.Gray{}
	for _, s := range w.shownPages() {
		w.drawLaid(a, c, s.n, s.at, pics)
	}
	w.shot, w.shotOf = m, of
	return m
}

// epubCut is the cut of an EPUB's page: its picture, as the screen has
// it, and its text.
func (w *bookWin) epubCut(a *bro) (*image.Gray, string) {
	r := w.cut.Intersect(w.p.Body)
	m := image.NewGray(image.Rectangle{Max: r.Size()})
	kobo.CopyGray(m, m.Rect, w.epubShot(a), r.Min)
	return m, w.epubText(a, r)
}

// epubText is the text of the pages shown in r: of each line the letters
// whose middles are in it, a line a line, a word broken by a hyphen at a
// line's end joined again.
func (w *bookWin) epubText(a *bro, r image.Rectangle) string {
	faces := w.bookFaces(a)
	var b strings.Builder
	line, end := -1, 0 // the line the last bit was on (its baseline), and where it ended
	for _, s := range w.shownPages() {
		if line >= 0 {
			line = -1 // the next page: a line of its own
			b.WriteString("\n")
		}
		for _, it := range w.pages[s.n].Items {
			if it.Kind != book.ItemText || it.Text == "" {
				continue
			}
			base := s.at.Y + it.Y
			if mid := base - it.Px/3; mid < r.Min.Y || mid >= r.Max.Y {
				continue
			}
			from, to, x0, x1 := lettersIn(it.Text, faces.Face(it.Style, it.Px), r.Min.X-s.at.X-it.X, r.Max.X-s.at.X-it.X)
			if from >= to {
				continue
			}
			bit := it.Text[from:to]
			switch {
			case line < 0:
			case kobo.Abs(base-line) < it.Px/2: // the same line: a space if they don't touch
				if x0+s.at.X+it.X-end > it.Px/6 && !strings.HasSuffix(b.String(), " ") && !strings.HasPrefix(bit, " ") {
					b.WriteByte(' ')
				}
			default: // the next line: a word broken at its end joined again
				t := strings.TrimRight(b.String(), " ")
				if cut, ok := trimHyphen(t); ok {
					t = cut
				} else {
					t += "\n"
				}
				b.Reset()
				b.WriteString(t)
			}
			b.WriteString(bit)
			line, end = base, s.at.X+it.X+x1
		}
	}
	return strings.TrimSpace(b.String())
}

// lettersIn is which of text's letters, drawn in f from 0, have their
// middles between px lo and hi: bytes [from, to), from x0 to x1 px.
func lettersIn(text string, f font.Face, lo, hi int) (from, to, x0, x1 int) {
	from = -1
	var x fixed.Int26_6
	prev := rune(-1)
	for i, r := range text {
		if prev >= 0 {
			x += f.Kern(prev, r)
		}
		adv, _ := f.GlyphAdvance(r)
		if mid := (x + adv/2).Round(); mid >= lo && mid < hi {
			if from < 0 {
				from, x0 = i, x.Round()
			}
			to, x1 = i+utf8.RuneLen(r), (x + adv).Round()
		}
		x += adv
		prev = r
	}
	if from < 0 {
		return 0, 0, 0, 0
	}
	return
}
