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

// The DU of a fast tab goes out in black and white only, and the GL16
// after it with the grays.
func TestFastTabsBlackAndWhite(t *testing.T) {
	scr := image.Rect(0, 0, 200, 300)
	a := &App{C: Canvas{image.NewGray(scr)}, jobs: make(chan blitJob, 4)}
	for i := range a.C.Pix {
		a.C.Pix[i] = uint8(i) // all the grays
	}
	page := image.Rect(0, 50, 200, 300)
	a.fast = true
	a.Mark(page, true)
	a.fast = false
	a.flush()
	j := <-a.jobs
	if j.wf != "DU" || j.img.Rect != page {
		t.Fatalf("%s %v", j.wf, j.img.Rect)
	}
	for i, v := range j.img.Pix {
		if want := map[bool]uint8{true: Black, false: White}[a.C.Pix[a.C.PixOffset(0, 50)+i] < 0x80]; v != want {
			t.Fatalf("DU pixel %d: %#x, want %#x", i, v, want)
		}
	}
	a.blitting = false
	a.fastClean = time.Now()
	a.cleanFast()
	a.flush()
	j = <-a.jobs
	if j.wf != "GL16" || j.img.Rect != page || j.img.Pix[3] != a.C.Pix[a.C.PixOffset(3, 50)] {
		t.Fatalf("cleanup: %s %v", j.wf, j.img.Rect)
	}
	// Later DU updates (the terminal's) are as drawn.
	a.blitting = false
	a.Mark(image.Rect(0, 60, 100, 70), false)
	a.flush()
	if j = <-a.jobs; j.img.Pix[5] != a.C.Pix[a.C.PixOffset(5, 60)] {
		t.Errorf("a DU after: %#x", j.img.Pix[5])
	}
}
