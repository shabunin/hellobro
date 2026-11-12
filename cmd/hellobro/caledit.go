package main

// The calendar's dialog: an event or a task, new or to change.

import (
	"fmt"
	"strings"
	"time"

	"hellobro/cal"
	"hellobro/ink"
	"hellobro/ink/layout"
	"hellobro/ink/theme"
	"hellobro/ink/widget"
)

type calEdit struct {
	it     cal.Item // a copy: Save takes it
	isNew  bool
	was    cal.Item // as it was: what isn't changed stays so
	in     string   // its calendar's ID
	title  widget.Editor
	place  widget.Editor
	kind   widget.Enum // a new one's: "event" or "task"
	allDay widget.Bool
	hasDue widget.Bool
	done   widget.Bool
	day    widget.Date
	start  widget.Clock
	long   widget.Select
	repeat widget.Select
	reps   []string // repeat's choices, as RRULE has them
	prio   widget.Select
	calSel widget.Select
	cals   []cal.Folder // calSel's

	save, cancel, del widget.Clickable
	sure              bool   // delete was tapped once
	from              string // a subscription's name: it's read only
}

var (
	evMins      = []int{15, 30, 45, 60, 90, 120, 180, 240}
	evDays      = []int{1, 2, 3, 4, 5, 7, 14}
	repRules    = []string{"", "FREQ=DAILY", "FREQ=WEEKLY", "FREQ=MONTHLY", "FREQ=YEARLY"}
	repNames    = []string{"doesn't repeat", "every day", "every week", "every month", "every year"}
	calEditKind = []string{"event", "task"}
)

// startNew opens the dialog for a new item: a task in the tasks, else
// an event on the day shown, at the next hour (today) or at 10.
func (u *calUI) startNew(gtx ink.Ctx) {
	kind := cal.Event
	if u.view.Value == "tasks" {
		kind = cal.Task
	}
	it := cal.New(kind)
	d := dayOf(u.at)
	h := 10
	if d.Equal(dayOf(gtx.Now)) {
		h = min(gtx.Now.Hour()+1, 23)
	}
	if kind == cal.Event {
		it.Start = d.Add(time.Duration(h) * time.Hour)
		it.End = it.Start.Add(time.Hour)
	}
	u.ed = u.newEdit(it, u.st.Home(kind), true)
	gtx.Focus(&u.ed.title)
}

// edit opens the dialog for e.
func (u *calUI) edit(e *cal.Entry) {
	u.ed = u.newEdit(e.Item, e.In.ID, false)
	if e.In.ReadOnly() {
		u.ed.from = e.In.Name
	}
}

func (u *calUI) newEdit(it *cal.Item, in string, isNew bool) *calEdit {
	ed := &calEdit{it: *it, was: *it, in: in, isNew: isNew}
	ed.kind.Value = map[string]string{cal.Event: "event", cal.Task: "task"}[it.Kind]
	ed.title.SetText(it.Summary)
	ed.place.SetText(it.Location)
	ed.allDay.Value = it.AllDay || it.Kind == cal.Task && it.End.IsZero()
	ed.done.Value = it.Done
	ed.start.Step = 15
	ed.prio.Value = prioIndex(it.Priority)
	ed.setWhen()
	ed.reps = append([]string{}, repRules...)
	ed.repeat.Value = -1
	for i, r := range repRules {
		if strings.EqualFold(r, it.RRule) {
			ed.repeat.Value = i
		}
	}
	if ed.repeat.Value < 0 { // its own: kept as it is, or not at all
		ed.reps = append([]string{it.RRule}, repRules...)
		ed.repeat.Value = 0
	}
	for _, f := range u.st.Folders() {
		if !f.ReadOnly() {
			ed.cals = append(ed.cals, f)
		}
	}
	for i, f := range ed.cals {
		if f.ID == in {
			ed.calSel.Value = i
		}
	}
	return ed
}

// setWhen puts the item's times into the pickers: an event's start and
// length, a task's due.
func (ed *calEdit) setWhen() {
	it := &ed.it
	at := it.Start
	if it.Kind == cal.Task {
		at = it.End
		ed.hasDue.Value = !at.IsZero()
		if at.IsZero() {
			at = dayOf(time.Now()).Add(18 * time.Hour)
		}
	}
	ed.day.Value = dayOf(at)
	ed.start.Hour, ed.start.Minute = at.Hour(), at.Minute()
	ed.long.Value = 3 // an hour
	if it.Kind == cal.Event {
		if it.AllDay {
			n := int(it.End.Sub(it.Start).Hours()/24 + 0.5)
			ed.long.Value = max(0, indexNear(evDays, n))
		} else {
			ed.long.Value = max(0, indexNear(evMins, int(it.End.Sub(it.Start).Minutes())))
		}
	}
}

