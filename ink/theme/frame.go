package theme

// The window manager's frame, in ink: the bar along the top (the tabs,
// a + for a new one, the status), a window's title strip and its
// buttons, a tabbed container's tab, a note in the corner. Where each part goes is
// the host's to say: it lays the frame out (its taps go by those
// places), and these paint it, so every app's frame looks the same.

import (
	"image"

	"golang.org/x/image/font"

	"hellobro/ink"
	"hellobro/ink/text"
)

// FrameLook is the frame's letters and room: a name's (bold), what's
// said aside, inside a bar's tab at either end, and between a tab's
// name and what's in it.
type FrameLook struct {
	Name, Small ink.Dp
	TabPad, Gap ink.Dp
}

// lineIn puts s on one line in r, the middle of it down r, cut to fit.
func lineIn(o *ink.Ops, f font.Face, r image.Rectangle, s string, g uint8, al Align) {
	if r.Empty() || s == "" {
		return
	}
	s = text.Fit(f, s, r.Dx())
	w := text.Width(f, s)
	x := r.Min.X
	switch al {
	case Center:
		x += (r.Dx() - w) / 2
	case Right:
		x = r.Max.X - w
	}
	m := f.Metrics()
	st := ink.Clip(o, r)
	ink.TextAt(o, f, x, r.Min.Y+(r.Dy()+m.Ascent.Ceil()-m.Descent.Ceil())/2, s, g)
	st.Pop()
}

// BarTab is a tab in the bar: its name and what's in it.
type BarTab struct {
	R           image.Rectangle
	Label, Info string
	Up          bool // shown: white in the black bar
	Line        bool // a thin line at its left, between two not up
}

// Bar is the bar along the top, black, as i3's: its tabs, a + after
// them for a new one (Plus empty: none), the status at the right and a
// note before it.
type Bar struct {
	Look           FrameLook
	Tabs           []BarTab
	Plus           image.Rectangle
	Status, Note   string
	StatusR, NoteR image.Rectangle // NoteR: where the note's room ends at the right
}

func (b Bar) Layout(gtx ink.Ctx, th *Theme) ink.Dims {
	size := gtx.Constraints.Max
	o := gtx.Ops
	ink.Fill(o, image.Rectangle{Max: size}, ink.Black)
	name, small := th.Face(gtx, b.Look.Name, text.Bold), th.Face(gtx, b.Look.Small, text.Regular)
	pad, gap := gtx.Dp(b.Look.TabPad), gtx.Dp(b.Look.Gap)
	for _, t := range b.Tabs {
		fg, dim := uint8(ink.White), uint8(ink.Light)
		if t.Up {
			ink.Fill(o, t.R, ink.White)
			fg, dim = ink.Black, ink.Dark
		} else if t.Line {
			ink.Fill(o, image.Rect(t.R.Min.X, t.R.Min.Y+10, t.R.Min.X+1, t.R.Max.Y-10), ink.Mid)
		}
		in := image.Rect(t.R.Min.X+pad, t.R.Min.Y, t.R.Max.X-pad, t.R.Max.Y)
		lineIn(o, name, in, t.Label, fg, Left)
		if lw := text.Width(name, t.Label) + gap; t.Info != "" && lw < in.Dx() {
			in.Min.X += lw
			lineIn(o, small, in, t.Info, dim, Left)
		}
	}
	if p := b.Plus; !p.Empty() {
		box := p.Inset(max(4, p.Dy()/6))
		ink.Frame(o, box, 2, ink.White)
		plus(o, box.Min.Add(box.Max).Div(2), box.Dy()/4, ink.White)
	}
	lineIn(o, small, b.StatusR, b.Status, ink.Light, Right)
	lineIn(o, small, b.NoteR, b.Note, ink.White, Right)
	return ink.Dims{Size: size}
}

// plus is a + round m, arm long, 3 px thick.
func plus(o *ink.Ops, m image.Point, arm int, g uint8) {
	ink.Fill(o, image.Rect(m.X-arm, m.Y-1, m.X+arm+1, m.Y+2), g)
	ink.Fill(o, image.Rect(m.X-1, m.Y-arm, m.X+2, m.Y+arm+1), g)
}

