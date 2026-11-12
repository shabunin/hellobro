package kobo

import (
	"testing"
	"time"
)

func TestAlarmRang(t *testing.T) {
	at := time.Date(2026, 11, 13, 0, 0, 30, 0, time.Local)
	for _, tc := range []struct {
		now  time.Time
		want bool
	}{
		{at, true},
		{at.Add(-5 * time.Second), true}, // the RTC a little early
		{at.Add(40 * time.Second), true},
		{at.Add(-time.Hour), false}, // the power button, before it
		{at.Add(2 * time.Minute), false},
	} {
		if got := alarmRang(at, tc.now); got != tc.want {
			t.Errorf("%v: %v", tc.now, got)
		}
	}
}
