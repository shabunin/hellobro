package main

import (
	"image"
	"slices"
	"testing"

	"hellobro/book"
	"hellobro/kobo"
	"hellobro/pdf"
	"hellobro/wm"
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

// A book opened (away from the main loop) is where it was left, its
// bookmarks too, a PDF's as an EPUB's; one closed while it opened is
// let go.
func TestBookOpened(t *testing.T) {
	a := &bro{App: &kobo.App{F: kobo.LoadFonts()}}
	a.wm.tree = wm.New(image.Rect(0, 0, 1000, 800), kobo.FoldH)
	a.wm.wins = map[*wm.Node]window{}
	a.mem.Books = map[string]bookPlace{"/b/x.pdf": {Spine: 2, Pages: "two", Zoom: 150, Marks: []bookMark{{Spine: 1}, {Spine: 9}}}}
	w := &bookWin{path: "/b/x.pdf", opening: true}
	w.opened(a, bookOpened{b: &book.Book{Spine: make([]string, 5)}, pdf: &pdf.Doc{}})
	if w.opening || w.b == nil || w.spine != 2 || w.pageMode != "two" || w.zoom != 150 || !w.panned || len(w.marks) != 1 {
		t.Errorf("opened: spine %d, pages %q, zoom %d, panned %v, marks %v", w.spine, w.pageMode, w.zoom, w.panned, w.marks)
	}
	g := &bookWin{path: "/b/x.pdf", opening: true, gone: true}
	g.opened(a, bookOpened{b: &book.Book{Spine: make([]string, 5)}, pdf: &pdf.Doc{}})
	if g.b != nil {
		t.Errorf("closed, it took the book")
	}
}

// A chapter laid out aside is taken only if it's still the one wanted;
// then the page is found: its last, or the contents' anchor.
func TestLaidOut(t *testing.T) {
	a := &bro{App: &kobo.App{F: kobo.LoadFonts()}}
	a.wm.tree = wm.New(image.Rect(0, 0, 1000, 800), kobo.FoldH)
	a.wm.wins = map[*wm.Node]window{}
	w := &bookWin{b: &book.Book{Spine: make([]string, 3)}, spine: 1, at: book.Pos{Block: 1 << 30}}
	want := bookLayingOut{spine: 1, laid: bookLaid{w: 100, h: 100}}
	w.laying = &want
	ch := &book.Chapter{Blocks: make([]book.Block, 3), IDs: map[string]int{"x": 1}}
	pages := []book.Page{{}, {Start: book.Pos{Block: 1}}, {Start: book.Pos{Block: 2}}}
	w.laidOut(a, bookLayingOut{spine: 0, laid: want.laid}, ch, pages, nil) // an older one's
	if w.pages != nil || w.laying == nil {
		t.Fatalf("took pages not wanted")
	}
	w.turn(a, 1)
	if w.spine != 1 {
		t.Errorf("turned while laying out")
	}
	w.laidOut(a, want, ch, pages, nil)
	if w.laying != nil || w.page != 2 || w.at != pages[2].Start {
		t.Errorf("its last page: page %d at %v", w.page, w.at)
	}
	w.frag, w.at = "x", book.Pos{}
	w.findPage()
	if w.page != 1 || w.frag != "" {
		t.Errorf("the anchor: page %d", w.page)
	}
}

// TestAtEnd: the last page is the book's last chapter's, or with a
// PDF's pages two side by side, the last two (or the last one alone).
func TestAtEnd(t *testing.T) {
	for _, c := range []struct {
		name        string
		spines, at  int
		pages, page int
		two, pdf    bool
		first, last bool
	}{
		{"an epub's first", 3, 0, 4, 0, false, false, true, false},
		{"an epub's last", 3, 2, 4, 3, false, false, false, true},
		{"an epub's last chapter, not its last page", 3, 2, 4, 2, false, false, false, false},
		{"an epub two: the last two", 3, 2, 4, 2, true, false, false, true},
		{"a pdf's last", 10, 9, 1, 0, false, true, false, true},
		{"a pdf two, even: 9-10", 10, 8, 1, 0, true, true, false, true},
		{"a pdf two, even: 7-8", 10, 6, 1, 0, true, true, false, false},
		{"a pdf two, odd: 11 alone", 11, 10, 1, 0, true, true, false, true},
	} {
		w := &bookWin{b: &book.Book{Spine: make([]string, c.spines)}, spine: c.at,
			pages: make([]book.Page, c.pages), page: c.page}
		w.laid.two = c.two
		if c.pdf {
			w.pdf = &pdf.Doc{}
		}
		if got := w.atEnd(-1); got != c.first {
			t.Errorf("%s: at the start %v, want %v", c.name, got, c.first)
		}
		if got := w.atEnd(1); got != c.last {
			t.Errorf("%s: at the end %v, want %v", c.name, got, c.last)
		}
	}
}

// TestListOpenEmpty: ↑ on a list with nothing in it (no bookmarks, a
// PDF with no outline), then Enter: nothing, not a panic.
func TestListOpenEmpty(t *testing.T) {
	for _, mks := range []bool{true, false} {
		w := &bookWin{path: "/b/x.pdf", b: &book.Book{Spine: make([]string, 3)}, list: true, listMks: mks, sel: -2}
		w.listOpen(nil, w.sel)
		if !w.list {
			t.Errorf("marks %v: the list went", mks)
		}
	}
}
