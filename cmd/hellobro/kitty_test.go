package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/charmbracelet/x/vt"
)

func TestKittyKey(t *testing.T) {
	const a, c, up, f3, enter, esc = 30, 46, 103, 61, 28, 1
	tests := []struct {
		name  string
		code  uint16
		value int32
		m     termMods
		f     int
		want  string
		ok    bool
	}{
		{"text, disambiguate", a, 1, termMods{}, 1, "", false},
		{"ctrl+c, disambiguate", c, 1, termMods{ctrl: true}, 1, "\x1b[99;5u", true},
		{"super+a", a, 1, termMods{meta: true}, 1, "\x1b[97;9u", true},
		{"esc", esc, 1, termMods{}, 1, "\x1b[27u", true},
		{"enter", enter, 1, termMods{}, 1, "", false},
		{"shift+enter", enter, 1, termMods{shift: true}, 1, "\x1b[13;2u", true},
		{"up", up, 1, termMods{}, 1, "", false},
		{"ctrl+up", up, 1, termMods{ctrl: true}, 1, "\x1b[1;5A", true},
		{"shift+f3", f3, 1, termMods{shift: true}, 1, "\x1b[13;2~", true},
		{"a release, no events", a, 0, termMods{}, 1, "", true},
		{"a repeat, no events", c, 2, termMods{ctrl: true}, 1, "\x1b[99;5u", true},
		{"a, all keys", a, 1, termMods{}, 11, "\x1b[97u", true},
		{"a release", a, 0, termMods{}, 11, "\x1b[97;1:3u", true},
		{"a repeat", a, 2, termMods{}, 11, "\x1b[97;1:2u", true},
		{"shift+a", a, 1, termMods{shift: true}, 11, "\x1b[97;2u", true},
		{"caps a", a, 1, termMods{caps: true}, 11, "\x1b[97;65u", true},
		{"super down", keyLeftMeta, 1, termMods{meta: true}, 11, "\x1b[57444;9u", true},
		{"super up", keyLeftMeta, 0, termMods{}, 11, "\x1b[57444;1:3u", true},
		{"super, disambiguate", keyLeftMeta, 1, termMods{meta: true}, 1, "", true},
		{"up, all keys", up, 1, termMods{}, 11, "\x1b[A", true},
		{"up release", up, 0, termMods{}, 11, "\x1b[1;1:3A", true},
		{"enter, all keys", enter, 1, termMods{}, 11, "\x1b[13u", true},
		{"shift+a, alternates and text", a, 1, termMods{shift: true}, 31, "\x1b[97:65;2;65u", true},
		{"a, text", a, 1, termMods{}, 24, "\x1b[97;1;97u", true},
	}
	for _, tt := range tests {
		got, ok := kittyKey(tt.code, tt.value, tt.m, tt.f)
		if got != tt.want || ok != tt.ok {
			t.Errorf("%s: %q %v, want %q %v", tt.name, got, ok, tt.want, tt.ok)
		}
	}
}

func TestKittyStack(t *testing.T) {
	var k kittyKeys
	if r := k.csi(0, '?', 0, 1); r != "\x1b[?0u" {
		t.Fatalf("query: %q", r)
	}
	k.csi(0, '>', 11, 1)
	k.csi(0, '>', 1, 1)
	if f := k.flags(0); f != 1 {
		t.Errorf("pushed: %d", f)
	}
	if f := k.flags(1); f != 0 {
		t.Errorf("the alternate screen's: %d", f)
	}
	k.csi(0, '=', 8, 2)
	if f := k.flags(0); f != 9 {
		t.Errorf("or: %d", f)
	}
	k.csi(0, '<', 0, 1)
	if r := k.csi(0, '?', 0, 1); r != "\x1b[?11u" {
		t.Errorf("popped: %q", r)
	}
	k.csi(0, '<', 5, 1)
	if f := k.flags(0); f != 0 {
		t.Errorf("popped them all: %d", f)
	}
	for i := 0; i < 20; i++ {
		k.csi(0, '>', i, 1)
	}
	if n := len(k.stack[0]); n != kittyStackMax {
		t.Errorf("stack %d deep", n)
	}
}

// TestKittyTerminal: a program asks a terminal of ours for the protocol,
// as hellobro-keys does, and the keys go out the kitty way: Super too.
func TestKittyTerminal(t *testing.T) {
	ts := &termSession{state: termConnected}
	ts.emu = vt.NewSafeEmulator(20, 4)
	ts.kittyRegister(ts.emu)
	out := make(chan string, 16)
	go func() {
		buf := make([]byte, 256)
		for {
			n, err := ts.emu.Read(buf)
			if err != nil {
				return
			}
			out <- string(buf[:n])
		}
	}()
	read := func() string {
		select {
		case b := <-out:
			return b
		case <-time.After(time.Second):
			t.Fatal("nothing from the terminal")
		}
		return ""
	}
	ts.emu.Write([]byte("\x1b[>11u\x1b[?u"))
	if r := read(); r != "\x1b[?11u" {
		t.Fatalf("asked for the flags: %q", r)
	}
	const keyA = 30
	steps := []struct {
		code  uint16
		value int32
		m     termMods
	}{
		{keyLeftMeta, 1, termMods{meta: true}},
		{keyA, 1, termMods{meta: true}},
		{keyA, 0, termMods{meta: true}},
		{keyLeftMeta, 0, termMods{}},
	}
	var got []string
	for _, s := range steps {
		if !ts.kittySend(s.code, s.value, s.m) {
			t.Fatalf("%d %d: not sent the kitty way", s.code, s.value)
		}
		got = append(got, read())
	}
	want := []string{"\x1b[57444;9u", "\x1b[97;9u", "\x1b[97;9:3u", "\x1b[57444;1:3u"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("got %q, want %q", got, want)
	}
	ts.kittySend(keyA, 0, termMods{}) // a release whose press wasn't sent: nothing
	select {
	case b := <-out:
		t.Errorf("a release for no press: %q", b)
	case <-time.After(50 * time.Millisecond):
	}
	ts.emu.Write([]byte("\x1b[<u"))
	if ts.kittySend(keyA, 1, termMods{}) {
		t.Error("popped: still the kitty way")
	}
}
