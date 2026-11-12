package ink

import (
	"image"
	"time"
)

// Event is input for a tag: a Pointer, Key, Text or Focus.
type Event interface{ isEvent() }

// PointerKind is what a pointer did.
type PointerKind uint8

const (
	Press PointerKind = iota
	Move
	Release
	Cancel // it's someone else's now (a swipe), or the window went
)

func (k PointerKind) String() string {
	return [...]string{"press", "move", "release", "cancel"}[k]
}

// Source is what touched: a finger or the pen.
type Source uint8

const (
	Finger Source = iota
	Pen
)

// Pointer is a contact coming down, moving, lifting. Pos is in the
// receiving area's coordinates (the window's when it's fed in).
type Pointer struct {
	Kind     PointerKind
	ID       int // the contact: a finger's slot
	Pos      image.Point
	Source   Source
	Time     time.Time
	Pressure float32 // the pen's, 0 to 1; 0: not known (a finger)
	Eraser   bool    // the pen's button held, or its eraser end down
}

// Mods are the modifier keys held.
type Mods uint8

const (
	Shift Mods = 1 << iota
	Ctrl
	Alt
	Super
)

// Key is a key pressed (or repeating). Name is "Tab", "Enter", "Space",
// "Esc", "Back" (backspace), "Delete", "Up", "Down", "Left", "Right",
// "Home", "End", "PageUp", "PageDown", or a letter or digit as on the
// key, lower case.
type Key struct {
	Name   string
	Mods   Mods
	Repeat bool
}

// Text is text typed: what the keys made of it, Shift and the layout's.
type Text struct {
	Text string
}

// Focus says the tag got the keys or lost them.
type Focus struct {
	On bool
}

// Reveal asks an area that scrolls (Area.Reveal) to show R, in its own
// coordinates: the keys moved the focus to something in it that doesn't
// show, or not all of it.
type Reveal struct {
	R image.Rectangle
}

func (Pointer) isEvent() {}
func (Key) isEvent()     {}
func (Text) isEvent()    {}
func (Focus) isEvent()   {}
func (Reveal) isEvent()  {}

// Root is the tag that gets the keys no one in focus took: a program's
// Esc, its letters.
var Root Tag = &rootTag{}

type rootTag struct{ _ byte }
