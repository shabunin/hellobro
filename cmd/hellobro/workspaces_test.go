package main

import "testing"

func TestFreeWorkspace(t *testing.T) {
	for _, c := range []struct {
		names []string
		want  string
	}{
		{nil, "1"},
		{[]string{"1"}, "2"},
		{[]string{"1", "2", "3"}, "4"},
		{[]string{"2", "3"}, "1"},
		{[]string{"1", "3"}, "2"},
		{[]string{"1", "mail", "2"}, "3"},
	} {
		if got := freeWorkspace(c.names); got != c.want {
			t.Errorf("freeWorkspace(%q) = %s, want %s", c.names, got, c.want)
		}
	}
}
