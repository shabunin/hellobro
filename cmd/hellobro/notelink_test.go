package main

import (
	"bytes"
	"image"
	"testing"
	"time"
)

// A link keeps where it goes through the file; a finger finds it; the
// marks are the last first, the same part marked once.
func TestNoteLinks(t *testing.T) {
	an := noteAnchor{Note: "Work/plan", Page: 2, Box: [4]int{10, 20, 300, 80}, Label: "the budget"}
	if s := an.linkText("Work/plan"); s != "→ the budget (p. 3)" {
		t.Errorf("in its own notebook: %q", s)
	}
	if s := an.linkText("diary"); s != "→ the budget (plan, p. 3)" {
		t.Errorf("in another: %q", s)
	}
	nb := &notebook{name: "diary", pages: [][]*noteStroke{{linkStroke(an, "diary", image.Pt(100, 200))}}}
	got, err := parseXopp("diary", bytes.NewReader(nb.xoppText()))
	must(t, err)
	s := got.pages[0][0]
	back, ok := linkOf(s)
	if !ok || back.Note != an.Note || back.Page != an.Page || back.Box != an.Box {
		t.Fatalf("read back: %+v %v", back, ok)
	}
	w := &noteWin{nb: got}
	if w.linkAt(0, s.box.Min.Add(image.Pt(5, 5))) != s || w.linkAt(0, image.Pt(5, 5)) != nil {
		t.Error("a finger on it, or off it")
	}
	if l := selLabel([]*noteStroke{s, textStroke(&noteText{s: "Budget for 2027, the whole of it\nand more", size: 14}, image.Pt(0, 0))}); l != "Budget for 2027, the whole of it" {
		t.Errorf("the label: %q", l)
	}

	old := loadAnchors()
	defer saveAnchors(old)
	must(t, saveAnchors(nil))
	t0 := time.Now()
	must(t, keepAnchor(noteAnchor{Note: "a", Page: 0, At: t0}))
	must(t, keepAnchor(noteAnchor{Note: "b", Page: 1, At: t0.Add(time.Second)}))
	must(t, keepAnchor(noteAnchor{Note: "a", Page: 0, At: t0.Add(2 * time.Second), Label: "again"}))
	as := loadAnchors()
	if len(as) != 2 || as[0].Label != "again" || as[1].Note != "b" {
		t.Fatalf("marks: %+v", as)
	}
	must(t, forgetAnchor(as[1].At))
	if as := loadAnchors(); len(as) != 1 || as[0].Note != "a" {
		t.Errorf("one forgotten: %+v", as)
	}
	// The ribbon asks the marks kept in memory, as they're written.
	marked = nil
	if _, ok := markedAs(anchorKey{note: "a"}); !ok {
		t.Error("a: not marked")
	}
	if _, ok := markedAs(anchorKey{note: "b", page: 1}); ok {
		t.Error("b: marked still")
	}
	must(t, keepAnchor(noteAnchor{Note: "c", Page: 2, Box: [4]int{1, 2, 3, 4}, At: t0.Add(3 * time.Second)}))
	if _, ok := markedAs(anchorKey{"c", 2, [4]int{1, 2, 3, 4}}); !ok {
		t.Error("c: not marked")
	}
}
