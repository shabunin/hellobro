package main

// A notebook's settings, over its window as the books' style panel is
// over theirs, with the same header: ‹ All Notebooks, Settings, back to
// the notebook. What a finger does, what export makes, the bar
// fullscreen. They're the notebooks' own: the app's Settings don't have
// them. The sliders on the bar open it (or :settings); Esc, q or "back
// to the notebook" goes back.

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"slices"
	"strings"
	"time"

	"hellobro/ink"
	"hellobro/ink/inkkobo"
	"hellobro/ink/layout"
	"hellobro/ink/theme"
	"hellobro/ink/widget"
	"hellobro/kobo"
)

// noteSet is one of its rows: what it says; its choices (a dropdown),
// the one it is, and what picking one does. Without choices, a row that
// says its value, and does open when tapped, if it opens something.
type noteSet struct {
	label string
	opts  func(a *bro, w *noteWin) ([]string, int)
	pick  func(a *bro, w *noteWin, i int)
	value func(a *bro, w *noteWin) string
	open  func(a *bro, w *noteWin)
}

// own is a notebook's setting kept with the app's (settings.go).
func own(label, key string) noteSet {
	i := settingIndex(key)
	return noteSet{label: label,
		opts: func(a *bro, _ *noteWin) ([]string, int) { return settingList[i].shown, a.settingAt(i) },
		pick: func(a *bro, _ *noteWin, v int) { a.pickSetting(i, v) }}
}

// two is a setting of two values, on (the first) or not, flipped.
func two(label, yes, no string, on func(a *bro, w *noteWin) bool, flip func(a *bro, w *noteWin)) noteSet {
	return noteSet{label: label,
		opts: func(a *bro, w *noteWin) ([]string, int) {
			if on(a, w) {
				return []string{yes, no}, 0
			}
			return []string{yes, no}, 1
		},
		pick: func(a *bro, w *noteWin, v int) {
			if (v == 0) != on(a, w) {
				flip(a, w)
			}
		}}
}

// noteSettings are the rows; set at init, as their picks draw them.
var noteSettings []noteSet

func init() { noteSettings = noteSetRows() }

func noteSetRows() []noteSet {
	return []noteSet{
		own("A finger", "note-finger"),
		own("Pen buttons", "pen-buttons"),
		own("Export", "note-export"),
		own("Light by the left edge", "note-light"),
		two("Bar, fullscreen", fullBarWord(true), fullBarWord(false), func(a *bro, _ *noteWin) bool { return a.fullBar("note") },
			func(a *bro, _ *noteWin) { a.flipFullBar("note") }),
		two("Pages (this notebook)", "a roll: scrolled up and down", "one by one", func(_ *bro, w *noteWin) bool { return w.nb.rollOn() },
			func(a *bro, w *noteWin) { w.flipRoll(a) }),
		{label: "Page size (new pages)", opts: func(_ *bro, w *noteWin) ([]string, int) {
			var ss []string
			at := -1
			v := attr(w.nb.attrs, pageSizeAttr)
			for k, ps := range pageSizes {
				ss = append(ss, fmt.Sprintf("%s, %g × %g mm", ps.label, unitsMM(ps.w), unitsMM(ps.h)))
				if v == ps.id || v == "" && ps.id == "screen" {
					at = k
				}
			}
			if at < 0 { // its own, set by :pagesize
				ss, at = append(ss, w.nb.sizeShown()), len(ss)
			}
			return ss, at
		}, pick: func(a *bro, w *noteWin, i int) {
			if i < len(pageSizes) {
				w.setPageSize(a, pageSizes[i].id)
			}
		}},
		two("AI diary (this notebook)", "on", "off", func(_ *bro, w *noteWin) bool { return w.nb.diaryOn() },
			func(a *bro, w *noteWin) { w.flipDiary(a) }),
		{label: "AI voice (this notebook)", opts: func(_ *bro, w *noteWin) ([]string, int) {
			ls, at, _ := w.voiceChoices()
			return ls, at
		}, pick: func(_ *bro, w *noteWin, i int) {
			if _, _, pick := w.voiceChoices(); i < len(pick) {
				pick[i]()
				w.changed = time.Now()
			}
		}},
		{label: "AI prompt (this notebook)", value: func(_ *bro, w *noteWin) string { return w.nb.voiceText() }},
		{label: "AI instructions (written)", value: func(a *bro, w *noteWin) string {
			if _, ps := diaryTalk(w.nb); len(ps) > 0 {
				return strings.Join(ps, "; ")
			}
			return "none: write one, as “from now on, …”"
		}},
		{label: "AI model", opts: func(a *bro, _ *noteWin) ([]string, int) {
			a.modelsWant()
			i := settingIndex("ai-model")
			shown := slices.Clone(settingList[i].shown)
			shown[0] = "as ai.conf says: " + a.diaryConfModel()
			return shown, a.settingAt(i)
		}, pick: func(a *bro, _ *noteWin, v int) { a.pickSetting(settingIndex("ai-model"), v) }},
		{label: "AI font (this notebook)", value: func(a *bro, w *noteWin) string {
			if f := w.nb.diaryFont(); f != diaryHand {
				return f
			}
			return "handwriting (the Hershey script)"
		}, open: func(a *bro, w *noteWin) { w.chooseFont(a) }},
		own("AI answers", "ai-format"),
		own("AI letters' size", "ai-size"),
		own("AI asks after a pause", "ai-pause"),
		own("AI asks on taps", "ai-taps"),
		{label: "AI usage", value: func(a *bro, _ *noteWin) string { return a.usageShown() }},
	}
}

