package theme

import (
	"image"

	"hellobro/ink"
	"hellobro/ink/layout"
	"hellobro/ink/text"
	"hellobro/ink/widget"
)

// The controls past the first few: a checkbox, radio buttons, a dropdown
// and a menu (their list over everything), a slider, a progress bar, a
// section's heading, a tile, a framed text field, a toast.

// chevron draws a ▾ in a box s high at x, y (its top left).
func chevron(gtx ink.Ctx, x, y, s int, g uint8) {
	w := s * 2 / 3
	top := y + s/3
	ink.Polygon(gtx.Ops, g, image.Pt(x, top), image.Pt(x+w, top), image.Pt(x+w/2, top+w/2))
}

// check draws a ✓ in a box s big at x, y.
func check(gtx ink.Ctx, x, y, s, w int, g uint8) {
	ink.Line(gtx.Ops, w, g, image.Pt(x+s/6, y+s/2), image.Pt(x+s*5/12, y+s*3/4), image.Pt(x+s*5/6, y+s/4))
}

// --- a checkbox, radio buttons ---

// CheckBoxStyle is a row: a box, ticked when it's on, and what it is. A
// tap anywhere on it flips it.
type CheckBoxStyle struct {
	Theme *Theme
	Bool  *widget.Bool
	Label string
}

// CheckBox is b as a checkbox saying label.
func CheckBox(th *Theme, b *widget.Bool, label string) CheckBoxStyle {
	return CheckBoxStyle{th, b, label}
}

// Layout lays it out.
func (c CheckBoxStyle) Layout(gtx ink.Ctx) ink.Dims {
	return c.Bool.Layout(gtx, func(gtx ink.Ctx) ink.Dims {
		return markRow(gtx, c.Theme, c.Label, c.Bool.Pressed(), func(gtx ink.Ctx, r image.Rectangle) {
			th := c.Theme
			b := th.border(gtx)
			ink.Fill(gtx.Ops, r, th.Bg)
			ink.Frame(gtx.Ops, r, b+1, th.Fg)
			if c.Bool.Value {
				ink.Fill(gtx.Ops, r, th.Fg)
				check(gtx, r.Min.X, r.Min.Y, r.Dx(), max(b*2, 3), th.Bg)
			}
		})
	})
}

// RadioStyle is a row: a ring, filled when it's the one chosen, and
// what it is.
type RadioStyle struct {
	Theme *Theme
	Enum  *widget.Enum
	Key   string
	Label string
}

// Radio is key's choice of e, saying label.
func Radio(th *Theme, e *widget.Enum, key, label string) RadioStyle {
	return RadioStyle{th, e, key, label}
}

// Layout lays it out.
func (r RadioStyle) Layout(gtx ink.Ctx) ink.Dims {
	return r.Enum.Layout(gtx, r.Key, func(gtx ink.Ctx) ink.Dims {
		return markRow(gtx, r.Theme, r.Label, r.Enum.Pressed(r.Key), func(gtx ink.Ctx, box image.Rectangle) {
			th := r.Theme
			c := box.Min.Add(box.Max).Div(2)
			rad := box.Dx() / 2
			ink.Disc(gtx.Ops, c, rad, th.Bg)
			ink.Ring(gtx.Ops, c, rad, th.border(gtx)+1, th.Fg)
			if r.Enum.Value == r.Key {
				ink.Disc(gtx.Ops, c, rad/2, th.Fg)
			}
		})
	})
}

// markRow is a row a finger high: mark drawn in a box at its start, then
// label.
func markRow(gtx ink.Ctx, th *Theme, label string, pressed bool, mark func(gtx ink.Ctx, r image.Rectangle)) ink.Dims {
	f := th.Face(gtx, th.TextSize, text.Regular)
	pad := gtx.Dp(th.Pad * 1.5)
	s := gtx.Dp(th.TextSize * 1.2)
	h := gtx.Dp(th.Touch)
	w := gtx.Constraints.Max.X
	size := image.Pt(w, h)
	ink.Fill(gtx.Ops, image.Rectangle{Max: size}, th.Bg)
	mark(gtx, image.Rect(pad, (h-s)/2, pad+s, (h-s)/2+s))
	cg := gtx
	x := pad + s + gtx.Dp(th.Pad*1.5)
	cg.Constraints = ink.Exact(image.Pt(max(w-x-pad, 0), h))
	st := ink.Offset(gtx.Ops, image.Pt(x, 0))
	d := line(cg, f, label, th.Fg, Left, h)
	st.Pop()
	if pressed {
		ink.Invert(gtx.Ops, image.Rectangle{Max: size})
	}
	return ink.Dims{Size: size, Baseline: d.Baseline}
}

