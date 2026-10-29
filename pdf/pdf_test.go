package pdf

import (
	"image"
	"math"
	"testing"
)

func TestInk(t *testing.T) {
	m := image.NewGray(image.Rect(10, 20, 110, 220))
	for i := range m.Pix {
		m.Pix[i] = 0xff
	}
	if got := Ink(m); !got.Empty() {
		t.Errorf("Ink of a white page: %v, want empty", got)
	}
	m.Pix[m.PixOffset(30, 50)] = 0x00
	m.Pix[m.PixOffset(80, 190)] = 0x80
	m.Pix[m.PixOffset(100, 60)] = 0xf8 // nearly white: no ink
	if got, want := Ink(m), image.Rect(30, 50, 81, 191); got != want {
		t.Errorf("Ink: %v, want %v", got, want)
	}
	if got, want := (Size{595, 842}).Scaled(2), image.Pt(1190, 1684); got != want {
		t.Errorf("Scaled: %v, want %v", got, want)
	}
}

func TestWords(t *testing.T) {
	var cs []Char
	for line, s := range []string{"hello world", "foo bar baz", "the end"} {
		for k, r := range []rune(s) {
			x, y := float64(k*10), float64(line*20)
			cs = append(cs, Char{R: r, Line: line, Box: Rect{x, y, x + 10, y + 10}})
		}
	}
	for _, c := range []struct {
		r    Rect
		want string
	}{
		{Rect{25, 0, 65, 30}, "hello world\nfoo bar"}, // words cut by its sides, whole
		{Rect{0, 21, 200, 29}, "foo bar baz"},
		{Rect{82, 0, 200, 50}, "world\nbaz"}, // "the end" is left of it
		{Rect{0, 12, 200, 18}, ""},           // between the lines
		{Rect{48, 0, 52, 30}, ""},            // only spaces' middles: nothing
	} {
		if got := Words(cs, c.r); got != c.want {
			t.Errorf("Words(%v) = %q, want %q", c.r, got, c.want)
		}
	}
}

func TestCapZoom(t *testing.T) {
	nan, inf := math.NaN(), math.Inf(1)
	for _, c := range []struct{ w, h, zoom, want float64 }{
		{595, 842, 2, 2},                                 // A4 at 2: as asked
		{10, 842, 139, MaxSide / 842.0},                  // a thin strip of ink fit to a window's width: no taller than MaxSide
		{612, 14400, 4, MaxSide / 14400.0},               // a page a metre long
		{2000, 2000, 10, math.Sqrt(MaxPixels) / 2000},    // a poster: no more than MaxPixels
		{0, 842, 1, 0}, {595, 0, 1, 0}, {595, 842, 0, 0}, // nothing to draw
		{nan, 842, 1, 0}, {595, 842, nan, 0}, {595, 842, inf, 0}, {inf, 842, 1, 0},
	} {
		got := CapZoom(c.w, c.h, c.zoom)
		if math.Abs(got-c.want) > 1e-9 {
			t.Errorf("%v × %v at %v: %v, want %v", c.w, c.h, c.zoom, got, c.want)
		}
		if got > 0 {
			if err := checkDraw(got, image.Rect(0, 0, int(c.w*got), int(c.h*got))); err != nil {
				t.Errorf("%v × %v at %v: drawn at its cap: %v", c.w, c.h, c.zoom, err)
			}
		}
	}
	for _, c := range []struct {
		zoom float64
		clip image.Rectangle
	}{
		{1, image.Rectangle{}},               // empty
		{1, image.Rect(0, 0, MaxSide+1, 10)}, // too wide
		{1, image.Rect(0, 0, 10000, 10000)},  // too many pixels
		{nan, image.Rect(0, 0, 10, 10)},      // a zoom that's no number
		{-1, image.Rect(0, 0, 10, 10)},
	} {
		if checkDraw(c.zoom, c.clip) == nil {
			t.Errorf("drew %v at %v", c.clip, c.zoom)
		}
	}
}
