package main

// The AI notebooks' prompts: how the diary answers. A few come with it
// (diaryVoices); the writer's own, typed or written by hand (the new
// notebook's wizard, notewizard.go), are a notebook's own (kept in its
// file, so it answers so on another Kobo too) and, if saved, every new
// notebook's to choose: ai-prompts.json in the app's folder.

import (
	"encoding/json"
	"encoding/xml"
	"os"
	"slices"
	"strings"
	"time"

	"hellobro/kobo"
)

// diaryVoices are the prompts that come with it, by name: what they
// say in full is the system prompt's start.
var diaryVoices = []struct{ id, label, text string }{
	{"diary", "a diary, as Tom Riddle's", "You are a diary that answers, like Tom Riddle's: personal, warm, a little mysterious. You remember what was written to you and come back to it; you ask the writer about themselves, their day and their plans, and answer as a confidant would, never as an assistant."},
	{"tutor", "a tutor: step by step", "You are a patient tutor. Given a problem, work it through step by step, saying why each step follows; given the writer's own working, check it line by line and point out where it goes wrong, without giving the whole answer away at once; ask a guiding question when it helps them learn."},
	{"helper", "a helper, plain", "You are a helpful assistant: plain, accurate and to the point. Answer what was asked; say so when you're not sure."},
	{"brief", "brief: a line at most", "Answer as briefly as you can: a word, a number, or one sentence. No greetings, no explanations unless asked for."},
	{"language", "a language partner", "You are a friendly partner for practising a language. Answer in the language the writer writes in, simply, and keep the talk going with a question. When they make mistakes, first show their sentence corrected, with each change in **bold**, then answer it."},
	{"translator", "a translator", "You translate what the writer writes: into the language they ask for; if they don't say, from English into Russian, and from anything else into English. Give the translation first, then, only where it matters, a short note on an idiom or a word with no exact match."},
	{"coder", "a programmer", "You are a senior programmer. Answer with working code where it helps, in a fenced block with its language, then a few lines on why it works and what to watch for. Prefer the standard library; say which version a feature needs."},
}

const (
	diaryVoiceAttr = "hellobro-diary-voice" // a voice's id, or "own"
	diaryOwnAttr   = "hellobro-diary-own"   // the own prompt's name
	diarySysAttr   = "hellobro-diary-prompt"
	ownVoice       = "own"
	promptsFile    = "ai-prompts.json"
)

// aiPrompt is one of the writer's own prompts, saved for every notebook.
type aiPrompt struct {
	Name string `json:"name"`
	Text string `json:"text"`
}

func loadPrompts() []aiPrompt {
	var ps []aiPrompt
	if b, err := os.ReadFile(kobo.AppFile(promptsFile)); err == nil {
		json.Unmarshal(b, &ps)
	}
	return ps
}

func savePrompts(ps []aiPrompt) error {
	b, err := json.MarshalIndent(ps, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(kobo.AppFile(promptsFile), append(b, '\n'))
}

// keepPrompt saves p for every notebook, in place of one of its name.
func keepPrompt(p aiPrompt) error {
	ps := loadPrompts()
	if i := slices.IndexFunc(ps, func(q aiPrompt) bool { return q.Name == p.Name }); i >= 0 {
		ps[i] = p // where it was
		return savePrompts(ps)
	}
	return savePrompts(append(ps, p))
}

// forgetPrompt takes the saved prompt name away (the notebooks that
// have it keep it).
func forgetPrompt(name string) error {
	return savePrompts(slices.DeleteFunc(loadPrompts(), func(q aiPrompt) bool { return q.Name == name }))
}

// promptName is a name for a prompt with none: its first words.
func promptName(text string) string {
	ws := strings.Fields(text)
	n := strings.Join(ws[:min(4, len(ws))], " ")
	if len(ws) > 4 {
		n += "…"
	}
	return n
}

// diaryVoice is nb's voice among diaryVoices, as its file keeps it; -1
// if it's its own.
func (nb *notebook) diaryVoice() int {
	v := attr(nb.attrs, diaryVoiceAttr)
	if v == ownVoice && attr(nb.attrs, diarySysAttr) != "" {
		return -1
	}
	for i, dv := range diaryVoices {
		if dv.id == v {
			return i
		}
	}
	return 0
}

// voiceText is the prompt nb's diary answers by.
func (nb *notebook) voiceText() string {
	if i := nb.diaryVoice(); i >= 0 {
		return diaryVoices[i].text
	}
	return attr(nb.attrs, diarySysAttr)
}

// voiceName is what it's called.
func (nb *notebook) voiceName() string {
	if i := nb.diaryVoice(); i >= 0 {
		return diaryVoices[i].label
	}
	return "yours: " + attr(nb.attrs, diaryOwnAttr)
}

// putAttr puts name=value in as, in place of what it was ("": none).
func putAttr(as []xml.Attr, name, value string) []xml.Attr {
	as = slices.DeleteFunc(as, func(a xml.Attr) bool { return a.Name.Local == name })
	if value != "" {
		as = append(as, xml.Attr{Name: xml.Name{Local: name}, Value: value})
	}
	return as
}

// setVoice makes nb answer as diaryVoices' id does.
func (nb *notebook) setVoice(id string) {
	nb.attrs = putAttr(nb.attrs, diaryVoiceAttr, id)
	nb.attrs = putAttr(nb.attrs, diarySysAttr, "")
	nb.attrs = putAttr(nb.attrs, diaryOwnAttr, "")
}

// setOwnVoice makes nb answer by a prompt of the writer's.
func (nb *notebook) setOwnVoice(p aiPrompt) {
	nb.attrs = putAttr(nb.attrs, diaryVoiceAttr, ownVoice)
	nb.attrs = putAttr(nb.attrs, diarySysAttr, p.Text)
	nb.attrs = putAttr(nb.attrs, diaryOwnAttr, p.Name)
}

// stepVoice turns to the notebook's next voice that comes with it, d on.
func (w *noteWin) stepVoice(d int) {
	n := len(diaryVoices)
	i := (max(0, w.nb.diaryVoice()) + d + n) % n
	w.nb.setVoice(diaryVoices[i].id)
	w.changed = time.Now()
}

// voiceChoices are the voices to choose for w's notebook: those that
// come with it, the saved ones, and its own if it's none of those; and
// what picking each does.
func (w *noteWin) voiceChoices() (labels []string, at int, pick []func()) {
	at = -1
	for i, v := range diaryVoices {
		labels = append(labels, v.label)
		pick = append(pick, func() { w.nb.setVoice(diaryVoices[i].id) })
		if w.nb.diaryVoice() == i {
			at = i
		}
	}
	own := w.nb.diaryVoice() < 0
	for _, p := range loadPrompts() {
		labels = append(labels, "yours: "+p.Name)
		pick = append(pick, func() { w.nb.setOwnVoice(p) })
		if own && p.Text == w.nb.voiceText() {
			at = len(labels) - 1
		}
	}
	if at < 0 { // its own, not saved
		labels = append(labels, w.nb.voiceName())
		pick = append(pick, func() {})
		at = len(labels) - 1
	}
	return
}
