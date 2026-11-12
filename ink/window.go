package ink

import (
	"image"
	"slices"
	"time"
)

// Window is a UI the host shows: it lays it out when it's due, keeps
// what it painted, and routes the input to the areas the last frame had.
type Window struct {
	Metric Metric
	UI     Widget

	size        image.Point
	front, back *image.Gray // what the host has; the frame being painted
	ops         Ops
	areas       []placed
	texts       []textBox

	queues   map[Tag][]Event
	asked    map[Tag]bool        // tags that asked for their events this frame
	contacts map[int][]contactTo // the contacts down, and who hears of them
	focus    Tag                 // the keys go to it
	ring     bool                // the focus shows: the keys moved it, no touch since
	again    bool                // another frame, at once
	quick    bool                // this frame's changes in the fast waveform
	wake     time.Time           // and one then
	input    bool                // input since the last frame
	updates  []Update            // painted, not yet taken
	painted  bool                // a frame's been painted at this size
	lastPos  map[int]image.Point // the contacts' last places, in the window
	byTag    map[Tag]image.Point // each area's origin, as last painted
}

type contactTo struct {
	tag Tag
	off image.Point // where its area was when the contact came down
}

// Update is a part of the window that changed: Gray if it has more
// than black and white in it (a grayscale waveform), else the fast one
// will do (or a widget asked for it: Quick). Full is all of it, new.
type Update struct {
	R    image.Rectangle
	Gray bool
	Full bool
}

// NewWindow is a window size big, for ui.
func NewWindow(size image.Point, m Metric, ui Widget) *Window {
	w := &Window{Metric: m, UI: ui, queues: map[Tag][]Event{}, asked: map[Tag]bool{}, contacts: map[int][]contactTo{}, lastPos: map[int]image.Point{}}
	w.Resize(size)
	return w
}

// Resize makes it size big: all of it's painted anew.
func (w *Window) Resize(size image.Point) {
	if size == w.size && w.front != nil {
		return
	}
	w.size = size
	w.front = image.NewGray(image.Rectangle{Max: size})
	w.back = image.NewGray(image.Rectangle{Max: size})
	w.painted = false
	w.input = true
}

// Size is its size.
func (w *Window) Size() image.Point { return w.size }

// Image is what it shows, as the last frame left it.
func (w *Window) Image() *image.Gray { return w.front }

// Invalidate asks for a frame: the program's state changed.
func (w *Window) Invalidate() { w.input = true }

// Due says a frame is: input came, it was asked for, or a time asked for
// came.
func (w *Window) Due(now time.Time) bool {
	return w.input || !w.painted || !w.wake.IsZero() && !now.Before(w.wake)
}

// Wake is when it next wants a frame by itself, if ever.
func (w *Window) Wake() time.Time { return w.wake }

func (w *Window) wakeAt(t time.Time) {
	if w.wake.IsZero() || t.Before(w.wake) {
		w.wake = t
	}
}

// Frame lays it out and paints it if it's due, and gives what changed
// since the host last took it.
func (w *Window) Frame(now time.Time) []Update {
	if w.Due(now) {
		w.frame(now)
	}
	u := w.updates
	w.updates = nil
	return u
}

func (w *Window) frame(now time.Time) {
	w.input = false
	if !w.wake.IsZero() && !now.Before(w.wake) {
		w.wake = time.Time{}
	}
	clear(w.asked)
	w.quick = false
	for pass := 0; ; pass++ {
		w.again = false
		w.ops.Reset()
		gtx := Ctx{Constraints: Exact(w.size), Metric: w.Metric, Now: now, Ops: &w.ops, w: w}
		if w.UI != nil {
			w.UI(gtx)
		}
		if !w.again || pass == 2 {
			break
		}
	}
	fill(w.back, w.back.Rect, White)
	p := painter{dst: w.back}
	p.run(w.ops.list, image.Point{}, w.back.Rect)
	w.areas, w.texts = p.areas, p.texts
	w.byTag = map[Tag]image.Point{}
	for _, a := range w.areas {
		if a.Tag != nil {
			w.byTag[a.Tag] = a.off
		}
	}
	// What no one asked for is stale: its widget's gone.
	for t := range w.queues {
		if !w.asked[t] {
			if _, ok := w.byTag[t]; !ok || t == Root {
				delete(w.queues, t)
			}
		}
	}
	if w.focus != nil && w.focus != Root && !w.focusable(w.focus) {
		w.focus = nil
	}
	if !w.painted {
		w.updates = []Update{{R: w.back.Rect, Gray: true, Full: true}}
		w.painted = true
	} else {
		d := diff(w.front, w.back)
		if w.quick {
			for i := range d {
				d[i].Gray = false
			}
		}
		w.updates = mergeUpdates(append(w.updates, d...))
	}
	w.front, w.back = w.back, w.front
}

func (w *Window) focusable(t Tag) bool {
	for _, a := range w.areas {
		if a.Tag == t && a.Focus {
			return true
		}
	}
	return false
}

func (w *Window) events(t Tag) []Event {
	w.asked[t] = true
	evs := w.queues[t]
	delete(w.queues, t)
	return evs
}

func (w *Window) send(t Tag, e Event) {
	if t != nil {
		w.queues[t] = append(w.queues[t], e)
		w.input = true
	}
}

