package widget

import (
	"image"
	"time"

	"hellobro/ink"
	"hellobro/ink/layout"
)

// Choice is a list that opens over everything from a field or a button:
// a dropdown's options, a menu's actions. A tap on its field opens it (or
// Enter, Space in focus); a tap on an item picks it, a tap outside
// closes it; open, ↑ ↓ go through the items, Enter picks, Esc closes.
type Choice struct {
	Popover
	Items []Clickable   // as many as there are items, made as needed
	List  layout.Scroll // the items, moving under the finger if they don't all fit

	hi     int // the item ↑ ↓ got to
	n      int // items, as last laid out
	picked []int
}

// Highlit is the item the keys are on, while it's open.
func (c *Choice) Highlit() int { return c.hi }

// Open opens it, the keys on item at.
func (c *Choice) Open(gtx ink.Ctx, at int) {
	c.hi = max(at, 0)
	c.List.Axis = layout.Vertical
	c.List.ScrollTo(c.hi)
	c.Popover.Open(gtx)
}

// Picked is the item picked since the last frame, if one was: a menu's
// action. n is how many items it has.
func (c *Choice) Picked(gtx ink.Ctx, n int) (int, bool) {
	c.Update(gtx, n, -1)
	if len(c.picked) == 0 {
		return 0, false
	}
	i := c.picked[0]
	c.picked = c.picked[1:]
	return i, true
}

// Update takes the events since the last frame: n items, current the
// one it shows now (where the keys start), -1: none. Picks wait for
// Picked; the theme's Layout calls it, the program may before.
func (c *Choice) Update(gtx ink.Ctx, n, current int) {
	c.n = n
	for len(c.Items) < n {
		c.Items = append(c.Items, Clickable{NoFocus: true})
	}
	if c.toggled(gtx) {
		c.Open(gtx, current)
	}
	for _, k := range c.keys(gtx) {
		switch k.Name {
		case "Up":
			c.hi = (c.hi - 1 + n) % max(n, 1)
			c.List.ScrollTo(c.hi)
		case "Down", "Tab":
			c.hi = (c.hi + 1) % max(n, 1)
			c.List.ScrollTo(c.hi)
		case "PageDown":
			c.List.ScrollBy(1)
		case "PageUp":
			c.List.ScrollBy(-1)
		case "Home":
			c.hi = 0
			c.List.ScrollTo(0)
		case "End":
			c.hi = max(n-1, 0)
			c.List.ScrollTo(c.hi)
		case "Enter", "Space":
			if c.hi < n {
				c.picked = append(c.picked, c.hi)
			}
			c.Close(gtx)
		}
	}
	for i := 0; i < n; i++ {
		for c.Items[i].Clicked(gtx) {
			c.picked = append(c.picked, i)
			c.hi = i
			c.Close(gtx)
		}
	}
}

// Select is a dropdown: a Choice whose pick is its Value.
type Select struct {
	Choice
	Value   int
	changed bool
}

// Update takes the events since the last frame, n options: a pick is
// the Value then.
func (s *Select) Update(gtx ink.Ctx, n int) {
	s.Choice.Update(gtx, n, s.Value)
	for _, i := range s.picked {
		if i != s.Value {
			s.Value, s.changed = i, true
		}
	}
	s.picked = s.picked[:0]
}

// Changed says another value was picked since it was last asked.
func (s *Select) Changed(gtx ink.Ctx, n int) bool {
	s.Update(gtx, n)
	c := s.changed
	s.changed = false
	return c
}

// Float is a value along a line, Min to Max, set where a finger comes
// down on it and as it moves (or ← → a Step at a time in focus).
type Float struct {
	Value, Min, Max, Step float32

	Clickable
	dragging bool
	width    int
	changed  bool
}

// Dragging says a finger's on it.
func (f *Float) Dragging() bool { return f.dragging }

// Changed says the value changed since the last frame.
func (f *Float) Changed(gtx ink.Ctx) bool {
	f.update(gtx)
	c := f.changed
	f.changed = false
	return c
}

// Frac is the value as a part of the way, 0 to 1.
func (f *Float) Frac() float32 {
	if f.Max <= f.Min {
		return 0
	}
	return (f.Value - f.Min) / (f.Max - f.Min)
}

func (f *Float) set(v float32) {
	if f.Step > 0 {
		v = f.Min + float32(int((v-f.Min)/f.Step+0.5))*f.Step
	}
	v = min(max(v, f.Min), f.Max)
	if v != f.Value {
		f.Value, f.changed = v, true
	}
}

func (f *Float) update(gtx ink.Ctx) {
	f.Keys = floatKeys
	for _, e := range gtx.Events(&f.Clickable) {
		switch e := e.(type) {
		case ink.Pointer:
			switch e.Kind {
			case ink.Press:
				f.dragging = true
				fallthrough
			case ink.Move:
				if f.dragging && f.width > 0 {
					f.set(f.Min + (f.Max-f.Min)*float32(e.Pos.X)/float32(f.width))
				}
			case ink.Release, ink.Cancel:
				f.dragging = false
			}
		case ink.Key:
			step := f.Step
			if step <= 0 {
				step = (f.Max - f.Min) / 10
			}
			switch e.Name {
			case "Left":
				f.set(f.Value - step)
			case "Right":
				f.set(f.Value + step)
			case "Home":
				f.set(f.Min)
			case "End":
				f.set(f.Max)
			}
		case ink.Focus:
			f.focused = e.On
		}
	}
}

// floatKeys are a slider's: ← → along it; ↑ ↓ are left to the focus,
// to the row above or below, as a slider is a row in a form.
var floatKeys = []string{"Left", "Right", "Home", "End"}

// Layout lays w (the slider as drawn) out as its area: a touch at x is
// the value x along its width.
func (f *Float) Layout(gtx ink.Ctx, w ink.Widget) ink.Dims {
	f.update(gtx)
	if f.dragging {
		gtx.Quick() // following the finger: black and white, at once
	}
	m := ink.Record(gtx.Ops)
	d := w(gtx)
	call := m.Stop()
	f.width = d.Size.X
	call.Add(gtx.Ops)
	if !gtx.IsDisabled() {
		ink.Input(gtx.Ops, ink.Area{Rect: image.Rectangle{Max: d.Size}, Tag: &f.Clickable, Focus: true, Keys: f.Keys})
	}
	f.focused = gtx.Focused(&f.Clickable)
	return d
}

// Toast is a word shown for a moment, then gone by itself.
type Toast struct {
	Text  string
	until time.Time
}

// ToastFor is how long a toast shows.
const ToastFor = 3 * time.Second

// Show shows s for a while.
func (t *Toast) Show(gtx ink.Ctx, s string) {
	t.Text, t.until = s, gtx.Now.Add(ToastFor)
	gtx.InvalidateAt(t.until)
	gtx.Invalidate()
}

// Showing says it's up, and asks for the frame that takes it down.
func (t *Toast) Showing(gtx ink.Ctx) bool {
	if t.Text == "" || !gtx.Now.Before(t.until) {
		return false
	}
	gtx.InvalidateAt(t.until)
	return true
}
