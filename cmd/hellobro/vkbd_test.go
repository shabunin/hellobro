package main

import (
	"image"
	"testing"
	"time"

	"hellobro/kobo"
)

// TestVkLayout: the keys are all there, on the page, none on another,
// none on what's left for the windows; the rows as wide as the page.
func TestVkLayout(t *testing.T) {
	for _, page := range []image.Rectangle{image.Rect(0, 46, 1404, 1872), image.Rect(0, 46, 1872, 1404)} {
		keys, rest := vkLayout(page)
		if len(keys) != 54+6+7 {
			t.Errorf("%v: %d keys", page, len(keys))
		}
		right := 0
		for i, k := range keys {
			if !k.r.In(page) || k.r.Overlaps(rest) || k.r.Dx() < 56 || k.r.Dy() < 56 {
				t.Errorf("%v: key %d at %v (left %v)", page, k.code, k.r, rest)
			}
			for _, o := range keys[i+1:] {
				if k.r.Overlaps(o.r) {
					t.Errorf("%v: keys %d and %d overlap", page, k.code, o.code)
				}
			}
			right = max(right, k.r.Max.X)
			if k.code == kobo.KeySpace && k.r.Max.Y != page.Max.Y {
				t.Errorf("%v: the space bar at %v", page, k.r)
			}
		}
		if right != page.Max.X || rest.Dx() < page.Dx()*4/5 || rest.Dy() < page.Dy()/2 {
			t.Errorf("%v: rows to %d, %v left", page, right, rest)
		}
	}
}

func TestVkWanted(t *testing.T) {
	vals := settingList[settingIndex("vkbd")].values
	for _, c := range []struct {
		set, kbd string
		ask      bool
		said     int
		want     bool
	}{
		{set: "fields", want: false},
		{set: "fields", ask: true, want: true},
		{set: "fields", ask: true, kbd: "NuPhy", want: false},
		{set: "none", ask: true, want: false},
		{set: "auto", want: true},
		{set: "auto", kbd: "NuPhy", want: false},
		{set: "on", kbd: "NuPhy", want: true},
		{set: "fields", said: 1, want: true}, // swiped up
		{set: "on", said: -1, want: false},   // swiped down
		{set: "none", said: 1, want: true},   // the menu's
		{set: "fields", ask: true, said: -1, want: false},
	} {
		a := &bro{App: &kobo.App{Keyboard: c.kbd}, set: settings{"vkbd": indexOf(vals, c.set)}}
		a.vk = vkbdState{ask: c.ask, said: c.said, since: time.Now().Add(-time.Minute)}
		if got := a.vkWanted(); got != c.want {
			t.Errorf("%+v: shows %v", c, got)
		}
	}
}