// indexNear is the index of the value in vs nearest v.
func indexNear(vs []int, v int) int {
	best := 0
	for i, x := range vs {
		if abs(x-v) < abs(vs[best]-v) {
			best = i
		}
	}
	return best
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// lengths are the length's choices: minutes, or days if it's all day.
func (ed *calEdit) lengths() []string {
	var ls []string
	if ed.allDay.Value {
		for _, d := range evDays {
			ls = append(ls, map[bool]string{true: "1 day", false: fmt.Sprintf("%d days", d)}[d == 1])
		}
		return ls
	}
	for _, m := range evMins {
		switch {
		case m < 60:
			ls = append(ls, fmt.Sprintf("%d min", m))
		case m%60 == 0:
			ls = append(ls, map[bool]string{true: "1 hour", false: fmt.Sprintf("%d hours", m/60)}[m == 60])
		default:
			ls = append(ls, fmt.Sprintf("%d h %d min", m/60, m%60))
		}
	}
	return ls
}

// apply puts what the dialog says into ed.it.
func (ed *calEdit) apply() {
	it := &ed.it
	it.Kind = map[string]string{"event": cal.Event, "task": cal.Task}[ed.kind.Value]
	if ed.isNew && it.Kind != ed.was.Kind { // a new one made the other kind
		n := cal.New(it.Kind)
		n.UID = it.UID
		it.Cal = n.Cal
	}
	it.Summary = strings.TrimSpace(ed.title.Text())
	if it.Summary == "" {
		it.Summary = "(no title)"
	}
	it.Location = strings.TrimSpace(ed.place.Text())
	d := dayOf(ed.day.Value)
	if ed.day.Value.IsZero() {
		d = dayOf(time.Now())
	}
	at := d.Add(time.Duration(ed.start.Hour)*time.Hour + time.Duration(ed.start.Minute)*time.Minute)
	if it.Kind == cal.Task {
		it.Done, it.Priority = ed.done.Value, prios[ed.prio.Value]
		it.RRule = ""
		if ed.was.Kind == cal.Task {
			it.RRule = ed.was.RRule // a task that repeats: kept, not offered
		}
		switch {
		case !ed.hasDue.Value:
			it.End = time.Time{}
		case ed.allDay.Value:
			it.End, it.AllDay = d, true
		default:
			it.End, it.AllDay = at, false
		}
		if it.AllDay && !it.Start.IsZero() {
			it.Start = dayOf(it.Start)
		}
		if !it.Start.IsZero() && !it.End.IsZero() && it.Start.After(it.End) {
			it.Start = time.Time{}
		}
		return
	}
	it.AllDay = ed.allDay.Value
	if it.AllDay {
		it.Start = d
		it.End = d.AddDate(0, 0, evDays[min(ed.long.Value, len(evDays)-1)])
	} else {
		it.Start = at
		it.End = at.Add(time.Duration(evMins[min(ed.long.Value, len(evMins)-1)]) * time.Minute)
	}
	it.RRule = ed.reps[min(ed.repeat.Value, len(ed.reps)-1)]
	if it.RRule != ed.was.RRule {
		it.ExDates = nil
	}
}

func (ed *calEdit) layout(gtx ink.Ctx, th *theme.Theme, u *calUI) {
	if ed.cancel.Clicked(gtx) {
		u.ed = nil
		return
	}
	if ed.from != "" {
		theme.Dialog(th, ed.it.Summary, calSays(&ed.it)+"\nfrom "+ed.from+", read only", theme.Button(th, &ed.cancel, "close")).Layout(gtx)
		return
	}
	if ed.del.Clicked(gtx) {
		if !ed.sure {
			ed.sure = true
		} else {
			u.saved(u.st.Delete(ed.it.UID, time.Now()))
			u.toast.Show(gtx, "deleted")
			u.ed = nil
			return
		}
	}
	if ed.allDay.Update(gtx) && ed.it.Kind == cal.Event {
		ed.long.Value = 0
		if !ed.allDay.Value {
			ed.long.Value = 3
		}
	}
	ed.hasDue.Update(gtx)
	ed.done.Update(gtx)
	if ed.kind.Update(gtx) {
		ed.it.Kind = map[string]string{"event": cal.Event, "task": cal.Task}[ed.kind.Value]
	}
	lens := ed.lengths()
	ed.long.Update(gtx, len(lens))
	repL := repNames
	if len(ed.reps) > len(repRules) {
		repL = append([]string{"as it was: " + ed.reps[0]}, repNames...)
	}
	ed.repeat.Update(gtx, len(repL))
	ed.prio.Update(gtx, len(prioNames))
	var calNames []string
	for _, f := range ed.cals {
		calNames = append(calNames, f.Name)
	}
	ed.calSel.Update(gtx, len(calNames))
	if ed.save.Clicked(gtx) || ed.title.Submitted(gtx) {
		ed.apply()
		in := ed.in
		if ed.calSel.Value < len(ed.cals) {
			f := ed.cals[ed.calSel.Value]
			if f.Holds(ed.it.Kind) {
				in = f.ID
			}
		}
		if in == "" || ed.isNew && ed.it.Kind != ed.was.Kind && in == ed.in {
			in = u.st.Home(ed.it.Kind)
		}
		it := ed.it
		_, err := u.st.Save(in, &it, time.Now())
		u.saved(err)
		if err == nil {
			u.toast.Show(gtx, "saved")
			if it.Kind == cal.Event {
				u.at = dayOf(it.Start)
			}
		}
		u.ed = nil
		return
	}

	task := ed.it.Kind == cal.Task
	title := "Event"
	if task {
		title = "Task"
	}
	if ed.isNew {
		title = "A new " + strings.ToLower(title)
	}
	save := theme.Button(th, &ed.save, "save")
	save.Strong = true
	buttons := []theme.ButtonStyle{theme.Button(th, &ed.cancel, "cancel"), save}
	if !ed.isNew {
		del := "delete"
		if ed.sure {
			del = "delete it?"
		}
		buttons = append([]theme.ButtonStyle{theme.Button(th, &ed.del, del)}, buttons...)
	}
	dl := theme.Dialog(th, title, "", buttons...)
	gap := layout.Rigid(layout.Spacer{Height: th.Pad / 2}.Layout)
	dl.Body = func(gtx ink.Ctx) ink.Dims {
		var kids []layout.FlexChild
		if ed.isNew {
			kids = append(kids, layout.Rigid(theme.Tabs(th, &ed.kind, calEditKind, calEditKind).Layout), gap)
		}
		hint := "what is it?"
		if task {
			hint = "what's to do?"
		}
		kids = append(kids, layout.Rigid(theme.Editor(th, &ed.title, hint).Layout), gap)
		if !task {
			kids = append(kids, layout.Rigid(theme.Editor(th, &ed.place, "where? (if it's somewhere)").Layout), gap)
		}
		on := "On"
		if !task && ed.was.RRule != "" && !ed.isNew {
			on = "From" // a series: its first day
		}
		if task {
			kids = append(kids, layout.Rigid(theme.CheckBox(th, &ed.done, "done").Layout),
				layout.Rigid(theme.CheckBox(th, &ed.hasDue, "a day it's due").Layout))
			if ed.hasDue.Value {
				kids = append(kids, layout.Rigid(theme.Date(th, &ed.day, "Due").Layout),
					layout.Rigid(theme.CheckBox(th, &ed.allDay, "any time that day").Layout))
				if !ed.allDay.Value {
					kids = append(kids, layout.Rigid(theme.Clock(th, &ed.start, "By").Layout))
				}
			}
			kids = append(kids, layout.Rigid(theme.Select(th, &ed.prio, "Priority", prioNames...).Layout))
		} else {
			kids = append(kids, layout.Rigid(theme.CheckBox(th, &ed.allDay, "all day").Layout),
				layout.Rigid(theme.Date(th, &ed.day, on).Layout))
			if !ed.allDay.Value {
				kids = append(kids, layout.Rigid(theme.Clock(th, &ed.start, "At").Layout))
			}
			kids = append(kids, layout.Rigid(theme.Select(th, &ed.long, "For", lens...).Layout),
				layout.Rigid(theme.Select(th, &ed.repeat, "Repeats", repL...).Layout))
		}
		if len(calNames) > 1 {
			kids = append(kids, layout.Rigid(theme.Select(th, &ed.calSel, "In", calNames...).Layout))
		}
		if d := strings.TrimSpace(ed.it.Description); d != "" {
			kids = append(kids, gap, layout.Rigid(theme.Small(th, firstLines(d, 3)).Layout))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
	}
	dl.Layout(gtx)
}

// firstLines is s's first n lines, cut short.
func firstLines(s string, n int) string {
	ls := strings.Split(s, "\n")
	if len(ls) > n {
		ls = append(ls[:n], "…")
	}
	for i, l := range ls {
		if r := []rune(l); len(r) > 90 {
			ls[i] = string(r[:90]) + "…"
		}
	}
	return strings.Join(ls, "\n")
}

// calSays is an item, in a few lines: when, where, what about, from where.
func calSays(it *cal.Item) string {
	var ls []string
	switch {
	case it.Kind == cal.Task && it.End.IsZero():
		ls = append(ls, "a task")
	case it.Kind == cal.Task && it.AllDay:
		ls = append(ls, "a task, due "+it.End.Format("Monday 2 January"))
	case it.Kind == cal.Task:
		ls = append(ls, "a task, due "+it.End.Format("Monday 2 January, 15:04"))
	case it.AllDay:
		s := it.Start.Format("Monday 2 January")
		if last := it.End.AddDate(0, 0, -1); last.After(it.Start) {
			s += " to " + last.Format("Monday 2 January")
		}
		ls = append(ls, s+", all day")
	default:
		ls = append(ls, it.Start.Format("Monday 2 January, 15:04")+"–"+it.End.Format("15:04"))
	}
	if it.RRule != "" {
		ls = append(ls, "repeats ("+strings.ToLower(it.RRule)+")")
	}
	if it.Location != "" {
		ls = append(ls, it.Location)
	}
	if d := strings.TrimSpace(it.Description); d != "" {
		ls = append(ls, firstLines(d, 4))
	}
	return strings.Join(ls, "\n")
}
