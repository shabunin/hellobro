package main

// The settings' windows: Wi-Fi, Bluetooth, Sharing, Settings (radio.go,
// sharepanel.go, settings.go), Battery and What uses it (power.go), one
// of each, opened from the launcher or the switcher.

import (
	"image"

	"hellobro/kobo"
)

type guiWin struct {
	key   string  // "wifi", "bluetooth", "sharing", "settings", "battery", "usage"
	shown bool    // it showed when last placed
	ink   *inkWin // its body's ink's, if it's one of those (inkwin.go)
}

var guiTitles = map[string]string{
	"wifi": "Wi-Fi", "bluetooth": "Bluetooth", "sharing": "Sharing", "settings": "Settings",
	"battery": "Battery", "usage": "What uses it", wizKey: "New notebook",
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
	a.inkOf(g).place(p)
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
	return a.winPanel(a.inkOf(g).p, a.guiFocused(g.key))
}

// draw draws it all, as it is now.
func (g *guiWin) draw(a *bro) { a.inkOf(g).draw(a, a.guiFocused(g.key)) }

func (g *guiWin) drawTitle(a *bro) { a.inkOf(g).drawTitle(a, a.guiFocused(g.key)) }

func (g *guiWin) press(a *bro, code uint16, value int32) bool {
	if u, ok := a.inkOf(g).ui.(*noteWizUI); ok && u.key(code, value) {
		a.inkTick()
		return true
	}
	took := a.inkOf(g).pane.Key(code, value, inkMods(a.tv.mods), a.tv.mods.caps)
	a.inkTick()
	return took
}

// tap: its contacts came as they were (inkDown).
func (g *guiWin) tap(a *bro, p image.Point) {}

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
