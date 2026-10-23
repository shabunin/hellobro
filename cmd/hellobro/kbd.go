package main

// The keyboard gone: hellobro's keys are the most of it, so after a while
// without one the Bluetooth window comes up, full screen, to connect it
// again; and goes when it's back. The while is for the keyboard's own
// naps and the radio's blips: back within it, nothing shows. Asleep, or
// just awake (Bluetooth comes back on with the keyboard), it waits. The
// while is Settings' "The keyboard gone", which may be never.

import (
	"log"
	"strconv"
	"time"
)

// kbdWait is the while; 0: nothing comes up.
func (a *bro) kbdWait() time.Duration {
	n, _ := strconv.Atoi(a.setting("kbd-gone"))
	return time.Duration(n) * time.Second
}

type kbdState struct {
	gen    int  // which going the timer is for
	opened bool // the Bluetooth window is ours: it closes when the keyboard's back
	full   bool // it was there, we put it full screen: it goes back
}

// kbdGone starts the while.
func (a *bro) kbdGone() {
	a.kbd.gen++
	gen, wait := a.kbd.gen, a.kbdWait()
	if wait == 0 {
		return
	}
	time.AfterFunc(wait, func() { a.Do(func() { a.kbdStillGone(gen) }) })
}

func (a *bro) kbdStillGone(gen int) {
	switch {
	case gen != a.kbd.gen || a.Keyboard != "":
	case a.Locked(): // the lock screen has its swipes
	case a.slp.asleep || time.Since(a.slp.wokeAt) < 3*a.kbdWait():
		a.kbdGone() // later
	case a.guiFocused("bluetooth"): // there already
	default:
		log.Print("keyboard: still gone, the Bluetooth window")
		a.kbdBluetooth()
	}
}

// kbdBluetooth opens the Bluetooth window full screen, for the keyboard
// to connect again: a big one, by touch.
func (a *bro) kbdBluetooth() {
	g := a.gui("bluetooth")
	opened := g == nil
	a.openGUI("bluetooth")
	if g = a.gui("bluetooth"); g == nil {
		return
	}
	n := a.nodeOf(g)
	a.kbd.opened, a.kbd.full = opened, !opened && n != nil && !n.Fullscreen
	a.fullWin(g)
}

// kbdBack puts away the Bluetooth window that came for the keyboard.
func (a *bro) kbdBack() {
	k := &a.kbd
	k.gen++
	g := a.gui("bluetooth")
	switch {
	case g == nil:
	case k.opened:
		a.closeWin(g)
	case k.full:
		if n := a.nodeOf(g); n != nil && n.Fullscreen {
			a.wm.tree.SetFullscreen(n, false)
			a.wmApply()
		}
	}
	k.opened, k.full = false, false
}
