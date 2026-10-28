package main

import (
	"image"
	"testing"

	"hellobro/kobo"
)

// TestVkLayout: the keys are all there, on the page, none on another,
// none on what's left for the windows; the rows as wide as the page.
func TestVkLayout(t *testing.T) {
	for _, page := range []image.Rectangle{image.Rect(0, 46, 1404, 1872), image.Rect(0, 46, 1872, 1404)} {
		keys, rest := vkLayout(page)
		if len(keys) != 54+6+7 {
			t.Errorf("%v: %d keys", page, len(keys))
		}
		right := 0
		for i, k := range keys {
			if !k.r.In(page) || k.r.Overlaps(rest) || k.r.Dx() < 56 || k.r.Dy() < 56 {
				t.Errorf("%v: key %d at %v (left %v)", page, k.code, k.r, rest)
			}
			for _, o := range keys[i+1:] {
				if k.r.Overlaps(o.r) {
					t.Errorf("%v: keys %d and %d overlap", page, k.code, o.code)
				}
			}
			right = max(right, k.r.Max.X)
			if k.code == kobo.KeySpace && k.r.Max.Y != page.Max.Y {
				t.Errorf("%v: the space bar at %v", page, k.r)
			}
		}
		if right != page.Max.X || rest.Dx() < page.Dx()*4/5 || rest.Dy() < page.Dy()/2 {
			t.Errorf("%v: rows to %d, %v left", page, right, rest)
		}
	}
}
