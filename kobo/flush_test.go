package kobo

import (
	"image"
	"testing"
	"time"
)

// While the pen writes, its ink goes to the panel before a grayscale
// update queued earlier, in pen mode; what's left of it when the pen
// lifts goes in pen mode still.
func TestFlushInkFirst(t *testing.T) {
	scr := image.Rect(0, 0, 400, 400)
	a := &App{C: Canvas{image.NewGray(scr)}, jobs: make(chan blitJob, 4)}
	next := func() blitJob {
		a.flush()
		a.blitting = false
		select {
		case j := <-a.jobs:
			return j
		default:
			return blitJob{}
		}
	}
	a.Mark(image.Rect(0, 0, 100, 20), true) // a stroke's cleanup, say
	a.Ink(true)
	a.inkSince = time.Now()
	a.Mark(image.Rect(200, 200, 220, 220), false)
	if j := next(); j.wf != "PEN" || j.img.Rect.Min.X != 200 || j.ink.IsZero() {
		t.Errorf("first %s at %v, want the ink in pen mode", j.wf, j.img.Rect)
	}
	if j := next(); j.wf != "GL16" {
		t.Errorf("then %s, want the grayscale one", j.wf)
	}
	a.Mark(image.Rect(300, 300, 310, 310), false)
	a.Ink(false) // lifted before it went
	if j := next(); j.wf != "PEN" {
		t.Errorf("the last ink %s, want pen mode", j.wf)
	}
	a.Mark(image.Rect(10, 300, 20, 310), false)
	if j := next(); j.wf != "DU" {
		t.Errorf("fast but not ink %s, want DU", j.wf)
	}
}

// While the pen writes, grayscale updates wait; a finger lets them go.
func TestFlushHoldsUI(t *testing.T) {
	scr := image.Rect(0, 0, 400, 400)
	a := &App{C: Canvas{image.NewGray(scr)}, jobs: make(chan blitJob, 4)}
	sent := func() string {
		a.flush()
		a.blitting = false
		select {
		case j := <-a.jobs:
			return j.wf
		default:
			return ""
		}
	}
	a.inkAt, a.holdUntil = time.Now(), time.Now().Add(holdAfterInk) // as a pen point leaves them
	a.Mark(image.Rect(0, 0, 100, 20), true)                          // the bar's stats
	if wf := sent(); wf != "" {
		t.Errorf("sent %s while the pen writes", wf)
	}
	a.holdUI(TouchEv{Kind: TouchHover, Pen: true})
	if wf := sent(); wf != "" {
		t.Errorf("sent %s with the pen in the air just after", wf)
	}
	a.holdUI(TouchEv{Kind: TouchDown})
	if wf := sent(); wf != "GL16" {
		t.Errorf("after a finger: %q, want the GL16", wf)
	}
	a.inkAt = time.Now().Add(-hoverWrites - time.Second) // long ago: the pen in the air isn't writing
	a.holdUntil = time.Time{}
	a.holdUI(TouchEv{Kind: TouchHover, Pen: true})
	a.Mark(image.Rect(0, 0, 100, 20), true)
	if wf := sent(); wf != "GL16" {
		t.Errorf("the pen hovering long after: %q, want the GL16", wf)
	}
}
