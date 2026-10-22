package kobo

import (
	"image"
	"testing"
	"time"
)

func TestBlinkAndToast(t *testing.T) {
	scr := image.Rect(0, 0, 600, 400)
	a := &App{C: Canvas{image.NewGray(scr)}, F: LoadFonts(), L: Layout{Page: scr}}
	a.C.Fill(scr, White)
	strip := image.Rect(10, 10, 300, 50)
	a.Blink(strip, 2)
	var phases []bool
	for range 10 {
		a.noticeTick()
		if len(a.blinks) == 0 {
			break
		}
		phases = append(phases, a.blinks[0].shown)
		img := image.NewGray(scr)
		CopyGray(img, scr, a.C.Gray, scr.Min)
		a.stampNotices(img)
		if got := img.GrayAt(20, 20).Y == Black; got != a.blinks[0].shown {
			t.Fatalf("phase %d: inverted %v, shown %v", len(phases), got, a.blinks[0].shown)
		}
		a.blinks[0].next = time.Now().Add(-time.Millisecond)
	}
	if len(phases) != 3 || !phases[0] || phases[1] || !phases[2] || len(a.blinks) != 0 { // and off: gone
		t.Errorf("phases %v, want on off on, then gone", phases)
	}
	a.Toast("SSH 1: build done", time.Second)
	r := a.toast.img.Rect
	if !r.In(scr) || r.Max.X != scr.Max.X-3*Pad+toastHalo || r.Max.Y != scr.Max.Y-3*Pad+toastHalo {
		t.Fatalf("toast at %v", r)
	}
	img := image.NewGray(scr)
	a.stampNotices(img)
	box := r.Inset(toastHalo)
	if img.GrayAt(r.Min.X, r.Min.Y).Y != White || img.GrayAt(box.Min.X, box.Min.Y).Y != Black || img.GrayAt(box.Min.X+2*border+2, box.Min.Y+2*border+2).Y != White {
		t.Error("toast not stamped")
	}
	a.dirty = nil
	a.toast.until = time.Now()
	a.noticeTick()
	if a.toast != nil || len(a.dirty) != 1 || a.dirty[0].r != r {
		t.Errorf("toast not gone: %v", a.dirty)
	}
}
