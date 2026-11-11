// Package tex draws formulas written in LaTeX, as a language model
// writes them in an answer: the math TeX lays out (fractions, roots,
// scripts, big operators and their limits, delimiters grown to fit,
// matrices, cases, aligned equations, accents...) in STIX Two Math, a
// font with every sign of it. Not TeX itself: its layout, as near as a
// few rules of it come, and what it doesn't know is written as it is.
package tex

import (
	"image"
	"image/draw"
	"math"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"

	"hellobro/third_party/fonts/gentium"
	"hellobro/third_party/fonts/stix"
)

// Result is a formula drawn: black on white, its baseline Baseline px
// from the top; and the commands it didn't know.
type Result struct {
	Img      *image.Gray
	Baseline int
	Unknown  []string
}

// maxSide is the most pixels a side of a formula has: a runaway one is
// cut there.
const maxSide = 4000

// mu keeps one formula drawn at a time: the faces aren't for more.
var mu sync.Mutex

// Render draws src, in display style (a formula on its own line) or in
// text style (in a line of text), its letters em px.
func Render(src string, display bool, em float64) Result {
	mu.Lock()
	defer mu.Unlock()
	p := newParser(src, em)
	st := styleT
	if display {
		st = styleD
	}
	b := p.top(st)
	pad := math.Ceil(em*0.06) + 1
	base := pad + math.Ceil(b.h)
	w := min(maxSide, int(math.Ceil(b.w+2*pad)))
	h := min(maxSide, int(math.Ceil(base+b.d+pad)))
	c := newCanvas(max(1, w), max(1, h))
	b.draw(c, pad, base)
	c.done()
	return Result{Img: c.img, Baseline: int(base), Unknown: p.unknown}
}

// The styles: display, text, script, scriptscript.
const (
	styleD = iota
	styleT
	styleS
	styleSS
)

var styleScale = [4]float64{1, 1, 0.7, 0.5}

// sup is the style of a script on an atom in st; frac, of a fraction's
// numerator and denominator.
func supStyle(st int) int {
	if st <= styleT {
		return styleS
	}
	return styleSS
}

func fracStyle(st int) int { return min(styleSS, st+1) }

// fonts are the fonts the signs are looked for in: STIX Two Math, then
// Gentium (Cyrillic, polytonic Greek), then Go's.
var fonts = sync.OnceValue(func() []*opentype.Font {
	var fs []*opentype.Font
	for _, data := range [][]byte{stix.OTF, gentium.TTF, goregular.TTF} {
		if f, err := opentype.Parse(data); err == nil {
			fs = append(fs, f)
		}
	}
	return fs
})

type faceKey struct {
	font int
	size int // quarter px
}

var faces = map[faceKey]font.Face{}

// face is font i at size px.
func face(i int, size float64) font.Face {
	k := faceKey{i, int(math.Round(size * 4))}
	f, ok := faces[k]
	if !ok {
		var err error
		f, err = opentype.NewFace(fonts()[i], &opentype.FaceOptions{Size: float64(k.size) / 4, DPI: 72, Hinting: font.HintingNone})
		if err != nil {
			return nil
		}
		faces[k] = f
	}
	return f
}

// faceFor is the first face that has r, at size: STIX Two Math's if
// none has.
func faceFor(r rune, size float64) font.Face {
	for i := range fonts() {
		if f := face(i, size); f != nil {
			if _, ok := f.GlyphAdvance(r); ok {
				return f
			}
		}
	}
	return face(0, size)
}

func fx(v fixed.Int26_6) float64 { return float64(v) / 64 }

// box is TeX's: w wide, h above its baseline and d under it; painted
// from its baseline's left, then what's in it.
type box struct {
	w, h, d float64
	ital    float64 // italic correction: how far its ink leans out at the right
	paint   func(c *canvas, x, y float64)
	kids    []kid
}

type kid struct {
	b    *box
	x, y float64 // y down from the parent's baseline
}

func (b *box) draw(c *canvas, x, y float64) {
	if b.paint != nil {
		b.paint(c, x, y)
	}
	for _, k := range b.kids {
		k.b.draw(c, x+k.x, y+k.y)
	}
}

// put places k in b at x, y (down), b growing to hold it.
func (b *box) put(k *box, x, y float64) {
	b.kids = append(b.kids, kid{k, x, y})
	b.h = max(b.h, k.h-y)
	b.d = max(b.d, k.d+y)
	b.w = max(b.w, x+k.w)
}

// hbox is bs side by side.
func hbox(bs ...*box) *box {
	out := &box{}
	x := 0.0
	for _, b := range bs {
		out.put(b, x, 0)
		x += b.w
	}
	out.w = x
	if len(bs) > 0 {
		out.ital = bs[len(bs)-1].ital
	}
	return out
}

