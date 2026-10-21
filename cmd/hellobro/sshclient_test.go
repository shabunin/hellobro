package main

import (
	"strings"
	"testing"
)

func TestParseSSHTarget(t *testing.T) {
	for in, want := range map[string]string{
		"bobalus@192.168.2.14":           "bobalus@192.168.2.14:22",
		"ssh bobalus@192.168.2.14":       "bobalus@192.168.2.14:22",
		"  ssh   bobalus@192.168.2.14  ": "bobalus@192.168.2.14:22",
		"ssh -p 2222 bro@host":           "bro@host:2222",
		"ssh bro@host -p2222":            "bro@host:2222",
		"ssh -l bro host":                "bro@host:22",
		"ssh://bro@host:2222":            "bro@host:2222",
		"bro@host:2222":                  "bro@host:2222",
		"host":                           "root@host:22",
		"bro@[fe80::1]:2222":             "bro@[fe80::1]:2222",
		"ssh -t bro@host tmux new -A":    "bro@host:22 tmux new -A",
		"a@b c@d":                        "a@b:22 c@d",
		"ssh bro@host -p 2222 ls -l":     "bro@host:2222 ls -l",
	} {
		h, err := parseSSHTarget(in)
		if err != nil || strings.TrimSpace(h.String()+" "+h.cmd) != want {
			t.Errorf("%q: got %v, %v; want %s", in, h, err, want)
		}
	}
	for _, bad := range []string{"", "ssh", "ssh -v host", "bro@host:99999", "bro@host:x"} {
		if h, err := parseSSHTarget(bad); err == nil {
			t.Errorf("%q: got %v, want an error", bad, h)
		}
	}
}
