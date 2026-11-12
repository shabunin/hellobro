// Package ink is a small immediate-mode UI for e-ink screens, after Gio
// (gioui.org): each frame the program lays its UI out again from its own
// state, as plain function calls, into a list of operations; ink paints
// them in software onto a gray image, compares that with the frame
// before, and hands the host only what changed, with the waveform it
// wants: the fast black-and-white one where only black and white
// changed, a grayscale one elsewhere.
//
// Gio itself can't run here: it wants a GPU, and knows nothing of
// e-ink's refreshes. So this is ours, shaped like it, so its habits
// carry over:
//
//	func (s *page) Layout(gtx ink.Ctx) ink.Dims {
//		if s.ok.Clicked(gtx) { … }
//		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
//			layout.Rigid(theme.Label(th, 20, "Hello").Layout),
//			layout.Rigid(theme.Button(th, &s.ok, "ok").Layout))
//	}
//
// The host (a window of hellobro's, a test) owns a Window: it feeds it
// input (Pointer, Key, Text events), asks for a Frame when it's due, and
// puts the damaged rectangles on the panel. ink knows nothing of the
// Kobo: inkkobo and hellobro's ink window do that.
package ink

import (
	"image"
	"math"
	"time"
)

// Dp is a length in device-independent pixels: a 160th of an inch, as
// Gio's and Android's. A finger wants about 48 of them.
type Dp float32

// Metric turns Dp into pixels.
type Metric struct {
	PxPerDp float32 // the Elipsa, 227 dpi: 1.42
}

// Px is v in whole pixels.
func (m Metric) Px(v Dp) int {
	s := m.PxPerDp
	if s == 0 {
		s = 1
	}
	return int(math.Round(float64(float32(v) * s)))
}

// Constraints are the sizes a widget may take: from Min to Max.
type Constraints struct {
	Min, Max image.Point
}

// Exact is just size.
func Exact(size image.Point) Constraints { return Constraints{size, size} }

// Loose is anything up to size.
func Loose(size image.Point) Constraints { return Constraints{Max: size} }

// Constrain is p within c.
func (c Constraints) Constrain(p image.Point) image.Point {
	p.X = max(c.Min.X, min(p.X, c.Max.X))
	p.Y = max(c.Min.Y, min(p.Y, c.Max.Y))
	return p
}

// Dims are what a widget took: its size, and its first text line's
// baseline, from its foot, so rows of them can line their words up.
type Dims struct {
	Size     image.Point
	Baseline int
}

// Widget lays something out in gtx and says how big it came out.
type Widget func(gtx Ctx) Dims

// Ctx is what a widget is laid out with: its constraints, the frame's
// operations, the time, and the way to its events.
type Ctx struct {
	Constraints
	Metric
	Now time.Time
	Ops *Ops

	w         *Window
	disabled  bool
	measuring bool
}

// Dp is v in pixels.
func (c Ctx) Dp(v Dp) int { return c.Metric.Px(v) }

// Disabled is gtx with input off: widgets laid out in it get no events,
// and show it.
func (c Ctx) Disabled() Ctx {
	c.disabled = true
	return c
}

// IsDisabled says input is off.
func (c Ctx) IsDisabled() bool { return c.disabled }

// Measuring is gtx for laying out only to know the size: no events, no
// areas, and widgets keep their state (a press stays a press).
func (c Ctx) Measuring() Ctx {
	c.disabled, c.measuring = true, true
	return c
}

// IsMeasuring says it's only to know the size.
func (c Ctx) IsMeasuring() bool { return c.measuring }

// Events are tag's events since it last asked, oldest first. A
// disabled Ctx has none.
func (c Ctx) Events(tag Tag) []Event {
	if c.w == nil || c.disabled {
		return nil
	}
	return c.w.events(tag)
}

// Focused says the keys go to tag.
func (c Ctx) Focused(tag Tag) bool { return c.w != nil && c.w.focus == tag && tag != nil }

// FocusedTag is the tag the keys go to, or nil.
func (c Ctx) FocusedTag() Tag {
	if c.w == nil {
		return nil
	}
	return c.w.focus
}

// Focus gives tag the keys (nil: no one).
func (c Ctx) Focus(tag Tag) {
	if c.w != nil {
		c.w.focus = tag
	}
}

// Invalidate asks for another frame at once: what was drawn is out of
// date already.
func (c Ctx) Invalidate() {
	if c.w != nil {
		c.w.again = true
	}
}

// InvalidateAt asks for a frame at t (a long press, a clock): the host
// sees it in Window.Due.
func (c Ctx) InvalidateAt(t time.Time) {
	if c.w != nil {
		c.w.wakeAt(t)
	}
}

// MoveFocus moves the focus step on (back, if negative), as Tab does,
// the ring showing: an editor of one line hands ↑ ↓ on.
func (c Ctx) MoveFocus(step int) {
	if c.w != nil && c.w.moveFocus(step) {
		c.w.ring = true
	}
}

// Pen says this frame is ink being written: the host may use its pen
// waveform, the quickest. It's a Quick frame too.
func (c Ctx) Pen() {
	if c.w != nil {
		c.w.quick, c.w.pen = true, true
	}
}

// Quick asks for this frame's changes in the fast black-and-white
// waveform, grays rounded: a button held down shows at once, and the
// frame that lets it up puts the grays back.
func (c Ctx) Quick() {
	if c.w != nil {
		c.w.quick = true
	}
}
