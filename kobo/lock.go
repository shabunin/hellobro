package kobo

// Locked: the touch screen does nothing, and of the keys only those the
// Program lets through (its unlock key, the music's), so the Kobo can go
// in a bag or a hand with the screen on. None of it counts as use: the
// light stays off, and sleeping when idle still comes. The panel shows
// the lock screen meanwhile (the title, what the Program says, how to
// unlock), and the page goes on in a.C off it, so unlocking shows it as
// it is by then.

import (
	"image"
	"log"
	"slices"
)

// Lock locks: hint says how to unlock, lines what else the lock screen
// shows (the music, say).
func (a *App) Lock(hint string, lines []string) {
	if a.locked {
		return
	}
	log.Print("lock: locked")
	a.HideGauge()
	a.lightOverlay = nil
	a.WaitBlit()
	a.locked, a.lockHint, a.lockLines = true, hint, lines
	a.drawLock()
}

// Unlock unlocks: the page back on the panel.
func (a *App) Unlock() {
	if !a.locked {
		return
	}
	log.Print("lock: unlocked")
	a.WaitBlit()
	a.locked, a.lockC = false, Canvas{}
	a.markAll(true)
	a.flush()
}

// Locked says whether the app is locked.
func (a *App) Locked() bool { return a.locked }

// SetLockLines changes what the lock screen shows, if it's up.
func (a *App) SetLockLines(lines []string) {
	if !a.locked || slices.Equal(lines, a.lockLines) {
		return
	}
	a.lockLines = lines
	a.drawLock()
}

// drawLock draws the lock screen and puts it on the panel.
func (a *App) drawLock() {
	if a.lockC.Gray == nil || a.lockC.Bounds() != a.C.Bounds() { // first, or turned meanwhile
		a.lockC = Canvas{image.NewGray(a.C.Bounds())}
	}
	c := a.lockC
	b := c.Bounds()
	c.Fill(b, White)
	cy := b.Dy() / 2
	c.Text(a.F.Huge, image.Rect(0, cy-260, b.Dx(), cy-120), a.cfg.Title, Black, AlignCenter)
	c.Fill(image.Rect(b.Dx()/2-160, cy-90, b.Dx()/2+160, cy-86), Black)
	c.Text(a.F.Title, image.Rect(0, cy-60, b.Dx(), cy+10), "locked", Black, AlignCenter)
	y := cy + 50
	for i, l := range a.lockLines {
		f, col := a.F.Body, uint8(Black)
		if i > 0 {
			f, col = a.F.Small, Dark
		}
		c.Text(f, image.Rect(Pad*4, y, b.Dx()-Pad*4, y+50), l, col, AlignCenter)
		y += 56
	}
	c.Text(a.F.Small, image.Rect(0, b.Dy()-140, b.Dx(), b.Dy()-96), a.lockHint, Mid, AlignCenter)
	a.blinks, a.toast = nil, nil
	a.dirty = []dirtyRect{{b, true}}
	a.flush()
}

// panel is the canvas on the panel: the lock screen while locked.
func (a *App) panel() Canvas {
	if a.locked {
		if a.lockC.Bounds() != a.C.Bounds() {
			a.drawLock()
		}
		return a.lockC
	}
	return a.C
}

// lockTouch takes the touch screen while locked, and a contact that began
// then till it's over, so its moves don't reach a gesture that never saw
// it start.
func (a *App) lockTouch(ev TouchEv) {
	a.eatTouch = ev.Kind != TouchUp
}
