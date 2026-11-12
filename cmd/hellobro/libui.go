package main

// The lists of them all, in ink (library.go has what's in them): the
// books' shelf, a book's contents and bookmarks, a PDF's pages small (a
// grid), the notebooks. The
// header, the tools under it, the rows, a name typed in its row. The
// keys are the windows' own, as they were: they move sel, and the list
// follows it; a finger scrolls it. What a tap picked is done after the
// frame (take): what it does draws the window again.

import (
	"image"

	"hellobro/ink"
	"hellobro/ink/inkkobo"
	"hellobro/ink/layout"
	"hellobro/ink/text"
	"hellobro/ink/theme"
	"hellobro/ink/widget"
)

// libHeadSpec is a header: at the left the way to them all (here: we're
// there), at the right the way back, the tabs between.
type libHeadSpec struct {
	all    string
	here   bool
	toAll  func()
	back   string // "": none
	toBack func()
	tabs   []libTab
}

// libSpec is what a list shows, as its window has it now.
type libSpec struct {
	id       string // which list: another, and it's shown from its start
	head     libHeadSpec
	tools    []libTab // under the header: what's made here
	toolNote string
	items    []libItem
	sel      *int
	empty    string // what it says with no items
	edit     int    // the row whose name is typed, -1 none
	prompt   string
	typed    string
	editOK   func()
	editNo   func()
	buttons  func(i int, it libItem) []libTab
	open     func(i int)
	hold     func(i int)      // a row held: picked (nil: as a tap)
	picking  bool             // the rows have boxes to tick
	picked   func(i int) bool // ticked
	shown    *int             // how many rows show, for PgUp and PgDn
	grid     *libGrid         // pages small instead of items, if not nil
}

// libGrid is a grid of pages drawn small, as many across as fit: a
// PDF's (bookpages.go).
type libGrid struct {
	n      int
	aspect float64                 // a page's height over its width
	thumb  func(i int) *image.Gray // page i small, nil till it's drawn
	label  func(i int) string      // under it
	// laid says how they're laid out: how wide (they're drawn so), how
	// many across, the first that shows and how many show.
	laid func(width, cols, first, per int)
}

// libHeadUI is a header's taps.
type libHeadUI struct {
	all, back widget.Clickable
	tabs      []widget.Clickable
}

type libUI struct {
	a     *bro
	spec  func() libSpec
	pane  *inkkobo.Pane
	head  libHeadUI
	tools []widget.Clickable
	rows  []widget.Clickable
	btns  [][]widget.Clickable
	ok    widget.Clickable
	no    widget.Clickable
	list  layout.Scroll
	sel   int    // the selection last shown: the list goes to it when it moves
	id    string // the list shown
	box   int    // a tag: the name's box, which a tap leaves be
	do    func() // what a tap picked, done after the frame
}

func newLibUI(a *bro, spec func() libSpec) *libUI {
	u := &libUI{a: a, spec: spec, sel: -1}
	u.pane = inkkobo.NewPane(u.Layout)
	return u
}

// take does what the frame picked.
func (u *libUI) take() {
	if do := u.do; do != nil {
		u.do = nil
		do()
	}
}

func (u *libUI) inkPane() *inkkobo.Pane { return u.pane }

// put draws u at r: all of it if it wasn't there, or all is asked for
// (the canvas under it was drawn over); else what changed. It says if
// it drew it all, for the caller to mark.
func (u *libUI) put(a *bro, r image.Rectangle, all bool) bool {
	if all || u.pane.At != r {
		u.pane.Place(r)
		u.pane.Win.Invalidate()
		u.pane.Draw(a.C.Gray)
		return true
	}
	u.pane.Win.Invalidate()
	a.flushPane(u.pane)
	u.take()
	return false
}

// rowH is a row's height: room for a finger.
func libRowHeight(gtx ink.Ctx, th *theme.Theme) int {
	body := th.Face(gtx, th.TextSize, text.Regular)
	return max(gtx.Dp(th.Touch), text.Height(body)+gtx.Dp(th.Pad*2.5))
}

