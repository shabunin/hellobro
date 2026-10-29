package main

// The notebook's select tool: a tap on a stroke or a picture takes it, a
// rectangle drawn round some takes all that are wholly in it. What's
// taken, dragged, moves, onto the page beside it too where two show: one
// change, to undo. Delete (or Backspace) takes it off the page. A tap on
// the paper where nothing is, nothing taken, marks where the next paste
// goes (a cross); what's pasted comes in taken, to be dragged where it
// should be. The pen and fingers both select while the tool's in hand.

import (
	"image"
	"log"
	"slices"

	"hellobro/kobo"
)

const (
	ownNoteSel   = 9  // the select tool's contact on a notebook's page
	noteSelPad   = 10 // px between what's taken and its frame
	noteSelStill = 12 // px a tap may move
	noteMarkArm  = 18 // px, the paste mark's cross: each arm
)

// noteDrag is the select tool's contact: drawing a rectangle, or moving
// what's taken.
type noteDrag struct {
	w      *noteWin
	g      *kobo.Gesture
	page   int
	at     image.Rectangle // its page on screen
	from   image.Point
	moving bool
	shown  image.Rectangle // the frame drawn last, on screen
}

// noteMark is where the next paste goes: a page, and a point on it.
type noteMark struct {
	page int
	at   image.Point
}

// setTool takes the pen, the eraser or the select tool; what was taken
// and the paste mark go with the select tool.
func (w *noteWin) setTool(a *bro, id string) {
	w.erasing, w.selecting = id == "eraser", id == "select"
	if !w.selecting && (len(w.sel) > 0 || w.mark != nil) {
		w.sel, w.mark = nil, nil
		w.show(a)
		return
	}
	if a.shows(w) {
		w.drawBar(a)
		a.Mark(w.bar, true)
	}
}

// slotOf is where page n shows, if it does.
func (w *noteWin) slotOf(n int) (image.Rectangle, bool) {
	if i := n - w.cur; i >= 0 && i < len(w.slots) {
		return w.slots[i], true
	}
	return image.Rectangle{}, false
}

// paperArea is the window under the tools' strip.
func (w *noteWin) paperArea() image.Rectangle {
	return image.Rect(w.p.Body.Min.X, w.bar.Max.Y, w.p.Body.Max.X, w.p.Body.Max.Y)
}

// selBox is what's taken, all told, in page units.
func (w *noteWin) selBox() image.Rectangle {
	var r image.Rectangle
	for _, s := range w.sel {
		r = r.Union(s.box)
	}
	return r
}

// selScreen is the frame round what's taken, on screen; empty if it
// doesn't show.
func (w *noteWin) selScreen() image.Rectangle {
	at, ok := w.slotOf(w.selPage)
	if len(w.sel) == 0 || !ok {
		return image.Rectangle{}
	}
	return w.toScreenRect(w.selBox(), at).Inset(-noteSelPad)
}

// markScreen is the paste mark's cross on screen; empty if it doesn't
// show.
func (w *noteWin) markScreen() (image.Point, image.Rectangle) {
	if w.mark == nil {
		return image.Point{}, image.Rectangle{}
	}
	at, ok := w.slotOf(w.mark.page)
	if !ok {
		return image.Point{}, image.Rectangle{}
	}
	p := w.toScreen(notePt{x: int16(w.mark.at.X), y: int16(w.mark.at.Y)}, at)
	return p, image.Rectangle{p, p}.Inset(-noteMarkArm - 2)
}

// frameStrips are r's four sides, t thick.
func frameStrips(r image.Rectangle, t int) []image.Rectangle {
	if r.Empty() {
		return nil
	}
	return []image.Rectangle{
		{r.Min, image.Pt(r.Max.X, r.Min.Y+t)}, {image.Pt(r.Min.X, r.Max.Y-t), r.Max},
		{r.Min, image.Pt(r.Min.X+t, r.Max.Y)}, {image.Pt(r.Max.X-t, r.Min.Y), r.Max},
	}
}

