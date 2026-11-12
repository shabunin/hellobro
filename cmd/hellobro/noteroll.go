package main

// A notebook as a roll ("Pages" in its settings): a page as wide as the
// window, scrolled up and down a screenful at a time, by a finger
// swiped or the arrows, and it grows as it's written on. A swipe up
// near its foot, with something written on the screen or under it,
// adds paper (a page can't grow past rollMaxH: the next page then goes
// on from its foot); on a blank screen at its foot, it's the next
// page's top. The AI diary's answers make room for themselves the same
// way, and the roll follows them. Swiped across, the pages turn.
//
// It's still pages in the .xopp, only taller: Xournal++ shows them so,
// and export makes each one PDF page.

import (
	"image"
	"math"
	"slices"
	"time"

	"hellobro/kobo"
)

const (
	rollAttr = "hellobro-roll" // the notebook's: "on", it's a roll
	rollMaxH = 30000           // a page's height at most, units: a stroke's points are int16
	rollStep = 0.85            // of a screen, a scroll
)

func (nb *notebook) rollOn() bool { return attr(nb.attrs, rollAttr) == "on" }

// roll is whether its pages show as a roll now: the overview's are
// small pages, whatever it is.
func (w *noteWin) roll() bool { return w.nb.rollOn() && !w.overview }

func (w *noteWin) flipRoll(a *bro) {
	w.nb.attrs = setAttr(w.nb.attrs, rollAttr, onOff(!w.nb.rollOn()))
	w.changed = time.Now()
	w.scroll = 0
}

// rollSlot is where page size shows in area as a roll, scrolled down
// scroll units: as wide as the area (no wider than itself, landscape),
// from the area's top less what's scrolled past.
func rollSlot(area image.Rectangle, size image.Point, scroll int) (image.Rectangle, float64) {
	scale := math.Min(1, float64(area.Dx()-kobo.Pad)/float64(size.X))
	pw, ph := int(float64(size.X)*scale), int(float64(size.Y)*scale)
	x := area.Min.X + (area.Dx()-pw)/2
	y := area.Min.Y - int(float64(scroll)*scale)
	return image.Rect(x, y, x+pw, y+ph), scale
}

// rollView is how much of a page a screen shows, units high.
func (w *noteWin) rollView() int {
	if w.scale <= 0 {
		return notePageH
	}
	return int(float64(w.paperArea().Dy()) / w.scale)
}

// rollEnd is how far page n scrolls: its foot at the screen's.
func (w *noteWin) rollEnd(n int) int { return max(0, w.pageSize(n).Y-w.rollView()) }

// inkFoot is how far down page n is written on.
func (w *noteWin) inkFoot(n int) int {
	foot := 0
	for _, s := range w.strokes(n) {
		foot = max(foot, s.box.Max.Y)
	}
	return foot
}

// growPage makes page n h units high, if it's less, and can grow: not
// a PDF's page, nor past rollMaxH. False if it's not as high as that.
func (w *noteWin) growPage(n, h int) bool {
	if n < 0 || h > rollMaxH || w.pdfPage(n) >= 0 {
		return false
	}
	if h <= w.pageSize(n).Y {
		return true
	}
	w.nb.padMeta()
	for len(w.nb.meta) <= n {
		w.nb.meta = append(w.nb.meta, defaultMeta())
	}
	w.nb.meta[n].h = math.Round(float64(h)*notePt72*100) / 100
	w.changed = time.Now()
	return true
}

// rollTo is where a scroll by screenfuls (on, or back if negative)
// gets to: the page and how far down it; ok false if it can't go. On,
// it's a whole step, the page grown for it if there's something
// written from the screen's top down (more paper: it goes on); else as
// far as the page goes, and from its foot, the next page.
func (w *noteWin) rollTo(by int, grow bool) (page, scroll int, ok bool) {
	step := int(float64(w.rollView()) * rollStep)
	end := w.rollEnd(w.cur)
	switch {
	case by < 0 && w.scroll > 0:
		return w.cur, max(0, w.scroll-step), true
	case by < 0 && w.cur > 0:
		return w.cur - 1, w.rollEnd(w.cur - 1), true
	case by < 0:
		return 0, 0, false
	case w.scroll+step <= end:
		return w.cur, w.scroll + step, true
	}
	if to := w.scroll + step; w.inkFoot(w.cur) > w.scroll && w.pdfPage(w.cur) < 0 && to+w.rollView() <= rollMaxH {
		if grow {
			w.growPage(w.cur, to+w.rollView())
		}
		return w.cur, to, true
	}
	if w.scroll < end {
		return w.cur, end, true
	}
	if n := w.turnTo(1); n >= 0 {
		return n, 0, true
	}
	return 0, 0, false
}

// rollBy scrolls by screenfuls.
func (w *noteWin) rollBy(a *bro, by int) {
	page, scroll, ok := w.rollTo(by, true)
	if !ok {
		return
	}
	w.cur, w.scroll = page, scroll
	w.place(a, w.p)
	w.show(a)
	if a.shows(w) {
		a.MarkFlash(w.paperArea())
	}
}

// rollShow scrolls page n's units y0 to y1 into sight, if they aren't:
// near the top, to be read on from there. False if they were.
func (w *noteWin) rollShow(a *bro, n, y0, y1 int) bool {
	if n == w.cur && y0 >= w.scroll && y1 <= w.scroll+w.rollView() {
		return false
	}
	w.cur = n
	w.scroll = max(0, min(w.rollEnd(n), y0-w.rollView()/8))
	w.place(a, w.p)
	w.show(a)
	if a.shows(w) {
		a.MarkFlash(w.paperArea())
	}
	return true
}

// slotAt is the slot p is in, where the paper shows; -1 if none.
func (w *noteWin) slotAt(p image.Point) int {
	if !p.In(w.paperArea()) {
		return -1
	}
	return slices.IndexFunc(w.slots, func(r image.Rectangle) bool { return p.In(r) })
}

// rollRoom makes room on a roll's page n down to foot (and a margin
// under it), for an answer: false if it's not a roll, or the page
// can't grow so far.
func (w *noteWin) rollRoom(n, foot int) bool {
	return w.nb.rollOn() && w.growPage(n, foot+diaryMargin/2)
}
