package widget

import (
	"image"
	"unicode/utf8"

	"golang.org/x/image/font"

	"hellobro/ink"
	"hellobro/ink/text"
)

// Editor is a line of text to type into: it takes the keys in focus (a
// tap gives it the focus, the caret where it tapped), ← → Home End move
// the caret, Back and Delete take a letter out, Enter submits.
type Editor struct {
	Mask rune // shown for each letter (a password), 0: the letters

	runes     []rune
	caret     int
	scroll    int // pixels of the line scrolled off its left
	submitted bool
	changed   bool
	focused   bool
	face      font.Face
	tapped    bool
	tapAt     int // the tap's x, to place the caret at
}

// Text is what's typed.
func (e *Editor) Text() string { return string(e.runes) }

// SetText replaces it, the caret at its end.
func (e *Editor) SetText(s string) {
	e.runes = []rune(s)
	e.caret = len(e.runes)
}

// Len is how many letters it has.
func (e *Editor) Len() int { return len(e.runes) }

// Caret is where the caret is, in letters.
func (e *Editor) Caret() int { return e.caret }

// Focused says it has the keys.
func (e *Editor) Focused() bool { return e.focused }

// Submitted says Enter was pressed in it since the last frame.
func (e *Editor) Submitted(gtx ink.Ctx) bool {
	e.events(gtx)
	s := e.submitted
	e.submitted = false
	return s
}

// Changed says its text changed since the last frame.
func (e *Editor) Changed(gtx ink.Ctx) bool {
	e.events(gtx)
	c := e.changed
	e.changed = false
	return c
}

func (e *Editor) insert(s string) {
	rs := []rune(s)
	e.runes = append(e.runes[:e.caret], append(rs, e.runes[e.caret:]...)...)
	e.caret += len(rs)
	e.changed = true
}

func (e *Editor) events(gtx ink.Ctx) {
	for _, ev := range gtx.Events(e) {
		switch ev := ev.(type) {
		case ink.Pointer:
			if ev.Kind == ink.Press {
				e.tapped, e.tapAt = true, ev.Pos.X
			}
		case ink.Focus:
			e.focused = ev.On
		case ink.Text:
			if utf8.ValidString(ev.Text) {
				e.insert(ev.Text)
			}
		case ink.Key:
			switch ev.Name {
			case "Left":
				e.caret = max(e.caret-1, 0)
			case "Right":
				e.caret = min(e.caret+1, len(e.runes))
			case "Home":
				e.caret = 0
			case "End":
				e.caret = len(e.runes)
			case "Back":
				if e.caret > 0 {
					e.runes = append(e.runes[:e.caret-1], e.runes[e.caret:]...)
					e.caret--
					e.changed = true
				}
			case "Delete":
				if e.caret < len(e.runes) {
					e.runes = append(e.runes[:e.caret], e.runes[e.caret+1:]...)
					e.changed = true
				}
			case "Enter":
				e.submitted = true
			case "Space":
				e.insert(" ")
			}
		}
	}
}

func (e *Editor) shown() []rune {
	if e.Mask == 0 {
		return e.runes
	}
	m := make([]rune, len(e.runes))
	for i := range m {
		m[i] = e.Mask
	}
	return m
}

// Layout lays the line out in face f, gray g, as wide as it's given,
// hint (gray) in it while it's empty.
func (e *Editor) Layout(gtx ink.Ctx, f font.Face, g uint8, hint string) ink.Dims {
	e.events(gtx)
	e.face = f
	lineH := text.Height(f)
	w := gtx.Constraints.Max.X
	size := gtx.Constraints.Constrain(image.Pt(w, lineH))
	rs := e.shown()
	e.caret = min(max(e.caret, 0), len(rs))
	if e.tapped {
		x := e.tapAt + e.scroll
		best, bestD := 0, 1<<30
		for i := 0; i <= len(rs); i++ {
			d := abs(text.Width(f, string(rs[:i])) - x)
			if d < bestD {
				best, bestD = i, d
			}
		}
		e.caret = best
	}
	e.tapped = false
	cx := text.Width(f, string(rs[:e.caret]))
	pad := gtx.Dp(2)
	if cx-e.scroll > size.X-pad {
		e.scroll = cx - size.X + pad + size.X/4
	}
	if cx-e.scroll < 0 {
		e.scroll = max(cx-size.X/4, 0)
	}
	if text.Width(f, string(rs)) < size.X-pad {
		e.scroll = 0
	}
	base := (size.Y + 2*text.Ascent(f) - lineH) / 2
	clip := ink.Clip(gtx.Ops, image.Rectangle{Max: size})
	if len(rs) == 0 && hint != "" {
		ink.TextAt(gtx.Ops, f, 0, base, hint, ink.Mid)
	} else {
		ink.TextAt(gtx.Ops, f, -e.scroll, base, string(rs), g)
	}
	if e.focused && !gtx.IsDisabled() {
		x := cx - e.scroll
		ink.Fill(gtx.Ops, image.Rect(x, 0, x+max(gtx.Dp(1.5), 2), size.Y), ink.Black)
	}
	clip.Pop()
	if !gtx.IsDisabled() {
		ink.Input(gtx.Ops, ink.Area{Rect: image.Rectangle{Max: size}, Tag: e, Focus: true, Keys: []string{"*"}})
	}
	e.focused = gtx.Focused(e)
	return ink.Dims{Size: size, Baseline: size.Y - base}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
