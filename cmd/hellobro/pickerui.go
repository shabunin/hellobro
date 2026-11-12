package main

// The switcher and the menus of rows, in ink: the title and its keys, the
// line typed into, the entries that match, the one the keys are on dark.
// The keys are pickerKey's, as they were (the query, the selection); the
// card only shows them, and a row tapped picks it.

import (
	"image"
	"strconv"

	"hellobro/ink"
	"hellobro/ink/layout"
	"hellobro/ink/text"
	"hellobro/ink/theme"
	"hellobro/ink/widget"
)

type pickerUI struct {
	a    *bro
	p    *picker
	rows []widget.Clickable
	list layout.Scroll
	sel  int // the selection last shown: the list goes to it when it moves
	pick int // the row tapped, -1: none
}

func newPickerUI(a *bro, p *picker) *pickerUI {
	return &pickerUI{a: a, p: p, sel: -1, pick: -1}
}

func (u *pickerUI) Layout(gtx ink.Ctx) ink.Dims {
	p, th := u.p, u.a.theme()
	for len(u.rows) < len(p.shown) {
		u.rows = append(u.rows, widget.Clickable{NoFocus: true})
	}
	for i := range p.shown {
		if u.rows[i].Clicked(gtx) {
			u.pick = i
		}
	}
	if u.sel != p.sel {
		u.sel = p.sel
		u.list.ScrollTo(p.sel)
	}
	var h int
	return card(gtx, th, &h, func(gtx ink.Ctx) ink.Dims {
		room := gtx.Constraints.Max.X
		size := image.Pt(room, p.box.Dy())
		gtx.Constraints = ink.Exact(size)
		return layout.Inset{Left: th.Pad * 1.5, Right: th.Pad * 1.5, Top: th.Pad, Bottom: th.Pad}.Layout(gtx, func(gtx ink.Ctx) ink.Dims {
			var kids []layout.FlexChild
			if p.title != "" {
				kids = append(kids, layout.Rigid(u.head))
			}
			if !p.keysOnly {
				kids = append(kids, layout.Rigid(u.line), layout.Rigid(layout.Spacer{Height: th.Pad}.Layout))
			}
			kids = append(kids, layout.Flexed(1, u.entries))
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
		})
	})
}

// head is the title, and at its end the keys it takes.
func (u *pickerUI) head(gtx ink.Ctx) ink.Dims {
	p, th := u.p, u.a.theme()
	keys := "type to narrow · ↑ ↓ Enter · Esc closes"
	if p.keysOnly {
		keys = p.hint
	}
	big := th.Face(gtx, th.TextSize*1.3, text.Bold)
	small := th.Face(gtx, th.TextSize*0.85, text.Regular)
	w := gtx.Constraints.Max.X
	h := text.Height(big) + gtx.Dp(th.Pad)
	base := (h-text.Height(big))/2 + text.Ascent(big)
	kw := min(text.Width(small, keys), w/2)
	ink.TextAt(gtx.Ops, big, 0, base, text.Fit(big, p.title, w-kw-gtx.Dp(th.Pad*2)), th.Fg)
	ink.TextAt(gtx.Ops, small, w-kw, base, text.Fit(small, keys, kw), th.Quiet)
	return ink.Dims{Size: image.Pt(w, h)}
}

// line is what's typed, a caret after it; the hint while it's empty.
func (u *pickerUI) line(gtx ink.Ctx) ink.Dims {
	p, th := u.p, u.a.theme()
	f := th.Face(gtx, th.TextSize, text.Regular)
	small := th.Face(gtx, th.TextSize*0.85, text.Regular)
	w, h := gtx.Constraints.Max.X, gtx.Dp(th.Touch)
	r := image.Rect(0, 0, w, h)
	ink.Fill(gtx.Ops, r, th.Bg)
	ink.Frame(gtx.Ops, r, max(gtx.Dp(1), 2), th.Fg)
	pad := gtx.Dp(th.Pad * 1.5)
	base := (h-text.Height(f))/2 + text.Ascent(f)
	q := "› " + p.query
	ink.TextAt(gtx.Ops, f, pad, base, text.Fit(f, q, w-2*pad), th.Fg)
	cx := pad + text.Width(f, q) + gtx.Dp(2)
	ink.Fill(gtx.Ops, image.Rect(cx, h/4, cx+max(gtx.Dp(2), 3), h-h/4), th.Fg) // the caret
	if p.query == "" && p.hint != "" {
		hw := min(text.Width(small, p.hint), w/2)
		ink.TextAt(gtx.Ops, small, w-pad-hw, (h-text.Height(small))/2+text.Ascent(small), text.Fit(small, p.hint, hw), th.Quiet)
	}
	return ink.Dims{Size: r.Size()}
}

// entries are the rows that match, in a list that follows the selection.
func (u *pickerUI) entries(gtx ink.Ctx) ink.Dims {
	p, th := u.p, u.a.theme()
	if len(p.shown) == 0 {
		l := theme.Body(th, "nothing matches")
		l.Gray = th.Quiet
		return layout.Inset{Left: th.Pad * 1.5, Top: th.Pad}.Layout(gtx, l.Layout)
	}
	u.list.Axis = layout.Vertical
	return theme.List(th, &u.list).Layout(gtx, len(p.shown), func(gtx ink.Ctx, n int) ink.Dims {
		return u.row(gtx, n)
	})
}

// row is entry n: its number (1-9 go to it), its title, what it is.
func (u *pickerUI) row(gtx ink.Ctx, n int) ink.Dims {
	p, th := u.p, u.a.theme()
	e := p.shown[n]
	return u.rows[n].Layout(gtx, func(gtx ink.Ctx) ink.Dims {
		w, h := gtx.Constraints.Max.X, pickRowH
		r := image.Rect(0, 0, w, h)
		fg, dim, bg := th.Fg, th.Quiet, th.Bg
		if n == p.sel || u.rows[n].Pressed() {
			fg, dim, bg = th.Bg, th.Bg, th.Fg
		}
		ink.Fill(gtx.Ops, r, bg)
		body := th.Face(gtx, th.TextSize, text.Regular)
		bold := th.Face(gtx, th.TextSize*1.1, text.Bold)
		small := th.Face(gtx, th.TextSize*0.85, text.Regular)
		pad := gtx.Dp(th.Pad * 1.5)
		base := (h-text.Height(body))/2 + text.Ascent(body)
		x := pad
		if p.query == "" && p.extra == nil && n < 9 {
			ink.TextAt(gtx.Ops, bold, x, base, strconv.Itoa(n+1), fg)
		}
		x += text.Width(bold, "8") + pad
		kw := text.Width(small, "workspace")
		ink.TextAt(gtx.Ops, small, w-pad-text.Width(small, e.kind), base, e.kind, dim)
		room := w - pad - kw - pad - x
		title := e.title
		if e.current {
			title += "  ●"
		}
		title = text.Fit(body, title, room)
		ink.TextAt(gtx.Ops, body, x, base, title, fg)
		x += text.Width(body, title) + pad*2
		if left := w - pad - kw - pad - x; left > gtx.Dp(30) && e.detail != "" {
			ink.TextAt(gtx.Ops, small, x, base, text.Fit(small, e.detail, left), dim)
		}
		return ink.Dims{Size: r.Size()}
	})
}

// picked is the row tapped, taken; ok false if none was.
func (u *pickerUI) picked() (int, bool) {
	n := u.pick
	u.pick = -1
	return n, n >= 0
}
