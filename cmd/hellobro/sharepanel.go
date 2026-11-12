package main

// The Settings tab's Sharing panel: how a computer reaches the Kobo
// (share.go), the pairing code, and who's connected, sending what.

import (
	"fmt"
	"log"
)

const panelShare = 2 // after Wi-Fi and Bluetooth, before the settings

// drawShare asks the Sharing window for a frame: what it says changed.
func (a *bro) drawShare() { a.inkRedraw("sharing") }

func (a *bro) shareToggle() {
	i := settingIndex("share")
	if a.setting("share") == "on" {
		a.set["share"] = indexOf(settingList[i].values, "off")
	} else {
		a.set["share"] = indexOf(settingList[i].values, "on")
	}
	a.set.save()
	a.shareApply()
}

func (a *bro) shareForgetAll() {
	s := &a.share
	if s.srv == nil || len(s.keys) == 0 {
		return
	}
	a.ask("Forget the paired computers?", fmt.Sprintf("%d of them will have to pair again to get in.", len(s.keys)), "forget", func() {
		for _, k := range s.keys {
			if err := s.srv.Forget(k.Fingerprint); err != nil {
				log.Printf("share: forget: %v", err)
			}
		}
		s.keys, _ = s.srv.Keys()
		s.note = "forgot the paired computers"
		a.drawShare()
	})
}