// drawSel draws, over the pages where they're in clip, the frame round
// what's taken (dashed) and the paste mark.
func (w *noteWin) drawSel(a *bro, clip image.Rectangle) {
	c := a.C
	clip = clip.Intersect(w.paperArea())
	for _, s := range frameStrips(w.selScreen(), 3) {
		long := s.Dx() > s.Dy()
		for i := 0; ; i += 20 { // dashes: 12 on, 8 off
			d := image.Rect(s.Min.X+i, s.Min.Y, s.Min.X+i+12, s.Max.Y)
			if !long {
				d = image.Rect(s.Min.X, s.Min.Y+i, s.Max.X, s.Min.Y+i+12)
			}
			d = d.Intersect(s)
			if d.Empty() {
				break
			}
			c.Fill(d.Intersect(clip), kobo.Black)
		}
	}
	if p, r := w.markScreen(); !r.Empty() {
		c.Fill(image.Rect(p.X-noteMarkArm, p.Y-1, p.X+noteMarkArm+1, p.Y+2).Intersect(clip), kobo.Black)
		c.Fill(image.Rect(p.X-1, p.Y-noteMarkArm, p.X+2, p.Y+noteMarkArm+1).Intersect(clip), kobo.Black)
	}
}

// redraw draws the pages again in each of rs, and shows them.
func (w *noteWin) redraw(a *bro, rs ...image.Rectangle) {
	if !a.shows(w) {
		return
	}
	for _, r := range rs {
		if r = r.Intersect(w.paperArea()); !r.Empty() {
			w.drawPages(a, r)
			a.Mark(r, true)
		}
	}
}

// noteSelTouch starts the select tool's contact on the page at slot.
func (a *bro) noteSelTouch(w *noteWin, slot int, p image.Point, g *kobo.Gesture) bool {
	if a.nt.drag != nil || a.nt.ink != nil {
		return false
	}
	g.Owner = ownNoteSel
	d := &noteDrag{w: w, g: g, page: w.cur + slot, at: w.slots[slot], from: p}
	d.moving = w.selPage == d.page && p.In(w.selScreen())
	a.nt.drag = d
	return true
}

// noteDragTo follows it: a frame where the rectangle is, or where what's
// taken would go, drawn fast.
func (a *bro) noteDragTo(p image.Point, g *kobo.Gesture) {
	d := a.nt.drag
	if d == nil || d.g != g || !a.shows(d.w) || d.shown.Empty() && g.Moved < noteSelStill {
		return
	}
	w := d.w
	r := image.Rectangle{d.from, p}.Canon().Inset(-1)
	if d.moving {
		r = w.selScreen().Add(p.Sub(d.from))
	}
	if d.shown.Empty() {
		a.Ink(true)
	}
	old := frameStrips(d.shown, 2)
	d.shown = r
	for _, s := range old {
		w.drawPages(a, s.Intersect(w.paperArea()))
	}
	cl := w.paperArea()
	for _, s := range frameStrips(r, 2) {
		a.C.Fill(s.Intersect(cl), kobo.Black)
	}
	for _, s := range append(old, frameStrips(r, 2)...) {
		a.Mark(s.Intersect(cl), false)
	}
}

// noteDragEnd ends it: a tap takes what's there (or marks where to
// paste), a rectangle takes what's in it, a drag moves what's taken.
func (a *bro) noteDragEnd(g *kobo.Gesture) {
	d := a.nt.drag
	if d == nil || d.g != g {
		return
	}
	a.nt.drag = nil
	w := d.w
	defer a.noteFocus(w)
	before, mark := w.selScreen(), image.Rectangle{}
	if _, r := w.markScreen(); !r.Empty() {
		mark = r
	}
	if !d.shown.Empty() {
		a.Ink(false)
	}
	switch {
	case d.shown.Empty():
		w.selectAt(d.page, w.toPage(d.from, d.at))
	case d.moving:
		page, at := d.page, d.at
		for i, s := range w.slots {
			if g.Last.In(s) {
				page, at = w.cur+i, s
			}
		}
		w.moveSel(page, w.toPage(g.Last, at).Sub(w.toPage(d.from, d.at)))
	default:
		r := image.Rectangle{w.toPage(d.from, d.at), w.toPage(g.Last, d.at)}.Canon()
		w.sel, w.selPage, w.mark = nil, d.page, nil
		for _, s := range w.strokes(d.page) {
			if s.box.In(r) {
				w.sel = append(w.sel, s)
			}
		}
		log.Printf("notebook %s: %d taken", w.nb.name, len(w.sel))
	}
	_, after := w.markScreen()
	w.redraw(a, append(frameStrips(d.shown, 2), before, mark, w.selScreen(), after)...)
	if a.shows(w) {
		w.drawBar(a) // undo, after a move
		a.Mark(w.bar, true)
	}
}

