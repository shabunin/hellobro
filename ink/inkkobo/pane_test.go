package inkkobo

import (
	"image"
	"testing"

	"hellobro/ink"
)

func TestTogether(t *testing.T) {
	for _, c := range []struct {
		name string
		us   []ink.Update
		n    int
	}{
		{"one", []ink.Update{{R: image.Rect(0, 0, 10, 10)}}, 1},
		{"a list's two columns", []ink.Update{{R: image.Rect(0, 0, 500, 400)}, {R: image.Rect(600, 0, 900, 400)}}, 1},
		{"two corners", []ink.Update{{R: image.Rect(0, 0, 50, 50)}, {R: image.Rect(800, 800, 850, 850)}}, 2},
	} {
		if got := together(c.us); len(got) != c.n {
			t.Errorf("%s: %d updates, want %d", c.name, len(got), c.n)
		}
	}
	got := together([]ink.Update{{R: image.Rect(0, 0, 500, 400)}, {R: image.Rect(600, 0, 900, 400), Gray: true}})
	if got[0].R != image.Rect(0, 0, 900, 400) || !got[0].Gray {
		t.Errorf("together: %+v", got[0])
	}
}
