package main

import (
	"slices"
	"strings"
	"testing"
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
	} {
		w.ovSel = st.sel
		if !w.pageEdit(st.id) {
			t.Fatalf("%s: can't", st.id)
		}
		if got := names(); got != st.want || w.ovSel != st.wantSel || len(nb.meta) != len(nb.pages) {
			t.Errorf("%s on %d: %s, chosen %d, %d metas; want %s, %d", st.id, st.sel, got, w.ovSel, len(nb.meta), st.want, st.wantSel)
		}
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
