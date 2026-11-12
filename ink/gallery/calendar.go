package gallery

import (
	"fmt"
	"image"
	"slices"
	"time"

	"hellobro/ink"
	"hellobro/ink/layout"
	"hellobro/ink/text"
	"hellobro/ink/theme"
	"hellobro/ink/widget"
)

// Calendar is a demo made of ink's parts, after a web calendar: a day, a
// week of hours, a month, an agenda; events to open, new ones to add.
type Calendar struct {
	view                   widget.Enum // day, week, month, agenda
	today, prev, next, add widget.Clickable
	at                     time.Time // a day in what's shown
	hours, agenda          layout.Scroll
	scrolled               bool // the hours were put at the morning
	events                 []event
	evBtn                  []widget.Clickable
	dayBtn                 [42]widget.Clickable
	colBtn                 [7]widget.Clickable

	open          int // the event shown in a dialog; -1: none
	close, del    widget.Clickable
	adding        bool
	title         widget.Editor
	day           widget.Date
	start         widget.Clock
	long          widget.Select
	save, dismiss widget.Clickable
}

type event struct {
	Title, Place string
	Start        time.Time
	Len          time.Duration
}

func (e event) End() time.Time { return e.Start.Add(e.Len) }

var (
	calViews     = []string{"day", "week", "month", "agenda"}
	calViewNames = []string{"Day", "Week", "Month", "Agenda"}
	lengths      = []string{"30 min", "1 hour", "1½ hours", "2 hours", "3 hours"}
	lengthMins   = []int{30, 60, 90, 120, 180}
)

// A week's events, Monday first: hour, minute, minutes long, title,
// place. Made up.
var weekly = [7][]struct {
	h, m, mins   int
	title, place string
}{
	{{9, 30, 30, "Stand-up", "the call"}, {14, 0, 60, "Read: chapter 4", ""}},
	{{9, 30, 30, "Stand-up", "the call"}, {11, 0, 90, "Dentist", "14 Elm street"}, {18, 30, 120, "Climbing", "the wall"}},
	{{9, 30, 30, "Stand-up", "the call"}, {15, 0, 60, "One to one with Mira", "room 3"}},
	{{9, 30, 30, "Stand-up", "the call"}, {10, 0, 120, "ink: the widgets", "home"}, {13, 0, 60, "Lunch", "the market"}, {19, 0, 90, "Book club", "the bookshop"}},
	{{9, 30, 30, "Stand-up", "the call"}, {16, 0, 60, "The week, looked back on", ""}},
	{{10, 0, 180, "Farmers' market", "the market"}, {20, 0, 150, "Cinema", "the Odeon"}},
	{{11, 0, 120, "A long walk", "river park"}},
}

