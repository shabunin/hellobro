package cal

import (
	"crypto/rand"
	"fmt"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // the Kobo has no zoneinfo for TZID's places
)

// Item is an event or a task, as the calendar shows it; Cal is all of
// its object, kept as it came (its alarms, its time zones, what we don't
// show), and Apply puts the fields back into it.
type Item struct {
	Kind        string // "VEVENT" or "VTODO"
	UID         string
	Summary     string
	Location    string
	Description string
	Start, End  time.Time // a task's Start may be zero; End is a task's Due
	AllDay      bool
	Done        bool      // a task's
	DoneAt      time.Time // when it was done
	Priority    int       // 1 (most) to 9; 0: none
	RRule       string    // how it repeats, as RRULE has it; "" not
	ExDates     []time.Time
	Cal         *Comp
}

// Event and Task are the kinds.
const (
	Event = "VEVENT"
	Task  = "VTODO"
)

// ItemOf reads the item in cal: its first event or task (another of the
// same UID with a RECURRENCE-ID is one time of it moved: not shown).
func ItemOf(cal *Comp) (*Item, error) {
	var c *Comp
	for _, s := range cal.Comps {
		if (s.Name == Event || s.Name == Task) && s.Get("RECURRENCE-ID") == nil {
			c = s
			break
		}
	}
	if c == nil {
		return nil, fmt.Errorf("no event or task in it")
	}
	it := &Item{Kind: c.Name, UID: c.Text("UID"), Summary: c.Text("SUMMARY"), Location: c.Text("LOCATION"),
		Description: c.Text("DESCRIPTION"), RRule: c.Text("RRULE"), Cal: cal}
	if p := c.Get("DTSTART"); p != nil {
		it.Start, it.AllDay = parseTime(p)
	}
	end := "DTEND"
	if c.Name == Task {
		end = "DUE"
	}
	if p := c.Get(end); p != nil {
		var allDay bool
		it.End, allDay = parseTime(p)
		if it.Start.IsZero() { // a task due on a day, not at a time
			it.AllDay = allDay
		}
	} else if d, ok := parseDuration(c.Text("DURATION")); ok && !it.Start.IsZero() {
		it.End = it.Start.Add(d)
	} else if c.Name == Event && !it.Start.IsZero() {
		it.End = it.Start // an instant; a day long if it's a date
		if it.AllDay {
			it.End = it.Start.AddDate(0, 0, 1)
		}
	}
	if c.Name == Task {
		it.Done = strings.EqualFold(c.Text("STATUS"), "COMPLETED") || c.Get("COMPLETED") != nil
		if p := c.Get("COMPLETED"); p != nil {
			it.DoneAt, _ = parseTime(p)
		}
	}
	it.Priority, _ = strconv.Atoi(c.Text("PRIORITY"))
	for _, p := range c.All("EXDATE") {
		for _, v := range strings.Split(p.Value, ",") {
			q := *p
			q.Value = v
			if t, _ := parseTime(&q); !t.IsZero() {
				it.ExDates = append(it.ExDates, t)
			}
		}
	}
	return it, nil
}

// comp is the item's own component in Cal.
func (it *Item) comp() *Comp {
	for _, s := range it.Cal.Comps {
		if s.Name == it.Kind && s.Get("RECURRENCE-ID") == nil {
			return s
		}
	}
	return nil
}

// New makes an item of kind, its calendar object made for it.
func New(kind string) *Item {
	it := &Item{Kind: kind, UID: NewUID()}
	it.Cal = &Comp{Name: "VCALENDAR", Props: []*Prop{{Name: "VERSION", Value: "2.0"}, {Name: "PRODID", Value: "-//hellobro//calendar//EN"}},
		Comps: []*Comp{{Name: kind}}}
	return it
}

// NewUID is a UID no one else has.
func NewUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x@hellobro", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// Apply writes the fields into the item's component, now: what it
// doesn't show stays as it was.
func (it *Item) Apply(now time.Time) {
	c := it.comp()
	c.SetText("UID", it.UID)
	c.Set("DTSTAMP", utc(now))
	c.Set("LAST-MODIFIED", utc(now))
	if c.Get("CREATED") == nil {
		c.Set("CREATED", utc(now))
	}
	c.SetText("SUMMARY", it.Summary)
	c.SetText("LOCATION", it.Location)
	c.SetText("DESCRIPTION", it.Description)
	setTime := func(name string, t time.Time) {
		switch {
		case t.IsZero():
			c.Del(name)
		case it.AllDay:
			c.Set(name, t.Format("20060102"), [2]string{"VALUE", "DATE"})
		default:
			c.Set(name, utc(t))
		}
	}
	setTime("DTSTART", it.Start)
	c.Del("DURATION")
	if it.Kind == Task {
		setTime("DUE", it.End)
		if it.Done {
			c.Set("STATUS", "COMPLETED")
			if it.DoneAt.IsZero() {
				it.DoneAt = now
			}
			c.Set("COMPLETED", utc(it.DoneAt))
			c.Set("PERCENT-COMPLETE", "100")
		} else {
			c.Set("STATUS", "NEEDS-ACTION")
			c.Del("COMPLETED")
			c.Del("PERCENT-COMPLETE")
			it.DoneAt = time.Time{}
		}
	} else {
		setTime("DTEND", it.End)
	}
	if it.Priority > 0 {
		c.Set("PRIORITY", strconv.Itoa(it.Priority))
	} else {
		c.Del("PRIORITY")
	}
	c.Set("RRULE", it.RRule)
	seq, _ := strconv.Atoi(c.Text("SEQUENCE"))
	c.Set("SEQUENCE", strconv.Itoa(seq+1))
}

