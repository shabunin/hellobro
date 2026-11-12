package main

// The new notebook's wizard: a plain notebook, or one with the AI diary
// on (notediary.go); for that, how it answers: a prompt that comes with
// it, one saved before, or one of the writer's own, typed or written by
// hand (the AI reads it to text first, to be looked at and put right).
// An own prompt can be saved for every notebook (aiprompts.go). It
// opens where "+ new notebook" was tapped (the list, the launcher), or
// as a window of its own (the switcher), and the notebook takes its
// place.
//
// The same steps are the saved prompts' window (Settings, AI notebooks,
// Your prompts): the list of them, each to read, change or forget, and
// a new one; no notebook made.

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"log"
	"os"
	"path"
	"strings"
	"time"

	"hellobro/ai"
	"hellobro/ink"
	"hellobro/ink/layout"
	"hellobro/ink/text"
	"hellobro/ink/theme"
	"hellobro/ink/widget"
	"hellobro/kobo"
)

const keyN = 49 // n, on the keyboard

const (
	wizKind    = iota // plain, or AI
	wizVoice          // which prompt
	wizOwn            // the writer's own
	wizKey     = "newnote"
	promptsKey = "prompts"
)

type noteWizUI struct {
	a       *bro
	dir     string // the folder it goes in
	step    int
	first   int    // the first step: wizVoice for the prompts' window
	manage  bool   // the prompts' window, not a new notebook's
	editing string // the saved prompt being changed, by name

	plain, withAI, back, create, own widget.Clickable
	cards, forget                    []widget.Clickable // the prompts': each, and the saved ones' ×
	pick                             int                // the card picked: diaryVoices', then the saved ones'
	saved                            []aiPrompt

	how            widget.Enum // "keys" or "hand"
	area           widget.TextArea
	sheet          widget.Canvas
	clear, read    widget.Clickable
	name           widget.Editor
	keep           widget.Bool
	reading, typed bool
	said           string // how the reading went

	list layout.Scroll
	do   func()
}

func newNoteWiz(a *bro) inkUI {
	u := &noteWizUI{a: a, dir: a.nt.wizDir}
	u.how.Value = "keys"
	u.keep.Value = true
	return u
}

// newPromptsUI is the saved prompts' window.
func newPromptsUI(a *bro) inkUI {
	u := &noteWizUI{a: a, manage: true, step: wizVoice, first: wizVoice, saved: loadPrompts()}
	u.how.Value = "keys"
	u.keep.Value = true
	return u
}

func (u *noteWizUI) Status() string { return "" }

func (u *noteWizUI) take() {
	if do := u.do; do != nil {
		u.do = nil
		do()
	}
}

// newNotebookWiz puts the wizard in from's place (nil: a window of its
// own), for a notebook in the folder dir.
func (a *bro) newNotebookWiz(from window, dir string) {
	if g := a.gui(wizKey); g != nil { // one at a time: that one
		if from != nil {
			a.closeWin(from)
		}
		a.openGUI(wizKey)
		return
	}
	a.nt.wizDir = dir
	if from != nil {
		a.replaceWin(from, &guiWin{key: wizKey})
		return
	}
	a.openGUI(wizKey)
}

// make makes the notebook, its diary on with p if ai, and puts it in
// the wizard's place.
func (u *noteWizUI) make(ai bool, p func(nb *notebook)) {
	a := u.a
	a.vkAsk(false)
	name := newNoteName(time.Now(), func(n string) bool {
		n = path.Join(u.dir, n)
		_, err := os.Stat(notePath(n))
		return err == nil || a.noteWindow(n) != nil
	})
	nw := a.noteOpen(path.Join(u.dir, name))
	g := a.gui(wizKey)
	if nw == nil || g == nil {
		return
	}
	if ai {
		nw.nb.setDiary(true)
		p(nw.nb)
		nw.changed = time.Now()
		if _, err := a.diaryConf(); err != nil {
			a.Notify("the diary needs ai.conf in the app's folder: " + diaryConfErr(err))
		}
		log.Printf("notebook %s: new, the AI's, as %s", nw.nb.name, nw.nb.voiceName())
	}
	a.replaceWin(g, nw)
}

