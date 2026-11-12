package main

import (
	"image"
	"testing"

	"hellobro/ink/inktest"
	"hellobro/kobo"
)

// askOn is a question up, laid out in a test window rather than a pane.
func askOn(a *bro, title, text string, opts ...askOpt) (*askBox, *inktest.T) {
	q := &askBox{title: title, text: text, opts: opts}
	q.ui = newAskUI(a, q)
	q.over = newOver(q.ui.Layout, 1000)
	a.asking = q
	return q, inktest.New(image.Pt(900, 500), inktest.ElipsaMetric, q.ui.Layout)
}

// A question: Enter picks the answer the keys are on, the first, till ↓
// moves them; a letter picks its answer, a tap its button, Esc none. The
// pick waits for the frame to be done.
func TestAskUI(t *testing.T) {
	a := &bro{App: &kobo.App{}}
	var did string
	opts := []askOpt{
		{"open", "o", func() { did = "open" }},
		{"rename", "r", func() { did = "rename" }},
		{"cancel", "", nil},
	}
	q, it := askOn(a, "wish-server", "what to do with it", opts...)
	inkSave(t, it, "ask")
	for _, s := range []string{"wish-server", "what to do with it", "open  (Enter)", "rename  (r)", "cancel  (Esc)"} {
		if _, err := it.Find(s); err != nil {
			t.Fatal(err)
		}
	}
	if _, ok := q.ui.picked(); ok {
		t.Fatal("picked, unasked")
	}
	it.Key("Down", 0)
	if _, err := it.Find("rename  (Enter)"); err != nil {
		t.Error("↓:", err)
	}
	it.Key("Enter", 0)
	if i, ok := q.ui.picked(); !ok || i != 1 {
		t.Errorf("Enter on rename: %d %v", i, ok)
	}
	it.W.Text("o")
	it.Frame()
	if i, _ := q.ui.picked(); i != 0 {
		t.Errorf("o: %d", i)
	}
	it.Key("Esc", 0)
	if i, _ := q.ui.picked(); i != -1 {
		t.Errorf("Esc: %d", i)
	}
	must(t, it.TapText("cancel  (Esc)"))
	if i, _ := q.ui.picked(); i != 2 {
		t.Errorf("tapped cancel: %d", i)
	}
	if did != "" {
		t.Errorf("did %q in the frame", did)
	}
}

// askTyped: the yes holds till the words are typed; the box has the keys.
func TestAskTypedUI(t *testing.T) {
	a := &bro{App: &kobo.App{}}
	q, it := askOn(a, "Remove Alpine?", "all of it goes", askOpt{"remove", "", func() {}}, askOpt{"cancel", "", nil})
	q.want, q.ui.selAt = "remove", -1
	it.W.Invalidate()
	it.Frame()
	if _, err := it.Find("type: remove"); err != nil {
		t.Fatal(err)
	}
	must(t, it.TapText("remove"))
	if i, ok := q.ui.picked(); ok && q.ready() {
		t.Errorf("tapped remove, not typed: %d", i)
	}
	it.Type("remove")
	if !q.ready() {
		t.Fatalf("typed %q", q.typed)
	}
	inkSave(t, it, "ask-typed")
	it.Key("Enter", 0)
	if i, ok := q.ui.picked(); !ok || i != 0 {
		t.Errorf("Enter, typed: %d %v", i, ok)
	}
}
