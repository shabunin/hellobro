package main

// The notebook's select tool: a tap on a stroke or a picture takes it, a
// rectangle drawn round some takes all that are wholly in it. A dashed
// frame goes round what's taken, a dot at each corner, one over it:
//
//   - dragged inside the frame, it moves, onto the page beside it too
//     where two show;
//   - a corner's dot dragged makes it bigger or smaller, the corner
//     across staying put, as wide for as high as it was;
//   - the dot over it dragged round turns it (pictures by quarter turns:
//     Xournal++ doesn't turn them any other way);
//   - under it: copy, cut, duplicate, turn (a quarter, clockwise),
//     delete.
//
// Each is one change, to undo. What's copied is a .xopp of its own
// (clipboard.xopp, by the clipboard's picture): pasted in this notebook
// or another, where it was on its page, or at the paste mark: a tap on
// the paper where nothing is, nothing taken, puts a cross there. What's
// pasted comes in taken, to be dragged where it should be. The pen and
// fingers both select while the tool's in hand; what's done stays on
// the page.

import (
	"bytes"
	"image"
	"image/png"
	"log"
	"math"
	"os"
	"slices"

	"hellobro/kobo"
)

const (
	ownNoteSel   = 9  // the select tool's contact on a notebook's page
	noteSelPad   = 10 // px between what's taken and its frame
	noteSelStill = 12 // px a tap may move
	noteMarkArm  = 18 // px, the paste mark's cross: each arm
	noteHandle   = 40 // px from a dot that still takes it
	noteTurnArm  = 56 // px from the frame to the turning dot
	noteDupStep  = 40 // page units a duplicate goes down and right
	clipInk      = "clipboard.xopp"
)

// The select tool's drags.
const (
	dragRect = iota
	dragMove
	dragScale
	dragTurn
	dragButton
)