func dayOf(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// monday is the Monday of t's week.
func monday(t time.Time) time.Time {
	t = dayOf(t)
	return t.AddDate(0, 0, -(int(t.Weekday())+6)%7)
}

// seed makes the events of five weeks around now.
func (c *Calendar) seed(now time.Time) {
	c.view.Value, c.at, c.open = "week", dayOf(now), -1
	m := monday(now)
	for w := -2; w <= 2; w++ {
		for d, evs := range weekly {
			day := m.AddDate(0, 0, w*7+d)
			for i, e := range evs {
				if w != 0 && (w+d+i)%4 == 3 { // not every week the same
					continue
				}
				c.events = append(c.events, event{e.title, e.place,
					day.Add(time.Duration(e.h)*time.Hour + time.Duration(e.m)*time.Minute), time.Duration(e.mins) * time.Minute})
			}
		}
	}
	c.sort()
	c.hours.Axis, c.agenda.Axis = layout.Vertical, layout.Vertical
	c.start.Step = 15
}

func (c *Calendar) sort() {
	slices.SortStableFunc(c.events, func(a, b event) int { return a.Start.Compare(b.Start) })
	for len(c.evBtn) < len(c.events) {
		c.evBtn = append(c.evBtn, widget.Clickable{})
	}
}

// on is the events of the day d.
func (c *Calendar) on(d time.Time) []int {
	var is []int
	for i, e := range c.events {
		if dayOf(e.Start).Equal(d) {
			is = append(is, i)
		}
	}
	return is
}

// Esc closes what's open: true if something was.
func (c *Calendar) Esc() bool {
	switch {
	case c.adding:
		c.adding = false
	case c.open >= 0:
		c.open = -1
	default:
		return false
	}
	return true
}

func (c *Calendar) scroller() layout.Scroller {
	switch c.view.Value {
	case "day", "week":
		return &c.hours
	case "agenda":
		return &c.agenda
	}
	return nil
}

// step moves what's shown by a day, a week or a month, back or on.
func (c *Calendar) step(n int) {
	switch c.view.Value {
	case "day":
		c.at = c.at.AddDate(0, 0, n)
	case "week", "agenda":
		c.at = c.at.AddDate(0, 0, 7*n)
	case "month":
		c.at = time.Date(c.at.Year(), c.at.Month()+time.Month(n), 1, 0, 0, 0, 0, c.at.Location())
	}
}

// Layout lays it out in what it's given; the dialogs are Over's.
func (c *Calendar) Layout(gtx ink.Ctx, g *Gallery) ink.Dims {
	th := g.Th
	if c.events == nil {
		c.seed(gtx.Now)
	}
	if c.today.Clicked(gtx) {
		c.at, c.scrolled = dayOf(gtx.Now), false
		g.Said = "the calendar: today"
	}
	if c.prev.Clicked(gtx) {
		c.step(-1)
	}
	if c.next.Clicked(gtx) {
		c.step(1)
	}
	if c.add.Clicked(gtx) {
		c.startAdding(gtx)
	}
	if c.view.Update(gtx) {
		c.scrolled = false
		g.Said = "the calendar: by the " + c.view.Value
	}
	for i := range c.events {
		if c.evBtn[i].Clicked(gtx) {
			c.open = i
			g.Said = "the calendar: " + c.events[i].Title
		}
	}
	for i := range c.dayBtn {
		if c.dayBtn[i].Clicked(gtx) {
			c.at, c.scrolled = c.monthDay(i), false
			c.view.Value = "day"
		}
	}
	for i := range c.colBtn {
		if c.colBtn[i].Clicked(gtx) {
			c.at, c.scrolled = monday(c.at).AddDate(0, 0, i), false
			c.view.Value = "day"
		}
	}
	title := c.at.Format("January 2006")
	if c.view.Value == "day" {
		title = c.at.Format("Monday 2 January")
	}
	sp := layout.Rigid(layout.Spacer{Width: th.Pad}.Layout)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx ink.Ctx) ink.Dims {
			return layout.UniformInset(th.Pad).Layout(gtx, func(gtx ink.Ctx) ink.Dims {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(theme.Button(th, &c.today, "today").Layout), sp,
					layout.Rigid(theme.Button(th, &c.prev, "‹").Layout), sp,
					layout.Rigid(theme.Button(th, &c.next, "›").Layout),
					layout.Flexed(1, func(gtx ink.Ctx) ink.Dims {
						return layout.Inset{Left: th.Pad * 2}.Layout(gtx, theme.Label(th, th.TextSize*1.3, title).Layout)
					}),
					layout.Rigid(func(gtx ink.Ctx) ink.Dims {
						b := theme.Button(th, &c.add, "+ new")
						b.Strong = true
						return b.Layout(gtx)
					}))
			})
		}),
		layout.Rigid(func(gtx ink.Ctx) ink.Dims {
			return layout.Inset{Left: th.Pad, Right: th.Pad}.Layout(gtx, theme.Tabs(th, &c.view, calViews, calViewNames).Layout)
		}),
		layout.Flexed(1, func(gtx ink.Ctx) ink.Dims {
			return layout.UniformInset(th.Pad).Layout(gtx, func(gtx ink.Ctx) ink.Dims {
				switch c.view.Value {
				case "day":
					return c.days(gtx, th, c.at, 1)
				case "month":
					return c.month(gtx, th)
				case "agenda":
					return c.agendaList(gtx, th)
				}
				return c.days(gtx, th, monday(c.at), 7)
			})
		}))
}

// gutter is the hours' column's width.
func gutter(gtx ink.Ctx) int { return gtx.Dp(58) }