func (u *libUI) Layout(gtx ink.Ctx) ink.Dims {
	s, th := u.spec(), u.a.theme()
	size := gtx.Constraints.Max
	ink.Fill(gtx.Ops, image.Rectangle{Max: size}, th.Bg)
	u.taps(gtx, s)
	if s.id != u.id {
		u.id, u.sel, u.list = s.id, -1, layout.Scroll{}
	}
	if s.sel != nil && s.grid == nil { // the grid follows it by its rows
		*s.sel = max(0, min(*s.sel, len(s.items)-1))
		if *s.sel != u.sel {
			u.sel = *s.sel
			u.list.ScrollTo(u.sel)
		}
	}
	rowH := libRowHeight(gtx, th)
	return layout.UniformInset(th.Pad).Layout(gtx, func(gtx ink.Ctx) ink.Dims {
		kids := []layout.FlexChild{
			layout.Rigid(func(gtx ink.Ctx) ink.Dims { return u.head.layout(gtx, th, s.head, rowH) }),
			layout.Rigid(layout.Spacer{Height: th.Pad}.Layout),
		}
		if len(s.tools) > 0 || s.toolNote != "" {
			kids = append(kids, layout.Rigid(func(gtx ink.Ctx) ink.Dims { return u.toolRow(gtx, s) }),
				layout.Rigid(layout.Spacer{Height: th.Pad}.Layout))
		}
		kids = append(kids, layout.Flexed(1, func(gtx ink.Ctx) ink.Dims {
			if s.grid != nil {
				return u.grid(gtx, s)
			}
			if s.shown != nil {
				*s.shown = max(1, gtx.Constraints.Max.Y/rowH)
			}
			if len(s.items) == 0 {
				l := theme.Body(th, s.empty)
				l.Gray = th.Quiet
				return layout.Inset{Left: th.Pad, Top: th.Pad * 2}.Layout(gtx, l.Layout)
			}
			u.list.Axis = layout.Vertical
			return theme.List(th, &u.list).Layout(gtx, len(s.items), func(gtx ink.Ctx, i int) ink.Dims {
				return u.row(gtx, s, i, rowH)
			})
		}))
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
	})
}

// taps finds what was tapped, for take.
func (u *libUI) taps(gtx ink.Ctx, s libSpec) {
	if do := u.head.taps(gtx, s.head); do != nil {
		u.do = do
	}
	for len(u.tools) < len(s.tools) {
		u.tools = append(u.tools, widget.Clickable{NoFocus: true})
	}
	for k, t := range s.tools {
		if u.tools[k].Clicked(gtx) {
			u.do = t.do
		}
	}
	n := len(s.items)
	if s.grid != nil {
		n = s.grid.n
	}
	for len(u.rows) < n {
		u.rows = append(u.rows, widget.Clickable{NoFocus: true})
		u.btns = append(u.btns, nil)
	}
	for i := range n {
		if u.rows[i].Clicked(gtx) && s.open != nil {
			u.do = func() { s.open(i) }
		}
		if u.rows[i].LongPressed(gtx) && s.hold != nil {
			u.do = func() { s.hold(i) }
		}
		for k := range u.btns[i] {
			if u.btns[i][k].Clicked(gtx) && s.buttons != nil && i < len(s.items) {
				if bs := s.buttons(i, s.items[i]); k < len(bs) {
					u.do = bs[k].do
				}
			}
		}
	}
	if u.ok.Clicked(gtx) {
		u.do = s.editOK
	}
	if u.no.Clicked(gtx) {
		u.do = s.editNo
	}
}

// taps is what the header's taps do, if any was.
func (h *libHeadUI) taps(gtx ink.Ctx, s libHeadSpec) func() {
	var do func()
	if h.all.Clicked(gtx) {
		do = s.toAll
	}
	if h.back.Clicked(gtx) && s.back != "" {
		do = s.toBack
	}
	for len(h.tabs) < len(s.tabs) {
		h.tabs = append(h.tabs, widget.Clickable{NoFocus: true})
	}
	for k, t := range s.tabs {
		if h.tabs[k].Clicked(gtx) {
			do = t.do
		}
	}
	return do
}