// noteDrag is the select tool's contact: drawing a rectangle, or moving
// what's taken, scaling it, turning it; or a tap on its buttons.
type noteDrag struct {
	w      *noteWin
	g      *kobo.Gesture
	page   int
	at     image.Rectangle // its page on screen
	from   image.Point
	kind   int
	corner int               // dragScale: which (0 top left, clockwise)
	shown  []image.Rectangle // what the last frame drew over, on screen
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

// boxOf is what ss cover, all told, in page units.
func boxOf(ss []*noteStroke) image.Rectangle {
	var r image.Rectangle
	for _, s := range ss {
		r = r.Union(s.box)
	}
	return r
}

func (w *noteWin) selBox() image.Rectangle { return boxOf(w.sel) }

// selScreen is the frame round what's taken, on screen; empty if it
// doesn't show.
func (w *noteWin) selScreen() image.Rectangle {
	at, ok := w.slotOf(w.selPage)
	if len(w.sel) == 0 || !ok {
		return image.Rectangle{}
	}
	return w.toScreenRect(w.selBox(), at).Inset(-noteSelPad)
}

// corners are a frame's, from the top left, clockwise.
func corners(r image.Rectangle) [4]image.Point {
	return [4]image.Point{r.Min, {r.Max.X, r.Min.Y}, r.Max, {r.Min.X, r.Max.Y}}
}

// turnDot is where the turning dot is: over the frame, or under it if
// there's no room over it.
func (w *noteWin) turnDot(r image.Rectangle) image.Point {
	if r.Min.Y-noteTurnArm > w.paperArea().Min.Y+noteHandle/2 {
		return image.Pt((r.Min.X+r.Max.X)/2, r.Min.Y-noteTurnArm)
	}
	return image.Pt((r.Min.X+r.Max.X)/2, r.Max.Y+noteTurnArm)
}

// selButtons are the buttons by what's taken: under it, or over it if
// there's no room under it (nor the turning dot there).
func (w *noteWin) selButtons(a *bro) []noteBtn {
	r := w.selScreen()
	if r.Empty() {
		return nil
	}
	area := w.paperArea()
	h := 48
	var bs []noteBtn
	x := 0
	for _, id := range []string{"copy", "cut", "duplicate", "turn", "delete"} {
		b := noteBtn{id: id, label: id, r: image.Rect(x, 0, x+kobo.TextWidth(a.F.Small, id)+32, h)}
		bs = append(bs, b)
		x = b.r.Max.X + 8
	}
	row := image.Rect(0, 0, x-8, h)
	y := r.Max.Y + 16
	if w.turnDot(r).Y > r.Max.Y {
		y += noteTurnArm
	}
	if y+h > area.Max.Y {
		y = r.Min.Y - 16 - h
		if w.turnDot(r).Y < r.Min.Y {
			y -= noteTurnArm
		}
		if y < area.Min.Y { // as big as the page: inside, at its top
			y = r.Min.Y + 16
		}
	}
	off := image.Pt(r.Min.X, y)
	off.X += into(row.Add(off), area).X
	for i := range bs {
		bs[i].r = bs[i].r.Add(off)
	}
	return bs
}

// selArea is all the select tool draws round what's taken: the frame,
// its dots, its buttons.
func (w *noteWin) selArea(a *bro) image.Rectangle {
	r := w.selScreen()
	if r.Empty() {
		return r
	}
	t := w.turnDot(r)
	out := r.Union(image.Rectangle{t, t.Add(image.Pt(1, 1))}).Inset(-noteHandle / 2) // a point is an empty rectangle: Union would leave it out
	for _, b := range w.selButtons(a) {
		out = out.Union(b.r)
	}
	return out
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

// handleDot is a dot to take: white round black.
func handleDot(c kobo.Canvas, p image.Point, clip image.Rectangle) {
	c.Disc(p, 14, kobo.White, clip)
	c.Disc(p, 11, kobo.Black, clip)
	c.Disc(p, 6, kobo.White, clip)
}

// drawSel draws, over the pages where they're in clip, the frame round
// what's taken (dashed), its dots and buttons, and the paste mark.
func (w *noteWin) drawSel(a *bro, clip image.Rectangle) {
	c := a.C
	clip = clip.Intersect(w.paperArea())
	r := w.selScreen()
	for _, s := range frameStrips(r, 3) {
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
	if !r.Empty() {
		t := w.turnDot(r)
		edge := image.Pt(t.X, r.Min.Y)
		if t.Y > r.Max.Y {
			edge.Y = r.Max.Y
		}
		c.Line(edge, t, 1, kobo.Black, clip)
		handleDot(c, t, clip)
		for _, p := range corners(r) {
			handleDot(c, p, clip)
		}
		for _, b := range w.selButtons(a) {
			if b.r.Overlaps(clip) {
				c.Fill(b.r, kobo.White)
				c.Frame(b.r, 2, kobo.Black)
				c.Text(a.F.Small, b.r, b.label, kobo.Black, kobo.AlignCenter)
			}
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

// selChanged shows what changed round what's taken: where it was
// (before, its selArea), and where it is now.
func (w *noteWin) selChanged(a *bro, before ...image.Rectangle) {
	w.redraw(a, append(before, w.selArea(a))...)
	if a.shows(w) {
		w.drawBar(a) // undo
		a.Mark(w.bar, true)
	}
}

// near is whether p is within d px of q.
func near(p, q image.Point, d int) bool {
	return kobo.Abs(p.X-q.X) <= d && kobo.Abs(p.Y-q.Y) <= d
}

// noteSelTouch starts the select tool's contact: on what's taken (its
// buttons, its dots, inside its frame) wherever that is, else on the
// page at slot (-1: none).
func (a *bro) noteSelTouch(w *noteWin, slot int, p image.Point, g *kobo.Gesture) bool {
	if a.nt.drag != nil || a.nt.ink != nil {
		return false
	}
	d := &noteDrag{w: w, g: g, from: p, kind: dragRect}
	if r := w.selScreen(); !r.Empty() {
		d.page, d.at = w.selPage, w.slots[w.selPage-w.cur]
		switch cs := corners(r); {
		case slices.ContainsFunc(w.selButtons(a), func(b noteBtn) bool { return p.In(b.r) }):
			d.kind = dragButton
		case near(p, w.turnDot(r), noteHandle):
			d.kind = dragTurn
		case slices.ContainsFunc(cs[:], func(c image.Point) bool { return near(p, c, noteHandle) }):
			d.kind = dragScale
			d.corner = slices.IndexFunc(cs[:], func(c image.Point) bool { return near(p, c, noteHandle) })
		case p.In(r):
			d.kind = dragMove
		}
	}
	if d.kind == dragRect {
		if slot < 0 {
			return false
		}
		d.page, d.at = w.cur+slot, w.slots[slot]
	}
	g.Owner = ownNoteSel
	a.nt.drag = d
	return true
}

// scaleTo is how much bigger what's taken would be, its corner dragged
// to p, and the corner across that stays: on screen.
func (w *noteWin) scaleTo(d *noteDrag, p image.Point) (float64, image.Point) {
	cs := corners(w.selScreen())
	anchor, from := cs[(d.corner+2)%4], cs[d.corner]
	dist := func(q image.Point) float64 { return math.Hypot(float64(q.X-anchor.X), float64(q.Y-anchor.Y)) }
	return max(0.1, min(10, dist(p)/max(1, dist(from)))), anchor
}

// angleTo is the angle (radians, clockwise) what's taken would turn by,
// its dot dragged to p: by quarters if it has a picture, else free but
// for near quarters.
func (w *noteWin) angleTo(d *noteDrag, p image.Point) float64 {
	r := w.selScreen()
	c := r.Min.Add(r.Max).Div(2)
	angle := func(q image.Point) float64 { return math.Atan2(float64(q.Y-c.Y), float64(q.X-c.X)) }
	th := angle(p) - angle(d.from)
	q := math.Round(th/(math.Pi/2)) * math.Pi / 2
	if slices.ContainsFunc(w.sel, func(s *noteStroke) bool { return s.pic != nil }) || math.Abs(th-q) < 6*math.Pi/180 {
		th = q
	}
	return th
}

// outline is where what's taken would be, d dragged to p, on screen: a
// polygon.
func (w *noteWin) outline(d *noteDrag, p image.Point) []image.Point {
	r := w.selScreen()
	switch d.kind {
	case dragMove:
		r = r.Add(p.Sub(d.from))
	case dragScale:
		k, o := w.scaleTo(d, p)
		var pts []image.Point
		for _, c := range corners(r) {
			pts = append(pts, image.Pt(o.X+int(float64(c.X-o.X)*k), o.Y+int(float64(c.Y-o.Y)*k)))
		}
		return pts
	case dragTurn:
		th := w.angleTo(d, p)
		c := r.Min.Add(r.Max).Div(2)
		var pts []image.Point
		for _, q := range corners(r) {
			x, y := float64(q.X-c.X), float64(q.Y-c.Y)
			pts = append(pts, image.Pt(c.X+int(x*math.Cos(th)-y*math.Sin(th)), c.Y+int(x*math.Sin(th)+y*math.Cos(th))))
		}
		return pts
	default:
		r = image.Rectangle{d.from, p}.Canon()
	}
	cs := corners(r)
	return cs[:]
}

// noteDragTo follows it: an outline where the rectangle is, or where
// what's taken would go, drawn fast.
func (a *bro) noteDragTo(p image.Point, g *kobo.Gesture) {
	d := a.nt.drag
	if d == nil || d.g != g || !a.shows(d.w) || d.kind == dragButton || d.shown == nil && g.Moved < noteSelStill {
		return
	}
	w := d.w
	if d.shown == nil {
		a.Ink(true)
	}
	cl := w.paperArea()
	old := d.shown
	for _, s := range old {
		w.drawPages(a, s.Intersect(cl))
	}
	pts := w.outline(d, p)
	d.shown = nil
	for i, q := range pts {
		n := pts[(i+1)%len(pts)]
		a.C.Line(q, n, 1, kobo.Black, cl)
		r := image.Rectangle{q, n}.Canon().Inset(-3)
		if d.kind == dragTurn && r.Dx() > 40 && r.Dy() > 40 { // a slanted side: its box, in pieces
			for j := range 8 {
				a2 := q.Add(n.Sub(q).Mul(j).Div(8))
				b2 := q.Add(n.Sub(q).Mul(j + 1).Div(8))
				d.shown = append(d.shown, image.Rectangle{a2, b2}.Canon().Inset(-3))
			}
			continue
		}
		d.shown = append(d.shown, r)
	}
	for _, s := range append(old, d.shown...) {
		a.Mark(s.Intersect(cl), false)
	}
}

// noteDragEnd ends it: a tap takes what's there (or marks where to
// paste, or is a button's), a rectangle takes what's in it, a drag
// moves, scales or turns what's taken.
func (a *bro) noteDragEnd(g *kobo.Gesture) {
	d := a.nt.drag
	if d == nil || d.g != g {
		return
	}
	a.nt.drag = nil
	w := d.w
	defer a.noteFocus(w)
	before, mark := w.selArea(a), image.Rectangle{}
	if _, r := w.markScreen(); !r.Empty() {
		mark = r
	}
	if d.shown != nil {
		a.Ink(false)
	}
	switch {
	case d.kind == dragButton:
		for _, b := range w.selButtons(a) {
			if g.Last.In(b.r) && g.Moved < noteSelStill {
				w.selAct(a, b.id)
				return
			}
		}
	case d.shown == nil:
		w.selectAt(d.page, w.toPage(d.from, d.at))
	case d.kind == dragMove:
		page, at := d.page, d.at
		for i, s := range w.slots {
			if g.Last.In(s) {
				page, at = w.cur+i, s
			}
		}
		w.moveSel(page, w.toPage(g.Last, at).Sub(w.toPage(d.from, d.at)))
	case d.kind == dragScale:
		k, o := w.scaleTo(d, g.Last)
		w.scaleSel(k, w.toPage(o, d.at))
	case d.kind == dragTurn:
		w.turnSel(w.angleTo(d, g.Last))
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
	w.selChanged(a, append(d.shown, before, mark, after)...)
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

// replaceSel puts made on page for what's taken (nil: on its own page),
// kept on the page, as one change, and takes it.
func (w *noteWin) replaceSel(page int, made []*noteStroke, what string) {
	if len(w.sel) == 0 || len(made) == 0 {
		return
	}
	if d := into(boxOf(made), image.Rect(0, 0, notePageW, notePageH)); d != (image.Point{}) {
		for i, s := range made {
			made[i] = s.moved(d)
		}
	}
	for len(w.nb.pages) <= page {
		w.nb.pages = append(w.nb.pages, nil)
	}
	ops := []noteOp{{page: w.selPage, del: w.sel}, {page: page, add: made}}
	if page == w.selPage {
		ops = []noteOp{{page: page, del: w.sel, add: made}}
	}
	for _, op := range ops {
		w.nb.apply(op, false)
	}
	w.edited(ops...)
	log.Printf("notebook %s: %d %s, on page %d", w.nb.name, len(made), what, page+1)
	w.sel, w.selPage = made, page
}

// moveSel moves what's taken by d (page units), onto page.
func (w *noteWin) moveSel(page int, d image.Point) {
	if d == (image.Point{}) && page == w.selPage {
		return
	}
	var made []*noteStroke
	for _, s := range w.sel {
		made = append(made, s.moved(d))
	}
	w.replaceSel(page, made, "moved")
}

// scaleSel makes what's taken k times as big, o (page units) staying.
func (w *noteWin) scaleSel(k float64, o image.Point) {
	if math.Abs(k-1) < 0.02 {
		return
	}
	var made []*noteStroke
	for _, s := range w.sel {
		made = append(made, s.transformed(func(x, y float64) (float64, float64) {
			return float64(o.X) + (x-float64(o.X))*k, float64(o.Y) + (y-float64(o.Y))*k
		}, k, 0))
	}
	w.replaceSel(w.selPage, made, "scaled")
}

// turnSel turns what's taken by th (radians, clockwise) round its
// middle; its pictures by as many quarters.
func (w *noteWin) turnSel(th float64) {
	quarters := int(math.Round(th/(math.Pi/2))) % 4
	if math.Abs(th) < 0.01 {
		return
	}
	b := w.selBox()
	cx, cy := float64(b.Min.X+b.Max.X)/2, float64(b.Min.Y+b.Max.Y)/2
	sin, cos := math.Sin(th), math.Cos(th)
	var made []*noteStroke
	for _, s := range w.sel {
		made = append(made, s.transformed(func(x, y float64) (float64, float64) {
			return cx + (x-cx)*cos - (y-cy)*sin, cy + (x-cx)*sin + (y-cy)*cos
		}, 1, (quarters+4)%4))
	}
	w.replaceSel(w.selPage, made, "turned")
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
	return s.transformed(func(x, y float64) (float64, float64) { return x + float64(d.X), y + float64(d.Y) }, 1, 0)
}

// transformed is s with its points through f (page units), its width k
// times; a picture's box goes through f, the picture turned clockwise
// by quarters.
func (s *noteStroke) transformed(f func(x, y float64) (float64, float64), k float64, quarters int) *noteStroke {
	clamp := func(v float64) int16 { return int16(max(-1<<15, min(1<<15-1, math.Round(v)))) }
	if s.pic != nil {
		x0, y0 := f(float64(s.box.Min.X), float64(s.box.Min.Y))
		x1, y1 := f(float64(s.box.Max.X), float64(s.box.Max.Y))
		box := image.Rect(int(math.Round(x0)), int(math.Round(y0)), int(math.Round(x1)), int(math.Round(y1))).Canon()
		p := s.pic
		for range quarters {
			p = p.turned()
		}
		return picStroke(p, box)
	}
	m := &noteStroke{attrs: s.attrs, gray: s.gray, marker: s.marker, pts: make([]notePt, len(s.pts))}
	for i, p := range s.pts {
		x, y := f(float64(p.x), float64(p.y))
		m.pts[i] = notePt{clamp(x), clamp(y), max(3, clamp(float64(p.r)*k))}
	}
	m.bound()
	return m
}

// turned is p a quarter turn clockwise: a new picture, its PNG gray.
func (p *notePic) turned() *notePic {
	b := p.gray.Bounds()
	t := image.NewGray(image.Rect(0, 0, b.Dy(), b.Dx()))
	for y := range b.Dy() {
		for x := range b.Dx() {
			t.Pix[x*t.Stride+b.Dy()-1-y] = p.gray.Pix[y*p.gray.Stride+x]
		}
	}
	var buf bytes.Buffer
	png.Encode(&buf, t) // a gray image into memory: it can't fail
	return &notePic{png: buf.Bytes(), gray: t}
}

// selAct does what a button by what's taken says.
func (w *noteWin) selAct(a *bro, id string) {
	before := w.selArea(a)
	switch id {
	case "copy":
		w.copySel(a)
		return
	case "cut":
		if !w.copySel(a) {
			return
		}
		w.deleteSel(a)
		return
	case "delete":
		w.deleteSel(a)
		return
	case "duplicate":
		w.duplicateSel()
	case "turn":
		w.turnSel(math.Pi / 2)
	}
	w.selChanged(a, before)
}

// copySel puts what's taken in the clipboard: a .xopp of one page.
func (w *noteWin) copySel(a *bro) bool {
	if len(w.sel) == 0 {
		return false
	}
	b, err := (&notebook{pages: [][]*noteStroke{w.sel}}).xopp()
	if err == nil {
		err = os.WriteFile(kobo.AppFile(clipInk), b, 0o600)
	}
	if err != nil {
		log.Printf("notebook %s: copy: %v", w.nb.name, err)
		a.Notify("not copied: " + err.Error())
		return false
	}
	log.Printf("notebook %s: %d copied", w.nb.name, len(w.sel))
	a.Notify("copied: paste it here, or in another notebook")
	return true
}

// duplicateSel puts a copy of what's taken by it, down and right, and
// takes that.
func (w *noteWin) duplicateSel() {
	if len(w.sel) == 0 {
		return
	}
	old := w.sel
	var made []*noteStroke
	for _, s := range old {
		made = append(made, s.moved(image.Pt(noteDupStep, noteDupStep)))
	}
	w.sel = nil
	w.addTaken(w.selPage, made, "duplicated")
}

// addTaken puts made on page, kept on it, as one change, and takes it.
func (w *noteWin) addTaken(page int, made []*noteStroke, what string) {
	if d := into(boxOf(made), image.Rect(0, 0, notePageW, notePageH)); d != (image.Point{}) {
		for i, s := range made {
			made[i] = s.moved(d)
		}
	}
	for len(w.nb.pages) <= page {
		w.nb.pages = append(w.nb.pages, nil)
	}
	op := noteOp{page: page, add: made}
	w.nb.apply(op, false)
	w.edited(op)
	log.Printf("notebook %s: %d %s, on page %d", w.nb.name, len(made), what, page+1)
	w.erasing, w.selecting = false, true
	w.sel, w.selPage = made, page
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

// pasteInk pastes what was copied from a notebook, if that was copied
// after the last picture: where it was on its page, or at the paste
// mark, on the page shown (the left of two). False if it wasn't.
func (w *noteWin) pasteInk(a *bro) bool {
	ink, err := os.Stat(kobo.AppFile(clipInk))
	if err != nil {
		return false
	}
	if pic, err := os.Stat(kobo.AppFile(clipPic)); err == nil && pic.ModTime().After(ink.ModTime()) {
		return false
	}
	f, err := os.Open(kobo.AppFile(clipInk))
	if err != nil {
		return false
	}
	defer f.Close()
	nb, err := parseXopp("clipboard", f)
	if err != nil || len(nb.pages[0]) == 0 {
		log.Printf("notebook: the clipboard's ink: %v", err)
		return false
	}
	made, page := nb.pages[0], w.cur
	if m := w.mark; m != nil {
		page = m.page
		d := m.at.Sub(boxOf(made).Min)
		for i, s := range made {
			made[i] = s.moved(d)
		}
		w.mark = nil
	}
	w.sel = nil
	w.addTaken(page, made, "pasted")
	if page >= w.cur+w.shown() {
		w.cur = page
	}
	w.show(a)
	return true
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
