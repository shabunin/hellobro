package main

import (
	"encoding/xml"
	"image"
	"strings"
	"testing"

	"hellobro/kobo"
)

// Each paper goes into a .xopp's background and comes back the same; a
// narrow one is marked, the rest not; Xournal++'s own styles read as
// the nearest.
func TestPaperXopp(t *testing.T) {
	for _, p := range strings.Fields(notePapers) {
		as := paperAttrs([]xml.Attr{{Name: xml.Name{Local: "type"}, Value: "solid"}, {Name: xml.Name{Local: "hellobro-paper"}, Value: "dots-narrow"}}, p)
		if got := xoppPaper(as); got != p {
			t.Errorf("%s: back as %q (%v)", p, got, as)
		}
		if narrow := strings.HasSuffix(p, "-narrow"); (attr(as, "hellobro-paper") != "") != narrow {
			t.Errorf("%s: marked %q", p, attr(as, "hellobro-paper"))
		}
	}
	for style, want := range map[string]string{"graph": "squares", "lined": "lines", "dotted": "dots", "staves": ""} {
		if got := xoppPaper([]xml.Attr{{Name: xml.Name{Local: "style"}, Value: style}}); got != want {
			t.Errorf("style %s: %q, want %q", style, got, want)
		}
	}
	// lines-narrow, then graph in Xournal++: squares, the mark left behind
	as := []xml.Attr{{Name: xml.Name{Local: "style"}, Value: "graph"}, {Name: xml.Name{Local: "hellobro-paper"}, Value: "lines-narrow"}}
	if got := xoppPaper(as); got != "squares" {
		t.Errorf("restyled: %q, want squares", got)
	}
}

// The papers drawn: lines across from the margin down, squares across
// and down, dots at the crossings only, nothing on plain; narrow ones
// closer.
func TestDrawPaper(t *testing.T) {
	size := image.Pt(400, 400)
	for _, c := range []struct {
		paper string
		inked []image.Point
		blank []image.Point
	}{
		{"plain", nil, []image.Point{{45, 90}, {200, 200}}},
		{"lines", []image.Point{{10, 90}, {200, 135}}, []image.Point{{45, 60}, {45, 100}}},
		{"lines-narrow", []image.Point{{10, 90}, {200, 120}}, []image.Point{{10, 105}}},
		{"squares", []image.Point{{45, 10}, {10, 135}}, []image.Point{{60, 60}}},
		{"squares-narrow", []image.Point{{30, 10}, {10, 120}}, []image.Point{{40, 100}}},
		{"dots", []image.Point{{45, 90}, {90, 135}}, []image.Point{{60, 90}, {45, 100}, {45, 45}}},
		{"dots-narrow", []image.Point{{30, 90}, {60, 120}}, []image.Point{{45, 90}, {30, 100}}},
	} {
		m := kobo.Blank(size)
		drawPaper(kobo.Canvas{Gray: m}, c.paper, size, image.Point{}, 1, m.Bounds(), kobo.Light)
		for _, p := range c.inked {
			if y := m.GrayAt(p.X, p.Y).Y; y == 0xFF {
				t.Errorf("%s: nothing at %v", c.paper, p)
			} else if y < kobo.Mid { // the paper's light: the ink's what's dark
				t.Errorf("%s: %#x at %v, darker than the paper's", c.paper, y, p)
			}
		}
		for _, p := range c.blank {
			if m.GrayAt(p.X, p.Y).Y != 0xFF {
				t.Errorf("%s: ink at %v", c.paper, p)
			}
		}
	}
}
