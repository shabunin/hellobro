package main

// The calendar: events and tasks (cal/), kept in the app's folder
// (calendar/) and, with calendar.conf there, in step with a CalDAV
// server's: when it opens, every quarter of an hour while it's open, a
// few seconds after a change, and when asked (sync, or s).

import (
	"context"
	"errors"
	"fmt"
	"image"
	"log"
	"os"
	"strings"
	"time"

	"hellobro/cal"
	"hellobro/ink"
	"hellobro/ink/layout"
	"hellobro/ink/text"
	"hellobro/ink/theme"
	"hellobro/ink/widget"
	"hellobro/kobo"
)

const (
	calKey   = "calendar"
	calConf  = "calendar.conf"
	calAfter = 5 * time.Second // a change goes to the server this long after it's made
)

var (
	calViews     = []string{"day", "week", "month", "agenda", "tasks"}
	calViewNames = []string{"day", "week", "month", "agenda", "tasks"}
)

type calUI struct {
	a       *bro
	st      *cal.Store
	openErr string

	view                              widget.Enum
	todayB, prevB, nextB, addB, syncB widget.Clickable
	at                                time.Time // a day in what's shown
	hours, agenda, tasks              layout.Scroll
	scrolled                          bool
	btn                               map[string]*widget.Clickable // an item's, at a time: by uid and time
	used                              map[string]bool
	dayBtn                            [42]widget.Clickable
	colBtn                            [7]widget.Clickable
	showDone                          widget.Bool
	quick                             widget.Editor // a task, typed

	ed *calEdit // the dialog: an item being changed

	syncing, again bool
	synced         time.Time // last in step
	said           string    // what went wrong, if it did
	next           time.Time // the next sync; zero: none
	wantSync       bool
	typing         bool // an editor had the focus at the last frame
	vk             int  // 1: ask for the keyboard, -1: let it go (after the frame)
	toast          widget.Toast
}

func newCalUI(a *bro) inkUI {
	u := &calUI{a: a, btn: map[string]*widget.Clickable{}, used: map[string]bool{}}
	u.view.Value = "week"
	u.hours.Axis, u.agenda.Axis, u.tasks.Axis = layout.Vertical, layout.Vertical, layout.Vertical
	st, err := cal.Open(kobo.AppFile("calendar"))
	if err != nil {
		log.Printf("calendar: %v", err)
		u.openErr = err.Error()
		return u
	}
	u.st = st
	return u
}

func (u *calUI) Status() string {
	switch {
	case u.st == nil:
		return "can't open it"
	case u.syncing:
		return "syncing…"
	case u.said != "":
		return u.said
	}
	if n := u.st.Pending(); n > 0 && !u.synced.IsZero() {
		return fmt.Sprintf("%d to send", n)
	}
	if !u.synced.IsZero() {
		return "synced " + u.synced.Format("15:04")
	}
	if _, err := os.Stat(kobo.AppFile(calConf)); err != nil {
		return "on this Kobo"
	}
	return ""
}

// take does what the frame asked for, after it.
func (u *calUI) take() {
	if u.wantSync {
		u.wantSync = false
		u.syncNow()
	}
	if u.vk != 0 {
		u.a.vkAsk(u.vk > 0)
		u.vk = 0
	}
}

// syncNow syncs with the server in calendar.conf, if there is one.
func (u *calUI) syncNow() {
	if u.st == nil {
		return
	}
	if u.syncing {
		u.again = true
		return
	}
	c, err := cal.LoadConf(kobo.AppFile(calConf))
	if err != nil {
		u.next = time.Time{}
		if !errors.Is(err, os.ErrNotExist) {
			u.said = calConf + ": " + err.Error()
		}
		return
	}
	u.syncing, u.said = true, ""
	cl := &cal.Client{URL: c.URL, User: c.User, Password: c.Password, UserAgent: "hellobro/" + version}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		res, err := u.st.Sync(ctx, cl, time.Now)
		u.a.Do(func() {
			u.syncing = false
			if err != nil {
				log.Printf("calendar: sync: %v", err)
				u.said = "not synced: " + calErr(err)
			} else {
				u.synced = time.Now()
				log.Printf("calendar: synced: %s", res)
				if res != (cal.Synced{}) {
					u.toast.Text = res.String() // shown at the next frame
				}
			}
			u.next = time.Now().Add(c.Every)
			if u.again {
				u.again = false
				u.next = time.Now()
			}
			u.a.inkRedraw(calKey)
		})
	}()
}

