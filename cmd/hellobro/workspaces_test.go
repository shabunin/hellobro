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
		n, w, h  int
		area     image.Rectangle
		cols, tw int
	}{
		{2, 1000, 1000, sq, 2, 494},                           // side by side: one over another is smaller
		{4, 1000, 1000, sq, 2, 444},                           // two by two, the height holds them
		{1, 1000, 2000, sq, 1, 1000},                          // one: the whole width
		{4, 1832, 1302, image.Rect(0, 0, 1872, 1330), 2, 837}, // the Kobo in landscape
		{100, 1000, 1000, sq, 6, 156},                         // none fit: six across, scrolled
	} {
		cols, tw, th := wideTiles(c.n, c.w, c.h, c.area)
		if cols != c.cols || tw != c.tw || th != wsHeadH+tw*c.area.Dy()/c.area.Dx() {
			t.Errorf("wideTiles(%d, %d, %d) = %d, %d, %d; want %d, %d", c.n, c.w, c.h, cols, tw, th, c.cols, c.tw)
		}
	}
}
