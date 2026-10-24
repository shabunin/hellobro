package main

import (
	"fmt"
	"image"
	"testing"
)

func TestReadKey(t *testing.T) {
	in := " \r\x7fq7\x1b[6~\x1b[5~\x1bOC\x1b[1~\x1b[4~\x1bx\x1b[2;5H\x1b"
	want := "[  1] [enter 1] [back 1] [q 1] [7 1] [pgdn 4] [pgup 4] [right 3] [home 4] [end 4] [ 2] [ 6] [esc 1]"
	var got []string
	for b := []byte(in); len(b) > 0; {
		k, n := readKey(b)
		got = append(got, fmt.Sprintf("[%s %d]", k, n))
		b = b[n:]
	}
	if s := fmt.Sprint(got); s != "["+want+"]" {
		t.Errorf("got %s\nwant [%s]", s, want)
	}
}

func TestReadPlaces(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if p, em := readPlace("/a.pdf"); p != 1 || em != readEm {
		t.Errorf("new: %d %d", p, em)
	}
	readKeep("/a.pdf", 12, 11)
	readKeep("/b b.epub", 3, 14)
	readKeep("/a.pdf", 13, 11)
	if p, em := readPlace("/a.pdf"); p != 13 || em != 11 {
		t.Errorf("a: %d %d", p, em)
	}
	if p, em := readPlace("/b b.epub"); p != 3 || em != 14 {
		t.Errorf("b: %d %d", p, em)
	}
}

// TestReadViews: Space goes down a zoomed page, then on to the next
// one's top; b back up, then to the one before's bottom; zooming keeps
// the window's middle where it was.
func TestReadViews(t *testing.T) {
	r := &reader{pages: 3, cols: 100, rows: 51, cell: image.Pt(10, 20)}
	r.px = image.Pt(1000, 1020) // the page's area: 1000×1000
	r.v = readView{page: 1, zoom: 2}
	r.d = &readDrawn{size: image.Pt(2000, 2600)}
	steps := []struct {
		f    func() readView
		want readView
	}{
		{r.forward, readView{1, 2, 0, 900}},
		{r.forward, readView{1, 2, 0, 1800}},
		{r.forward, readView{2, 2, 0, 0}}, // 1800+1000 ≥ 2600: the next page
		{r.backward, readView{1, 2, 0, 1 << 30}},
		{func() readView { return r.move(500, 0) }, readView{1, 2, 500, 1600}},
		{func() readView { return r.move(5000, 5000) }, readView{1, 2, 1000, 1600}},
		{func() readView { return r.zoomTo(3) }, readView{1, 3, 1750, 2650}}, // the middle, 1500,2100, at 2250,3150
		{func() readView { return r.zoomTo(1.01) }, readView{1, 1, 0, 0}},
	}
	for i, s := range steps {
		v := s.f()
		if v != s.want {
			t.Errorf("step %d: %+v, want %+v", i, v, s.want)
		}
		r.v = v
		if r.v.y > r.d.size.Y-1000 {
			r.v.y = r.d.size.Y - 1000 // as mutool keeps it in
		}
	}
}
