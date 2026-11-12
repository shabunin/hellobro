package text

import (
	"strings"
	"testing"
)

func TestWrapFit(t *testing.T) {
	sh := NewShaper()
	f := sh.Face(Regular, 20)
	if sh.Face(Regular, 20) != f {
		t.Error("a face made twice")
	}
	w := Width(f, "the quick brown")
	lines := Wrap(f, "the quick brown fox jumps over the lazy dog", w)
	if lines[0] != "the quick brown" {
		t.Errorf("first line %q", lines[0])
	}
	for _, l := range lines {
		if Width(f, l) > w {
			t.Errorf("%q wider than %d", l, w)
		}
	}
	if got := Wrap(f, "a\n\nb", 1000); len(got) != 3 || got[1] != "" {
		t.Errorf("newlines: %q", got)
	}
	long := strings.Repeat("x", 80)
	for _, l := range Wrap(f, long, 100) {
		if Width(f, l) > 100 || l == "" {
			t.Errorf("a long word's piece %q", l)
		}
	}
	if s := Fit(f, "the quick brown fox", w); !strings.HasSuffix(s, "…") || Width(f, s) > w {
		t.Errorf("fit %q", s)
	}
	if s := Fit(f, "fox", 1000); s != "fox" {
		t.Errorf("fit %q", s)
	}
}
