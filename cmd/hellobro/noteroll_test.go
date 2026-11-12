package main

import (
	"image"
	"testing"

	"hellobro/kobo"
)

// A roll scrolls a screenful at a time, a whole step: grown for it,
// written on; on a blank screen at its foot, it's the next page; back
// up past its top, the page before's foot.
func TestRoll(t *testing.T) {
	a := &bro{App: &kobo.App{F: kobo.LoadFonts()}}
	near := lineAt(notePageH-200, false)
	nb := &notebook{name: "t", pages: [][]*noteStroke{{near}, {lineAt(300, false)}}}
	nb.attrs = setAttr(nb.attrs, rollAttr, "on")
	nb.padMeta()
	w := &noteWin{nb: nb}
	body := image.Rect(0, 0, 1404, 1872)
	w.place(a, kobo.Panel{Outer: body, Body: body})
	if len(w.slots) != 1 || w.slots[0].Dx() > body.Dx() || w.scale > 1 {
		t.Fatalf("slots %v, scale %v", w.slots, w.scale)
	}
	view := w.rollView()
	if view >= notePageH || view < notePageH*3/4 {
		t.Fatalf("a screen shows %d units of %d", view, notePageH)
	}
	if !w.cantTurn(-1) || w.cantTurn(1) {
		t.Errorf("at the top: back %v, on %v", !w.cantTurn(-1), !w.cantTurn(1))
	}
	if end := w.rollEnd(0); end <= 0 {
		t.Fatalf("the page scrolls %d", end)
	}
	page, scroll, _ := w.rollTo(1, true) // written on: a whole step, grown for it
	step := int(float64(view) * rollStep)
	if page != 0 || scroll != step || w.pageSize(0).Y != step+view {
		t.Errorf("on: page %d at %d, %d high; want 0 at %d, %d high", page, scroll, w.pageSize(0).Y, step, step+view)
	}
	w.place(a, w.p)
	w.scroll = w.rollEnd(0)
	nb.pages[0] = []*noteStroke{lineAt(100, false)} // nothing on the screen or under it
	page, scroll, _ = w.rollTo(1, true)
	if page != 1 || scroll != 0 {
		t.Errorf("past the foot, nothing near it: page %d at %d, want the next's top", page, scroll)
	}
	w.cur, w.scroll = 1, 0
	page, scroll, _ = w.rollTo(-1, true)
	if page != 0 || scroll != w.rollEnd(0) {
		t.Errorf("back past the top: page %d at %d, want the first's foot %d", page, scroll, w.rollEnd(0))
	}
	if !w.growPage(1, 5000) || w.pageSize(1).Y != 5000 || w.growPage(1, rollMaxH+1) {
		t.Errorf("growing: %v", w.pageSize(1))
	}
}

// An answer on a roll goes on down the page, grown for it, and not on
// a page put in.
func TestRollAnswer(t *testing.T) {
	q := lineAt(notePageH-400, false)
	w := &noteWin{nb: &notebook{name: "t", pages: [][]*noteStroke{{q}}}, diary: &noteDiary{}}
	w.nb.attrs = setAttr(w.nb.attrs, rollAttr, "on")
	w.nb.padMeta()
	w.diaryLayMD(0, q.box, diaryReply{Status: "answer", Question: "roots?", Answer: mdAnswer}, famGentium, 15)
	a := &bro{}
	for len(w.diary.writing) > 0 {
		w.diaryWriteWord(a)
	}
	if len(w.nb.pages) != 1 {
		t.Errorf("%d pages, want the one", len(w.nb.pages))
	}
	h := w.pageSize(0).Y
	if h <= notePageH {
		t.Errorf("the page %d high: not grown", h)
	}
	for _, s := range w.nb.pages[0] {
		if s.box.Max.Y > h {
			t.Errorf("%v past the page's foot %d", s.box, h)
		}
	}
}
