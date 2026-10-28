package main

import (
	"image"
	"strings"
	"testing"
	"time"
)

func TestNotebookFile(t *testing.T) {
	in := "hellobro notebook 1\npaper lines\npage\ns 10,20,15 30,40,25\ns 5,5,40\npage\npage\ns 1,2,3\npage\npage\n"
	nb, err := parseNotebook("x", strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if nb.paper != "lines" || len(nb.pages) != 5 || len(nb.pages[0]) != 2 || len(nb.pages[2]) != 1 {
		t.Fatalf("paper %q, pages %d", nb.paper, len(nb.pages))
	}
	if s := nb.pages[0][0]; s.pts[1] != (notePt{30, 40, 25}) || s.box != image.Rect(8, 18, 34, 44) {
		t.Errorf("stroke %v, box %v", s.pts, s.box)
	}
	want := "hellobro notebook 1\npaper lines\npage\ns 10,20,15 30,40,25\ns 5,5,40\npage\npage\ns 1,2,3\n" // the empty pages at the end go
	if got := string(nb.noteText()); got != want {
		t.Errorf("written:\n%s\nwant:\n%s", got, want)
	}
	for _, bad := range []string{"", "a shopping list\n", "hellobro notebook 1\ns 1,2\n", "hellobro notebook 1\ns 1,x,3\n"} {
		if _, err := parseNotebook("x", strings.NewReader(bad)); err == nil {
			t.Errorf("%q: no error", bad)
		}
	}
	nb, _ = parseNotebook("x", strings.NewReader("hellobro notebook 1\npaper wallpaper\n"))
	if nb.paper != "squares" || len(nb.pages) != 1 {
		t.Errorf("a notebook of nothing: paper %q, %d pages", nb.paper, len(nb.pages))
	}
}

func TestNotebookUndo(t *testing.T) {
	a, b, c := &noteStroke{}, &noteStroke{}, &noteStroke{}
	nb := &notebook{pages: [][]*noteStroke{{a, b}}}
	add := noteOp{page: 0, add: []*noteStroke{c}}
	del := noteOp{page: 0, del: []*noteStroke{a, c}}
	nb.apply(add, false)
	nb.apply(del, false)
	if pg := nb.pages[0]; len(pg) != 1 || pg[0] != b {
		t.Fatalf("after: %v", pg)
	}
	nb.apply(del, true)
	nb.apply(add, true)
	if pg := nb.pages[0]; len(pg) != 2 || pg[0] != b || pg[1] != a {
		t.Fatalf("undone: %v", pg)
	}
}

func TestNoteEraser(t *testing.T) {
	s := &noteStroke{pts: []notePt{{100, 100, 20}, {300, 100, 20}}}
	s.bound()
	for _, tc := range []struct {
		p    image.Point
		want bool
	}{
		{image.Pt(200, 100), true},
		{image.Pt(200, 115), true},
		{image.Pt(200, 130), false},
		{image.Pt(90, 100), true},
		{image.Pt(60, 100), false},
		{image.Pt(320, 110), false},
	} {
		if got := s.hits(tc.p); got != tc.want {
			t.Errorf("%v: %v", tc.p, got)
		}
	}
	one := &noteStroke{pts: []notePt{{50, 50, 15}}}
	one.bound()
	if !one.hits(image.Pt(55, 55)) {
		t.Error("a dot isn't erased")
	}
}

func TestNewNoteName(t *testing.T) {
	now := time.Date(2026, 10, 28, 14, 5, 0, 0, time.UTC)
	taken := map[string]bool{"2026-10-28 14.05": true, "2026-10-28 14.05 (2)": true}
	if got := newNoteName(now, func(n string) bool { return taken[n] }); got != "2026-10-28 14.05 (3)" {
		t.Errorf("got %q", got)
	}
}

func TestNoteSlots(t *testing.T) {
	for _, tc := range []struct {
		area image.Rectangle
		n    int
	}{
		{image.Rect(0, 0, 1388, 1760), 1}, // portrait, fullscreen
		{image.Rect(0, 0, 1856, 1290), 2}, // landscape, fullscreen
		{image.Rect(0, 0, 920, 1290), 1},  // landscape, half
		{image.Rect(0, 0, 1388, 860), 2},  // portrait, the top half: wide
		{image.Rect(0, 0, 1388, 1500), 1}, // a little short
	} {
		slots, scale := noteSlots(tc.area)
		if len(slots) != tc.n {
			t.Errorf("%v: %d pages, want %d (scale %.2f)", tc.area, len(slots), tc.n, scale)
		}
		for _, s := range slots {
			if !s.In(tc.area) {
				t.Errorf("%v: a page at %v", tc.area, s)
			}
		}
		if len(slots) == 2 && slots[0].Overlaps(slots[1]) {
			t.Errorf("%v: the pages overlap", tc.area)
		}
	}
}

func TestNoteTurn(t *testing.T) {
	s := &noteStroke{}
	for _, tc := range []struct {
		pages      [][]*noteStroke
		cur, shown int
		by, want   int
	}{
		{[][]*noteStroke{{s}}, 0, 1, 1, 1},           // past the last, written on: a new page
		{[][]*noteStroke{{s}, nil}, 1, 1, 1, -1},     // past a blank one: no
		{[][]*noteStroke{{s}, nil}, 1, 1, -1, 0},     // back
		{[][]*noteStroke{nil}, 0, 1, -1, -1},         // before the first
		{[][]*noteStroke{{s}}, 0, 2, 1, -1},          // two shown, the second not written on
		{[][]*noteStroke{{s}, {s}}, 0, 2, 1, 2},      // both written on: the next two
		{[][]*noteStroke{{s}, {s}, {s}}, 0, 2, 1, 2}, // the third and a new one
		{[][]*noteStroke{{s}, {s}, {s}}, 1, 2, -1, 0},
	} {
		w := &noteWin{nb: &notebook{pages: tc.pages}, cur: tc.cur}
		for range tc.shown {
			w.slots = append(w.slots, image.Rect(0, 0, 1, 1))
		}
		if got := w.turnTo(tc.by); got != tc.want {
			t.Errorf("%d pages, at %d, %d shown, by %d: %d, want %d", len(tc.pages), tc.cur, tc.shown, tc.by, got, tc.want)
		}
	}
}
