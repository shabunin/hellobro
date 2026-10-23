package kobo

// Locked: the touch screen does nothing, and of the keys only those the
// Program lets through (its unlock key, the music's), so the Kobo can go
// in a bag or a hand with the screen on. None of it counts as use: the
// light stays off, and sleeping when idle still comes.

import (
	"log"
	"time"
)

const (
	lockToast      = 3 * time.Second
	lockNudgeEvery = 10 * time.Second // a hand on the screen in a bag mustn't keep the panel busy
)

// Lock locks; hint says how to unlock, in a toast now and on input that
// goes nowhere.
func (a *App) Lock(hint string) {
	log.Print("lock: locked")
	a.locked, a.lockHint, a.lockNudgeAt = true, hint, time.Now()
	a.Toast(hint, lockToast)
}

// Unlock unlocks.
func (a *App) Unlock() {
	if !a.locked {
		return
	}
	log.Print("lock: unlocked")
	a.locked = false
	a.Toast("unlocked", lockToast)
}

// Locked says whether the app is locked.
func (a *App) Locked() bool { return a.locked }

// LockNudge shows the hint again, for input that went nowhere, now and
// then.
func (a *App) LockNudge() {
	if time.Since(a.lockNudgeAt) < lockNudgeEvery {
		return
	}
	a.lockNudgeAt = time.Now()
	a.Toast(a.lockHint, lockToast)
}

// lockTouch takes the touch screen while locked, and a contact that began
// then till it's over, so its moves don't reach a gesture that never saw
// it start.
func (a *App) lockTouch(ev TouchEv) {
	if ev.Kind == TouchDown && a.locked {
		a.LockNudge()
	}
	a.eatTouch = ev.Kind != TouchUp
}
