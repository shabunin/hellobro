package ink

import (
	"image"
	"image/color"
	"image/draw"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// The painter: the ops onto a gray image, a row at a time where it can
// (draw.Draw goes a pixel at a time for Gray images, 30× slower on the
// Kobo), and the input areas gathered on the way, where they ended up.

type placed struct {
	Area
	r   image.Rectangle // on the image, clipped
	off image.Point     // its origin there: pointers are told of relative to it
}

// textBox is a text painted, where: inktest finds things by their words.
type textBox struct {
	r image.Rectangle
	s string
}

type painter struct {
	dst    *image.Gray
	areas  []placed
	texts  []textBox
	popups []popup // painted after the rest, in order
}

type popup struct {
	anchor image.Rectangle // on the image
	size   image.Point
	tag    Tag
	ops    []op
}

func (p *painter) run(ops []op, off image.Point, clip image.Rectangle) {
	for i := 0; i < len(ops); i++ {
		o := &ops[i]
		switch o.kind {
		case opFill:
			fill(p.dst, o.r.Add(off).Intersect(clip), o.gray)
		case opFrame:
			r := o.r.Add(off)
			t := o.t
			fill(p.dst, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+t).Intersect(clip), o.gray)
			fill(p.dst, image.Rect(r.Min.X, r.Max.Y-t, r.Max.X, r.Max.Y).Intersect(clip), o.gray)
			fill(p.dst, image.Rect(r.Min.X, r.Min.Y+t, r.Min.X+t, r.Max.Y-t).Intersect(clip), o.gray)
			fill(p.dst, image.Rect(r.Max.X-t, r.Min.Y+t, r.Max.X, r.Max.Y-t).Intersect(clip), o.gray)
		case opText:
			at := o.off.Add(off)
			r := textText(p.dst, o.face, at, o.s, o.gray, clip)
			if !r.Empty() {
				p.texts = append(p.texts, textBox{r, o.s})
			}
		case opImage:
			r := o.r.Add(off)
			c := r.Intersect(clip)
			copyGray(p.dst, c, o.img, o.sp.Add(c.Min.Sub(r.Min)))
		case opDraw:
			r := o.r.Add(off).Intersect(clip)
			if !r.Empty() {
				o.draw(p.dst.SubImage(r).(*image.Gray), r)
			}
		case opInvert:
			invert(p.dst, o.r.Add(off).Intersect(clip))
		case opPush:
			end := matchPop(ops, i)
			c := clip
			if o.clips {
				c = o.r.Add(off).Intersect(clip)
			}
			p.run(ops[i+1:end], off.Add(o.off), c)
			i = end
		case opPop: // a stray one: ignored
		case opCall:
			p.run(o.call, off, clip)
		case opArea:
			r := o.area.Rect.Add(off).Intersect(clip)
			if !r.Empty() {
				p.areas = append(p.areas, placed{o.area, r, off})
			}
		case opLine:
			for k := 1; k < len(o.pts); k++ {
				line(p.dst, o.pts[k-1].Add(off), o.pts[k].Add(off), o.t, o.gray, clip)
			}
		case opDisc:
			disc(p.dst, o.off.Add(off), o.t, o.r.Dx(), o.gray, clip)
		case opPoly:
			pts := make([]image.Point, len(o.pts))
			for k, q := range o.pts {
				pts[k] = q.Add(off)
			}
			poly(p.dst, pts, o.gray, clip)
		case opPopup:
			p.popups = append(p.popups, popup{o.r.Add(off), o.sp, o.tag, o.call})
		}
	}
}

// paint runs the ops, then the popups over them: each at its place,
// its backdrop under it.
func (p *painter) paint(ops []op) {
	p.run(ops, image.Point{}, p.dst.Rect)
	for i := 0; i < len(p.popups); i++ { // a popup may open another
		pp := p.popups[i]
		win := p.dst.Rect
		at := image.Pt(pp.anchor.Min.X, pp.anchor.Max.Y)
		if at.Y+pp.size.Y > win.Max.Y && pp.anchor.Min.Y-pp.size.Y >= win.Min.Y {
			at.Y = pp.anchor.Min.Y - pp.size.Y // no room under it: over it
		}
		at.X = max(win.Min.X, min(at.X, win.Max.X-pp.size.X))
		at.Y = max(win.Min.Y, min(at.Y, win.Max.Y-pp.size.Y))
		p.areas = append(p.areas, placed{Area{Rect: win, Tag: pp.tag, Block: true}, win, image.Point{}})
		p.run(pp.ops, at, win)
	}
}

