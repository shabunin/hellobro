// Package theme is how ink's controls look: black on white, framed,
// pressed ones inverted (the fast black-and-white waveform shows it at
// once), grays only for what's quiet. Each is a Style made by a function
// (Button, Toggle, Select, …) and laid out with its Layout, as Gio's
// material package does.
package theme

import (
	"image"

	"golang.org/x/image/font"

	"hellobro/ink"
	"hellobro/ink/layout"
	"hellobro/ink/text"
	"hellobro/ink/widget"
)

// Theme is the look: fonts, sizes, grays.
type Theme struct {
	Shaper   *text.Shaper
	TextSize ink.Dp // the body's letters
	Fg, Bg   uint8
	Quiet    uint8  // what's said aside, and what's off
	Border   ink.Dp // a frame's thickness
	Touch    ink.Dp // a control's height, at least: a finger's
	Pad      ink.Dp // between things
}

// New is the theme as hellobro's UI looks.
func New() *Theme {
	return &Theme{
		Shaper: text.NewShaper(), TextSize: 18, Fg: ink.Black, Bg: ink.White, Quiet: ink.Dark,
		Border: 1.5, Touch: 44, Pad: 8,
	}
}

// Face is the face of style, size big.
func (th *Theme) Face(gtx ink.Ctx, size ink.Dp, st text.Style) font.Face {
	return th.Shaper.Face(st, gtx.Dp(size))
}

func (th *Theme) border(gtx ink.Ctx) int { return max(gtx.Dp(th.Border), 1) }

// --- labels ---

// Align is where a line goes across its room.
type Align uint8

const (
	Left Align = iota
	Center
	Right
)

// LabelStyle is text: wrapped to the room it has, as many lines as it
// may have, the last cut with an ellipsis.
type LabelStyle struct {
	Theme    *Theme
	Text     string
	Size     ink.Dp
	Style    text.Style
	Gray     uint8
	Align    Align
	MaxLines int // 0: as many as it takes
}

// Label is text size big.
func Label(th *Theme, size ink.Dp, s string) LabelStyle {
	return LabelStyle{Theme: th, Text: s, Size: size, Gray: th.Fg}
}

// Body is text at the body's size.
func Body(th *Theme, s string) LabelStyle { return Label(th, th.TextSize, s) }

// Small is text said aside: smaller, gray.
func Small(th *Theme, s string) LabelStyle {
	l := Label(th, th.TextSize*0.85, s)
	l.Gray = th.Quiet
	return l
}

// Title is a heading: bigger, bold.
func Title(th *Theme, s string) LabelStyle {
	l := Label(th, th.TextSize*1.4, s)
	l.Style = text.Bold
	return l
}

// Layout lays it out.
func (l LabelStyle) Layout(gtx ink.Ctx) ink.Dims {
	f := l.Theme.Face(gtx, l.Size, l.Style)
	w := gtx.Constraints.Max.X
	lines := text.Wrap(f, l.Text, w)
	if l.MaxLines > 0 && len(lines) > l.MaxLines {
		lines = lines[:l.MaxLines]
		lines[len(lines)-1] = text.Fit(f, lines[len(lines)-1]+" …", w)
	}
	lh, asc := text.Height(f), text.Ascent(f)
	width := 0
	for _, s := range lines {
		width = max(width, text.Width(f, s))
	}
	size := gtx.Constraints.Constrain(image.Pt(width, lh*len(lines)))
	for i, s := range lines {
		x := 0
		switch l.Align {
		case Center:
			x = (size.X - text.Width(f, s)) / 2
		case Right:
			x = size.X - text.Width(f, s)
		}
		ink.TextAt(gtx.Ops, f, x, i*lh+asc, s, l.Gray)
	}
	return ink.Dims{Size: size, Baseline: size.Y - asc}
}

