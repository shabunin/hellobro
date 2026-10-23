package keyfeed

import (
	"reflect"
	"testing"
)

func ev(pairs ...int) []Event {
	var out []Event
	for i := 0; i < len(pairs); i += 2 {
		out = append(out, Event{uint16(pairs[i]), int32(pairs[i+1])})
	}
	return out
}

func TestPlain(t *testing.T) {
	for _, c := range []struct {
		name string
		in   string
		want []Event
	}{
		{"letter", "a", ev(30, 1, 30, 0)},
		{"capital", "A", ev(42, 1, 30, 1, 30, 0, 42, 0)},
		{"bang", "!", ev(42, 1, 2, 1, 2, 0, 42, 0)},
		{"enter", "\r", ev(28, 1, 28, 0)},
		{"ctrl+c", "\x03", ev(29, 1, 46, 1, 46, 0, 29, 0)},
		{"esc alone", "\x1b", ev(1, 1, 1, 0)},
		{"alt", "\x1bh", ev(56, 1, 35, 1, 35, 0, 56, 0)},
		{"alt+shift+e", "\x1bE", ev(42, 1, 56, 1, 18, 1, 18, 0, 56, 0, 42, 0)},
		{"up", "\x1b[A", ev(103, 1, 103, 0)},
		{"ctrl+left", "\x1b[1;5D", ev(29, 1, 105, 1, 105, 0, 29, 0)},
		{"delete", "\x1b[3~", ev(111, 1, 111, 0)},
		{"f5", "\x1b[15~", ev(63, 1, 63, 0)},
		{"ss3 f1", "\x1bOP", ev(59, 1, 59, 0)},
		{"backspace", "\x7f", ev(14, 1, 14, 0)},
	} {
		var d Decoder
		if got := d.Feed([]byte(c.in)); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %q gave %v, want %v", c.name, c.in, got, c.want)
		}
	}
}

func TestKitty(t *testing.T) {
	d := Decoder{}
	d.Feed([]byte("\x1b[?11u")) // the terminal answers the query
	if !d.Kitty {
		t.Fatal("the reply didn't say kitty")
	}
	for _, c := range []struct {
		name string
		in   string
		want []Event
	}{
		{"super down", "\x1b[57444;9u", ev(125, 1)},
		{"enter with super", "\x1b[13;9u", ev(28, 1)},
		{"enter up", "\x1b[13;9:3u", ev(28, 0)},
		{"super up", "\x1b[57444;1:3u", ev(125, 0)},
		{"a, repeating", "\x1b[97u\x1b[97;1:2u\x1b[97;1:3u", ev(30, 1, 30, 2, 30, 0)},
		// shift held but never seen going down: pressed for it, let go after
		{"shift unseen", "\x1b[97;2u\x1b[98u", ev(42, 1, 30, 1, 42, 0, 48, 1)},
		{"arrow with ctrl", "\x1b[1;5:1D", ev(29, 1, 105, 1)},
	} {
		if got := d.Feed([]byte(c.in)); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %q gave %v, want %v", c.name, c.in, got, c.want)
		}
	}
}

func TestSplitAndQuit(t *testing.T) {
	var d Decoder
	if got := d.Feed([]byte("\x1b[1;5")); got != nil {
		t.Errorf("half a sequence gave %v", got)
	}
	if got := d.Feed([]byte("A")); !reflect.DeepEqual(got, ev(29, 1, 103, 1, 103, 0, 29, 0)) {
		t.Errorf("its end gave %v", got)
	}
	d.Feed([]byte("x\x1dy"))
	if !d.Quit {
		t.Error("Ctrl+] didn't quit")
	}
	k := Decoder{Kitty: true}
	k.Feed([]byte("\x1b[57442u\x1b[93;5u"))
	if !k.Quit {
		t.Error("Ctrl+] (kitty) didn't quit")
	}
	if got := k.ReleaseAll(); !reflect.DeepEqual(got, ev(29, 0)) {
		t.Errorf("release all: %v", got)
	}
}
