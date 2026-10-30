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

// shrink3 is scaleGray's 3:1, fast: the same pixels, a part of a bigger
// image too, what's left over left out.
func TestShrink3(t *testing.T) {
	big := image.NewGray(image.Rect(0, 0, 40, 30))
	for i := range big.Pix {
		big.Pix[i] = uint8(i * 37 % 251)
	}
	for _, r := range []image.Rectangle{image.Rect(0, 0, 9, 6), image.Rect(5, 4, 35, 28), image.Rect(1, 2, 12, 10)} {
		src := big.SubImage(r).(*image.Gray)
		got := shrink3(src)
		w, h := r.Dx()/3, r.Dy()/3
		want := image.NewGray(image.Rect(0, 0, w, h))
		scaleGray(want, want.Rect, big.SubImage(image.Rect(r.Min.X, r.Min.Y, r.Min.X+w*3, r.Min.Y+h*3)).(*image.Gray))
		if got.Rect != want.Rect || string(got.Pix) != string(want.Pix) {
			t.Errorf("shrink3 of %v: %v %v, want %v %v", r, got.Rect, got.Pix, want.Rect, want.Pix)
		}
	}
}