func utc(t time.Time) string { return t.UTC().Format("20060102T150405Z") }

// parseTime reads a DATE-TIME (UTC, in a TZID, or floating: here) or a
// DATE (all day, from local midnight).
func parseTime(p *Prop) (time.Time, bool) {
	v := strings.TrimSpace(p.Value)
	if p.Param("VALUE") == "DATE" || len(v) == 8 {
		t, err := time.ParseInLocation("20060102", v, time.Local)
		if err != nil {
			return time.Time{}, false
		}
		return t, true
	}
	if strings.HasSuffix(v, "Z") {
		t, _ := time.Parse("20060102T150405Z", v)
		return t.Local(), false
	}
	loc := time.Local
	if tz := p.Param("TZID"); tz != "" {
		if l, err := time.LoadLocation(strings.TrimPrefix(tz, "/")); err == nil {
			loc = l
		} else if l, ok := windowsZones[tz]; ok {
			loc = l
		}
	}
	t, err := time.ParseInLocation("20060102T150405", v, loc)
	if err != nil {
		return time.Time{}, false
	}
	return t.Local(), false
}

// windowsZones are the names Outlook gives a few zones, where they're
// not IANA's.
var windowsZones = func() map[string]*time.Location {
	m := map[string]*time.Location{}
	for win, iana := range map[string]string{
		"W. Europe Standard Time": "Europe/Berlin", "Central Europe Standard Time": "Europe/Budapest",
		"Romance Standard Time": "Europe/Paris", "GMT Standard Time": "Europe/London", "Russian Standard Time": "Europe/Moscow",
		"Eastern Standard Time": "America/New_York", "Central Standard Time": "America/Chicago",
		"Mountain Standard Time": "America/Denver", "Pacific Standard Time": "America/Los_Angeles",
		"E. Europe Standard Time": "Europe/Chisinau", "FLE Standard Time": "Europe/Kiev", "UTC": "UTC",
	} {
		if l, err := time.LoadLocation(iana); err == nil {
			m[win] = l
		}
	}
	return m
}()

// parseDuration reads P1DT2H30M, P2W, -PT15M.
func parseDuration(s string) (time.Duration, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	sign := time.Duration(1)
	switch s[0] {
	case '-':
		sign, s = -1, s[1:]
	case '+':
		s = s[1:]
	}
	if !strings.HasPrefix(s, "P") {
		return 0, false
	}
	s = s[1:]
	var d time.Duration
	inTime := false
	num := 0
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			num = num*10 + int(r-'0')
		case r == 'T':
			inTime = true
		default:
			unit := map[rune]time.Duration{'W': 7 * 24 * time.Hour, 'D': 24 * time.Hour, 'H': time.Hour, 'S': time.Second}[r]
			if r == 'M' {
				unit = time.Minute
				if !inTime {
					return 0, false // months: not a duration iCalendar has
				}
			}
			if unit == 0 {
				return 0, false
			}
			d += time.Duration(num) * unit
			num = 0
		}
	}
	return sign * d, true
}

// Occurs is when the item happens from from to to: its start, and each
// time it repeats (RRULE: DAILY, WEEKLY, MONTHLY, YEARLY, with
// INTERVAL, COUNT, UNTIL, BYDAY and BYMONTHDAY), less its EXDATEs. A
// time counts if the item's span overlaps the range.
func (it *Item) Occurs(from, to time.Time) []time.Time {
	start := it.Start
	if start.IsZero() {
		start = it.End // a task's: its due
	}
	if start.IsZero() {
		return nil
	}
	span := max(it.End.Sub(it.Start), 0)
	if it.Start.IsZero() {
		span = 0
	}
	in := func(t time.Time) bool {
		return t.Before(to) && (t.Add(span).After(from) || t.Equal(from))
	}
	if it.RRule == "" {
		if in(start) {
			return []time.Time{start}
		}
		return nil
	}
	r := parseRule(it.RRule, start)
	var out []time.Time
	n := 0
	for i, t := range r.times(start, to) {
		if i > 100000 {
			break
		}
		if r.count > 0 && n >= r.count || !r.until.IsZero() && t.After(r.until) {
			break
		}
		n++
		if in(t) && !it.excluded(t) {
			out = append(out, t)
		}
	}
	return out
}

