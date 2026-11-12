package main

// An empty workspace, in ink: what to do on it, tap it or the keys, as
// many of them as fit, in a table in the middle. It's only painted: a
// tap on it is wmTap's (a new window).

import (
	"image"

	"hellobro/ink"
	"hellobro/ink/inkkobo"
	"hellobro/ink/text"
)

type emptyUI struct {
	a    *bro
	pane *inkkobo.Pane
	help [][2]string
}

// drawEmpty paints the empty workspace's words over its area.
func (a *bro) drawEmpty() {
	r := a.wm.area.Inset(wmGap)
	u := a.wm.emptyUI
	if u == nil {
		u = &emptyUI{a: a}
		a.wm.emptyUI = u
	}
	u.help = a.emptyHelp()
	a.inkPaint(&u.pane, r, u.Layout)
	a.wm.empty = r
}

func (u *emptyUI) Layout(gtx ink.Ctx) ink.Dims {
	th := u.a.theme()
	size := gtx.Constraints.Max
	ink.Fill(gtx.Ops, image.Rectangle{Max: size}, th.Bg)
	head := th.Face(gtx, th.TextSize*1.2, text.Bold)
	small := th.Face(gtx, th.TextSize*0.85, text.Regular)
	pad := gtx.Dp(th.Pad)
	rowH := text.Height(small) + pad
	say := text.Fit(head, "Nothing here yet: tap it for a new window, or the keys.", size.X-2*pad)
	h := text.Height(head) + 2*pad + len(u.help)*rowH
	y := max(0, (size.Y-h)/3)
	ink.TextAt(gtx.Ops, head, (size.X-text.Width(head, say))/2, y+text.Ascent(head), say, ink.Dark)
	y += text.Height(head) + 2*pad
	keyW, whatW := 0, 0
	for _, l := range u.help {
		keyW = max(keyW, text.Width(small, l[0]))
		whatW = max(whatW, text.Width(small, l[1]))
	}
	keyW = min(keyW, size.X/2-pad)
	gap := 2 * pad
	left := max(pad, (size.X-keyW-gap-whatW)/2) // the table in the middle
	for _, l := range u.help {
		if y+rowH > size.Y {
			break
		}
		k := text.Fit(small, l[0], keyW)
		base := y + (rowH-text.Height(small))/2 + text.Ascent(small)
		ink.TextAt(gtx.Ops, small, left+keyW-text.Width(small, k), base, k, th.Fg)
		ink.TextAt(gtx.Ops, small, left+keyW+gap, base, text.Fit(small, l[1], size.X-left-keyW-gap-pad), ink.Dark)
		y += rowH
	}
	return ink.Dims{Size: size}
}
