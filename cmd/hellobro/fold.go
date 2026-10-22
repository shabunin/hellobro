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
	{"wifi", "Wi-Fi", tabRadio},
	{"bluetooth", "Bluetooth", tabRadio},
	{"settings", "Settings", tabRadio},
	{"sharing", "Sharing", tabFiles},
	{"files", "On this Kobo", tabFiles},
	{"battery", "Battery", tabPower},
	{"usage", "What uses it", tabPower},
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
	// The focus goes to the panel opened, or off the one folded.
	switch foldables[i].tab {
	case tabRadio:
		if p := a.rs.prompt; on && p != nil && p.panel == radioPanelOf(key) {
			a.radioAnswer(false) // nowhere to answer it now
		}
		a.layoutRadio(a.lay)
		if on {
			a.radioFocusOpen()
		} else {
			a.rs.focus = radioPanelOf(key)
		}
	case tabFiles:
		a.fv.shareFocus = (key == "sharing") != on
		a.layoutFiles(a.lay)
	case tabPower:
		a.pv.useFocus = (key == "usage") != on
		a.layoutPower(a.lay)
	}
	if foldables[i].tab == a.Tab && !a.overlay() {
		a.DrawPage()
		a.MarkFlash(a.lay.Page)
	}
}

// foldPanel is the panel as drawn, with its button, for taps.
func (a *bro) foldPanel(key string) kobo.Panel {
	switch key {
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
	case "battery":
		return a.pv.batP
	case "usage":
		return a.pv.useP
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
	if g := a.termGroup(); g != nil && !a.Full {
		for _, t := range g.sessions {
			if kobo.FoldHit(p, a.termPanel(t)) {
				a.setSessionFold(t, !t.folded)
				return true
			}
		}
		return false
	}
	for _, f := range foldables {
		if f.tab == a.Tab && kobo.FoldHit(p, a.foldPanel(f.key)) {
			a.setFold(f.key, !a.folded(f.key))
			return true
		}
	}
	return false
}

// foldCommands are the switcher's: minimize or open a panel of the page.
// A folded session opens as it's picked in the switcher.
func foldCommands() []*command {
	cs := []*command{{id: "panel.fold", title: "Panel in focus: minimize / open", keys: []string{"ctrl+shift+m"},
		when: func(a *bro) bool { return !a.Full && (!isTermTab(a.Tab) || a.term() != nil) },
		run:  (*bro).foldFocused}}
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
	ws      radio.WiFiStatus
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
			n.polling, n.at, n.ws = false, time.Now(), ws
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

// --- the focus ---

// stepPanel moves the focus to the next panel on the page (d -1: the one
// before), as Ctrl+Tab does: the keys go to it. A folded one takes the
// focus too, and Enter opens it.
func (a *bro) stepPanel(d int) {
	delete(a.fs.at, a.Tab) // to the new panel's list, or its first control
	switch a.Tab {
	case tabFiles:
		a.fv.shareFocus = !a.fv.shareFocus
	case tabPower:
		a.pv.useFocus = !a.pv.useFocus
	case tabRadio:
		a.radioStep(d)
		return
	default:
		a.stepSession(d)
		return
	}
	a.focusMoved()
}

// focusMoved shows the focus on a new panel: its mark, or fullscreen, the
// whole page, which is its now.
func (a *bro) focusMoved() {
	if a.Full {
		a.Layout(a.lay)
		a.DrawPage()
		a.MarkFlash(a.lay.Page)
		return
	}
	switch a.Tab {
	case tabFiles:
		a.fv.shownShare, a.fv.shownList = "", ""
		a.drawFiles()
	case tabRadio:
		a.drawRadio()
	case tabPower:
		a.pv.shownBat, a.pv.shownUse = "", ""
		a.drawPower()
	}
}

// tile places a page's panels in rows, as Arrange does. Fullscreen, the
// one in focus (its place in the rows) has it all, and the others are
// folded out of sight: what's folded comes back with the rects.
func tile(K kobo.Layout, focus int, rows ...kobo.Row) ([]image.Rectangle, []bool) {
	var folded []bool
	for _, r := range rows {
		folded = append(folded, r.Folded...)
	}
	if !K.Full {
		return K.Arrange(K.Body, rows...), folded
	}
	at := make([]image.Rectangle, len(folded))
	for i := range folded {
		folded[i] = i != focus
	}
	at[focus] = K.Body.Inset(kobo.Pad)
	return at, folded
}

// pagePanel is a page's panel at r, foldable. Fullscreen, the one in focus
// doesn't fold, and the others are nowhere: folded, with nothing to draw
// or tap.
func (a *bro) pagePanel(K kobo.Layout, name string, r image.Rectangle, folded bool) kobo.Panel {
	switch {
	case !K.Full:
		return kobo.WithFold(a.F, kobo.NewPanel(a.F, name, r, ""), folded)
	case folded:
		return kobo.Panel{Folded: true}
	}
	return kobo.NewPanel(a.F, name, r, "")
}

// focusMark marks the name of the panel in focus.
func focusMark(p kobo.Panel, focused bool) kobo.Panel {
	if focused {
		p.Name = "► " + p.Name
	}
	return p
}

// focusKey is the foldable panel in focus; "" on a terminal tab.
func (a *bro) focusKey() string {
	switch a.Tab {
	case tabFiles:
		return map[bool]string{true: "sharing", false: "files"}[a.fv.shareFocus]
	case tabRadio:
		return []string{"wifi", "bluetooth", "settings"}[a.rs.focus]
	case tabPower:
		return map[bool]string{false: "battery", true: "usage"}[a.pv.useFocus]
	}
	return ""
}

// focusFolded says whether the panel in focus is folded to its strip.
func (a *bro) focusFolded() bool {
	if a.Full { // the one in focus has the screen
		return false
	}
	if k := a.focusKey(); k != "" {
		return a.folded(k)
	}
	t := a.term()
	return t != nil && t.folded && !a.Full
}

// foldFocused folds the panel in focus, or opens it.
func (a *bro) foldFocused() {
	if a.Full {
		return
	}
	if k := a.focusKey(); k != "" {
		a.setFold(k, !a.folded(k))
	} else if t := a.term(); t != nil {
		a.setSessionFold(t, !t.folded)
	}
}
