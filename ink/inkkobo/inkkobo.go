// Package inkkobo runs an ink UI as a Kobo app: a kobo.Program whose
// page is one ink Window. The App keeps what every app has (the header,
// its buttons, rotation, sleep, the light); the page is ink's: the
// touches that come down on it, the keys, and each tick what changed
// goes to the panel, black and white quickly, grays with a grayscale
// update.
package inkkobo

import (
	"image"
	"time"

	"hellobro/ink"
	"hellobro/kobo"
)

// Host is the app.
type Host struct {
	*kobo.App
	Win *ink.Window
	// Quit, if set, is asked after each frame: true ends the app.
	Quit func() bool
	// Idle is how long untouched before the app quits; 0: 10 minutes.
	Idle time.Duration

	page     image.Rectangle
	contacts map[*kobo.Gesture]int
	next     int
	mods     ink.Mods
	caps     bool
	inking   bool // the App's in its pen mode
}

const owner = 1 // a contact on the page: ink's

// New is cfg's app showing ui.
func New(cfg kobo.Config, ui ink.Widget) *Host {
	h := &Host{contacts: map[*kobo.Gesture]int{}}
	h.Win = ink.NewWindow(image.Pt(1, 1), Metric(), ui)
	h.App = kobo.New(cfg, h)
	return h
}

// Metric is ink's Dp on the Elipsa: 1.42 px, 2 with the frame for
// fingers (kobo.SetBig).
func Metric() ink.Metric {
	if kobo.IsBig() {
		return ink.Metric{PxPerDp: 2}
	}
	return ink.Metric{PxPerDp: 1.42}
}

func (h *Host) Layout(L kobo.Layout) {
	h.page = L.Page
	h.Win.Metric = Metric()
	h.Win.Resize(L.Page.Size())
	h.Win.Invalidate()
}

// Draw paints it all, as it is now (the App marks the page).
func (h *Host) Draw() {
	h.Win.Invalidate()
	h.Win.Frame(time.Now())
	kobo.CopyGray(h.C.Gray, h.page, h.Win.Image(), image.Point{})
	h.SetRing(h.ring())
}

func (h *Host) Enter(tab int) {}

func (h *Host) ring() image.Rectangle {
	r := h.Win.Ring()
	if r.Empty() {
		return r
	}
	return r.Add(h.page.Min)
}

// flush lays it out if it's due, and puts what changed on the panel.
func (h *Host) flush() {
	if h.Calibrating() || h.page.Empty() {
		return
	}
	us := h.Win.Frame(time.Now())
	pen := false
	for _, u := range us {
		pen = pen || u.Pen
	}
	if len(us) > 0 && pen != h.inking { // ink written: the pen's waveform, the quickest
		h.inking = pen
		h.Ink(pen)
	}
	for _, u := range us {
		r := u.R.Add(h.page.Min)
		kobo.CopyGray(h.C.Gray, r, h.Win.Image(), u.R.Min)
		h.Mark(r, u.Gray || u.Full)
	}
	h.SetRing(h.ring())
	if h.Quit != nil && h.Quit() {
		h.Quit = nil
		h.App.Quit()
	}
}

func (h *Host) Tick() { h.flush() }

func (h *Host) IdleTimeout() time.Duration {
	if h.Idle > 0 {
		return h.Idle
	}
	return 10 * time.Minute
}

func (h *Host) Exit() {}

// Keys ink knows by name.
var keyNames = map[uint16]string{
	kobo.KeyEsc: "Esc", kobo.KeyBackspace: "Back", kobo.KeyTab: "Tab", kobo.KeyEnter: "Enter", kobo.KeySpace: "Space",
	102: "Home", 103: "Up", 104: "PageUp", 105: "Left", 106: "Right", 107: "End", 108: "Down", 109: "PageDown", 111: "Delete",
}

// Mods by their keys' codes.
var modKeys = map[uint16]ink.Mods{
	kobo.KeyLeftShift: ink.Shift, kobo.KeyRightShift: ink.Shift, kobo.KeyLeftCtrl: ink.Ctrl, kobo.KeyRightCtrl: ink.Ctrl,
	56: ink.Alt, 100: ink.Alt, 125: ink.Super, 126: ink.Super,
}

// Key hands a key to ink; true if it took it (else Esc quits, as in
// every app).
func (h *Host) Key(m kobo.KeyMsg) bool {
	if m.Connected != "" || m.Gone != "" {
		if m.Gone != "" {
			h.mods = 0 // what it held won't come up now
		}
		return true
	}
	if md, ok := modKeys[m.Code]; ok {
		if m.Value == 0 {
			h.mods &^= md
		} else {
			h.mods |= md
		}
		return true
	}
	if m.Value == 0 {
		return true
	}
	if m.Code == kobo.KeyCapsLock {
		h.caps = !h.caps
		return true
	}
	took := false
	if name, ok := keyNames[m.Code]; ok {
		took = h.Win.Key(ink.Key{Name: name, Mods: h.mods, Repeat: m.Value == 2})
	} else if ch, ok := kobo.KeyChar(m.Code, h.mods&ink.Shift != 0, h.caps); ok {
		if h.mods&^ink.Shift == 0 {
			took = h.Win.Text(ch)
		} else {
			took = h.Win.Key(ink.Key{Name: lower(ch), Mods: h.mods})
		}
	}
	h.flush()
	return took
}

func lower(s string) string {
	if len(s) == 1 && s[0] >= 'A' && s[0] <= 'Z' {
		return string(s[0] + 'a' - 'A')
	}
	return s
}

// pointer is ev as ink has it: the pen's pressure (the Elipsa's goes to
// 4095) and its button.
func pointer(k ink.PointerKind, id int, p image.Point, ev kobo.TouchEv) ink.Pointer {
	e := ink.Pointer{Kind: k, ID: id, Pos: p, Source: source(ev), Time: ev.At}
	if ev.Pen {
		e.Pressure = min(float32(ev.Pressure)/4095, 1)
		e.Eraser = ev.Button != kobo.PenNoButton
	}
	return e
}

func source(ev kobo.TouchEv) ink.Source {
	if ev.Pen {
		return ink.Pen
	}
	return ink.Finger
}

// Touch: a contact coming down on the page is ink's, its moves too.
func (h *Host) Touch(ev kobo.TouchEv, p image.Point, g *kobo.Gesture) {
	if g == nil { // the pen in the air
		return
	}
	switch ev.Kind {
	case kobo.TouchDown:
		if !p.In(h.page) {
			return
		}
		g.Owner = owner
		g.NoLight()
		h.next++
		h.contacts[g] = h.next
		h.Win.Pointer(pointer(ink.Press, h.next, p.Sub(h.page.Min), ev))
		h.flush()
	case kobo.TouchMove:
		if id, ok := h.contacts[g]; ok {
			// Laid out at the tick: a frame a move would be too many.
			h.Win.Pointer(pointer(ink.Move, id, p.Sub(h.page.Min), ev))
		}
	}
}

// Tap: the touches came as they were, taps among them.
func (h *Host) Tap(p image.Point) {}

func (h *Host) Lift(g *kobo.Gesture) {
	id, ok := h.contacts[g]
	if !ok {
		return
	}
	delete(h.contacts, g)
	h.Win.Pointer(ink.Pointer{Kind: ink.Release, ID: id, Pos: g.Last.Sub(h.page.Min), Time: time.Now()})
	h.flush()
}
