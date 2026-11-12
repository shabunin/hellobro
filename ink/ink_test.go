package ink

import (
	"image"
	"testing"
	"time"
)

// A change is found where it is, gray or not, and far-apart changes stay
// apart.
func TestDiff(t *testing.T) {
	a := image.NewGray(image.Rect(0, 0, 400, 300))
	b := image.NewGray(a.Rect)
	fill(a, a.Rect, White)
	fill(b, b.Rect, White)
	fill(b, image.Rect(10, 10, 20, 20), Black)
	fill(b, image.Rect(300, 250, 310, 260), Mid)
	us := mergeUpdates(diff(a, b))
	if len(us) != 2 {
		t.Fatalf("%v", us)
	}
	for _, u := range us {
		switch {
		case u.R.Overlaps(image.Rect(10, 10, 20, 20)):
			if u.Gray {
				t.Error("black on white called gray")
			}
		case u.R.Overlaps(image.Rect(300, 250, 310, 260)):
			if !u.Gray {
				t.Error("mid gray not called gray")
			}
		default:
			t.Errorf("%v: nothing changed there", u.R)
		}
	}
	if us := diff(a, a); len(us) != 0 {
		t.Errorf("no change: %v", us)
	}
}

// Taps go to the topmost area under them, watchers hear of them too,
// and a backdrop stops them.
func TestRouting(t *testing.T) {
	var top, under, watch byte
	backdrop := false
	ui := func(gtx Ctx) Dims {
		Input(gtx.Ops, Area{Rect: image.Rect(0, 0, 100, 100), Tag: &watch, Watch: true})
		Input(gtx.Ops, Area{Rect: image.Rect(0, 0, 100, 100), Tag: &under, Focus: true})
		st := Offset(gtx.Ops, image.Pt(10, 10))
		Input(gtx.Ops, Area{Rect: image.Rect(0, 0, 20, 20), Tag: &top, Focus: true})
		st.Pop()
		if backdrop {
			Input(gtx.Ops, Area{Rect: image.Rect(0, 0, 100, 100)})
		}
		return Dims{Size: gtx.Constraints.Max}
	}
	w := NewWindow(image.Pt(100, 100), Metric{1}, ui)
	now := time.Now()
	w.Frame(now)
	w.Pointer(Pointer{Kind: Press, Pos: image.Pt(15, 15)})
	ctx := Ctx{w: w}
	evs := ctx.Events(&top)
	if len(evs) != 2 { // the press, the focus
		t.Fatalf("top got %v", evs)
	}
	for _, e := range evs {
		if p, ok := e.(Pointer); ok && p.Pos != image.Pt(5, 5) {
			t.Errorf("at %v, want its own 5,5", p.Pos)
		}
	}
	if len(ctx.Events(&under)) != 0 || len(ctx.Events(&watch)) != 1 {
		t.Error("under heard, or the watcher didn't")
	}
	if !ctx.Focused(&top) {
		t.Error("a tap didn't focus it")
	}
	if !w.Key(Key{Name: "Tab"}) || !ctx.Focused(&under) {
		t.Error("Tab didn't move on")
	}
	backdrop = true
	w.Invalidate()
	w.Frame(now)
	w.Pointer(Pointer{Kind: Release, Pos: image.Pt(15, 15)})
	w.Pointer(Pointer{Kind: Press, ID: 1, Pos: image.Pt(15, 15)})
	ctx.Events(&top)
	ctx.Events(&watch)
	w.Pointer(Pointer{Kind: Release, ID: 1, Pos: image.Pt(15, 15)})
	if len(ctx.Events(&top)) != 0 || len(ctx.Events(&watch)) != 0 {
		t.Error("heard through the backdrop")
	}
}

func TestFlexSizes(t *testing.T) {
	// Without the layout package: a Rigid's size, two Flexed's shares, as
	// that package's test would, through Record and Offset.
	o := &Ops{}
	m := Record(o)
	Fill(o, image.Rect(0, 0, 10, 10), Black)
	c := m.Stop()
	if len(o.list) != 0 || len(c.ops) != 1 {
		t.Fatalf("recorded %d, left %d", len(c.ops), len(o.list))
	}
	st := Offset(o, image.Pt(5, 5))
	c.Add(o)
	st.Pop()
	img := image.NewGray(image.Rect(0, 0, 20, 20))
	fill(img, img.Rect, White)
	p := painter{dst: img}
	p.run(o.list, image.Point{}, img.Rect)
	if img.GrayAt(4, 4).Y != White || img.GrayAt(5, 5).Y != Black || img.GrayAt(14, 14).Y != Black || img.GrayAt(15, 15).Y != White {
		t.Error("the recording isn't where it was put")
	}
}
