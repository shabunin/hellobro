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
// after the finger lifts puts it back in grays. A flick coasts on a
// moment (unless NoFling). Items are laid out only as they show (and
// the one past each end, for the keys to go to); one item taller than
// the room (a page of text) scrolls as well as many.
type Scroll struct {
	Axis    Axis // as Gio's: Vertical must be said
	NoFling bool

	first, off int // the first item shown, and how much of it's gone off the start
	n          int
	shown      int     // items shown, the first and last partly perhaps
	startFrac  float32 // how much of the first is off the start
	endFrac    float32 // how far into the last the room ends
	pages      int     // pages to go at the next layout
	to         int
	toSet      bool

	contact int
	down    bool
	from    image.Point // where the contact came down
	last    image.Point // where it was at the last move taken
	handed  bool
	moved   int // pixels to go at the next layout, along the axis
	fling   fling

	spanSet        bool
	spanLo, spanHi int
}

// RevealSpan shows lo to hi of the first item, along the list, moving as
// little as it takes: a text's caret, in its one item.
func (s *Scroll) RevealSpan(lo, hi int) { s.spanSet, s.spanLo, s.spanHi = true, lo, hi }

// ScrollBy moves it pages on (back if negative), nearly a page each.
func (s *Scroll) ScrollBy(pages int) { s.pages += pages; s.fling.stop() }

// ScrollTo shows item i, moving as little as it takes.
func (s *Scroll) ScrollTo(i int) { s.to, s.toSet = i, true; s.fling.stop() }

// First is the first item that shows, Offset how much of it's off the
// start, in pixels.
func (s *Scroll) First() int  { return s.first }
func (s *Scroll) Offset() int { return s.off }

// Dragging says a finger's moving it; Flinging, it's coasting on.
func (s *Scroll) Dragging() bool { return s.down && s.handed }
func (s *Scroll) Flinging() bool { return s.fling.active() }

// Span is the part that shows, as the items go.
func (s *Scroll) Span() (from, to float32) {
	if s.n == 0 {
		return 0, 1
	}
	n := float32(s.n)
	return (float32(s.first) + s.startFrac) / n, min((float32(s.first+max(s.shown, 1)-1)+s.endFrac)/n, 1)
}

func (s *Scroll) events(gtx ink.Ctx, room int) {
	ax := s.Axis
	slop := gtx.Dp(swipeSlop) / 2
	for _, e := range gtx.Events(s) {
		switch e := e.(type) {
		case ink.Reveal:
			lo, hi := ax.main(e.R.Min), ax.main(e.R.Max)
			switch {
			case lo < 0:
				s.moved += lo
			case hi > room:
				s.moved += min(hi-room, lo)
			}
		case ink.Pointer:
			p := e
			switch p.Kind {
			case ink.Press:
				if !s.down {
					s.down, s.contact, s.from, s.last, s.handed = true, p.ID, p.Pos, p.Pos, false
					if s.fling.active() {
						// A finger on it as it coasts stops it, and it's
						// the list's then, not the row's under it.
						s.fling.stop()
						s.handed = true
						gtx.Hand(p.ID, s)
					}
					s.fling.track(p.Pos, p.Time)
				}
			case ink.Move:
				if !s.down || p.ID != s.contact {
					continue
				}
				s.fling.track(p.Pos, p.Time)
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
					s.down = false
					if s.handed && p.Kind == ink.Release {
						s.moved -= ax.main(p.Pos.Sub(s.last))
						s.fling.track(p.Pos, p.Time)
						if !s.NoFling && s.fling.start(gtx, p.Time, [2]bool{ax == Horizontal, ax == Vertical}) {
							continue
						}
					}
					gtx.Invalidate() // the frame in grays
				}
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
	s.events(gtx, room)
	if d := s.fling.step(gtx); d != (image.Point{}) {
		s.moved += ax.main(d)
	}
	if s.Dragging() || s.fling.active() {
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

	if s.first >= n {
		s.first, s.off = max(n-1, 0), 0
	}
	if s.toSet {
		s.toSet = false
		if t := min(max(s.to, 0), max(n-1, 0)); t <= s.first {
			s.first, s.off = t, 0
		} else {
			end := -s.off
			for i := s.first; i <= t && end <= room; i++ {
				end += length(i)
			}
			if end > room {
				s.off += end - room
			}
		}
	}
	if s.spanSet {
		s.spanSet = false
		if s.first == 0 {
			if s.spanLo < s.off {
				s.off = s.spanLo
			} else if s.spanHi > s.off+room {
				s.off = min(s.spanHi-room, s.spanLo)
			}
		}
	}
	s.off += s.moved + s.pages*room*9/10
	s.moved, s.pages = 0, 0
	// Back: an offset before the start is the items before it.
	for s.off < 0 && s.first > 0 {
		s.first--
		s.off += length(s.first)
	}
	if s.off < 0 || n == 0 {
		s.off = 0
		if s.fling.active() {
			s.fling.stop()
			gtx.InvalidateAt(gtx.Now)
		}
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
	// What shows, from first on.
	type laid struct {
		call ink.CallOp
		d    ink.Dims
	}
	var shown []laid
	lay := func() int {
		shown = shown[:0]
		pos := -s.off
		for i := s.first; i < n && pos < room; i++ {
			call, d := item(gtx, i)
			shown = append(shown, laid{call, d})
			pos += ax.main(d.Size)
		}
		return pos
	}
	// The end: no room left after the last item, unless they all fit.
	if end := lay(); end < room && (s.off > 0 || s.first > 0) {
		if s.fling.active() {
			s.fling.stop()
			gtx.InvalidateAt(gtx.Now)
		}
		short := room - end
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
		lay()
	}

	clip := ink.Clip(gtx.Ops, image.Rectangle{Max: size})
	ink.Input(gtx.Ops, ink.Area{Rect: image.Rectangle{Max: size}, Tag: s, Watch: true, Reveal: true})
	at := func(call ink.CallOp, pos int) {
		st := ink.Offset(gtx.Ops, ax.pt(pos, 0))
		call.Add(gtx.Ops)
		st.Pop()
	}
	// The one before, out of sight: Shift+Tab goes to it from the first.
	if s.first > 0 {
		call, d := item(gtx, s.first-1)
		at(call, -s.off-ax.main(d.Size))
	}
	pos, cross := -s.off, 0
	s.shown, s.startFrac, s.endFrac = len(shown), 0, 1
	for k, it := range shown {
		at(it.call, pos)
		l := ax.main(it.d.Size)
		if l > 0 {
			if k == 0 {
				s.startFrac = float32(s.off) / float32(l)
			}
			s.endFrac = float32(min(room-pos, l)) / float32(l) // how far into it the room ends
		}
		pos += l
		cross = max(cross, ax.cross(it.d.Size))
	}
	i := s.first + len(shown)
	// And the one after: Tab goes to it from the last.
	if i < n {
		call, _ := item(gtx, i)
		at(call, pos)
	}
	clip.Pop()
	return ink.Dims{Size: gtx.Constraints.Constrain(ax.pt(room, cross))}
}
