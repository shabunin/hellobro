package main

// Which calendars show: a dialog of them, ticked when shown. A hidden one
// is still kept and synced; it's only left out of the views, the tasks
// and the sleep screen's agenda, and new items don't go into it.

import (
	"hellobro/cal"
	"hellobro/ink"
	"hellobro/ink/layout"
	"hellobro/ink/theme"
	"hellobro/ink/widget"
)

type calPick struct {
	fs    []cal.Folder
	shown []widget.Bool
	done  widget.Clickable
}

// pickCals opens the dialog, with the calendars as they are now.
func (u *calUI) pickCals() {
	p := &calPick{fs: u.st.Folders()}
	p.shown = make([]widget.Bool, len(p.fs))
	for i, f := range p.fs {
		p.shown[i].Value = !f.Hidden
	}
	u.cals = p
}

// calName is how a calendar is called in the list.
func calName(f cal.Folder) string {
	n := f.Name
	if n == "" {
		n = f.ID
	}
	switch {
	case f.ReadOnly():
		n += " · subscribed, read only"
	case f.Href == "":
		n += " · on this Kobo only"
	}
	return n
}

func (p *calPick) layout(gtx ink.Ctx, th *theme.Theme, u *calUI) {
	for i := range p.shown {
		if p.shown[i].Update(gtx) {
			if err := u.st.SetHidden(p.fs[i].ID, !p.shown[i].Value); err != nil {
				u.toast.Show(gtx, "not changed: "+err.Error())
				p.shown[i].Value = !p.shown[i].Value
			}
		}
	}
	if p.done.Clicked(gtx) {
		u.cals = nil
		return
	}
	body := func(gtx ink.Ctx) ink.Dims {
		kids := make([]layout.FlexChild, len(p.fs))
		for i, f := range p.fs {
			kids[i] = layout.Rigid(theme.CheckBox(th, &p.shown[i], calName(f)).Layout)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
	}
	done := theme.Button(th, &p.done, "done")
	done.Strong = true
	d := theme.Dialog(th, "Calendars", "The ticked ones show, here and on the sleep screen. Those hidden are still synced.", done)
	d.Body = body
	d.Layout(gtx)
}
