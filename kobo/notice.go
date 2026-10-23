package kobo

import (
	"image"
	"log"
	"time"
)

// Notices for the Program to get the reader's eye: the light on a moment,
// a strip blinking, a note in the corner. The strip and the note are
// stamped at blit time, as the ring is (ring.go), so the page goes on
// being drawn under them as ever.

const (
	blinkPhase = 400 * time.Millisecond
	toastH     = 72
	toastHalo  = 8
)

type blinking struct {
	r     image.Rectangle
	left  int // phases to go: on, off, on…
	next  time.Time
	shown bool // inverted now
}

type toast struct {
	img   *image.Gray // as it shows, at its place
	until time.Time
}

// FlashLight turns the light on at level for d, if it's off, and off
// again; quietly, with no gauge. A hand on the light meanwhile keeps it.
func (a *App) FlashLight(level int, d time.Duration) {
	if a.light == nil || a.light.level > 0 || a.asleep || a.locked || a.calib != nil {
		return
	}
	if err := a.light.set(max(1, min(100, level))); err != nil {
		log.Print(err)
		return
	}
	a.lightOffAt = time.Now().Add(d)
}

// Blink inverts r n times, on and off, a strip's way of calling out.
func (a *App) Blink(r image.Rectangle, n int) {
	for i, b := range a.blinks {
		if b.r == r { // still at it: from the start
			a.blinks[i].left = 2 * n
			if b.shown { // off first
				a.blinks[i].left++
			}
			return
		}
	}
	a.blinks = append(a.blinks, blinking{r: r, left: 2 * n, next: time.Now()})
}

// Toast shows text in the page's lower right corner for d, over what's
// there; a new one takes an old one's place.
func (a *App) Toast(text string, d time.Duration) {
	a.hideToast()
	page := a.L.Page
	if page.Empty() {
		return
	}
	w := min(page.Dx()-6*Pad, TextWidth(a.F.Body, text)+56)
	r := image.Rect(page.Max.X-3*Pad-w, page.Max.Y-3*Pad-toastH, page.Max.X-3*Pad, page.Max.Y-3*Pad) // in off the panels' frames
	c := Canvas{image.NewGray(r.Inset(-toastHalo))}
	c.Fill(c.Rect, White) // a margin, to stand apart from what's under it
	c.Frame(r, 2*border, Black)
	c.Text(a.F.Body, r.Inset(24), text, Black, AlignLeft)
	a.toast = &toast{c.Gray, time.Now().Add(d)}
	a.Mark(r, true)
}

func (a *App) hideToast() {
	if a.toast != nil {
		a.Mark(a.toast.img.Rect, true)
		a.toast = nil
	}
}

// noticeTick moves the notices on: the light off, a blink's next phase,
// the toast gone.
func (a *App) noticeTick() {
	now := time.Now()
	if !a.lightOffAt.IsZero() && now.After(a.lightOffAt) {
		a.lightOffAt = time.Time{}
		if a.light != nil && a.light.level > 0 {
			a.light.set(0)
		}
	}
	live := a.blinks[:0]
	for _, b := range a.blinks {
		if now.After(b.next) {
			b.shown, b.left, b.next = !b.shown, b.left-1, now.Add(blinkPhase)
			a.Mark(b.r, true)
		}
		if b.left > 0 || b.shown {
			live = append(live, b)
		}
	}
	a.blinks = live
	if a.toast != nil && now.After(a.toast.until) {
		a.hideToast()
	}
}

// stampNotices puts the blinks and the toast on img, a blit's.
func (a *App) stampNotices(img *image.Gray) {
	for _, b := range a.blinks {
		if !b.shown {
			continue
		}
		s := b.r.Intersect(img.Rect)
		for y := s.Min.Y; y < s.Max.Y; y++ {
			row := img.Pix[img.PixOffset(s.Min.X, y):][:s.Dx()]
			for i, v := range row {
				row[i] = 0xff - v
			}
		}
	}
	if t := a.toast; t != nil && img.Rect.Overlaps(t.img.Rect) {
		CopyGray(img, t.img.Rect, t.img, t.img.Rect.Min)
	}
}
