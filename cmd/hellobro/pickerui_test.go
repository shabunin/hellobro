package main

import (
	"image"
	"testing"

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
	if n, ok := p.ui.picked(); !ok || n != 0 {
		t.Errorf("tapped Bluetooth: %d %v", n, ok)
	}
}