// settingsOn shows the settings, or the page again.
func (w *noteWin) settingsOn(a *bro, on bool) {
	w.endTyping(a)
	w.optsOf, w.settings = "", on
	if w.set != nil {
		w.set.list = layout.Scroll{}
	}
	w.place(a, w.p) // the bar fullscreen may be another now
	w.draw(a)
}

// noteSetUI is the settings in ink: the header (‹ All Notebooks,
// Settings, back to the notebook), a dropdown for each, rows for the
// rest, and what they're about at the foot.
type noteSetUI struct {
	a    *bro
	w    *noteWin
	pane *inkkobo.Pane
	head libHeadUI
	sel  []widget.Select
	act  []widget.Clickable
	list layout.Scroll
	do   func()

	marks               bool // the Marks tab: the parts marked, to link to (notelink.go)
	anc                 []noteAnchor
	only                widget.Bool // this notebook's marks only: on at first
	thumbs              map[int64]*image.Gray
	here, goto_, forget []widget.Clickable
}

// marksOn shows the marks, over the notebook.
func (w *noteWin) marksOn(a *bro) {
	u := w.setOf(a)
	u.marks, u.anc = true, loadAnchors()
	w.settingsOn(a, true)
}

// setOf is w's settings' ink, made the first time.
func (w *noteWin) setOf(a *bro) *noteSetUI {
	if w.set == nil {
		u := &noteSetUI{a: a, w: w, sel: make([]widget.Select, len(noteSettings)), act: make([]widget.Clickable, len(noteSettings)), only: widget.Bool{Value: true}}
		u.pane = inkkobo.NewPane(u.Layout)
		w.set = u
	}
	return w.set
}

func (u *noteSetUI) inkPane() *inkkobo.Pane { return u.pane }

func (u *noteSetUI) take() {
	if do := u.do; do != nil {
		u.do = nil
		do()
	}
}

// inkBodies are its settings' ink, or its bar's, whichever shows.
func (w *noteWin) inkBodies(a *bro) []inkBody {
	if b := w.setBody(a); b != nil {
		return []inkBody{b}
	}
	if w.barUI != nil && !w.settings && a.shows(w) && w.barUI.shows(w.bar) {
		return []inkBody{w.barUI}
	}
	return nil
}

// setBody is its settings' ink, if they show.
func (w *noteWin) setBody(a *bro) inkBody {
	if w.set != nil && w.settings && !w.p.Outer.Empty() && a.shows(w) && w.set.pane.At == w.p.Body {
		return w.set
	}
	return nil
}

// drawSettings draws them over the window's body, all of it.
func (w *noteWin) drawSettings(a *bro) {
	u := w.setOf(a)
	u.pane.Place(w.p.Body)
	u.pane.Win.Invalidate()
	u.pane.Draw(a.C.Gray)
}

// showSettings draws what changed of them.
func (w *noteWin) showSettings(a *bro) {
	if w.setBody(a) == nil || a.overlay() {
		w.drawSettings(a)
		a.Mark(w.p.Body, true)
		return
	}
	w.set.pane.Win.Invalidate()
	a.flushPane(w.set.pane)
	w.set.take()
}