// ownPrompt is the prompt typed (or read), named.
func (u *noteWizUI) ownPrompt() aiPrompt {
	t := strings.TrimSpace(u.area.Text())
	n := strings.TrimSpace(u.name.Text())
	if n == "" {
		n = promptName(t)
	}
	return aiPrompt{Name: n, Text: t}
}

// readHand sends what's written on the sheet to the AI, to be read to
// text: it's put in the text, to be looked at.
func (u *noteWizUI) readHand() {
	a := u.a
	m := u.sheet.Image()
	b := inkBox(m)
	if m == nil || b.Empty() {
		u.said = "write it on the sheet first"
		return
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, m.SubImage(b.Inset(-20).Intersect(m.Rect))); err != nil {
		u.said = err.Error()
		return
	}
	cfg, err := a.diaryConf()
	if err != nil {
		u.said = "ai.conf: " + diaryConfErr(err)
		return
	}
	cfg.Session = "hellobro-wizard"
	u.reading, u.said = true, "reading it…"
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		out, err := cfg.Ask(ctx, "Transcribe the handwriting in the picture exactly, in its own language, as plain text. Reply with the text only, nothing around it.",
			[]ai.Msg{{Role: "user", Parts: []ai.Part{{PNG: buf.Bytes()}, {Text: "The handwriting:"}}}})
		a.Do(func() {
			u.reading = false
			if err != nil {
				log.Printf("wizard: reading the prompt: %v", err)
				u.said = "couldn't read it: " + err.Error()
			} else {
				diaryCount(cfg.Model, out)
				t := strings.TrimSpace(u.area.Text())
				if t != "" {
					t += "\n"
				}
				u.area.SetText(t + strings.TrimSpace(out.Text))
				u.how.Value, u.said = "keys", "read: put it right if it needs it"
				log.Printf("wizard: read the prompt, %d characters", len(out.Text))
			}
			a.inkRedraw(u.winKey())
		})
	}()
}