func space(w float64) *box { return &box{w: w} }

// glyph is r's box at size, its height and depth its ink's.
func glyph(r rune, size float64) *box {
	f := faceFor(r, size)
	if f == nil {
		return &box{}
	}
	bd, adv, ok := f.GlyphBounds(r)
	if !ok {
		return &box{w: size / 2}
	}
	b := &box{w: fx(adv), h: max(0, -fx(bd.Min.Y)), d: max(0, fx(bd.Max.Y))}
	if bd.Max.X > bd.Min.X {
		b.ital = max(0, fx(bd.Max.X)-fx(adv))
	} else {
		b.h, b.d = 0, 0
	}
	b.paint = func(c *canvas, x, y float64) { c.glyph(f, r, x, y) }
	return b
}

// text is s in the upright font, at size: a function's name, \text's.
func text(s string, size float64, font string) *box {
	var bs []*box
	for _, r := range s {
		if r == ' ' || r == '~' {
			bs = append(bs, space(size*0.25))
			continue
		}
		bs = append(bs, glyph(letter(r, font), size))
	}
	return hbox(bs...)
}

// canvas is what a formula's drawn on: glyphs straight on, shapes
// through a rasterizer, all at once at the end (shapes that overlap add
// up to black, as they all go the same way round).
type canvas struct {
	img *image.Gray
	ras *vector.Rasterizer
}

func newCanvas(w, h int) *canvas {
	img := image.NewGray(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 0xff
	}
	return &canvas{img: img, ras: vector.NewRasterizer(w, h)}
}

func (c *canvas) glyph(f font.Face, r rune, x, y float64) {
	dot := fixed.Point26_6{X: fixed.Int26_6(math.Round(x * 64)), Y: fixed.Int26_6(math.Round(y * 64))}
	dr, mask, mp, _, ok := f.Glyph(dot, r)
	if ok {
		draw.DrawMask(c.img, dr, image.Black, image.Point{}, mask, mp, draw.Over)
	}
}

type pt struct{ x, y float64 }

// poly fills the polygon ps.
func (c *canvas) poly(ps []pt) {
	if len(ps) < 3 {
		return
	}
	area := 0.0
	for i, p := range ps {
		q := ps[(i+1)%len(ps)]
		area += p.x*q.y - q.x*p.y
	}
	if area < 0 {
		for i, j := 0, len(ps)-1; i < j; i, j = i+1, j-1 {
			ps[i], ps[j] = ps[j], ps[i]
		}
	}
	c.ras.MoveTo(float32(ps[0].x), float32(ps[0].y))
	for _, p := range ps[1:] {
		c.ras.LineTo(float32(p.x), float32(p.y))
	}
	c.ras.ClosePath()
}

// rect fills x0,y0 to x1,y1.
func (c *canvas) rect(x0, y0, x1, y1 float64) {
	c.poly([]pt{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}})
}

// line draws a stroke through ps, w thick, round at its joints.
func (c *canvas) line(w float64, ps ...pt) {
	r := w / 2
	for i, p := range ps {
		if i > 0 {
			q := ps[i-1]
			dx, dy := p.x-q.x, p.y-q.y
			l := math.Hypot(dx, dy)
			if l == 0 {
				continue
			}
			nx, ny := -dy/l*r, dx/l*r
			c.poly([]pt{{q.x + nx, q.y + ny}, {p.x + nx, p.y + ny}, {p.x - nx, p.y - ny}, {q.x - nx, q.y - ny}})
		}
		if len(ps) > 2 || i > 0 && i < len(ps)-1 {
			c.disc(p, r)
		}
	}
}

func (c *canvas) disc(p pt, r float64) {
	var ps []pt
	for i := range 10 {
		a := float64(i) * math.Pi / 5
		ps = append(ps, pt{p.x + r*math.Cos(a), p.y + r*math.Sin(a)})
	}
	c.poly(ps)
}

func (c *canvas) done() {
	c.ras.DrawOp = draw.Over
	c.ras.Draw(c.img, c.img.Bounds(), image.Black, image.Point{})
}

// quad is n points along the quadratic curve from a by ctl to b.
func quad(a, ctl, b pt, n int) []pt {
	ps := make([]pt, 0, n+1)
	for i := 0; i <= n; i++ {
		t := float64(i) / float64(n)
		u := 1 - t
		ps = append(ps, pt{u*u*a.x + 2*u*t*ctl.x + t*t*b.x, u*u*a.y + 2*u*t*ctl.y + t*t*b.y})
	}
	return ps
}
