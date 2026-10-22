package main

// A question over the page: yes or no ("restart into the new hellobro?"),
// or one of a few things to do (a saved host's menu). Enter picks the
// first, a choice's letter that one, Esc none; or tap a button. The page
// holds still under it, like under the switcher.

import (
	"image"

	"hellobro/kobo"
)

// askOpt is one of the answers: its button, its key (a letter), and what
// it does (nil: nothing, as Esc).
type askOpt struct {
	label, key string
	do         func()
}

type askBox struct {
	title, text string
	opts        []askOpt
	box         image.Rectangle
	optR        []image.Rectangle
}

// ask puts a yes-or-no question up; do runs on a yes.
func (a *bro) ask(title, text, yes string, do func()) {
	a.choose(title, text, askOpt{yes, "y", do}, askOpt{"no", "n", nil})
}

// choose puts a question up with these answers: side by side if two, the
// second being no; else one under another.
func (a *bro) choose(title, text string, opts ...askOpt) {
	b := a.C.Bounds()
	w := min(1000, b.Dx()-80)
	h := 330
	if len(opts) > 2 {
		h = 200 + len(opts)*84
	}
	top := b.Min.Y + (b.Dy()-h)/2
	q := &askBox{title: title, text: text, opts: opts}
	q.box = image.Rect((b.Dx()-w)/2, top, (b.Dx()+w)/2, top+h)
	if len(opts) == 2 {
		bw, by := 240, q.box.Max.Y-96
		no := image.Rect(q.box.Max.X-32-bw, by, q.box.Max.X-32, by+68)
		q.optR = []image.Rectangle{image.Rect(no.Min.X-20-bw, by, no.Min.X-20, by+68), no}
	} else {
		y := q.box.Min.Y + 160
		for range opts {
			q.optR = append(q.optR, image.Rect(q.box.Min.X+32, y, q.box.Max.X-32, y+68))
			y += 84
		}
	}
	a.asking = q
	a.drawAsk()
}

// overlay says whether something is up over the page.
func (a *bro) overlay() bool {
	return a.picker != nil || a.asking != nil || a.info != nil || a.tabsEd != nil || a.keyset != nil
}

func (a *bro) drawAsk() {
	q, c := a.asking, a.C
	c.Fill(q.box, kobo.White)
	c.Frame(q.box, 4, kobo.Black)
	in := q.box.Inset(32)
	c.Text(a.F.Panel, image.Rect(in.Min.X, in.Min.Y, in.Max.X, in.Min.Y+56), q.title, kobo.Black, kobo.AlignLeft)
	c.Text(a.F.Small, image.Rect(in.Min.X, in.Min.Y+70, in.Max.X, in.Min.Y+110), q.text, kobo.Dark, kobo.AlignLeft)
	for i, o := range q.opts {
		r, label := q.optR[i], o.label
		switch {
		case i == 0:
			label += "  (Enter)"
		case o.do == nil:
			label += "  (Esc)"
		case o.key != "":
			label += "  (" + o.key + ")"
		}
		fg := uint8(kobo.Black)
		if i == 0 {
			c.Fill(r, kobo.Black)
			fg = kobo.White
		} else {
			c.Frame(r, 2, kobo.Black)
		}
		c.Text(a.F.Small, r, label, fg, kobo.AlignCenter)
	}
	a.Mark(q.box, true)
}

// answer takes the question down, and does what the answer i says (-1:
// none).
func (a *bro) answer(i int) {
	q := a.asking
	a.asking = nil
	a.DrawPage()
	a.drawStat(true)
	if i >= 0 && q.opts[i].do != nil {
		q.opts[i].do()
	}
}

func (a *bro) askKey(code uint16, value int32) {
	if a.termMod(code, value) || value != 1 {
		return
	}
	ch, _ := kobo.KeyChar(code, false, false)
	switch {
	case code == kobo.KeyEnter:
		a.answer(0)
	case code == kobo.KeyEsc:
		a.answer(-1)
	case ch != "":
		for i, o := range a.asking.opts {
			if o.key == ch {
				a.answer(i)
				return
			}
		}
	}
}

func (a *bro) askTap(p image.Point) {
	q := a.asking
	for i, r := range q.optR {
		if kobo.Hit(p, r) {
			a.answer(i)
			return
		}
	}
	if !p.In(q.box) {
		a.answer(-1)
	}
}
