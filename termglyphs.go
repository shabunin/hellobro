package main

// Drawing the terminal's characters: font glyphs, cached per cell size,
// and box-drawing, block and braille characters drawn to fill the cell.

import (
	"image"
	"image/color"
	"image/draw"
	"unicode/utf8"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

type glyphKey struct {
	s     string
	bold  bool
	width int
}

func (a *app) glyph(s string, bold bool, width int) *image.Alpha {
	v := &a.tv
	k := glyphKey{s, bold, width}
	if g, ok := v.glyphs[k]; ok {
		return g
	}
	if v.glyphs == nil || len(v.glyphs) > 4000 {
		v.glyphs = map[glyphKey]*image.Alpha{}
	}
	m := image.NewAlpha(image.Rect(0, 0, v.cw*width, v.ch))
	r, _ := utf8.DecodeRuneInString(s)
	if !drawSpecial(m, r) {
		f := a.f.term
		if bold {
			f = a.f.termBold
		}
		if _, ok := f.GlyphAdvance(r); !ok {
			f = a.f.termFallback // Go Mono lacks it: try the proportional Go font, centered
		}
		x := 0
		if f == a.f.termFallback {
			if _, ok := f.GlyphAdvance(r); !ok {
				drawMissing(m)
				v.glyphs[k] = m
				return m
			}
			x = (v.cw*width - textWidth(f, s)) / 2
		}
		d := font.Drawer{Dst: m, Src: image.Opaque, Face: f, Dot: fixed.P(x, 1+a.f.term.Metrics().Ascent.Ceil())}
		d.DrawString(s)
	}
	v.glyphs[k] = m
	return m
}

// box-drawing segments: up, right, down, left; 1 light, 2 heavy, 3 double.
var boxChars = map[rune][4]uint8{
	'─': {0, 1, 0, 1}, '│': {1, 0, 1, 0}, '┌': {0, 1, 1, 0}, '┐': {0, 0, 1, 1}, '└': {1, 1, 0, 0}, '┘': {1, 0, 0, 1},
	'├': {1, 1, 1, 0}, '┤': {1, 0, 1, 1}, '┬': {0, 1, 1, 1}, '┴': {1, 1, 0, 1}, '┼': {1, 1, 1, 1},
	'━': {0, 2, 0, 2}, '┃': {2, 0, 2, 0}, '┏': {0, 2, 2, 0}, '┓': {0, 0, 2, 2}, '┗': {2, 2, 0, 0}, '┛': {2, 0, 0, 2},
	'┣': {2, 2, 2, 0}, '┫': {2, 0, 2, 2}, '┳': {0, 2, 2, 2}, '┻': {2, 2, 0, 2}, '╋': {2, 2, 2, 2},
	'═': {0, 3, 0, 3}, '║': {3, 0, 3, 0}, '╔': {0, 3, 3, 0}, '╗': {0, 0, 3, 3}, '╚': {3, 3, 0, 0}, '╝': {3, 0, 0, 3},
	'╠': {3, 3, 3, 0}, '╣': {3, 0, 3, 3}, '╦': {0, 3, 3, 3}, '╩': {3, 3, 0, 3}, '╬': {3, 3, 3, 3},
	'╭': {0, 1, 1, 0}, '╮': {0, 0, 1, 1}, '╯': {1, 0, 0, 1}, '╰': {1, 1, 0, 0},
	'╴': {0, 0, 0, 1}, '╵': {1, 0, 0, 0}, '╶': {0, 1, 0, 0}, '╷': {0, 0, 1, 0},
	'┄': {0, 1, 0, 1}, '┈': {0, 1, 0, 1}, '╌': {0, 1, 0, 1}, '┅': {0, 2, 0, 2}, '┉': {0, 2, 0, 2}, '╍': {0, 2, 0, 2},
	'┆': {1, 0, 1, 0}, '┊': {1, 0, 1, 0}, '╎': {1, 0, 1, 0}, '┇': {2, 0, 2, 0}, '┋': {2, 0, 2, 0}, '╏': {2, 0, 2, 0},
}

// drawSpecial draws box-drawing, block and braille characters to fill the
// cell exactly, so lines and blocks join up across cells (font glyphs don't).
func drawSpecial(m *image.Alpha, r rune) bool {
	b := m.Bounds()
	w, h := b.Dx(), b.Dy()
	fill := func(x0, y0, x1, y1 int, a uint8) {
		draw.Draw(m, image.Rect(x0, y0, x1, y1).Intersect(b), image.NewUniform(color.Alpha{a}), image.Point{}, draw.Src)
	}
	if seg, ok := boxChars[r]; ok {
		t := max(2, w/8)
		cx, cy := w/2, h/2
		line := func(dir int, weight uint8) {
			th := t
			if weight == 2 {
				th = 2 * t
			}
			offs := []int{0}
			if weight == 3 {
				offs = []int{-t, t}
			}
			for _, o := range offs {
				switch dir {
				case 0: // up
					fill(cx-th/2+o, 0, cx-th/2+o+th, cy+th/2, 255)
				case 1: // right
					fill(cx-th/2, cy-th/2+o, w, cy-th/2+o+th, 255)
				case 2: // down
					fill(cx-th/2+o, cy-th/2, cx-th/2+o+th, h, 255)
				case 3: // left
					fill(0, cy-th/2+o, cx+th/2, cy-th/2+o+th, 255)
				}
			}
		}
		for dir, weight := range seg {
			if weight > 0 {
				line(dir, weight)
			}
		}
		return true
	}
	tri := func(dir int) { // a filled triangle pointing right, left, up or down
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				u, v := float64(x)/float64(w), float64(y)/float64(h) // 0..1
				var in bool
				switch dir {
				case 0:
					in = u >= 0.2 && u <= 0.85 && abs(int((v-0.5)*1000)) <= int((0.85-u)/0.65*0.3*1000)
				case 1:
					in = u >= 0.15 && u <= 0.8 && abs(int((v-0.5)*1000)) <= int((u-0.15)/0.65*0.3*1000)
				case 2:
					in = v >= 0.3 && v <= 0.7 && abs(int((u-0.5)*1000)) <= int((v-0.3)/0.4*0.4*1000)
				case 3:
					in = v >= 0.3 && v <= 0.7 && abs(int((u-0.5)*1000)) <= int((0.7-v)/0.4*0.4*1000)
				}
				if in {
					m.SetAlpha(x, y, color.Alpha{255})
				}
			}
		}
	}
	stroke := func(x0, y0, x1, y1 float64) { // a thick line, in cell fractions
		steps := 2 * max(w, h)
		th := max(1, w/7)
		for i := 0; i <= steps; i++ {
			f := float64(i) / float64(steps)
			cx, cy := int((x0+(x1-x0)*f)*float64(w)), int((y0+(y1-y0)*f)*float64(h))
			fill(cx-th/2, cy-th/2, cx-th/2+th, cy-th/2+th, 255)
		}
	}
	switch {
	case r == '▶' || r == '►' || r == '▸':
		tri(0)
	case r == '◀' || r == '◄' || r == '◂':
		tri(1)
	case r == '▲' || r == '▴':
		tri(2)
	case r == '▼' || r == '▾':
		tri(3)
	case r == '✓' || r == '✔':
		stroke(0.15, 0.55, 0.4, 0.75)
		stroke(0.4, 0.75, 0.88, 0.25)
	case r == '✗' || r == '✘':
		stroke(0.2, 0.3, 0.8, 0.75)
		stroke(0.8, 0.3, 0.2, 0.75)
	case r == '█':
		fill(0, 0, w, h, 255)
	case r == '▀':
		fill(0, 0, w, h/2, 255)
	case r == '▔':
		fill(0, 0, w, h/8, 255)
	case r >= '▁' && r <= '▇': // lower eighths
		fill(0, h-h*int(r-'▁'+1)/8, w, h, 255)
	case r >= '▉' && r <= '▏': // left eighths, 7/8 down to 1/8
		fill(0, 0, w*int('▏'-r+1)/8, h, 255)
	case r == '▐':
		fill(w/2, 0, w, h, 255)
	case r == '▕':
		fill(w-w/8, 0, w, h, 255)
	case r >= '░' && r <= '▓':
		fill(0, 0, w, h, uint8(64*int(r-'░'+1)))
	case r >= '▖' && r <= '▟':
		// quadrants: bits UL, UR, LL, LR
		q := map[rune]int{'▖': 4, '▗': 8, '▘': 1, '▙': 1 | 4 | 8, '▚': 1 | 8, '▛': 1 | 2 | 4, '▜': 1 | 2 | 8, '▝': 2, '▞': 2 | 4, '▟': 2 | 4 | 8}[r]
		for i, rr := range []image.Rectangle{image.Rect(0, 0, w/2, h/2), image.Rect(w/2, 0, w, h/2), image.Rect(0, h/2, w/2, h), image.Rect(w/2, h/2, w, h)} {
			if q&(1<<i) != 0 {
				fill(rr.Min.X, rr.Min.Y, rr.Max.X, rr.Max.Y, 255)
			}
		}
	case r >= 0x2800 && r <= 0x28FF: // braille: 2×4 dots
		bits := int(r - 0x2800)
		dots := [8][2]int{{0, 0}, {0, 1}, {0, 2}, {1, 0}, {1, 1}, {1, 2}, {0, 3}, {1, 3}}
		dw, dh := w/2, h/4
		s := max(2, min(dw, dh)*2/3)
		for i, d := range dots {
			if bits&(1<<i) != 0 {
				x0, y0 := d[0]*dw+(dw-s)/2, d[1]*dh+(dh-s)/2
				fill(x0, y0, x0+s, y0+s, 255)
			}
		}
	default:
		return false
	}
	return true
}

// drawMissing marks a character we have no glyph for: a small hollow box.
func drawMissing(m *image.Alpha) {
	b := m.Bounds()
	r := image.Rect(b.Dx()/4, b.Dy()/4, b.Dx()*3/4, b.Dy()*3/4)
	for x := r.Min.X; x < r.Max.X; x++ {
		m.SetAlpha(x, r.Min.Y, color.Alpha{160})
		m.SetAlpha(x, r.Max.Y-1, color.Alpha{160})
	}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		m.SetAlpha(r.Min.X, y, color.Alpha{160})
		m.SetAlpha(r.Max.X-1, y, color.Alpha{160})
	}
}
