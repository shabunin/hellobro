package main

// Facts over the page in ink (info.go): their names and values, copy and
// close; or a cheat sheet (sheet.go), its rows dealt into columns.

import (
	"image"

	"hellobro/ink"
	"hellobro/ink/layout"
	"hellobro/ink/text"
	"hellobro/ink/theme"
	"hellobro/ink/widget"
)

type infoUI struct {
	a             *bro
	q             *infoBox
	copyB, closeB widget.Clickable
	list          layout.Scroll
	act           string // what the frame picked: "copy", "close"
}

func (u *infoUI) Layout(gtx ink.Ctx) ink.Dims {
	q, th := u.q, u.a.theme()
	if u.closeB.Clicked(gtx) {
		u.act = "close"
	}
	if u.copyB.Clicked(gtx) {
		u.act = "copy"
	}
	for _, e := range gtx.Events(ink.Root) {
		switch e := e.(type) {
		case ink.Key:
			if e.Name == "Esc" || e.Name == "Enter" || q.sheet {
				u.act = "close"
			}
		case ink.Text:
			switch {
			case q.sheet, e.Text == "q":
				u.act = "close"
			case e.Text == "c":
				u.act = "copy"
			}
		}
	}
	room := gtx.Constraints.Max.Y
	return card(gtx, th, &q.over.h, func(gtx ink.Ctx) ink.Dims {
		return layout.UniformInset(th.Pad*2.5).Layout(gtx, func(gtx ink.Ctx) ink.Dims {
			inner := room - 2*gtx.Dp(th.Pad*2.5)
			head := func(gtx ink.Ctx) ink.Dims {
				return layout.Inset{Bottom: th.Pad * 1.5}.Layout(gtx, theme.Title(th, q.title).Layout)
			}
			hd, fd := natural(gtx, head), natural(gtx, u.buttons)
			body := inner - hd.Size.Y - fd.Size.Y
			var mid layout.FlexChild
			if q.sheet {
				mid = layout.Rigid(func(gtx ink.Ctx) ink.Dims { return u.sheet(gtx, max(body, 0)) })
			} else {
				mid = layout.Rigid(func(gtx ink.Ctx) ink.Dims { return u.facts(gtx, body) })
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, layout.Rigid(head), mid, layout.Rigid(u.buttons))
		})
	})
}

// natural is how big w would be laid out in gtx, not laid out.
func natural(gtx ink.Ctx, w ink.Widget) ink.Dims {
	m := ink.Record(gtx.Ops)
	d := w(gtx)
	m.Stop()
	return d
}

// buttons are copy and close at the foot; a sheet's close only.
func (u *infoUI) buttons(gtx ink.Ctx) ink.Dims {
	th := u.a.theme()
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	cl := theme.Button(th, &u.closeB, "close  (Esc)")
	cl.Strong = true
	kids := []layout.FlexChild{}
	if u.q.sheet {
		cl.Text = "close  (any key)"
	} else {
		kids = append(kids, layout.Rigid(theme.Button(th, &u.copyB, "copy  (c)").Layout), layout.Rigid(layout.Spacer{Width: th.Pad * 1.5}.Layout))
	}
	kids = append(kids, layout.Rigid(cl.Layout))
	return layout.Inset{Top: th.Pad * 2}.Layout(gtx, func(gtx ink.Ctx) ink.Dims {
		return layout.Flex{Spacing: layout.SpaceStart}.Layout(gtx, kids...)
	})
}

// facts are the rows, a name and its value; in a list if they're more
// than room holds.
func (u *infoUI) facts(gtx ink.Ctx, room int) ink.Dims {
	th := u.a.theme()
	f := th.Face(gtx, th.TextSize, text.Regular)
	nameW := 0
	for _, r := range u.q.rows {
		nameW = max(nameW, text.Width(f, r[0]))
	}
	nameW = min(nameW, gtx.Constraints.Max.X*2/5)
	row := func(r [2]string) ink.Widget {
		return func(gtx ink.Ctx) ink.Dims {
			return layout.Inset{Top: th.Pad / 2, Bottom: th.Pad / 2}.Layout(gtx, func(gtx ink.Ctx) ink.Dims {
				name := theme.Body(th, r[0])
				name.Gray = th.Quiet
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx ink.Ctx) ink.Dims {
						gtx.Constraints = ink.Constraints{Min: image.Pt(nameW, 0), Max: image.Pt(nameW, gtx.Constraints.Max.Y)}
						return name.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Width: th.Pad * 3}.Layout),
					layout.Flexed(1, theme.Body(th, r[1]).Layout))
			})
		}
	}
	all := 0
	for _, r := range u.q.rows {
		all += natural(gtx, row(r)).Size.Y
	}
	if all <= room {
		var kids []layout.FlexChild
		for _, r := range u.q.rows {
			kids = append(kids, layout.Rigid(row(r)))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
	}
	u.list.Axis = layout.Vertical
	gtx.Constraints = ink.Exact(image.Pt(gtx.Constraints.Max.X, max(room, 0)))
	return theme.List(th, &u.list).Layout(gtx, len(u.q.rows), func(gtx ink.Ctx, i int) ink.Dims { return row(u.q.rows[i])(gtx) })
}

// sheet deals the rows into as many columns as the width holds, a line
// each, h high at most.
func (u *infoUI) sheet(gtx ink.Ctx, h int) ink.Dims {
	th := u.a.theme()
	small := th.Face(gtx, th.TextSize*0.9, text.Regular)
	bold := th.Face(gtx, th.TextSize, text.Bold)
	rowH := text.Height(bold) + gtx.Dp(th.Pad)
	w := gtx.Constraints.Max.X
	gap := gtx.Dp(th.Pad * 4)
	colsMax := max(1, (w+gap)/(gtx.Dp(390)+gap))
	cols := sheetLayout(u.q.rows, h/rowH, colsMax)
	n := max(1, min(len(cols), colsMax))
	cw := (w - (n-1)*gap) / n
	for i, col := range cols {
		if i >= n {
			break // a short screen: the rest are left out
		}
		x := i * (cw + gap)
		nameW := 0
		for _, r := range col {
			if r[0] != sheetHead {
				nameW = max(nameW, text.Width(small, r[0]))
			}
		}
		nameW = min(nameW, cw*2/5)
		y := 0
		for _, r := range col {
			if y+rowH > h {
				break
			}
			if r[0] == sheetHead {
				base := y + (rowH-text.Height(bold))/2 + text.Ascent(bold)
				ink.TextAt(gtx.Ops, bold, x, base, text.Fit(bold, r[1], cw), th.Fg)
				ink.Fill(gtx.Ops, image.Rect(x, y+rowH-2, x+cw, y+rowH-1), th.Quiet)
			} else {
				base := y + (rowH-text.Height(small))/2 + text.Ascent(small)
				ink.TextAt(gtx.Ops, small, x, base, text.Fit(small, r[0], nameW), th.Fg)
				vx := x + nameW + gtx.Dp(th.Pad*2)
				ink.TextAt(gtx.Ops, small, vx, base, text.Fit(small, r[1], x+cw-vx), th.Quiet)
			}
			y += rowH
		}
	}
	return ink.Dims{Size: image.Pt(w, h)}
}