// layout lays the header out, rowH high: the way to them all at the
// left, black if we're there; the way back at the right; the tabs
// between, the one on underlined.
func (h *libHeadUI) layout(gtx ink.Ctx, th *theme.Theme, s libHeadSpec, rowH int) ink.Dims {
	for len(h.tabs) < len(s.tabs) {
		h.tabs = append(h.tabs, widget.Clickable{NoFocus: true})
	}
	h.all.NoFocus, h.back.NoFocus = true, true
	w := gtx.Constraints.Max.X
	f := th.Face(gtx, th.TextSize, text.Regular)
	bd := max(gtx.Dp(th.Border), 2)
	gap := gtx.Dp(th.Pad * 2)
	button := func(clk *widget.Clickable, r image.Rectangle, label string, filled bool) {
		st := ink.Offset(gtx.Ops, r.Min)
		cg := gtx
		cg.Constraints = ink.Exact(r.Size())
		clk.Layout(cg, func(gtx ink.Ctx) ink.Dims {
			b := image.Rectangle{Max: r.Size()}
			fg, bg := th.Fg, th.Bg
			if filled {
				fg, bg = bg, fg
			}
			ink.Fill(gtx.Ops, b, bg)
			ink.Frame(gtx.Ops, b, bd, th.Fg)
			s := text.Fit(f, label, b.Dx()-gtx.Dp(th.Pad))
			ink.TextAt(gtx.Ops, f, (b.Dx()-text.Width(f, s))/2, (b.Dy()-text.Height(f))/2+text.Ascent(f), s, fg)
			if clk.Pressed() {
				ink.Invert(gtx.Ops, b)
			}
			return ink.Dims{Size: b.Size()}
		})
		st.Pop()
	}
	bh := rowH - gtx.Dp(th.Pad)
	all := image.Rect(0, 0, min(gtx.Dp(160), w/5), bh)
	label := s.all
	if !s.here {
		label = "‹ " + label
	}
	button(&h.all, all, label, s.here)
	bw := min(gtx.Dp(210), w/4) // the way back's room, kept if there's none: the tabs stay put
	right := w - bw
	if s.back != "" {
		back := s.back
		if text.Width(f, back)+gtx.Dp(th.Pad*3) > bw { // a narrow window
			back = "back"
		}
		button(&h.back, image.Rect(w-bw, 0, w, bh), back, false)
	}
	tabs := image.Rect(all.Max.X+gap, 0, right-gap, rowH)
	ink.Fill(gtx.Ops, image.Rect(tabs.Min.X, rowH-2, tabs.Max.X, rowH), ink.Light)
	if n := len(s.tabs); n > 0 && tabs.Dx() > 0 {
		each := tabs.Dx() / n
		for k, t := range s.tabs {
			r := image.Rect(tabs.Min.X+k*each, 0, tabs.Min.X+(k+1)*each, rowH)
			st := ink.Offset(gtx.Ops, r.Min)
			cg := gtx
			cg.Constraints = ink.Exact(r.Size())
			clk := &h.tabs[k]
			clk.Layout(cg, func(gtx ink.Ctx) ink.Dims {
				b := image.Rectangle{Max: r.Size()}
				ink.Fill(gtx.Ops, b, th.Bg)
				g := th.Quiet
				if t.on {
					g = th.Fg
					ink.Fill(gtx.Ops, image.Rect(0, b.Dy()-gtx.Dp(3.5), b.Dx(), b.Dy()), th.Fg)
				}
				s := text.Fit(f, t.name, b.Dx()-gtx.Dp(th.Pad))
				ink.TextAt(gtx.Ops, f, (b.Dx()-text.Width(f, s))/2, (b.Dy()-text.Height(f))/2+text.Ascent(f), s, g)
				if clk.Pressed() {
					ink.Invert(gtx.Ops, b)
				}
				return ink.Dims{Size: b.Size()}
			})
			st.Pop()
		}
	}
	return ink.Dims{Size: image.Pt(w, rowH)}
}

