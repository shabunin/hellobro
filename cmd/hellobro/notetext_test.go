package main

import (
	"bytes"
	"image"
	"strings"
	"testing"

	"hellobro/kobo"
	"hellobro/wm"
)

// A text goes into the .xopp as Xournal++ has it, and comes back where
// it was, its size and words as they were.
func TestNoteTextXopp(t *testing.T) {
	s := textStroke(&noteText{s: "x² < 1 & so\non", size: 14}, image.Pt(300, 450))
	nb := &notebook{name: "t", paper: "plain", pages: [][]*noteStroke{{s}}}
	out := string(nb.xoppText())
	for _, want := range []string{`<text font="Sans" color="#000000ff" size="14.00" x="95.15" y="142.73">x² &lt; 1 &amp; so&#xA;on</text>`} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in\n%s", want, out)
		}
	}
	back, err := parseXopp("t", bytes.NewReader([]byte(out)))
	if err != nil {
		t.Fatal(err)
	}
	if len(back.pages[0]) != 1 || back.pages[0][0].txt == nil {
		t.Fatalf("read back: %v", back.pages[0])
	}
	b := back.pages[0][0]
	if b.txt.s != s.txt.s || b.txt.size != 14 || b.box.Min != s.box.Min || b.box != s.box {
		t.Errorf("read back %q %v at %v, wrote %q at %v", b.txt.s, b.txt.size, b.box, s.txt.s, s.box)
	}
}

// Typing: words, a new line, the cursor moved, a letter taken back; done,
// it's undone as one; a text emptied is gone, and nothing to undo.
func TestNoteTyping(t *testing.T) {
	a := &bro{App: &kobo.App{F: kobo.LoadFonts(), Keyboard: "a keyboard"}}
	a.wm.tree = wm.New(image.Rect(0, 0, 1000, 800), kobo.FoldH)
	a.wm.wins = map[*wm.Node]window{}
	w := &noteWin{nb: &notebook{name: "t", paper: "plain", pages: [][]*noteStroke{nil}}, texting: true}
	typeIn := func(s string) {
		for _, r := range s {
			w.typeKey(a, 0, string(r))
		}
	}
	w.textAt(a, 0, image.Pt(100, 200))
	typeIn("helo")
	w.typeKey(a, kobo.KeyEnter, "")
	typeIn("world")
	w.typeKey(a, keyUp, "")   // from "world"'s end to "helo"'s, shorter
	w.typeKey(a, keyLeft, "") // before its o
	typeIn("l")
	w.typeKey(a, keyEnd, "")
	w.typeKey(a, kobo.KeyBackspace, "")
	typeIn("o")
	if got := w.typing.s.txt.s; got != "hello\nworld" {
		t.Fatalf("typed %q", got)
	}
	w.typeKey(a, kobo.KeyEsc, "")
	if w.typing != nil || len(w.nb.pages[0]) != 1 || len(w.undo) != 1 {
		t.Fatalf("done: typing %v, %d on the page, %d to undo", w.typing, len(w.nb.pages[0]), len(w.undo))
	}
	s := w.nb.pages[0][0]
	if s.box.Min.X != 100 || s.box.Dy() < 2*int(s.txt.px()) {
		t.Errorf("the text's box %v", s.box)
	}
	w.textAt(a, 0, s.box.Min.Add(image.Pt(5, 5))) // into it again: unchanged, as it was
	w.endTyping(a)
	if len(w.undo) != 1 || w.nb.pages[0][0] != s {
		t.Errorf("unchanged: %d to undo, the same %v", len(w.undo), w.nb.pages[0][0] == s)
	}
	w.textAt(a, 0, s.box.Min.Add(image.Pt(5, 5))) // emptied: gone, undone as one
	for range len("hello\nworld") {
		w.typeKey(a, kobo.KeyBackspace, "")
	}
	w.endTyping(a)
	if len(w.nb.pages[0]) != 0 || len(w.undo) != 2 {
		t.Fatalf("emptied: %d on the page, %d to undo", len(w.nb.pages[0]), len(w.undo))
	}
	w.doUndo(a, true)
	if len(w.nb.pages[0]) != 1 || w.nb.pages[0][0] != s {
		t.Errorf("undone: %v", w.nb.pages[0])
	}
	w.textAt(a, 0, image.Pt(600, 600)) // a new one left empty: nothing
	w.endTyping(a)
	if len(w.nb.pages[0]) != 1 || len(w.undo) != 1 {
		t.Errorf("an empty one: %d on the page, %d to undo", len(w.nb.pages[0]), len(w.undo))
	}
}
