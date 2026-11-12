package main

// The question over the page, in ink: a card with its title, its words,
// the box askTyped's words go in, and its answers. The keys come to its
// pane while it's up (askKey); a contact on it is the pane's (askDown),
// and a tap outside it is no (askTap).

import (
	"hellobro/ink"
	"hellobro/ink/layout"
	"hellobro/ink/theme"
	"hellobro/ink/widget"
)

// askUI lays the question out.
type askUI struct {
	a     *bro
	q     *askBox
	btn   []widget.Clickable
	typed widget.Editor
	selAt int // the answer the keys were put on, -1: none yet
	pick  int // the answer picked in the frame, -2: none (-1 is none of them, Esc)
}

// picked is the answer picked, taken; ok false if none was. The answer
// is acted on after the frame, not in it: the page drawn again under a
// frame still going would have the card's last pixels put over it.
func (u *askUI) picked() (int, bool) {
	i := u.pick
	u.pick = -2
	return i, i != -2
}

func newAskUI(a *bro, q *askBox) *askUI {
	return &askUI{a: a, q: q, btn: make([]widget.Clickable, len(q.opts)), selAt: -1, pick: -2}
}

func (u *askUI) Layout(gtx ink.Ctx) ink.Dims {
	q, th := u.q, u.a.theme()
	// The keys start on the answer Enter picks, or in the box to type in.
	if u.selAt != q.sel {
		u.selAt = q.sel
		if q.want != "" {
			gtx.Focus(&u.typed)
		} else {
			gtx.Focus(&u.btn[q.sel])
		}
	}
	for _, e := range gtx.Events(ink.Root) {
		switch e := e.(type) {
		case ink.Key:
			switch e.Name {
			case "Esc":
				u.pick = -1
			case "Enter":
				u.pick = q.sel
			}
		case ink.Text:
			for i, o := range q.opts {
				if o.key != "" && o.key == e.Text {
					u.pick = i
				}
			}
		}
	}
	if q.want != "" {
		if u.typed.Changed(gtx) {
			q.typed = u.typed.Text()
		}
		if u.typed.Submitted(gtx) {
			q.typed = u.typed.Text()
			u.pick = 0
		}
	}
	for i := range u.btn {
		if u.btn[i].Clicked(gtx) {
			u.pick = i
		}
	}

	return card(gtx, th, &q.over.h, func(gtx ink.Ctx) ink.Dims {
		return layout.UniformInset(th.Pad*2.5).Layout(gtx, func(gtx ink.Ctx) ink.Dims {
			kids := []layout.FlexChild{layout.Rigid(theme.Title(th, q.title).Layout)}
			if q.text != "" {
				l := theme.Body(th, q.text)
				l.Gray = th.Quiet
				kids = append(kids, layout.Rigid(layout.Spacer{Height: th.Pad}.Layout), layout.Rigid(l.Layout))
			}
			if q.want != "" {
				kids = append(kids, layout.Rigid(layout.Spacer{Height: th.Pad * 1.5}.Layout),
					layout.Rigid(theme.Editor(th, &u.typed, "type: "+q.want).Layout))
			}
			kids = append(kids, layout.Rigid(layout.Spacer{Height: th.Pad * 2}.Layout), layout.Rigid(u.buttons))
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
		})
	})
}

// buttons are the answers: side by side at the right if two (the second
// no), else one under another, as wide as the card.
func (u *askUI) buttons(gtx ink.Ctx) ink.Dims {
	q, th := u.q, u.a.theme()
	button := func(i int) ink.Widget {
		o := q.opts[i]
		label := o.label
		switch {
		case gtx.Focused(&u.btn[i]):
			label += "  (Enter)"
		case o.do == nil:
			label += "  (Esc)"
		case o.key != "":
			label += "  (" + o.key + ")"
		}
		bs := theme.Button(th, &u.btn[i], label)
		bs.Strong = gtx.Focused(&u.btn[i]) && (i != 0 || q.ready())
		return func(gtx ink.Ctx) ink.Dims {
			if i == 0 && !q.ready() {
				gtx = gtx.Disabled() // not yet: the words aren't typed
			}
			return bs.Layout(gtx)
		}
	}
	if len(q.opts) == 2 {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Spacing: layout.SpaceStart}.Layout(gtx,
			layout.Rigid(button(0)), layout.Rigid(layout.Spacer{Width: th.Pad * 1.5}.Layout), layout.Rigid(button(1)))
	}
	var kids []layout.FlexChild
	for i := range q.opts {
		if i > 0 {
			kids = append(kids, layout.Rigid(layout.Spacer{Height: th.Pad}.Layout))
		}
		b := button(i)
		kids = append(kids, layout.Rigid(func(gtx ink.Ctx) ink.Dims {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return b(gtx)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
}
