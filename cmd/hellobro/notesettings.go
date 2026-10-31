package main

// A notebook's settings, over its window as the books' style panel is
// over theirs, with the same header: ‹ All Notebooks, Settings, back to
// the notebook. What a finger does, what export makes, the bar
// fullscreen. They're the notebooks' own: the app's Settings don't have
// them. The sliders on the bar open it (or :settings); Esc, q or "back
// to the notebook" goes back.

import (
	"image"

	"hellobro/kobo"
)

// noteSet is one of its rows: what it says, and a step on.
type noteSet struct {
	label string
	value func(a *bro) string
	step  func(a *bro, d int)
}

// stepOwn steps a notebook's setting kept with the app's (settings.go),
// d on.
func stepOwn(key string) func(a *bro, d int) {
	return func(a *bro, d int) { a.stepSetting(settingIndex(key), d) }
}

// ownShown is how setting key says its value.
func ownShown(key string) func(a *bro) string {
	return func(a *bro) string {
		st := settingList[settingIndex(key)]
		return st.shown[a.set[key]]
	}
}

var noteSettings = []noteSet{
	{"A finger", ownShown("note-finger"), stepOwn("note-finger")},
	{"Export", ownShown("note-export"), stepOwn("note-export")},
	{"Bar, fullscreen", func(a *bro) string { return fullBarWord(a.fullBar("note")) }, func(a *bro, _ int) { a.flipFullBar("note") }},
}

// settingsOn shows the settings, or the page again.
func (w *noteWin) settingsOn(a *bro, on bool) {
	w.endTyping(a)
	w.optsOf, w.settings, w.setSel = "", on, 0
	w.place(a, w.p) // the bar fullscreen may be another now
	w.draw(a)
}

// drawSettings draws them over the window's body: the header, then a
// row each, its value a button that steps it.
func (w *noteWin) drawSettings(a *bro) {
	c := a.C
	area := w.p.Body.Inset(kobo.Pad)
	c.Fill(w.p.Body, kobo.White)
	w.setRows = nil
	rowH := a.libRowH()
	h := a.drawLibHeader(image.Rect(area.Min.X, area.Min.Y, area.Max.X, area.Min.Y+rowH), "All Notebooks", false,
		func() { w.settings = false; a.notesFrom(w) }, "back to the notebook", func() { w.settingsOn(a, false) },
		[]libTab{{"Settings", true, func() {}}})
	w.setRows = append(w.setRows, h.rows...)
	if w.setSel == len(noteSettings) { // "back to the notebook", by keys
		c.Frame(h.back.Inset(-4), 3, kobo.Dark)
	}
	y, boxW := area.Min.Y+rowH+16, min(area.Dx()/2, 560)
	for i, s := range noteSettings {
		r := image.Rect(area.Min.X+6, y, area.Max.X-6, y+64)
		y += 72
		if i == w.setSel {
			c.Frame(r.Inset(-4), 3, kobo.Dark)
		}
		c.Text(a.F.Body, image.Rect(r.Min.X+12, r.Min.Y, r.Max.X-boxW, r.Max.Y), s.label, kobo.Black, kobo.AlignLeft)
		b := image.Rect(r.Max.X-boxW, r.Min.Y, r.Max.X, r.Max.Y)
		c.Frame(b, 2, kobo.Black)
		c.Text(a.F.Small, b.Inset(8), kobo.Fit(a.F.Small, s.value(a), b.Dx()-16), kobo.Black, kobo.AlignCenter)
		w.setRows = append(w.setRows, bookRow{b, func() { w.setSel = i; w.setStep(a, 1) }})
	}
	c.Text(a.F.Small, image.Rect(area.Min.X+18, y+8, area.Max.X, y+48), "the pen always writes; the paper is a tool on the bar, each notebook its own", kobo.Mid, kobo.AlignLeft)
}

// setStep steps the row in focus d on, and shows it.
func (w *noteWin) setStep(a *bro, d int) {
	if w.setSel == len(noteSettings) {
		w.settingsOn(a, false)
		return
	}
	noteSettings[w.setSel].step(a, d)
	w.drawSettings(a)
	a.Mark(w.p.Body, true)
}

// settingsTap takes a tap on them.
func (w *noteWin) settingsTap(a *bro, p image.Point) {
	for _, r := range w.setRows {
		if kobo.Hit(p, r.r) {
			r.do()
			return
		}
	}
}

// settingsKey: ↑ ↓ (j k, Tab) a row, ← → Enter Space a step, Esc q back.
func (w *noteWin) settingsKey(a *bro, code uint16, ch string) bool {
	n := len(noteSettings) + 1 // and "back to the notebook"
	switch {
	case code == keyUp || ch == "k":
		w.setSel = (w.setSel - 1 + n) % n
	case code == keyDown || ch == "j" || code == kobo.KeyTab:
		w.setSel = (w.setSel + 1) % n
	case code == keyLeft || ch == "h":
		w.setStep(a, -1)
		return true
	case code == keyRight || ch == "l" || code == kobo.KeyEnter || code == kobo.KeySpace:
		w.setStep(a, 1)
		return true
	case code == kobo.KeyEsc || ch == "q":
		w.settingsOn(a, false)
		return true
	default:
		return false
	}
	w.drawSettings(a)
	a.Mark(w.p.Body, true)
	return true
}
