package cal

import (
	"os"
	"strings"
	"testing"
	"time"
)

const sample = "BEGIN:VCALENDAR\r\n" +
	"VERSION:2.0\r\n" +
	"PRODID:-//someone//EN\r\n" +
	"BEGIN:VTIMEZONE\r\nTZID:Europe/Berlin\r\nEND:VTIMEZONE\r\n" +
	"BEGIN:VEVENT\r\n" +
	"UID:abc@example.com\r\n" +
	"DTSTAMP:20261101T100000Z\r\n" +
	"DTSTART;TZID=Europe/Berlin:20261105T090000\r\n" +
	"DTEND;TZID=Europe/Berlin:20261105T100000\r\n" +
	"SUMMARY:stand-up\\, with tea\r\n" +
	"DESCRIPTION:first line\\nsecond line that goes on and on so that the line is fo\r\n" +
	" lded here\r\n" +
	"RRULE:FREQ=WEEKLY;BYDAY=TH;COUNT=4\r\n" +
	"EXDATE;TZID=Europe/Berlin:20261112T090000\r\n" +
	"X-SOMETHING;X-PARAM=\"a:b\":kept as it was\r\n" +
	"BEGIN:VALARM\r\nACTION:DISPLAY\r\nTRIGGER:-PT15M\r\nEND:VALARM\r\n" +
	"END:VEVENT\r\n" +
	"END:VCALENDAR\r\n"

func TestParseRoundTrip(t *testing.T) {
	c, err := ParseString(sample)
	if err != nil {
		t.Fatal(err)
	}
	// Folded where we fold, the same lines.
	a, _ := unfold(strings.NewReader(sample))
	b, _ := unfold(strings.NewReader(c.String()))
	if strings.Join(a, "\n") != strings.Join(b, "\n") {
		t.Errorf("round trip:\n%s\nwant\n%s", c.String(), sample)
	}
	ev := c.Sub("VEVENT")
	if s := ev.Text("SUMMARY"); s != "stand-up, with tea" {
		t.Errorf("summary %q", s)
	}
	if s := ev.Text("DESCRIPTION"); !strings.Contains(s, "\nsecond line") || !strings.Contains(s, "folded here") {
		t.Errorf("description %q", s)
	}
	if p := ev.Get("X-SOMETHING"); p.Param("X-PARAM") != "a:b" {
		t.Errorf("param %q", p.Param("X-PARAM"))
	}
}

func TestFold(t *testing.T) {
	c := &Comp{Name: "VEVENT"}
	c.SetText("SUMMARY", strings.Repeat("ж", 100)) // two bytes each
	s := c.String()
	for _, l := range strings.Split(strings.TrimSuffix(s, "\r\n"), "\r\n") {
		if len(l) > 75 {
			t.Errorf("line of %d bytes", len(l))
		}
	}
	back, err := ParseString("BEGIN:VCALENDAR\r\n" + s + "END:VCALENDAR\r\n")
	if err != nil {
		t.Fatal(err)
	}
	if got := back.Sub("VEVENT").Text("SUMMARY"); got != strings.Repeat("ж", 100) {
		t.Errorf("folded and back: %q", got)
	}
}

func TestParseErrors(t *testing.T) {
	for _, s := range []string{
		"",
		"BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nEND:VCALENDAR\r\n",
		"SUMMARY:x\r\n",
		"BEGIN:VCALENDAR\r\nX;P=\"open:v\r\nEND:VCALENDAR\r\n",
	} {
		if _, err := ParseString(s); err == nil {
			t.Errorf("%q: no error", s)
		}
	}
}

func TestItem(t *testing.T) {
	c, _ := ParseString(sample)
	it, err := ItemOf(c)
	if err != nil {
		t.Fatal(err)
	}
	berlin, _ := time.LoadLocation("Europe/Berlin")
	want := time.Date(2026, 11, 5, 9, 0, 0, 0, berlin)
	if !it.Start.Equal(want) || it.End.Sub(it.Start) != time.Hour || it.AllDay {
		t.Errorf("start %v end %v", it.Start, it.End)
	}
	from := time.Date(2026, 11, 1, 0, 0, 0, 0, berlin)
	got := it.Occurs(from, from.AddDate(0, 2, 0))
	var days []int
	for _, o := range got {
		days = append(days, o.In(berlin).Day())
	}
	if want := []int{5, 19, 26}; !equal(days, want) { // the 12th excluded; 4 in all
		t.Errorf("days %v, want %v", days, want)
	}

	// Changed and written: the rest stays.
	it.Summary = "stand-up"
	it.Apply(time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC))
	s := it.Cal.String()
	for _, w := range []string{"SUMMARY:stand-up\r\n", "X-SOMETHING", "BEGIN:VALARM", "SEQUENCE:1\r\n", "BEGIN:VTIMEZONE", "LAST-MODIFIED:20261102T000000Z"} {
		if !strings.Contains(s, w) {
			t.Errorf("no %q in\n%s", w, s)
		}
	}
	again, _ := ParseString(s)
	it2, _ := ItemOf(again)
	if !it2.Start.Equal(it.Start) || !it2.End.Equal(it.End) {
		t.Errorf("times moved: %v %v", it2.Start, it2.End)
	}
}

