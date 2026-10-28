package main

import (
	"image"
	"testing"

	"hellobro/kobo"
)

// TestTermFontEach: each terminal has its font's size, those of a size
// sharing a face; a new one takes the size last set. It's kept with the
// layout, before what the window was.
func TestTermFontEach(t *testing.T) {
	a := &bro{App: &kobo.App{F: kobo.LoadFonts()}}
	a.tv.fontPx = termFontDefault
	a.C = kobo.Canvas{Gray: image.NewGray(image.Rect(0, 0, 1000, 700))}
	t1 := testTerm(a)
	t2 := &termSession{fontPx: 34}
	if a.face(t1).px != termFontDefault || a.face(t2).px != 34 || a.face(t2).cw <= a.face(t1).cw {
		t.Errorf("faces: %d %d, cells %d %d", a.face(t1).px, a.face(t2).px, a.face(t1).cw, a.face(t2).cw)
	}
	if a.face(&termSession{fontPx: 34}) != a.face(t2) {
		t.Error("a size's face isn't shared")
	}
	a.tv.fontPx = 20
	if t3 := (&termSession{}); a.face(t3).px != 20 || t3.fontPx != 20 || t1.fontPx != termFontDefault {
		t.Errorf("a new one: %d, the first still %d", t3.fontPx, t1.fontPx)
	}
	for _, c := range []struct {
		t    *termSession
		want string
	}{
		{&termSession{fontPx: 20, state: termConnected, kind: kindShell, alpine: true}, "20px/alpine"},
		{&termSession{fontPx: 30, state: termConnected, kind: kindSSH, target: sshHost{name: "box px/1"}}, "30px/ssh:box px/1"},
		{&termSession{}, "launcher"},
	} {
		if got := winWord(c.t); got != c.want {
			t.Errorf("winWord: %q, want %q", got, c.want)
		}
	}
}

// TestTermGlyphStyles: bold, italic and both are each a face of their own.
func TestTermGlyphStyles(t *testing.T) {
	f := newTermFace(termFontDefault)
	seen := map[string]uint8{}
	for _, st := range []uint8{0, glyphBold, glyphItalic, glyphBold | glyphItalic} {
		k := string(f.glyph("a", st, 1).Pix)
		if prev, ok := seen[k]; ok {
			t.Errorf("style %d draws as style %d", st, prev)
		}
		seen[k] = st
	}
}
