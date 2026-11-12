package main

// A question over the page: yes or no ("restart into the new hellobro?"),
// or one of a few things to do (a saved host's menu). Enter picks the
// one selected (the first, till the arrows or Tab move it), a choice's
// letter that one, Esc none; or tap a button. The page
// holds still under it, like under the switcher. For what can't be
// undone, the yes waits for words typed first (askTyped).

import (
	"image"
	"time"
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
	sel         int       // the answer Enter picks
	keysAt      time.Time // the keys answer from then on (a tap at once)
	want, typed string    // askTyped's: the words wanted, and what's typed so far
	ui          *askUI
	over        *inkOver
}

// ask puts a yes-or-no question up; do runs on a yes.
func (a *bro) ask(title, text, yes string, do func()) {
	a.choose(title, text, askOpt{yes, "y", do}, askOpt{"no", "n", nil})
}

// choose puts a question up with these answers: side by side if two, the
// second being no; else one under another.
func (a *bro) choose(title, text string, opts ...askOpt) {
	q := &askBox{title: title, text: text, opts: opts}
	q.ui = newAskUI(a, q)
	q.over = newOver(q.ui.Layout, 1000)
	q.over.take = func() {
		if i, ok := q.ui.picked(); ok {
			a.answer(i)
		}
	}
	a.asking = q
	a.drawAsk()
}

// askTyped is ask, but its yes holds till want is typed, as it is.
func (a *bro) askTyped(title, text, want, yes string, do func()) {
	a.choose(title, text, askOpt{yes, "", do}, askOpt{"cancel", "", nil})
	q := a.asking
	q.want = want
	q.ui.selAt = -1 // the keys into the box
	a.drawAsk()
}

// ready says whether the first answer can be taken.
func (q *askBox) ready() bool { return q.typed == q.want }

// overlay says whether something is up over the page: never over the
// windows drawn for the workspaces' tiles (drawThumbs).
func (a *bro) overlay() bool {
	return !a.wm.offscreen && (a.picker != nil || a.asking != nil || a.info != nil || a.keyset != nil)
}

// drawAsk draws the question as it is now, all of it.
func (a *bro) drawAsk() { a.drawOver(a.asking.over) }

// answer takes the question down, and does what the answer i says (-1:
// none).
func (a *bro) answer(i int) {
	q := a.asking
	if i == 0 && !q.ready() {
		return
	}
	a.asking = nil
	q.over.gone()
	a.DrawPage()
	if i >= 0 && q.opts[i].do != nil {
		q.opts[i].do()
	}
}

func (a *bro) askKey(code uint16, value int32) {
	if a.termMod(code, value) {
		return
	}
	if value != 0 && time.Now().Before(a.asking.keysAt) {
		return
	}
	a.overKey(a.asking.over, code, value)
}

// askTap: a tap outside the card is no. One on it was its pane's.
func (a *bro) askTap(p image.Point) {
	if !p.In(a.asking.over.box) {
		a.answer(-1)
	}
}