// --- a dropdown, a menu ---

// SelectStyle is a dropdown in a row: what it is, then its value in a
// box with a ▾; a tap opens the options under it, the one chosen ticked.
type SelectStyle struct {
	Theme   *Theme
	Select  *widget.Select
	Title   string // "": only the box, as wide as it's given
	Options []string
	BoxW    ink.Dp // the box, at most; 0: half the row, to 300
}

// Select is s as a row saying title, its options options.
func Select(th *Theme, s *widget.Select, title string, options ...string) SelectStyle {
	return SelectStyle{Theme: th, Select: s, Title: title, Options: options}
}

// Layout lays it out.
func (s SelectStyle) Layout(gtx ink.Ctx) ink.Dims {
	s.Select.Update(gtx, len(s.Options))
	value := ""
	if v := s.Select.Value; v >= 0 && v < len(s.Options) {
		value = s.Options[v]
	}
	box, d := fieldRow(gtx, s.Theme, s.Title, s.BoxW, &s.Select.Field, value, s.Select.IsOpen())
	choiceList(gtx, s.Theme, &s.Select.Choice, box, s.Options, s.Select.Value)
	return d
}

// field draws a dropdown's box: its value, a ▾ at its end; black while
// it's held or open.
func field(gtx ink.Ctx, th *Theme, value string, down, focused bool) ink.Dims {
	size := gtx.Constraints.Min
	r := image.Rectangle{Max: size}
	f := th.Face(gtx, th.TextSize*0.9, text.Regular)
	b := th.border(gtx)
	pad := gtx.Dp(th.Pad * 1.5)
	cs := gtx.Dp(th.TextSize)
	ink.Fill(gtx.Ops, r, th.Bg)
	ink.Frame(gtx.Ops, r, b+boolInt(focused), th.Fg)
	cg := gtx
	cg.Constraints = ink.Exact(image.Pt(max(size.X-2*pad-cs-gtx.Dp(th.Pad), 0), size.Y))
	st := ink.Offset(gtx.Ops, image.Pt(pad, 0))
	line(cg, f, value, th.Fg, Left, size.Y)
	st.Pop()
	chevron(gtx, size.X-pad-cs, (size.Y-cs)/2+cs/8, cs, th.Fg)
	if down {
		ink.Invert(gtx.Ops, r)
	}
	return ink.Dims{Size: size}
}

// choiceList lays c's list out by anchor, if it's open: items, the one
// current ticked (-1: none), the one the keys are on framed.
func choiceList(gtx ink.Ctx, th *Theme, c *widget.Choice, anchor image.Rectangle, items []string, current int) {
	if !c.IsOpen() {
		return
	}
	f := th.Face(gtx, th.TextSize, text.Regular)
	pad := gtx.Dp(th.Pad * 1.5)
	b := th.border(gtx)
	rowH := gtx.Dp(th.Touch)
	mark := gtx.Dp(th.TextSize)
	w := anchor.Dx()
	for _, s := range items {
		w = max(w, text.Width(f, s)+3*pad+mark+gtx.Dp(8))
	}
	win := gtx.WindowSize()
	w = min(w, win.X-2*pad)
	room := max(win.Y/2, rowH*3)
	h := min(len(items)*rowH, room-4*b) + 4*b
	c.Popup(gtx, anchor, image.Pt(w, h), func(gtx ink.Ctx) ink.Dims {
		r := image.Rectangle{Max: image.Pt(w, h)}
		ink.Fill(gtx.Ops, r, th.Bg)
		inner := r.Inset(2 * b)
		el := func(gtx ink.Ctx, i int) ink.Dims {
			cg := gtx
			cg.Constraints = ink.Exact(image.Pt(gtx.Constraints.Max.X, rowH))
			return c.Items[i].Layout(cg, func(gtx ink.Ctx) ink.Dims {
				rr := image.Rectangle{Max: gtx.Constraints.Min}
				ink.Fill(gtx.Ops, rr, th.Bg)
				if i == current {
					check(gtx, pad, (rowH-mark)/2, mark, max(2*b, 3), th.Fg)
				}
				cg := gtx
				cg.Constraints = ink.Exact(image.Pt(max(rr.Dx()-2*pad-mark-gtx.Dp(8), 0), rowH))
				st := ink.Offset(gtx.Ops, image.Pt(pad+mark+gtx.Dp(8), 0))
				line(cg, f, items[i], th.Fg, Left, rowH)
				st.Pop()
				if i == c.Highlit() {
					ink.Frame(gtx.Ops, rr.Inset(b), b*2, th.Fg)
				}
				if i < len(items)-1 {
					ink.Fill(gtx.Ops, image.Rect(pad/2, rowH-1, rr.Dx()-pad/2, rowH), th.Quiet)
				}
				if c.Items[i].Pressed() {
					ink.Invert(gtx.Ops, rr)
				}
				return ink.Dims{Size: rr.Size()}
			})
		}
		st := ink.Offset(gtx.Ops, inner.Min)
		cg := gtx
		cg.Constraints = ink.Exact(inner.Size())
		if len(items)*rowH <= inner.Dy() {
			for i := range items {
				s := ink.Offset(gtx.Ops, image.Pt(0, i*rowH))
				el(cg, i)
				s.Pop()
			}
		} else {
			List(th, &c.List).Layout(cg, len(items), el)
		}
		st.Pop()
		ink.Frame(gtx.Ops, r, 2*b, th.Fg)
		return ink.Dims{Size: r.Size()}
	})
}

