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
