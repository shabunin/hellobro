package widget

import (
	"image"
	"time"

	"hellobro/ink"
)

// Date is a day picked from a month's calendar that opens over the rest:
// a tap on a day picks it; ‹ › the month before and after. Open, the
// arrows go a day (← →) or a week (↑ ↓), PageUp and PageDown a month,
// Home to today, Enter picks.
type Date struct {
	Popover
	Value      time.Time // the day picked; zero: none yet
	Days       [42]Clickable
	Prev, Next Clickable

	month   time.Time // the first of the month shown
	hi      time.Time // the day the keys are on
	changed bool
}

func day(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func firstOf(t time.Time) time.Time {
	y, m, _ := t.Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, t.Location())
}

// Month is the first of the month shown.
func (d *Date) Month() time.Time { return d.month }

// Highlit is the day the keys are on.
func (d *Date) Highlit() time.Time { return d.hi }

// Day is the grid's cell i: six weeks, Monday first, from the week the
// month starts in.
func (d *Date) Day(i int) time.Time {
	back := (int(d.month.Weekday()) + 6) % 7 // days since Monday
	return d.month.AddDate(0, 0, i-back)
}

func (d *Date) show(t time.Time) {
	d.hi = day(t)
	d.month = firstOf(t)
}

// Update takes the events since the last frame; Now is today, if no day's
// picked yet.
func (d *Date) Update(gtx ink.Ctx) {
	if d.toggled(gtx) {
		at := d.Value
		if at.IsZero() {
			at = gtx.Now
		}
		d.show(at)
		d.Open(gtx)
	}
	for _, k := range d.keys(gtx) {
		switch k.Name {
		case "Left":
			d.show(d.hi.AddDate(0, 0, -1))
		case "Right":
			d.show(d.hi.AddDate(0, 0, 1))
		case "Up":
			d.show(d.hi.AddDate(0, 0, -7))
		case "Down":
			d.show(d.hi.AddDate(0, 0, 7))
		case "PageUp":
			d.show(d.hi.AddDate(0, -1, 0))
		case "PageDown":
			d.show(d.hi.AddDate(0, 1, 0))
		case "Home":
			d.show(gtx.Now)
		case "Enter", "Space":
			d.pick(gtx, d.hi)
		}
	}
	for d.Prev.Clicked(gtx) {
		d.month = d.month.AddDate(0, -1, 0)
	}
	for d.Next.Clicked(gtx) {
		d.month = d.month.AddDate(0, 1, 0)
	}
	for i := range d.Days {
		d.Days[i].NoFocus = true
		for d.Days[i].Clicked(gtx) {
			d.pick(gtx, d.Day(i))
		}
	}
	d.Prev.NoFocus, d.Next.NoFocus = true, true
}

func (d *Date) pick(gtx ink.Ctx, t time.Time) {
	t = day(t)
	if !t.Equal(d.Value) {
		d.Value, d.changed = t, true
	}
	d.Close(gtx)
}

// Changed says another day was picked since it was last asked.
func (d *Date) Changed(gtx ink.Ctx) bool {
	d.Update(gtx)
	c := d.changed
	d.changed = false
	return c
}

// Clock is a time of day picked from a grid of hours and one of minutes
// (Step apart): a tap on an hour, then on the minutes. Open, ← → go
// between the two, ↑ ↓ change the one the keys are on, Enter goes on to
// the minutes, then closes it.
type Clock struct {
	Popover
	Hour, Minute int
	Step         int // the minutes apart; 0: 5
	Hours        [24]Clickable
	Minutes      []Clickable

	col     int // 0 the hours, 1 the minutes: where the keys are
	changed bool
}

// StepOf is the minutes apart its choices are.
func (c *Clock) StepOf() int {
	if c.Step <= 0 || c.Step > 60 {
		return 5
	}
	return c.Step
}

// Column is where the keys are: 0 the hours, 1 the minutes.
func (c *Clock) Column() int { return c.col }

func (c *Clock) set(h, m int) {
	h, m = (h%24+24)%24, (m%60+60)%60
	if h != c.Hour || m != c.Minute {
		c.Hour, c.Minute, c.changed = h, m, true
	}
}

