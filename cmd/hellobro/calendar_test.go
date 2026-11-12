package main

import (
	"testing"
	"time"

	"hellobro/cal"
)

func TestCalDay(t *testing.T) {
	today := time.Date(2026, 11, 12, 0, 0, 0, 0, time.Local) // a Thursday
	for _, tc := range []struct {
		d    time.Time
		want string
	}{
		{today.Add(15 * time.Hour), "today"},
		{today.AddDate(0, 0, 1), "tomorrow"},
		{today.AddDate(0, 0, -1), "yesterday"},
		{today.AddDate(0, 0, 3), "Sunday"},
		{today.AddDate(0, 0, 9), "21 Nov"},
		{today.AddDate(0, 2, 0), "12 Jan 2027"},
	} {
		if got := calDay(tc.d, today); got != tc.want {
			t.Errorf("%v: %q, want %q", tc.d, got, tc.want)
		}
	}
}

func TestOnDay(t *testing.T) {
	d := time.Date(2026, 11, 14, 0, 0, 0, 0, time.Local)
	mk := func(kind string, start, end time.Time, allDay bool) cal.Occurrence {
		it := &cal.Item{Kind: kind, Start: start, End: end, AllDay: allDay}
		at := start
		if kind == cal.Task {
			it.Start, at = time.Time{}, end
		}
		return cal.Occurrence{Entry: &cal.Entry{Item: it}, At: at}
	}
	os := []cal.Occurrence{
		mk(cal.Event, d.Add(9*time.Hour), d.Add(10*time.Hour), false),               // timed, on it
		mk(cal.Event, d.AddDate(0, 0, -1), d.AddDate(0, 0, 2), true),                // over it
		mk(cal.Event, d.AddDate(0, 0, -2), d, true),                                 // ended the day before
		mk(cal.Task, time.Time{}, d, true),                                          // due on it
		mk(cal.Event, d.AddDate(0, 0, 1).Add(time.Hour), d.AddDate(0, 0, 1), false), // the next day
	}
	timed, allDay := onDay(os, d)
	if len(timed) != 1 || len(allDay) != 2 {
		t.Errorf("timed %d, all day %d", len(timed), len(allDay))
	}
}
