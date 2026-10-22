package wm

import (
	"image"
	"math"
	"testing"
)

func TestRender(t *testing.T) {
	r := image.Rect
	type place struct {
		win              string
		rect, deco, win_ image.Rectangle
	}
	for _, c := range []struct {
		in     string
		places []place
		hidden []string
	}{
		{"1:splith[a*, b]", []place{
			{"a", r(0, 0, 600, 900), r(0, 0, 600, 20), r(0, 20, 600, 900)},
			{"b", r(600, 0, 1200, 900), r(600, 0, 1200, 20), r(600, 20, 1200, 900)},
		}, nil},
		{"1:splith[a*, b, c]", []place{
			{"a", r(0, 0, 400, 900), r(0, 0, 400, 20), r(0, 20, 400, 900)},
			{"c", r(800, 0, 1200, 900), r(800, 0, 1200, 20), r(800, 20, 1200, 900)},
		}, nil},
		{"1:splitv[a*, tabbed[b, c*]]", []place{
			{"a", r(0, 0, 1200, 450), r(0, 0, 1200, 20), r(0, 20, 1200, 450)},
			{"b", r(0, 470, 1200, 900), r(0, 450, 600, 470), image.Rectangle{}},
			{"c", r(0, 470, 1200, 900), r(600, 450, 1200, 470), r(0, 470, 1200, 900)},
		}, []string{"b"}},
		{"1:splith[stacked[a, b*, c]]", []place{
			{"a", r(0, 60, 1200, 900), r(0, 0, 1200, 20), image.Rectangle{}},
			{"b", r(0, 60, 1200, 900), r(0, 20, 1200, 40), r(0, 60, 1200, 900)},
			{"c", r(0, 60, 1200, 900), r(0, 40, 1200, 60), image.Rectangle{}},
		}, []string{"a", "c"}},
		{"1:splith[a!, b*]", []place{
			{"a", r(0, 0, 1200, 900), image.Rectangle{}, r(0, 0, 1200, 900)},
		}, []string{"b"}},
		{"1:splith[a] float[f*]", []place{
			{"a", r(0, 0, 1200, 900), r(0, 0, 1200, 20), r(0, 20, 1200, 900)},
			{"f", r(100, 100, 500, 400), r(100, 100, 500, 120), r(100, 120, 500, 400)},
		}, nil},
	} {
		tr := build(t, c.in)
		tr.Render()
		for _, p := range c.places {
			n := w(tr, p.win)
			if n.Rect != p.rect || n.Deco != p.deco || n.WinRect != p.win_ {
				t.Errorf("%s: %s at %v %v %v, want %v %v %v", c.in, p.win, n.Rect, n.Deco, n.WinRect, p.rect, p.deco, p.win_)
			}
		}
		for _, h := range c.hidden {
			if w(tr, h).Visible {
				t.Errorf("%s: %s shows", c.in, h)
			}
		}
		shown := 0
		for _, n := range tr.Windows() {
			if n.Visible {
				shown++
			}
		}
		if len(tr.Shown()) != shown {
			t.Errorf("%s: Shown %d of %d", c.in, len(tr.Shown()), shown)
		}
	}
}

func TestSizes(t *testing.T) {
	ns := []*Node{{Percent: 1.0 / 3}, {Percent: 1.0 / 3}, {Percent: 1.0 / 3}}
	if got := sizes(ns, 1000); got[0]+got[1]+got[2] != 1000 || got[0] != 334 {
		t.Errorf("sizes %v", got)
	}
}

func TestResize(t *testing.T) {
	near := func(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
	tr := build(t, "1:splith[a*, b, c]")
	tr.Render()
	if err := tr.ResizeSize(tr.Focused(), true, Amount{Ppt: 10}); err != nil {
		t.Fatal(err)
	}
	if a, b := w(tr, "a").Percent, w(tr, "b").Percent; !near(a, 1.0/3+0.1) || !near(b, 1.0/3-0.05) {
		t.Errorf("grow width: %v %v", a, b)
	}
	if err := tr.ResizeSize(tr.Focused(), false, Amount{Ppt: 10}); err != errNothing {
		t.Errorf("grow height in a row: %v", err)
	}
	if err := tr.ResizeDir(w(tr, "c"), Right, Amount{Ppt: 10}); err != errNothing {
		t.Errorf("grow right at the edge: %v", err)
	}
	if err := tr.ResizeDir(w(tr, "c"), Left, Amount{Px: 120}); err != nil || !near(w(tr, "c").Percent, 1.0/3-0.05+0.1) {
		t.Errorf("grow left 120 px: %v %v", err, w(tr, "c").Percent)
	}
	if err := tr.ResizeSize(tr.Focused(), true, Amount{Ppt: -60}); err != errNoRoom {
		t.Errorf("shrink to nothing: %v", err)
	}

	// up through a split across
	tr = build(t, "1:splith[a, splitv[b*, c]]")
	tr.Render()
	tr.ResizeSize(tr.Focused(), true, Amount{Ppt: 20})
	if p := w(tr, "b").Parent.Percent; !near(p, 0.7) {
		t.Errorf("grow width in a column: %v", p)
	}
	tr.ResizeSet(tr.Focused(), Amount{Px: 300}, Amount{Ppt: 25})
	if p, b := w(tr, "b").Parent.Percent, w(tr, "b").Percent; !near(p, 0.25) || !near(b, 0.25) {
		t.Errorf("set 300 px by 25 ppt: %v %v", p, b)
	}

	// floating
	tr = build(t, "1:splith[a] float[f*]")
	tr.ResizeSize(tr.Focused(), true, Amount{Px: 50})
	tr.ResizeDir(tr.Focused(), Up, Amount{Px: 50})
	if got := tr.FloatRect(tr.Focused()); got != r(100, 50, 550, 400) {
		t.Errorf("floating resized to %v", got)
	}
	tr.MoveFloating(tr.Focused(), Left, 500)
	if got := tr.FloatRect(tr.Focused()); got != r(0, 50, 450, 400) {
		t.Errorf("floating moved off the screen to %v", got)
	}
}

func r(x0, y0, x1, y1 int) image.Rectangle { return image.Rect(x0, y0, x1, y1) }
