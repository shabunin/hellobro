package main

import (
	"image"
	"slices"
	"strings"
	"testing"

	"hellobro/kobo"
	"hellobro/wm"
)

func TestNotePages(t *testing.T) {
	a, b, c := &noteStroke{pts: []notePt{{1, 1, 10}}}, &noteStroke{pts: []notePt{{2, 2, 10}}}, &noteStroke{pts: []notePt{{3, 3, 10}}}
	nb := &notebook{pages: [][]*noteStroke{{a}, {b}, {c}}, meta: []notePage{{w: 100, h: 200}}} // the first's from a file
	w := &noteWin{nb: nb}
	names := func() string { // each page by its stroke's x, 0 for none
		var s []string
		for _, pg := range nb.pages {
			if len(pg) == 0 {
				s = append(s, "0")
				continue
			}
			s = append(s, string(rune('0'+pg[0].pts[0].x)))
		}
		return strings.Join(s, "")
	}
	for _, st := range []struct {
		sel        int
		id, want   string
		wantSel    int
		wantFirstW float64 // the first page's width in the file: its meta goes with it
	}{
		{0, "later", "213", 1, 0},
		{1, "copy page", "2113", 2, 0},
		{3, "earlier", "2131", 2, 0},
		{0, "new before", "02131", 0, 0},
		{0, "delete", "2131", 0, 0},
		{3, "new after", "21310", 4, 0},
		{0, "clear", "01310", 0, 0},
	} {
		w.ovSel = st.sel
		if !w.pageEdit(st.id) {
			t.Fatalf("%s: can't", st.id)
		}
		if got := names(); got != st.want || w.ovSel != st.wantSel || len(nb.meta) != len(nb.pages) {
			t.Errorf("%s on %d: %s, chosen %d, %d metas; want %s, %d", st.id, st.sel, got, w.ovSel, len(nb.meta), st.want, st.wantSel)
		}
	}
	if w.pageEdit("clear") { // the page cleared again
		t.Error("an empty page cleared")
	}
	if nb.meta[1].w != 100 { // the first page, now second
		t.Errorf("its meta: %v", nb.meta)
	}
	if nb.pages[1][0] == nb.pages[2][0] {
		t.Error("a copy page shares its strokes")
	}
	for i := len(w.undo) - 1; i >= 0; i-- { // as doUndo does
		for j := len(w.undo[i]) - 1; j >= 0; j-- {
			nb.apply(w.undo[i][j], true)
		}
	}
	if got := names(); got != "123" || !slices.Equal(nb.pages[0], []*noteStroke{a}) || nb.meta[0].w != 100 {
		t.Errorf("all undone: %s, %v", got, nb.meta)
	}
	one := &noteWin{nb: &notebook{pages: [][]*noteStroke{{a}}}}
	if !one.pageEdit("delete") || len(one.nb.pages) != 1 || len(one.nb.pages[0]) != 0 {
		t.Errorf("the only page deleted: %v", one.nb.pages)
	}
	if one.pageEdit("delete") {
		t.Error("an empty only page deleted")
	}
}

// TestNotePagesPastEnd: a choice past the last page (the screenful of
// the new page's tile alone) changes nothing, and doesn't crash.
func TestNotePagesPastEnd(t *testing.T) {
	a := &noteStroke{pts: []notePt{{1, 1, 10}}}
	for _, id := range []string{"copy page", "delete", "clear", "earlier", "later", "new before", "new after"} {
		w := &noteWin{nb: &notebook{pages: [][]*noteStroke{{a}, {a}}}, ovSel: 2}
		if w.pageEdit(id) || len(w.nb.pages) != 2 {
			t.Errorf("%s past the end: did it, %d pages", id, len(w.nb.pages))
		}
	}
}

// TestNotePageClose: a page's × asks first, and the second tap deletes
// that page (not the one chosen before); a tap elsewhere in between
// takes the ask back. A lone blank page has none.
func TestNotePageClose(t *testing.T) {
	a := &bro{App: &kobo.App{F: kobo.LoadFonts()}}
	a.wm.tree = wm.New(image.Rect(0, 0, 1000, 800), kobo.FoldH)
	a.wm.wins = map[*wm.Node]window{}
	s := func(x int16) []*noteStroke { return []*noteStroke{{pts: []notePt{{x, x, 10}}}} }
	nb := &notebook{pages: [][]*noteStroke{s(1), s(2), s(3)}}
	w := &noteWin{nb: nb, overview: true}
	w.p.Body = image.Rect(0, 0, 1400, 1800)
	w.bar = image.Rect(0, 0, 1400, 70)
	tiles, _ := w.ovLayout()
	x := func(n int) image.Point { b := ovCloseBox(tiles[n]); return b.Min.Add(b.Max).Div(2) }
	w.ovTap(a, x(1))
	if len(nb.pages) != 3 || !w.ovDelArmed(1) || w.ovSel != 1 {
		t.Fatalf("one tap: %d pages, armed %v, chosen %d", len(nb.pages), w.ovDelArmed(1), w.ovSel)
	}
	w.ovTap(a, tiles[0].Min.Add(image.Pt(20, tiles[0].Dy()/2))) // the first page: chosen, the ask gone
	w.ovTap(a, x(1))
	if len(nb.pages) != 3 || !w.ovDelArmed(1) {
		t.Fatalf("asked again: %d pages", len(nb.pages))
	}
	w.ovTap(a, x(1))
	if len(nb.pages) != 2 || nb.pages[1][0].pts[0].x != 3 || w.ovDelArmed(1) || len(w.undo) != 1 {
		t.Fatalf("deleted: %d pages, %d undos", len(nb.pages), len(w.undo))
	}
	one := &noteWin{nb: &notebook{pages: [][]*noteStroke{nil, s(1)}}}
	if !one.ovDelOK(0) || !one.ovDelOK(1) || one.ovDelOK(2) {
		t.Error("a page's × where it shouldn't be")
	}
	one.nb.pages = one.nb.pages[:1]
	if one.ovDelOK(0) {
		t.Error("a lone blank page has a ×")
	}
}