// line draws a w thick line from a to b: a square pen stamped along it.
func line(img *image.Gray, a, b image.Point, w int, g uint8, clip image.Rectangle) {
	d := b.Sub(a)
	steps := max(abs(d.X), abs(d.Y), 1)
	h := w / 2
	for i := 0; i <= steps; i++ {
		q := image.Pt(a.X+d.X*i/steps, a.Y+d.Y*i/steps)
		fill(img, image.Rect(q.X-h, q.Y-h, q.X-h+w, q.Y-h+w).Intersect(clip), g)
	}
}

// disc fills a circle of radius r about c, or only its edge, w thick
// (w 0: all of it).
func disc(img *image.Gray, c image.Point, r, w int, g uint8, clip image.Rectangle) {
	r2 := r*r + r
	in := -1
	if w > 0 && w < r {
		ri := r - w
		in = ri*ri + ri
	}
	for y := -r; y <= r; y++ {
		x0, x1 := 1, 0 // the run across, and the hole in it
		for x := -r; x <= r; x++ {
			if x*x+y*y <= r2 {
				if x0 > x1 {
					x0 = x
				}
				x1 = x
			}
		}
		if x0 > x1 {
			continue
		}
		if in < 0 {
			fill(img, image.Rect(c.X+x0, c.Y+y, c.X+x1+1, c.Y+y+1).Intersect(clip), g)
			continue
		}
		for x := x0; x <= x1; x++ {
			if x*x+y*y > in {
				fill(img, image.Rect(c.X+x, c.Y+y, c.X+x+1, c.Y+y+1).Intersect(clip), g)
			}
		}
	}
}

