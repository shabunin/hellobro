package main

import (
	"image"
	"os"
	"testing"

	"hellobro/kobo"
	"hellobro/wm"
)

// TestNotePlace: a notebook opens where it was left, after a restart
// too; a page gone since, at its last.
func TestNotePlace(t *testing.T) {
	defer func(r string) { shareRoot = r }(shareRoot)
	shareRoot = t.TempDir()
	os.Remove(kobo.AppFile(notePlacesFile))
	defer os.Remove(kobo.AppFile(notePlacesFile))
	nb := &notebook{name: "plan", paper: "plain", pages: [][]*noteStroke{{lineAt(100, true)}, {lineAt(100, true)}, {lineAt(100, true)}, {lineAt(100, true)}}}
	must(t, nb.save())
	newBro := func() *bro {
		a := &bro{App: &kobo.App{}}
		a.wm.tree = wm.New(image.Rect(0, 0, 1000, 800), kobo.FoldH)
		return a
	}
	a := newBro()
	w := a.noteOpen("plan")
	if w == nil || w.cur != 0 {
		t.Fatalf("first: %+v", w)
	}
	w.cur = 2
	a.notePlaceSeen(w)
	a.notePlacesWrite()

	b := newBro() // restarted
	if w := b.noteOpen("plan"); w == nil || w.cur != 2 {
		t.Errorf("reopened at %d", w.cur)
	}
	nb.pages = nb.pages[:2]
	must(t, nb.save())
	c := newBro()
	if w := c.noteOpen("plan"); w == nil || w.cur != 1 {
		t.Errorf("shorter now: at %d", w.cur)
	}
	must(t, os.Remove(notePath("plan")))
	c.notePlacesWrite() // a notebook gone is forgotten
	if _, ok := c.places().m["plan"]; ok {
		t.Error("kept the place of a notebook gone")
	}
}
