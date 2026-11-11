package main

import (
	"testing"

	"golang.org/x/image/font/basicfont"
)

func TestLettersIn(t *testing.T) {
	f := basicfont.Face7x13 // 7 px a letter: middles at 4, 11, 18, 25…
	for _, c := range []struct {
		text     string
		lo, hi   int
		from, to int
		x0, x1   int
	}{
		{"hello world", 14, 35, 2, 5, 14, 35}, // "llo"
		{"hello world", 0, 100, 0, 11, 0, 77}, // all of it
		{"hello world", 5, 10, 0, 0, 0, 0},    // between two middles: none
		{"hello world", 80, 120, 0, 0, 0, 0},  // past its end
		{"héllo", 7, 14, 1, 3, 7, 14},         // "é": two bytes
	} {
		from, to, x0, x1 := lettersIn(c.text, f, c.lo, c.hi)
		if from != c.from || to != c.to || x0 != c.x0 || x1 != c.x1 {
			t.Errorf("lettersIn(%q, %d, %d) = %d, %d, %d, %d; want %d, %d, %d, %d", c.text, c.lo, c.hi, from, to, x0, x1, c.from, c.to, c.x0, c.x1)
		}
	}
}
