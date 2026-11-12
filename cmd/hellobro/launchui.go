package main

// The launcher's tiles in ink (launcher.go has what they are): in groups
// under their headings, as tall as there's room for, in a list a finger
// scrolls if they don't all fit; under them how the last thing ended,
// the find> line and the keys. The arrows (menuKey) light a tile, which
// the list shows; a tile tapped opens after the frame.

import (
	"fmt"
	"image"

	"hellobro/ink"
	"hellobro/ink/inkkobo"
	"hellobro/ink/layout"
	"hellobro/ink/text"
	"hellobro/ink/theme"
	"hellobro/ink/widget"
	"hellobro/kobo"
)

type launchUI struct {
	a     *bro
	t     *termSession
	pane  *inkkobo.Pane
	tiles []widget.Clickable
	field widget.Clickable
	list  layout.Scroll
	shown int // the tile lit last shown: the list goes to it when it moves
	pick  int // the tile tapped, -1: none
	asked bool
}

// launchOf is t's launcher pane, made the first time.
func (a *bro) launchOf(t *termSession) *launchUI {
	if t.pane.launch == nil {
		u := &launchUI{a: a, t: t, shown: -1, pick: -1}
		u.pane = inkkobo.NewPane(u.Layout)
		t.pane.launch = u
	}
	return t.pane.launch
}

// take does what the frame picked: a tile, or the keyboard on the
// screen for the find> line.
func (u *launchUI) take() {
	a, t, m := u.a, u.t, &u.t.pane.menu
	if n := u.pick; n >= 0 {
		u.pick = -1
		if n < len(m.tiles) {
			a.launchDo(t, m.tiles[n].do)()
		}
	}
	if u.asked {
		u.asked = false
		m.asked = true
		a.vkAsk(true)
	}
}

// shows says the pane is on the screen, the tiles in it.
func (u *launchUI) shows(a *bro) bool {
	return !u.pane.At.Empty() && launchMenu(u.t) && !u.t.connected() && a.onScreen(u.t)
}

func (u *launchUI) Layout(gtx ink.Ctx) ink.Dims {
	a, t, th := u.a, u.t, u.a.theme()
	m := &t.pane.menu
	gs := a.launchGroups(t)
	var tiles []launchTile
	var ns []int
	for _, g := range gs {
		ns = append(ns, len(g.tiles))
		tiles = append(tiles, g.tiles...)
	}
	for len(u.tiles) < len(tiles) {
		u.tiles = append(u.tiles, widget.Clickable{NoFocus: true})
	}
	for i := range tiles {
		if u.tiles[i].Clicked(gtx) {
			u.pick = i
		}
	}
	if u.field.Clicked(gtx) {
		u.asked = true
	}
	m.tiles = tiles
	size := gtx.Constraints.Max
	ink.Fill(gtx.Ops, image.Rectangle{Max: size}, th.Bg)
	return layout.UniformInset(th.Pad).Layout(gtx, func(gtx ink.Ctx) ink.Dims {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Flexed(1, func(gtx ink.Ctx) ink.Dims { return u.grid(gtx, gs, ns, tiles) }),
			layout.Rigid(u.foot))
	})
}

