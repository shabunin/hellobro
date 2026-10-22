package wm

import (
	"errors"
	"image"
	"slices"
)

// A resize's amount: px pixels, or ppt percentage points of the
// container's size (for i3's "10 px or 10 ppt", tiled windows take the
// points and floating ones the pixels). Shrinking is less than 0.
type Amount struct{ Px, Ppt int }

var errNoRoom = errors.New("no room to resize")
var errNothing = errors.New("nothing to resize against there")

// minShare is the least share a tiled container may have of p: a title
// bar's height, at least.
func (t *Tree) minShare(p *Node, horizontal bool) float64 {
	size := p.Rect.Dy()
	if horizontal {
		size = p.Rect.Dx()
	}
	if size <= 0 {
		return 0.05
	}
	return float64(max(t.TitleH, 16)) / float64(size)
}

// share is a in parts of p's size along horizontal: the points, or the
// pixels if no points were given.
func share(p *Node, horizontal bool, a Amount) float64 {
	if a.Ppt != 0 {
		return float64(a.Ppt) / 100
	}
	size := p.Rect.Dy()
	if horizontal {
		size = p.Rect.Dx()
	}
	if size <= 0 {
		return 0
	}
	return float64(a.Px) / float64(size)
}

// participants finds what a resize in d changes: the container above n
// (n, or up) in a split along d with a neighbour on d's side, and that
// neighbour; with both, the one on the other side will do too. Tabbed
// and stacked containers are passed over: their children are the same
// size.
func participants(n *Node, d Dir, both bool) (first, second *Node) {
	first = n
	for first.Type != Workspace && first.Type != FloatingCon {
		p := first.Parent
		if p.Layout.horizontal() == d.horizontal() && p.Layout.split() {
			i := slices.Index(p.Nodes, first)
			j, k := i+1, i-1
			if d.back() {
				j, k = k, j
			}
			if j >= 0 && j < len(p.Nodes) {
				return first, p.Nodes[j]
			}
			if both && k >= 0 && k < len(p.Nodes) {
				return first, p.Nodes[k]
			}
		}
		first = p
	}
	return first, nil
}

// ResizeDir grows n (shrinks, for less than 0) on its d side, into its
// neighbour there: i3's resize grow|shrink left|right|up|down. A
// floating window's edge moves.
func (t *Tree) ResizeDir(n *Node, d Dir, a Amount) error {
	if f := floatOf(n); f != nil {
		r := f.FloatRect
		px := t.floatPx(a, d.horizontal())
		switch d {
		case Left:
			r.Min.X -= px
		case Right:
			r.Max.X += px
		case Up:
			r.Min.Y -= px
		case Down:
			r.Max.Y += px
		}
		return t.resizeFloat(n, r)
	}
	first, second := participants(n, d, false)
	if second == nil {
		return errNothing
	}
	p := first.Parent
	s := share(p, d.horizontal(), a)
	min := t.minShare(p, d.horizontal())
	if first.Percent+s < min || second.Percent-s < min {
		return errNoRoom
	}
	first.Percent += s
	second.Percent -= s
	return nil
}

// ResizeSize grows n's width (or height) by a, taken from (or given to)
// the others beside it in equal parts: i3's resize grow|shrink
// width|height.
func (t *Tree) ResizeSize(n *Node, width bool, a Amount) error {
	if f := floatOf(n); f != nil {
		r := f.FloatRect
		if width {
			r.Max.X += t.floatPx(a, true)
		} else {
			r.Max.Y += t.floatPx(a, false)
		}
		return t.resizeFloat(n, r)
	}
	d := Down
	if width {
		d = Left
	}
	c, other := participants(n, d, true)
	if other == nil {
		return errNothing
	}
	return t.grow(c, width, share(c.Parent, width, a))
}

// grow gives c s more of its parent, the others s/(n-1) less each.
func (t *Tree) grow(c *Node, horizontal bool, s float64) error {
	p := c.Parent
	min := t.minShare(p, horizontal)
	each := s / float64(len(p.Nodes)-1)
	if c.Percent+s < min {
		return errNoRoom
	}
	for _, o := range p.Nodes {
		if o != c && o.Percent-each < min {
			return errNoRoom
		}
	}
	for _, o := range p.Nodes {
		if o == c {
			o.Percent += s
		} else {
			o.Percent -= each
		}
	}
	return nil
}

// ResizeSet makes n w wide and h high (a zero Amount: as it is): pixels,
// or percent of the screen (floating) or its container (tiled).
func (t *Tree) ResizeSet(n *Node, w, h Amount) error {
	if f := floatOf(n); f != nil {
		r := f.FloatRect
		scr := t.content.Rect
		if w != (Amount{}) {
			r.Max.X = r.Min.X + abs(w, scr.Dx())
		}
		if h != (Amount{}) {
			r.Max.Y = r.Min.Y + abs(h, scr.Dy())
		}
		return t.resizeFloat(n, r)
	}
	for _, s := range []struct {
		a     Amount
		width bool
	}{{w, true}, {h, false}} {
		if s.a == (Amount{}) {
			continue
		}
		d := Down
		if s.width {
			d = Left
		}
		c, other := participants(n, d, true)
		if other == nil {
			continue
		}
		want := share(c.Parent, s.width, s.a)
		if err := t.grow(c, s.width, want-c.Percent); err != nil {
			return err
		}
	}
	return nil
}

// abs is a in pixels of size.
func abs(a Amount, size int) int {
	if a.Px != 0 {
		return a.Px
	}
	return a.Ppt * size / 100
}

// floatPx is a for a floating window: the pixels, or the points of the
// screen.
func (t *Tree) floatPx(a Amount, horizontal bool) int {
	if a.Px != 0 {
		return a.Px
	}
	size := t.content.Rect.Dy()
	if horizontal {
		size = t.content.Rect.Dx()
	}
	return a.Ppt * size / 100
}

// resizeFloat makes n's floating container r, if it's not too small.
func (t *Tree) resizeFloat(n *Node, r image.Rectangle) error {
	least := 2 * max(t.TitleH, 16)
	if r.Dx() < least || r.Dy() < least {
		return errNoRoom
	}
	t.SetFloatRect(n, r)
	return nil
}
