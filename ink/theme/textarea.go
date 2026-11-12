package theme

import (
	"image"

	"hellobro/ink"
	"hellobro/ink/text"
	"hellobro/ink/widget"
)

// TextAreaStyle is a TextArea in a frame filling what it's given, a
// toolbar of the paragraphs' kinds over it (the caret's kind black), the
// text moving under the finger and after the caret.
type TextAreaStyle struct {
	Theme *Theme
	Area  *widget.TextArea
	Hint  string // said while it's empty
	Tools bool   // the toolbar
}

// TextArea is a as a framed text with its toolbar.
func TextArea(th *Theme, a *widget.TextArea) TextAreaStyle {
	return TextAreaStyle{Theme: th, Area: a, Tools: true}
}

var kindNames = [...]struct {
	kind widget.Para
	name string
}{
	{widget.Plain, "Text"}, {widget.Title, "Title"}, {widget.Head, "Heading"},
	{widget.Bullet, "• List"}, {widget.Todo, "To-do"}, {widget.Quote, "Quote"},
}

// Look is how th shows a TextArea's paragraphs.
func (th *Theme) Look(gtx ink.Ctx) widget.AreaLook {
	var l widget.AreaLook
	for p := range l.Faces {
		l.Faces[p] = th.Face(gtx, th.TextSize, text.Regular)
	}
	l.Faces[widget.Title] = th.Face(gtx, th.TextSize*1.6, text.Bold)
	l.Faces[widget.Head] = th.Face(gtx, th.TextSize*1.25, text.Bold)
	l.Faces[widget.Quote] = th.Face(gtx, th.TextSize, text.Italic)
	l.Fg, l.Quiet = th.Fg, th.Quiet
	l.Indent = gtx.Dp(th.TextSize * 1.6)
	l.Gap = gtx.Dp(th.Pad * 0.75)
	l.Stroke = max(gtx.Dp(th.Border), 2)
	return l
}

// Layout lays it out.
func (s TextAreaStyle) Layout(gtx ink.Ctx) ink.Dims {
	th, a := s.Theme, s.Area
	a.Update(gtx)
	size := gtx.Constraints.Max
	pad := gtx.Dp(th.Pad * 1.5)
	b := th.border(gtx)
	y := 0
	if s.Tools {
		f := th.Face(gtx, th.TextSize*0.85, text.Regular)
		h := gtx.Dp(th.Touch * 0.85)
		x := 0
		kind := a.Kind()
		for _, k := range kindNames {
			w := text.Width(f, k.name) + 2*pad
			on := kind == k.kind || kind == widget.Done && k.kind == widget.Todo
			clk := &a.Kinds[k.kind]
			st := ink.Offset(gtx.Ops, image.Pt(x, 0))
			cg := gtx
			cg.Constraints = ink.Exact(image.Pt(w, h))
			clk.Layout(cg, func(gtx ink.Ctx) ink.Dims {
				r := image.Rect(0, 0, w, h)
				ink.Fill(gtx.Ops, r, th.Bg)
				ink.Frame(gtx.Ops, r, b, th.Fg)
				line(gtx, f, k.name, th.Fg, Center, h)
				if on != clk.Pressed() {
					ink.Invert(gtx.Ops, r)
				}
				return ink.Dims{Size: r.Size()}
			})
			st.Pop()
			x += w - b // their frames shared
		}
		y = h + gtx.Dp(th.Pad)
	}
	box := image.Rect(0, y, size.X, size.Y)
	ink.Fill(gtx.Ops, box, th.Bg)
	ink.Frame(gtx.Ops, box, b+boolInt(a.Focused()), th.Fg)
	in := box.Inset(pad)
	st := ink.Offset(gtx.Ops, in.Min)
	cg := gtx
	cg.Constraints = ink.Exact(in.Size())
	look := th.Look(gtx)
	List(th, &a.Scroll).Layout(cg, 1, func(gtx ink.Ctx, _ int) ink.Dims {
		gtx.Constraints.Min.Y = in.Dy()
		return a.Layout(gtx, look, in.Dy())
	})
	if a.Text() == "" && s.Hint != "" {
		f := th.Face(gtx, th.TextSize, text.Regular)
		ink.TextAt(gtx.Ops, f, 0, text.Ascent(f), s.Hint, ink.Mid)
	}
	st.Pop()
	// The keys moved the caret: the text follows it, this frame.
	if r, moved := a.CaretRect(); moved {
		a.Scroll.RevealSpan(r.Min.Y-pad, r.Max.Y+pad)
		gtx.Invalidate()
	}
	return ink.Dims{Size: size}
}
