package main

import "testing"

func TestRadioNote(t *testing.T) {
	for _, c := range []struct {
		unread, busy, on bool
		network, want    string
	}{
		{true, false, false, "", "…"},
		{true, true, false, "", "a moment…"},
		{false, true, true, "HomeNet", "a moment…"},
		{false, false, true, "HomeNet", "HomeNet"},
		{false, false, true, "", "on"},
		{false, false, false, "HomeNet", "off"},
	} {
		if got := radioNote(c.unread, c.busy, c.on, c.network); got != c.want {
			t.Errorf("radioNote(%v, %v, %v, %q) = %q, want %q", c.unread, c.busy, c.on, c.network, got, c.want)
		}
	}
}