// calErr is err, short: the strip has room for a few words.
func calErr(err error) string {
	s := err.Error()
	var se *cal.StatusError
	switch {
	case errors.As(err, &se) && se.Code == 401:
		return "the server says the user or password is wrong"
	case strings.Contains(s, "no such host"), strings.Contains(s, "network is unreachable"):
		return "offline?"
	case errors.Is(err, context.DeadlineExceeded):
		return "the server took too long"
	}
	if len(s) > 120 {
		s = s[:120] + "…"
	}
	return s
}

func dayOf(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func mondayOf(t time.Time) time.Time {
	t = dayOf(t)
	return t.AddDate(0, 0, -(int(t.Weekday())+6)%7)
}

// step moves what's shown by a day, a week or a month, back or on.
func (u *calUI) step(n int) {
	switch u.view.Value {
	case "day":
		u.at = u.at.AddDate(0, 0, n)
	case "week", "agenda":
		u.at = u.at.AddDate(0, 0, 7*n)
	case "month":
		u.at = time.Date(u.at.Year(), u.at.Month()+time.Month(n), 1, 0, 0, 0, 0, u.at.Location())
	}
	u.scrolled = false
}

// clk is the button of an occurrence (or a task: at zero), the same from
// frame to frame.
func (u *calUI) clk(o *cal.Entry, at time.Time, kind string) *widget.Clickable {
	k := kind + "|" + o.Item.UID + "|" + fmt.Sprint(at.Unix())
	u.used[k] = true
	c := u.btn[k]
	if c == nil {
		c = &widget.Clickable{}
		u.btn[k] = c
	}
	return c
}

func (u *calUI) Layout(gtx ink.Ctx) ink.Dims {
	th := u.a.theme()
	size := gtx.Constraints.Max
	ink.Fill(gtx.Ops, image.Rectangle{Max: size}, th.Bg)
	if u.st == nil {
		return layout.UniformInset(th.Pad*2).Layout(gtx, theme.Body(th, "the calendar's folder can't be opened: "+u.openErr).Layout)
	}
	if u.at.IsZero() { // just opened
		u.at = dayOf(gtx.Now)
		u.next = gtx.Now
	}
	if !u.next.IsZero() && !u.syncing {
		if !gtx.Now.Before(u.next) {
			u.wantSync, u.next = true, time.Time{}
		} else {
			gtx.InvalidateAt(u.next)
		}
	}
	if u.toast.Text != "" && !u.toast.Showing(gtx) {
		u.toast.Show(gtx, u.toast.Text)
	}
	u.events(gtx)
	for k := range u.used {
		delete(u.used, k)
	}

	title := u.at.Format("January 2006")
	switch u.view.Value {
	case "day":
		title = u.at.Format("Monday 2 January")
	case "tasks":
		title = "Tasks"
	}
	sp := layout.Rigid(layout.Spacer{Width: th.Pad}.Layout)
	syncL := "sync"
	if u.syncing {
		syncL = "syncing…"
	}
	dims := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx ink.Ctx) ink.Dims {
			return layout.UniformInset(th.Pad).Layout(gtx, func(gtx ink.Ctx) ink.Dims {
				kids := []layout.FlexChild{}
				if u.view.Value != "tasks" {
					kids = append(kids,
						layout.Rigid(theme.Button(th, &u.todayB, "today").Layout), sp,
						layout.Rigid(theme.Button(th, &u.prevB, "‹").Layout), sp,
						layout.Rigid(theme.Button(th, &u.nextB, "›").Layout))
				}
				kids = append(kids,
					layout.Flexed(1, func(gtx ink.Ctx) ink.Dims {
						return layout.Inset{Left: th.Pad * 2}.Layout(gtx, theme.Label(th, th.TextSize*1.3, title).Layout)
					}),
					layout.Rigid(theme.Button(th, &u.syncB, syncL).Layout), sp,
					layout.Rigid(func(gtx ink.Ctx) ink.Dims {
						b := theme.Button(th, &u.addB, "+ new")
						b.Strong = true
						return b.Layout(gtx)
					}))
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx, kids...)
			})
		}),
		layout.Rigid(func(gtx ink.Ctx) ink.Dims {
			return layout.Inset{Left: th.Pad, Right: th.Pad}.Layout(gtx, theme.Tabs(th, &u.view, calViews, calViewNames).Layout)
		}),
		layout.Flexed(1, func(gtx ink.Ctx) ink.Dims {
			return layout.UniformInset(th.Pad).Layout(gtx, func(gtx ink.Ctx) ink.Dims {
				switch u.view.Value {
				case "day":
					return u.days(gtx, th, u.at, 1)
				case "month":
					return u.month(gtx, th)
				case "agenda":
					return u.agendaList(gtx, th)
				case "tasks":
					return u.taskList(gtx, th)
				}
				return u.days(gtx, th, mondayOf(u.at), 7)
			})
		}))
	for k := range u.btn {
		if !u.used[k] {
			delete(u.btn, k)
		}
	}
	if u.ed != nil {
		u.ed.layout(gtx, th, u)
	}
	if u.toast.Showing(gtx) {
		theme.Toast(th, &u.toast).Layout(gtx)
	} else {
		u.toast.Text = ""
	}
	typing := u.quick.Focused() || u.ed != nil && (u.ed.title.Focused() || u.ed.place.Focused())
	if typing != u.typing {
		u.typing = typing
		u.vk = map[bool]int{true: 1, false: -1}[typing]
	}
	return dims
}

