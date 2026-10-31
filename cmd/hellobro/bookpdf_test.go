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

func TestPDFFind(t *testing.T) {
	pages := []string{"Cover", "Chapter one:\nthe little\nengine", "a hyphen-\nated word", "THE END"}
	text := func(i int) (string, error) { return pages[i], nil }
	never := func() bool { return false }
	for _, c := range []struct {
		from int
		q    string
		page int
		ok   bool
	}{
		{0, "little engine", 1, true}, // across a line break
		{0, "hyphenated", 2, true},    // a hyphen at a line's end
		{2, "the", 3, true},           // from the page on
		{3, "cover", 0, true},         // round to the start
		{1, "the", 1, true},           // the page itself first
		{0, "nowhere", 0, false},
		{0, "  ", 0, false},
	} {
		page, ok := pdfFind(len(pages), c.from, c.q, text, never)
		if page != c.page || ok != c.ok {
			t.Errorf("pdfFind(from %d, %q) = %d, %v; want %d, %v", c.from, c.q, page, ok, c.page, c.ok)
		}
	}
}
