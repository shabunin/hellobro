package main

// A card over the page, in ink: a question (ask.go), facts or a cheat
// sheet (info.go). It's a pane of its own in the middle of what the
// keyboard leaves, as tall as its UI says it wants (laid out once to
// know, then placed). What a frame picked (an answer, close) is done
// after the frame, by its take: the page drawn again under a frame still
// going would have the card's last pixels put over it.

import (
	"image"
	"time"

	"hellobro/ink"
	"hellobro/ink/inkkobo"
	"hellobro/ink/theme"
	"hellobro/kobo"
)

type inkOver struct {
	pane *inkkobo.Pane
	box  image.Rectangle // on the canvas
	w    int             // its width at the Elipsa's density, as wide as the screen lets it be
	h    int             // the height its UI wants, as laid out last; 0: as tall as it may be
	tall bool            // as tall as it may be, always (a sheet)
	take func()          // after a frame: what it picked, done
}

func newOver(ui ink.Widget, w int) *inkOver {
	return &inkOver{pane: inkkobo.NewPane(ui), w: w}
}

// over is the card up, if one is.
func (a *bro) over() *inkOver {
	switch {
	case a.asking != nil:
		return a.asking.over
	case a.info != nil:
		return a.info.over
	}
	return nil
}

// place puts o in the middle of what the keyboard leaves.
func (o *inkOver) place(a *bro) {
	b := a.screenFree()
	w := min(int(float32(o.w)*inkkobo.Metric().PxPerDp/1.42), b.Dx()-80)
	h := o.h
	if h == 0 || o.tall {
		h = b.Dy() - 80
	}
	h = min(h, b.Dy()-40)
	top := b.Min.Y + (b.Dy()-h)/2
	o.box = image.Rect(b.Min.X+(b.Dx()-w)/2, top, b.Min.X+(b.Dx()+w)/2, top+h)
	o.pane.Place(o.box)
}

// drawOver draws o as it is now, all of it.
func (a *bro) drawOver(o *inkOver) {
	o.place(a)
	o.pane.Win.Invalidate()
	o.pane.Win.Frame(time.Now())
	o.place(a)
	o.pane.Draw(a.C.Gray)
	a.Mark(o.box, true)
}

// overTick puts what changed of the card on the panel, and does what it
// picked.
func (a *bro) overTick() {
	o := a.over()
	if o == nil {
		return
	}
	o.pane.Flush(a.App)
	if o.take != nil {
		o.take()
	}
}

// overGone takes the card's pane off the screen: the page is drawn again
// over it.
func (o *inkOver) gone() { o.pane.Place(image.Rectangle{}) }

// overKey hands a key to the card.
func (a *bro) overKey(o *inkOver, code uint16, value int32) {
	o.pane.Key(code, value, inkMods(a.tv.mods), a.tv.mods.caps)
	a.overTick()
}

// overDown gives a contact coming down on the card to it; not a sheet's:
// a tap anywhere on that closes it (infoTap).
func (a *bro) overDown(ev kobo.TouchEv, p image.Point, g *kobo.Gesture) bool {
	o := a.over()
	if o == nil || a.info != nil && a.info.sheet || !o.pane.Down(ev, p, g, ownPane) {
		return false
	}
	a.overTick()
	return true
}

// card lays a card's frame out over all it's given, and in it what in
// lays out, unbounded down: the card's height is what in wants (h). A
// tap on the card, between its controls, leaves the keys where they are.
func card(gtx ink.Ctx, th *theme.Theme, h *int, in ink.Widget) ink.Dims {
	size := gtx.Constraints.Max
	r := image.Rectangle{Max: size}
	b := max(gtx.Dp(2), 3)
	ink.Fill(gtx.Ops, r, th.Bg)
	ink.Frame(gtx.Ops, r, b, th.Fg)
	ink.Input(gtx.Ops, ink.Area{Rect: r, Tag: h, Keep: true})
	cg := gtx
	cg.Constraints = ink.Constraints{Max: image.Pt(size.X, 1<<20)}
	d := in(cg)
	*h = d.Size.Y
	return ink.Dims{Size: size}
}
