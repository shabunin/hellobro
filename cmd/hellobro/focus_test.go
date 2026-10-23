package main

import (
	"image"
	"testing"

	"hellobro/kobo"
	"hellobro/wm"
)

// Tab goes round a window's controls in order, Shift+Tab back, and only a
// control moved to takes Enter or Space.
func TestFocusTab(t *testing.T) {
	a := &bro{App: &kobo.App{}}
	a.wm.tree = wm.New(image.Rect(0, 0, 900, 900), kobo.FoldH)
	a.wm.wins = map[*wm.Node]window{}
	g := &guiWin{key: "wifi"}
	a.wm.wins[a.wm.tree.Open(g)] = g
	a.wm.tree.Render()
	a.rv[panelWiFi].p = kobo.Panel{Outer: image.Rect(0, 0, 900, 900), Body: image.Rect(0, 0, 900, 900)}
	a.rv[panelWiFi].list = image.Rect(16, 60, 880, 700)
	for i, l := range []string{"open", "up"} {
		a.rv[panelWiFi].buttons = append(a.rv[panelWiFi].buttons, radioButton{r: image.Rect(16+i*200, 720, 200+i*200, 780), label: l})
	}
	at := func() string { cs := a.controls(g); return cs[a.focused(g, cs)].id }
	if got := at(); got != "wifi.list" {
		t.Fatalf("at first: %s, want the list", got)
	}
	if a.focusKeyed(kobo.KeyEnter, 1) || a.focusKeyed(keyDown, 1) {
		t.Error("the list's keys taken")
	}
	press := func(code uint16, shift bool) {
		a.tv.mods.shift = shift
		if !a.focusKeyed(code, 1) {
			t.Fatalf("key %d not taken", code)
		}
	}
	for _, want := range []string{"wifi.open", "wifi.up", "wifi.list", "wifi.open"} {
		press(kobo.KeyTab, false)
		if got := at(); got != want {
			t.Errorf("Tab: %s, want %s", got, want)
		}
	}
	press(kobo.KeyTab, true)
	press(kobo.KeyTab, true)
	if got := at(); got != "wifi.up" {
		t.Errorf("Shift+Tab twice: %s", got)
	}
	if !a.fs.ring {
		t.Error("no ring after Tab")
	}
	a.focusTouched(image.Pt(400, 400)) // the list
	if a.fs.ring || at() != "wifi.list" {
		t.Errorf("touched: ring %v, at %s", a.fs.ring, at())
	}
	// A button that's gone: back to the panel's list, and nothing pressed.
	a.fs.at = "wifi.delete"
	if at() != "wifi.list" || a.focusKeyed(kobo.KeySpace, 1) {
		t.Errorf("gone: at %s", at())
	}
	// No list: a button is in focus by default, but Enter isn't for it.
	a.rv[panelWiFi].list = image.Rectangle{}
	a.fs.at = ""
	if at() != "wifi.open" || a.focusKeyed(kobo.KeyEnter, 1) {
		t.Errorf("by default: at %s, and Enter taken", at())
	}
}
