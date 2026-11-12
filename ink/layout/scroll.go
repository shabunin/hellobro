package layout

import (
	"image"

	"hellobro/ink"
)

// Scroller is a list of items as List and Scroll both are: by the page
// or under the finger.
type Scroller interface {
	Layout(gtx ink.Ctx, n int, el func(gtx ink.Ctx, i int) ink.Dims) ink.Dims
	// Span is the part of it that shows, 0 to 1, as the last layout had it.
	Span() (from, to float32)
	ScrollBy(pages int)
	ScrollTo(i int)
}

// Span is the part of the items the page shows.
func (l *List) Span() (from, to float32) {
	if l.n == 0 {
		return 0, 1
	}
	return float32(l.first) / float32(l.n), float32(l.first+l.count) / float32(l.n)
}

// Scroll is a list that moves under the finger, a pixel at a time: each
// move is a frame in black and white at once (Ctx.Quick), and the frame
// after the finger lifts puts it back in grays. Items are laid out only
// as they show; one item taller than the room (a page of text) scrolls
// as well as many.
type Scroll struct {
	Axis Axis // as Gio's: Vertical must be said

	first, off int // the first item shown, and how much of it's gone off the start
	n          int
	shown      int     // items shown, the first and last partly perhaps
	startFrac  float32 // how much of the first is off the start
	endFrac    float32 // how far into the last the room ends
	pages      int     // pages to go at the next layout
	to         int
	toSet      bool
	room       int

	contact int
	down    bool
	from    image.Point // where the contact came down
	last    image.Point // where it was at the last move taken
	handed  bool
	moved   int // pixels to go at the next layout, along the axis
}

// ScrollBy moves it pages on (back if negative), nearly a page each.
func (s *Scroll) ScrollBy(pages int) { s.pages += pages }

// ScrollTo shows item i at the start, or as near as it goes.
func (s *Scroll) ScrollTo(i int) { s.to, s.toSet = i, true }

// First is the first item that shows, Offset how much of it's off the
// start, in pixels.
func (s *Scroll) First() int  { return s.first }
func (s *Scroll) Offset() int { return s.off }

// Dragging says a finger's moving it.
func (s *Scroll) Dragging() bool { return s.down && s.handed }

// Span is the part that shows, as the items go.
func (s *Scroll) Span() (from, to float32) {
	if s.n == 0 {
		return 0, 1
	}
	n := float32(s.n)
	return (float32(s.first) + s.startFrac) / n, min((float32(s.first+max(s.shown, 1)-1)+s.endFrac)/n, 1)
}

func (s *Scroll) events(gtx ink.Ctx) {
	ax := s.Axis
	slop := gtx.Dp(swipeSlop) / 2
	for _, e := range gtx.Events(s) {
		p, ok := e.(ink.Pointer)
		if !ok {
			continue
		}
		switch p.Kind {
		case ink.Press:
			if !s.down {
				s.down, s.contact, s.from, s.last, s.handed = true, p.ID, p.Pos, p.Pos, false
			}
		case ink.Move:
			if !s.down || p.ID != s.contact {
				continue
			}
			if !s.handed {
				d := p.Pos.Sub(s.from)
				along, across := abs(ax.main(d)), abs(ax.cross(d))
				if along <= slop || along <= across {
					continue
				}
				s.handed = true
				gtx.Hand(p.ID, s)
			}
			s.moved -= ax.main(p.Pos.Sub(s.last))
			s.last = p.Pos
		case ink.Release, ink.Cancel:
			if s.down && p.ID == s.contact {
				if s.handed && p.Kind == ink.Release {
					s.moved -= ax.main(p.Pos.Sub(s.last))
				}
				s.down = false
				gtx.Invalidate() // the frame in grays
			}
		}
	}
}

// big is the room an item has along the list: as much as it wants.
const big = 1 << 24

// Layout lays out the items that show of n, el laying out item i,
// filling what it's given.
func (s *Scroll) Layout(gtx ink.Ctx, n int, el func(gtx ink.Ctx, i int) ink.Dims) ink.Dims {
	ax := s.Axis
	size := gtx.Constraints.Max
	room := ax.main(size)
	s.room = room
	s.events(gtx)
	if s.Dragging() {
		gtx.Quick()
	}
	s.n = n
	item := func(gtx ink.Ctx, i int) (ink.CallOp, ink.Dims) {
		cg := gtx
		cg.Constraints = ink.Constraints{Min: ax.pt(0, ax.cross(size)), Max: ax.pt(big, ax.cross(size))}
		m := ink.Record(gtx.Ops)
		d := el(cg, i)
		return m.Stop(), d
	}
	length := func(i int) int { _, d := item(gtx.Measuring(), i); return ax.main(d.Size) }

	if s.toSet {
		s.toSet = false
		s.first, s.off = min(max(s.to, 0), max(n-1, 0)), 0
	}
	s.off += s.moved + s.pages*room*9/10
	s.moved, s.pages = 0, 0
	if s.first >= n {
		s.first, s.off = max(n-1, 0), 0
	}
	// Back: an offset before the start is the items before it.
	for s.off < 0 && s.first > 0 {
		s.first--
		s.off += length(s.first)
	}
	if s.off < 0 || n == 0 {
		s.off = 0
	}
	// On: whole items gone off the start.
	for s.first < n-1 {
		l := length(s.first)
		if s.off < l {
			break
		}
		s.off -= l
		s.first++
	}
	// The end: no room left after the last item, unless they all fit.
	used := -s.off
	for i := s.first; i < n && used < room; i++ {
		used += length(i)
	}
	if used < room {
		short := room - used
		for short > 0 && (s.off > 0 || s.first > 0) {
			if s.off >= short {
				s.off -= short
				short = 0
			} else {
				short -= s.off
				s.off = 0
				if s.first == 0 {
					break
				}
				s.first--
				s.off = length(s.first)
			}
		}
	}

	clip := ink.Clip(gtx.Ops, image.Rectangle{Max: size})
	ink.Input(gtx.Ops, ink.Area{Rect: image.Rectangle{Max: size}, Tag: s, Watch: true})
	pos, cross := -s.off, 0
	s.shown, s.startFrac, s.endFrac = 0, 0, 1
	for i := s.first; i < n && pos < room; i++ {
		call, d := item(gtx, i)
		st := ink.Offset(gtx.Ops, ax.pt(pos, 0))
		call.Add(gtx.Ops)
		st.Pop()
		l := ax.main(d.Size)
		if l > 0 {
			if i == s.first {
				s.startFrac = float32(s.off) / float32(l)
			}
			s.endFrac = float32(min(room-pos, l)) / float32(l) // how far into it the room ends
		}
		pos += l
		cross = max(cross, ax.cross(d.Size))
		s.shown++
	}
	clip.Pop()
	return ink.Dims{Size: gtx.Constraints.Constrain(ax.pt(room, cross))}
}