// toolRow is the buttons under the header, and a word beside them.
func (u *libUI) toolRow(gtx ink.Ctx, s libSpec) ink.Dims {
	th := u.a.theme()
	var kids []layout.FlexChild
	for k, t := range s.tools {
		kids = append(kids, layout.Rigid(theme.Button(th, &u.tools[k], t.name).Layout), layout.Rigid(layout.Spacer{Width: th.Pad * 2}.Layout))
	}
	if s.toolNote != "" {
		kids = append(kids, layout.Flexed(1, func(gtx ink.Ctx) ink.Dims {
			small := th.Face(gtx, th.TextSize*0.85, text.Regular)
			w, h := gtx.Constraints.Max.X, gtx.Dp(th.Touch)
			ink.TextAt(gtx.Ops, small, gtx.Dp(th.Pad), (h-text.Height(small))/2+text.Ascent(small), text.Fit(small, s.toolNote, w-gtx.Dp(th.Pad)), th.Quiet)
			return ink.Dims{Size: image.Pt(w, h)}
		}))
	}
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, kids...)
}

// row is item i: lit if it's the selection; a folder's sign, its name,
// a word on it at the right, then its buttons. Being renamed, it's the
// name typed, with ok and cancel.
func (u *libUI) row(gtx ink.Ctx, s libSpec, i, rowH int) ink.Dims {
	th := u.a.theme()
	w := gtx.Constraints.Max.X
	it := s.items[i]
	if i == s.edit {
		return u.editRow(gtx, s, w, rowH)
	}
	lit := s.sel != nil && i == *s.sel
	fg, quiet, bg := th.Fg, th.Quiet, th.Bg
	if lit {
		fg, quiet, bg = th.Bg, ink.Light, th.Fg
	}
	ink.Fill(gtx.Ops, image.Rect(0, 0, w, rowH), bg)
	ink.Fill(gtx.Ops, image.Rect(0, rowH-1, w, rowH), ink.Light)
	var bs []libTab
	if s.buttons != nil {
		bs = s.buttons(i, it)
	}
	for len(u.btns[i]) < len(bs) {
		u.btns[i] = append(u.btns[i], widget.Clickable{NoFocus: true})
	}
	f := th.Face(gtx, th.TextSize, text.Regular)
	small := th.Face(gtx, th.TextSize*0.85, text.Regular)
	pad := gtx.Dp(th.Pad)
	bd := max(gtx.Dp(th.Border), 2)
	right := w - pad
	for k := len(bs) - 1; k >= 0; k-- {
		bw := max(rowH, text.Width(f, bs[k].name)+gtx.Dp(th.Pad*4))
		r := image.Rect(right-bw, pad*3/4, right, rowH-pad*3/4)
		clk := &u.btns[i][k]
		name := bs[k].name
		st := ink.Offset(gtx.Ops, r.Min)
		cg := gtx
		cg.Constraints = ink.Exact(r.Size())
		clk.Layout(cg, func(gtx ink.Ctx) ink.Dims {
			b := image.Rectangle{Max: r.Size()}
			ink.Fill(gtx.Ops, b, bg)
			ink.Frame(gtx.Ops, b, bd, fg)
			ink.TextAt(gtx.Ops, f, (b.Dx()-text.Width(f, name))/2, (b.Dy()-text.Height(f))/2+text.Ascent(f), name, fg)
			if clk.Pressed() {
				ink.Invert(gtx.Ops, b)
			}
			return ink.Dims{Size: b.Size()}
		})
		st.Pop()
		right = r.Min.X - pad
	}
	// The row itself, up to its buttons: a tap opens it.
	cg := gtx
	cg.Constraints = ink.Exact(image.Pt(right, rowH))
	clk := &u.rows[i]
	clk.Layout(cg, func(gtx ink.Ctx) ink.Dims {
		x, end := pad, right-pad
		if s.picking && !it.up { // its box, ticked if it's picked
			bs := gtx.Dp(24)
			b := image.Rect(x, (rowH-bs)/2, x+bs, (rowH+bs)/2)
			ink.Frame(gtx.Ops, b, bd, fg)
			if s.picked != nil && s.picked(i) {
				ink.Fill(gtx.Ops, b.Inset(bd+gtx.Dp(3)), fg)
			}
			x += bs + gtx.Dp(14)
		}
		if it.dir && !it.up {
			drawFolderInk(gtx, image.Rect(x, 0, x+gtx.Dp(26), rowH), fg)
			x += gtx.Dp(38)
		}
		if it.note != "" {
			nw := min(text.Width(small, it.note), (end-x)/2)
			note := text.Fit(small, it.note, nw)
			ink.TextAt(gtx.Ops, small, end-text.Width(small, note), (rowH-text.Height(small))/2+text.Ascent(small), note, quiet)
			end -= nw + gtx.Dp(th.Pad*2)
		}
		ink.TextAt(gtx.Ops, f, x, (rowH-text.Height(f))/2+text.Ascent(f), text.Fit(f, it.name, end-x), fg)
		if clk.Pressed() {
			ink.Invert(gtx.Ops, image.Rect(0, 0, right, rowH))
		}
		return ink.Dims{Size: image.Pt(right, rowH)}
	})
	return ink.Dims{Size: image.Pt(w, rowH)}
}