// events takes the header's taps and the keys nobody else took.
func (u *calUI) events(gtx ink.Ctx) {
	if u.todayB.Clicked(gtx) {
		u.at, u.scrolled = dayOf(gtx.Now), false
	}
	if u.prevB.Clicked(gtx) {
		u.step(-1)
	}
	if u.nextB.Clicked(gtx) {
		u.step(1)
	}
	if u.syncB.Clicked(gtx) {
		u.wantSync = true
	}
	if u.addB.Clicked(gtx) {
		u.startNew(gtx)
	}
	if u.view.Update(gtx) {
		u.scrolled = false
	}
	for i := range u.dayBtn {
		if u.dayBtn[i].Clicked(gtx) {
			u.at, u.scrolled, u.view.Value = u.monthDay(i), false, "day"
		}
	}
	for i := range u.colBtn {
		if u.colBtn[i].Clicked(gtx) {
			u.at, u.scrolled, u.view.Value = mondayOf(u.at).AddDate(0, 0, i), false, "day"
		}
	}
	for _, e := range gtx.Events(ink.Root) {
		switch e := e.(type) {
		case ink.Key:
			switch e.Name {
			case "Esc":
				u.ed = nil
			case "PgUp":
				u.step(-1)
			case "PgDn":
				u.step(1)
			}
		case ink.Text:
			if u.ed != nil {
				break
			}
			switch e.Text {
			case "t":
				u.at, u.scrolled = dayOf(gtx.Now), false
			case "n":
				u.startNew(gtx)
			case "s":
				u.wantSync = true
			case ",", "<":
				u.step(-1)
			case ".", ">":
				u.step(1)
			case "1", "2", "3", "4", "5":
				u.view.Value, u.scrolled = calViews[e.Text[0]-'1'], false
			}
		}
	}
}

// saved is an item saved or deleted: it goes to the server soon.
func (u *calUI) saved(err error) {
	if err != nil {
		u.said = "not saved: " + err.Error()
		return
	}
	if _, err := os.Stat(kobo.AppFile(calConf)); err == nil && !u.syncing {
		u.next = time.Now().Add(calAfter)
	} else if u.syncing {
		u.again = true
	}
}

// occurrences in a day's row: those at a time of day, and those all day
// (tasks due on the day, too) that are on it.
func onDay(os []cal.Occurrence, d time.Time) (timed, allDay []cal.Occurrence) {
	next := d.AddDate(0, 0, 1)
	for _, o := range os {
		if o.Item.AllDay {
			end := o.End()
			if !end.After(o.At) {
				end = o.At.AddDate(0, 0, 1)
			}
			if o.At.Before(next) && end.After(d) {
				allDay = append(allDay, o)
			}
		} else if !o.At.Before(d) && o.At.Before(next) {
			timed = append(timed, o)
		}
	}
	return
}

func gutter(gtx ink.Ctx) int { return gtx.Dp(58) }

