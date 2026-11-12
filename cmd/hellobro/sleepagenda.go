package main

// The sleep screen as an agenda (Settings, Power: the sleep screen): the
// day's date, what's on today and the days after, as much as fits, the
// tasks to do; drawn again at midnight (the Kobo wakes a moment for it).
// It's the calendar as last synced: the radios are off while it sleeps.

import (
	"fmt"
	"image"
	"os"
	"strings"
	"time"

	"hellobro/cal"
	"hellobro/ink"
	"hellobro/ink/inkkobo"
	"hellobro/ink/text"
	"hellobro/kobo"
)

// SleepScreen draws the agenda on c, if the setting says so.
func (a *bro) SleepScreen(c kobo.Canvas, now time.Time) (time.Time, bool) {
	if a.setting("sleep-screen") != "agenda" {
		return time.Time{}, false
	}
	st := a.calStore()
	if st == nil {
		return time.Time{}, false
	}
	battery := ""
	if b, err := os.ReadFile("/sys/class/power_supply/battery/capacity"); err == nil {
		battery = strings.TrimSpace(string(b)) + "%"
	}
	ag := sleepAgenda{a: a, st: st, now: now, battery: battery}
	p := inkkobo.NewPane(ag.layout)
	p.Place(c.Bounds())
	p.Draw(c.Gray)
	tomorrow := dayOf(now).AddDate(0, 0, 1)
	return tomorrow.Add(30 * time.Second), true
}

type sleepAgenda struct {
	a       *bro
	st      *cal.Store
	now     time.Time
	battery string
}

// agendaRow is a line of it: a day's heading, or an item.
type agendaRow struct {
	head string
	o    *cal.Occurrence
	task *cal.Entry // a task to do, not on a day
}

func (s sleepAgenda) layout(gtx ink.Ctx) ink.Dims {
	th := s.a.theme()
	size := gtx.Constraints.Max
	o := gtx.Ops
	ink.Fill(o, image.Rectangle{Max: size}, ink.White)
	pad := gtx.Dp(th.Pad * 4)
	huge := th.Face(gtx, th.TextSize*3, text.Bold)
	big := th.Face(gtx, th.TextSize*1.6, text.Regular)
	f := th.Face(gtx, th.TextSize*1.15, text.Regular)
	bold := th.Face(gtx, th.TextSize*1.15, text.Bold)
	small := th.Face(gtx, th.TextSize, text.Regular)
	w := size.X - 2*pad

	// The date, and how it sleeps.
	y := pad + text.Ascent(huge)
	ink.TextAt(o, huge, pad, y, s.now.Format("Monday"), ink.Black)
	y += text.Height(big) + gtx.Dp(6)
	ink.TextAt(o, big, pad, y, s.now.Format("2 January 2006"), ink.Black)
	note := "sleeping since " + s.now.Format("15:04")
	if s.battery != "" {
		note += " · battery " + s.battery
	}
	ink.TextAt(o, small, pad+w-text.Width(small, note), y, note, ink.Dark)
	y += gtx.Dp(18)
	ink.Fill(o, image.Rect(pad, y, pad+w, y+max(gtx.Dp(3), 3)), ink.Black)
	y += gtx.Dp(14)

	foot := "Press the power button to wake up."
	footY := size.Y - pad/2
	ink.TextAt(o, small, (size.X-text.Width(small, foot))/2, footY, foot, ink.Mid)
	room := footY - text.Height(small) - gtx.Dp(8)

	rows := s.rows()
	if len(rows) == 0 {
		ink.TextAt(o, big, pad, y+text.Height(big), "nothing on, nothing to do", ink.Dark)
		return ink.Dims{Size: size}
	}
	lineH := text.Height(f) + gtx.Dp(14)
	headH := text.Height(bold) + gtx.Dp(22)
	timeW := text.Width(small, "00:00–00:00") + gtx.Dp(24)
	for i, r := range rows {
		h := lineH
		if r.head != "" {
			h = headH
		}
		if y+h > room || r.head != "" && y+h+lineH > room { // no room for the rest (nor a heading alone): how many there are
			n := 0
			for _, q := range rows[i:] {
				if q.head == "" {
					n++
				}
			}
			if n > 0 {
				ink.TextAt(o, small, pad, y+text.Ascent(small)+gtx.Dp(6), fmt.Sprintf("and %d more", n), ink.Dark)
			}
			break
		}
		switch {
		case r.head != "":
			ink.TextAt(o, bold, pad, y+gtx.Dp(14)+text.Ascent(bold), r.head, ink.Black)
			ink.Fill(o, image.Rect(pad, y+h-gtx.Dp(3), pad+w, y+h-gtx.Dp(2)), ink.Mid)
		default:
			var it *cal.Item
			when := ""
			if r.task != nil {
				it = r.task.Item
				if !it.End.IsZero() {
					when = "late"
				}
			} else {
				it = r.o.Item
				switch {
				case it.Kind == cal.Task && it.AllDay:
					when = "due"
				case it.Kind == cal.Task:
					when = "by " + r.o.At.Format("15:04")
				case it.AllDay:
					when = "all day"
				default:
					when = r.o.At.Format("15:04") + "–" + r.o.End().Format("15:04")
				}
			}
			base := y + (lineH+text.Ascent(f)-f.Metrics().Descent.Ceil())/2
			ink.TextAt(o, small, pad, base, when, ink.Dark)
			x := pad + timeW
			if it.Kind == cal.Task {
				bs := text.Ascent(f) * 3 / 4
				box := image.Rect(x, base-bs, x+bs, base)
				ink.Frame(o, box, max(gtx.Dp(2), 2), ink.Black)
				x = box.Max.X + gtx.Dp(12)
			}
			title := it.Summary
			ink.TextAt(o, f, x, base, text.Fit(f, title, pad+w-x), ink.Black)
			if it.Location != "" {
				if tw := text.Width(f, title) + gtx.Dp(16); x+tw < pad+w-gtx.Dp(60) {
					ink.TextAt(o, small, x+tw, base, text.Fit(small, it.Location, pad+w-x-tw), ink.Dark)
				}
			}
		}
		y += h
	}
	return ink.Dims{Size: size}
}

