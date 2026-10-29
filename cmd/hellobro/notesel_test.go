package main

import (
	"image"
	"slices"
	"testing"
)

func TestNoteSelect(t *testing.T) {
	ink := &noteStroke{pts: []notePt{{100, 100, 20}, {300, 100, 20}}}
	ink.bound()
	pic := picStroke(&notePic{}, image.Rect(50, 50, 400, 400))
	w := &noteWin{nb: &notebook{pages: [][]*noteStroke{{pic, ink}, nil}}}
	w.selectAt(0, image.Pt(200, 100)) // the ink, over the picture
	if len(w.sel) != 1 || w.sel[0] != ink {
		t.Fatalf("took %v", w.sel)
	}
	w.selectAt(0, image.Pt(200, 300))
	if len(w.sel) != 1 || w.sel[0] != pic {
		t.Fatalf("took %v, not the picture", w.sel)
	}
	w.selectAt(0, image.Pt(900, 900)) // nothing there: let go
	if w.sel != nil || w.mark != nil {
		t.Fatalf("still %v, mark %v", w.sel, w.mark)
	}
	w.selectAt(0, image.Pt(900, 900)) // again: the paste mark
	if w.mark == nil || w.mark.at != image.Pt(900, 900) {
		t.Fatalf("mark %v", w.mark)
	}
	if page, box := w.pasteWhere(image.Pt(1000, 200)); page != 0 || box != image.Rect(404, 900, 1404, 1100) || w.mark != nil {
		t.Errorf("pasted on %d at %v (kept on the page), mark %v", page, box, w.mark)
	}
	w.selectAt(0, image.Pt(200, 300))
	w.moveSel(1, image.Pt(10, 20)) // onto the next page
	if len(w.nb.pages[0]) != 1 || len(w.nb.pages[1]) != 1 || w.selPage != 1 || w.nb.pages[1][0].box != image.Rect(60, 70, 410, 420) {
		t.Fatalf("pages %v / %v, sel on %d", w.nb.pages[0], w.nb.pages[1], w.selPage)
	}
	w.selectAt(0, image.Pt(200, 100))
	w.moveSel(0, image.Pt(-500, 5000)) // off the page: kept on it
	if b := w.nb.pages[0][0].box; !b.In(image.Rect(0, 0, notePageW, notePageH)) || b.Min.X != 0 || b.Max.Y != notePageH {
		t.Errorf("moved to %v", b)
	}
	for i := len(w.undo) - 1; i >= 0; i-- { // as doUndo does
		for j := len(w.undo[i]) - 1; j >= 0; j-- {
			w.nb.apply(w.undo[i][j], true)
		}
	}
	if pg := w.nb.pages[0]; len(pg) != 2 || !slices.Contains(pg, pic) || !slices.Contains(pg, ink) || len(w.nb.pages[1]) != 0 { // in another order: pictures are drawn first anyway
		t.Errorf("undone: %v / %v", pg, w.nb.pages[1])
	}
}
