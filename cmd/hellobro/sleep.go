package main

// Sleep and wake, around kobo.App's suspend: the music stops where it
// was, the radios go off for the night as Nickel has them do, and come
// back on waking, the SSH sessions reconnecting once Wi-Fi is up.

import (
	"log"
	"time"

	"hellobro/radio"
)

// sleepState is what was on when the Kobo went to sleep.
type sleepState struct {
	wifiWas, btWas bool
}

func (a *bro) Sleep() func() {
	a.picker = nil // the sleep screen goes over it; waking draws the page
	if a.mus.player != nil {
		a.musicStop() // the Bluetooth audio goes with the radio: start afresh after
		a.saveMusicState()
		a.musicHeader()
	}
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
		if s.wifiWas {
			if err := wifi.SetPower(false, func(p string) { log.Printf("sleep: wifi: %s", p) }); err != nil {
				log.Printf("sleep: wifi off: %v", err)
			}
		}
		bs, _ := bt.Status()
		if s.btWas = bs.Powered; s.btWas {
			if err := bt.SetPower(false); err != nil {
				log.Printf("sleep: bluetooth off: %v", err)
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
		return
	}
	a.radioRun(panelWiFi, "turning Wi-Fi back on…", func(p func(string)) error {
		err := r.wifi.SetPower(true, p)
		a.Do(a.termKick)
		return err
	})
}

// termKick has the sessions waiting to reconnect try now, from the start.
func (a *bro) termKick() {
	for _, g := range a.tv.groups {
		for _, t := range g.sessions {
			if t.state == termWaiting {
				t.retries, t.retryAt = 0, time.Now()
			}
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
