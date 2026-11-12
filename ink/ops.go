package ink

import (
	"image"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// Ops are a frame's operations, as the widgets add them: paint, offsets
// and clips, input areas. A widget adds them where it is (0, 0), and the
// one laying it out moves it with Offset; Record keeps some aside to be
// put in later, once their size is known (a Flex's children).
type Ops struct {
	list []op
}

// Reset empties it for the next frame.
func (o *Ops) Reset() { o.list = o.list[:0] }

type opKind uint8

const (
	opFill opKind = iota
	opFrame
	opText
	opImage
	opDraw
	opInvert
	opPush // an offset and a clip, till its pop
	opPop
	opCall
	opArea
	opLine
	opDisc
	opPoly
	opPopup
)

type op struct {
	kind  opKind
	r     image.Rectangle // fill, frame, invert, image, draw: where; push: its clip (empty: none)
	off   image.Point     // push: the offset; text: the baseline's start
	gray  uint8
	t     int // frame: thickness
	face  font.Face
	s     string
	img   *image.Gray
	sp    image.Point // image: from where in it
	draw  func(dst *image.Gray, r image.Rectangle)
	call  []op
	area  Area
	pts   []image.Point // line, poly
	tag   Tag           // popup: its backdrop's
	clips bool          // push: r clips
}

// Gray levels the panel shows cleanly: multiples of 0x11. Black and
// White are all a DU update shows.
const (
	Black = 0x00
	Dark  = 0x44
	Mid   = 0x88
	Light = 0xDD
	White = 0xFF
)

// Fill paints r gray g.
func Fill(o *Ops, r image.Rectangle, g uint8) {
	o.list = append(o.list, op{kind: opFill, r: r, gray: g})
}

// Frame paints a border t thick just inside r.
func Frame(o *Ops, r image.Rectangle, t int, g uint8) {
	o.list = append(o.list, op{kind: opFrame, r: r, t: t, gray: g})
}

// TextAt paints s in face f, its baseline starting at x, y.
func TextAt(o *Ops, f font.Face, x, y int, s string, g uint8) {
	o.list = append(o.list, op{kind: opText, face: f, off: image.Pt(x, y), s: s, gray: g})
}

// Image puts src's pixels from sp on into r, as they are.
func Image(o *Ops, r image.Rectangle, src *image.Gray, sp image.Point) {
	o.list = append(o.list, op{kind: opImage, r: r, img: src, sp: sp})
}

// Draw has f paint r itself, onto dst (dst's bounds are r, clipped, in
// the frame's own coordinates): for what's drawn its own way, a page of
// a book, a notebook's ink, a terminal's cells.
func Draw(o *Ops, r image.Rectangle, f func(dst *image.Gray, r image.Rectangle)) {
	o.list = append(o.list, op{kind: opDraw, r: r, draw: f})
}

// Invert turns r's black white and white black: a button held down.
func Invert(o *Ops, r image.Rectangle) {
	o.list = append(o.list, op{kind: opInvert, r: r})
}

// Stack is an offset or a clip pushed, till its Pop.
type Stack struct {
	o *Ops
}

// Pop ends it.
func (s Stack) Pop() { s.o.list = append(s.o.list, op{kind: opPop}) }

// Offset moves what's added till the Pop by p.
func Offset(o *Ops, p image.Point) Stack {
	o.list = append(o.list, op{kind: opPush, off: p})
	return Stack{o}
}

// Clip keeps what's added till the Pop within r (paint and input both).
func Clip(o *Ops, r image.Rectangle) Stack {
	o.list = append(o.list, op{kind: opPush, r: r, clips: true})
	return Stack{o}
}

// Macro is ops being recorded, to be put in later.
type Macro struct {
	o     *Ops
	start int
}

// Record starts keeping what's added aside.
func Record(o *Ops) Macro { return Macro{o, len(o.list)} }

// Stop ends the recording: what was added since is in the CallOp now,
// not in the ops.
func (m Macro) Stop() CallOp {
	c := CallOp{append([]op(nil), m.o.list[m.start:]...)}
	m.o.list = m.o.list[:m.start]
	return c
}

// CallOp is recorded ops.
type CallOp struct {
	ops []op
}

// Add puts them in o, where o is now.
func (c CallOp) Add(o *Ops) {
	if len(c.ops) > 0 {
		o.list = append(o.list, op{kind: opCall, call: c.ops})
	}
}

// Tag names an input area's owner: a pointer to its state, as a
// *widget.Clickable.
type Tag any

// Area is where a tag gets input: the pointers that come down in it, and
// the keys when it has the focus.
type Area struct {
	Rect  image.Rectangle
	Tag   Tag // nil: it only stops what's under it (a dialog's backdrop)
	Focus bool
	// Keys are the keys it takes when in focus ("Left", "Enter"), past
	// Enter and Space, which a focusable area always gets; "*": all of
	// them but Tab, and text typed.
	Keys []string
	// Watch: it hears of the pointers that come down in it even under
	// another area, as a list does, to tell a swipe from a tap on a row.
	Watch bool
	// Block: what's under it hears nothing of a pointer that came down
	// in it, watchers neither (a popup's backdrop).
	Block bool
	// Reveal: it scrolls what's laid out in it after it (in the same
	// clip): when the keys move the focus to something in it that
	// doesn't show, it's sent a Reveal.
	Reveal bool
	// Keep: a tap on it leaves the focus where it is (a toolbar's
	// buttons, which act on the text in focus).
	Keep bool
}

// Input adds a.
func Input(o *Ops, a Area) {
	o.list = append(o.list, op{kind: opArea, area: a})
}

// fixedP is x, y as a font.Drawer's dot.
func fixedP(x, y int) fixed.Point26_6 { return fixed.P(x, y) }

// Line draws a line through pts, w thick, gray g (an icon's strokes, a
// check mark).
func Line(o *Ops, w int, g uint8, pts ...image.Point) {
	o.list = append(o.list, op{kind: opLine, pts: pts, t: max(w, 1), gray: g})
}

// Disc fills a circle of radius r about c.
func Disc(o *Ops, c image.Point, r int, g uint8) {
	o.list = append(o.list, op{kind: opDisc, off: c, t: r, gray: g})
}

// Ring draws a circle's edge, w thick, radius r about c.
func Ring(o *Ops, c image.Point, r, w int, g uint8) {
	o.list = append(o.list, op{kind: opDisc, off: c, t: r, gray: g, r: image.Rect(0, 0, w, 0)})
}

// Polygon fills the polygon pts (a triangle, an arrow head).
func Polygon(o *Ops, g uint8, pts ...image.Point) {
	o.list = append(o.list, op{kind: opPoly, pts: pts, gray: g})
}

// Popup puts call over everything else, size big, by anchor (where it
// is now, in the ops' coordinates): under it if there's room in the
// window, else over it, moved in from the window's edges. Its backdrop
// is the whole window, an area of backdrop's (Block): a tap outside the
// popup goes there and nowhere else.
func Popup(o *Ops, anchor image.Rectangle, size image.Point, backdrop Tag, call CallOp) {
	o.list = append(o.list, op{kind: opPopup, r: anchor, sp: size, tag: backdrop, call: call.ops})
}