// MenuStyle is a button with a ▾ that opens a list of actions.
type MenuStyle struct {
	Theme  *Theme
	Choice *widget.Choice
	Label  string
	Items  []string
}

// Menu is c as a button saying label, its actions items.
func Menu(th *Theme, c *widget.Choice, label string, items ...string) MenuStyle {
	return MenuStyle{th, c, label, items}
}

// Layout lays it out: the button as wide as its words.
func (m MenuStyle) Layout(gtx ink.Ctx) ink.Dims {
	th := m.Theme
	m.Choice.Update(gtx, len(m.Items), -1)
	f := th.Face(gtx, th.TextSize*0.9, text.Regular)
	pad := gtx.Dp(th.Pad * 1.5)
	size := image.Pt(text.Width(f, m.Label)+3*pad+gtx.Dp(th.TextSize)+gtx.Dp(th.Pad), gtx.Dp(th.Touch))
	cg := gtx
	cg.Constraints = ink.Exact(size)
	m.Choice.Field.Layout(cg, func(gtx ink.Ctx) ink.Dims {
		return field(gtx, th, m.Label, m.Choice.Field.Pressed() || m.Choice.IsOpen(), m.Choice.Field.Focused())
	})
	choiceList(gtx, th, m.Choice, image.Rectangle{Max: size}, m.Items, -1)
	return ink.Dims{Size: size}
}

// --- a slider, a progress bar ---

// SliderStyle is a row saying what it is and its value, a track under
// them: a finger on it sets the value, ← → step it.
type SliderStyle struct {
	Theme *Theme
	Float *widget.Float
	Title string
	Value string // the value as it's said
	Ticks bool   // a mark at each Step
}

// Slider is f as a slider saying title, its value value.
func Slider(th *Theme, f *widget.Float, title, value string) SliderStyle {
	return SliderStyle{Theme: th, Float: f, Title: title, Value: value}
}

// Layout lays it out.
func (s SliderStyle) Layout(gtx ink.Ctx) ink.Dims {
	th, fl := s.Theme, s.Float
	return sliderRow(gtx, th, s.Title, s.Value, func(gtx ink.Ctx) ink.Dims {
		return fl.Layout(gtx, func(gtx ink.Ctx) ink.Dims {
			size := gtx.Constraints.Min
			ink.Fill(gtx.Ops, image.Rectangle{Max: size}, th.Bg)
			b := th.border(gtx)
			mid := size.Y / 2
			k := gtx.Dp(th.Touch * 0.3)
			x := min(max(int(float32(size.X)*fl.Frac()), k), size.X-k)
			ink.Fill(gtx.Ops, image.Rect(0, mid-b, size.X, mid+b), th.Quiet)
			if steps := (fl.Max - fl.Min) / fl.Step; s.Ticks && fl.Step > 0 && steps <= 60 {
				for i := 0; i <= int(steps+0.5); i++ {
					tx := min(max(size.X*i/max(int(steps+0.5), 1), k), size.X-k) // where the thumb is at it
					ink.Fill(gtx.Ops, image.Rect(tx-b/2, mid+3*b, tx-b/2+max(b, 1), mid+k), th.Quiet)
				}
			}
			ink.Fill(gtx.Ops, image.Rect(0, mid-2*b, x, mid+2*b), th.Fg)
			ink.Disc(gtx.Ops, image.Pt(x, mid), k, th.Fg)
			if fl.Focused() {
				ink.Disc(gtx.Ops, image.Pt(x, mid), k/2, th.Bg)
			}
			return ink.Dims{Size: size}
		})
	})
}