// editRow is a name being typed: the prompt, the name, a caret; ok and
// cancel at the right.
func (u *libUI) editRow(gtx ink.Ctx, s libSpec, w, rowH int) ink.Dims {
	th := u.a.theme()
	f := th.Face(gtx, th.TextSize, text.Regular)
	small := th.Face(gtx, th.TextSize*0.85, text.Regular)
	pad := gtx.Dp(th.Pad)
	u.ok.NoFocus, u.no.NoFocus = true, true
	ink.Fill(gtx.Ops, image.Rect(0, 0, w, rowH), th.Bg)
	right := w
	for _, b := range []struct {
		clk   *widget.Clickable
		label string
	}{{&u.no, "cancel"}, {&u.ok, "ok"}} {
		bw := text.Width(f, b.label) + gtx.Dp(th.Pad*5)
		st := ink.Offset(gtx.Ops, image.Pt(right-bw, pad*3/4))
		cg := gtx
		cg.Constraints = ink.Exact(image.Pt(bw, rowH-pad*3/2))
		theme.Button(th, b.clk, b.label).Layout(cg)
		st.Pop()
		right -= bw + pad
	}
	box := image.Rect(0, pad/2, right-pad, rowH-pad/2)
	ink.Frame(gtx.Ops, box, max(gtx.Dp(2), 3), th.Fg)
	ink.Input(gtx.Ops, ink.Area{Rect: box, Tag: &u.box, Keep: true}) // a tap on it does nothing
	x := pad * 3 / 2
	base := box.Min.Y + (box.Dy()-text.Height(f))/2 + text.Ascent(f)
	ink.TextAt(gtx.Ops, small, x, base, s.prompt, th.Quiet)
	x += text.Width(small, s.prompt) + pad/2
	t := text.Fit(f, s.typed, box.Max.X-pad-x-gtx.Dp(4))
	ink.TextAt(gtx.Ops, f, x, base, t, th.Fg)
	cx := x + text.Width(f, t) + gtx.Dp(1)
	ink.Fill(gtx.Ops, image.Rect(cx, box.Min.Y+box.Dy()/4, cx+max(gtx.Dp(2), 3), box.Max.Y-box.Dy()/4), th.Fg) // the caret
	return ink.Dims{Size: image.Pt(w, rowH)}
}

