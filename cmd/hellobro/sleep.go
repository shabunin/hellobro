package main

// Sleep and wake, around kobo.App's suspend: the players pause where
// they are, the radios go off for the night as Nickel has them do, and come
// back on waking, the SSH sessions reconnecting once Wi-Fi is up.

import (
	"fmt"
	"log"
	"strings"
	"time"

	"hellobro/kobo"
	"hellobro/radio"
)

// sleepState is what was on when the Kobo went to sleep.
type sleepState struct {
	wifiWas, btWas bool
	warned         bool      // the toast said it's going to sleep
	plugged        bool      // on power, as last seen
	plugAt         time.Time // plugged in or out: idle counts from then too
	low            int       // the battery's %, once it's low and not charging; 0 otherwise
	asleep         bool
	wokeAt         time.Time
}

const batteryLow = 10 // %: a warning, once on the way down

const sleepWarn = time.Minute // the toast comes this long before

// sleepWhenIdle puts the Kobo to sleep after the settings' while without
// input, on battery or plugged in: left awake, it lasts a night at most.
// Whatever runs goes down with it (SSH, the relay, a copy to it), so a
// toast says so a minute before, and a touch or a key keeps it awake.
// Music doesn't keep it awake: it's paused, the battery comes first.
// Plugging in or out starts the while again, or the battery's, long run
// out on power, would sleep it at once with no toast. It's the only way
// hellobro goes idle: it doesn't quit for want of input (IdleTimeout).
func (a *bro) sleepWhenIdle() {
	key := "sleep-battery"
	if a.pw.plugged {
		key = "sleep-power"
	}
	d := a.settingMinutes(key)
	if a.pw.plugged != a.slp.plugged {
		a.slp.plugged, a.slp.plugAt = a.pw.plugged, time.Now()
	}
	if d == 0 || a.Calibrating() {
		a.slp.warned = false
		return
	}
	switch idle := min(a.IdleFor(), time.Since(a.slp.plugAt)); {
	case idle >= d:
		a.slp.warned = false
		log.Printf("sleep: %v without input (%s)", d, key)
		a.App.Sleep()
	case idle >= d-sleepWarn:
		if !a.slp.warned {
			a.slp.warned = true
			a.Toast("sleeping in a minute: SSH, the relay and sharing drop. touch or type to stay awake", sleepWarn)
		}
	default:
		a.slp.warned = false
	}
}

func (a *bro) Sleep() func() {
	a.slp.asleep = true
	a.picker = nil   // the sleep screen goes over it; waking draws the page
	a.pausePlayers() // the Bluetooth audio goes with the radio: on again after, where it was
	a.unmountAll(true)
	a.radioStart()
	r, s := &a.rs, &a.slp
	r.discoverTo = time.Time{}
	if p := r.prompt; p != nil && p.reply != nil {
		p.reply <- radioReply{}
	}
	r.prompt = nil
	wifi, bt := r.wifi, r.bt
	return func() {
		s.wifiWas = wifi.Status().On // also notes how Kobo runs it, for turning it on again
		bs, _ := bt.Status()
		if s.btWas = bs.Powered; s.btWas { // first: Wi-Fi's chip may be Bluetooth's too
			if err := bt.SetPower(false); err != nil {
				log.Printf("sleep: bluetooth off: %v", err)
			}
		}
		if s.wifiWas {
			if err := wifi.SetPower(false, func(p string) { log.Printf("sleep: wifi: %s", p) }); err != nil {
				log.Printf("sleep: wifi off: %v", err)
			}
		}
		log.Printf("sleep: radios off (wifi was on %v, bluetooth %v)", s.wifiWas, s.btWas)
	}
}

func (a *bro) Wake() {
	r, s := &a.rs, &a.slp
	s.asleep, s.wokeAt = false, time.Now()
	if s.btWas {
		a.radioRun(panelBT, "turning Bluetooth back on…", func(func(string)) error { return r.bt.SetPower(true) })
	}
	if !s.wifiWas {
		a.termKick()
		a.mountAgain()
		return
	}
	a.radioRun(panelWiFi, "turning Wi-Fi back on…", func(p func(string)) error {
		err := r.wifi.SetPower(true, p)
		a.Do(func() {
			a.termKick()
			a.mountAgain()
		})
		return err
	})
}

// termKick has the sessions waiting to reconnect try now, from the start.
func (a *bro) termKick() {
	for _, t := range a.terms() {
		if t.state == termWaiting {
			t.retries, t.retryAt = 0, time.Now()
		}
	}
}

// radioStart makes the radios' backends, the first time they're wanted.
func (a *bro) radioStart() {
	r := &a.rs
	if r.started {
		return
	}
	r.started = true
	r.wifi = radio.NewWiFi()
	r.bt = radio.NewBluetooth(radioAgent{a})
	factsOnce.Do(func() { go radio.LogFacts() })
}

// lock locks the screen till the lock's keys again. With no keyboard, or
// no key for it, the lock screen has swipes to get out by touch.
func (a *bro) lock() {
	k := a.keyLabel("lock")
	hint := k + " unlocks"
	if k == "" {
		hint = ""
	}
	a.picker = nil // what's open stays put; the switcher would only be in the way
	a.App.Lock(hint, a.lockLines())
	if a.Keyboard == "" || k == "" { // no key to unlock it: swipes
		a.SetLockSwipes(a.lockSwipes())
	}
}

// lockLines are what the lock screen says: the music's track and whether
// it plays, and the battery, if it's low.
func (a *bro) lockLines() []string {
	var ls []string
	if p, ok := a.target(); ok && p.title != "" {
		s := "♪ " + p.title
		if p.artist != "" {
			s += " · " + p.artist
		}
		ls = append(ls, s, strings.ToLower(p.status))
	}
	if a.slp.low > 0 {
		ls = append(ls, fmt.Sprintf("battery %d%%: plug it in", a.slp.low))
	}
	return ls
}

// lockSwipes are the lock screen's, with the keyboard gone: nothing else
// could unlock it. Bluetooth unlocks too, for the keyboard to connect
// again, its window full screen: easy to hit by touch.
func (a *bro) lockSwipes() []kobo.LockSwipe {
	return []kobo.LockSwipe{
		{Dir: kobo.SwipeUp, Label: "unlock", Run: a.Unlock},
		{Dir: kobo.SwipeRight, Label: "Bluetooth", Run: func() { a.Unlock(); a.kbdBluetooth() }},
		{Dir: kobo.SwipeDown, Label: "back to Nickel", Run: a.Quit},
	}
}

// batteryCheck warns once the battery is low on its way down: a toast,
// and a line on the lock screen. pct is the battery's now.
func (a *bro) batteryCheck(pct int, charging bool) {
	low := 0
	if !charging && pct <= batteryLow {
		low = pct
	}
	if low > 0 && a.slp.low == 0 {
		log.Printf("battery: %d%%, low", pct)
		a.Toast(fmt.Sprintf("battery %d%%: plug it in", pct), 10*time.Second)
	}
	if low != a.slp.low {
		a.slp.low = low
		a.SetLockLines(a.lockLines())
	}
}

// lockedKey is a key while locked: the lock's unlocks, the modifiers are
// kept track of, anything else goes nowhere.
func (a *bro) lockedKey(code uint16, value int32) {
	if a.termMod(code, value) {
		return
	}
	if value == 0 {
		delete(a.keys.down, code) // the lock key's release, say
		return
	}
	if c := a.keyCommand(code); c != nil && c.id == "lock" {
		if value == 1 {
			a.keys.down[code] = true
			a.Unlock()
		}
	}
}
