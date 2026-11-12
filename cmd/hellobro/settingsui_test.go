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
