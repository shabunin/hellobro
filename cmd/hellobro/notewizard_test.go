package main

import (
	"image"
	"testing"

	"hellobro/ink/inktest"
	"hellobro/kobo"
)

// The wizard's steps: plain or AI; the prompts in full, a saved one
// among them; the own prompt's sheet and text.
func TestNoteWizard(t *testing.T) {
	if err := keepPrompt(aiPrompt{Name: "chemist", Text: "You are a chemistry teacher."}); err != nil {
		t.Fatal(err)
	}
	defer forgetPrompt("chemist") // AppFile: next to the test's binary
	a := &bro{App: &kobo.App{}}
	u := newNoteWiz(a).(*noteWizUI)
	it := inktest.New(image.Pt(900, 1200), inktest.ElipsaMetric, u.Layout)
	inkSave(t, it, "wizard-kind")
	for _, s := range []string{"A new notebook", "A notebook", "An AI notebook"} {
		if _, err := it.Find(s); err != nil {
			t.Fatal(err)
		}
	}
	must(t, it.TapText("An AI notebook"))
	if u.step != wizVoice || len(u.saved) != 1 {
		t.Fatalf("step %d, %d saved", u.step, len(u.saved))
	}
	it.W.Invalidate()
	it.Frame()
	inkSave(t, it, "wizard-voice")
	for _, s := range []string{"How should it answer?", "a tutor: step by step", "make it"} {
		if _, err := it.Find(s); err != nil {
			t.Fatal(err)
		}
	}
	must(t, it.TapText("a tutor: step by step"))
	if u.pick != 1 {
		t.Errorf("tapped the tutor: %d", u.pick)
	}
	u.step = wizOwn
	it.W.Invalidate()
	it.Frame()
	inkSave(t, it, "wizard-own")
	if _, err := it.Find("Your own prompt"); err != nil {
		t.Fatal(err)
	}
	u.area.SetText("You are a pirate. Answer in rhymes.")
	if p := u.ownPrompt(); p.Name != "You are a pirate.…" || p.Text == "" {
		t.Errorf("own prompt: %+v", p)
	}
	nb := &notebook{}
	nb.setOwnVoice(u.ownPrompt())
	if nb.diaryVoice() != -1 || nb.voiceText() != "You are a pirate. Answer in rhymes." {
		t.Errorf("own voice: %d %q", nb.diaryVoice(), nb.voiceText())
	}
	nb.setVoice("coder")
	if nb.voiceText() != diaryVoices[6].text {
		t.Errorf("coder: %q", nb.voiceText())
	}
}

// The saved prompts' window: the saved ones, one changed by a tap, a new
// one kept.
func TestPromptsUI(t *testing.T) {
	must(t, keepPrompt(aiPrompt{Name: "chemist", Text: "You are a chemistry teacher."}))
	defer forgetPrompt("chemist")
	defer forgetPrompt("pirate")
	a := &bro{App: &kobo.App{}, set: settings{"vkbd": 1}} // none: nothing to draw it on
	u := newPromptsUI(a).(*noteWizUI)
	it := inktest.New(image.Pt(900, 1200), inktest.ElipsaMetric, u.Layout)
	inkSave(t, it, "prompts")
	for _, s := range []string{"Your AI prompts", "chemist", "You are a chemistry teacher.", "+ a new one"} {
		if _, err := it.Find(s); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := it.Find("a tutor: step by step"); err == nil {
		t.Error("the ones that come with it, shown")
	}
	must(t, it.TapText("You are a chemistry teacher."))
	it.Frame()
	if u.step != wizOwn || u.editing != "chemist" || u.area.Text() != "You are a chemistry teacher." {
		t.Fatalf("to change it: step %d, %q, %q", u.step, u.editing, u.area.Text())
	}
	inkSave(t, it, "prompts-change")
	u.area.SetText("You are a chemistry teacher. Answer in Russian.")
	must(t, it.TapText("keep it"))
	it.Frame()
	if u.step != wizVoice {
		t.Fatalf("kept: step %d", u.step)
	}
	if ps := loadPrompts(); len(ps) != 1 || ps[0].Text != "You are a chemistry teacher. Answer in Russian." {
		t.Errorf("changed: %+v", ps)
	}
	must(t, it.TapText("+ a new one"))
	it.Frame()
	if u.area.Text() != "" || u.editing != "" {
		t.Errorf("a new one: %q %q", u.area.Text(), u.editing)
	}
	u.area.SetText("You are a pirate.")
	u.name.SetText("pirate")
	must(t, it.TapText("keep it"))
	it.Frame()
	if ps := loadPrompts(); len(ps) != 2 || ps[1].Name != "pirate" {
		t.Errorf("a new one kept: %+v", ps)
	}
}
