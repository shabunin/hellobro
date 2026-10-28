package main

import (
	"slices"
	"testing"

	"hellobro/book"
)

func TestBookMarks(t *testing.T) {
	w := &bookWin{
		b: &book.Book{Spine: []string{"a", "b", "c"}, TOC: []book.TOCEntry{
			{Title: "One", Spine: 0}, {Title: "Two", Spine: 1}, {Title: "Two, part 2", Spine: 1, Frag: "p2"},
		}},
		spine: 1,
		ch: &book.Chapter{Blocks: []book.Block{
			{Spans: []book.Span{{Text: "Down the "}, {Text: "Rabbit", Style: book.Italic}, {Text: "-Hole"}}},
			{Spans: []book.Span{{Text: "Alice was begin­ning to get   very tired"}}},
		}},
		pages: []book.Page{{Start: book.Pos{}}, {Start: book.Pos{Block: 1, Off: 6}}, {Start: book.Pos{Block: 1, Off: 30}}},
		page:  1,
	}
	w.marks = []bookMark{
		{Spine: 0, Block: 3},
		{Spine: 1, Block: 0, Off: 4},  // page 0
		{Spine: 1, Block: 1, Off: 6},  // page 1
		{Spine: 1, Block: 1, Off: 29}, // page 1
		{Spine: 1, Block: 1, Off: 30}, // page 2
		{Spine: 2},
	}
	if got := w.pageMarks(); !slices.Equal(got, []int{2, 3}) {
		t.Errorf("pageMarks on page 1: %v, want [2 3]", got)
	}
	w.page = 2
	if got := w.pageMarks(); !slices.Equal(got, []int{4}) {
		t.Errorf("pageMarks on the last page: %v, want [4]", got)
	}

	w.page = 0
	if got, want := w.pageWords(30), "Down the Rabbit-Hole Alice was…"; got != want {
		t.Errorf("pageWords: %q, want %q", got, want)
	}
	w.page = 1
	if got, want := w.pageWords(80), "was beginning to get very tired"; got != want {
		t.Errorf("pageWords: %q, want %q", got, want)
	}

	w.marks = []bookMark{{Pct: 9, Text: "ONE Printed in England"}, {Spine: 1, Pct: 12, Text: "Alice was"}}
	if got, want := w.markLines(), []string{"9%  ·  One  ·  Printed in England", "12%  ·  Two  ·  Alice was"}; !slices.Equal(got, want) {
		t.Errorf("markLines: %q, want %q", got, want)
	}

	for spine, want := range []string{"One", "Two", "Two"} {
		if got := w.tocName(spine); got != want {
			t.Errorf("tocName(%d): %q, want %q", spine, got, want)
		}
	}
}