// days is n days from first, in columns of their hours: a head of the
// days' names, the hours under it moving under the finger.
func (c *Calendar) days(gtx ink.Ctx, th *theme.Theme, first time.Time, n int) ink.Dims {
	size := gtx.Constraints.Max
	gw := gutter(gtx)
	colW := (size.X - gw) / n
	small := th.Face(gtx, th.TextSize*0.8, text.Regular)
	bold := th.Face(gtx, th.TextSize*1.2, text.Bold)
	today := dayOf(gtx.Now)
	headH := gtx.Dp(th.Touch * 1.3)
	for i := 0; i < n; i++ {
		d := first.AddDate(0, 0, i)
		x := gw + i*colW
		draw := func(gtx ink.Ctx) ink.Dims {
			r := image.Rect(0, 0, colW, headH)
			ink.Fill(gtx.Ops, r, th.Bg)
			name := d.Format("Mon")
			ink.TextAt(gtx.Ops, small, (colW-text.Width(small, name))/2, text.Ascent(small)+gtx.Dp(2), name, th.Quiet)
			num := fmt.Sprint(d.Day())
			nw, nh := text.Width(bold, num), text.Height(bold)
			ny := text.Height(small) + gtx.Dp(4)
			box := image.Rect((colW-nw)/2-gtx.Dp(6), ny, (colW+nw)/2+gtx.Dp(6), ny+nh+gtx.Dp(2))
			ink.TextAt(gtx.Ops, bold, (colW-nw)/2, ny+gtx.Dp(1)+text.Ascent(bold), num, th.Fg)
			if d.Equal(today) {
				ink.Invert(gtx.Ops, box)
			}
			if n > 1 && c.colBtn[i].Pressed() {
				ink.Invert(gtx.Ops, r)
			}
			return ink.Dims{Size: r.Size()}
		}
		st := ink.Offset(gtx.Ops, image.Pt(x, 0))
		cg := gtx
		cg.Constraints = ink.Exact(image.Pt(colW, headH))
		if n > 1 {
			c.colBtn[i].Layout(cg, draw)
		} else {
			draw(cg)
		}
		st.Pop()
	}
	ink.Fill(gtx.Ops, image.Rect(0, headH, size.X, headH+max(gtx.Dp(th.Border), 1)), th.Fg)
	hourH := gtx.Dp(52)
	if !c.scrolled { // the morning shows first, or now if it's later
		c.scrolled = true
		h := 7
		if first.Equal(today) || n == 7 && today.Sub(first) >= 0 && today.Sub(first) < 7*24*time.Hour {
			h = max(min(gtx.Now.Hour()-2, 15), 7)
		}
		c.hours.RevealSpan(h*hourH, 1<<20)
	}
	st := ink.Offset(gtx.Ops, image.Pt(0, headH+gtx.Dp(2)))
	cg := gtx
	cg.Constraints = ink.Exact(image.Pt(size.X, size.Y-headH-gtx.Dp(2)))
	theme.List(th, &c.hours).Layout(cg, 1, func(gtx ink.Ctx, _ int) ink.Dims {
		return c.hourGrid(gtx, th, first, n, colW, hourH)
	})
	st.Pop()
	return ink.Dims{Size: size}
}

