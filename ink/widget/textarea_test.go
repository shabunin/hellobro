package widget_test

import (
	"image"
	"strings"
	"testing"

	"hellobro/ink"
	"hellobro/ink/inktest"
	"hellobro/ink/text"
	"hellobro/ink/widget"
)

// area is a TextArea in a window of its own, in focus, its text s and
// the caret at the end.
func area(t *testing.T, s string) (*widget.TextArea, *inktest.T) {
	t.Helper()
	ta := &widget.TextArea{}
	ta.SetText(s)
	sh := text.NewShaper()
	look := func(gtx ink.Ctx) widget.AreaLook {
		var l widget.AreaLook
		for i := range l.Faces {
			l.Faces[i] = sh.Face(text.Regular, 24)
		}
		l.Fg, l.Quiet, l.Indent, l.Gap, l.Stroke = ink.Black, ink.Dark, 30, 6, 2
		return l
	}
	it := inktest.New(image.Pt(600, 800), ink.Metric{PxPerDp: 1}, func(gtx ink.Ctx) ink.Dims {
		return ta.Layout(gtx, look(gtx), 800)
	})
	it.Tap(image.Pt(590, 790)) // the end
	if !ta.Focused() {
		t.Fatal("a tap didn't focus it")
	}
	it.Key("End", ink.Ctrl)
	return ta, it
}

func TestTextAreaTyping(t *testing.T) {
	ta, it := area(t, "one")
	it.Type(" two")
	it.Key("Enter", 0)
	it.Type("three")
	if got := ta.Text(); got != "one two\nthree" {
		t.Fatalf("typed: %q", got)
	}
	it.Key("Back", 0)
	it.Key("Left", ink.Ctrl)
	it.Key("Back", 0) // joins the lines
	if got := ta.Text(); got != "one twothre" {
		t.Errorf("back: %q", got)
	}
	it.Key("z", ink.Ctrl)
	it.Key("z", ink.Ctrl)
	if got := ta.Text(); got != "one two\nthree" {
		t.Errorf("undone: %q", got)
	}
	it.Key("a", ink.Ctrl)
	it.Key("x", ink.Ctrl)
	if ta.Text() != "" {
		t.Errorf("cut all: %q", ta.Text())
	}
	it.Key("v", ink.Ctrl)
	it.Key("v", ink.Ctrl)
	if ta.Text() != "one two\nthreeone two\nthree" {
		t.Errorf("pasted twice: %q", ta.Text())
	}
}

// A list goes on with Enter, and ends on an empty item; Back at an
// item's start takes it out of the list.
func TestTextAreaLists(t *testing.T) {
	ta, it := area(t, "- milk")
	it.Key("Enter", 0)
	it.Type("bread")
	it.Key("Enter", 0)
	if got := ta.Text(); got != "- milk\n- bread\n- " {
		t.Fatalf("a list: %q", got)
	}
	it.Key("Enter", 0) // empty: the list ends
	it.Type("done")
	if got := ta.Text(); got != "- milk\n- bread\ndone" {
		t.Errorf("ended: %q", got)
	}
	it.Key("Up", 0)
	it.Key("Home", 0)
	it.Key("Back", 0)
	if got := ta.Text(); got != "- milk\nbread\ndone" {
		t.Errorf("back at its start: %q", got)
	}
	// The caret never goes into a mark: ← from an item's start goes to
	// the end of the line before.
	it.Key("Home", ink.Ctrl)
	if ta.Caret() != 2 {
		t.Errorf("at the start: caret %d, want 2 (past \"- \")", ta.Caret())
	}
	it.Key("t", ink.Ctrl) // a to-do
	if !strings.HasPrefix(ta.Text(), "- [ ] milk") {
		t.Errorf("Ctrl+T: %q", ta.Text())
	}
}

// A tap on a to-do's box ticks it, and again unticks it.
func TestTextAreaTodo(t *testing.T) {
	ta, it := area(t, "- [ ] call\n- [ ] write")
	it.Tap(image.Pt(15, 15)) // the first box, in the indent
	if !strings.HasPrefix(ta.Text(), "- [x] call") {
		t.Fatalf("ticked: %q", ta.Text())
	}
	it.Tap(image.Pt(15, 15))
	if !strings.HasPrefix(ta.Text(), "- [ ] call") {
		t.Errorf("unticked: %q", ta.Text())
	}
	if ta.Words() != "call\nwrite" {
		t.Errorf("words: %q", ta.Words())
	}
}

// Long lines wrap; ↓ goes down the lines the wrap made, keeping its place
// across.
func TestTextAreaWrap(t *testing.T) {
	long := strings.Repeat("word ", 60)
	ta, it := area(t, long)
	it.Key("Home", ink.Ctrl)
	it.Key("Right", 0)
	it.Key("Right", 0)
	it.Key("Down", 0)
	if c := ta.Caret(); c < 20 || c > len(long)/2 {
		t.Errorf("↓ from the first line: caret %d", c)
	}
	r, _ := ta.CaretRect()
	if r.Min.Y == 0 {
		t.Errorf("↓: still on the first line, %v", r)
	}
}