// rows are today's and tomorrow's, the tasks to do with no day and
// those late, then the next two weeks' days with something on.
func (s sleepAgenda) rows() []agendaRow {
	today := dayOf(s.now)
	occ := s.st.Between(today, today.AddDate(0, 0, 14))
	var rows []agendaRow
	for i := 0; i < 14; i++ {
		if i == 2 {
			rows = append(rows, s.todo(today)...)
		}
		d := today.AddDate(0, 0, i)
		timed, ad := onDay(occ, d)
		var day []agendaRow
		for k, o := range append(ad, timed...) {
			if i == 0 && o.Item.Kind == cal.Event && !o.Item.AllDay && o.End().Before(s.now) {
				continue // over already
			}
			day = append(day, agendaRow{o: occAt(ad, timed, k)})
		}
		if len(day) == 0 {
			continue
		}
		head := d.Format("Monday 2 January")
		switch i {
		case 0:
			head = "Today"
		case 1:
			head = "Tomorrow"
		}
		rows = append(append(rows, agendaRow{head: head}), day...)
	}
	return rows
}

// occAt is the kth of ad and timed, one after the other.
func occAt(ad, timed []cal.Occurrence, k int) *cal.Occurrence {
	if k < len(ad) {
		return &ad[k]
	}
	return &timed[k-len(ad)]
}

// todo is the tasks to do with no day, and those late, under a heading.
func (s sleepAgenda) todo(today time.Time) []agendaRow {
	var rows []agendaRow
	for _, e := range s.st.Tasks() {
		if !e.Item.Done && (e.Item.End.IsZero() || e.Item.End.Before(today)) {
			rows = append(rows, agendaRow{task: e})
		}
	}
	if len(rows) == 0 {
		return nil
	}
	return append([]agendaRow{{head: "To do"}}, rows...)
}
