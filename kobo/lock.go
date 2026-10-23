package kobo

// Locked: the touch screen does nothing, and of the keys only those the
// Program lets through (its unlock key, the music's), so the Kobo can go
// in a bag or a hand with the screen on. The light goes off till it's
// unlocked, and none of it counts as use: sleeping when idle still
// comes. The panel shows
// the lock screen meanwhile (the title, what the Program says, how to
// unlock), and the page goes on in a.C off it, so unlocking shows it as
// it is by then.

import (
	"image"
	"log"
	"slices"
	"strings"
	"time"
)

// Lock locks: hint says how to unlock, lines what else the lock screen
// shows (the music, say).
func (a *App) Lock(hint string, lines []string) {
	if a.locked {
		return
	}
	a.HideGauge()
	a.lightOverlay = nil
	a.WaitBlit()
	a.locked, a.lockHint, a.lockLines, a.lockSlot = true, hint, lines, -1
	a.lockLight = 0
	if a.light != nil && a.light.level > 0 {
		if a.lightOffAt.IsZero() { // not a notice's flash: the reader's
			a.lockLight = a.light.level
		}
		a.lightOffAt = time.Time{}
		a.light.set(0)
	}
	log.Printf("lock: locked, the light off (was %d%%)", a.lockLight)
	a.drawLock()
}

// Unlock unlocks: the page back on the panel.
func (a *App) Unlock() {
	if !a.locked {
		return
	}
	log.Printf("lock: unlocked, the light back to %d%%", a.lockLight)
	a.WaitBlit()
	a.locked, a.lockC, a.lockSwipes = false, Canvas{}, nil
	if a.light != nil && a.lockLight > 0 && a.light.level == 0 {
		a.light.set(a.lockLight)
	}
	a.markAll(true)
	a.flush()
}

// The ways a lock screen's swipe goes, as the screen is held.
const (
	SwipeUp = iota
	SwipeRight
	SwipeDown
	SwipeLeft
)

// swipeArrows are how the lock screen shows them.
var swipeArrows = [...]string{"↑", "→", "↓", "←"}

// LockSwipe is a swipe on the lock screen: the touch screen's way out,
// when the keyboard that would unlock it is gone.
type LockSwipe struct {
	Dir   int // SwipeUp...
	Label string
	Run   func()
}

// SetLockSwipes gives the lock screen swipes, or none, if it's up.
func (a *App) SetLockSwipes(ss []LockSwipe) {
	if !a.locked {
		return
	}
	a.lockSwipes, a.lockSlot = ss, -1
	a.drawLock()
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
	c.Text(a.F.Huge, image.Rect(0, cy-200, b.Dx(), cy-60), a.cfg.Title, Black, AlignCenter)
	y := cy
	for i, l := range a.lockLines {
		f, col := a.F.Body, uint8(Black)
		if i > 0 {
			f, col = a.F.Small, Dark
		}
		c.Text(f, image.Rect(Pad*4, y, b.Dx()-Pad*4, y+50), l, col, AlignCenter)
		y += 56
	}
	// The swipes, in a row under it all.
	if len(a.lockSwipes) > 0 {
		ws := make([]string, len(a.lockSwipes))
		for i, sw := range a.lockSwipes {
			ws[i] = swipeArrows[sw.Dir&3] + " " + sw.Label
		}
		y = max(y+40, cy+170)
		c.Text(a.F.Body, image.Rect(Pad*2, y, b.Dx()-Pad*2, y+50), "swipe "+strings.Join(ws, " · "), Black, AlignCenter)
	}
	y = b.Dy() - 90 - 48*len(a.cfg.About) // at the foot, as the start's card: the version, where it's from
	c.Text(a.F.Body, image.Rect(0, y-80, b.Dx(), y-30), a.lockHint, Dark, AlignCenter)
	for _, l := range a.cfg.About {
		c.Text(a.F.Small, image.Rect(0, y, b.Dx(), y+44), l, Mid, AlignCenter)
		y += 48
	}
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
// it start. A swipe, if the lock screen has any, goes when the finger
// lifts: a long one (a quarter of the screen's short side), mostly one
// way, so a brush in a bag isn't one.
func (a *App) lockTouch(ev TouchEv) {
	p := a.tmap.toView(ev.x, ev.y)
	switch {
	case ev.Kind == TouchDown && a.lockSlot < 0:
		a.lockSlot, a.lockFrom, a.lockTo = ev.slot, p, p
	case ev.Kind == TouchMove && ev.slot == a.lockSlot:
		a.lockTo = p
	case ev.Kind == TouchUp && ev.slot == a.lockSlot:
		a.lockSlot = -1
		if !a.locked {
			break
		}
		b := a.C.Bounds()
		d, ok := swipeDir(a.lockTo.Sub(a.lockFrom), min(b.Dx(), b.Dy())/4)
		for _, sw := range a.lockSwipes {
			if ok && sw.Dir == d {
				log.Printf("lock: swiped %s, %s", swipeArrows[d], sw.Label)
				sw.Run()
				break
			}
		}
	}
	a.eatTouch = ev.Kind != TouchUp
}

// swipeDir says which way a move of v goes, if it's a swipe: at least
// long, and twice as far that way as across.
func swipeDir(v image.Point, long int) (int, bool) {
	switch {
	case -v.Y >= long && -v.Y >= 2*Abs(v.X):
		return SwipeUp, true
	case v.Y >= long && v.Y >= 2*Abs(v.X):
		return SwipeDown, true
	case v.X >= long && v.X >= 2*Abs(v.Y):
		return SwipeRight, true
	case -v.X >= long && -v.X >= 2*Abs(v.Y):
		return SwipeLeft, true
	}
	return 0, false
}