// hourGrid is the day's 24 hours, n columns of them, the events on
// them and a line at now.
func (c *Calendar) hourGrid(gtx ink.Ctx, th *theme.Theme, first time.Time, n, colW, hourH int) ink.Dims {
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
		is := c.on(d)
		for k, e := range is {
			ev := c.events[e]
			// Those it overlaps share the column.
			cols, col := 1, 0
			for k2, e2 := range is {
				o := c.events[e2]
				if o.Start.Before(ev.End()) && ev.Start.Before(o.End()) && k2 != k {
					cols++
					if k2 < k {
						col++
					}
				}
			}
			cw := colW / cols
			y0 := int(ev.Start.Sub(d).Minutes()) * hourH / 60
			y1 := int(ev.End().Sub(d).Minutes()) * hourH / 60
			r := image.Rect(gw+i*colW+col*cw+pad, y0+pad, gw+i*colW+(col+1)*cw-pad, max(y1-pad, y0+gtx.Dp(22)))
			st := ink.Offset(gtx.Ops, r.Min)
			cg := gtx
			cg.Constraints = ink.Exact(r.Size())
			clk := &c.evBtn[e]
			clk.Layout(cg, func(gtx ink.Ctx) ink.Dims { return eventBlock(gtx, th, ev, r.Size(), clk.Pressed()) })
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

// eventBlock is an event in its hours: framed, a bar down its left, its
// title and time, as much as fits.
func eventBlock(gtx ink.Ctx, th *theme.Theme, e event, size image.Point, pressed bool) ink.Dims {
	r := image.Rectangle{Max: size}
	b := max(gtx.Dp(th.Border), 1)
	ink.Fill(gtx.Ops, r, th.Bg)
	ink.Frame(gtx.Ops, r, b, th.Fg)
	ink.Fill(gtx.Ops, image.Rect(0, 0, gtx.Dp(5), size.Y), th.Fg)
	bold := th.Face(gtx, th.TextSize*0.8, text.Bold)
	small := th.Face(gtx, th.TextSize*0.75, text.Regular)
	x, room := gtx.Dp(9), size.X-gtx.Dp(12)
	clip := ink.Clip(gtx.Ops, r.Inset(b))
	y := gtx.Dp(3) + text.Ascent(bold)
	ink.TextAt(gtx.Ops, bold, x, y, text.Fit(bold, e.Title, room), th.Fg)
	y += text.Height(small)
	if y < size.Y {
		s := e.Start.Format("15:04") + "–" + e.End().Format("15:04")
		if e.Place != "" {
			s += ", " + e.Place
		}
		ink.TextAt(gtx.Ops, small, x, y, text.Fit(small, s, room), th.Quiet)
	}
	clip.Pop()
	if pressed {
		ink.Invert(gtx.Ops, r)
	}
	return ink.Dims{Size: size}
}

// monthDay is the day of the month view's cell i: six weeks from the
// Monday on or before the first.
func (c *Calendar) monthDay(i int) time.Time {
	first := time.Date(c.at.Year(), c.at.Month(), 1, 0, 0, 0, 0, c.at.Location())
	return monday(first).AddDate(0, 0, i)
}

// month is six weeks of days, each with what fits of its events.
func (c *Calendar) month(gtx ink.Ctx, th *theme.Theme) ink.Dims {
	size := gtx.Constraints.Max
	small := th.Face(gtx, th.TextSize*0.8, text.Regular)
	tiny := th.Face(gtx, th.TextSize*0.7, text.Regular)
	bold := th.Face(gtx, th.TextSize*0.9, text.Bold)
	headH := text.Height(small) + gtx.Dp(8)
	cw, ch := size.X/7, (size.Y-headH)/6
	for i := 0; i < 7; i++ {
		s := monday(gtx.Now).AddDate(0, 0, i).Format("Mon")
		ink.TextAt(gtx.Ops, small, i*cw+(cw-text.Width(small, s))/2, text.Ascent(small)+gtx.Dp(2), s, th.Quiet)
	}
	today := dayOf(gtx.Now)
	b := max(gtx.Dp(th.Border), 1)
	for i := range c.dayBtn {
		d := c.monthDay(i)
		at := image.Pt(i%7*cw, headH+i/7*ch)
		st := ink.Offset(gtx.Ops, at)
		cg := gtx
		cg.Constraints = ink.Exact(image.Pt(cw, ch))
		clk := &c.dayBtn[i]
		clk.Layout(cg, func(gtx ink.Ctx) ink.Dims {
			r := image.Rect(0, 0, cw+b, ch+b) // the frames shared
			ink.Fill(gtx.Ops, r, th.Bg)
			ink.Frame(gtx.Ops, r, b, ink.Mid)
			num := fmt.Sprint(d.Day())
			g := th.Fg
			if d.Month() != c.at.Month() {
				g = th.Quiet
			}
			y := gtx.Dp(4) + text.Ascent(bold)
			nx := gtx.Dp(6)
			ink.TextAt(gtx.Ops, bold, nx, y, num, g)
			if d.Equal(today) {
				ink.Invert(gtx.Ops, image.Rect(nx-gtx.Dp(3), gtx.Dp(3), nx+text.Width(bold, num)+gtx.Dp(3), y+gtx.Dp(4)))
			}
			y += gtx.Dp(4)
			is := c.on(d)
			lh := text.Height(tiny) + gtx.Dp(2)
			for k, e := range is {
				if y+2*lh > ch && k < len(is)-1 {
					ink.TextAt(gtx.Ops, tiny, nx, y+text.Ascent(tiny), fmt.Sprintf("%d more", len(is)-k), th.Quiet)
					break
				}
				if y+lh > ch {
					break
				}
				ev := c.events[e]
				s := ev.Start.Format("15:04") + " " + ev.Title
				ink.Fill(gtx.Ops, image.Rect(nx, y+gtx.Dp(3), nx+gtx.Dp(3), y+lh-gtx.Dp(2)), th.Fg)
				ink.TextAt(gtx.Ops, tiny, nx+gtx.Dp(6), y+text.Ascent(tiny), text.Fit(tiny, s, cw-nx-gtx.Dp(10)), th.Fg)
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

// agendaList is the coming two weeks' events, by day.
func (c *Calendar) agendaList(gtx ink.Ctx, th *theme.Theme) ink.Dims {
	type item struct {
		day time.Time
		ev  int // -1: the day's heading
	}
	var items []item
	from := dayOf(c.at)
	for i := 0; i < 14; i++ {
		d := from.AddDate(0, 0, i)
		is := c.on(d)
		if len(is) == 0 {
			continue
		}
		items = append(items, item{d, -1})
		for _, e := range is {
			items = append(items, item{d, e})
		}
	}
	if len(items) == 0 {
		return theme.Small(th, "nothing in the next two weeks").Layout(gtx)
	}
	today := dayOf(gtx.Now)
	return theme.List(th, &c.agenda).Layout(gtx, len(items), func(gtx ink.Ctx, i int) ink.Dims {
		it := items[i]
		if it.ev < 0 {
			s := it.day.Format("Monday 2 January")
			if it.day.Equal(today) {
				s = "Today, " + s
			}
			return theme.Section(th, s)(gtx)
		}
		e := c.events[it.ev]
		r := theme.Row(th, &c.evBtn[it.ev], e.Title)
		r.Sub = e.Place
		r.Value = e.Start.Format("15:04") + "–" + e.End().Format("15:04")
		return r.Layout(gtx)
	})
}

func (c *Calendar) startAdding(gtx ink.Ctx) {
	c.adding = true
	c.title.SetText("")
	c.day.Value = c.at
	h := gtx.Now.Hour() + 1
	if !c.at.Equal(dayOf(gtx.Now)) {
		h = 10
	}
	c.start.Hour, c.start.Minute = min(h, 23), 0
	c.long.Value = 1
	gtx.Focus(&c.title)
}

// Over lays the dialogs out over all the gallery, if one's open.
func (c *Calendar) Over(gtx ink.Ctx, g *Gallery) {
	th := g.Th
	if c.close.Clicked(gtx) {
		c.open = -1
	}
	if c.del.Clicked(gtx) && c.open >= 0 {
		g.Said = "the calendar: deleted " + c.events[c.open].Title
		g.toast.Show(gtx, "deleted")
		c.events = slices.Delete(c.events, c.open, c.open+1)
		c.open = -1
	}
	if c.dismiss.Clicked(gtx) {
		c.adding = false
	}
	c.long.Update(gtx, len(lengths))
	if (c.save.Clicked(gtx) || c.title.Submitted(gtx)) && c.adding {
		t := c.title.Text()
		if t == "" {
			t = "(no title)"
		}
		d := dayOf(c.day.Value)
		if c.day.Value.IsZero() {
			d = c.at
		}
		e := event{Title: t, Start: d.Add(time.Duration(c.start.Hour)*time.Hour + time.Duration(c.start.Minute)*time.Minute),
			Len: time.Duration(lengthMins[c.long.Value]) * time.Minute}
		c.events = append(c.events, e)
		c.sort()
		c.adding, c.at, c.scrolled = false, d, false
		g.Said = "the calendar: added " + t + ", " + e.Start.Format("Mon 2 Jan 15:04")
		g.toast.Show(gtx, "added")
	}
	switch {
	case c.adding:
		save := theme.Button(th, &c.save, "save")
		save.Strong = true
		dl := theme.Dialog(th, "A new event", "", theme.Button(th, &c.dismiss, "cancel"), save)
		dl.Body = func(gtx ink.Ctx) ink.Dims {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(theme.Editor(th, &c.title, "what is it?").Layout),
				layout.Rigid(layout.Spacer{Height: th.Pad}.Layout),
				layout.Rigid(theme.Date(th, &c.day, "On").Layout),
				layout.Rigid(theme.Clock(th, &c.start, "At").Layout),
				layout.Rigid(theme.Select(th, &c.long, "For", lengths...).Layout))
		}
		dl.Layout(gtx)
	case c.open >= 0 && c.open < len(c.events):
		e := c.events[c.open]
		s := e.Start.Format("Monday 2 January, 15:04") + "–" + e.End().Format("15:04")
		if e.Place != "" {
			s += "\n" + e.Place
		}
		theme.Dialog(th, e.Title, s, theme.Button(th, &c.del, "delete"), theme.Button(th, &c.close, "close")).Layout(gtx)
	}
}
