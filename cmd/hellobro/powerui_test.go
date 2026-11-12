package main

import (
	"image"
	"os"
	"path/filepath"
	"testing"
	"time"

	"hellobro/ink/inktest"
	"hellobro/kobo"
)

// inkSave writes what shows to $INK_OUT/name.png, if INK_OUT is set.
func inkSave(t *testing.T, it *inktest.T, name string) {
	if dir := os.Getenv("INK_OUT"); dir != "" {
		if err := it.Save(filepath.Join(dir, name+".png")); err != nil {
			t.Fatal(err)
		}
	}
}

// The Battery window says the sample, the busy processes, the graph; a
// new sample changes only what it changes.
func TestBatteryUI(t *testing.T) {
	a := &bro{App: &kobo.App{}}
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	ps := &a.pw
	ps.at = now
	ps.bat = battery{ok: true, pct: 57, ma: 271, mv: 3800, temp: 241, status: "Discharging", health: "Good", toEmpty: 9 * time.Hour}
	for i := 0; i < 600; i += 2 {
		ps.hist = append(ps.hist, batSample{at: now.Add(-time.Duration(600-i) * time.Minute), pct: 57 + (600-i)/40, ma: 270})
	}
	ps.use = usage{ok: true, busy: 0.12, procs: []ranked{{"hellobro", 8.5}, {"wpa_supplicant", 1.2}, {"kworker/0:1", 0.6}, {"idle", 0.1}}}
	u := &batteryUI{a: a}
	it := inktest.New(image.Pt(900, 900), inktest.ElipsaMetric, u.Layout)
	inkSave(t, it, "battery")
	for _, s := range []string{"57% · on battery", "hellobro", "8.5%", "kworker/0:1", "The last 12 hours", "now"} {
		if _, err := it.Find(s); err != nil {
			t.Error(err)
		}
	}
	if _, err := it.Find("idle"); err == nil {
		t.Error("a process under 0.5% shows")
	}
	if u.Status() != "57%" {
		t.Errorf("status %q", u.Status())
	}
	ps.bat.ma = 300
	it.W.Invalidate()
	it.Frame()
	if len(it.Updates) == 0 || len(it.Updates) > 2 {
		t.Errorf("a new current: %v", it.Updates)
	}
	for _, up := range it.Updates {
		if up.R.Dy() > 120 {
			t.Errorf("a new current redrew %v", up.R)
		}
	}
}

func TestUsageUI(t *testing.T) {
	a := &bro{App: &kobo.App{}}
	a.pw.use = usage{ok: true, busy: 0.3, cores: "0-1", load: "0.42", freqs: []ranked{{"1.0 GHz", 0.7}, {"600 MHz", 0.3}},
		cstates: []ranked{{"WFI", 0.6}}, wakeups: 120, irqs: []ranked{{"timer", 80}}, procs: []ranked{{"hellobro", 20}}}
	u := &usageUI{a: a}
	it := inktest.New(image.Pt(900, 900), inktest.ElipsaMetric, u.Layout)
	inkSave(t, it, "usage")
	for _, s := range []string{"CPU 30% busy", "cores 0-1 on · load 0.42", "Wakeups, 120 a second", "hellobro"} {
		if _, err := it.Find(s); err != nil {
			t.Error(err)
		}
	}
}
