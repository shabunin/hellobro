package layout

import (
	"image"

	"hellobro/ink"
)

// List shows a page of items at a time, as e-ink likes it: no scrolling
// a pixel at a time, a swipe up or down (or ScrollBy) turns the page.
// The items are laid out only when they show, so it can be long.
type List struct {
	Axis Axis // as Gio's: Vertical must be said

	first, count, n int // the page shown: its first item, how many, of how many
	turn            int // pages to turn at the next layout
	to              int // an item to show at the next layout, -1: none
	toSet           bool

	contact int // the pointer that came down on it, while it's down
	down    bool
	from    image.Point
	handed  bool // it's a swipe: the list took it
	size    image.Point
}

// Swiping tells a swipe from a tap: how far a contact goes first, in Dp.
const swipeSlop ink.Dp = 24

// ScrollBy turns the page: pages on, or back if negative.
func (l *List) ScrollBy(pages int) { l.turn += pages }

// ScrollTo shows item i, on the page it falls on as the pages go from
// the first item.
func (l *List) ScrollTo(i int) { l.to, l.toSet = i, true }

// First is the first item shown, Count how many are.
func (l *List) First() int { return l.first }

// Count is how many items the page shows.
func (l *List) Count() int { return l.count }

// AtStart says it shows the first item; AtEnd, the last.
func (l *List) AtStart() bool { return l.first == 0 }

// AtEnd says the last item shows.
func (l *List) AtEnd() bool { return l.first+l.count >= l.n }

// Len is how many items it had, last laid out.
func (l *List) Len() int { return l.n }

func (l *List) axis() Axis { return l.Axis }

// events takes swipes: one that goes far enough along the list, before
// across it, is the list's, and turns its page as it lifts.
func (l *List) events(gtx ink.Ctx) {
	ax := l.axis()
	slop := gtx.Dp(swipeSlop)
	for _, e := range gtx.Events(l) {
		if r, ok := e.(ink.Reveal); ok {
			// The keys went to what's past the page: the page it's on.
			// Or came round to the list's other end: its first page, or
			// its last.
			switch {
			case r.Edge < 0:
				l.ScrollTo(0)
			case r.Edge > 0:
				l.ScrollTo(l.n - 1)
			case ax.main(r.R.Max) > ax.main(l.size):
				l.turn++
			case ax.main(r.R.Min) < 0:
				l.turn--
			}
			continue
		}
		p, ok := e.(ink.Pointer)
		if !ok {
			continue
		}
		switch p.Kind {
		case ink.Press:
			if !l.down {
				l.down, l.contact, l.from, l.handed = true, p.ID, p.Pos, false
			}
		case ink.Move:
			if !l.down || p.ID != l.contact || l.handed {
				continue
			}
			d := p.Pos.Sub(l.from)
			along, across := abs(ax.main(d)), abs(ax.cross(d))
			if along > slop && along > across {
				l.handed = true
				gtx.Hand(p.ID, l)
			}
		case ink.Release:
			if !l.down || p.ID != l.contact {
				continue
			}
			l.down = false
			d := ax.main(p.Pos.Sub(l.from))
			if l.handed && abs(d) > max(slop, ax.main(l.size)/10) {
				if d < 0 {
					l.turn++
				} else {
					l.turn--
				}
			}
		case ink.Cancel:
			if p.ID == l.contact {
				l.down = false
			}
		}
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// Layout lays out a page of n items, el laying out item i, filling what
// it's given.
func (l *List) Layout(gtx ink.Ctx, n int, el func(gtx ink.Ctx, i int) ink.Dims) ink.Dims {
	ax := l.axis()
	size := gtx.Constraints.Max
	l.size = size
	l.events(gtx)
	l.n = n
	room := ax.main(size)
	// Each item may be up to the page long, as wide as the list.
	item := func(gtx ink.Ctx, i int) (ink.CallOp, ink.Dims) {
		cgtx := gtx
		cgtx.Constraints = ink.Constraints{Min: ax.pt(0, ax.cross(size)), Max: ax.pt(room, ax.cross(size))}
		m := ink.Record(gtx.Ops)
		d := el(cgtx, i)
		return m.Stop(), d
	}
	// How many fit from first on: measured, not drawn (no events eaten).
	fits := func(first int) int {
		used, k := 0, 0
		for i := first; i < n; i++ {
			_, d := item(gtx.Measuring(), i)
			used += ax.main(d.Size)
			if used > room && k > 0 {
				break
			}
			k++
			if used >= room {
				break
			}
		}
		return k
	}
	// The page before first: as many as fit, back from it.
	before := func(first int) int {
		used := 0
		i := first
		for i > 0 {
			_, d := item(gtx.Measuring(), i-1)
			if used+ax.main(d.Size) > room && i < first {
				break
			}
			used += ax.main(d.Size)
			i--
		}
		return i
	}
	if l.toSet {
		l.toSet = false
		if t := min(max(l.to, 0), max(n-1, 0)); t < l.first || t >= l.first+max(l.count, 1) {
			p := 0 // the pages from the top, to t's
			for k := fits(0); p+k <= t && k > 0 && p+k < n; k = fits(p) {
				p += k
			}
			l.first = p
		}
	}
	for ; l.turn > 0; l.turn-- {
		if c := fits(l.first); l.first+c < n {
			l.first += c
		}
	}
	for ; l.turn < 0; l.turn++ {
		l.first = before(l.first)
	}
	if l.first >= n {
		l.first = before(n)
	}
	l.first = max(l.first, 0)

	clip := ink.Clip(gtx.Ops, image.Rectangle{Max: size})
	ink.Input(gtx.Ops, ink.Area{Rect: image.Rectangle{Max: size}, Tag: l, Watch: true, Reveal: true})
	at := func(call ink.CallOp, pos int) {
		st := ink.Offset(gtx.Ops, ax.pt(pos, 0))
		call.Add(gtx.Ops)
		st.Pop()
	}
	// The item before the page, out of sight: Shift+Tab goes to it.
	if l.first > 0 {
		call, d := item(gtx, l.first-1)
		at(call, -ax.main(d.Size))
	}
	used, cross := 0, 0
	l.count = 0
	i := l.first
	for ; i < n; i++ {
		call, d := item(gtx, i)
		if used+ax.main(d.Size) > room && l.count > 0 {
			break
		}
		at(call, used)
		used += ax.main(d.Size)
		cross = max(cross, ax.cross(d.Size))
		l.count++
		if used >= room {
			i++
			break
		}
	}
	// And the one after it, past the page's end: Tab goes to it.
	if i < n {
		call, _ := item(gtx, i)
		at(call, room)
	}
	clip.Pop()
	return ink.Dims{Size: gtx.Constraints.Constrain(ax.pt(room, cross))}
}
