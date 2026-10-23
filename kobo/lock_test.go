package kobo

import (
	"image"
	"testing"
)

func TestSwipeDir(t *testing.T) {
	for _, c := range []struct {
		v    image.Point
		dir  int
		want bool
	}{
		{image.Pt(0, -400), SwipeUp, true},
		{image.Pt(150, -400), SwipeUp, true},
		{image.Pt(0, 400), SwipeDown, true},
		{image.Pt(400, 30), SwipeRight, true},
		{image.Pt(-400, 0), SwipeLeft, true},
		{image.Pt(0, -200), 0, false},   // too short: a brush
		{image.Pt(300, -400), 0, false}, // a slant
		{image.Pt(0, 0), 0, false},      // a tap
	} {
		d, ok := swipeDir(c.v, 350)
		if ok != c.want || ok && d != c.dir {
			t.Errorf("swipeDir(%v) = %d, %v; want %d, %v", c.v, d, ok, c.dir, c.want)
		}
	}
}
