package main

import (
	"slices"
	"testing"
)

func TestPDFSteps(t *testing.T) {
	for _, c := range []struct {
		n, screen int
		want      []int
	}{
		{800, 1000, []int{0}},
		{1000, 1000, []int{0}},
		{1500, 1000, []int{0, 500}},             // the last one flush with the end
		{3000, 1000, []int{0, 950, 1900, 2000}}, // a 20th shows again
	} {
		if got := pdfSteps(c.n, c.screen); !slices.Equal(got, c.want) {
			t.Errorf("pdfSteps(%d, %d) = %v, want %v", c.n, c.screen, got, c.want)
		}
	}
}

func TestFileName(t *testing.T) {
	for in, want := range map[string]string{
		"Athenaze":                           "Athenaze",
		"A World of Heroes: Homer/Sophocles": "A World of Heroes HomerSophocles",
		"Ἀθήναζε, βιβλίον α′":                "Ἀθήναζε βιβλίον α",
		"  lots   of    space.pdf ":          "lots of space pdf",
	} {
		if got := fileName(in, 40); got != want {
			t.Errorf("fileName(%q) = %q, want %q", in, got, want)
		}
	}
	if got := fileName("abcdefghij", 4); got != "abcd" {
		t.Errorf("fileName cut at 4: %q", got)
	}
}
