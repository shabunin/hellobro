package main

// The Settings window, in ink (inkwin.go): each setting a dropdown of its
// values, under its group's heading; the ones that open something (the
// keys, the languages, removing Alpine) a row that does. A pick is kept
// at once (pickSetting).

import (
	"hellobro/ink"
	"hellobro/ink/layout"
	"hellobro/ink/theme"
	"hellobro/ink/widget"
)

type settingsUI struct {
	a    *bro
	list layout.Scroll
	sel  []widget.Select    // by settingList's index
	act  []widget.Clickable // the rows that open something
}

func (u *settingsUI) Status() string { return "" }

// opens says setting st is a row that opens something, not a choice.
func (st setting) opens() bool {
	return st.key == "keys" || st.key == "alpine-remove" || st.key == "languages"
}

func (u *settingsUI) Layout(gtx ink.Ctx) ink.Dims {
	a, th := u.a, u.a.theme()
	for i, st := range settingList {
		if st.app != "" {
			continue
		}
		if u.act[i].Clicked(gtx) {
			a.pickSetting(i, a.settingAt(i))
		}
		s := &u.sel[i]
		if s.Changed(gtx, len(st.values)) {
			a.pickSetting(i, s.Value)
		}
		if !s.IsOpen() {
			s.Value = a.settingAt(i) // as it is: a command, a key may have changed it
		}
	}
	rows := make([]ink.Widget, 0, len(settingLines))
	for _, l := range settingLines {
		if l.head != "" {
			rows = append(rows, theme.Section(th, l.head))
			continue
		}
		i := l.i
		st := settingList[i]
		label := a.settingLabel(st)
		switch {
		case st.opens(), st.key == "share" && a.share.code != "":
			r := theme.Row(th, &u.act[i], label)
			r.Sub, r.More = a.settingShown(i), st.opens()
			rows = append(rows, r.Layout)
		default:
			rows = append(rows, theme.Select(th, &u.sel[i], label, st.shown...).Layout)
		}
	}
	u.list.Axis = layout.Vertical
	return layout.UniformInset(th.Pad).Layout(gtx, func(gtx ink.Ctx) ink.Dims {
		return theme.List(th, &u.list).Layout(gtx, len(rows), func(gtx ink.Ctx, i int) ink.Dims { return rows[i](gtx) })
	})
}
