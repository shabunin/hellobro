package main

// The AI letters' page, in a notebook's settings: how big the answers
// are written (a slider: every notebook's), and this notebook's font,
// each shown in itself at that size, in Latin, Cyrillic and Greek.

import (
	"image"
	"strconv"

	"hellobro/hand"
	"hellobro/ink"
	"hellobro/ink/layout"
	"hellobro/ink/text"
	"hellobro/ink/theme"
	"hellobro/ink/widget"
)

// lettersSample is what each font shows itself by.
const lettersSample = "Quick brown fox · Съешь ещё булок · Γαζέες καὶ μυρτιές"

type lettersUI struct {
	back  widget.Clickable
	size  widget.Float
	held  bool
	fonts []widget.Clickable
}

// lettersOn shows the page.
func (w *noteWin) lettersOn(a *bro) {
	u := w.setOf(a)
	u.letters, u.list = true, layout.Scroll{}
	w.showSettings(a)
}

// pxOfPt is a notebook's points on the screen: a page as wide as it is
// (157 mm) at 227 dpi.
func pxOfPt(pt float64) int { return int(pt*227/72 + 0.5) }

func (u *noteSetUI) lettersLayout(gtx ink.Ctx, hs libHeadSpec) ink.Dims {
	a, w, th := u.a, u.w, u.a.theme()
	l := &u.lt
	fonts := diaryFonts()
	for len(l.fonts) < len(fonts) {
		l.fonts = append(l.fonts, widget.Clickable{})
	}
	si := settingIndex("ai-size")
	st := settingList[si]
	l.size.Min, l.size.Max, l.size.Step = 0, float32(len(st.values)-1), 1
	// Picked when the finger lifts, as the Settings' sliders are; the
	// samples follow it meanwhile.
	if l.size.Changed(gtx) {
		l.held = true
	}
	if l.held && !l.size.Dragging() {
		l.held = false
		if v := int(l.size.Value + 0.5); v != a.settingAt(si) {
			u.do = func() { a.pickSetting(si, v); w.showSettings(a) }
		}
	}
	if !l.held {
		l.size.Value = float32(a.settingAt(si))
	}
	k := min(max(int(l.size.Value+0.5), 0), len(st.values)-1)
	pt, _ := strconv.ParseFloat(st.values[k], 64)
	if l.back.Clicked(gtx) {
		u.do = func() { u.letters, u.list = false, layout.Scroll{}; w.showSettings(a) }
	}
	cur := w.nb.diaryFont()
	for i, f := range fonts {
		if l.fonts[i].Clicked(gtx) {
			u.do = func() { w.setDiaryFont(f); w.showSettings(a) }
		}
	}

	size := gtx.Constraints.Max
	ink.Fill(gtx.Ops, image.Rectangle{Max: size}, th.Bg)
	rowH := libRowHeight(gtx, th)
	rows := []ink.Widget{
		func(gtx ink.Ctx) ink.Dims {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(theme.Button(th, &l.back, "‹ settings").Layout),
				layout.Flexed(1, func(gtx ink.Ctx) ink.Dims {
					return layout.Inset{Left: th.Pad * 2}.Layout(gtx, theme.Title(th, "AI letters").Layout)
				}))
		},
		func(gtx ink.Ctx) ink.Dims {
			sl := theme.Slider(th, &l.size, "Size (every notebook's answers, from the next)", st.shown[k])
			sl.Ticks = true
			return layout.Inset{Top: th.Pad}.Layout(gtx, sl.Layout)
		},
		func(gtx ink.Ctx) ink.Dims {
			return layout.Inset{Top: th.Pad, Left: th.Pad, Bottom: th.Pad / 2}.Layout(gtx, theme.Small(th, "this notebook's font: a tap picks it; each is shown as an answer would be").Layout)
		},
	}
	for i, f := range fonts {
		rows = append(rows, func(gtx ink.Ctx) ink.Dims { return u.letterRow(gtx, th, &l.fonts[i], f, f == cur, pt) })
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

// letterRow is a font: its name, and the sample in it at pt points
// (the hand, which is drawn, says so); framed if it's the one.
func (u *noteSetUI) letterRow(gtx ink.Ctx, th *theme.Theme, clk *widget.Clickable, fam string, on bool, pt float64) ink.Dims {
	pad := gtx.Dp(th.Pad)
	small := th.Face(gtx, th.TextSize*0.8, text.Regular)
	name := fam
	f := noteFontFace(fam, pxOfPt(pt))
	w := gtx.Constraints.Max.X
	sh := text.Height(f)
	var hw hand.Writing
	if fam == diaryHand { // drawn with the pen: its strokes
		name = "handwriting: the Hershey script, plain text only"
		if h, err := hand.Script(); err == nil {
			size := float64(pxOfPt(pt)) * 0.7 // a capital's height
			hw = h.Write(lettersSample, size, 1e6, 1)
			sh = int(hand.LineHeight(size))
		}
	}
	if on {
		name = "● " + name
	}
	h := pad + text.Height(small) + pad/2 + sh + pad
	cg := gtx
	cg.Constraints = ink.Exact(image.Pt(w, h))
	return clk.Layout(cg, func(gtx ink.Ctx) ink.Dims {
		r := image.Rectangle{Max: image.Pt(w, h)}
		ink.Fill(gtx.Ops, r, th.Bg)
		y := pad + text.Ascent(small)
		ink.TextAt(gtx.Ops, small, pad, y, name, th.Fg)
		y += small.Metrics().Descent.Ceil() + pad/2 + text.Ascent(f)
		if fam != diaryHand {
			ink.TextAt(gtx.Ops, f, pad, y, text.Fit(f, lettersSample, w-2*pad), th.Fg)
		}
		top := y - text.Ascent(f) + pad/2
		lw := max(gtx.Dp(1.5), 2)
		for _, wd := range hw.Words {
			if pad+int(wd.Max.X) > w-pad {
				break // as much as fits
			}
			for _, st := range wd.Strokes {
				pts := make([]image.Point, len(st))
				for i, p := range st {
					pts[i] = image.Pt(pad+int(p.X), top+int(p.Y))
				}
				ink.Line(gtx.Ops, lw, th.Fg, pts...)
			}
		}
		ink.Fill(gtx.Ops, image.Rect(0, h-1, w, h), th.Quiet)
		if on {
			ink.Frame(gtx.Ops, r, max(gtx.Dp(2), 2), th.Fg)
		}
		if clk.Pressed() {
			ink.Invert(gtx.Ops, r)
		}
		return ink.Dims{Size: r.Size()}
	})
}
