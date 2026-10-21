package kobo

// Dozing: a light sleep for listening. The light goes off and the panel
// holds still on a screen the Program draws for it, but nothing is
// suspended: the music plays on, and the media keys (headphones' buttons)
// still reach the Program. A tap, or any other key, wakes it up. The
// Program goes on drawing its page meanwhile, onto a canvas off the
// panel, so waking shows it as it is by then.

import (
	"image"
	"log"
	"time"
)

// Dozer is a Program that can doze.
type Dozer interface {
	// DozeInstead says whether Sleep should doze instead (music playing).
	DozeInstead() bool
	// DrawDoze draws the screen shown while dozing, all of a.C.
	DrawDoze()
	// Dozed is called as dozing begins, before DrawDoze, and once it's
	// over, the page back on the panel.
	Dozed(on bool)
}

// Dozing says whether the app is dozing.
func (a *App) Dozing() bool { return a.dozing }

// IdleFor is how long since the last touch or key.
func (a *App) IdleFor() time.Duration { return time.Since(a.inputAt) }

// Doze starts dozing, if the Program can.
func (a *App) Doze() {
	d, ok := a.prog.(Dozer)
	if !ok || a.dozing || a.asleep || a.calib != nil {
		return
	}
	log.Print("doze: dozing")
	a.HideGauge()
	a.lightOverlay = nil
	a.WaitBlit()
	a.dirty, a.flashR = nil, image.Rectangle{} // of the page: shown again on waking
	a.dozeLight = -1
	if a.light != nil {
		a.dozeLight = a.light.level
		a.light.set(0)
	}
	// The page goes on in a copy; the panel's canvas gets the doze screen.
	a.panelC = a.C
	a.C = Canvas{image.NewGray(a.panelC.Bounds())}
	copy(a.C.Pix, a.panelC.Pix)
	a.dozing = true
	d.Dozed(true)
	a.drawDoze(true)
}

// Redoze draws the doze screen again: what it shows changed.
func (a *App) Redoze() {
	if a.dozing {
		a.drawDoze(false)
	}
}

func (a *App) drawDoze(flash bool) {
	page := a.C
	a.C = a.panelC
	a.prog.(Dozer).DrawDoze()
	a.C = page
	a.markAll(flash)
	a.flush()
}

// Undoze wakes up from dozing: the page as it is now, the light back.
func (a *App) Undoze() {
	if !a.dozing {
		return
	}
	log.Print("doze: awake")
	a.endDoze()
	if a.light != nil && a.dozeLight > 0 {
		a.light.set(a.dozeLight)
	}
	a.inputAt = time.Now()
	a.markAll(true)
	a.flush()
	a.prog.(Dozer).Dozed(false)
}

// endDoze puts the page's canvas back on the panel.
func (a *App) endDoze() {
	a.WaitBlit()
	a.dozing = false
	a.panelC = Canvas{}
	a.dirty, a.flashR = nil, image.Rectangle{}
}

// dozeTouch takes the touch screen while dozing: it's nobody's, so a hand
// on the screen doesn't wake it; a key or the power button does. With
// neither, a finger down wakes it, and the rest of that contact is
// nobody's.
func (a *App) dozeTouch(ev TouchEv) {
	switch {
	case ev.Kind == TouchDown && a.dozing:
		if a.noPower && a.Keyboard == "" {
			a.eatTouch = true
			a.Undoze()
		}
	case ev.Kind == TouchUp:
		a.eatTouch = false
	}
}
