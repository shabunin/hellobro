package main

// The settings' windows: Wi-Fi, Bluetooth, Sharing, Settings (radio.go,
// sharepanel.go, settings.go), Battery and What uses it (power.go), one
// of each, opened from the launcher or the switcher.

import (
	"image"

	"hellobro/kobo"
)

type guiWin struct {
	key   string // "wifi", "bluetooth", "sharing", "settings", "battery", "usage"
	shown bool   // it showed when last placed
}

var guiTitles = map[string]string{
	"wifi": "Wi-Fi", "bluetooth": "Bluetooth", "sharing": "Sharing", "settings": "Settings",
	"battery": "Battery", "usage": "What uses it",
}

// guiName is the window key's name, "" if there's none.
func guiName(key string) string { return guiTitles[key] }

func (g *guiWin) Title() string    { return guiName(g.key) }
func (g *guiWin) Class() string    { return "hellobro" }
func (g *guiWin) Instance() string { return g.key }

// radio says whether it's one of the Settings tab's that were: the radios
// start for those.
func (g *guiWin) radio() bool {
	return g.key == "wifi" || g.key == "bluetooth" || g.key == "sharing" || g.key == "settings"
}

func (g *guiWin) place(a *bro, p kobo.Panel) {
	p.Name = g.Title()
	switch g.key {
	case "wifi":
		a.placeRadio(panelWiFi, p)
	case "bluetooth":
		a.placeRadio(panelBT, p)
	case "sharing":
		a.placeShare(p)
	case "settings":
		a.placeSettings(p)
	case "battery":
		a.placeBattery(p)
	case "usage":
		a.placeUsage(p)
	}
	shown := !p.Outer.Empty()
	if shown && !g.shown { // it comes up: what it shows, fresh
		if g.radio() {
			a.enterRadio()
		} else {
			a.enterPower()
		}
	}
	g.shown = shown
}

func (g *guiWin) panel(a *bro) kobo.Panel {
	var p kobo.Panel
	switch g.key {
	case "wifi":
		p = a.rv[panelWiFi].p
	case "bluetooth":
		p = a.rv[panelBT].p
	case "sharing":
		p = a.shp.p
	case "settings":
		p = a.sv.p
	case "battery":
		p = a.pv.batP
	case "usage":
		p = a.pv.useP
	}
	return a.winPanel(p, a.guiFocused(g.key))
}

// draw draws it all, as it is now.
func (g *guiWin) draw(a *bro) {
	switch g.key {
	case "wifi":
		a.rv[panelWiFi].shown = ""
		a.drawRadioPanel(panelWiFi)
	case "bluetooth":
		a.rv[panelBT].shown = ""
		a.drawRadioPanel(panelBT)
	case "sharing":
		a.shp.shown = ""
		a.drawShare()
	case "settings":
		a.sv.shown = ""
		a.drawSettings()
	case "battery":
		a.pv.shownBat = ""
		a.drawBattery()
	case "usage":
		a.pv.shownUse = ""
		a.drawUsage()
	}
}

func (g *guiWin) drawTitle(a *bro) { g.draw(a) }

func (g *guiWin) press(a *bro, code uint16, value int32) bool {
	if g.radio() {
		return a.radioKey(code, value)
	}
	return false
}

func (g *guiWin) tap(a *bro, p image.Point) {
	if g.radio() {
		a.radioTap(p)
		return
	}
	a.powerTap(p)
}

func (g *guiWin) closing(a *bro) {
	if p := a.rs.prompt; p != nil && radioPanelOf(g.key) == p.panel {
		a.radioAnswer(false) // nowhere to answer it now
	}
	g.place(a, kobo.Panel{})
}

// winPanel is p as a window's, in focus or not.
func (a *bro) winPanel(p kobo.Panel, focused bool) kobo.Panel {
	return kobo.WithWindow(a.F, p, focused, focused)
}

// radioPanelOf is the Settings tab's panel that was key's.
func radioPanelOf(key string) int {
	switch key {
	case "wifi":
		return panelWiFi
	case "bluetooth":
		return panelBT
	case "sharing":
		return panelShare
	}
	return panelSet
}
