package kobo

import (
	"image"
	"testing"
	"time"
)

// With FastTabs a tab goes up with DU, and a GL16 cleans it once the
// switching stops.
func TestFastTabs(t *testing.T) {
	a := &App{}
	tabs, page := image.Rect(0, 76, 1404, 140), image.Rect(0, 140, 1404, 1800)
	a.Mark(image.Rect(20, 200, 600, 700), true) // the old tab's, still waiting: the new one covers it
	a.fast = true
	a.Mark(tabs, true)
	a.Mark(page, true)
	a.fast = false
	a.Mark(image.Rect(0, 0, 100, 76), true) // not part of it: GL16 as ever
	if len(a.dirty) != 2 {
		t.Errorf("dirty: %v", a.dirty)
	}
	for _, d := range a.dirty {
		if d.ui != (d.r.Max.Y == 76) {
			t.Errorf("%v: ui %v", d.r, d.ui)
		}
	}
	a.dirty = nil
	a.cleanFast()
	if len(a.dirty) != 0 {
		t.Fatalf("cleaned too soon: %v", a.dirty)
	}
	a.fastClean = time.Now()
	a.cleanFast()
	if len(a.dirty) != 1 || !a.dirty[0].ui || a.dirty[0].r != tabs.Union(page) {
		t.Errorf("cleanup: %v", a.dirty)
	}
	a.dirty = nil
	a.cleanFast()
	if len(a.dirty) != 0 {
		t.Errorf("cleaned twice: %v", a.dirty)
	}
}
