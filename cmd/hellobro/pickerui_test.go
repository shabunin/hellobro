package main

import (
	"image"
	"testing"
	"time"

	"hellobro/ink/inktest"
	"hellobro/kobo"
)

// The switcher in ink: its rows numbered, the selection dark, what's
// typed in its line; a row tapped is picked after the frame.
func TestPickerUI(t *testing.T) {
	a := &bro{App: &kobo.App{}}
	p := &picker{title: "Go to", hint: "windows, workspaces, hosts, commands…", box: image.Rect(0, 0, 1000, 700)}
	for _, s := range []string{"Wi-Fi", "Bluetooth", "Sharing"} {
		p.all = append(p.all, pickEntry{kind: "window", title: s, detail: "settings"})
	}
	p.all[0].current = true
	a.picker = p
	a.filterPicker()
	p.ui = newPickerUI(a, p)
	it := inktest.New(p.box.Size(), inktest.ElipsaMetric, p.ui.Layout)
	inkSave(t, it, "picker")
	for _, s := range []string{"Go to", "› ", "1", "Wi-Fi  ●", "Bluetooth", "window"} {
		if _, err := it.Find(s); err != nil {
			t.Fatal(err)
		}
	}
	p.query = "blu"
	a.filterPicker()
	it.W.Invalidate()
	it.Frame()
	if _, err := it.Find("› blu"); err != nil {
		t.Error(err)
	}
	if len(p.shown) != 1 || p.shown[0].title != "Bluetooth" {
		t.Errorf("blu: %v", p.shown)
	}
	must(t, it.TapText("Bluetooth"))
	if p.ui.pick != 0 {
		t.Errorf("tapped Bluetooth: %d", p.ui.pick)
	}
}

// A menu of tiles: an icon's tiles and painted ones; a tap picks, a hold
// holds, a tile's × closes.
func TestPickerTiles(t *testing.T) {
	a := &bro{App: &kobo.App{}}
	p := &picker{title: "hello bro", hint: "type to narrow, or tap", tiles: 2, box: image.Rect(0, 0, 900, 700)}
	var did []string
	for _, s := range []string{"Rotate", "Wi-Fi", "Sleep"} {
		p.all = append(p.all, pickEntry{kind: "command", title: s, detail: "a word", icon: "rotate"})
	}
	p.all[1].on = true
	p.all[1].hold = func() { did = append(did, "held") }
	p.all = append(p.all, pickEntry{kind: "workspace", title: "Workspace 1",
		paint: func(c kobo.Canvas, r image.Rectangle) {
			c.Frame(r, 2, kobo.Black)
			c.Fill(image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+wsHeadH), kobo.Black)
		},
		close: func() { did = append(did, "closed") }})
	a.picker = p
	a.filterPicker()
	p.ui = newPickerUI(a, p)
	a.picker.over = newOver(p.ui.Layout, 900)
	it := inktest.New(p.box.Size(), inktest.ElipsaMetric, p.ui.Layout)
	inkSave(t, it, "pickertiles")
	for _, s := range []string{"hello bro", "Rotate", "Wi-Fi", "Sleep"} {
		if _, err := it.Find(s); err != nil {
			t.Fatal(err)
		}
	}
	must(t, it.TapText("Sleep"))
	if p.ui.pick != 2 {
		t.Errorf("tapped Sleep: %d", p.ui.pick)
	}
	p.ui.pick = -1
	at, _ := it.Find("Wi-Fi")
	it.Hold(at, 700*time.Millisecond)
	it.Frame()
	if p.ui.held != 1 || p.ui.pick != -1 {
		t.Errorf("held Wi-Fi: held %d, picked %d", p.ui.held, p.ui.pick)
	}
	p.ui.held = -1
	if len(p.ui.painted) != 1 {
		t.Errorf("painted tiles kept: %d", len(p.ui.painted))
	}
	// The × at the painted tile's header's right, by Sleep's.
	at, _ = it.Find("Sleep")
	it.Tap(image.Pt(p.box.Dx()-40, at.Y-10))
	it.Frame()
	if p.ui.closed != 3 || p.ui.pick != -1 {
		t.Errorf("tapped ×: closed %d, picked %d", p.ui.closed, p.ui.pick)
	}
}
