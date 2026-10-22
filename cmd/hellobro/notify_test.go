package main

import (
	"testing"

	"github.com/charmbracelet/x/vt"
)

func TestParseNotify(t *testing.T) {
	for in, want := range map[string]string{
		"9;build done":                   "build done",
		"9;4;1;50":                       "", // ConEmu's progress
		"9;":                             "",
		"777;notify;make;all done":       "make: all done",
		"777;notify;;just the body":      "just the body",
		"777;notify;title only":          "title only",
		"777;notify;cmd;a; with ; semis": "cmd: a; with ; semis",
		"777;preexec":                    "",
		"52;c;aGk=":                      "",
	} {
		got, ok := parseNotify(in)
		if got != want || ok != (want != "") {
			t.Errorf("%q: %q %v, want %q", in, got, ok, want)
		}
	}
}

// The emulator hands us the OSC's data with its number, and a bell only
// for a BEL of its own: not the one that ends an OSC.
func TestEmulatorNotify(t *testing.T) {
	emu := vt.NewSafeEmulator(40, 5)
	var got []string
	bells := 0
	emu.SetCallbacks(vt.Callbacks{Bell: func() { bells++ }})
	for _, n := range []int{9, 777} {
		emu.RegisterOscHandler(n, func(data []byte) bool {
			msg, _ := parseNotify(string(data))
			got = append(got, msg)
			return true
		})
	}
	emu.Write([]byte("hi\x1b]9;one\x07there\x1b]777;notify;two;three\x1b\\\x07"))
	if len(got) != 2 || got[0] != "one" || got[1] != "two: three" || bells != 1 {
		t.Errorf("got %q and %d bells", got, bells)
	}
}
