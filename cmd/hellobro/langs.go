package main

// The languages to type in (kobo/layouts.go): Settings' "Languages to
// type in" is any of them, chosen with ticks; the first is typed in at
// start, and Ctrl+Alt+Space (or the on-screen keyboard's key) goes on
// to the next (typeIn, vkbd.go).

import (
	"log"
	"strings"
	"time"

	"hellobro/kobo"
)

var langNames = map[string]string{"en": "English", "ru": "Russian", "uk": "Ukrainian", "de": "German", "el": "Greek"}

// langs are the languages to type in, set in Settings.
func (a *bro) langs() []string { return strings.Fields(a.setting("languages")) }

// langValues are the setting's values: every choice of the languages,
// in their order, English alone first; or, shown, their names.
func langValues(shown bool) []string {
	all := kobo.Layouts()
	var vs []string
	for set := 1; set < 1<<len(all); set++ {
		var v []string
		for i, l := range all {
			if set&(1<<i) != 0 {
				if shown {
					l = langNames[l]
				}
				v = append(v, l)
			}
		}
		vs = append(vs, strings.Join(v, map[bool]string{false: " ", true: ", "}[shown]))
	}
	return vs
}

// openLangs puts up the languages as tiles, ticked or not: a tap ticks
// or unticks one, and the menu stays till Esc or a tap outside.
func (a *bro) openLangs() {
	a.openTiles("Languages to type in", "tap to tick or untick", a.langEntries())
}

func (a *bro) langEntries() []pickEntry {
	on := map[string]bool{}
	for _, l := range a.langs() {
		on[l] = true
	}
	var es []pickEntry
	for _, l := range kobo.Layouts() {
		e := pickEntry{kind: "language", title: langNames[l], detail: strings.ToUpper(l), icon: "unticked", stay: true}
		if on[l] {
			e.icon = "ticked"
		}
		e.do = func() { a.tickLang(l) }
		es = append(es, e)
	}
	return es
}

// tickLang ticks language l, or unticks it, keeping one.
func (a *bro) tickLang(l string) {
	var now []string
	for _, o := range kobo.Layouts() {
		if in := strings.Contains(" "+a.setting("languages")+" ", " "+o+" "); in != (o == l) {
			now = append(now, o)
		}
	}
	if len(now) == 0 {
		a.Toast("one language at least", 2*time.Second)
		return
	}
	i := settingIndex("languages")
	a.set["languages"] = indexOf(settingList[i].values, strings.Join(now, " "))
	a.set.save()
	log.Printf("settings: languages = %s", a.setting("languages"))
	a.typeIn(0)
	a.drawSettings()
	if p := a.picker; p != nil {
		p.all = a.langEntries()
	}
}