func equal(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestTask(t *testing.T) {
	it := New(Task)
	it.Summary = "buy milk"
	it.End = time.Date(2026, 11, 14, 0, 0, 0, 0, time.Local)
	it.AllDay = true
	it.Priority = 1
	now := time.Date(2026, 11, 12, 8, 0, 0, 0, time.UTC)
	it.Apply(now)
	s := it.Cal.String()
	for _, w := range []string{"BEGIN:VTODO", "DUE;VALUE=DATE:20261114", "STATUS:NEEDS-ACTION", "PRIORITY:1"} {
		if !strings.Contains(s, w) {
			t.Errorf("no %q in\n%s", w, s)
		}
	}
	c0, _ := ParseString(it.Cal.String())
	if back, _ := ItemOf(c0); !back.AllDay || !back.End.Equal(it.End) {
		t.Errorf("due on a day: %+v", back)
	}
	it.Done = true
	it.Apply(now)
	s = it.Cal.String()
	if !strings.Contains(s, "STATUS:COMPLETED") || !strings.Contains(s, "COMPLETED:20261112T080000Z") {
		t.Errorf("done:\n%s", s)
	}
	c, _ := ParseString(s)
	back, _ := ItemOf(c)
	if !back.Done || !back.DoneAt.Equal(now) || back.Kind != Task || !back.End.Equal(it.End) {
		t.Errorf("back %+v", back)
	}
	it.Done = false
	it.Apply(now)
	if s := it.Cal.String(); strings.Contains(s, "COMPLETED:") {
		t.Errorf("not done, still:\n%s", s)
	}
}

func TestTimes(t *testing.T) {
	for _, tc := range []struct{ prop, want string }{
		{"DTSTART:20261105T090000Z", "2026-11-05 09:00 UTC"},
		{"DTSTART;VALUE=DATE:20261105", "2026-11-05 00:00 local"},
		{"DTSTART;TZID=W. Europe Standard Time:20260705T090000", "2026-07-05 07:00 UTC"},
		{"DTSTART:20261105T090000", "2026-11-05 09:00 local"},
	} {
		p, err := parseLine(tc.prop)
		if err != nil {
			t.Fatal(err)
		}
		tm, _ := parseTime(p)
		var got string
		if strings.HasSuffix(tc.want, "local") {
			got = tm.Format("2006-01-02 15:04") + " local"
		} else {
			got = tm.UTC().Format("2006-01-02 15:04") + " UTC"
		}
		if got != tc.want {
			t.Errorf("%s: %s, want %s", tc.prop, got, tc.want)
		}
	}
	for _, tc := range []struct {
		s    string
		want time.Duration
	}{{"PT1H30M", 90 * time.Minute}, {"P1D", 24 * time.Hour}, {"-PT15M", -15 * time.Minute}, {"P2W", 14 * 24 * time.Hour}} {
		if d, ok := parseDuration(tc.s); !ok || d != tc.want {
			t.Errorf("%s: %v", tc.s, d)
		}
	}
}

func TestRecurrence(t *testing.T) {
	loc := time.Local
	day := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 10, 0, 0, 0, loc) }
	for _, tc := range []struct {
		rule  string
		start time.Time
		from  time.Time
		days  int
		want  []string
	}{
		{"FREQ=DAILY;COUNT=3", day(2026, 11, 1), day(2026, 10, 1), 60, []string{"11-01", "11-02", "11-03"}},
		{"FREQ=DAILY;INTERVAL=10", day(2026, 11, 1), day(2026, 11, 15), 20, []string{"11-21", "12-01"}},
		{"FREQ=WEEKLY;BYDAY=MO,WE", day(2026, 11, 4), day(2026, 11, 1), 11, []string{"11-04", "11-09", "11-11"}},
		{"FREQ=WEEKLY;INTERVAL=2;UNTIL=20261201", day(2026, 11, 2), day(2026, 11, 1), 60, []string{"11-02", "11-16", "11-30"}},
		{"FREQ=MONTHLY", day(2026, 1, 31), day(2026, 1, 1), 100, []string{"01-31", "03-31"}},
		{"FREQ=MONTHLY;BYMONTHDAY=-1", day(2026, 1, 31), day(2026, 1, 1), 90, []string{"01-31", "02-28", "03-31"}},
		{"FREQ=MONTHLY;BYDAY=2MO", day(2026, 11, 9), day(2026, 11, 1), 72, []string{"11-09", "12-14", "01-11"}},
		{"FREQ=MONTHLY;BYDAY=-1FR", day(2026, 11, 27), day(2026, 11, 1), 60, []string{"11-27", "12-25"}},
		{"FREQ=YEARLY", day(2024, 2, 29), day(2024, 1, 1), 365 * 5, []string{"02-29", "02-29"}},
	} {
		it := &Item{Kind: Event, Start: tc.start, End: tc.start.Add(time.Hour), RRule: tc.rule}
		var got []string
		for _, o := range it.Occurs(tc.from, tc.from.AddDate(0, 0, tc.days)) {
			got = append(got, o.Format("01-02"))
			if o.Hour() != 10 {
				t.Errorf("%s: at %v", tc.rule, o)
			}
		}
		if strings.Join(got, " ") != strings.Join(tc.want, " ") {
			t.Errorf("%s: %v, want %v", tc.rule, got, tc.want)
		}
	}
	// An event that started before the range and goes into it counts.
	it := &Item{Kind: Event, Start: day(2026, 11, 1), End: day(2026, 11, 3)}
	if got := it.Occurs(day(2026, 11, 2), day(2026, 11, 4)); len(got) != 1 {
		t.Errorf("spanning: %v", got)
	}
}

func TestConf(t *testing.T) {
	p := t.TempDir() + "/calendar.conf"
	os.WriteFile(p, []byte("# mine\nurl = dav.example.com/\nuser = me\npassword = a b c\nevery = 30\n"), 0o644)
	c, err := LoadConf(p)
	if err != nil || c.URL != "https://dav.example.com/" || c.User != "me" || c.Password != "a b c" || c.Every != 30*time.Minute {
		t.Errorf("%+v %v", c, err)
	}
	os.WriteFile(p, []byte("user = me\n"), 0o644)
	if _, err := LoadConf(p); err == nil {
		t.Error("no url: no error")
	}
}
