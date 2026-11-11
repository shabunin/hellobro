package hand

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"testing"
)

func TestScriptLoads(t *testing.T) {
	h, err := Script()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range "AZaz09?!.,Жжщ" {
		if _, ok := h.glyph(r); !ok {
			t.Errorf("no glyph for %q", r)
		}
	}
	for r, want := range map[rune]rune{'é': 'e', 'Ü': 'U', 'ё': 'е', '—': '-', '’': '\''} {
		g, ok := h.glyph(r)
		w, _ := h.glyph(want)
		if !ok || len(g.strokes) != len(w.strokes) || g.right != w.right {
			t.Errorf("%q isn't written as %q", r, want)
		}
	}
	if _, ok := h.glyph('☃'); ok {
		t.Error("a snowman has a glyph")
	}
	// a capital is about the size asked for
	H, _ := h.glyph('H')
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, st := range H.strokes {
		for _, p := range st {
			lo, hi = math.Min(lo, p.Y), math.Max(hi, p.Y)
		}
	}
	if hi != 9 || hi-lo < 18 || hi-lo > 24 {
		t.Errorf("H from %v to %v: not a capital about 21 high on the baseline at 9", lo, hi)
	}
}

func TestWriteWraps(t *testing.T) {
	h, _ := Script()
	const size, width = 40.0, 600.0
	text := "Hello? Who are you, writing in my diary at this hour of the night?\n\nПривет, это я.\nA verylongwordthatcannotfitonalineatallnomatterwhat end.\nx^2 + e^(i pi) = 2^{10}, x^-1."
	wr := h.Write(text, size, width, 1)
	if len(wr.Words) < 15 {
		t.Fatalf("%d words", len(wr.Words))
	}
	lines := map[int]bool{}
	prevY := -1.0
	for i, w := range wr.Words {
		if w.Min.X < -size*0.3 || w.Max.X > width+size*0.3 {
			t.Errorf("word %d out of the width: %v–%v", i, w.Min.X, w.Max.X)
		}
		if w.Max.Y > wr.H+size*0.2 {
			t.Errorf("word %d below the writing's height %v: %v", i, wr.H, w.Max.Y)
		}
		mid := (w.Min.Y + w.Max.Y) / 2
		if mid < prevY-size { // in order: never back up a line
			t.Errorf("word %d is above the one before", i)
		}
		prevY = mid
		lines[int(mid/LineHeight(size))] = true
	}
	if len(lines) < 4 {
		t.Errorf("%d lines: it didn't wrap", len(lines))
	}
	if wr.H <= 0 || wr.H > 12*LineHeight(size) {
		t.Errorf("height %v", wr.H)
	}
	if os.Getenv("HAND_PNG") != "" { // to look at it
		m := image.NewGray(image.Rect(0, 0, int(width)+40, int(wr.H)+40))
		for i := range m.Pix {
			m.Pix[i] = 255
		}
		for _, w := range wr.Words {
			for _, l := range w.Strokes {
				for j := 1; j < len(l); j++ {
					a, b := l[j-1], l[j]
					n := int(math.Hypot(b.X-a.X, b.Y-a.Y)) + 1
					for s := 0; s <= n; s++ {
						x, y := a.X+(b.X-a.X)*float64(s)/float64(n), a.Y+(b.Y-a.Y)*float64(s)/float64(n)
						m.SetGray(int(x)+20, int(y)+20, color.Gray{0})
						m.SetGray(int(x)+21, int(y)+20, color.Gray{0})
						m.SetGray(int(x)+20, int(y)+21, color.Gray{0})
					}
				}
			}
		}
		f, _ := os.Create(os.Getenv("HAND_PNG"))
		png.Encode(f, m)
		f.Close()
	}
}

func TestSplit(t *testing.T) {
	h, _ := Script()
	parts := h.split("abcdefghijklmnopqrstuvwxyz", 100)
	if len(parts) < 2 {
		t.Fatalf("not split: %q", parts)
	}
	for _, p := range parts {
		if h.width(p) > 100 {
			t.Errorf("%q is %v wide", p, h.width(p))
		}
	}
}

func TestSuperscripts(t *testing.T) {
	h, _ := Script()
	for _, c := range []struct {
		in   string
		sups string // the letters raised, in order
	}{
		{"x^2", "2"},
		{"x^-1+y", "-1"},
		{"e^(i*pi)", "(i*pi)"},
		{"2^{10}", "10"},
		{"x^3/3", "3"},
		{"a^", ""},
		{"no", ""},
	} {
		var sups []glyph
		for _, l := range h.letters(c.in) {
			if l.k != 1 {
				if l.up <= 0 {
					t.Errorf("%q: a superscript not up", c.in)
				}
				sups = append(sups, l.g)
			}
		}
		var want []glyph
		for _, r := range c.sups {
			g, _ := h.glyph(r)
			want = append(want, g)
		}
		if len(sups) != len(want) {
			t.Errorf("%q: %d raised, want %q", c.in, len(sups), c.sups)
			continue
		}
		for i := range want {
			if sups[i].right != want[i].right || len(sups[i].strokes) != len(want[i].strokes) {
				t.Errorf("%q: raised letter %d isn't %q's", c.in, i, c.sups[i])
			}
		}
	}
	if n := len(h.letters("2^{10}")); n != 3 {
		t.Errorf("2^{10}: %d letters, the braces in", n)
	}
}
