package main

// Panels fold: the − at the end of a panel's title strip folds it to the
// strip, and the other panels on its page take the room; + (or a tap
// anywhere on the strip) opens it again. Which are folded is kept in
// folded, a panel's key a line. The terminal doesn't fold: it's alone on
// its page.
//
// And the header says where we are on the network: the Wi-Fi's name and
// our address, looked at now and then.

import (
	"image"
	"log"
	"os"
	"slices"
	"strings"
	"time"

	"hellobro/kobo"
	"hellobro/radio"
)

const (
	foldedTxt = "folded"
	netPoll   = 15 * time.Second
)

type foldable struct {
	key, name string
	tab       int
}

var foldables = []foldable{
	{"now", "Now playing", tabMusic},
	{"library", "Library", tabMusic},
	{"wifi", "Wi-Fi", tabRadio},
	{"bluetooth", "Bluetooth", tabRadio},
	{"settings", "Settings", tabRadio},
	{"sharing", "Sharing", tabFiles},
	{"files", "On this Kobo", tabFiles},
}

func loadFolds() map[string]bool {
	b, _ := os.ReadFile(kobo.AppFile(foldedTxt))
	return parseFolds(string(b))
}

// parseFolds reads the folded file: a key a line, the ones we don't know
// left out.
func parseFolds(s string) map[string]bool {
	f := map[string]bool{}
	for _, line := range strings.Split(s, "\n") {
		k := strings.TrimSpace(line)
		if slices.ContainsFunc(foldables, func(p foldable) bool { return p.key == k }) {
			f[k] = true
		}
	}
	return f
}

func (a *bro) folded(key string) bool { return a.folds[key] }

// setFold folds the panel or opens it, and lays its page out again.
func (a *bro) setFold(key string, on bool) {
	i := slices.IndexFunc(foldables, func(p foldable) bool { return p.key == key })
	if i < 0 || a.folds[key] == on {
		return
	}
	if on {
		a.folds[key] = true
	} else {
		delete(a.folds, key)
	}
	var keys []string
	for _, p := range foldables {
		if a.folds[p.key] {
			keys = append(keys, p.key)
		}
	}
	if err := os.WriteFile(kobo.AppFile(foldedTxt), []byte(strings.Join(keys, "\n")+"\n"), 0o644); err != nil {
		log.Printf("folds: %v", err)
	}
	log.Printf("folds: %s %s", key, map[bool]string{true: "folded", false: "open"}[on])
	switch foldables[i].tab {
	case tabMusic:
		a.layoutMusic(a.lay)
	case tabRadio:
		if p := a.rs.prompt; on && p != nil && p.panel == radioPanelOf(key) {
			a.radioAnswer(false) // nowhere to answer it now
		}
		a.layoutRadio(a.lay)
		a.radioFocusOpen()
	case tabFiles:
		a.layoutFiles(a.lay)
	}
	if foldables[i].tab == a.Tab && !a.overlay() {
		a.DrawPage()
		a.MarkFlash(a.lay.Page)
	}
}

// foldPanel is the panel as drawn, with its button, for taps.
func (a *bro) foldPanel(key string) kobo.Panel {
	switch key {
	case "now":
		return a.nowPanel()
	case "library":
		return a.libPanel()
	case "wifi":
		return a.rv[panelWiFi].p
	case "bluetooth":
		return a.rv[panelBT].p
	case "settings":
		return a.sv.p
	case "sharing":
		return a.fv.shareP
	case "files":
		return a.fv.listP
	}
	return kobo.Panel{}
}

func radioPanelOf(key string) int {
	switch key {
	case "wifi":
		return panelWiFi
	case "bluetooth":
		return panelBT
	}
	return panelSet
}

// foldTap folds or opens the panel tapped on its box (or its strip).
func (a *bro) foldTap(p image.Point) bool {
	for _, f := range foldables {
		if f.tab == a.Tab && kobo.FoldHit(p, a.foldPanel(f.key)) {
			a.setFold(f.key, !a.folded(f.key))
			return true
		}
	}
	return false
}

// foldCommands are the switcher's: minimize or open a panel of the page.
func foldCommands() []*command {
	var cs []*command
	for _, f := range foldables {
		cs = append(cs,
			&command{id: "fold." + f.key, title: f.name + " panel: minimize",
				when: func(a *bro) bool { return a.Tab == f.tab && !a.folded(f.key) },
				run:  func(a *bro) { a.setFold(f.key, true) }},
			&command{id: "unfold." + f.key, title: f.name + " panel: open",
				when: func(a *bro) bool { return a.Tab == f.tab && a.folded(f.key) },
				run:  func(a *bro) { a.setFold(f.key, false) }})
	}
	return cs
}

// --- the network, in the header ---

type netState struct {
	wifi    radio.WiFi // ours, till the Settings tab starts the radios
	at      time.Time
	polling bool
}

// netTick looks at the Wi-Fi now and then, off the UI.
func (a *bro) netTick() {
	n := &a.net
	if n.polling || time.Since(n.at) < netPoll {
		return
	}
	w := a.rs.wifi
	if !a.rs.started {
		if n.wifi == nil {
			n.wifi = radio.NewWiFi()
		}
		w = n.wifi
	}
	n.polling, n.at = true, time.Now()
	go func() {
		ws := w.Status()
		a.Do(func() {
			n.polling, n.at = false, time.Now()
			a.SetNetwork(netText(ws))
		})
	}()
}

// netText is the network for the header: "home-wifi · 192.168.1.20".
func netText(ws radio.WiFiStatus) string {
	switch {
	case !ws.On:
		return "Wi-Fi off"
	case ws.State != "COMPLETED" || ws.SSID == "":
		return "no network"
	case ws.IP == "":
		return ws.SSID + " · no address"
	}
	return ws.SSID + " · " + ws.IP
}