// grid lays the groups' tiles out, as tall as there's room for, down to
// a finger's; else their names alone; scrolled if they still don't fit.
func (u *launchUI) grid(gtx ink.Ctx, gs []launchGroup, ns []int, tiles []launchTile) ink.Dims {
	th, m := u.a.theme(), &u.t.pane.menu
	w, room := gtx.Constraints.Max.X, gtx.Constraints.Max.Y
	small := th.Face(gtx, th.TextSize*0.85, text.Regular)
	headH := text.Height(small) + gtx.Dp(th.Pad*1.2)
	gap := gtx.Dp(th.Pad)
	minW := gtx.Dp(launchTileMin / 1.42)
	compact := false
	var heads []image.Rectangle
	var rects [][]image.Rectangle
	var h int
	for tileH := gtx.Dp(105); ; tileH -= gtx.Dp(6) {
		heads, rects, h = menuLayout(w, ns, minW, tileH, headH, gap)
		if h <= room || tileH <= gtx.Dp(73) {
			break
		}
	}
	if h > room {
		compact = true
		heads, rects, h = menuLayout(w, ns, minW, gtx.Dp(51), headH, gap)
	}
	var flat []image.Rectangle
	for _, rs := range rects {
		flat = append(flat, rs...)
	}
	m.rects = flat
	if m.lit >= len(flat) {
		m.lit = len(flat) - 1
	}
	if m.lit != u.shown {
		u.shown = m.lit
		if m.lit >= 0 {
			u.list.RevealSpan(flat[m.lit].Min.Y, flat[m.lit].Max.Y)
		} else {
			u.list.ScrollTo(0)
		}
	}
	u.list.Axis = layout.Vertical
	return theme.List(th, &u.list).Layout(gtx, 1, func(gtx ink.Ctx, _ int) ink.Dims {
		for i, g := range gs {
			r := heads[i]
			base := r.Min.Y + (r.Dy()-text.Height(small))/2 + text.Ascent(small) - gtx.Dp(2)
			ink.TextAt(gtx.Ops, small, r.Min.X, base, text.Fit(small, g.name, r.Dx()), th.Quiet)
			ink.Fill(gtx.Ops, image.Rect(r.Min.X, r.Max.Y-2, r.Max.X, r.Max.Y-1), ink.Light)
		}
		for i, r := range flat {
			st := ink.Offset(gtx.Ops, r.Min)
			cg := gtx
			cg.Constraints = ink.Exact(r.Size())
			tl, clk := tiles[i], &u.tiles[i]
			clk.Layout(cg, func(gtx ink.Ctx) ink.Dims {
				return inkTile(gtx, th, tl, i == m.lit || clk.Pressed(), compact)
			})
			st.Pop()
		}
		return ink.Dims{Size: image.Pt(w, h)}
	})
}

// inkTile is a tile, as big as it's given: its icon, its name, a word
// on it, its number; lit, black. The launcher's and the quick menu's.
func inkTile(gtx ink.Ctx, th *theme.Theme, tl launchTile, lit, compact bool) ink.Dims {
	size := gtx.Constraints.Min
	r := image.Rectangle{Max: size}
	fg, bg, quiet := th.Fg, th.Bg, th.Quiet
	if lit {
		fg, bg, quiet = th.Bg, th.Fg, ink.Light
	}
	ink.Fill(gtx.Ops, r, bg)
	ink.Frame(gtx.Ops, r, max(gtx.Dp(1), 2), th.Fg)
	pad := gtx.Dp(th.Pad)
	side := min(size.Y-2*pad, gtx.Dp(56))
	ir := image.Rect(pad, (size.Y-side)/2, pad+side, (size.Y+side)/2)
	ink.Image(gtx.Ops, ir, iconImage(tl.icon, side, fg, bg), image.Point{})
	body := th.Face(gtx, th.TextSize, text.Regular)
	small := th.Face(gtx, th.TextSize*0.85, text.Regular)
	x := ir.Max.X + pad
	right := size.X - pad
	if tl.num > 0 {
		n := fmt.Sprint(tl.num)
		ink.TextAt(gtx.Ops, small, right-text.Width(small, n), pad/2+text.Ascent(small), n, quiet)
		right -= text.Width(small, "9") + pad
	}
	if compact || tl.note == "" {
		ink.TextAt(gtx.Ops, body, x, (size.Y-text.Height(body))/2+text.Ascent(body), text.Fit(body, tl.name, right-x), fg)
		return ink.Dims{Size: size}
	}
	both := text.Height(body) + text.Height(small)
	y := (size.Y - both) / 2
	ink.TextAt(gtx.Ops, body, x, y+text.Ascent(body), text.Fit(body, tl.name, right-x), fg)
	ink.TextAt(gtx.Ops, small, x, y+text.Height(body)+text.Ascent(small), text.Fit(small, tl.note, size.X-pad-x), quiet)
	return ink.Dims{Size: size}
}