// Pointer takes a contact's event, at its place in the window.
func (w *Window) Pointer(p Pointer) {
	switch p.Kind {
	case Press:
		w.ring = false
		var to []contactTo
		var target Tag
		hit := false
		for i := len(w.areas) - 1; i >= 0; i-- {
			a := w.areas[i]
			if !p.Pos.In(a.r) {
				continue
			}
			if a.Watch {
				to = append(to, contactTo{a.Tag, a.off})
				continue
			}
			if hit {
				continue // under the one it hit: only watchers hear of it
			}
			hit = true
			if a.Tag == nil {
				break // a backdrop: nothing under it hears
			}
			target = a.Tag
			to = append([]contactTo{{a.Tag, a.off}}, to...)
			if a.Focus {
				w.setFocus(a.Tag)
			}
		}
		if target == nil || !w.focusable(target) {
			w.setFocus(nil)
		}
		w.contacts[p.ID] = to
	}
	to := w.contacts[p.ID]
	for _, c := range to {
		q := p
		off := c.off
		if o, ok := w.byTag[c.tag]; ok {
			off = o
		}
		q.Pos = p.Pos.Sub(off)
		w.send(c.tag, q)
	}
	w.lastPos[p.ID] = p.Pos
	if p.Kind == Release || p.Kind == Cancel {
		delete(w.contacts, p.ID)
		delete(w.lastPos, p.ID)
	}
	w.input = true
}

// Hand gives contact id to tag alone: the others that heard of it get a
// Cancel (a list took a swipe that began on a row's button).
func (c Ctx) Hand(id int, tag Tag) {
	w := c.w
	if w == nil {
		return
	}
	var kept []contactTo
	for _, to := range w.contacts[id] {
		if to.tag == tag {
			kept = append(kept, to)
			continue
		}
		w.send(to.tag, Pointer{Kind: Cancel, ID: id, Time: c.Now})
	}
	w.contacts[id] = kept
}

func (w *Window) setFocus(t Tag) {
	if t == w.focus {
		return
	}
	if w.focus != nil {
		w.send(w.focus, Focus{false})
	}
	w.focus = t
	if t != nil {
		w.send(t, Focus{true})
	}
	w.input = true
}

// takes says the area in focus takes key k.
func (a placed) takes(k Key) bool {
	if k.Name == "Tab" {
		return false
	}
	if k.Name == "Enter" || k.Name == "Space" {
		return true
	}
	for _, n := range a.Keys {
		if n == "*" || n == k.Name {
			return true
		}
	}
	return false
}

func (w *Window) focused() (placed, bool) {
	for _, a := range w.areas {
		if a.Tag == w.focus && a.Focus && w.focus != nil {
			return a, true
		}
	}
	return placed{}, false
}

// Key takes a key, and says if anyone took it: Tab and the arrows move
// the focus, the area in focus takes its keys, the rest go to Root,
// which the program may or may not have asked for (a frame is laid out
// to see).
func (w *Window) Key(k Key) bool {
	f, ok := w.focused()
	if ok && f.takes(k) {
		w.ring = true
		w.send(f.Tag, k)
		return true
	}
	step := 0
	switch k.Name {
	case "Tab":
		step = 1
		if k.Mods&Shift != 0 {
			step = -1
		}
	case "Down", "Right":
		step = 1
	case "Up", "Left":
		step = -1
	}
	if step != 0 && k.Mods&^Shift == 0 && w.moveFocus(step) {
		w.ring = true
		w.input = true
		return true
	}
	return w.toRoot(k)
}

// Text takes text typed: the area in focus's, if it takes all keys,
// else Root's.
func (w *Window) Text(s string) bool {
	f, ok := w.focused()
	if ok && slices.Contains(f.Keys, "*") {
		w.send(f.Tag, Text{s})
		return true
	}
	return w.toRoot(Text{s})
}

func (w *Window) toRoot(e Event) bool {
	w.send(Root, e)
	w.frame(time.Now())
	return w.asked[Root]
}

// moveFocus moves it step on (or back) through the focusable areas, in
// the order they were laid out.
func (w *Window) moveFocus(step int) bool {
	var fs []Tag
	for _, a := range w.areas {
		if a.Focus && a.Tag != nil && !slices.Contains(fs, a.Tag) {
			fs = append(fs, a.Tag)
		}
	}
	if len(fs) == 0 {
		return false
	}
	i := slices.Index(fs, w.focus)
	switch {
	case i < 0 && step > 0:
		i = 0
	case i < 0:
		i = len(fs) - 1
	default:
		i = (i + step + len(fs)) % len(fs)
	}
	w.setFocus(fs[i])
	return true
}

// RingOut is how far out of its control the focus ring's outer edge
// goes.
const RingOut = 7

// Ring is where the focus ring goes, its outer edge, if it shows: the
// host stamps it on what it sends the panel, it's never painted into the
// frame.
func (w *Window) Ring() image.Rectangle {
	if !w.ring {
		return image.Rectangle{}
	}
	if f, ok := w.focused(); ok {
		return f.r.Inset(-RingOut).Intersect(w.front.Rect)
	}
	return image.Rectangle{}
}

// ShowRing shows the focus where it is, or hides it.
func (w *Window) ShowRing(on bool) { w.ring = on }

// FindText is where the last frame painted s (the first place), for
// tests and the harness's taps.
func (w *Window) FindText(s string) (image.Rectangle, bool) {
	for _, t := range w.texts {
		if t.s == s {
			return t.r, true
		}
	}
	return image.Rectangle{}, false
}

// Texts are what the last frame painted, in order.
func (w *Window) Texts() []string {
	var ss []string
	for _, t := range w.texts {
		ss = append(ss, t.s)
	}
	return ss
}