// days is n days from first in columns of their hours, their names and
// what's all day over them.
func (u *calUI) days(gtx ink.Ctx, th *theme.Theme, first time.Time, n int) ink.Dims {
	size := gtx.Constraints.Max
	occ := u.st.Between(first, first.AddDate(0, 0, n))
	gw := gutter(gtx)
	colW := (size.X - gw) / n
	small := th.Face(gtx, th.TextSize*0.8, text.Regular)
	tiny := th.Face(gtx, th.TextSize*0.7, text.Regular)
	bold := th.Face(gtx, th.TextSize*1.2, text.Bold)
	today := dayOf(gtx.Now)
	nameH := gtx.Dp(th.Touch * 1.3)
	lineH := text.Height(tiny) + gtx.Dp(6)
	rows := 0
	for i := 0; i < n; i++ {
		_, ad := onDay(occ, first.AddDate(0, 0, i))
		rows = max(rows, min(len(ad), 3))
	}
	headH := nameH + rows*lineH
	if rows > 0 {
		headH += gtx.Dp(4)
	}
	for i := 0; i < n; i++ {
		d := first.AddDate(0, 0, i)
		x := gw + i*colW
		draw := func(gtx ink.Ctx) ink.Dims {
			r := image.Rect(0, 0, colW, nameH)
			ink.Fill(gtx.Ops, r, th.Bg)
			name := d.Format("Mon")
			ink.TextAt(gtx.Ops, small, (colW-text.Width(small, name))/2, text.Ascent(small)+gtx.Dp(2), name, th.Quiet)
			num := fmt.Sprint(d.Day())
			nw, nh := text.Width(bold, num), text.Height(bold)
			ny := text.Height(small) + gtx.Dp(4)
			ink.TextAt(gtx.Ops, bold, (colW-nw)/2, ny+gtx.Dp(1)+text.Ascent(bold), num, th.Fg)
			if d.Equal(today) {
				ink.Invert(gtx.Ops, image.Rect((colW-nw)/2-gtx.Dp(6), ny, (colW+nw)/2+gtx.Dp(6), ny+nh+gtx.Dp(2)))
			}
			if n > 1 && u.colBtn[i].Pressed() {
				ink.Invert(gtx.Ops, r)
			}
			return ink.Dims{Size: r.Size()}
		}
		st := ink.Offset(gtx.Ops, image.Pt(x, 0))
		cg := gtx
		cg.Constraints = ink.Exact(image.Pt(colW, nameH))
		if n > 1 {
			u.colBtn[i].Layout(cg, draw)
		} else {
			draw(cg)
		}
		st.Pop()
		_, ad := onDay(occ, d)
		for k, o := range ad {
			if k == rows {
				break
			}
			r := image.Rect(x+gtx.Dp(2), nameH+k*lineH, x+colW-gtx.Dp(2), nameH+(k+1)*lineH-gtx.Dp(2))
			s := o.Item.Summary
			if k == rows-1 && len(ad) > rows {
				s = fmt.Sprintf("%d more", len(ad)-k)
			}
			u.chip(gtx, th, o, d, r, s)
		}
	}
	ink.Fill(gtx.Ops, image.Rect(0, headH, size.X, headH+max(gtx.Dp(th.Border), 1)), th.Fg)
	hourH := gtx.Dp(52)
	if !u.scrolled { // the morning shows first, or now if it's later
		u.scrolled = true
		h := 7
		if !today.Before(first) && today.Before(first.AddDate(0, 0, n)) {
			h = max(min(gtx.Now.Hour()-2, 15), 7)
		}
		u.hours.RevealSpan(h*hourH, 1<<20)
	}
	st := ink.Offset(gtx.Ops, image.Pt(0, headH+gtx.Dp(2)))
	cg := gtx
	cg.Constraints = ink.Exact(image.Pt(size.X, max(size.Y-headH-gtx.Dp(2), 0)))
	theme.List(th, &u.hours).Layout(cg, 1, func(gtx ink.Ctx, _ int) ink.Dims {
		return u.hourGrid(gtx, th, occ, first, n, colW, hourH)
	})
	st.Pop()
	return ink.Dims{Size: size}
}

// chip is an all-day item, a line in r: framed, a task's box before it.
func (u *calUI) chip(gtx ink.Ctx, th *theme.Theme, o cal.Occurrence, d time.Time, r image.Rectangle, s string) {
	tiny := th.Face(gtx, th.TextSize*0.7, text.Regular)
	clk := u.clk(o.Entry, o.At, "chip"+d.Format("0102"))
	if clk.Clicked(gtx) {
		u.edit(o.Entry)
	}
	st := ink.Offset(gtx.Ops, r.Min)
	cg := gtx
	cg.Constraints = ink.Exact(r.Size())
	clk.Layout(cg, func(gtx ink.Ctx) ink.Dims {
		rr := image.Rectangle{Max: r.Size()}
		ink.Fill(gtx.Ops, rr, ink.Light)
		x := gtx.Dp(4)
		if o.Item.Kind == cal.Task {
			x = taskBox(gtx, th, x, rr.Dy(), o.Item.Done)
		}
		ink.TextAt(gtx.Ops, tiny, x, (rr.Dy()+text.Ascent(tiny)-tiny.Metrics().Descent.Ceil())/2, text.Fit(tiny, s, rr.Dx()-x-gtx.Dp(2)), th.Fg)
		if clk.Pressed() {
			ink.Invert(gtx.Ops, rr)
		}
		return ink.Dims{Size: rr.Size()}
	})
	st.Pop()
}