// Pages small: at least thumbW px wide, thumbGap between, thumbLab under
// each for its number.
const (
	thumbW   = 240
	thumbGap = 24
	thumbLab = 36
)

// grid lays the pages out small, as many across as fit (on a short
// window one row, as tall as it goes), in a list of their rows that
// follows the selection; the one chosen framed thick.
func (u *libUI) grid(gtx ink.Ctx, s libSpec) ink.Dims {
	g, th := s.grid, u.a.theme()
	w, room := gtx.Constraints.Max.X, gtx.Constraints.Max.Y
	if g.n == 0 || w <= 0 || room <= 0 {
		return ink.Dims{Size: gtx.Constraints.Max}
	}
	cols := max(2, (w+thumbGap)/(thumbW+thumbGap))
	tw := (w - (cols-1)*thumbGap) / cols
	ph := int(float64(tw) * g.aspect)
	if ph+thumbLab+thumbGap > room {
		ph = max(1, room-thumbLab-thumbGap)
		tw = max(1, int(float64(ph)/g.aspect))
		cols = max(1, (w+thumbGap)/(tw+thumbGap))
	}
	rowH := ph + thumbLab + thumbGap
	x0 := (w - cols*tw - (cols-1)*thumbGap) / 2
	sel := 0
	if s.sel != nil {
		*s.sel = max(0, min(*s.sel, g.n-1))
		sel = *s.sel
	}
	if sel != u.sel {
		u.sel = sel
		u.list.ScrollTo(sel / cols)
	}
	small := th.Face(gtx, th.TextSize*0.85, text.Regular)
	u.list.Axis = layout.Vertical
	d := theme.List(th, &u.list).Layout(gtx, (g.n+cols-1)/cols, func(gtx ink.Ctx, row int) ink.Dims {
		for c := range cols {
			i := row*cols + c
			if i >= g.n {
				break
			}
			at := image.Pt(x0+c*(tw+thumbGap), thumbGap/2)
			st := ink.Offset(gtx.Ops, at)
			cg := gtx
			cg.Constraints = ink.Exact(image.Pt(tw, ph+thumbLab))
			clk := &u.rows[i]
			clk.Layout(cg, func(gtx ink.Ctx) ink.Dims {
				r := image.Rect(0, 0, tw, ph)
				ink.Fill(gtx.Ops, image.Rect(0, 0, tw, ph+thumbLab), th.Bg)
				if m := g.thumb(i); m != nil {
					ink.Image(gtx.Ops, r, m, m.Rect.Min)
				}
				frame, gray := 1, uint8(ink.Mid)
				if i == sel {
					frame, gray = 5, th.Fg
				}
				ink.Frame(gtx.Ops, r.Inset(-frame), frame, gray)
				l := text.Fit(small, g.label(i), tw)
				ink.TextAt(gtx.Ops, small, (tw-text.Width(small, l))/2, ph+(thumbLab-text.Height(small))/2+text.Ascent(small), l, th.Fg)
				if clk.Pressed() {
					ink.Invert(gtx.Ops, r)
				}
				return ink.Dims{Size: cg.Constraints.Max}
			})
			st.Pop()
		}
		return ink.Dims{Size: image.Pt(w, rowH)}
	})
	if s.shown != nil {
		*s.shown = cols * max(1, room/rowH)
	}
	g.laid(tw, cols, u.list.First()*cols, cols*max(1, room/rowH))
	return d
}

// drawFolderInk draws a folder's sign in the middle of r, in g.
func drawFolderInk(gtx ink.Ctx, r image.Rectangle, g uint8) {
	m := r.Min.Y + r.Dy()/2
	w := r.Dx()
	h := w * 2 / 3
	t := max(gtx.Dp(2), 3)
	ink.Fill(gtx.Ops, image.Rect(r.Min.X, m-h/2-t, r.Min.X+w*2/5, m-h/2+1), g) // its tab
	ink.Frame(gtx.Ops, image.Rect(r.Min.X, m-h/2, r.Min.X+w, m+h/2), t, g)
}