// ProgressStyle is a bar, filled as far as it's got, words under it.
type ProgressStyle struct {
	Theme *Theme
	Frac  float32 // 0 to 1
	Text  string
}

// Progress is a bar frac of the way, saying s.
func Progress(th *Theme, frac float32, s string) ProgressStyle {
	return ProgressStyle{th, frac, s}
}

// Layout lays it out.
func (p ProgressStyle) Layout(gtx ink.Ctx) ink.Dims {
	th := p.Theme
	w := gtx.Constraints.Max.X
	pad := gtx.Dp(th.Pad * 1.5)
	b := th.border(gtx)
	bh := gtx.Dp(th.TextSize * 0.9)
	r := image.Rect(pad, gtx.Dp(th.Pad/2), w-pad, gtx.Dp(th.Pad/2)+bh)
	ink.Fill(gtx.Ops, r, th.Bg)
	in := r.Inset(b + gtx.Dp(1.5))
	in.Max.X = in.Min.X + int(float32(in.Dx())*min(max(p.Frac, 0), 1))
	ink.Fill(gtx.Ops, in, th.Fg)
	ink.Frame(gtx.Ops, r, b, th.Fg)
	h := r.Max.Y
	if p.Text != "" {
		small := th.Face(gtx, th.TextSize*0.85, text.Regular)
		lh := text.Height(small) + gtx.Dp(th.Pad/2)
		cg := gtx
		cg.Constraints = ink.Exact(image.Pt(r.Dx(), lh))
		st := ink.Offset(gtx.Ops, image.Pt(pad, h))
		line(cg, small, p.Text, th.Quiet, Left, lh)
		st.Pop()
		h += lh
	}
	return ink.Dims{Size: image.Pt(w, h+gtx.Dp(th.Pad/2))}
}

// --- headings, tiles ---

// Section is a group's heading: small, bold, a rule under it.
func Section(th *Theme, title string) ink.Widget {
	return func(gtx ink.Ctx) ink.Dims {
		f := th.Face(gtx, th.TextSize*0.85, text.Bold)
		pad := gtx.Dp(th.Pad * 1.5)
		w := gtx.Constraints.Max.X
		h := text.Height(f) + gtx.Dp(th.Pad*1.5)
		cg := gtx
		cg.Constraints = ink.Exact(image.Pt(max(w-2*pad, 0), h))
		st := ink.Offset(gtx.Ops, image.Pt(pad, gtx.Dp(th.Pad/2)))
		line(cg, f, title, th.Quiet, Left, h)
		st.Pop()
		ink.Fill(gtx.Ops, image.Rect(pad/2, h, w-pad/2, h+1), th.Quiet)
		return ink.Dims{Size: image.Pt(w, h+gtx.Dp(th.Pad/2))}
	}
}

// TileStyle is a framed box to tap, as the launcher's: an icon maybe, a
// title, a line under it.
type TileStyle struct {
	Theme  *Theme
	Button *widget.Clickable
	Title  string
	Sub    string
	Icon   func(gtx ink.Ctx, r image.Rectangle) // draws the icon in r; nil: none
}

// Tile is clk as a tile saying title, and sub under it.
func Tile(th *Theme, clk *widget.Clickable, title, sub string) TileStyle {
	return TileStyle{Theme: th, Button: clk, Title: title, Sub: sub}
}

// Layout lays it out, as wide as it's given.
func (t TileStyle) Layout(gtx ink.Ctx) ink.Dims {
	th := t.Theme
	return t.Button.Layout(gtx, func(gtx ink.Ctx) ink.Dims {
		w := gtx.Constraints.Max.X
		f := th.Face(gtx, th.TextSize, text.Bold)
		small := th.Face(gtx, th.TextSize*0.8, text.Regular)
		pad := gtx.Dp(th.Pad * 1.5)
		h := max(gtx.Dp(th.Touch*1.6), text.Height(f)+text.Height(small)+2*pad)
		size := image.Pt(w, h)
		r := image.Rectangle{Max: size}
		ink.Fill(gtx.Ops, r, th.Bg)
		ink.Frame(gtx.Ops, r, th.border(gtx)+boolInt(t.Button.Focused()), th.Fg)
		x := pad
		if t.Icon != nil {
			is := h - 2*pad
			t.Icon(gtx, image.Rect(pad, pad, pad+is, pad+is))
			x += is + pad
		}
		tw := max(w-x-pad, 0)
		top := (h - text.Height(f) - text.Height(small)) / 2
		if t.Sub == "" {
			top = (h - text.Height(f)) / 2
		}
		ink.TextAt(gtx.Ops, f, x, top+text.Ascent(f), text.Fit(f, t.Title, tw), th.Fg)
		if t.Sub != "" {
			ink.TextAt(gtx.Ops, small, x, top+text.Height(f)+text.Ascent(small), text.Fit(small, t.Sub, tw), th.Quiet)
		}
		if t.Button.Pressed() {
			ink.Invert(gtx.Ops, r)
		}
		return ink.Dims{Size: size}
	})
}

