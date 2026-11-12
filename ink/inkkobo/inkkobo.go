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
	*Pane
	// Quit, if set, is asked after each frame: true ends the app.
	Quit func() bool
	// Idle is how long untouched before the app quits; 0: 10 minutes.
	Idle time.Duration

	mods   ink.Mods
	caps   bool
	inking bool // the App's in its pen mode
}

const owner = 1 // a contact on the page: ink's

// New is cfg's app showing ui.
func New(cfg kobo.Config, ui ink.Widget) *Host {
	h := &Host{Pane: NewPane(ui)}
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

func (h *Host) Layout(L kobo.Layout) { h.Place(L.Page) }

// Draw paints it all, as it is now (the App marks the page).
func (h *Host) Draw() {
	h.Pane.Draw(h.C.Gray)
	h.SetRing(h.Ring())
}

func (h *Host) Enter(tab int) {}

// flush lays it out if it's due, and puts what changed on the panel.
func (h *Host) flush() {
	if h.Calibrating() || h.At.Empty() {
		return
	}
	if pen := h.Flush(h.App); pen != h.inking { // ink written: the pen's waveform, the quickest
		h.inking = pen
		h.Ink(pen)
	}
	h.SetRing(h.Ring())
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

// Key hands a key to ink; true if it took it (else Esc quits, as in
// every app).
func (h *Host) Key(m kobo.KeyMsg) bool {
	if m.Connected != "" || m.Gone != "" {
		if m.Gone != "" {
			h.mods = 0 // what it held won't come up now
		}
		return true
	}
	if md, ok := ModOf(m.Code); ok {
		if m.Value == 0 {
			h.mods &^= md
		} else {
			h.mods |= md
		}
		return true
	}
	if m.Code == kobo.KeyCapsLock {
		if m.Value == 1 {
			h.caps = !h.caps
		}
		return true
	}
	took := h.Pane.Key(m.Code, m.Value, h.mods, h.caps)
	h.flush()
	return took
}

// Touch: a contact coming down on the page is ink's, its moves too.
func (h *Host) Touch(ev kobo.TouchEv, p image.Point, g *kobo.Gesture) {
	switch ev.Kind {
	case kobo.TouchDown:
		if h.Down(ev, p, g, owner) {
			h.flush()
		}
	case kobo.TouchMove:
		h.Move(ev, p, g)
	}
}

// Tap: the touches came as they were, taps among them.
func (h *Host) Tap(p image.Point) {}

func (h *Host) Lift(g *kobo.Gesture) {
	if h.Pane.Lift(g) {
		h.flush()
	}
}
