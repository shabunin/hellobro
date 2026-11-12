// Package widget is the state of ink's controls, and what input does to
// it: a thing to press (Clickable), a switch (Bool), a choice of one
// (Enum), a dropdown or a menu (Choice, Select), a value along a line
// (Float), a line of text (Editor), a word for a moment (Toast). How
// they look is the theme's.
package widget

import (
	"image"
	"time"

	"hellobro/ink"
)

// LongPress is how long a press is held to be a long one.
const LongPress = 600 * time.Millisecond

// slop is how far a press may wander and still be one, in Dp.
const slop ink.Dp = 24

// Clickable is a thing to press: a tap on it, or Enter or Space while it
// has the focus, is a click; held, a long press.
type Clickable struct {
	// Keys are more keys it takes in focus, past Enter and Space: they
	// come out of TakeKeys.
	Keys []string
	// NoFocus: Tab passes it by (a popup's rows: the popup takes the
	// keys).
	NoFocus bool
	// KeepFocus: a tap on it leaves the focus where it was, and Tab
	// passes it by (a text's toolbar).
	KeepFocus bool

	clicks, longs int
	typed         []ink.Key
	pressed       bool // a contact's down on it, and hasn't wandered off
	contact       int
	at            time.Time
	from          image.Point
	longDone      bool
	focused       bool
	size          image.Point
}

// Click is a click as Update tells of it.
type Click struct {
	Long bool // held
}

// Update takes the events since the last frame: true for each click,
// one at a time (call it till it's false, or use Clicked).
func (b *Clickable) Update(gtx ink.Ctx) (Click, bool) {
	b.events(gtx)
	switch {
	case b.longs > 0:
		b.longs--
		return Click{Long: true}, true
	case b.clicks > 0:
		b.clicks--
		return Click{}, true
	}
	return Click{}, false
}

// Clicked says it was tapped (or Entered) since the last frame.
func (b *Clickable) Clicked(gtx ink.Ctx) bool {
	b.events(gtx)
	if b.clicks > 0 {
		b.clicks--
		return true
	}
	return false
}

// LongPressed says it was held since the last frame.
func (b *Clickable) LongPressed(gtx ink.Ctx) bool {
	b.events(gtx)
	if b.longs > 0 {
		b.longs--
		return true
	}
	return false
}

// Pressed says a contact's down on it.
func (b *Clickable) Pressed() bool { return b.pressed }

// Focused says it has the keys.
func (b *Clickable) Focused() bool { return b.focused }

// TakeKeys are its Keys pressed since it last took them.
func (b *Clickable) TakeKeys(gtx ink.Ctx) []ink.Key {
	b.events(gtx)
	k := b.typed
	b.typed = nil
	return k
}

// Click clicks it, as a tap would.
func (b *Clickable) Click() { b.clicks++ }

func (b *Clickable) events(gtx ink.Ctx) {
	s := gtx.Dp(slop)
	for _, e := range gtx.Events(b) {
		switch e := e.(type) {
		case ink.Pointer:
			switch e.Kind {
			case ink.Press:
				if !b.pressed {
					gtx.Quick() // held down: inverted at once
					b.pressed, b.contact, b.at, b.from, b.longDone = true, e.ID, e.Time, e.Pos, false
					if e.Time.IsZero() {
						b.at = gtx.Now
					}
				}
			case ink.Move:
				if b.pressed && e.ID == b.contact {
					d := e.Pos.Sub(b.from)
					if d.X*d.X+d.Y*d.Y > s*s {
						b.pressed = false // it's a swipe
					}
				}
			case ink.Release:
				if b.pressed && e.ID == b.contact {
					b.pressed = false
					if !b.longDone && e.Pos.In(image.Rectangle{Max: b.size}.Inset(-s)) {
						b.clicks++
					}
				}
			case ink.Cancel:
				if e.ID == b.contact {
					b.pressed = false
				}
			}
		case ink.Key:
			if e.Name == "Enter" || e.Name == "Space" {
				b.clicks++
			} else {
				b.typed = append(b.typed, e)
			}
		case ink.Focus:
			b.focused = e.On
		}
	}
	if b.pressed && !b.longDone {
		if gtx.Now.Sub(b.at) >= LongPress {
			b.longDone, b.pressed = true, false
			b.longs++
		} else {
			gtx.InvalidateAt(b.at.Add(LongPress))
		}
	}
}

// Layout lays w out and makes it pressable: its area, focusable.
func (b *Clickable) Layout(gtx ink.Ctx, w ink.Widget) ink.Dims {
	b.events(gtx)
	m := ink.Record(gtx.Ops)
	d := w(gtx)
	call := m.Stop()
	b.size = d.Size
	call.Add(gtx.Ops)
	if !gtx.IsDisabled() {
		ink.Input(gtx.Ops, ink.Area{Rect: image.Rectangle{Max: d.Size}, Tag: b, Focus: !b.NoFocus && !b.KeepFocus, Keys: b.Keys, Keep: b.KeepFocus})
	} else if !gtx.IsMeasuring() {
		b.pressed = false
	}
	b.focused = gtx.Focused(b)
	return d
}

// Bool is a switch: a click flips it.
type Bool struct {
	Value bool
	Clickable
}

// Update says it changed since the last frame.
func (b *Bool) Update(gtx ink.Ctx) bool {
	changed := false
	for b.Clicked(gtx) {
		b.Value = !b.Value
		changed = true
	}
	return changed
}

// Layout lays w out as its switch.
func (b *Bool) Layout(gtx ink.Ctx, w ink.Widget) ink.Dims {
	b.Update(gtx)
	return b.Clickable.Layout(gtx, w)
}

// Enum is a choice of one, by key: each choice is a Clickable of its own.
type Enum struct {
	Value   string
	choices map[string]*Clickable
	changed bool
}

func (e *Enum) of(key string) *Clickable {
	if e.choices == nil {
		e.choices = map[string]*Clickable{}
	}
	c := e.choices[key]
	if c == nil {
		c = &Clickable{}
		e.choices[key] = c
	}
	return c
}

// Update says the choice changed since the last frame.
func (e *Enum) Update(gtx ink.Ctx) bool {
	for k, c := range e.choices {
		for c.Clicked(gtx) {
			if e.Value != k {
				e.Value, e.changed = k, true
			}
		}
	}
	ch := e.changed
	e.changed = false
	return ch
}

// Pressed says key's choice is held down.
func (e *Enum) Pressed(key string) bool { return e.of(key).Pressed() }

// Layout lays w out as key's choice.
func (e *Enum) Layout(gtx ink.Ctx, key string, w ink.Widget) ink.Dims {
	c := e.of(key)
	for c.Clicked(gtx) {
		if e.Value != key {
			e.Value, e.changed = key, true
		}
	}
	return c.Layout(gtx, w)
}