// --- a text field ---

// EditorStyle is a framed line to type in, a hint in it while it's
// empty, maybe a × that empties it.
type EditorStyle struct {
	Theme  *Theme
	Editor *widget.Editor
	Hint   string
	Clear  *widget.Clickable // nil: no ×
}

// Editor is e framed, saying hint while it's empty.
func Editor(th *Theme, e *widget.Editor, hint string) EditorStyle {
	return EditorStyle{Theme: th, Editor: e, Hint: hint}
}

// Layout lays it out, as wide as it's given.
func (e EditorStyle) Layout(gtx ink.Ctx) ink.Dims {
	th := e.Theme
	if e.Clear != nil {
		for e.Clear.Clicked(gtx) {
			e.Editor.SetText("")
			gtx.Focus(e.Editor)
		}
	}
	w, h := gtx.Constraints.Max.X, gtx.Dp(th.Touch)
	size := image.Pt(w, h)
	r := image.Rectangle{Max: size}
	b := th.border(gtx)
	ink.Fill(gtx.Ops, r, th.Bg)
	ink.Frame(gtx.Ops, r, b+boolInt(e.Editor.Focused()), th.Fg)
	pad := gtx.Dp(th.Pad * 1.5)
	xw := 0
	if e.Clear != nil && e.Editor.Len() > 0 {
		xw = h
	}
	cg := gtx
	cg.Constraints = ink.Exact(image.Pt(max(w-2*pad-xw, 0), h))
	st := ink.Offset(gtx.Ops, image.Pt(pad, 0))
	layout.W.Layout(cg, func(gtx ink.Ctx) ink.Dims {
		return e.Editor.Layout(gtx, th.Face(gtx, th.TextSize, text.Regular), th.Fg, e.Hint)
	})
	st.Pop()
	if xw > 0 {
		st := ink.Offset(gtx.Ops, image.Pt(w-xw, 0))
		cg.Constraints = ink.Exact(image.Pt(xw, h))
		e.Clear.NoFocus = true
		e.Clear.Layout(cg, func(gtx ink.Ctx) ink.Dims {
			s := h / 4
			c := image.Pt(xw/2, h/2)
			ink.Line(gtx.Ops, max(2*b, 3), th.Fg, c.Add(image.Pt(-s/2, -s/2)), c.Add(image.Pt(s/2, s/2)))
			ink.Line(gtx.Ops, max(2*b, 3), th.Fg, c.Add(image.Pt(s/2, -s/2)), c.Add(image.Pt(-s/2, s/2)))
			if e.Clear.Pressed() {
				ink.Invert(gtx.Ops, image.Rectangle{Max: image.Pt(xw, h)}.Inset(b+1))
			}
			return ink.Dims{Size: image.Pt(xw, h)}
		})
		st.Pop()
	}
	return ink.Dims{Size: size}
}

// --- a toast ---

// ToastStyle is a word over the foot of the page, black, for a moment.
type ToastStyle struct {
	Theme *Theme
	Toast *widget.Toast
}

// Toast is t as it shows.
func Toast(th *Theme, t *widget.Toast) ToastStyle { return ToastStyle{th, t} }

// Layout lays it out over the foot of all it's given, if it's up: last,
// so it's over the rest. It takes no room.
func (t ToastStyle) Layout(gtx ink.Ctx) ink.Dims {
	if !t.Toast.Showing(gtx) {
		return ink.Dims{}
	}
	th := t.Theme
	f := th.Face(gtx, th.TextSize, text.Regular)
	pad := gtx.Dp(th.Pad * 2)
	max := gtx.Constraints.Max
	s := text.Fit(f, t.Toast.Text, max.X-4*pad)
	w, h := text.Width(f, s)+2*pad, text.Height(f)+pad
	r := image.Rect((max.X-w)/2, max.Y-h-pad*2, (max.X+w)/2, max.Y-pad*2)
	ink.Fill(gtx.Ops, r, th.Fg)
	ink.TextAt(gtx.Ops, f, r.Min.X+pad, r.Min.Y+(h+text.Ascent(f)-f.Metrics().Descent.Ceil())/2, s, th.Bg)
	return ink.Dims{}
}
