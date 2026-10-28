package pdf

import (
	"image"
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