// selectAt takes what's at p (page units) on page: the ink over a
// picture. Nothing there, it lets go of what was taken, or, nothing
// taken, marks p for a paste (again on the mark: no mark).
func (w *noteWin) selectAt(page int, p image.Point) {
	var hit *noteStroke
	for _, ink := range []bool{true, false} { // the last written first
		for _, s := range slices.Backward(w.strokes(page)) {
			if hit == nil && (ink && s.hits(p) || !ink && s.pic != nil && p.In(s.box)) {
				hit = s
			}
		}
	}
	switch {
	case hit != nil:
		w.sel, w.selPage, w.mark = []*noteStroke{hit}, page, nil
	case len(w.sel) > 0:
		w.sel = nil
	case w.mark != nil && w.mark.page == page && kobo.Abs(w.mark.at.X-p.X) < 3*noteMarkArm && kobo.Abs(w.mark.at.Y-p.Y) < 3*noteMarkArm:
		w.mark = nil
	default:
		w.mark = &noteMark{page, p}
	}
}

// moveSel moves what's taken by d (page units), onto page, kept on it.
func (w *noteWin) moveSel(page int, d image.Point) {
	b := w.selBox()
	d = d.Add(into(b.Add(d), image.Rect(0, 0, notePageW, notePageH)))
	if len(w.sel) == 0 || d == (image.Point{}) && page == w.selPage {
		return
	}
	var moved []*noteStroke
	for _, s := range w.sel {
		moved = append(moved, s.moved(d))
	}
	for len(w.nb.pages) <= page {
		w.nb.pages = append(w.nb.pages, nil)
	}
	ops := []noteOp{{page: w.selPage, del: w.sel}, {page: page, add: moved}}
	if page == w.selPage {
		ops = []noteOp{{page: page, del: w.sel, add: moved}}
	}
	for _, op := range ops {
		w.nb.apply(op, false)
	}
	w.edited(ops...)
	log.Printf("notebook %s: %d moved by %v to page %d", w.nb.name, len(moved), d, page+1)
	w.sel, w.selPage = moved, page
}

// into is how far r goes to be inside in, as far as it fits.
func into(r, in image.Rectangle) image.Point {
	var d image.Point
	if r.Max.X > in.Max.X {
		d.X = in.Max.X - r.Max.X
	}
	if r.Min.X+d.X < in.Min.X {
		d.X = in.Min.X - r.Min.X
	}
	if r.Max.Y > in.Max.Y {
		d.Y = in.Max.Y - r.Max.Y
	}
	if r.Min.Y+d.Y < in.Min.Y {
		d.Y = in.Min.Y - r.Min.Y
	}
	return d
}

// moved is s, by d: a new one, so undo puts back the old.
func (s *noteStroke) moved(d image.Point) *noteStroke {
	if s.pic != nil {
		return picStroke(s.pic, s.box.Add(d))
	}
	m := &noteStroke{attrs: s.attrs, pts: make([]notePt, len(s.pts))}
	clamp := func(v int) int16 { return int16(max(-1<<15, min(1<<15-1, v))) }
	for i, p := range s.pts {
		m.pts[i] = notePt{clamp(int(p.x) + d.X), clamp(int(p.y) + d.Y), p.r}
	}
	m.bound()
	return m
}

// deleteSel takes what's taken off its page.
func (w *noteWin) deleteSel(a *bro) {
	if len(w.sel) == 0 {
		return
	}
	op := noteOp{page: w.selPage, del: w.sel}
	w.nb.apply(op, false)
	w.edited(op)
	log.Printf("notebook %s: %d deleted", w.nb.name, len(w.sel))
	w.sel = nil
	w.show(a)
}

// pasteWhere is where a picture of size goes: at the paste mark if
// there's one (shrunk as pastePlace would), else where pastePlace says.
func (w *noteWin) pasteWhere(size image.Point) (int, image.Rectangle) {
	page, box := w.pastePlace(size)
	if m := w.mark; m != nil {
		page, box = m.page, image.Rectangle{Max: box.Size()}.Add(m.at)
		box = box.Add(into(box, image.Rect(0, 0, notePageW, notePageH)))
		w.mark = nil
	}
	return page, box
}

// pasted takes s, just pasted on page, with the select tool, to be
// dragged where it should be.
func (w *noteWin) pasted(page int, s *noteStroke) {
	w.erasing, w.selecting = false, true
	w.sel, w.selPage = []*noteStroke{s}, page
}
