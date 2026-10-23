package kobo

// Touch marks (hellobro's Settings, Touch marks): a ring where a finger
// or the pen came down, and a dot where it lifted if it moved, for a few
// seconds: to see where touches land. Like the focus ring they're never
// drawn into C: flush stamps them on what goes to the panel, so taking
// one off is redrawing its box.

import (
	"image"
	"time"
)

const (
	markR    = 20 // the ring's radius
	markW    = 4  // and its width
	markDotR = 8
	markTime = 3 * time.Second
)

type touchMark struct {
	at    image.Point
	dot   bool // where it lifted
	until time.Time
}

func (m touchMark) box() image.Rectangle {
	return image.Rect(m.at.X-markR, m.at.Y-markR, m.at.X+markR+1, m.at.Y+markR+1)
}

// SetTouchMarks turns the marks on or off.
func (a *App) SetTouchMarks(on bool) {
	a.touchMarks = on
	if !on {
		for _, m := range a.marks {
			a.Mark(m.box(), true)
		}
		a.marks = nil
	}
}

func (a *App) addMark(p image.Point, dot bool) {
	if !a.touchMarks || a.locked {
		return
	}
	m := touchMark{at: p, dot: dot, until: time.Now().Add(markTime)}
	a.marks = append(a.marks, m)
	a.Mark(m.box(), true)
}

// markTick takes off the marks whose time is up.
func (a *App) markTick() {
	now := time.Now()
	live := a.marks[:0]
	for _, m := range a.marks {
		if now.After(m.until) {
			a.Mark(m.box(), true)
			continue
		}
		live = append(live, m)
	}
	a.marks = live
}

// stampMarks draws the marks into img, as far as img goes.
func stampMarks(img *image.Gray, marks []touchMark) {
	for _, m := range marks {
		b := m.box().Intersect(img.Rect)
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				dx, dy := x-m.at.X, y-m.at.Y
				d := dx*dx + dy*dy
				in := d <= markR*markR && d >= (markR-markW)*(markR-markW)
				if m.dot {
					in = d <= markDotR*markDotR
				}
				if in {
					img.Pix[img.PixOffset(x, y)] = Black
				}
			}
		}
	}
}
