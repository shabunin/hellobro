package main

// The bar along the top of a book's or a notebook's page, the same in
// both: ‹ All Books (or All Notebooks) at its left, the tools, and at
// its right the page before, which page (a button too), and the next.
// What doesn't fit goes, the least needed first.

import (
	"image"
	"slices"

	"hellobro/kobo"
)

type barBtn struct {
	id, label string // no label: a picture
	r         image.Rectangle
}

const barGap = 10 // between the bar's buttons

// barHeight is the bar's (sizes: bigger for fingers).
func barHeight() int {
	if kobo.IsBig() {
		return 76
	}
	return 56
}

// barBtnWidth is b's width on bar: a picture's about square, words as
// wide as they are.
func barBtnWidth(a *bro, bar image.Rectangle, b barBtn) int {
	if b.label == "" {
		return bar.Dy() + 8
	}
	return max(kobo.TextWidth(a.F.Small, b.label)+36, bar.Dy()+8)
}

// barNeed is how wide tools are along bar.
func barNeed(a *bro, bar image.Rectangle, tools []barBtn) int {
	n := 0
	for _, t := range tools {
		n += barBtnWidth(a, bar, t) + barGap
	}
	return n
}

// barRoom is the room along bar for the tools: the page arrows and the
// counter between them, counterW wide, are at its end.
func barRoom(bar image.Rectangle, counterW int) int {
	return bar.Dx() - 2*(bar.Dy()+8) - counterW - 4*barGap
}

// layTop lays tools along bar, as many as fit: drops go first, in
// their order, then the first's label is its arrow alone, then the last
// go. The page before and the next are at the bar's end, room for the
// counter between: they're the last two, the next then the one before.
func layTop(a *bro, bar image.Rectangle, tools []barBtn, drops []string, counterW int) []barBtn {
	tools = slices.Clone(tools)
	counterW = max(0, min(counterW, barRoom(bar, 0))) // narrower than the arrows and it: it's what's left
	room := barRoom(bar, counterW)
	for _, drop := range drops {
		if barNeed(a, bar, tools) <= room {
			break
		}
		tools = slices.DeleteFunc(tools, func(t barBtn) bool { return t.id == drop })
	}
	if len(tools) > 0 && barNeed(a, bar, tools) > room { // narrower still: All its arrow alone, then the last tools go (and it, last)
		tools[0].label = "‹"
	}
	for len(tools) > 0 && barNeed(a, bar, tools) > room {
		tools = tools[:len(tools)-1]
	}
	var btns []barBtn
	x := bar.Min.X
	for _, t := range tools {
		t.r = image.Rect(x, bar.Min.Y+4, x+barBtnWidth(a, bar, t), bar.Max.Y-4)
		btns = append(btns, t)
		x = t.r.Max.X + barGap
	}
	x = bar.Max.X
	for _, t := range []barBtn{{id: "next"}, {id: "prev"}} {
		t.r = image.Rect(x-barBtnWidth(a, bar, t), bar.Min.Y+4, x, bar.Max.Y-4)
		btns = append(btns, t)
		x = t.r.Min.X - counterW - 2*barGap
	}
	return btns
}

// barCounter is where the counter is: between the arrows, the last two
// of btns (one over the other, down a side).
func barCounter(btns []barBtn, bar image.Rectangle, side bool) image.Rectangle {
	n := len(btns)
	if n < 2 {
		return image.Rectangle{}
	}
	prev, next := btns[n-1].r, btns[n-2].r
	if side {
		return image.Rect(prev.Min.X, prev.Max.Y+6, prev.Max.X, next.Min.Y-6)
	}
	return image.Rect(prev.Max.X+6, bar.Min.Y+4, next.Min.X-6, bar.Max.Y-4)
}

// drawBarBtn draws b framed: black if it's on, its ink gray if it's
// off. glyph draws its picture, if it has one of its own (true); else
// its label, or the arrows' triangles. It says the ink and paper used.
func drawBarBtn(a *bro, b barBtn, on, off bool, glyph func(r image.Rectangle, ink, bg uint8) bool) (ink, bg uint8) {
	c := a.C
	bg, ink = kobo.White, kobo.Black
	switch {
	case on:
		bg, ink = kobo.Black, kobo.White
	case off:
		ink = kobo.Mid
	}
	c.Fill(b.r, bg)
	c.Frame(b.r, 2, kobo.Black)
	switch {
	case glyph != nil && glyph(b.r, ink, bg):
	case b.id == "prev" || b.id == "next": // the font has no arrows: a triangle
		m, h := b.r.Min.Add(b.r.Max).Div(2), b.r.Dy()/4
		for dy := -h; dy <= h; dy++ {
			l := h - kobo.Abs(dy)
			if b.id == "next" {
				c.Fill(image.Rect(m.X-h/2, m.Y+dy, m.X-h/2+l+1, m.Y+dy+1), ink)
			} else {
				c.Fill(image.Rect(m.X+h/2-l, m.Y+dy, m.X+h/2+1, m.Y+dy+1), ink)
			}
		}
	case b.label == "‹": // All's arrow alone: big enough to see
		c.Text(a.F.Panel, b.r, b.label, ink, kobo.AlignCenter)
	default:
		c.Text(a.F.Small, b.r, b.label, ink, kobo.AlignCenter)
	}
	return ink, bg
}

// drawBarCounter draws the counter in r, a button too: black if on.
func drawBarCounter(a *bro, r image.Rectangle, text string, on bool) {
	if r.Empty() {
		return
	}
	bg, ink := uint8(kobo.White), uint8(kobo.Black)
	if on {
		bg, ink = kobo.Black, kobo.White
	}
	a.C.Fill(r, bg)
	a.C.Frame(r, 2, kobo.Black)
	a.C.Text(a.F.Small, r, kobo.Fit(a.F.Small, text, r.Dx()-8), ink, kobo.AlignCenter)
}