// inkBox is where m isn't white.
func inkBox(m *image.Gray) image.Rectangle {
	if m == nil {
		return image.Rectangle{}
	}
	var r image.Rectangle
	for y := m.Rect.Min.Y; y < m.Rect.Max.Y; y++ {
		for x := m.Rect.Min.X; x < m.Rect.Max.X; x++ {
			if m.GrayAt(x, y).Y < 0xC0 {
				r = r.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return r
}

func (u *noteWizUI) Layout(gtx ink.Ctx) ink.Dims {
	th := u.a.theme()
	u.events(gtx)
	ink.Fill(gtx.Ops, image.Rectangle{Max: gtx.Constraints.Max}, th.Bg)
	var body ink.Widget
	switch u.step {
	case wizKind:
		body = u.kindStep
	case wizVoice:
		body = u.voiceStep
	default:
		body = u.ownStep
	}
	return layout.UniformInset(th.Pad).Layout(gtx, body)
}

// events does what was tapped.
func (u *noteWizUI) events(gtx ink.Ctx) {
	a := u.a
	if u.plain.Clicked(gtx) {
		u.do = func() { u.make(false, nil) }
	}
	if u.withAI.Clicked(gtx) {
		u.step, u.saved = wizVoice, loadPrompts()
	}
	if u.back.Clicked(gtx) && u.step > u.first {
		u.step--
		a.vkAsk(false)
	}
	if u.own.Clicked(gtx) {
		u.step, u.editing = wizOwn, ""
		u.area.SetText("")
		u.name.SetText("")
		u.sheet.Clear()
		u.said = ""
		a.vkAsk(true)
	}
	for i := range u.cards {
		if !u.cards[i].Clicked(gtx) {
			continue
		}
		u.pick = i
		if k := i - len(diaryVoices); u.manage && k >= 0 && k < len(u.saved) { // to change it
			p := u.saved[k]
			u.step, u.editing, u.said = wizOwn, p.Name, ""
			u.how.Value = "keys"
			u.area.SetText(p.Text)
			u.name.SetText(p.Name)
		}
	}
	for i := range u.forget {
		if u.forget[i].Clicked(gtx) && i < len(u.saved) {
			if err := forgetPrompt(u.saved[i].Name); err != nil {
				a.Notify("can't forget it: " + err.Error())
			}
			u.saved = loadPrompts()
			u.pick = 0
		}
	}
	if u.how.Update(gtx) {
		a.vkAsk(u.how.Value == "keys")
	}
	if u.clear.Clicked(gtx) {
		u.sheet.Clear()
	}
	if u.read.Clicked(gtx) && !u.reading {
		u.readHand()
	}
	u.keep.Update(gtx)
	if !u.create.Clicked(gtx) {
		return
	}
	switch {
	case u.step == wizVoice && u.pick < len(diaryVoices):
		id := diaryVoices[u.pick].id
		u.do = func() { u.make(true, func(nb *notebook) { nb.setVoice(id) }) }
	case u.step == wizVoice && u.pick-len(diaryVoices) < len(u.saved):
		p := u.saved[u.pick-len(diaryVoices)]
		u.do = func() { u.make(true, func(nb *notebook) { nb.setOwnVoice(p) }) }
	case u.step == wizOwn && u.manage:
		p := u.ownPrompt()
		if p.Text == "" {
			u.said = "write the prompt first: typed, or by hand and read"
			return
		}
		err := keepPrompt(p)
		if err == nil && u.editing != "" && u.editing != p.Name { // renamed
			err = forgetPrompt(u.editing)
		}
		if err != nil {
			u.said = "can't save it: " + err.Error()
			return
		}
		u.step, u.saved, u.editing = wizVoice, loadPrompts(), ""
		a.vkAsk(false)
		a.drawSettings()
	case u.step == wizOwn:
		p := u.ownPrompt()
		if p.Text == "" {
			u.said = "write the prompt first: typed, or by hand and read"
			return
		}
		if u.keep.Value {
			if err := keepPrompt(p); err != nil {
				a.Notify("can't save it for all: " + err.Error())
			}
		}
		u.do = func() { u.make(true, func(nb *notebook) { nb.setOwnVoice(p) }) }
	}
}

// key takes a key on the first step, where nothing's in focus: Enter
// or n a plain notebook, a an AI's; Esc (or Backspace) a step back.
func (u *noteWizUI) key(code uint16, value int32) bool {
	if value != 1 {
		return false
	}
	switch {
	case u.step == wizKind && (code == kobo.KeyEnter || code == keyN):
		u.plain.Click()
	case u.step == wizKind && code == kobo.KeyA:
		u.withAI.Click()
	case u.step > u.first && code == kobo.KeyEsc && !u.area.Focused() && !u.name.Focused():
		u.back.Click()
	default:
		return false
	}
	u.a.inkRedraw(u.winKey())
	return true
}

// winKey is the window it's in.
func (u *noteWizUI) winKey() string {
	if u.manage {
		return promptsKey
	}
	return wizKey
}

// head is a step's title, with ‹ back before it but on the first.
func (u *noteWizUI) head(gtx ink.Ctx, title string) ink.Dims {
	th := u.a.theme()
	kids := []layout.FlexChild{}
	if u.step > u.first {
		kids = append(kids, layout.Rigid(theme.Button(th, &u.back, "‹ back").Layout), layout.Rigid(layout.Spacer{Width: th.Pad}.Layout))
	}
	kids = append(kids, layout.Flexed(1, theme.Title(th, title).Layout))
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, kids...)
}

func (u *noteWizUI) kindStep(gtx ink.Ctx) ink.Dims {
	th := u.a.theme()
	where := "in the top folder"
	if u.dir != "" {
		where = "in " + u.dir
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx ink.Ctx) ink.Dims { return u.head(gtx, "A new notebook") }),
		layout.Rigid(theme.Small(th, where).Layout),
		layout.Rigid(layout.Spacer{Height: th.Pad * 2}.Layout),
		layout.Rigid(func(gtx ink.Ctx) ink.Dims {
			return u.card(gtx, &u.plain, "A notebook", "To write and draw in with the pen: pages, papers, pictures, text, links to other notebooks. "+
				"Nothing in it is sent anywhere.", false)
		}),
		layout.Rigid(func(gtx ink.Ctx) ink.Dims {
			return u.card(gtx, &u.withAI, "An AI notebook", "It reads what you write by hand and answers under it, in a hand or a font, once the pen rests "+
				"(or three taps). Next: how it should answer, a prompt that comes with it, one you saved, or your own, typed or written. "+
				"What you write in it goes to the model in ai.conf.", false)
		}),
		layout.Rigid(theme.Small(th, "Enter or n: a notebook · a: an AI notebook").Layout))
}

