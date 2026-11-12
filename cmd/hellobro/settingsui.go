package main

// The Settings window, in ink (inkwin.go): each setting a dropdown of its
// values, under its group's heading; the ones that open something (the
// keys, the languages, removing Alpine) a row that does. A pick is kept
// at once (pickSetting).

import (
	"slices"

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
	fl   []widget.Float     // the sliders'
	held []bool             // a slider moved, its value not yet picked
}

func newSettingsUI(a *bro) inkUI {
	n := len(settingList)
	u := &settingsUI{a: a, sel: make([]widget.Select, n), act: make([]widget.Clickable, n), fl: make([]widget.Float, n), held: make([]bool, n)}
	for i, st := range settingList {
		u.fl[i] = widget.Float{Min: 0, Max: float32(len(st.values) - 1), Step: 1}
	}
	return u
}

func (u *settingsUI) Status() string { return "" }

// opens says setting st is a row that opens something, not a choice.
func (st setting) opens() bool {
	return st.key == "keys" || st.key == "alpine-remove" || st.key == "languages" || st.key == "ai-prompts"
}

func (u *settingsUI) Layout(gtx ink.Ctx) ink.Dims {
	a, th := u.a, u.a.theme()
	for i, st := range settingList {
		if u.act[i].Clicked(gtx) {
			a.pickSetting(i, a.settingAt(i))
		}
		if st.slider {
			// Picked when the finger lifts: the text's size changing
			// under it would move the track away.
			f := &u.fl[i]
			if f.Changed(gtx) {
				u.held[i] = true
			}
			if u.held[i] && !f.Dragging() {
				u.held[i] = false
				if v := int(f.Value + 0.5); v != a.settingAt(i) {
					a.pickSetting(i, v)
				}
			}
			if !u.held[i] {
				f.Value = float32(a.settingAt(i))
			}
			continue
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
		var row ink.Widget
		switch {
		case st.opens(), st.key == "share" && a.share.code != "":
			r := theme.Row(th, &u.act[i], label)
			r.Sub, r.More = a.settingShown(i), st.opens()
			row = r.Layout
		case st.slider:
			v := st.shown[min(max(int(u.fl[i].Value+0.5), 0), len(st.shown)-1)]
			sl := theme.Slider(th, &u.fl[i], label, v)
			sl.Ticks = true
			row = func(gtx ink.Ctx) ink.Dims { return layout.Inset{Top: th.Pad}.Layout(gtx, sl.Layout) } // as a dropdown's row has
		case st.key == "ai-model": // the API's list, fetched when it's old
			a.modelsWant()
			shown := slices.Clone(st.shown)
			shown[0] = "as ai.conf says: " + a.diaryConfModel()
			row = theme.Select(th, &u.sel[i], label, shown...).Layout
		default:
			row = theme.Select(th, &u.sel[i], label, st.shown...).Layout
		}
		if st.help != "" {
			row = withHelp(th, row, st.help)
		}
		rows = append(rows, row)
	}
	u.list.Axis = layout.Vertical
	return layout.UniformInset(th.Pad).Layout(gtx, func(gtx ink.Ctx) ink.Dims {
		return theme.List(th, &u.list).Layout(gtx, len(rows), func(gtx ink.Ctx, i int) ink.Dims { return rows[i](gtx) })
	})
}

// withHelp is row with a quiet line under it, saying what it's for.
func withHelp(th *theme.Theme, row ink.Widget, help string) ink.Widget {
	return func(gtx ink.Ctx) ink.Dims {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(row),
			layout.Rigid(func(gtx ink.Ctx) ink.Dims {
				// Top: clear of a focus ring round the row.
				return layout.Inset{Top: th.Pad / 2, Left: th.Pad * 1.5, Right: th.Pad * 1.5, Bottom: th.Pad}.Layout(gtx, theme.Small(th, help).Layout)
			}))
	}
}