// taskBox draws a task's box at x, in the middle of h: where what's
// after it goes.
func taskBox(gtx ink.Ctx, th *theme.Theme, x, h int, done bool) int {
	s := min(gtx.Dp(12), h-4)
	r := image.Rect(x, (h-s)/2, x+s, (h-s)/2+s)
	ink.Frame(gtx.Ops, r, max(gtx.Dp(1.5), 2), th.Fg)
	if done {
		ink.Fill(gtx.Ops, r.Inset(max(gtx.Dp(3), 3)), th.Fg)
	}
	return r.Max.X + gtx.Dp(5)
}

// hourGrid is the days' 24 hours, n columns of them, what's on them and
// a line at now.
func (u *calUI) hourGrid(gtx ink.Ctx, th *theme.Theme, occ []cal.Occurrence, first time.Time, n, colW, hourH int) ink.Dims {
	gw := gutter(gtx)
	w := gtx.Constraints.Max.X - gtx.Dp(th.Pad)
	size := image.Pt(gtx.Constraints.Max.X, 24*hourH)
	small := th.Face(gtx, th.TextSize*0.75, text.Regular)
	for h := 0; h < 24; h++ {
		y := h * hourH
		ink.Fill(gtx.Ops, image.Rect(gw-gtx.Dp(4), y, w, y+1), ink.Light)
		if h > 0 {
			s := fmt.Sprintf("%02d:00", h)
			ink.TextAt(gtx.Ops, small, gw-gtx.Dp(8)-text.Width(small, s), y+text.Ascent(small)/2, s, th.Quiet)
		}
	}
	for i := 1; i < n; i++ {
		x := gw + i*colW
		ink.Fill(gtx.Ops, image.Rect(x, 0, x+1, size.Y), ink.Light)
	}
	pad := gtx.Dp(2)
	for i := 0; i < n; i++ {
		d := first.AddDate(0, 0, i)
		is, _ := onDay(occ, d)
		for k, o := range is {
			// Those it overlaps share the column.
			cols, col := 1, 0
			end := func(o cal.Occurrence) time.Time { return maxTime(o.End(), o.At.Add(30*time.Minute)) }
			for k2, o2 := range is {
				if k2 != k && o2.At.Before(end(o)) && o.At.Before(end(o2)) {
					cols++
					if k2 < k {
						col++
					}
				}
			}
			cw := colW / cols
			y0 := int(o.At.Sub(d).Minutes()) * hourH / 60
			y1 := min(int(o.End().Sub(d).Minutes())*hourH/60, size.Y)
			r := image.Rect(gw+i*colW+col*cw+pad, y0+pad, gw+i*colW+(col+1)*cw-pad, max(y1-pad, y0+gtx.Dp(24)))
			clk := u.clk(o.Entry, o.At, "block")
			if clk.Clicked(gtx) {
				u.edit(o.Entry)
			}
			st := ink.Offset(gtx.Ops, r.Min)
			cg := gtx
			cg.Constraints = ink.Exact(r.Size())
			clk.Layout(cg, func(gtx ink.Ctx) ink.Dims { return itemBlock(gtx, th, o, r.Size(), clk.Pressed()) })
			st.Pop()
		}
		if now := gtx.Now; dayOf(now).Equal(d) {
			y := int(now.Sub(d).Minutes()) * hourH / 60
			x := gw + i*colW
			t := max(gtx.Dp(2), 2)
			ink.Fill(gtx.Ops, image.Rect(x, y-t/2, x+colW, y+t-t/2), th.Fg)
			ink.Disc(gtx.Ops, image.Pt(x, y), gtx.Dp(5), th.Fg)
		}
	}
	return ink.Dims{Size: size}
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

// itemBlock is an item in its hours: framed, a bar down its left, its
// title and time, as much as fits.
func itemBlock(gtx ink.Ctx, th *theme.Theme, o cal.Occurrence, size image.Point, pressed bool) ink.Dims {
	r := image.Rectangle{Max: size}
	b := max(gtx.Dp(th.Border), 1)
	ink.Fill(gtx.Ops, r, th.Bg)
	ink.Frame(gtx.Ops, r, b, th.Fg)
	ink.Fill(gtx.Ops, image.Rect(0, 0, gtx.Dp(5), size.Y), th.Fg)
	bold := th.Face(gtx, th.TextSize*0.8, text.Bold)
	small := th.Face(gtx, th.TextSize*0.75, text.Regular)
	x := gtx.Dp(9)
	clip := ink.Clip(gtx.Ops, r.Inset(b))
	y := gtx.Dp(3) + text.Ascent(bold)
	if o.Item.Kind == cal.Task {
		x = taskBox(gtx, th, x, 2*y-text.Ascent(bold), o.Item.Done)
	}
	ink.TextAt(gtx.Ops, bold, x, y, text.Fit(bold, o.Item.Summary, size.X-x-gtx.Dp(3)), th.Fg)
	y += text.Height(small)
	if y < size.Y {
		s := o.At.Format("15:04")
		if o.Item.Kind == cal.Event {
			s += "–" + o.End().Format("15:04")
		}
		if o.Item.Location != "" {
			s += ", " + o.Item.Location
		}
		ink.TextAt(gtx.Ops, small, gtx.Dp(9), y, text.Fit(small, s, size.X-gtx.Dp(12)), th.Quiet)
	}
	clip.Pop()
	if pressed {
		ink.Invert(gtx.Ops, r)
	}
	return ink.Dims{Size: size}
}

// monthDay is the month view's cell i: six weeks from the Monday on or
// before the first.
func (u *calUI) monthDay(i int) time.Time {
	first := time.Date(u.at.Year(), u.at.Month(), 1, 0, 0, 0, 0, u.at.Location())
	return mondayOf(first).AddDate(0, 0, i)
}

// month is six weeks of days, each with what fits of what's on it.
func (u *calUI) month(gtx ink.Ctx, th *theme.Theme) ink.Dims {
	size := gtx.Constraints.Max
	occ := u.st.Between(u.monthDay(0), u.monthDay(42))
	small := th.Face(gtx, th.TextSize*0.8, text.Regular)
	tiny := th.Face(gtx, th.TextSize*0.7, text.Regular)
	bold := th.Face(gtx, th.TextSize*0.9, text.Bold)
	headH := text.Height(small) + gtx.Dp(8)
	cw, ch := size.X/7, (size.Y-headH)/6
	for i := 0; i < 7; i++ {
		s := mondayOf(gtx.Now).AddDate(0, 0, i).Format("Mon")
		ink.TextAt(gtx.Ops, small, i*cw+(cw-text.Width(small, s))/2, text.Ascent(small)+gtx.Dp(2), s, th.Quiet)
	}
	today := dayOf(gtx.Now)
	b := max(gtx.Dp(th.Border), 1)
	for i := range u.dayBtn {
		d := u.monthDay(i)
		timed, ad := onDay(occ, d)
		all := append(ad, timed...)
		at := image.Pt(i%7*cw, headH+i/7*ch)
		st := ink.Offset(gtx.Ops, at)
		cg := gtx
		cg.Constraints = ink.Exact(image.Pt(cw, ch))
		clk := &u.dayBtn[i]
		clk.Layout(cg, func(gtx ink.Ctx) ink.Dims {
			r := image.Rect(0, 0, cw+b, ch+b) // the frames shared
			ink.Fill(gtx.Ops, r, th.Bg)
			ink.Frame(gtx.Ops, r, b, ink.Mid)
			num := fmt.Sprint(d.Day())
			g := th.Fg
			if d.Month() != u.at.Month() {
				g = th.Quiet
			}
			y := gtx.Dp(4) + text.Ascent(bold)
			nx := gtx.Dp(6)
			ink.TextAt(gtx.Ops, bold, nx, y, num, g)
			if d.Equal(today) {
				ink.Invert(gtx.Ops, image.Rect(nx-gtx.Dp(3), gtx.Dp(3), nx+text.Width(bold, num)+gtx.Dp(3), y+gtx.Dp(4)))
			}
			y += gtx.Dp(4)
			lh := text.Height(tiny) + gtx.Dp(2)
			for k, o := range all {
				if y+2*lh > ch && k < len(all)-1 {
					ink.TextAt(gtx.Ops, tiny, nx, y+text.Ascent(tiny), fmt.Sprintf("%d more", len(all)-k), th.Quiet)
					break
				}
				if y+lh > ch {
					break
				}
				s := o.Item.Summary
				x := nx
				switch {
				case o.Item.Kind == cal.Task:
					x = taskBoxAt(gtx, th, nx, y, lh, o.Item.Done)
				case !o.Item.AllDay:
					s = o.At.Format("15:04") + " " + s
					ink.Fill(gtx.Ops, image.Rect(nx, y+gtx.Dp(3), nx+gtx.Dp(3), y+lh-gtx.Dp(2)), th.Fg)
					x = nx + gtx.Dp(6)
				default:
					ink.Fill(gtx.Ops, image.Rect(nx-gtx.Dp(2), y+gtx.Dp(1), cw-gtx.Dp(4), y+lh), ink.Light)
					x = nx + gtx.Dp(2)
				}
				ink.TextAt(gtx.Ops, tiny, x, y+text.Ascent(tiny), text.Fit(tiny, s, cw-x-gtx.Dp(4)), th.Fg)
				y += lh
			}
			if clk.Pressed() {
				ink.Invert(gtx.Ops, image.Rect(0, 0, cw, ch))
			}
			return ink.Dims{Size: image.Pt(cw, ch)}
		})
		st.Pop()
	}
	return ink.Dims{Size: size}
}

// taskBoxAt is taskBox on a line at y, lh high.
func taskBoxAt(gtx ink.Ctx, th *theme.Theme, x, y, lh int, done bool) int {
	st := ink.Offset(gtx.Ops, image.Pt(0, y))
	x = taskBox(gtx, th, x, lh, done)
	st.Pop()
	return x
}

// agendaList is the coming four weeks, by day.
func (u *calUI) agendaList(gtx ink.Ctx, th *theme.Theme) ink.Dims {
	type row struct {
		day time.Time
		o   *cal.Occurrence // nil: the day's heading
	}
	var rows []row
	from := dayOf(u.at)
	occ := u.st.Between(from, from.AddDate(0, 0, 28))
	for i := 0; i < 28; i++ {
		d := from.AddDate(0, 0, i)
		timed, ad := onDay(occ, d)
		all := append(ad, timed...)
		if len(all) == 0 {
			continue
		}
		rows = append(rows, row{d, nil})
		for k := range all {
			rows = append(rows, row{d, &all[k]})
		}
	}
	if len(rows) == 0 {
		return theme.Small(th, "nothing in the next four weeks").Layout(gtx)
	}
	today := dayOf(gtx.Now)
	return theme.List(th, &u.agenda).Layout(gtx, len(rows), func(gtx ink.Ctx, i int) ink.Dims {
		r := rows[i]
		if r.o == nil {
			s := r.day.Format("Monday 2 January")
			if r.day.Equal(today) {
				s = "Today, " + s
			}
			return theme.Section(th, s)(gtx)
		}
		o := *r.o
		clk := u.clk(o.Entry, o.At, "agenda"+r.day.Format("0102"))
		if clk.Clicked(gtx) {
			u.edit(o.Entry)
		}
		rs := theme.Row(th, clk, o.Item.Summary)
		rs.Sub = o.Item.Location
		switch {
		case o.Item.Kind == cal.Task:
			rs.Value = "a task, due"
			if !o.Item.AllDay {
				rs.Value += " " + o.At.Format("15:04")
			}
		case o.Item.AllDay:
			rs.Value = "all day"
		default:
			rs.Value = o.At.Format("15:04") + "–" + o.End().Format("15:04")
		}
		if o.Item.RRule != "" {
			rs.Value += " ↻"
		}
		return rs.Layout(gtx)
	})
}

// taskList is the tasks: a line to add one, those to do, those done.
func (u *calUI) taskList(gtx ink.Ctx, th *theme.Theme) ink.Dims {
	if u.quick.Submitted(gtx) {
		if s := strings.TrimSpace(u.quick.Text()); s != "" {
			it := cal.New(cal.Task)
			it.Summary = s
			_, err := u.st.Save(u.st.Home(cal.Task), it, time.Now())
			u.saved(err)
			u.quick.SetText("")
			gtx.Focus(&u.quick)
		}
	}
	u.showDone.Update(gtx)
	all := u.st.Tasks()
	var ts []*cal.Entry
	done := 0
	for _, e := range all {
		if e.Item.Done {
			done++
			if !u.showDone.Value {
				continue
			}
		}
		ts = append(ts, e)
	}
	today := dayOf(gtx.Now)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(theme.Editor(th, &u.quick, "a new task: type it, Enter").Layout),
		layout.Rigid(layout.Spacer{Height: th.Pad}.Layout),
		layout.Rigid(theme.CheckBox(th, &u.showDone, fmt.Sprintf("the done ones too (%d)", done)).Layout),
		layout.Flexed(1, func(gtx ink.Ctx) ink.Dims {
			if len(ts) == 0 {
				return layout.Inset{Top: th.Pad}.Layout(gtx, theme.Small(th, "nothing to do").Layout)
			}
			return theme.List(th, &u.tasks).Layout(gtx, len(ts), func(gtx ink.Ctx, i int) ink.Dims {
				return u.taskRow(gtx, th, ts[i], today)
			})
		}))
}

