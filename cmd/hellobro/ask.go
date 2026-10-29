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
	sel         int       // the answer Enter picks
	keysAt      time.Time // the keys answer from then on (a tap at once)
	want, typed string    // askTyped's: the words wanted, and what's typed so far
	field       image.Rectangle
}

// ask puts a yes-or-no question up; do runs on a yes.
func (a *bro) ask(title, text, yes string, do func()) {
	a.choose(title, text, askOpt{yes, "y", do}, askOpt{"no", "n", nil})
}

// choose puts a question up with these answers: side by side if two, the
// second being no; else one under another.
func (a *bro) choose(title, text string, opts ...askOpt) {
	b := a.screenFree()
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

// askTyped is ask, but its yes holds till want is typed, as it is.
func (a *bro) askTyped(title, text, want, yes string, do func()) {
	a.choose(title, text, askOpt{yes, "", do}, askOpt{"cancel", "", nil})
	q := a.asking
	q.want = want
	q.box.Min.Y -= 50 // room for the field
	q.box.Max.Y += 50
	for i := range q.optR {
		q.optR[i] = q.optR[i].Add(image.Pt(0, 50))
	}
	q.field = image.Rect(q.box.Min.X+32, q.box.Min.Y+150, q.box.Max.X-32, q.box.Min.Y+220)
	a.drawAsk()
}

// ready says whether the first answer can be taken.
func (q *askBox) ready() bool { return q.typed == q.want }

// overlay says whether something is up over the page.
func (a *bro) overlay() bool {
	return a.picker != nil || a.asking != nil || a.info != nil || a.keyset != nil
}

func (a *bro) drawAsk() {
	q, c := a.asking, a.C
	c.Fill(q.box, kobo.White)
	c.Frame(q.box, 4, kobo.Black)
	in := q.box.Inset(32)
	c.Text(a.F.Panel, image.Rect(in.Min.X, in.Min.Y, in.Max.X, in.Min.Y+56), q.title, kobo.Black, kobo.AlignLeft)
	c.Text(a.F.Small, image.Rect(in.Min.X, in.Min.Y+70, in.Max.X, in.Min.Y+110), q.text, kobo.Dark, kobo.AlignLeft)
	if q.want != "" {
		c.Frame(q.field, 2, kobo.Black)
		f := q.field.Inset(4)
		f.Min.X += 12
		if q.typed == "" {
			c.Text(a.F.Small, f, "type: "+q.want, kobo.Mid, kobo.AlignLeft)
		} else {
			c.Text(a.F.Small, f, q.typed+"_", kobo.Black, kobo.AlignLeft)
		}
	}
	for i, o := range q.opts {
		r, label := q.optR[i], o.label
		switch {
		case i == q.sel:
			label += "  (Enter)"
		case o.do == nil:
			label += "  (Esc)"
		case o.key != "":
			label += "  (" + o.key + ")"
		}
		fg := uint8(kobo.Black)
		switch {
		case i == 0 && !q.ready(): // not yet
			c.Frame(r, map[bool]int{false: 2, true: 4}[i == q.sel], kobo.Mid)
			fg = kobo.Mid
		case i == q.sel:
			c.Fill(r, kobo.Black)
			fg = kobo.White
		default:
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
	if i == 0 && !q.ready() {
		return
	}
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
	q := a.asking
	if time.Now().Before(q.keysAt) {
		return
	}
	if d := askStep(code, a.tv.mods.shift); d != 0 { // the arrows, Tab: another answer
		q.sel = (q.sel + d + len(q.opts)) % len(q.opts)
		a.drawAsk()
		return
	}
	if q.want != "" {
		m := &a.tv.mods
		ch, printable := kobo.KeyChar(code, m.shift, m.caps)
		switch {
		case code == kobo.KeyEnter:
			a.answer(q.sel)
		case code == kobo.KeyEsc:
			a.answer(-1)
		case code == kobo.KeyBackspace:
			if rs := []rune(q.typed); len(rs) > 0 {
				q.typed = string(rs[:len(rs)-1])
				a.drawAsk()
			}
		case printable && !m.ctrl && len(q.typed) < 2*len(q.want):
			q.typed += ch
			a.drawAsk()
		}
		return
	}
	ch, _ := kobo.KeyChar(code, false, false)
	switch {
	case code == kobo.KeyEnter:
		a.answer(q.sel)
	case code == kobo.KeyEsc:
		a.answer(-1)
	case ch != "":
		for i, o := range q.opts {
			if o.key == ch {
				a.answer(i)
				return
			}
		}
	}
}

// askStep is how far a key moves the selection: ← ↑ and Shift+Tab back,
// → ↓ and Tab on.
func askStep(code uint16, shift bool) int {
	switch {
	case code == keyLeft, code == keyUp, code == kobo.KeyTab && shift:
		return -1
	case code == keyRight, code == keyDown, code == kobo.KeyTab:
		return 1
	}
	return 0
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