// Update takes the events since the last frame.
func (c *Clock) Update(gtx ink.Ctx) {
	st := c.StepOf()
	for len(c.Minutes) < 60/st {
		c.Minutes = append(c.Minutes, Clickable{NoFocus: true})
	}
	if c.toggled(gtx) {
		c.col = 0
		c.Open(gtx)
	}
	for _, k := range c.keys(gtx) {
		switch k.Name {
		case "Left":
			c.col = 0
		case "Right", "Tab":
			c.col = 1 - c.col
		case "Up", "Down":
			d := 1
			if k.Name == "Up" {
				d = -1
			}
			if c.col == 0 {
				c.set(c.Hour+d, c.Minute)
			} else {
				c.set(c.Hour, (c.Minute/st+d)*st)
			}
		case "Enter", "Space":
			if c.col == 0 {
				c.col = 1
			} else {
				c.Close(gtx)
			}
		}
	}
	for h := range c.Hours {
		c.Hours[h].NoFocus = true
		for c.Hours[h].Clicked(gtx) {
			c.set(h, c.Minute)
			c.col = 1
		}
	}
	for i := range c.Minutes[:60/st] {
		for c.Minutes[i].Clicked(gtx) {
			c.set(c.Hour, i*st)
			c.Close(gtx)
		}
	}
}

// Changed says the time changed since it was last asked.
func (c *Clock) Changed(gtx ink.Ctx) bool {
	c.Update(gtx)
	ch := c.changed
	c.changed = false
	return ch
}

// Range is a span along a line, Low to High within Min to Max: a finger
// moves the end it comes down nearer to; in focus ← → step the end last
// moved, Space goes to the other.
type Range struct {
	Low, High, Min, Max, Step float32

	Clickable
	active   int // 0 Low, 1 High
	dragging bool
	width    int
	changed  bool
}

// Active is the end a finger or the keys move: 0 Low, 1 High.
func (r *Range) Active() int { return r.active }

// Dragging says a finger's on it.
func (r *Range) Dragging() bool { return r.dragging }

// Frac is v as a part of the way along, 0 to 1.
func (r *Range) Frac(v float32) float32 {
	if r.Max <= r.Min {
		return 0
	}
	return (v - r.Min) / (r.Max - r.Min)
}

// Changed says it changed since the last frame.
func (r *Range) Changed(gtx ink.Ctx) bool {
	r.update(gtx)
	c := r.changed
	r.changed = false
	return c
}

func (r *Range) round(v float32) float32 {
	if r.Step > 0 {
		v = r.Min + float32(int((v-r.Min)/r.Step+0.5))*r.Step
	}
	return min(max(v, r.Min), r.Max)
}

func (r *Range) set(v float32) {
	v = r.round(v)
	lo, hi := r.Low, r.High
	if r.active == 0 {
		lo = min(v, r.High)
	} else {
		hi = max(v, r.Low)
	}
	if lo != r.Low || hi != r.High {
		r.Low, r.High, r.changed = lo, hi, true
	}
}

func (r *Range) at(x int) float32 {
	return r.Min + (r.Max-r.Min)*float32(x)/float32(max(r.width, 1))
}

func (r *Range) update(gtx ink.Ctx) {
	r.Keys = rangeKeys
	for _, e := range gtx.Events(&r.Clickable) {
		switch e := e.(type) {
		case ink.Pointer:
			switch e.Kind {
			case ink.Press:
				v := r.at(e.Pos.X)
				dl, dh := abs32(v-r.Low), abs32(v-r.High)
				if dl < dh || dl == dh && v < r.Low {
					r.active = 0
				} else {
					r.active = 1
				}
				r.dragging = true
				fallthrough
			case ink.Move:
				if r.dragging && r.width > 0 {
					r.set(r.at(e.Pos.X))
				}
			case ink.Release, ink.Cancel:
				r.dragging = false
			}
		case ink.Key:
			step := r.Step
			if step <= 0 {
				step = (r.Max - r.Min) / 10
			}
			v := r.Low
			if r.active == 1 {
				v = r.High
			}
			switch e.Name {
			case "Left":
				r.set(v - step)
			case "Right":
				r.set(v + step)
			case "Space":
				r.active = 1 - r.active
			}
		case ink.Focus:
			r.focused = e.On
		}
	}
}

var rangeKeys = []string{"Left", "Right", "Space"} // ↑ ↓ the focus's, as floatKeys

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// Layout lays w (the range as drawn) out as its area.
func (r *Range) Layout(gtx ink.Ctx, w ink.Widget) ink.Dims {
	r.update(gtx)
	if r.dragging {
		gtx.Quick()
	}
	m := ink.Record(gtx.Ops)
	d := w(gtx)
	call := m.Stop()
	r.width = d.Size.X
	call.Add(gtx.Ops)
	if !gtx.IsDisabled() {
		ink.Input(gtx.Ops, ink.Area{Rect: image.Rectangle{Max: d.Size}, Tag: &r.Clickable, Focus: true, Keys: r.Keys})
	}
	r.focused = gtx.Focused(&r.Clickable)
	return d
}
