package mermaid

// Gantt charts: tasks as bars along a time axis, a row each, their
// names on the left under their sections'. Done tasks gray, active ones
// framed thick, critical ones dark, milestones diamonds.

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

type ganttTask struct {
	name, id, section string
	start, end        time.Time
	done, active      bool
	crit, milestone   bool
}

type gantt struct {
	title string
	tasks []*ganttTask
	axis  string // axisFormat, strftime's
}

var durRE = regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*(ms|s|m|h|d|w|M|y)$`)

// ganttLayout is a dateFormat (Day.js's tokens) as Go's layout.
func ganttLayout(f string) string {
	return strings.NewReplacer("YYYY", "2006", "YY", "06", "MM", "01", "DD", "02", "HH", "15", "mm", "04", "ss", "05",
		"M", "1", "D", "2", "H", "15").Replace(f)
}

func parseDur(s string) (time.Duration, bool) {
	m := durRE.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0, false
	}
	v, _ := strconv.ParseFloat(m[1], 64)
	unit := map[string]time.Duration{"ms": time.Millisecond, "s": time.Second, "m": time.Minute, "h": time.Hour,
		"d": 24 * time.Hour, "w": 7 * 24 * time.Hour, "M": 30 * 24 * time.Hour, "y": 365 * 24 * time.Hour}[m[2]]
	return time.Duration(v * float64(unit)), true
}

func parseGantt(lines []string) *gantt {
	g := &gantt{}
	layout := "2006-01-02"
	section := ""
	ids := map[string]*ganttTask{}
	var last *ganttTask
	when := func(s string) (time.Time, bool) {
		s = strings.TrimSpace(s)
		if strings.HasPrefix(s, "after ") {
			var t time.Time
			for _, id := range strings.Fields(s[6:]) {
				if o, ok := ids[id]; ok && o.end.After(t) {
					t = o.end
				}
			}
			return t, !t.IsZero()
		}
		for _, l := range []string{layout, "2006-01-02", "2006-01-02 15:04", "15:04"} {
			if t, err := time.Parse(l, s); err == nil {
				return t, true
			}
		}
		return time.Time{}, false
	}
	for _, l := range lines[1:] {
		key, rest, _ := strings.Cut(l, " ")
		rest = strings.TrimSpace(rest)
		switch key {
		case "title":
			g.title = rest
			continue
		case "dateFormat":
			layout = ganttLayout(rest)
			continue
		case "axisFormat":
			g.axis = rest
			continue
		case "section":
			section = rest
			continue
		case "excludes", "includes", "todayMarker", "tickInterval", "weekday", "inclusiveEndDates", "topAxis",
			"displayMode", "accTitle:", "accDescr:", "click":
			continue
		}
		name, meta, ok := strings.Cut(l, ":")
		if !ok {
			continue
		}
		t := &ganttTask{name: strings.TrimSpace(name), section: section}
		var rest2 []string
		for _, f := range strings.Split(meta, ",") {
			f = strings.TrimSpace(f)
			switch f {
			case "done":
				t.done = true
			case "active":
				t.active = true
			case "crit":
				t.crit = true
			case "milestone":
				t.milestone = true
			case "":
			default:
				rest2 = append(rest2, f)
			}
		}
		// id, start, end; start, end; or the end alone, after the last
		if len(rest2) == 3 {
			t.id, rest2 = rest2[0], rest2[1:]
		}
		var start time.Time
		switch len(rest2) {
		case 1:
			if last != nil {
				start = last.end
			}
		case 2:
			s, ok := when(rest2[0])
			if !ok {
				if last == nil { // an id and a length, after nothing: skip it
					continue
				}
				t.id, s = rest2[0], last.end
			}
			start, rest2 = s, rest2[1:]
		default:
			continue
		}
		if start.IsZero() {
			continue
		}
		t.start = start
		if d, ok := parseDur(rest2[0]); ok {
			t.end = start.Add(d)
		} else if e, ok := when(rest2[0]); ok {
			t.end = e
		} else {
			continue
		}
		if t.end.Before(t.start) {
			t.end = t.start
		}
		if t.id != "" {
			ids[t.id] = t
		}
		g.tasks = append(g.tasks, t)
		last = t
	}
	return g
}

// strftime is t as format says, its % letters as C's.
func strftime(format string, t time.Time) string {
	r := strings.NewReplacer("%Y", "2006", "%y", "06", "%m", "01", "%d", "02", "%e", "_2", "%b", "Jan", "%B", "January",
		"%a", "Mon", "%A", "Monday", "%H", "15", "%I", "03", "%M", "04", "%S", "05", "%p", "PM", "%%", "%")
	return t.Format(r.Replace(format))
}

// ganttTicks are the times the axis marks from t0 to t1, about n, on
// round steps; and how they're written.
func ganttTicks(t0, t1 time.Time, n int) ([]time.Time, string) {
	span := t1.Sub(t0)
	day := 24 * time.Hour
	steps := []struct {
		d   time.Duration
		fmt string
	}{{time.Minute, "%H:%M"}, {5 * time.Minute, "%H:%M"}, {15 * time.Minute, "%H:%M"}, {time.Hour, "%H:%M"},
		{3 * time.Hour, "%H:%M"}, {6 * time.Hour, "%b %d %H:%M"}, {day, "%b %d"}, {2 * day, "%b %d"}, {7 * day, "%b %d"},
		{14 * day, "%b %d"}, {30 * day, "%b %Y"}, {91 * day, "%b %Y"}, {365 * day, "%Y"}}
	st := steps[len(steps)-1]
	for _, s := range steps {
		if span/s.d <= time.Duration(n) {
			st = s
			break
		}
	}
	var ts []time.Time
	var t time.Time
	switch {
	case st.d >= 365*day:
		t = time.Date(t0.Year(), 1, 1, 0, 0, 0, 0, t0.Location())
	case st.d >= 30*day:
		t = time.Date(t0.Year(), t0.Month(), 1, 0, 0, 0, 0, t0.Location())
	case st.d == 7*day || st.d == 14*day: // Mondays
		t = time.Date(t0.Year(), t0.Month(), t0.Day(), 0, 0, 0, 0, t0.Location())
		for t.Weekday() != time.Monday {
			t = t.AddDate(0, 0, -1)
		}
	default:
		t = t0.Truncate(st.d)
	}
	for ; !t.After(t1) && len(ts) < 60; t = stepTime(t, st.d) {
		if !t.Before(t0) {
			ts = append(ts, t)
		}
	}
	return ts, st.fmt
}

// stepTime is t a step d on: months and years by the calendar.
func stepTime(t time.Time, d time.Duration) time.Time {
	day := 24 * time.Hour
	switch d {
	case 365 * day:
		return t.AddDate(1, 0, 0)
	case 91 * day:
		return t.AddDate(0, 3, 0)
	case 30 * day:
		return t.AddDate(0, 1, 0)
	}
	return t.Add(d)
}

func (g *gantt) draw(d *drawer) *picture {
	em := d.em
	lw := max(1.2, em*0.07)
	p := &picture{}
	t0, t1 := g.tasks[0].start, g.tasks[0].end
	for _, t := range g.tasks {
		t0, t1 = minTime(t0, t.start), maxTime(t1, t.end)
	}
	if !t1.After(t0) {
		t1 = t0.Add(24 * time.Hour)
	}
	// the names' column: as wide as the longest, within reason
	nameW := 0.0
	type row struct {
		lab     *label
		section bool
		t       *ganttTask
	}
	var rows []row
	sec := "\x00"
	for _, t := range g.tasks {
		if t.section != sec {
			sec = t.section
			if sec != "" {
				rows = append(rows, row{lab: d.label("**"+sec+"**", em*14), section: true})
			}
		}
		rows = append(rows, row{lab: d.label(t.name, em*12), t: t})
	}
	for _, r := range rows {
		nameW = max(nameW, r.lab.w)
	}
	x0 := nameW + em*0.8
	x1 := x0 + max(em*20, min(em*30, float64(len(g.tasks))*em*2.5))
	X := func(t time.Time) float64 {
		return x0 + float64(t.Sub(t0))/float64(t1.Sub(t0))*(x1-x0)
	}
	y := 0.0
	rh := em * 1.6
	ys := make([]float64, len(rows))
	for i, r := range rows {
		h := max(rh, r.lab.h+em*0.4)
		ys[i] = y + h/2
		y += h
	}
	bottom := y
	// the grid: a line up from each tick
	ticks, tf := ganttTicks(t0, t1, 7)
	if g.axis != "" {
		tf = g.axis
	}
	for i, r := range rows { // the sections' bands
		if r.section {
			h := max(rh, r.lab.h+em*0.4)
			p.fill([]pt{{0, ys[i] - h/2}, {x1, ys[i] - h/2}, {x1, ys[i] + h/2}, {0, ys[i] + h/2}}, 0xec)
		}
	}
	for _, t := range ticks {
		x := X(t)
		p.stroke([]pt{{x, 0}, {x, bottom}}, lw*0.7, 0, 0xc8)
		lb := d.label(strftime(tf, t), 0)
		p.label(lb, pt{x, bottom + em*0.35 + lb.h/2})
	}
	p.stroke([]pt{{x0, bottom}, {x1, bottom}}, lw, 0, 0)
	for i, r := range rows {
		cy := ys[i]
		if r.section {
			p.label(r.lab, pt{r.lab.w/2 + em*0.2, cy})
			continue
		}
		p.label(r.lab, pt{nameW - r.lab.w/2 + em*0.2, cy})
		t := r.t
		a, b := X(t.start), X(t.end)
		if t.milestone || b-a < 1 {
			m := (a + b) / 2
			k := em * 0.45
			dia := []pt{{m, cy - k}, {m + k, cy}, {m, cy + k}, {m - k, cy}}
			p.fill(dia, 0)
			continue
		}
		h := rh * 0.62
		bar := []pt{{a, cy - h/2}, {b, cy - h/2}, {b, cy + h/2}, {a, cy + h/2}}
		fill, edge := uint8(0xb0), lw
		switch {
		case t.crit && t.done:
			fill = 0x70
		case t.crit:
			fill = 0x40
		case t.done:
			fill = 0xe0
		case t.active:
			fill, edge = 0xff, lw*2.4
		}
		p.fill(bar, fill)
		p.stroke(closed(bar), edge, 0, 0)
	}
	if g.title != "" {
		lb := d.label("**"+g.title+"**", x1)
		p.label(lb, pt{x1 / 2, -em*0.6 - lb.h/2})
	}
	return p
}

func minTime(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}

func maxTime(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}
