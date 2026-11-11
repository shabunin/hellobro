package main

import (
	"encoding/xml"
	"image"
	"slices"
	"strings"
	"testing"
)

func lineAt(y int16, diary bool) *noteStroke {
	s := &noteStroke{pts: []notePt{{100, y, 20}, {500, y, 20}}}
	if diary {
		s.attrs = []xml.Attr{{Name: xml.Name{Local: diaryAttr}, Value: "diary"}}
	}
	s.bound()
	return s
}

func TestDiaryQuestion(t *testing.T) {
	q1, ans, q2, q3 := lineAt(100, false), lineAt(200, true), lineAt(300, false), lineAt(400, false)
	if got := diaryQuestion([]*noteStroke{q1, ans, q2, q3}); !slices.Equal(got, []*noteStroke{q2, q3}) {
		t.Errorf("under the answer: %v", got)
	}
	if got := diaryQuestion([]*noteStroke{q1, q2}); len(got) != 2 {
		t.Errorf("no answer yet: %v", got)
	}
	if got := diaryQuestion([]*noteStroke{q1, ans}); len(got) != 0 {
		t.Errorf("all answered: %v", got)
	}
}

// TestDiaryLay: an answer under a question low on the page goes on on a
// page put in after it, and is one step to undo.
func TestDiaryLay(t *testing.T) {
	q := lineAt(notePageH-300, false)
	after := lineAt(200, false)
	w := &noteWin{nb: &notebook{pages: [][]*noteStroke{{q}, {after}}}, diary: &noteDiary{}}
	w.nb.padMeta()
	w.diaryLay(0, q.box, strings.Repeat("It was a dark and stormy night, and the diary answered. ", 6), 1)
	pages := map[int]bool{}
	for _, wd := range w.diary.writing {
		pages[wd.page] = true
		for _, s := range wd.strokes {
			if !isDiary(s) || !s.box.In(image.Rect(0, 0, notePageW, notePageH)) {
				t.Fatalf("a stroke off the page, or not the diary's: %v", s.box)
			}
			if wd.page == 0 && s.box.Min.Y < q.box.Max.Y {
				t.Errorf("a stroke over the question: %v", s.box)
			}
		}
	}
	if !pages[0] || !pages[1] || len(w.nb.pages) != 3 || len(w.nb.pages[1]) != 0 || w.nb.pages[2][0] != after {
		t.Fatalf("not on a page put in: on %v, %d pages", pages, len(w.nb.pages))
	}
	a := &bro{}
	for len(w.diary.writing) > 0 {
		w.diaryWriteWord(a)
	}
	if len(w.undo) != 1 || len(w.nb.pages[1]) == 0 {
		t.Fatalf("%d steps, %d strokes on the new page", len(w.undo), len(w.nb.pages[1]))
	}
	w.doUndoOps()
	if len(w.nb.pages) != 2 || !slices.Equal(w.nb.pages[0], []*noteStroke{q}) || w.nb.pages[1][0] != after {
		t.Errorf("undone: %d pages, %v", len(w.nb.pages), w.nb.pages[0])
	}
}

// doUndoOps undoes the last step, as doUndo does, without a screen.
func (w *noteWin) doUndoOps() {
	ops := w.undo[len(w.undo)-1]
	w.undo = w.undo[:len(w.undo)-1]
	for i := len(ops) - 1; i >= 0; i-- {
		w.nb.apply(ops[i], true)
	}
}
