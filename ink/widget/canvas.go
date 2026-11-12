package widget

import (
	"image"

	"hellobro/ink"
)

// Canvas is a sheet to draw on: the pen draws (its button or eraser end
// rubs out), a finger too if FingerDraws, else a finger's left to what
// it's in (a list moves). The pen's pressure makes its line thicker.
// Each move is drawn at once, in the pen's waveform; the strokes are
// kept, to Undo.
type Canvas struct {
	FingerDraws bool
	Erasing     bool   // what's drawn rubs out
	Width       ink.Dp // a line's, at the pen's lightest; 0: 1.5
	EraseWidth  ink.Dp // the eraser's; 0: 16

	img     *image.Gray
	strokes []stroke
	cur     *stroke
	contact int
	changed bool
	px      float32 // pixels a Dp, as last laid out
}

type stroke struct {
	pts   []image.Point
	ws    []int // each point's width
	erase bool
}

// Strokes is how many there are.
func (c *Canvas) Strokes() int { return len(c.strokes) }

// Changed says a stroke ended (or went) since it was last asked.
func (c *Canvas) Changed() bool {
	ch := c.changed
	c.changed = false
	return ch
}

// Undo takes the last stroke away.
func (c *Canvas) Undo() {
	if len(c.strokes) > 0 {
		c.strokes = c.strokes[:len(c.strokes)-1]
		c.redraw()
		c.changed = true
	}
}

// Clear takes them all away.
func (c *Canvas) Clear() {
	c.strokes, c.cur = nil, nil
	c.redraw()
	c.changed = true
}

func (c *Canvas) redraw() {
	if c.img == nil {
		return
	}
	fill(c.img, ink.White)
	for i := range c.strokes {
		c.paint(&c.strokes[i], 1)
	}
}

// paint draws s's segments from point from on.
func (c *Canvas) paint(s *stroke, from int) {
	g := uint8(ink.Black)
	if s.erase {
		g = ink.White
	}
	if len(s.pts) == 1 {
		dot(c.img, s.pts[0], s.ws[0], g)
	}
	for i := max(from, 1); i < len(s.pts); i++ {
		seg(c.img, s.pts[i-1], s.pts[i], s.ws[i], g)
	}
}

func (c *Canvas) width(e ink.Pointer, erase bool) int {
	px := c.px
	if erase {
		w := c.EraseWidth
		if w == 0 {
			w = 16
		}
		return max(int(float32(w)*px), 4)
	}
	w := c.Width
	if w == 0 {
		w = 1.5
	}
	base := float32(w) * px
	if e.Source == ink.Pen && e.Pressure > 0 {
		base *= 0.6 + 1.4*e.Pressure // light to firm: thin to thick
	} else if e.Source == ink.Finger {
		base *= 1.6
	}
	return max(int(base+0.5), 1)
}

func (c *Canvas) events(gtx ink.Ctx) {
	for _, ev := range gtx.Events(c) {
		e, ok := ev.(ink.Pointer)
		if !ok {
			continue
		}
		switch e.Kind {
		case ink.Press:
			if c.cur != nil || e.Source == ink.Finger && !c.FingerDraws {
				continue
			}
			gtx.Hand(e.ID, c) // it's the sheet's: no list takes it
			erase := c.Erasing || e.Eraser
			c.strokes = append(c.strokes, stroke{pts: []image.Point{e.Pos}, ws: []int{c.width(e, erase)}, erase: erase})
			c.cur, c.contact = &c.strokes[len(c.strokes)-1], e.ID
			if c.img != nil {
				c.paint(c.cur, 0)
			}
			gtx.Pen()
		case ink.Move:
			if c.cur == nil || e.ID != c.contact {
				continue
			}
			n := len(c.cur.pts)
			if e.Pos == c.cur.pts[n-1] {
				continue
			}
			c.cur.pts = append(c.cur.pts, e.Pos)
			c.cur.ws = append(c.cur.ws, c.width(e, c.cur.erase))
			if c.img != nil {
				c.paint(c.cur, n)
			}
			gtx.Pen()
		case ink.Release, ink.Cancel:
			if c.cur != nil && e.ID == c.contact {
				c.cur, c.changed = nil, true
				gtx.Invalidate() // the frame after: what the pen drew, in grays
			}
		}
	}
}

// Layout lays it out as big as it's given: what's drawn, on white.
func (c *Canvas) Layout(gtx ink.Ctx) ink.Dims {
	size := gtx.Constraints.Max
	c.px = gtx.Metric.PxPerDp
	if c.img == nil || c.img.Rect.Size() != size {
		c.img = image.NewGray(image.Rectangle{Max: size})
		c.redraw()
	}
	c.events(gtx)
	if c.cur != nil {
		gtx.Pen()
	}
	ink.Image(gtx.Ops, image.Rectangle{Max: size}, c.img, image.Point{})
	if !gtx.IsDisabled() {
		ink.Input(gtx.Ops, ink.Area{Rect: image.Rectangle{Max: size}, Tag: c})
	}
	return ink.Dims{Size: size}
}

func fill(img *image.Gray, g uint8) {
	for i := range img.Pix {
		img.Pix[i] = g
	}
}

// dot stamps a w wide square at p.
func dot(img *image.Gray, p image.Point, w int, g uint8) {
	h := w / 2
	r := image.Rect(p.X-h, p.Y-h, p.X-h+w, p.Y-h+w).Intersect(img.Rect)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		row := img.Pix[img.PixOffset(r.Min.X, y):img.PixOffset(r.Max.X, y)]
		for i := range row {
			row[i] = g
		}
	}
}

// seg draws a w thick line a to b: dots along it.
func seg(img *image.Gray, a, b image.Point, w int, g uint8) {
	d := b.Sub(a)
	n := max(abs(d.X), abs(d.Y))
	for i := 0; i <= n; i++ {
		p := a
		if n > 0 {
			p = a.Add(d.Mul(i).Div(n))
		}
		dot(img, p, w, g)
	}
}