// taskRow is a task: its box to tick, its title, when it's due.
func (u *calUI) taskRow(gtx ink.Ctx, th *theme.Theme, e *cal.Entry, today time.Time) ink.Dims {
	it := e.Item
	tick := u.clk(e, time.Time{}, "tick")
	row := u.clk(e, time.Time{}, "task")
	if tick.Clicked(gtx) {
		n := *it
		n.Done = !n.Done
		_, err := u.st.Save(e.In.ID, &n, time.Now())
		u.saved(err)
	}
	if row.Clicked(gtx) {
		u.edit(e)
	}
	w := gtx.Constraints.Max.X
	f := th.Face(gtx, th.TextSize, text.Regular)
	small := th.Face(gtx, th.TextSize*0.85, text.Regular)
	h := max(gtx.Dp(th.Touch), text.Height(f)+text.Height(small)+gtx.Dp(th.Pad*1.5))
	boxW := gtx.Dp(th.Touch)
	cg := gtx
	cg.Constraints = ink.Exact(image.Pt(boxW, h))
	tick.Layout(cg, func(gtx ink.Ctx) ink.Dims {
		s := gtx.Dp(26)
		r := image.Rect((boxW-s)/2, (h-s)/2, (boxW+s)/2, (h+s)/2)
		ink.Fill(gtx.Ops, image.Rect(0, 0, boxW, h), th.Bg)
		ink.Frame(gtx.Ops, r, max(gtx.Dp(2), 2), th.Fg)
		if it.Done {
			ink.Fill(gtx.Ops, r.Inset(max(gtx.Dp(5), 4)), th.Fg)
		}
		if tick.Pressed() {
			ink.Invert(gtx.Ops, r)
		}
		return ink.Dims{Size: image.Pt(boxW, h)}
	})
	st := ink.Offset(gtx.Ops, image.Pt(boxW, 0))
	cg.Constraints = ink.Exact(image.Pt(w-boxW, h))
	row.Layout(cg, func(gtx ink.Ctx) ink.Dims {
		rw := w - boxW
		ink.Fill(gtx.Ops, image.Rect(0, 0, rw, h), th.Bg)
		pad := gtx.Dp(th.Pad)
		g := uint8(th.Fg)
		if it.Done {
			g = th.Quiet
		}
		var sub []string
		due := ""
		if !it.End.IsZero() {
			due = "due " + calDay(it.End, today)
			if !it.AllDay {
				due += " " + it.End.Format("15:04")
			}
			if !it.Done && it.End.Before(today) {
				due = "late: " + due
			}
		}
		if it.Done && !it.DoneAt.IsZero() {
			due = "done " + calDay(it.DoneAt, today)
		}
		if due != "" {
			sub = append(sub, due)
		}
		if p := prioName(it.Priority); p != "" {
			sub = append(sub, p)
		}
		if it.Description != "" {
			sub = append(sub, strings.ReplaceAll(it.Description, "\n", " "))
		}
		top := (h - text.Height(f) - text.Height(small)) / 2
		if len(sub) == 0 {
			top = (h - text.Height(f)) / 2
		}
		ink.TextAt(gtx.Ops, f, 0, top+text.Ascent(f), text.Fit(f, it.Summary, rw-pad), g)
		if it.Done {
			y := top + text.Ascent(f)*2/3
			ink.Fill(gtx.Ops, image.Rect(0, y, min(text.Width(f, it.Summary), rw-pad), y+max(gtx.Dp(1.5), 2)), th.Quiet)
		}
		if len(sub) > 0 {
			ink.TextAt(gtx.Ops, small, 0, top+text.Height(f)+text.Ascent(small), text.Fit(small, strings.Join(sub, " · "), rw-pad), th.Quiet)
		}
		ink.Fill(gtx.Ops, image.Rect(0, h-1, rw-pad/2, h), th.Quiet)
		if row.Pressed() {
			ink.Invert(gtx.Ops, image.Rect(0, 0, rw, h))
		}
		return ink.Dims{Size: image.Pt(rw, h)}
	})
	st.Pop()
	return ink.Dims{Size: image.Pt(w, h)}
}

// calDay is a day said shortly: today, tomorrow, a weekday this week,
// else its date.
func calDay(t, today time.Time) string {
	d := dayOf(t)
	switch n := int(d.Sub(today).Hours() / 24); {
	case n == 0:
		return "today"
	case n == 1:
		return "tomorrow"
	case n == -1:
		return "yesterday"
	case n > 1 && n < 7:
		return d.Format("Monday")
	case d.Year() == today.Year():
		return d.Format("2 Jan")
	}
	return d.Format("2 Jan 2006")
}

var (
	prios     = []int{0, 1, 5, 9}
	prioNames = []string{"no priority", "high", "medium", "low"}
)

func prioName(p int) string {
	switch {
	case p == 0:
		return ""
	case p < 5:
		return "high"
	case p == 5:
		return "medium"
	}
	return "low"
}

func prioIndex(p int) int {
	switch {
	case p == 0:
		return 0
	case p < 5:
		return 1
	case p == 5:
		return 2
	}
	return 3
}
