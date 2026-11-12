package main

import (
	"image"
	"strings"
	"testing"
	"time"

	"hellobro/ai"
	"hellobro/ink/inktest"
	"hellobro/kobo"
	"hellobro/wm"
)

// TestModelsTable: the window's rows, searched, filtered and sorted.
func TestModelsTable(t *testing.T) {
	a := &bro{}
	a.nt.models = modelsState{loaded: true, List: []ai.Model{
		{ID: "cheap", Known: true, Images: true, In: 0.5, Out: 3, Context: 200000, Released: "2026-01-02", About: "quick at code"},
		{ID: "blind", Known: true, In: 0.1, Out: 0.2, Context: 1000000, Reasoning: true, Released: "2026-05-01"},
		{ID: "free-one", Known: true, Images: true, Tools: true, Released: "2025-12-01"},
		{ID: "dear", Known: true, Images: true, PDF: true, In: 15, Out: 75, Context: 1500000, Released: "2026-10-01"},
	}}
	u := newModelsUI(a).(*modelsUI)
	ids := func() string {
		var s []string
		for _, m := range u.shown {
			s = append(s, m.ID)
		}
		return strings.Join(s, " ")
	}
	u.filter()
	if got := ids(); got != "free-one cheap dear" { // pictures, by price in
		t.Errorf("at first: %s", got)
	}
	u.on[0] = false
	u.table.Sort, u.table.Desc = 3, true // by context, the most first
	u.filter()
	if got := ids(); got != "dear blind cheap free-one" {
		t.Errorf("by context: %s", got)
	}
	u.table.Sort, u.table.Desc = 5, false
	u.on[1] = true // PDF
	u.filter()
	if got := ids(); got != "dear" {
		t.Errorf("PDF: %s", got)
	}
	u.on[1] = false
	u.find.SetText("CODE")
	u.filter()
	if got := ids(); got != "cheap" {
		t.Errorf("searched: %s", got)
	}
	for n, w := range map[int]string{0: "", 999: "999", 200000: "200k", 1000000: "1M", 1500000: "1.5M", 1048576: "1M", 10500000: "10M"} {
		if tokens(n) != w {
			t.Errorf("tokens(%d) = %q, want %q", n, tokens(n), w)
		}
	}
	if dollars(-1) != "—" || dollars(0) != "free" || dollars(0.15) != "$0.15" {
		t.Error("dollars")
	}
}

// TestModelsUse: a row tapped, then "use it": the model's the setting's.
func TestModelsUse(t *testing.T) {
	i := settingIndex("ai-model")
	was := settingList[i]
	defer func() { settingList[i] = was }()
	a := &bro{App: &kobo.App{}, set: settings{}}
	a.wm.tree = wm.New(image.Rect(0, 0, 1000, 800), kobo.FoldH)
	a.nt.models = modelsState{loaded: true, V: modelsV, At: time.Now(), List: []ai.Model{
		{ID: "cheap", Known: true, Images: true, In: 0.5, Out: 3, Proto: "openai"},
		{ID: "dear", Known: true, Images: true, In: 15, Out: 75, Proto: "anthropic"},
	}}
	a.applyModels()
	it := inktest.New(image.Pt(1404, 1800), inktest.ElipsaMetric, newModelsUI(a).Layout)
	it.Frame()
	must(t, it.TapText("dear"))
	it.Frame()
	must(t, it.TapText("use it"))
	it.Frame()
	if got := a.setting("ai-model"); got != "dear" {
		t.Errorf("used: %q", got)
	}
}
