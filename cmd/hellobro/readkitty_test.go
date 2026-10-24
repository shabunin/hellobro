package main

import (
	"fmt"
	"testing"
)

func TestReadKey(t *testing.T) {
	in := " \r\x7fq7\x1b[6~\x1b[5~\x1bOC\x1b[1~\x1b[4~\x1bx\x1b[2;5H\x1b"
	want := "[  1] [enter 1] [back 1] [q 1] [7 1] [next 4] [prev 4] [next 3] [home 4] [end 4] [ 2] [ 6] [esc 1]"
	var got []string
	for b := []byte(in); len(b) > 0; {
		k, n := readKey(b)
		got = append(got, fmt.Sprintf("[%s %d]", k, n))
		b = b[n:]
	}
	if s := fmt.Sprint(got); s != "["+want+"]" {
		t.Errorf("got %s\nwant [%s]", s, want)
	}
}

func TestReadPlaces(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if p, em := readPlace("/a.pdf"); p != 1 || em != readEm {
		t.Errorf("new: %d %d", p, em)
	}
	readKeep("/a.pdf", 12, 11)
	readKeep("/b b.epub", 3, 14)
	readKeep("/a.pdf", 13, 11)
	if p, em := readPlace("/a.pdf"); p != 13 || em != 11 {
		t.Errorf("a: %d %d", p, em)
	}
	if p, em := readPlace("/b b.epub"); p != 3 || em != 14 {
		t.Errorf("b: %d %d", p, em)
	}
}