// card is a prompt to pick: its name, and what it says in full; the
// picked one's frame is thick.
func (u *noteWizUI) card(gtx ink.Ctx, clk *widget.Clickable, name, body string, picked bool) ink.Dims {
	th := u.a.theme()
	return layout.Inset{Bottom: th.Pad}.Layout(gtx, func(gtx ink.Ctx) ink.Dims {
		return clk.Layout(gtx, func(gtx ink.Ctx) ink.Dims {
			m := ink.Record(gtx.Ops)
			d := layout.UniformInset(th.Pad*1.5).Layout(gtx, func(gtx ink.Ctx) ink.Dims {
				title := theme.Body(th, name)
				title.Style = text.Bold
				if picked {
					title.Text = "● " + name
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(title.Layout),
					layout.Rigid(layout.Spacer{Height: th.Pad / 2}.Layout),
					layout.Rigid(theme.Small(th, body).Layout))
			})
			call := m.Stop()
			d.Size.X = gtx.Constraints.Max.X
			r := image.Rectangle{Max: d.Size}
			ink.Fill(gtx.Ops, r, th.Bg)
			call.Add(gtx.Ops)
			bw := max(gtx.Dp(1), 2)
			if picked {
				bw = gtx.Dp(4)
			}
			ink.Frame(gtx.Ops, r, bw, th.Fg)
			return d
		})
	})
}

func (u *noteWizUI) voiceStep(gtx ink.Ctx) ink.Dims {
	th := u.a.theme()
	n := len(diaryVoices) + len(u.saved)
	for len(u.cards) < n {
		u.cards = append(u.cards, widget.Clickable{})
	}
	for len(u.forget) < len(u.saved) {
		u.forget = append(u.forget, widget.Clickable{})
	}
	u.pick = min(u.pick, n-1)
	rows := []ink.Widget{}
	for i, v := range diaryVoices {
		if u.manage {
			break
		}
		rows = append(rows, func(gtx ink.Ctx) ink.Dims { return u.card(gtx, &u.cards[i], v.label, v.text, u.pick == i) })
	}
	switch {
	case len(u.saved) > 0 && !u.manage:
		rows = append(rows, theme.Section(th, "Yours, saved"))
	case len(u.saved) == 0 && u.manage:
		rows = append(rows, func(gtx ink.Ctx) ink.Dims {
			return layout.Inset{Bottom: th.Pad}.Layout(gtx, theme.Body(th, "none saved yet: write one here, or save one when you make an AI notebook").Layout)
		})
	}
	for i, p := range u.saved {
		k := len(diaryVoices) + i
		rows = append(rows, func(gtx ink.Ctx) ink.Dims {
			return layout.Flex{}.Layout(gtx,
				layout.Flexed(1, func(gtx ink.Ctx) ink.Dims { return u.card(gtx, &u.cards[k], p.Name, p.Text, u.pick == k && !u.manage) }),
				layout.Rigid(layout.Spacer{Width: th.Pad}.Layout),
				layout.Rigid(theme.Button(th, &u.forget[i], "forget").Layout))
		})
	}
	u.list.Axis = layout.Vertical
	list := func(gtx ink.Ctx) ink.Dims {
		return theme.List(th, &u.list).Layout(gtx, len(rows), func(gtx ink.Ctx, i int) ink.Dims { return rows[i](gtx) })
	}
	if u.manage {
		rows = append(rows, theme.Row(th, &u.own, "+ a new one").Layout)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx ink.Ctx) ink.Dims { return u.head(gtx, "Your AI prompts") }),
			layout.Rigid(theme.Small(th, "saved for every notebook: the AI voices after the ones that come with it; tap one to change it. "+
				"Forgotten, a notebook that has it keeps it").Layout),
			layout.Rigid(layout.Spacer{Height: th.Pad}.Layout),
			layout.Flexed(1, list))
	}
	rows = append(rows, theme.Row(th, &u.own, "+ your own prompt").Layout)
	make := theme.Button(th, &u.create, "make it")
	make.Strong = true
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx ink.Ctx) ink.Dims { return u.head(gtx, "How should it answer?") }),
		layout.Rigid(theme.Small(th, "what it's told, in full, before what you write; it's the notebook's own, to change in its settings").Layout),
		layout.Rigid(layout.Spacer{Height: th.Pad}.Layout),
		layout.Flexed(1, list),
		layout.Rigid(layout.Spacer{Height: th.Pad}.Layout),
		layout.Rigid(make.Layout))
}

