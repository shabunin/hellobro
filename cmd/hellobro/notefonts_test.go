package main

import "testing"

func TestNoteFontFallback(t *testing.T) {
	for _, name := range []string{"Go", "Go Mono Bold", "Gentium Book Italic", "Sans", "Nonesuch Bold"} {
		f := noteFontFace(name, 30)
		for _, r := range "aЖжλἀ∑∫≤" {
			if _, ok := f.GlyphAdvance(r); !ok {
				t.Errorf("%s: no %c", name, r)
			}
		}
	}
	for in, want := range map[string]string{
		"Gentium Book Bold Italic": "Gentium Book Bold Italic", "gentium": "Gentium Book",
		"Go Mono Italic": "Go Mono Italic", "Sans Bold": " Bold", "Go": "Go",
	} {
		if fam, st := splitFont(in); fontName(fam, st) != want {
			t.Errorf("%q: %q", in, fontName(fam, st))
		}
	}
}