// foot is how the last thing ended, the find> line, the keys.
func (u *launchUI) foot(gtx ink.Ctx) ink.Dims {
	a, t, th := u.a, u.t, u.a.theme()
	small := th.Face(gtx, th.TextSize*0.85, text.Regular)
	body := th.Face(gtx, th.TextSize, text.Regular)
	var kids []layout.FlexChild
	notes := t.notes
	if len(notes)*text.Height(small) > gtx.Constraints.Max.Y/3 { // a small window: the notes are in the log
		notes = nil
	}
	for _, n := range notes {
		kids = append(kids, layout.Rigid(theme.Label(th, th.TextSize*0.85, n).Layout))
	}
	kids = append(kids, layout.Rigid(layout.Spacer{Height: th.Pad}.Layout), layout.Rigid(func(gtx ink.Ctx) ink.Dims {
		w, h := gtx.Constraints.Max.X, gtx.Dp(th.Touch)
		return u.field.Layout(gtx, func(gtx ink.Ctx) ink.Dims {
			r := image.Rect(0, 0, w, h)
			ink.Fill(gtx.Ops, r, th.Bg)
			ink.Frame(gtx.Ops, r, max(gtx.Dp(1), 2), th.Fg)
			pad := gtx.Dp(th.Pad * 1.5)
			prompt := "find>"
			if t.input != "" && (launchQuery(t) == "" || len(t.pane.menu.tiles) == 0) {
				prompt = "ssh>"
			}
			base := (h-text.Height(body))/2 + text.Ascent(body)
			ink.TextAt(gtx.Ops, small, pad, base, prompt, th.Quiet)
			x := pad + text.Width(small, "find>") + pad
			if t.input == "" {
				hint := "type to find an app, a book, a notebook; user@host[:port] [command]: log in"
				ink.TextAt(gtx.Ops, small, x, base, text.Fit(small, hint, w-pad-x), th.Quiet)
			} else {
				s := text.Fit(body, t.input, w-pad-x-gtx.Dp(4))
				ink.TextAt(gtx.Ops, body, x, base, s, th.Fg)
				cx := x + text.Width(body, s) + gtx.Dp(1)
				ink.Fill(gtx.Ops, image.Rect(cx, h/4, cx+max(gtx.Dp(2), 3), h-h/4), th.Fg) // the caret
			}
			if u.field.Pressed() {
				ink.Invert(gtx.Ops, r)
			}
			return ink.Dims{Size: r.Size()}
		})
	}))
	keys := "1–9, or ↑ ↓ ← → and Enter   " + a.keyHints("launcher", "another window", "palette", "the switcher", "kill", "close it") + "   Esc: close this window"
	kids = append(kids, layout.Rigid(layout.Spacer{Height: th.Pad / 2}.Layout), layout.Rigid(func(gtx ink.Ctx) ink.Dims {
		w := gtx.Constraints.Max.X
		h := text.Height(small)
		ink.TextAt(gtx.Ops, small, 0, text.Ascent(small), text.Fit(small, keys, w), th.Quiet)
		return ink.Dims{Size: image.Pt(w, h)}
	}))
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
}

// iconImage is an app's icon drawn side square, fg on bg: drawn once.
func iconImage(id string, side int, fg, bg uint8) *image.Gray {
	k := fmt.Sprint(id, side, fg, bg)
	if img, ok := icons[k]; ok {
		return img
	}
	img := image.NewGray(image.Rect(0, 0, side, side))
	c := kobo.Canvas{Gray: img}
	c.Fill(img.Bounds(), bg)
	drawIcon(c, appIcon(id), img.Bounds(), fg, bg)
	icons[k] = img
	return img
}

var icons = map[string]*image.Gray{}
