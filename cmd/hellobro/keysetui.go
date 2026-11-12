package main

// The question for a command's new key, in ink: what it has now, its
// defaults, why the last key won't do; no key, the defaults back, or
// as it is. The keys pressed are keySetKey's, not the card's: any of
// them may be the answer.

import (
	"hellobro/ink"
	"hellobro/ink/layout"
	"hellobro/ink/theme"
	"hellobro/ink/widget"
)

type keySetUI struct {
	a              *bro
	k              *keySet
	none, def, now widget.Clickable
	act            string // what the frame picked: "none", "default", "back"
}

func (u *keySetUI) Layout(gtx ink.Ctx) ink.Dims {
	k, th := u.k, u.a.theme()
	for u.none.Clicked(gtx) {
		u.act = "none"
	}
	for u.def.Clicked(gtx) {
		u.act = "default"
	}
	for u.now.Clicked(gtx) {
		u.act = "back"
	}
	now, def := u.a.keySetWords(k.c)
	say := "Press the new key, with Ctrl, Alt or Mod (an F-key alone will do)."
	if k.note != "" {
		say = k.note
	}
	return card(gtx, th, &k.over.h, func(gtx ink.Ctx) ink.Dims {
		return layout.UniformInset(th.Pad*2.5).Layout(gtx, func(gtx ink.Ctx) ink.Dims {
			has := theme.Body(th, "now: "+now+"   ·   by default: "+def)
			has.Gray = th.Quiet
			sp := layout.Rigid(layout.Spacer{Width: th.Pad * 1.5}.Layout)
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(theme.Title(th, "Key for: "+k.c.title).Layout),
				layout.Rigid(layout.Spacer{Height: th.Pad}.Layout),
				layout.Rigid(has.Layout),
				layout.Rigid(layout.Spacer{Height: th.Pad * 2}.Layout),
				layout.Rigid(theme.Label(th, th.TextSize*1.15, say).Layout),
				layout.Rigid(layout.Spacer{Height: th.Pad * 3}.Layout),
				layout.Rigid(func(gtx ink.Ctx) ink.Dims {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return layout.Flex{Spacing: layout.SpaceStart}.Layout(gtx,
						layout.Rigid(theme.Button(th, &u.none, "no key  (Backspace)").Layout), sp,
						layout.Rigid(theme.Button(th, &u.def, "default  (Delete)").Layout), sp,
						layout.Rigid(theme.Button(th, &u.now, "as it is  (Esc)").Layout))
				}))
		})
	})
}
