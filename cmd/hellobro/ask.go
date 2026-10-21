package main

// A question over the page, yes or no: "restart into the new hellobro?",
// "delete this album?". Enter (or y) says yes, Esc (or n) no; or tap a
// button. The page holds still under it, like under the switcher.

import (
	"image"

	"hellobro/kobo"
)

type askBox struct {
	title, text, yes string
	do               func()
	box, yesR, noR   image.Rectangle
}

// ask puts the question up; do runs on a yes.
func (a *bro) ask(title, text, yes string, do func()) {
	b := a.C.Bounds()
	w := min(1000, b.Dx()-80)
	h := 330
	top := b.Min.Y + (b.Dy()-h)/2
	q := &askBox{title: title, text: text, yes: yes, do: do}
	q.box = image.Rect((b.Dx()-w)/2, top, (b.Dx()+w)/2, top+h)
	bw, by := 240, q.box.Max.Y-96
	q.noR = image.Rect(q.box.Max.X-32-bw, by, q.box.Max.X-32, by+68)
	q.yesR = image.Rect(q.noR.Min.X-20-bw, by, q.noR.Min.X-20, by+68)
	a.asking = q
	a.drawAsk()
}

// overlay says whether something is up over the page.
func (a *bro) overlay() bool { return a.picker != nil || a.asking != nil }

func (a *bro) drawAsk() {
	q, c := a.asking, a.C
	c.Fill(q.box, kobo.White)
	c.Frame(q.box, 4, kobo.Black)
	in := q.box.Inset(32)
	c.Text(a.F.Panel, image.Rect(in.Min.X, in.Min.Y, in.Max.X, in.Min.Y+56), q.title, kobo.Black, kobo.AlignLeft)
	c.Text(a.F.Small, image.Rect(in.Min.X, in.Min.Y+70, in.Max.X, in.Min.Y+110), q.text, kobo.Dark, kobo.AlignLeft)
	for _, b := range []struct {
		r     image.Rectangle
		label string
		dark  bool
	}{{q.yesR, q.yes + "  (Enter)", true}, {q.noR, "no  (Esc)", false}} {
		fg := uint8(kobo.Black)
		if b.dark {
			c.Fill(b.r, kobo.Black)
			fg = kobo.White
		} else {
			c.Frame(b.r, 2, kobo.Black)
		}
		c.Text(a.F.Small, b.r, b.label, fg, kobo.AlignCenter)
	}
	a.Mark(q.box, true)
}

// answer takes the question down, and does it on a yes.
func (a *bro) answer(yes bool) {
	q := a.asking
	a.asking = nil
	a.DrawPage()
	a.drawStat(true)
	if yes {
		q.do()
	}
}

func (a *bro) askKey(code uint16, value int32) {
	if a.termMod(code, value) || value != 1 {
		return
	}
	ch, _ := kobo.KeyChar(code, false, false)
	switch {
	case code == kobo.KeyEnter || ch == "y":
		a.answer(true)
	case code == kobo.KeyEsc || ch == "n":
		a.answer(false)
	}
}

func (a *bro) askTap(p image.Point) {
	q := a.asking
	switch {
	case kobo.Hit(p, q.yesR):
		a.answer(true)
	case kobo.Hit(p, q.noR), !p.In(q.box):
		a.answer(false)
	}
}
