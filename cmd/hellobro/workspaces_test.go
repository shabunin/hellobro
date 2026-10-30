package main

import (
	"image"
	"testing"
)

func TestFreeWorkspace(t *testing.T) {
	for _, c := range []struct {
		names []string
		want  string
	}{
		{nil, "1"},
		{[]string{"1"}, "2"},
		{[]string{"1", "2", "3"}, "4"},
		{[]string{"2", "3"}, "1"},
		{[]string{"1", "3"}, "2"},
		{[]string{"1", "mail", "2"}, "3"},
	} {
		if got := freeWorkspace(c.names); got != c.want {
			t.Errorf("freeWorkspace(%q) = %s, want %s", c.names, got, c.want)
		}
	}
}

func TestWideTiles(t *testing.T) {
	sq := image.Rect(0, 0, 100, 100)
	for _, c := range []struct {
		n, w, h   int
		area      image.Rectangle
		cols, thH int
	}{
		{2, 1000, 1000, sq, 2, wsHeadH + 494},   // one across is too high
		{4, 1000, 1000, sq, 3, wsHeadH + 325},   // two rows of two are too
		{1, 1000, 2000, sq, 1, wsHeadH + 1000},  // one: the whole width
		{100, 1000, 1000, sq, 6, wsHeadH + 156}, // none fit: the smallest
	} {
		if cols, th := wideTiles(c.n, c.w, c.h, c.area); cols != c.cols || th != c.thH {
			t.Errorf("wideTiles(%d, %d, %d) = %d, %d; want %d, %d", c.n, c.w, c.h, cols, th, c.cols, c.thH)
		}
	}
}