func (u *noteSetUI) Layout(gtx ink.Ctx) ink.Dims {
	a, w, th := u.a, u.w, u.a.theme()
	hs := libHeadSpec{all: "All Notebooks", toAll: func() { w.settings = false; a.notesFrom(w) },
		back: "back to the notebook", toBack: func() { w.settingsOn(a, false) }, tabs: []libTab{
			{"Settings", !u.marks, func() { u.marks = false; u.list = layout.Scroll{}; w.showSettings(a) }},
			{"Marks", u.marks, func() { u.marks, u.anc = true, loadAnchors(); u.list = layout.Scroll{}; w.showSettings(a) }}}}
	if do := u.head.taps(gtx, hs); do != nil {
		u.do = do
	}
	if u.marks {
		return u.marksLayout(gtx, hs)
	}
	for i, s := range noteSettings {
		if s.opts == nil {
			if u.act[i].Clicked(gtx) && s.open != nil {
				u.do = func() { s.open(a, w) }
			}
			continue
		}
		opts, at := s.opts(a, w)
		sel := &u.sel[i]
		if sel.Changed(gtx, len(opts)) {
			v := sel.Value
			u.do = func() { s.pick(a, w, v); w.showSettings(a) }
		}
		if !sel.IsOpen() {
			sel.Value = at
		}
	}
	size := gtx.Constraints.Max
	ink.Fill(gtx.Ops, image.Rectangle{Max: size}, th.Bg)
	rowH := libRowHeight(gtx, th)
	rows := make([]ink.Widget, 0, len(noteSettings)+2)
	for i, s := range noteSettings {
		if s.opts != nil {
			opts, _ := s.opts(a, w)
			rows = append(rows, theme.Select(th, &u.sel[i], s.label, opts...).Layout)
			continue
		}
		r := theme.Row(th, &u.act[i], s.label)
		if s.open == nil {
			r.Button = nil
		}
		r.Sub, r.More = s.value(a, w), s.open != nil
		rows = append(rows, r.Layout)
	}
	for _, note := range []string{"the pen always writes; the paper is a tool on the bar, each notebook its own", a.usageNote()} {
		l := theme.Small(th, note)
		rows = append(rows, func(gtx ink.Ctx) ink.Dims { return layout.Inset{Top: th.Pad, Left: th.Pad}.Layout(gtx, l.Layout) })
	}
	return layout.UniformInset(th.Pad).Layout(gtx, func(gtx ink.Ctx) ink.Dims {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx ink.Ctx) ink.Dims { return u.head.layout(gtx, th, hs, rowH) }),
			layout.Rigid(layout.Spacer{Height: th.Pad}.Layout),
			layout.Flexed(1, func(gtx ink.Ctx) ink.Dims {
				u.list.Axis = layout.Vertical
				return theme.List(th, &u.list).Layout(gtx, len(rows), func(gtx ink.Ctx, i int) ink.Dims { return rows[i](gtx) })
			}))
	})
}

// settingsKey hands a key to them: Tab and the arrows (j k) go through
// them, Enter or Space opens a dropdown; Esc or q goes back to the
// notebook.
func (w *noteWin) settingsKey(a *bro, code uint16, ch string, value int32) bool {
	u := w.setOf(a)
	switch ch { // vi's, as the notebook has them
	case "j":
		code = keyDown
	case "k":
		code = keyUp
	}
	if u.pane.Key(code, value, inkMods(a.tv.mods), a.tv.mods.caps) {
		w.showSettings(a)
		return true
	}
	if value == 1 && (code == kobo.KeyEsc || ch == "q") {
		w.settingsOn(a, false)
		return true
	}
	return false
}

// marksLayout is the Marks tab: each part marked, the last first, its
// picture, what it says, where it is; a link to it put here, or gone to.
func (u *noteSetUI) marksLayout(gtx ink.Ctx, hs libHeadSpec) ink.Dims {
	a, w, th := u.a, u.w, u.a.theme()
	for len(u.here) < len(u.anc) {
		u.here, u.goto_, u.forget = append(u.here, widget.Clickable{}), append(u.goto_, widget.Clickable{}), append(u.forget, widget.Clickable{})
	}
	for i, an := range u.anc {
		switch {
		case u.here[i].Clicked(gtx):
			u.do = func() { w.settingsOn(a, false); w.linkHere(a, an) }
		case u.goto_[i].Clicked(gtx):
			u.do = func() { w.settingsOn(a, false); a.followLink(w, an, false) }
		case u.forget[i].Clicked(gtx):
			u.do = func() {
				if err := forgetAnchor(an.At); err != nil {
					a.Notify("can't forget it: " + err.Error())
				}
				u.anc = loadAnchors()
				w.showSettings(a)
			}
		}
	}
	if u.only.Update(gtx) {
		u.list = layout.Scroll{}
	}
	size := gtx.Constraints.Max
	ink.Fill(gtx.Ops, image.Rectangle{Max: size}, th.Bg)
	rowH := libRowHeight(gtx, th)
	rows := []ink.Widget{func(gtx ink.Ctx) ink.Dims {
		say := "mark a part of a page with the select tool (its ribbon), then put a link to it here: link here, and tap the page. " +
			"A finger's tap on a link goes there; held, in a window of its own"
		return layout.Inset{Left: th.Pad, Bottom: th.Pad}.Layout(gtx, theme.Small(th, say).Layout)
	}, func(gtx ink.Ctx) ink.Dims {
		return layout.Inset{Left: th.Pad, Bottom: th.Pad}.Layout(gtx, theme.CheckBox(th, &u.only, "only this notebook's").Layout)
	}}
	n := 0
	for i, an := range u.anc {
		if u.only.Value && an.Note != w.nb.name {
			continue
		}
		n++
		rows = append(rows, func(gtx ink.Ctx) ink.Dims { return u.markRow(gtx, i, an) })
	}
	if n == 0 {
		none := "nothing marked yet"
		if len(u.anc) > 0 {
			none = "nothing marked in this notebook: take the tick away for the others'"
		}
		rows = append(rows, func(gtx ink.Ctx) ink.Dims {
			return layout.Inset{Left: th.Pad, Top: th.Pad}.Layout(gtx, theme.Body(th, none).Layout)
		})
	}
	return layout.UniformInset(th.Pad).Layout(gtx, func(gtx ink.Ctx) ink.Dims {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx ink.Ctx) ink.Dims { return u.head.layout(gtx, th, hs, rowH) }),
			layout.Rigid(layout.Spacer{Height: th.Pad}.Layout),
			layout.Flexed(1, func(gtx ink.Ctx) ink.Dims {
				u.list.Axis = layout.Vertical
				return theme.List(th, &u.list).Layout(gtx, len(rows), func(gtx ink.Ctx, i int) ink.Dims { return rows[i](gtx) })
			}))
	})
}