// StripBox is a box in a title strip: a window's button (an icon, and
// a word if there's room), its fold, its +, a button with words.
type StripBox struct {
	R     image.Rectangle
	Icon  func(dst *image.Gray, m image.Point) // drawn round m, black on white; nil: none
	IconX int                                  // with a word, the icon's middle from the box's left
	IconW int                                  // and its width: the word after it
	Label string
	Plus  bool // a + (a fold folded, a window's new one)
	Minus bool // a − (a fold open)
}

// Strip is a window's title strip: light, black while it has the
// focus; its name, the status at the right, its boxes.
type Strip struct {
	Look      FrameLook
	Name      string
	Status    string
	Focused   bool
	NameR     image.Rectangle
	StatusR   image.Rectangle
	Boxes     []StripBox
	LabelOnly bool // the boxes' words alone, in the middle (a button)
}

func (s Strip) Layout(gtx ink.Ctx, th *Theme) ink.Dims {
	size := gtx.Constraints.Max
	o := gtx.Ops
	bg, fg, dim := uint8(ink.Light), uint8(ink.Black), uint8(ink.Dark)
	if s.Focused {
		bg, fg, dim = ink.Black, ink.White, ink.Light
	}
	ink.Fill(o, image.Rectangle{Max: size}, bg)
	ink.Fill(o, image.Rect(0, size.Y-2, size.X, size.Y), ink.Black)
	name, small := th.Face(gtx, s.Look.Name, text.Bold), th.Face(gtx, s.Look.Small, text.Regular)
	lineIn(o, name, s.NameR, s.Name, fg, Left)
	lineIn(o, small, s.StatusR, s.Status, dim, Right)
	for _, b := range s.Boxes {
		if b.R.Empty() {
			continue
		}
		ink.Fill(o, b.R, ink.White)
		ink.Frame(o, b.R, 2, ink.Black)
		m := b.R.Min.Add(b.R.Max).Div(2)
		switch {
		case b.Plus:
			plus(o, m, 9, ink.Black)
		case b.Minus:
			ink.Fill(o, image.Rect(m.X-9, m.Y-1, m.X+10, m.Y+2), ink.Black)
		case b.Icon != nil:
			if b.Label != "" {
				m.X = b.R.Min.X + b.IconX
			}
			icon, at := b.Icon, m
			ink.Draw(o, b.R, func(dst *image.Gray, dr image.Rectangle) { icon(dst, at.Add(dr.Min.Sub(b.R.Min))) })
			if b.Label != "" {
				lineIn(o, small, image.Rect(m.X+b.IconW/2+8, b.R.Min.Y, b.R.Max.X, b.R.Max.Y), b.Label, ink.Black, Left)
			}
		default:
			lineIn(o, small, b.R, b.Label, ink.Black, Center)
		}
	}
	return ink.Dims{Size: size}
}

// WinTab is a tabbed (or stacked) container's tab: the one shown open
// white, black if the focus is in it; the others light.
type WinTab struct {
	Look      FrameLook
	Name      string
	Shown, In bool
}

func (t WinTab) Layout(gtx ink.Ctx, th *Theme) ink.Dims {
	size := gtx.Constraints.Max
	o, r := gtx.Ops, image.Rectangle{Max: size}
	bg, fg := uint8(ink.Light), uint8(ink.Dark)
	switch {
	case t.Shown && t.In:
		bg, fg = ink.Black, ink.White
	case t.Shown:
		bg, fg = ink.White, ink.Black
	}
	ink.Fill(o, r, bg)
	ink.Frame(o, r, 2, ink.Black)
	f := th.Face(gtx, t.Look.Small, text.Regular)
	if t.Shown {
		f = th.Face(gtx, t.Look.Name, text.Bold)
	}
	lineIn(o, f, r.Inset(10), t.Name, fg, Center)
	return ink.Dims{Size: size}
}

// Corner is a note in a corner, over what's there: framed thick, a
// margin round it to stand apart.
type Corner struct {
	Text   string
	Size   ink.Dp // its letters
	Margin int    // white round the frame, px
	Thick  int    // the frame's
}

func (t Corner) Layout(gtx ink.Ctx, th *Theme) ink.Dims {
	size := gtx.Constraints.Max
	o := gtx.Ops
	ink.Fill(o, image.Rectangle{Max: size}, ink.White)
	r := image.Rectangle{Max: size}.Inset(t.Margin)
	ink.Frame(o, r, t.Thick, ink.Black)
	lineIn(o, th.Face(gtx, t.Size, text.Regular), r.Inset(24), t.Text, ink.Black, Left)
	return ink.Dims{Size: size}
}
