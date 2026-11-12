package main

import (
	"image"
	"testing"

	"hellobro/kobo"
)

func TestPageSize(t *testing.T) {
	for in, want := range map[string]image.Point{"landscape": {notePageH, notePageW}, "A4": {1877, 2654}, "portrait": {notePageW, notePageH},
		"200x120": {1787, 1072}, "200 × 120 mm": {1787, 1072}} {
		if p, _, err := parsePageSize(in); err != nil || p != want {
			t.Errorf("%q: %v %v, want %v", in, p, err, want)
		}
	}
	for _, in := range []string{"", "huge", "10x10", "5000x100", "a x b"} {
		if _, _, err := parsePageSize(in); err == nil {
			t.Errorf("%q read", in)
		}
	}
}

// A size chosen: the blank page shown takes it, and new pages; a page
// written on keeps its own. Pages of two sizes don't show side by side.
func TestPageSizeSet(t *testing.T) {
	a := &bro{App: &kobo.App{F: kobo.LoadFonts()}}
	nb := &notebook{name: "t", pages: [][]*noteStroke{{lineAt(300, false)}, nil}}
	nb.padMeta()
	w := &noteWin{nb: nb}
	body := image.Rect(0, 0, 1872, 1404) // landscape: two pages side by side
	w.place(a, kobo.Panel{Outer: body, Body: body})
	if len(w.slots) != 2 {
		t.Fatalf("%d slots, want 2 alike", len(w.slots))
	}
	if err := w.setPageSize(a, "landscape"); err != nil {
		t.Fatal(err)
	}
	if w.pageSize(0) != image.Pt(notePageW, notePageH) {
		t.Errorf("the page written on: %v, want its own", w.pageSize(0))
	}
	w.cur = 1
	if err := w.setPageSize(a, "landscape"); err != nil {
		t.Fatal(err)
	}
	if w.pageSize(1) != image.Pt(notePageH, notePageW) || w.pageSize(5) != image.Pt(notePageH, notePageW) {
		t.Errorf("the blank page %v, a new one %v: want landscape", w.pageSize(1), w.pageSize(5))
	}
	w.cur = 0
	w.place(a, w.p)
	if len(w.slots) != 1 {
		t.Errorf("%d slots for pages of two sizes", len(w.slots))
	}
	w.cur = 1
	w.place(a, w.p)
	if len(w.slots) != 1 || w.slots[0].Dx() < 1700 {
		t.Errorf("a landscape page on a landscape screen: %v", w.slots)
	}
	if nb.insertPage(2, nil, nb.newMeta()); w.pageSize(2) != image.Pt(notePageH, notePageW) {
		t.Errorf("a page put in: %v", w.pageSize(2))
	}
}
