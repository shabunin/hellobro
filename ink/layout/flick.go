package layout

import (
	"image"

	"hellobro/ink"
)

// Flick is a view onto something bigger than it, both ways (a map, a
// page zoomed in), that moves under the finger any way it goes and
// coasts on after a flick: each move a quick frame, grays once it
// rests. What's in it may be tapped, as in a Scroll.
type Flick struct {
	Pos     image.Point // the content's point at the view's top left
	NoFling bool

	content, view image.Point
	contact       int
	down, handed  bool
	from, last    image.Point
	moved         image.Point
	fling         fling
}

// Dragging says a finger's moving it; Flinging, it's coasting on.
func (f *Flick) Dragging() bool { return f.down && f.handed }
func (f *Flick) Flinging() bool { return f.fling.active() }

// Content is how big what's in it was, last laid out; View, the view.
func (f *Flick) Content() image.Point { return f.content }
func (f *Flick) View() image.Point    { return f.view }

// MoveBy moves the content by d (as a finger going -d would).
func (f *Flick) MoveBy(d image.Point) { f.moved = f.moved.Add(d); f.fling.stop() }

func (f *Flick) events(gtx ink.Ctx) {
	slop := gtx.Dp(swipeSlop) / 2
	for _, e := range gtx.Events(f) {
		switch e := e.(type) {
		case ink.Reveal:
			r := e.R
			d := image.Point{}
			if r.Min.X < 0 {
				d.X = r.Min.X
			} else if r.Max.X > f.view.X {
				d.X = min(r.Max.X-f.view.X, r.Min.X)
			}
			if r.Min.Y < 0 {
				d.Y = r.Min.Y
			} else if r.Max.Y > f.view.Y {
				d.Y = min(r.Max.Y-f.view.Y, r.Min.Y)
			}
			f.moved = f.moved.Add(d)
		case ink.Pointer:
			p := e
			switch p.Kind {
			case ink.Press:
				if !f.down {
					f.down, f.contact, f.from, f.last, f.handed = true, p.ID, p.Pos, p.Pos, false
					if f.fling.active() {
						f.fling.stop()
						f.handed = true
						gtx.Hand(p.ID, f)
					}
					f.fling.track(p.Pos, p.Time)
				}
			case ink.Move:
				if !f.down || p.ID != f.contact {
					continue
				}
				f.fling.track(p.Pos, p.Time)
				if !f.handed {
					d := p.Pos.Sub(f.from)
					if d.X*d.X+d.Y*d.Y <= slop*slop {
						continue
					}
					f.handed = true
					gtx.Hand(p.ID, f)
				}
				f.moved = f.moved.Sub(p.Pos.Sub(f.last))
				f.last = p.Pos
			case ink.Release, ink.Cancel:
				if f.down && p.ID == f.contact {
					f.down = false
					if f.handed && p.Kind == ink.Release {
						f.moved = f.moved.Sub(p.Pos.Sub(f.last))
						f.fling.track(p.Pos, p.Time)
						if !f.NoFling && f.fling.start(gtx, p.Time, [2]bool{true, true}) {
							continue
						}
					}
					gtx.Invalidate()
				}
			}
		}
	}
}

// Layout lays w out as big as it likes and shows the part at Pos,
// filling what it's given.
func (f *Flick) Layout(gtx ink.Ctx, w ink.Widget) ink.Dims {
	view := gtx.Constraints.Max
	f.view = view
	f.events(gtx)
	f.moved = f.moved.Add(f.fling.step(gtx))
	if f.Dragging() || f.fling.active() {
		gtx.Quick()
	}
	cg := gtx
	cg.Constraints = ink.Constraints{Max: image.Pt(big, big)}
	m := ink.Record(gtx.Ops)
	d := w(cg)
	call := m.Stop()
	f.content = d.Size
	p := f.Pos.Add(f.moved)
	f.moved = image.Point{}
	room := image.Pt(max(d.Size.X-view.X, 0), max(d.Size.Y-view.Y, 0))
	q := image.Pt(min(max(p.X, 0), room.X), min(max(p.Y, 0), room.Y))
	if q != p && f.fling.active() { // at an edge: it stops that way
		if q.X != p.X {
			f.fling.v[0] = 0
		}
		if q.Y != p.Y {
			f.fling.v[1] = 0
		}
		if !f.fling.active() {
			gtx.InvalidateAt(gtx.Now)
		}
	}
	f.Pos = q
	clip := ink.Clip(gtx.Ops, image.Rectangle{Max: view})
	ink.Input(gtx.Ops, ink.Area{Rect: image.Rectangle{Max: view}, Tag: f, Watch: true, Reveal: true})
	st := ink.Offset(gtx.Ops, f.Pos.Mul(-1))
	call.Add(gtx.Ops)
	st.Pop()
	clip.Pop()
	return ink.Dims{Size: view}
}
