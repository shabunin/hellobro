package main

import (
	"image"
	"testing"

	"github.com/charmbracelet/x/vt"
)

func TestCellText(t *testing.T) {
	emu := vt.NewSafeEmulator(10, 4)
	emu.Write([]byte("hello\r\n0123456789abc\r\nend")) // the second line wraps: a 10-wide screen
	ts := &termSession{emu: emu}
	for _, c := range []struct {
		from, to image.Point
		want     string
	}{
		{image.Pt(0, 0), image.Pt(4, 0), "hello"},
		{image.Pt(1, 0), image.Pt(2, 0), "el"},
		{image.Pt(0, 0), image.Pt(9, 0), "hello"},         // the blanks after it: off
		{image.Pt(0, 1), image.Pt(2, 2), "0123456789abc"}, // wrapped: one line
		{image.Pt(3, 0), image.Pt(1, 1), "lo\n01"},        // across lines
		{image.Pt(0, 0), image.Pt(9, 3), "hello\n0123456789abc\nend"},
	} {
		if got := cellText(ts, 10, c.from, c.to); got != c.want {
			t.Errorf("%v-%v: %q, want %q", c.from, c.to, got, c.want)
		}
	}
}

func TestSelHas(t *testing.T) {
	s := &termSel{on: true, from: image.Pt(5, 2), to: image.Pt(2, 1)} // dragged backwards
	for _, c := range []struct {
		x, y int
		want bool
	}{{2, 1, true}, {1, 1, false}, {9, 1, true}, {0, 2, true}, {5, 2, true}, {6, 2, false}, {3, 0, false}, {3, 3, false}} {
		if got := s.has(c.x, c.y); got != c.want {
			t.Errorf("%d,%d: %v", c.x, c.y, got)
		}
	}
	if (&termSel{from: image.Pt(0, 0), to: image.Pt(3, 0)}).has(1, 0) {
		t.Error("not on yet: nothing selected")
	}
}

func TestClipSummary(t *testing.T) {
	for in, want := range map[string]string{
		"hi\n":                         "“hi”",
		"a\nb\nc\n":                    "3 lines",
		"ssh-ed25519 AAAAC3NzaC1lZDI1": "“ssh-ed25519 AAAAC3Nz…”",
	} {
		if got := clipSummary(in); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
}
