package main

// The switcher and the menus, in ink: the title and its keys, the line
// typed into, the entries that match, as rows (the one the keys are on
// dark) or tiles (it ringed). The keys are pickerKey's, as they were
// (the query, the selection); the card only shows them, and an entry
// tapped picks it, held does its hold, a tile's × tapped its close.
//
// A tile with a paint of its own (a workspace, its windows small) is
// painted once into an image kept till it changes (repaintTiles): every
// frame the card lays out would paint it again otherwise.

import (
	"image"
	"strconv"

	"hellobro/ink"
	"hellobro/ink/layout"
	"hellobro/ink/text"
	"hellobro/ink/theme"
	"hellobro/ink/widget"
	"hellobro/kobo"
)

type pickerUI struct {
	a       *bro
	p       *picker
	rows    []widget.Clickable // the entries, rows or tiles
	closes  []widget.Clickable // the tiles' ×
	list    layout.Scroll
	sel     int                    // the selection last shown: the list goes to it when it moves
	pick    int                    // the entry tapped, -1: none
	held    int                    // the entry held, -1: none
	closed  int                    // the tile whose × was tapped, -1: none
	painted map[string]*image.Gray // the tiles painted their own way, by title
}

// tileGap is the room between tiles, the ring around the one lit in it.
const tileGap = 12

func newPickerUI(a *bro, p *picker) *pickerUI {
	return &pickerUI{a: a, p: p, sel: -1, pick: -1, held: -1, closed: -1, painted: map[string]*image.Gray{}}
}

// take does what the frame picked, after it.
func (u *pickerUI) take() {
	a, p := u.a, u.p
	pick, held, closed := u.pick, u.held, u.closed
	u.pick, u.held, u.closed = -1, -1, -1
	switch {
	case closed >= 0 && closed < len(p.shown) && p.shown[closed].close != nil:
		p.shown[closed].close()
	case held >= 0 && held < len(p.shown) && p.shown[held].hold != nil:
		hold := p.shown[held].hold
		a.unpick()
		hold()
	case held >= 0: // nothing to hold: as a tap
		a.pick(held)
	case pick >= 0:
		a.pick(pick)
	}
}

func (u *pickerUI) Layout(gtx ink.Ctx) ink.Dims {
	p, th := u.p, u.a.theme()
	for len(u.rows) < len(p.shown) {
		u.rows = append(u.rows, widget.Clickable{NoFocus: true})
		u.closes = append(u.closes, widget.Clickable{NoFocus: true})
	}
	for i := range p.shown {
		for {
			c, ok := u.rows[i].Update(gtx)
			if !ok {
				break
			}
			if c.Long {
				u.held = i
			} else {
				u.pick = i
			}
		}
		if u.closes[i].Clicked(gtx) {
			u.closed = i
		}
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
	switch {
	case p.keysOnly:
		keys = p.hint
	case p.tiles > 0:
		keys = "tap one, or the arrows and Enter · Esc closes"
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
	if p.tiles > 0 {
		return u.tiles(gtx)
	}
	if u.sel != p.sel {
		u.sel = p.sel
		u.list.ScrollTo(p.sel)
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

// tiles are the entries that match as tiles, so many across (or for
// the whole screen as many as makes them biggest), in a list that
// follows the selection.
func (u *pickerUI) tiles(gtx ink.Ctx) ink.Dims {
	p, th := u.p, u.a.theme()
	w, room := gtx.Constraints.Max.X, gtx.Constraints.Max.Y
	ring := tileGap / 2 // the room the ring takes at the edges
	cols, tw, tileH := p.tiles, (w-2*ring-(p.tiles-1)*tileGap)/p.tiles, p.tileHeight()
	if p.wide {
		cols, tw, tileH = wideTiles(len(p.shown), w-2*ring, room-2*ring, u.a.wm.area)
	}
	left := ring + (w-2*ring-cols*tw-(cols-1)*tileGap)/2 // narrower: centered
	rects := make([]image.Rectangle, len(p.shown))
	for n := range rects {
		x, y := left+n%cols*(tw+tileGap), ring+n/cols*(tileH+tileGap)
		rects[n] = image.Rect(x, y, x+tw, y+tileH)
	}
	h := rects[len(rects)-1].Max.Y + ring
	if u.sel != p.sel {
		u.sel = p.sel
		if p.sel < len(rects) {
			r := rects[p.sel]
			u.list.RevealSpan(r.Min.Y-ring, r.Max.Y+ring)
		}
	}
	u.list.Axis = layout.Vertical
	return theme.List(th, &u.list).Layout(gtx, 1, func(gtx ink.Ctx, _ int) ink.Dims {
		for n, r := range rects {
			u.tile(gtx, n, r)
			if n == p.sel { // the keys' choice: ringed, as dark is on
				ink.Frame(gtx.Ops, r.Inset(-ring), 4, th.Fg)
			}
		}
		return ink.Dims{Size: image.Pt(w, h)}
	})
}

// tile is entry n in r: painted its own way, or an icon and its words;
// on, dark; held down, inverted. A tile that closes has its × over it.
func (u *pickerUI) tile(gtx ink.Ctx, n int, r image.Rectangle) {
	p, th := u.p, u.a.theme()
	e, clk := p.shown[n], &u.rows[n]
	st := ink.Offset(gtx.Ops, r.Min)
	defer st.Pop()
	cg := gtx
	cg.Constraints = ink.Exact(r.Size())
	clk.Layout(cg, func(gtx ink.Ctx) ink.Dims {
		size := r.Size()
		if e.paint == nil {
			num := 0
			if p.query == "" && n < 9 {
				num = n + 1
			}
			return inkTile(gtx, th, launchTile{name: e.title, note: e.detail, icon: e.icon, num: num}, e.on != clk.Pressed(), false)
		}
		img := u.painted[e.title]
		if img == nil || img.Rect.Size() != size {
			img = kobo.Blank(size)
			e.paint(kobo.Canvas{Gray: img}, img.Rect)
			u.painted[e.title] = img
		}
		ink.Image(gtx.Ops, image.Rectangle{Max: size}, img, image.Point{})
		if clk.Pressed() {
			ink.Invert(gtx.Ops, image.Rectangle{Max: size})
		}
		return ink.Dims{Size: size}
	})
	if e.close != nil {
		cr := tileClose(image.Rectangle{Max: r.Size()})
		xs := ink.Offset(gtx.Ops, cr.Min)
		cg.Constraints = ink.Exact(cr.Size())
		u.closes[n].Layout(cg, func(gtx ink.Ctx) ink.Dims { return ink.Dims{Size: cr.Size()} })
		xs.Pop()
	}
}
