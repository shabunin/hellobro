package main

// Sleep and wake, around kobo.App's suspend: the players pause where
// they are, the radios go off for the night as Nickel has them do, and come
// back on waking, the SSH sessions reconnecting once Wi-Fi is up.

import (
	"log"
	"time"

	"hellobro/radio"
)

// sleepState is what was on when the Kobo went to sleep.
type sleepState struct {
	wifiWas, btWas bool
	warned         bool      // the toast said it's going to sleep
	musicAt        time.Time // the music last played: idle counts from then too
}

const sleepWarn = time.Minute // the toast comes this long before

// sleepWhenIdle puts the Kobo to sleep after the settings' while without
// input, on battery or plugged in: left awake, it lasts a night at most.
// Whatever runs goes down with it (SSH, the relay, a copy to it), so a
// toast says so a minute before, and a touch or a key keeps it awake.
// Music playing keeps it awake too, and once it stops the while starts
// again: after an album, the toast still comes first.
func (a *bro) sleepWhenIdle() {
	key := "sleep-battery"
	if a.pw.plugged {
		key = "sleep-power"
	}
	d := a.settingMinutes(key)
	if a.musicPlaying() {
		a.slp.musicAt = time.Now()
	}
	if d == 0 || a.musicPlaying() || a.Calibrating() {
		a.slp.warned = false
		return
	}
	switch idle := min(a.IdleFor(), time.Since(a.slp.musicAt)); {
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

// lock locks the screen till the lock's keys again. Only with a keyboard
// and a key for it: a lock from the switcher or the menu would leave a
// Kobo with neither stuck.
func (a *bro) lock() {
	k := a.keyLabel("lock")
	switch {
	case a.Keyboard == "":
		a.Toast("locking wants a keyboard: its keys unlock", 4*time.Second)
	case k == "":
		a.Toast("locking wants a key for it (keys.conf: lock)", 4*time.Second)
	default:
		a.picker = nil // what's open stays put; the switcher would only be in the way
		a.App.Lock("locked: touch does nothing. " + k + " unlocks")
	}
}

// lockedKey is a key while locked: the lock's unlocks, the modifiers are
// kept track of, anything else only says how to unlock.
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
		return
	}
	a.LockNudge()
}
