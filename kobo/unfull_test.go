package kobo

import (
	"image"
	"testing"
)

func TestUnfulling(t *testing.T) {
	top := image.Pt(500, 40)
	for _, c := range []struct {
		name     string
		unfull   int
		unfulled bool
		to       image.Point
		want     bool
	}{
		{"down", 1, false, image.Pt(510, 100), true},
		{"down at an angle", 1, false, image.Pt(540, 90), true},
		{"sideways", 1, false, image.Pt(600, 60), false},
		{"up", 1, false, image.Pt(500, 20), false},
		{"not from the top", 0, false, image.Pt(500, 100), false},
		{"already out", 1, true, image.Pt(500, 300), false},
	} {
		g := &Gesture{Start: top, unfull: c.unfull, unfulled: c.unfulled}
		if got := g.Unfulling(c.to); got != c.want {
			t.Errorf("%s: got %v", c.name, got)
		}
	}
}
