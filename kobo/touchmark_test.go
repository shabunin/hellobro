package kobo

import (
	"image"
	"testing"
)

// A ring where it touched, a dot where it lifted, on the part of them in
// the blit only.
func TestStampMarks(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 100, 100))
	for i := range img.Pix {
		img.Pix[i] = White
	}
	stampMarks(img, []touchMark{{at: image.Pt(30, 30)}, {at: image.Pt(98, 70), dot: true}})
	for _, c := range []struct {
		x, y int
		want uint8
	}{
		{30, 30, White},             // the ring's middle
		{30 + markR, 30, Black},     // its edge
		{30, 30 - markR + 1, Black}, // its edge, above
		{30 + markR + 2, 30, White}, // outside it
		{98, 70, Black},             // the dot, cut off at the blit's edge
		{98 - markDotR - 1, 70, White},
	} {
		if got := img.GrayAt(c.x, c.y).Y; got != c.want {
			t.Errorf("(%d,%d) = %#x, want %#x", c.x, c.y, got, c.want)
		}
	}
}

// A mark at the screen's edge marks only what's on the screen: a
// framebuffer written straight has nothing past it.
func TestMarkAtEdge(t *testing.T) {
	scr := image.Rect(0, 0, 1072, 1448)
	a := &App{C: Canvas{image.NewGray(scr)}, touchMarks: true}
	a.addMark(image.Pt(1062, 650), false)
	a.addMark(image.Pt(3, 3), true)
	for _, d := range a.dirty {
		if !d.r.In(scr) {
			t.Errorf("marked %v, off the screen %v", d.r, scr)
		}
	}
	if len(a.dirty) == 0 {
		t.Error("nothing marked")
	}
}
