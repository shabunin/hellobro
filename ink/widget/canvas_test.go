package widget_test

import (
	"image"
	"testing"

	"hellobro/ink"
	"hellobro/ink/inktest"
	"hellobro/ink/layout"
	"hellobro/ink/widget"
)

// The sheet's contacts are drawn as they come, into the window, with no
// frame: the line follows the pen. Its frame comes once it's lifted, a
// moment after.
func TestCanvasLive(t *testing.T) {
	c := &widget.Canvas{FingerDraws: true}
	frames := 0
	it := inktest.New(image.Pt(400, 300), ink.Metric{PxPerDp: 1}, func(gtx ink.Ctx) ink.Dims {
		frames++
		return layout.Inset{Top: 100}.Layout(gtx, c.Layout)
	})
	it.W.Pointer(ink.Pointer{Kind: ink.Press, ID: 1, Pos: image.Pt(50, 150), Time: it.Now})
	if !it.W.Inking() {
		t.Fatal("the sheet didn't take the contact live")
	}
	it.W.Pointer(ink.Pointer{Kind: ink.Move, ID: 1, Pos: image.Pt(200, 150), Time: it.Now})
	before := frames
	us := it.Frame()
	if frames != before {
		t.Error("a move laid the window out")
	}
	if len(us) != 2 || !us[1].Pen || !image.Pt(120, 150).In(us[1].R) { // the dot, the line
		t.Errorf("the line's updates: %+v", us)
	}
	if it.W.Image().GrayAt(120, 150).Y != ink.Black {
		t.Error("the line isn't in the window")
	}
	if c.Strokes() != 1 {
		t.Errorf("%d strokes", c.Strokes())
	}
	it.W.Pointer(ink.Pointer{Kind: ink.Release, ID: 1, Pos: image.Pt(200, 150), Time: it.Now})
	if it.W.Inking() || it.W.Wake().IsZero() {
		t.Error("lifted: no frame asked for")
	}
	it.Wait(ink.LiveRest + 100e6)
	if frames == before {
		t.Error("no frame after it rested")
	}
	if !c.Changed() || it.W.Image().GrayAt(120, 150).Y != ink.Black {
		t.Error("the frame lost the line, or the change")
	}
	// Outside the sheet a contact goes the usual way.
	it.W.Pointer(ink.Pointer{Kind: ink.Press, ID: 2, Pos: image.Pt(50, 50), Time: it.Now})
	if it.W.Inking() {
		t.Error("over the sheet: live")
	}
}