// poly fills pts, even-odd, a row at a time.
func poly(img *image.Gray, pts []image.Point, g uint8, clip image.Rectangle) {
	if len(pts) < 3 {
		return
	}
	b := image.Rectangle{Min: pts[0], Max: pts[0]}
	for _, q := range pts {
		b = b.Union(image.Rectangle{Min: q, Max: q.Add(image.Pt(1, 1))})
	}
	var xs []int
	for y := b.Min.Y; y < b.Max.Y; y++ {
		xs = xs[:0]
		fy := float64(y) + 0.5
		for i := range pts {
			a, c := pts[i], pts[(i+1)%len(pts)]
			if (float64(a.Y) <= fy) != (float64(c.Y) <= fy) {
				x := float64(a.X) + (fy-float64(a.Y))*float64(c.X-a.X)/float64(c.Y-a.Y)
				xs = append(xs, int(x+0.5))
			}
		}
		for i := 1; i < len(xs); i++ { // few: an insertion sort
			for j := i; j > 0 && xs[j] < xs[j-1]; j-- {
				xs[j], xs[j-1] = xs[j-1], xs[j]
			}
		}
		for i := 0; i+1 < len(xs); i += 2 {
			fill(img, image.Rect(xs[i], y, xs[i+1], y+1).Intersect(clip), g)
		}
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// matchPop is the index of the pop that ends ops[i]'s push (len(ops) if
// it's missing: the rest is in it).
func matchPop(ops []op, i int) int {
	depth := 0
	for j := i + 1; j < len(ops); j++ {
		switch ops[j].kind {
		case opPush:
			depth++
		case opPop:
			if depth == 0 {
				return j
			}
			depth--
		}
	}
	return len(ops)
}

func fill(img *image.Gray, r image.Rectangle, g uint8) {
	r = r.Intersect(img.Rect)
	if r.Empty() {
		return
	}
	row := img.Pix[img.PixOffset(r.Min.X, r.Min.Y):][:r.Dx()]
	for i := range row {
		row[i] = g
	}
	for y := r.Min.Y + 1; y < r.Max.Y; y++ {
		copy(img.Pix[img.PixOffset(r.Min.X, y):], row)
	}
}

func invert(img *image.Gray, r image.Rectangle) {
	r = r.Intersect(img.Rect)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		row := img.Pix[img.PixOffset(r.Min.X, y):][:r.Dx()]
		for i, v := range row {
			row[i] = 0xff - v
		}
	}
}

// copyGray copies src's pixels from sp on into dst's r.
func copyGray(dst *image.Gray, r image.Rectangle, src *image.Gray, sp image.Point) {
	if src == nil {
		return
	}
	c := r.Intersect(dst.Rect).Intersect(src.Rect.Sub(sp).Add(r.Min))
	if c.Empty() {
		return
	}
	sp = sp.Add(c.Min.Sub(r.Min))
	for y := 0; y < c.Dy(); y++ {
		copy(dst.Pix[dst.PixOffset(c.Min.X, c.Min.Y+y):][:c.Dx()], src.Pix[src.PixOffset(sp.X, sp.Y+y):])
	}
}

// textText paints s from the baseline's start at, clipped, and says
// where its ink went.
func textText(img *image.Gray, f font.Face, at image.Point, s string, g uint8, clip image.Rectangle) image.Rectangle {
	var drawn image.Rectangle
	dot, prev := fixedP(at.X, at.Y), rune(-1)
	for _, ch := range s {
		if prev >= 0 {
			dot.X += f.Kern(prev, ch)
		}
		dr, mask, mp, adv, ok := glyph(f, dot, ch)
		if ok {
			if in := dr.Intersect(clip); !in.Empty() {
				maskGray(img, in, g, mask, mp.Add(in.Min.Sub(dr.Min)))
				drawn = drawn.Union(in)
			}
		}
		dot.X += adv
		prev = ch
	}
	return drawn
}

// maskGray paints gray g into r through mask (at mp).
func maskGray(img *image.Gray, r image.Rectangle, g uint8, mask image.Image, mp image.Point) {
	m, ok := mask.(*image.Alpha)
	if !ok {
		draw.DrawMask(img, r, image.NewUniform(color.Gray{g}), image.Point{}, mask, mp, draw.Over)
		return
	}
	c := r.Intersect(img.Rect).Intersect(m.Rect.Sub(mp).Add(r.Min))
	if c.Empty() {
		return
	}
	mp = mp.Add(c.Min.Sub(r.Min))
	src := int(g)
	for y := 0; y < c.Dy(); y++ {
		dst := img.Pix[img.PixOffset(c.Min.X, c.Min.Y+y):][:c.Dx()]
		as := m.Pix[m.PixOffset(mp.X, mp.Y+y):][:len(dst)]
		for i, a := range as {
			switch a {
			case 0:
			case 0xff:
				dst[i] = g
			default:
				d := int(dst[i])
				dst[i] = uint8(d + (src-d)*int(a)/0xff)
			}
		}
	}
}

// The glyphs drawn so far, as the face gave them at a dot's fraction of
// a pixel: opentype rasterizes each anew, the most of drawing text.
type glyphKey struct {
	f    font.Face
	ch   rune
	x, y fixed.Int26_6
}

type cachedGlyph struct {
	dr   image.Rectangle // from the dot, floored
	mask *image.Alpha
	adv  fixed.Int26_6
	ok   bool
}

var (
	glyphMu    sync.Mutex
	glyphCache = map[glyphKey]cachedGlyph{}
)

const glyphCacheMax = 8192

func glyph(f font.Face, dot fixed.Point26_6, ch rune) (image.Rectangle, image.Image, image.Point, fixed.Int26_6, bool) {
	k := glyphKey{f, ch, dot.X & 63, dot.Y & 63}
	at := image.Pt(dot.X.Floor(), dot.Y.Floor())
	glyphMu.Lock()
	defer glyphMu.Unlock()
	if g, ok := glyphCache[k]; ok {
		return g.dr.Add(at), g.mask, image.Point{}, g.adv, g.ok
	}
	dr, mask, mp, adv, ok := f.Glyph(dot, ch)
	a, isAlpha := mask.(*image.Alpha)
	if !isAlpha && ok {
		return dr, mask, mp, adv, ok
	}
	g := cachedGlyph{dr: dr.Sub(at), adv: adv, ok: ok}
	if ok { // the face draws its next glyph into the same mask: a copy
		g.mask = image.NewAlpha(image.Rect(0, 0, dr.Dx(), dr.Dy()))
		for y := 0; y < dr.Dy(); y++ {
			copy(g.mask.Pix[y*g.mask.Stride:][:dr.Dx()], a.Pix[a.PixOffset(mp.X, mp.Y+y):])
		}
	}
	if len(glyphCache) >= glyphCacheMax {
		clear(glyphCache)
	}
	glyphCache[k] = g
	return g.dr.Add(at), g.mask, image.Point{}, adv, ok
}