// markRow is a mark: its picture at the left, what it says and where it
// is, its buttons under them; a line under it all.
func (u *noteSetUI) markRow(gtx ink.Ctx, i int, an noteAnchor) ink.Dims {
	th := u.a.theme()
	m := u.thumb(an)
	pic := func(gtx ink.Ctx) ink.Dims {
		if m == nil {
			return ink.Dims{}
		}
		sz := m.Bounds().Size()
		k := min(1, float64(gtx.Dp(180))/float64(sz.X), float64(gtx.Dp(70))/float64(sz.Y))
		r := image.Rectangle{Max: image.Pt(int(float64(sz.X)*k), int(float64(sz.Y)*k))}
		ink.Draw(gtx.Ops, r, func(dst *image.Gray, dr image.Rectangle) {
			for y := dr.Min.Y; y < dr.Max.Y; y++ {
				for x := dr.Min.X; x < dr.Max.X; x++ {
					sx, sy := m.Bounds().Min.X+int(float64(x-dr.Min.X)/k), m.Bounds().Min.Y+int(float64(y-dr.Min.Y)/k)
					dst.SetGray(x, y, m.GrayAt(sx, sy))
				}
			}
		})
		ink.Frame(gtx.Ops, r, 1, th.Quiet)
		return ink.Dims{Size: r.Size()}
	}
	title := an.Label
	if title == "" {
		title = "a part of " + an.where()
	}
	when := an.At.Format("Jan 2, 15:04")
	link := theme.Button(th, &u.here[i], "link here")
	link.Strong = true
	return layout.Inset{Top: th.Pad, Bottom: th.Pad}.Layout(gtx, func(gtx ink.Ctx) ink.Dims {
		d := layout.Flex{Alignment: layout.Start}.Layout(gtx,
			layout.Rigid(func(gtx ink.Ctx) ink.Dims { return layout.Inset{Left: th.Pad, Right: th.Pad * 1.5}.Layout(gtx, pic) }),
			layout.Flexed(1, func(gtx ink.Ctx) ink.Dims {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(theme.Body(th, title).Layout),
					layout.Rigid(theme.Small(th, an.where()+" · marked "+when).Layout),
					layout.Rigid(layout.Spacer{Height: th.Pad}.Layout),
					layout.Rigid(func(gtx ink.Ctx) ink.Dims {
						return layout.Flex{}.Layout(gtx,
							layout.Rigid(link.Layout), layout.Rigid(layout.Spacer{Width: th.Pad}.Layout),
							layout.Rigid(theme.Button(th, &u.goto_[i], "go to").Layout), layout.Rigid(layout.Spacer{Width: th.Pad}.Layout),
							layout.Rigid(theme.Button(th, &u.forget[i], "forget").Layout))
					}))
			}))
		ink.Fill(gtx.Ops, image.Rect(0, d.Size.Y+gtx.Dp(th.Pad)-1, d.Size.X, d.Size.Y+gtx.Dp(th.Pad)), th.Quiet)
		return d
	})
}

// thumb is an's picture, read once.
func (u *noteSetUI) thumb(an noteAnchor) *image.Gray {
	if m, ok := u.thumbs[an.At.UnixNano()]; ok {
		return m
	}
	if u.thumbs == nil {
		u.thumbs = map[int64]*image.Gray{}
	}
	var g *image.Gray
	if im, err := png.Decode(bytes.NewReader(an.Thumb)); err == nil {
		g = image.NewGray(im.Bounds())
		draw.Draw(g, g.Bounds(), im, im.Bounds().Min, draw.Src)
	}
	u.thumbs[an.At.UnixNano()] = g
	return g
}