func (it *Item) excluded(t time.Time) bool {
	for _, x := range it.ExDates {
		if x.Equal(t) || it.AllDay && sameDay(x, t) {
			return true
		}
	}
	return false
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

type rule struct {
	freq     string
	interval int
	count    int
	until    time.Time
	byDay    []time.Weekday
	nth      []int // byDay's: the nth of the month (-1 the last); 0 any
	byMonth  []int
}

func parseRule(s string, start time.Time) rule {
	r := rule{interval: 1}
	for _, part := range strings.Split(s, ";") {
		k, v, _ := strings.Cut(part, "=")
		switch strings.ToUpper(k) {
		case "FREQ":
			r.freq = strings.ToUpper(v)
		case "INTERVAL":
			r.interval = max(1, atoi(v))
		case "COUNT":
			r.count = atoi(v)
		case "UNTIL":
			r.until, _ = parseTime(&Prop{Value: v})
			if len(v) == 8 { // a date: the whole day
				r.until = r.until.AddDate(0, 0, 1).Add(-time.Second)
			}
		case "BYDAY":
			for _, d := range strings.Split(v, ",") {
				num := strings.TrimRight(d, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz") // "2MO", "-1FR"
				d = d[len(num):]
				if i := strings.Index("SUMOTUWETHFRSA", strings.ToUpper(d)); i >= 0 && i%2 == 0 && len(d) == 2 {
					r.byDay = append(r.byDay, time.Weekday(i/2))
					r.nth = append(r.nth, atoi(strings.TrimPrefix(num, "+")))
				}
			}
		case "BYMONTHDAY":
			for _, d := range strings.Split(v, ",") {
				r.byMonth = append(r.byMonth, atoi(d))
			}
		}
	}
	return r
}

func atoi(s string) int { n, _ := strconv.Atoi(s); return n }

// times are the rule's times from start on, up to to.
func (r rule) times(start, to time.Time) func(func(int, time.Time) bool) {
	return func(yield func(int, time.Time) bool) {
		i := 0
		emit := func(t time.Time) bool {
			if t.Before(start) {
				return true
			}
			ok := yield(i, t)
			i++
			return ok
		}
		h, m, s := start.Clock()
		at := func(y int, mo time.Month, d int) time.Time {
			return time.Date(y, mo, d, h, m, s, 0, start.Location())
		}
		for step := 0; ; step++ {
			var period []time.Time
			switch r.freq {
			case "DAILY":
				period = []time.Time{start.AddDate(0, 0, step*r.interval)}
			case "WEEKLY":
				wk := start.AddDate(0, 0, step*7*r.interval)
				if len(r.byDay) == 0 {
					period = []time.Time{wk}
					break
				}
				mon := wk.AddDate(0, 0, -((int(wk.Weekday()) + 6) % 7))
				for d := 0; d < 7; d++ {
					day := mon.AddDate(0, 0, d)
					for _, wd := range r.byDay {
						if day.Weekday() == wd {
							period = append(period, at(day.Year(), day.Month(), day.Day()))
						}
					}
				}
			case "MONTHLY":
				y, mo, _ := start.Date()
				first := time.Date(y, mo+time.Month(step*r.interval), 1, 0, 0, 0, 0, start.Location())
				days := r.byMonth
				if len(days) == 0 && len(r.byDay) == 0 {
					days = []int{start.Day()}
				}
				last := first.AddDate(0, 1, -1).Day()
				for d := 1; d <= last; d++ {
					day := at(first.Year(), first.Month(), d)
					ok := false
					for _, md := range days {
						if md == d || md < 0 && last+md+1 == d {
							ok = true
						}
					}
					for k, wd := range r.byDay {
						n := r.nth[k]
						ok = ok || day.Weekday() == wd && (n == 0 || n > 0 && (d-1)/7+1 == n || n < 0 && (last-d)/7+1 == -n)
					}
					if ok {
						period = append(period, day)
					}
				}
			case "YEARLY":
				y, mo, d := start.Date()
				t := at(y+step*r.interval, mo, d)
				if t.Day() == d { // not the 29th of February in a year without one
					period = []time.Time{t}
				}
			default:
				emit(start)
				return
			}
			for _, t := range period {
				if !t.Before(to) {
					return
				}
				if !emit(t) {
					return
				}
			}
			if step > 50000 {
				return
			}
		}
	}
}