func (u *noteWizUI) ownStep(gtx ink.Ctx) ink.Dims {
	th := u.a.theme()
	make := theme.Button(th, &u.create, "make it")
	if u.manage {
		make.Text = "keep it"
	}
	make.Strong = true
	keep := func(gtx ink.Ctx) ink.Dims {
		if u.manage { // it's what this window is for
			return ink.Dims{}
		}
		return theme.CheckBox(th, &u.keep, "save it for every notebook").Layout(gtx)
	}
	title := "Your own prompt"
	if u.editing != "" {
		title = u.editing
	}
	editor := func(gtx ink.Ctx) ink.Dims {
		if u.how.Value == "hand" {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Flexed(1, func(gtx ink.Ctx) ink.Dims {
					size := gtx.Constraints.Max
					u.sheet.Layout(gtx)
					ink.Frame(gtx.Ops, image.Rectangle{Max: size}, max(gtx.Dp(1), 2), th.Fg)
					return ink.Dims{Size: size}
				}),
				layout.Rigid(layout.Spacer{Height: th.Pad}.Layout),
				layout.Rigid(func(gtx ink.Ctx) ink.Dims {
					read := theme.Button(th, &u.read, "read it to text")
					read.Strong = !u.reading
					return layout.Flex{}.Layout(gtx,
						layout.Rigid(theme.Button(th, &u.clear, "clear").Layout),
						layout.Rigid(layout.Spacer{Width: th.Pad}.Layout),
						layout.Rigid(read.Layout))
				}))
		}
		ta := theme.TextArea(th, &u.area)
		ta.Hint, ta.Tools = "e.g. You are a chemistry teacher. Answer in Russian, step by step…", false
		return ta.Layout(gtx)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx ink.Ctx) ink.Dims { return u.head(gtx, title) }),
		layout.Rigid(theme.Small(th, "how it should answer: who it is, its manner, its language; typed, or written by hand and read to text by the AI").Layout),
		layout.Rigid(layout.Spacer{Height: th.Pad}.Layout),
		layout.Rigid(theme.Tabs(th, &u.how, []string{"keys", "hand"}, []string{"typed", "by hand"}).Layout),
		layout.Rigid(layout.Spacer{Height: th.Pad}.Layout),
		layout.Flexed(1, editor),
		layout.Rigid(func(gtx ink.Ctx) ink.Dims {
			if u.said == "" {
				return ink.Dims{}
			}
			return layout.Inset{Top: th.Pad / 2}.Layout(gtx, theme.Small(th, u.said).Layout)
		}),
		layout.Rigid(layout.Spacer{Height: th.Pad}.Layout),
		layout.Rigid(theme.Editor(th, &u.name, "its name (to find it again)").Layout),
		layout.Rigid(keep),
		layout.Rigid(layout.Spacer{Height: th.Pad}.Layout),
		layout.Rigid(make.Layout))
}
