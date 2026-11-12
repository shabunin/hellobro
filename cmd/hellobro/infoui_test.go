package main

import (
	"fmt"
	"image"
	"testing"

	"hellobro/ink/inktest"
	"hellobro/kobo"
)

// infoOn is q up, laid out in a test window size big.
func infoOn(a *bro, q *infoBox, size image.Point) *inktest.T {
	q.ui = &infoUI{a: a, q: q}
	q.over = newOver(q.ui.Layout, 1100)
	a.info = q
	return inktest.New(size, inktest.ElipsaMetric, q.ui.Layout)
}

// Facts: names and values; c and its button copy, Esc and close close,
// both after the frame.
func TestInfoUI(t *testing.T) {
	a := &bro{App: &kobo.App{}}
	q := &infoBox{title: "Wi-Fi details", rows: [][2]string{{"Wi-Fi", "on · connected"}, {"network", "HomeNet"}, {"DNS", "192.168.2.1"}}}
	it := infoOn(a, q, image.Pt(1000, 700))
	inkSave(t, it, "info")
	for _, s := range []string{"Wi-Fi details", "network", "HomeNet", "copy  (c)", "close  (Esc)"} {
		if _, err := it.Find(s); err != nil {
			t.Fatal(err)
		}
	}
	it.W.Text("c")
	it.Frame()
	if q.ui.act != "copy" {
		t.Errorf("c: %q", q.ui.act)
	}
	it.Key("Esc", 0)
	if q.ui.act != "close" {
		t.Errorf("Esc: %q", q.ui.act)
	}
	q.ui.act = ""
	must(t, it.TapText("copy  (c)"))
	if q.ui.act != "copy" {
		t.Errorf("tapped copy: %q", q.ui.act)
	}
	if q.over.h <= 0 || q.over.h > 700 {
		t.Errorf("the card wants %d", q.over.h)
	}
}

// More facts than the screen holds: a list, the buttons still there.
func TestInfoUILong(t *testing.T) {
	a := &bro{App: &kobo.App{}}
	q := &infoBox{title: "many"}
	for i := range 60 {
		q.rows = append(q.rows, [2]string{fmt.Sprintf("fact %d", i), "its value"})
	}
	it := infoOn(a, q, image.Pt(1000, 700))
	inkSave(t, it, "info-long")
	if _, err := it.Find("close  (Esc)"); err != nil {
		t.Fatal(err)
	}
	if _, err := it.Find("fact 59"); err == nil {
		t.Error("all 60 show in 700 px")
	}
}

// A sheet: its rows in columns, the headings too.
func TestSheetUI(t *testing.T) {
	a := &bro{App: &kobo.App{}}
	q := &infoBox{title: "a book: keys and commands", rows: sheetRows(bookKeys(false), nil), sheet: true}
	it := infoOn(a, q, image.Pt(1800, 1200))
	inkSave(t, it, "sheet")
	for _, s := range []string{"Keys", "the next page", "close  (any key)"} {
		if _, err := it.Find(s); err != nil {
			t.Fatal(err)
		}
	}
	it.W.Text("x")
	it.Frame()
	if q.ui.act != "close" {
		t.Errorf("a letter: %q", q.ui.act)
	}
}
