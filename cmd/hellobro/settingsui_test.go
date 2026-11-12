package main

import (
	"image"
	"testing"

	"hellobro/ink/inktest"
	"hellobro/kobo"
)

// Settings: a setting's dropdown shows its values; a pick is kept.
func TestSettingsUI(t *testing.T) {
	a := &bro{App: &kobo.App{}, set: settings{}}
	u := inkUIs["settings"](a)
	it := inktest.New(image.Pt(900, 1300), inktest.ElipsaMetric, u.Layout)
	inkSave(t, it, "settings")
	for _, s := range []string{"Light", "Light off when idle", "after 10 s untouched", "Power"} {
		if _, err := it.Find(s); err != nil {
			t.Fatal(err)
		}
	}
	i := settingIndex("light-start")
	if err := it.TapText("as you last set it"); err != nil {
		t.Fatal(err)
	}
	inkSave(t, it, "settings-open")
	if err := it.TapText("as Nickel had it"); err != nil {
		t.Fatal(err)
	}
	if a.set["light-start"] != 1 || a.settingAt(i) != 1 {
		t.Errorf("picked Nickel's: %v", a.set)
	}
	if _, err := it.Find("as Nickel had it"); err != nil {
		t.Error(err)
	}
}

// ↓ goes through every setting in a short window too, the list following,
// past the headings out of sight: it comes round to the first as often as
// in a window they all fit.
func TestSettingsUIKeys(t *testing.T) {
	round := func(size image.Point) int {
		a := &bro{App: &kobo.App{}, set: settings{}}
		it := inktest.New(size, inktest.ElipsaMetric, inkUIs["settings"](a).Layout)
		it.Key("Down", 0)
		first := it.W.Ring()
		for n := 1; n < 200; n++ {
			it.Key("Down", 0)
			if r := it.W.Ring(); r.Empty() || r.Max.Y > size.Y {
				t.Fatalf("%v: ↓ %d: the ring at %v", size, n, r)
			}
			if it.W.Ring() == first {
				return n
			}
		}
		t.Fatalf("%v: never back at the first", size)
		return 0
	}
	all, short := round(image.Pt(900, 6000)), round(image.Pt(1400, 500))
	if all < 10 || short != all {
		t.Errorf("round in %d with all showing, in %d in a short window", all, short)
	}
}

// In a narrow window a setting's name goes over its dropdown, whole.
func TestSettingsUINarrow(t *testing.T) {
	a := &bro{App: &kobo.App{}, set: settings{}}
	it := inktest.New(image.Pt(560, 1300), inktest.ElipsaMetric, inkUIs["settings"](a).Layout)
	inkSave(t, it, "settings-narrow")
	name, err := it.Find("Light off when idle")
	must(t, err)
	value, err := it.Find("after 10 s untouched")
	must(t, err)
	if value.Y <= name.Y {
		t.Errorf("the value at %v, its name at %v: not under it", value, name)
	}
}

// The sliders: their value said over the track, picked when the finger
// lifts; each setting with a help line under it; the notebooks' and the
// AI's settings in the app's too.
func TestSettingsUISliders(t *testing.T) {
	a := &bro{App: &kobo.App{}, set: settings{"text-size": 1, "ai-size": 5}}
	u := inkUIs["settings"](a)
	it := inktest.New(image.Pt(900, 7000), inktest.ElipsaMetric, u.Layout)
	inkSave(t, it, "settings-all")
	for _, s := range []string{"Text size", "as it comes, 100%", "how tall the strips, rows and buttons are", "Notebooks", "A finger in a notebook",
		"AI notebooks", "Letters of an answer", "15 pt", "Your prompts"} {
		if _, err := it.Find(s); err != nil {
			t.Fatal(err)
		}
	}
	p, err := it.Find("Letters of an answer")
	must(t, err)
	y := p.Y + 50 // the track, under its title
	it.Press(image.Pt(60, y))
	it.Move(image.Pt(880, y))
	it.Frame()
	if a.set["ai-size"] != 5 {
		t.Errorf("picked while held: %d", a.set["ai-size"])
	}
	it.Release(image.Pt(880, y))
	it.Frame()
	if a.setting("ai-size") != "32" {
		t.Errorf("dragged to the end: %q", a.setting("ai-size"))
	}
	if _, err := it.Find("32 pt"); err != nil {
		t.Error(err)
	}
	a.set["ai-size"] = 5
	a.set.save()

	a.set["text-size"] = 4
	a.applyTextSize()
	if ts := a.theme().TextSize; ts != 27 {
		t.Errorf("150%%: %v", ts)
	}
}