// line lays a line of text out vertically centered in a box h high (as
// wide as given), cut short if it's too long.
func line(gtx ink.Ctx, f font.Face, s string, g uint8, al Align, h int) ink.Dims {
	w := gtx.Constraints.Max.X
	s = text.Fit(f, s, w)
	tw := text.Width(f, s)
	size := gtx.Constraints.Constrain(image.Pt(tw, h))
	x := 0
	switch al {
	case Center:
		x = (size.X - tw) / 2
	case Right:
		x = size.X - tw
	}
	m := f.Metrics()
	base := (size.Y + m.Ascent.Ceil() - m.Descent.Ceil()) / 2
	ink.TextAt(gtx.Ops, f, x, base, s, g)
	return ink.Dims{Size: size, Baseline: size.Y - base}
}

// --- buttons ---

// ButtonStyle is a framed button with words in it.
type ButtonStyle struct {
	Theme  *Theme
	Button *widget.Clickable
	Text   string
	Strong bool // filled: the one the dialog wants
}

// Button is clk as a button saying s.
func Button(th *Theme, clk *widget.Clickable, s string) ButtonStyle {
	return ButtonStyle{Theme: th, Button: clk, Text: s}
}

// Layout lays it out: as wide as its words, at least a finger high.
func (b ButtonStyle) Layout(gtx ink.Ctx) ink.Dims {
	th := b.Theme
	return b.Button.Layout(gtx, func(gtx ink.Ctx) ink.Dims {
		f := th.Face(gtx, th.TextSize, text.Regular)
		h := gtx.Dp(th.Touch)
		padX := gtx.Dp(th.Pad * 2)
		w := text.Width(f, b.Text) + 2*padX
		size := gtx.Constraints.Constrain(image.Pt(w, h))
		r := image.Rectangle{Max: size}
		fg, bg := th.Fg, th.Bg
		if gtx.IsDisabled() {
			fg = th.Quiet
		}
		if b.Strong {
			fg, bg = bg, fg
		}
		ink.Fill(gtx.Ops, r, bg)
		ink.Frame(gtx.Ops, r, th.border(gtx)+boolInt(b.Button.Focused()), th.Fg) // black, filled or not: the same size
		in := gtx
		in.Constraints = ink.Exact(image.Pt(max(size.X-2*padX, 0), size.Y))
		st := ink.Offset(gtx.Ops, image.Pt(padX, 0))
		line(in, f, b.Text, fg, Center, size.Y)
		st.Pop()
		if b.Button.Pressed() {
			ink.Invert(gtx.Ops, r)
		}
		return ink.Dims{Size: size, Baseline: size.Y - (size.Y+f.Metrics().Ascent.Ceil()-f.Metrics().Descent.Ceil())/2}
	})
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// --- rows ---

// RowStyle is a row of a list: a title, maybe a line under it, a value
// at its end, the whole of it a thing to tap.
type RowStyle struct {
	Theme  *Theme
	Button *widget.Clickable // nil: it's not for tapping
	Title  string
	Sub    string // under the title, smaller
	Value  string // at the end
	More   bool   // a › at the end: it opens something
	Rule   bool   // a hairline under it
}

// Row is a row saying title, tapped through clk.
func Row(th *Theme, clk *widget.Clickable, title string) RowStyle {
	return RowStyle{Theme: th, Button: clk, Title: title, Rule: true}
}

// Layout lays it out, as wide as it's given.
func (r RowStyle) Layout(gtx ink.Ctx) ink.Dims {
	if r.Button == nil {
		return r.layout(gtx, false)
	}
	return r.Button.Layout(gtx, func(gtx ink.Ctx) ink.Dims { return r.layout(gtx, r.Button.Pressed()) })
}

func (r RowStyle) layout(gtx ink.Ctx, pressed bool) ink.Dims {
	th := r.Theme
	w := gtx.Constraints.Max.X
	pad := gtx.Dp(th.Pad * 1.5)
	f := th.Face(gtx, th.TextSize, text.Regular)
	small := th.Face(gtx, th.TextSize*0.85, text.Regular)
	h := max(gtx.Dp(th.Touch), text.Height(f)+pad)
	if r.Sub != "" {
		h = max(h, text.Height(f)+text.Height(small)+pad)
	}
	size := image.Pt(w, h)
	ink.Fill(gtx.Ops, image.Rectangle{Max: size}, th.Bg)
	end := w - pad
	if r.More {
		mw := text.Width(f, "›")
		ink.TextAt(gtx.Ops, f, end-mw, (h+text.Ascent(f)-f.Metrics().Descent.Ceil())/2, "›", th.Fg)
		end -= mw + pad
	}
	if r.Value != "" {
		vw := min(text.Width(small, r.Value), (end-pad)/2)
		v := text.Fit(small, r.Value, vw)
		ink.TextAt(gtx.Ops, small, end-text.Width(small, v), (h+text.Ascent(small)-small.Metrics().Descent.Ceil())/2, v, th.Quiet)
		end -= vw + pad
	}
	tw := max(end-pad, 0)
	if r.Sub == "" {
		ink.TextAt(gtx.Ops, f, pad, (h+text.Ascent(f)-f.Metrics().Descent.Ceil())/2, text.Fit(f, r.Title, tw), th.Fg)
	} else {
		top := (h - text.Height(f) - text.Height(small)) / 2
		ink.TextAt(gtx.Ops, f, pad, top+text.Ascent(f), text.Fit(f, r.Title, tw), th.Fg)
		ink.TextAt(gtx.Ops, small, pad, top+text.Height(f)+text.Ascent(small), text.Fit(small, r.Sub, tw), th.Quiet)
	}
	if r.Rule {
		ink.Fill(gtx.Ops, image.Rect(pad/2, h-1, w-pad/2, h), th.Quiet)
	}
	if pressed {
		ink.Invert(gtx.Ops, image.Rectangle{Max: size})
	}
	return ink.Dims{Size: size, Baseline: h / 3}
}

// --- a switch ---

// ToggleStyle is a row with a switch at its end: a tap anywhere on it
// flips it.
type ToggleStyle struct {
	Theme *Theme
	Bool  *widget.Bool
	Title string
	Sub   string
}

// Toggle is b as a row saying title.
func Toggle(th *Theme, b *widget.Bool, title string) ToggleStyle {
	return ToggleStyle{Theme: th, Bool: b, Title: title}
}

// Layout lays it out.
func (t ToggleStyle) Layout(gtx ink.Ctx) ink.Dims {
	th := t.Theme
	return t.Bool.Layout(gtx, func(gtx ink.Ctx) ink.Dims {
		w := gtx.Constraints.Max.X
		pad := gtx.Dp(th.Pad * 1.5)
		sw, sh := gtx.Dp(th.Touch*1.1), gtx.Dp(th.Touch*0.6)
		row := RowStyle{Theme: th, Title: t.Title, Sub: t.Sub}
		// The row's words end before the switch.
		m := ink.Record(gtx.Ops)
		cg := gtx
		cg.Constraints = ink.Constraints{Min: image.Pt(w-sw-pad, 0), Max: image.Pt(w-sw-pad, gtx.Constraints.Max.Y)}
		d := row.layout(cg, false)
		call := m.Stop()
		size := image.Pt(w, d.Size.Y)
		ink.Fill(gtx.Ops, image.Rectangle{Max: size}, th.Bg)
		call.Add(gtx.Ops)
		ink.Fill(gtx.Ops, image.Rect(pad/2, size.Y-1, w-pad/2, size.Y), th.Quiet)
		// The switch: a track, its knob at the end that's on.
		tr := image.Rect(w-pad-sw, (size.Y-sh)/2, w-pad, (size.Y+sh)/2)
		b := th.border(gtx)
		knob := tr.Inset(b + gtx.Dp(2))
		knob.Max.X = knob.Min.X + knob.Dy()
		if t.Bool.Value {
			ink.Fill(gtx.Ops, tr, th.Fg)
			knob = knob.Add(image.Pt(tr.Dx()-2*(b+gtx.Dp(2))-knob.Dx(), 0))
			ink.Fill(gtx.Ops, knob, th.Bg)
		} else {
			ink.Fill(gtx.Ops, tr, th.Bg)
			ink.Frame(gtx.Ops, tr, b, th.Fg)
			ink.Fill(gtx.Ops, knob, th.Fg)
		}
		if t.Bool.Pressed() {
			ink.Invert(gtx.Ops, image.Rectangle{Max: size})
		}
		return ink.Dims{Size: size, Baseline: d.Baseline}
	})
}

// --- tabs ---

// TabsStyle is a row of choices, the one chosen black.
type TabsStyle struct {
	Theme *Theme
	Enum  *widget.Enum
	Keys  []string
	Names []string
}

// Tabs is e's choices keys, saying names.
func Tabs(th *Theme, e *widget.Enum, keys, names []string) TabsStyle {
	return TabsStyle{Theme: th, Enum: e, Keys: keys, Names: names}
}

// Layout lays them out, sharing the row as their names are long.
func (t TabsStyle) Layout(gtx ink.Ctx) ink.Dims {
	th := t.Theme
	var kids []layout.FlexChild
	bold := th.Face(gtx, th.TextSize, text.Bold)
	pad := gtx.Dp(th.Pad * 2)
	for i, k := range t.Keys {
		name := k
		if i < len(t.Names) {
			name = t.Names[i]
		}
		kids = append(kids, layout.Flexed(float32(text.Width(bold, name)+2*pad), func(gtx ink.Ctx) ink.Dims {
			return t.Enum.Layout(gtx, k, func(gtx ink.Ctx) ink.Dims {
				f := th.Face(gtx, th.TextSize, text.Regular)
				on := t.Enum.Value == k
				if on {
					f = th.Face(gtx, th.TextSize, text.Bold)
				}
				size := image.Pt(gtx.Constraints.Min.X, gtx.Dp(th.Touch))
				r := image.Rectangle{Max: size}
				fg, bg := th.Fg, th.Bg
				if on {
					fg, bg = bg, fg
				}
				ink.Fill(gtx.Ops, r, bg)
				ink.Frame(gtx.Ops, r, th.border(gtx), th.Fg)
				cg := gtx
				cg.Constraints = ink.Exact(size)
				line(cg, f, name, fg, Center, size.Y)
				if t.Enum.Pressed(k) {
					ink.Invert(gtx.Ops, r)
				}
				return ink.Dims{Size: size}
			})
		}))
	}
	return layout.Flex{}.Layout(gtx, kids...)
}

// --- a header ---

// HeaderStyle is a window's top line: ‹ back to where it came from, its
// title, maybe a button at the end.
type HeaderStyle struct {
	Theme    *Theme
	Back     *widget.Clickable // nil: no way back
	BackText string
	Title    string
	Action   *widget.Clickable
	ActText  string
}

// Header is a header saying title.
func Header(th *Theme, title string) HeaderStyle { return HeaderStyle{Theme: th, Title: title} }

// Layout lays it out, a rule under it.
func (h HeaderStyle) Layout(gtx ink.Ctx) ink.Dims {
	th := h.Theme
	w := gtx.Constraints.Max.X
	var kids []layout.FlexChild
	if h.Back != nil {
		kids = append(kids, layout.Rigid(func(gtx ink.Ctx) ink.Dims {
			return h.Back.Layout(gtx, func(gtx ink.Ctx) ink.Dims {
				f := th.Face(gtx, th.TextSize, text.Regular)
				s := "‹ " + h.BackText
				size := image.Pt(text.Width(f, s)+gtx.Dp(th.Pad*2), gtx.Dp(th.Touch))
				cg := gtx
				cg.Constraints = ink.Exact(size)
				ink.Fill(gtx.Ops, image.Rectangle{Max: size}, th.Bg)
				line(cg, f, s, th.Fg, Center, size.Y)
				if h.Back.Pressed() {
					ink.Invert(gtx.Ops, image.Rectangle{Max: size})
				}
				return ink.Dims{Size: size}
			})
		}))
	}
	kids = append(kids, layout.Flexed(1, func(gtx ink.Ctx) ink.Dims {
		f := th.Face(gtx, th.TextSize*1.2, text.Bold)
		al := Center
		if h.Back == nil {
			al = Left
		}
		cg := gtx
		cg.Constraints = ink.Exact(image.Pt(gtx.Constraints.Min.X, gtx.Dp(th.Touch)))
		st := ink.Offset(gtx.Ops, image.Pt(gtx.Dp(th.Pad), 0))
		cg.Constraints.Max.X = max(cg.Constraints.Max.X-2*gtx.Dp(th.Pad), 0)
		cg.Constraints.Min.X = cg.Constraints.Max.X
		line(cg, f, h.Title, th.Fg, al, cg.Constraints.Max.Y)
		st.Pop()
		return ink.Dims{Size: image.Pt(gtx.Constraints.Min.X, gtx.Dp(th.Touch))}
	}))
	if h.Action != nil {
		kids = append(kids, layout.Rigid(Button(th, h.Action, h.ActText).Layout))
	}
	d := layout.Flex{Alignment: layout.Middle}.Layout(gtx, kids...)
	ink.Fill(gtx.Ops, image.Rect(0, d.Size.Y, w, d.Size.Y+th.border(gtx)), th.Fg)
	return ink.Dims{Size: image.Pt(w, d.Size.Y+th.border(gtx))}
}

// --- lists ---

// ListStyle is a list, by the page (layout.List) or under the finger
// (layout.Scroll), with a bar at its side: where it is in it.
type ListStyle struct {
	Theme *Theme
	List  layout.Scroller
}

// List is l with its bar.
func List(th *Theme, l layout.Scroller) ListStyle { return ListStyle{th, l} }

// Layout lays out what shows of the n items, and the bar if they don't
// all fit.
func (l ListStyle) Layout(gtx ink.Ctx, n int, el func(gtx ink.Ctx, i int) ink.Dims) ink.Dims {
	bar := l.Theme.bar(gtx)
	from, to := l.List.Span()
	full := n > 0 && (from > 0 || to < 1)
	cg := gtx
	if full {
		cg.Constraints.Max.X = max(gtx.Constraints.Max.X-bar, 0)
		cg.Constraints.Min.X = min(cg.Constraints.Min.X, cg.Constraints.Max.X)
	}
	d := l.List.Layout(cg, n, el)
	if from, to = l.List.Span(); n > 0 && (from > 0 || to < 1) {
		l.Theme.drawBar(gtx, gtx.Constraints.Max.X, gtx.Constraints.Max.Y, from, to)
		if !full {
			gtx.Invalidate() // it learned it doesn't all fit: again, narrower
		}
		d.Size.X = gtx.Constraints.Max.X
	}
	return d
}

// bar is the room a list's bar takes at its side, the gap before it too.
func (th *Theme) bar(gtx ink.Ctx) int { return gtx.Dp(4) + gtx.Dp(th.Pad/2) }

// drawBar draws a bar down the right of a box w by h: the part from to
// to shows.
func (th *Theme) drawBar(gtx ink.Ctx, w, h int, from, to float32) {
	bar := gtx.Dp(4)
	x := w - bar
	y0, y1 := int(float32(h)*from), int(float32(h)*to)
	ink.Fill(gtx.Ops, image.Rect(x+bar/2-1, 0, x+bar/2+1, h), th.Quiet)
	ink.Fill(gtx.Ops, image.Rect(x, y0, x+bar, max(y1, y0+bar*2)), th.Fg)
}

// ScrollBox is w in a box it moves in under the finger, a bar at its
// side when it's longer than the box.
func ScrollBox(th *Theme, s *layout.Scroll, w ink.Widget) ink.Widget {
	return func(gtx ink.Ctx) ink.Dims {
		s.Axis = layout.Vertical
		return List(th, s).Layout(gtx, 1, func(gtx ink.Ctx, _ int) ink.Dims { return w(gtx) })
	}
}

// --- dialogs ---

// DialogStyle is a box over everything: a title, words, buttons. Nothing
// under it takes a tap while it's up.
type DialogStyle struct {
	Theme   *Theme
	Title   string
	Text    string
	Buttons []ButtonStyle
	Body    ink.Widget // more, under the words
}

// Dialog is a dialog saying title and s, with buttons.
func Dialog(th *Theme, title, s string, buttons ...ButtonStyle) DialogStyle {
	return DialogStyle{Theme: th, Title: title, Text: s, Buttons: buttons}
}

// Layout lays it out over all it's given.
func (d DialogStyle) Layout(gtx ink.Ctx) ink.Dims {
	th := d.Theme
	size := gtx.Constraints.Max
	ink.Input(gtx.Ops, ink.Area{Rect: image.Rectangle{Max: size}}) // the backdrop: taps stop here
	bw := min(size.X-gtx.Dp(th.Pad*4), gtx.Dp(480))
	cg := gtx
	cg.Constraints = ink.Constraints{Min: image.Pt(bw, 0), Max: image.Pt(bw, size.Y)}
	m := ink.Record(gtx.Ops)
	pad := th.Pad * 2
	box := layout.UniformInset(pad).Layout(cg, func(gtx ink.Ctx) ink.Dims {
		var kids []layout.FlexChild
		if d.Title != "" {
			kids = append(kids, layout.Rigid(Title(th, d.Title).Layout), layout.Rigid(layout.Spacer{Height: th.Pad}.Layout))
		}
		if d.Text != "" {
			kids = append(kids, layout.Rigid(Body(th, d.Text).Layout))
		}
		if d.Body != nil {
			kids = append(kids, layout.Rigid(layout.Spacer{Height: th.Pad}.Layout), layout.Rigid(d.Body))
		}
		if len(d.Buttons) > 0 {
			kids = append(kids, layout.Rigid(layout.Spacer{Height: th.Pad * 2}.Layout))
			kids = append(kids, layout.Rigid(func(gtx ink.Ctx) ink.Dims {
				var bs []layout.FlexChild
				for i, b := range d.Buttons {
					if i > 0 {
						bs = append(bs, layout.Rigid(layout.Spacer{Width: th.Pad}.Layout))
					}
					bs = append(bs, layout.Rigid(b.Layout))
				}
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.Flex{Spacing: layout.SpaceStart}.Layout(gtx, bs...)
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
	})
	call := m.Stop()
	at := image.Pt((size.X-box.Size.X)/2, (size.Y-box.Size.Y)/2)
	r := image.Rectangle{Min: at, Max: at.Add(box.Size)}
	b := th.border(gtx)
	// A thick frame, a shadow-ish rule at its foot and right: it stands
	// over the page.
	ink.Fill(gtx.Ops, r.Inset(-b*3), th.Bg)
	ink.Fill(gtx.Ops, image.Rectangle{Min: r.Min.Add(image.Pt(b*4, b*4)), Max: r.Max.Add(image.Pt(b*4, b*4))}, th.Quiet)
	ink.Fill(gtx.Ops, r, th.Bg)
	ink.Frame(gtx.Ops, r, b*2, th.Fg)
	st := ink.Offset(gtx.Ops, at)
	call.Add(gtx.Ops)
	st.Pop()
	return ink.Dims{Size: size}
}

// Divider is a hairline across.
func Divider(th *Theme) ink.Widget {
	return func(gtx ink.Ctx) ink.Dims {
		h := gtx.Dp(th.Pad)
		w := gtx.Constraints.Max.X
		ink.Fill(gtx.Ops, image.Rect(0, h/2, w, h/2+1), th.Quiet)
		return ink.Dims{Size: image.Pt(w, h)}
	}
}
