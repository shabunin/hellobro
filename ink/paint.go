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
	dst   *image.Gray
	areas []placed
	texts []textBox
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
		}
	}
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
