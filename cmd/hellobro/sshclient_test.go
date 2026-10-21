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
		"ssh -J kobo@vps:2200 me@127.0.0.1 -p 2222 tmux": "me@127.0.0.1:2222 via kobo@vps:2200 tmux",
	} {
		h, err := parseSSHTarget(in)
		if err != nil || strings.TrimSpace(h.String()+" "+h.cmd) != want {
			t.Errorf("%q: got %v, %v; want %s", in, h, err, want)
		}
	}
	for _, bad := range []string{"", "ssh", "ssh -v host", "bro@host:99999", "bro@host:x", "ssh -J vps:x me@pc"} {
		if h, err := parseSSHTarget(bad); err == nil {
			t.Errorf("%q: got %v, want an error", bad, h)
		}
	}
}

func TestSSHConfLine(t *testing.T) {
	for in, want := range map[string]string{
		"pc  me@192.168.1.10  tmux new -A -s kobo":                "pc me@192.168.1.10:22 tmux new -A -s kobo",
		"home  me@127.0.0.1:2222  via kobo@203.0.113.7  tmux new": "home me@127.0.0.1:2222 via kobo@203.0.113.7:22 tmux new",
		"home  me@127.0.0.1:2222  via kobo@vps:2200":              "home me@127.0.0.1:2222 via kobo@vps:2200",
		"odd  me@pc  via": "odd me@pc:22 via", // a command named via, then
	} {
		h, ok := parseSSHConfLine(in)
		if got := strings.Join(strings.Fields(h.line()), " "); !ok || got != want {
			t.Errorf("%q: got %q, %v; want %q", in, got, ok, want)
		}
		if again, ok := parseSSHConfLine(h.line()); !ok || !again.same(h) || again.name != h.name {
			t.Errorf("%q: its line %q reads back as %v", in, h.line(), again)
		}
	}
	for _, bad := range []string{"", "# pc me@pc", "pc", "home me@pc via vps:x"} {
		if h, ok := parseSSHConfLine(bad); ok {
			t.Errorf("%q: got %v, want nothing", bad, h)
		}
	}
}

func TestRelayLine(t *testing.T) {
	for in, want := range map[string]string{
		"kobo@203.0.113.7":        "kobo@203.0.113.7:22, port 2023",
		"  kobo@vps:2200   2099 ": "kobo@vps:2200, port 2099",
	} {
		c, ok, err := parseRelayLine(in)
		if !ok || err != nil || c.String() != want {
			t.Errorf("%q: got %v, %v, %v; want %s", in, c, ok, err, want)
		}
	}
	for _, skip := range []string{"", "  ", "# kobo@vps 2023"} {
		if _, ok, err := parseRelayLine(skip); ok || err != nil {
			t.Errorf("%q: want it skipped, got %v, %v", skip, ok, err)
		}
	}
	for _, bad := range []string{"kobo@vps x", "kobo@vps 0", "kobo@vps:x"} {
		if _, ok, err := parseRelayLine(bad); !ok || err == nil {
			t.Errorf("%q: want an error, got %v, %v", bad, ok, err)
		}
	}
}
