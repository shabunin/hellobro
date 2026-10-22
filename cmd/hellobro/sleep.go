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
	dozedOff := a.doze.wifiOff // off for a doze we're leaving for sleep: it was on
	a.doze.wifiAsk = false     // and waking from the doze doesn't turn it on: sleep does
	if dozedOff {
		a.dozeWiFi("forget")
	}
	return func() {
		s.wifiWas = wifi.Status().On || dozedOff // also notes how Kobo runs it, for turning it on again
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
