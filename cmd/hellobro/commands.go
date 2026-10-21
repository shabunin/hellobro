package main

// Commands: what hellobro does on a key, or from the switcher. Each has
// an id for keys.conf, a title for the switcher, and default keys.
// keys.conf, next to hellobro, changes them: a line is a key and a
// command ("ctrl+alt+1 tab.1"), a key and "-", which frees it (a
// terminal gets it then), or "none" and a command, which has no key
// then. A command named there loses its defaults. The file is written
// the first time, every default in it commented out, and read again
// when it's saved; Settings' Keys row writes it too (keyset.go).

import (
	"errors"
	"fmt"
	"log"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"hellobro/kobo"
)

const keysTxt = "keys.conf"

type command struct {
	id, title string
	keys      []string        // the defaults
	when      func(*bro) bool // nil: always
	run       func(*bro)
	hidden    bool // not in the switcher
	repeat    bool // held, it goes on
}

func onTerm(a *bro) bool { return isTermTab(a.Tab) }

var commands []*command

func init() { commands = makeCommands() } // the switcher, one of them, lists them

func makeCommands() []*command {
	cs := []*command{
		{id: "palette", title: "Go to…", keys: []string{"ctrl+shift+p"}, run: (*bro).openPicker, hidden: true},
	}
	for n := 1; n <= 9; n++ {
		cs = append(cs, &command{id: fmt.Sprintf("tab.%d", n), title: fmt.Sprintf("Tab %d", n),
			keys: []string{fmt.Sprintf("ctrl+alt+%d", n)}, hidden: true,
			run: func(a *bro) {
				if tabs := a.Tabs(); n <= len(tabs) {
					a.SwitchTab(tabs[n-1])
				}
			}})
	}
	for t, name := range tabNames {
		cs = append(cs, &command{id: "tab." + tabKey(t), title: name + " tab", hidden: true,
			run: func(a *bro) { a.SwitchTab(t) }})
	}
	cs = append(cs,
		&command{id: "tab.next", title: "Next tab", run: func(a *bro) { a.stepTab(1) }},
		&command{id: "tab.prev", title: "Previous tab", run: func(a *bro) { a.stepTab(-1) }},
		&command{id: "session.new", title: "New session", keys: []string{"ctrl+shift+t"}, when: onTerm,
			run: func(a *bro) {
				a.newSession(true)
				a.drawTerm()
				a.MarkFlash(a.termP.Outer)
			}},
		&command{id: "panel.next", title: "Next panel", keys: []string{"ctrl+tab"}, repeat: true,
			run: func(a *bro) { a.stepPanel(1) }},
		&command{id: "panel.prev", title: "Previous panel", keys: []string{"ctrl+shift+tab"}, repeat: true,
			run: func(a *bro) { a.stepPanel(-1) }},
		&command{id: "session.next", title: "Next session", keys: []string{"ctrl+pgdn", "ctrl+shift+right"},
			when: onTerm, repeat: true, run: func(a *bro) { a.stepSession(1) }},
		&command{id: "session.prev", title: "Previous session", keys: []string{"ctrl+pgup", "ctrl+shift+left"},
			when: onTerm, repeat: true, run: func(a *bro) { a.stepSession(-1) }},
		&command{id: "copy", title: "Copy the selection", keys: []string{"ctrl+shift+c"}, when: onTerm, run: (*bro).copySel},
		&command{id: "copy.screen", title: "Copy the screen's text", when: onTerm, run: func(a *bro) {
			if t := a.term(); t != nil && t.connected() {
				a.copyText(a.screenText(t), "")
			}
		}},
		&command{id: "paste", title: "Paste", keys: []string{"ctrl+shift+v"}, run: func(a *bro) { a.paste(readClip(), "") }},
		&command{id: "copy.key", title: "Copy the Kobo's SSH key", run: (*bro).copyKoboKey},
		&command{id: "paste.key", title: "Paste the Kobo's SSH key", run: (*bro).pasteKoboKey},
		&command{id: "fullscreen", title: "Fullscreen", keys: []string{"f11"}, when: onTerm,
			run: func(a *bro) { a.SetFull(!a.Full) }},
		&command{id: "font.bigger", title: "Bigger font", keys: []string{"ctrl+="}, when: onTerm, repeat: true,
			run: func(a *bro) { a.setTermFont(a.tv.fontPx + termFontStep) }},
		&command{id: "font.smaller", title: "Smaller font", keys: []string{"ctrl+-"}, when: onTerm, repeat: true,
			run: func(a *bro) { a.setTermFont(a.tv.fontPx - termFontStep) }},
		&command{id: "font.reset", title: "Font size back", keys: []string{"ctrl+0"}, when: onTerm,
			run: func(a *bro) { a.setTermFont(termFontDefault) }},
		&command{id: "music.play", title: "Play / pause", keys: []string{"ctrl+shift+space"},
			run: func(a *bro) { a.mediaKey(kobo.KeyPlayPause) }},
		&command{id: "music.next", title: "Next track", keys: []string{"ctrl+shift+."},
			run: func(a *bro) { a.mediaKey(kobo.KeyNextSong) }},
		&command{id: "music.prev", title: "Previous track", keys: []string{"ctrl+shift+,"},
			run: func(a *bro) { a.mediaKey(kobo.KeyPreviousSong) }},
		&command{id: "statusbar", title: "Status bar: show / hide", keys: []string{"ctrl+shift+b"}, run: (*bro).toggleStat},
		&command{id: "sleep", title: "Sleep", keys: []string{"ctrl+alt+s"}, run: func(a *bro) { a.App.Sleep() }},
		&command{id: "doze", title: "Light sleep", run: func(a *bro) { a.Doze() }},
		&command{id: "wifi", title: "Wi-Fi on / off", run: func(a *bro) { a.radioToggle(panelWiFi) }},
		&command{id: "bluetooth", title: "Bluetooth on / off", run: func(a *bro) { a.radioToggle(panelBT) }},
		&command{id: "sharing", title: "File sharing on / off", run: (*bro).shareToggle},
		&command{id: "sharing.pair", title: "Pair a computer", run: (*bro).sharePair},
		&command{id: "relay", title: "Relay on / off", run: func(a *bro) { a.stepSetting(settingIndex("relay"), 1) }},
		&command{id: "tabs", title: "Tabs: show, hide, order", run: (*bro).openTabs},
		&command{id: "keys", title: "Keys: set a command's", run: func(a *bro) { a.openKeys(nil) }},
	)
	cs = append(cs, foldCommands()...)
	for t, name := range tabNames {
		if t == tabRadio {
			continue // where tabs are set: always there
		}
		cs = append(cs,
			&command{id: "tabs.show." + tabKey(t), title: "Show the " + name + " tab",
				when: func(a *bro) bool { return a.tabs.hidden[t] }, run: func(a *bro) { a.showTab(t, true) }},
			&command{id: "tabs.hide." + tabKey(t), title: "Hide the " + name + " tab",
				when: func(a *bro) bool { return !a.tabs.hidden[t] }, run: func(a *bro) { a.showTab(t, false) }})
	}
	cs = append(cs,
		&command{id: "light.up", title: "Light: brighter", keys: []string{"ctrl+alt+up", "brightnessup"}, repeat: true,
			run: func(a *bro) { a.stepLight(lightStep) }},
		&command{id: "light.down", title: "Light: dimmer", keys: []string{"ctrl+alt+down", "brightnessdown"}, repeat: true,
			run: func(a *bro) { a.stepLight(-lightStep) }},
		&command{id: "light.toggle", title: "Light: on / off (the backlight)", keys: []string{"ctrl+alt+l"}, run: (*bro).toggleLight},
		&command{id: "rotate", title: "Rotate the screen", run: func(a *bro) { a.Rotate() }},
	)
	for r, name := range []string{"Portrait", "Landscape", "Portrait, upside down", "Landscape, the other way"} {
		cs = append(cs, &command{id: "screen." + []string{"portrait", "landscape", "portrait2", "landscape2"}[r], title: "Screen: " + name,
			when: func(a *bro) bool { return a.Rotation() != r }, run: func(a *bro) { a.SetRotation(r) }})
	}
	cs = append(cs,
		&command{id: "calibrate", title: "Touch calibration", run: func(a *bro) { a.Calibrate() }},
		&command{id: "quit", title: "Quit, back to Nickel", run: func(a *bro) { a.Quit() }},
	)
	return cs
}

func commandByID(id string) *command {
	for _, c := range commands {
		if c.id == id {
			return c
		}
	}
	return nil
}

// --- keys ---

// combo is a key with the modifiers held.
type combo struct {
	mods uint8
	code uint16
}

const (
	modCtrl = 1 << iota
	modShift
	modAlt
	modMeta
)

type modName struct {
	name  string
	bit   uint8
	label string
}

var modNames = []modName{{"ctrl", modCtrl, "Ctrl"}, {"shift", modShift, "Shift"}, {"alt", modAlt, "Alt"}, {"meta", modMeta, "Meta"}}

var keyCodes = map[string]uint16{
	"esc": kobo.KeyEsc, "tab": kobo.KeyTab, "space": kobo.KeySpace, "enter": kobo.KeyEnter,
	"backspace": kobo.KeyBackspace, "up": keyUp, "down": keyDown, "left": keyLeft, "right": keyRight,
	"pgup": keyPgUp, "pgdn": keyPgDn, "home": 102, "end": 107, "insert": 110, "delete": 111,
	"f11": keyF11, "f12": keyF12, "minus": keyMinus, "brightnessdown": 224, "brightnessup": 225,
}

var keyLabelNames = map[uint16]string{224: "Brightness−", 225: "Brightness+"}

// keyConfNames are the keys' names in keys.conf, by code.
var keyConfNames = map[uint16]string{}

func init() {
	for code := uint16(kobo.Key1); code <= 53; code++ { // the rows of letters, digits and marks
		if ch, ok := kobo.KeyChar(code, false, false); ok {
			keyCodes[ch] = code
		}
	}
	for i := range uint16(10) {
		keyCodes[fmt.Sprintf("f%d", i+1)] = kobo.KeyF1 + i
	}
	for name, code := range keyCodes {
		keyConfNames[code] = name
	}
	keyConfNames[keyMinus] = "minus" // of its two names, the one that reads well before " -"
}

// parseKey reads a key as keys.conf has it: "ctrl+shift+p", "f11", "ctrl+=".
func parseKey(s string) (combo, error) {
	var c combo
	s = strings.ToLower(s)
	parts := strings.Split(s, "+")
	if strings.HasSuffix(s, "++") { // "ctrl++" can't be: + is Shift+=
		return c, fmt.Errorf("%q: + is shift+=", s)
	}
	for _, m := range parts[:len(parts)-1] {
		i := slices.IndexFunc(modNames, func(n modName) bool { return n.name == m })
		if i < 0 {
			return c, fmt.Errorf("%q: no modifier %q (ctrl, shift, alt, meta)", s, m)
		}
		c.mods |= modNames[i].bit
	}
	name := parts[len(parts)-1]
	code, ok := keyCodes[name]
	if num, isNum := strings.CutPrefix(name, "key"); !ok && isNum {
		if n, err := strconv.Atoi(num); err == nil && n > 0 && n < 0x300 {
			code, ok = uint16(n), true // any other, by its Linux code: key183
		}
	}
	if !ok {
		return c, fmt.Errorf("%q: no key %q", s, name)
	}
	c.code = code
	return c, nil
}

// label is c as shown: "Ctrl+Shift+P", "Ctrl+Shift+<".
func (c combo) label() string {
	name := kobo.KeyName(c.code)
	if n, ok := keyLabelNames[c.code]; ok {
		name = n
	}
	if ch, ok := kobo.KeyChar(c.code, false, false); ok && c.mods&modShift != 0 && ch != " " && !isAlnum(ch) {
		name, _ = kobo.KeyChar(c.code, true, false) // Shift+, is <
	}
	return modsLabel(c.mods) + name
}

// String is c as keys.conf has it: "ctrl+shift+p".
func (c combo) String() string {
	name, ok := keyConfNames[c.code]
	if !ok {
		name = fmt.Sprintf("key%d", c.code)
	}
	return strings.ToLower(modsLabel(c.mods)) + name
}

// modsLabel is "Ctrl+Alt+" for those.
func modsLabel(mods uint8) string {
	var b strings.Builder
	for _, m := range modNames {
		if mods&m.bit != 0 {
			b.WriteString(m.label + "+")
		}
	}
	return b.String()
}

func isAlnum(s string) bool { return s >= "a" && s <= "z" || s >= "0" && s <= "9" }

// keyMap is what the keys run, as keys.conf has it.
type keyMap struct {
	bind      map[combo][]*command // the first that can, runs
	keys      map[string][]combo   // by command
	named     map[string]bool      // the commands keys.conf sets, not left at their defaults
	down      map[uint16]bool      // keys pressed for a command, their releases ours too
	mtime     time.Time
	checkedAt time.Time
}

// loadKeys reads keys.conf; the defaults if there's none, which it writes.
func loadKeys() keyMap {
	path := kobo.AppFile(keysTxt)
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(path, []byte(defaultKeysConf()), 0o644); err != nil {
			log.Printf("keys: %v", err)
		}
	}
	m, errs := parseKeys(string(b))
	for _, e := range errs {
		log.Printf("keys: %s", e)
	}
	if fi, err := os.Stat(path); err == nil {
		m.mtime = fi.ModTime()
	}
	return m
}

func parseKeys(conf string) (keyMap, []string) {
	named := map[string]bool{}
	m := keyMap{bind: map[combo][]*command{}, keys: map[string][]combo{}, named: named, down: map[uint16]bool{}}
	var errs []string
	freed := map[combo]bool{}
	add := func(k combo, c *command) {
		if !slices.Contains(m.bind[k], c) {
			m.bind[k] = append(m.bind[k], c)
			m.keys[c.id] = append(m.keys[c.id], k)
		}
	}
	for n, line := range strings.Split(conf, "\n") {
		line, _, _ = strings.Cut(line, "#") // no key is # (it's shift+3)
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		if len(f) != 2 {
			errs = append(errs, fmt.Sprintf("line %d: a key and a command, not %q", n+1, line))
			continue
		}
		if f[0] == "none" {
			if c := commandByID(f[1]); c != nil {
				named[c.id] = true
			} else {
				errs = append(errs, fmt.Sprintf("line %d: no command %q", n+1, f[1]))
			}
			continue
		}
		k, err := parseKey(f[0])
		if err != nil {
			errs = append(errs, fmt.Sprintf("line %d: %v", n+1, err))
			continue
		}
		if f[1] == "-" {
			freed[k] = true
			continue
		}
		c := commandByID(f[1])
		if c == nil {
			errs = append(errs, fmt.Sprintf("line %d: no command %q", n+1, f[1]))
			continue
		}
		named[c.id] = true
		add(k, c)
	}
	for _, c := range commands { // after the file's: those come first
		if named[c.id] {
			continue
		}
		for _, s := range c.keys {
			k, err := parseKey(s)
			if err != nil {
				panic(err) // a default: a bug
			}
			if !freed[k] {
				add(k, c)
			}
		}
	}
	return m, errs
}

func defaultKeysConf() string {
	var b strings.Builder
	b.WriteString(`# hellobro's keys. A line is a key and a command, "ctrl+alt+1 tab.1",
# a key and -, "f11 -": the key isn't ours then, a terminal gets it, or
# none and a command, "none session.new": it has no key. A command named
# here loses its default keys; the rest keep theirs. Settings, Keys, sets
# them too, and writes here.
#
# Modifiers: ctrl shift alt meta. Keys: a-z 0-9 f1-f12 tab space enter esc
# backspace up down left right pgup pgdn home end insert delete, the marks
# (minus or -, = [ ] ; ' ` + "`" + ` \ , . /), brightnessup brightnessdown,
# and any other by its Linux code: key183.
# Saved, it's read again at once.
#
# The defaults:
#
`)
	var none []*command
	for _, c := range commands {
		if len(c.keys) == 0 {
			none = append(none, c)
		}
		for _, k := range c.keys {
			fmt.Fprintf(&b, "# %-18s %-16s %s\n", k, c.id, c.title)
		}
	}
	b.WriteString("#\n# With no key, till you give them one:\n#\n")
	for _, c := range none {
		fmt.Fprintf(&b, "# %-18s %-16s %s\n", "", c.id, c.title)
	}
	return b.String()
}

// keysTick reads keys.conf again if it was saved.
func (a *bro) keysTick() {
	m := &a.keys
	if time.Since(m.checkedAt) < 2*time.Second {
		return
	}
	m.checkedAt = time.Now()
	fi, err := os.Stat(kobo.AppFile(keysTxt))
	if err != nil || fi.ModTime().Equal(m.mtime) {
		return
	}
	log.Print("keys: keys.conf changed")
	a.keys = loadKeys()
	a.keys.checkedAt = time.Now()
	a.keysChanged()
}

// keysChanged has what shows the keys show the new ones.
func (a *bro) keysChanged() {
	a.applyTabs()
	if isTermTab(a.Tab) && !a.overlay() {
		a.drawTerm()
	}
	a.sv.shown = ""
	a.drawSettings()
}

// mods is the modifiers held now.
func (a *bro) mods() uint8 {
	m := &a.tv.mods
	var b uint8
	for _, x := range []struct {
		on  bool
		bit uint8
	}{{m.ctrl, modCtrl}, {m.shift, modShift}, {m.alt, modAlt}, {m.meta, modMeta}} {
		if x.on {
			b |= x.bit
		}
	}
	return b
}

// keyCommand is the command code runs now, with the modifiers held.
func (a *bro) keyCommand(code uint16) *command {
	for _, c := range a.keys.bind[combo{a.mods(), code}] {
		if c.when == nil || c.when(a) {
			return c
		}
	}
	return nil
}

// runKey runs the command for the key, if there's one: true if it did.
func (a *bro) runKey(code uint16, value int32) bool {
	if value == 0 {
		if a.keys.down[code] { // the release of a key that ran one
			delete(a.keys.down, code)
			return true
		}
		return false
	}
	c := a.keyCommand(code)
	if c == nil {
		return false
	}
	a.keys.down[code] = true
	if value == 1 || c.repeat {
		if value == 1 {
			log.Printf("key: %s", c.id)
		}
		c.run(a)
	}
	return true
}

// keyLabel is the first key for command id, as shown; "" if none.
func (a *bro) keyLabel(id string) string {
	if ks := a.keys.keys[id]; len(ks) > 0 {
		return ks[0].label()
	}
	return ""
}

// keyLabels are all its keys.
func (a *bro) keyLabels(id string) string {
	var ls []string
	for _, k := range a.keys.keys[id] {
		ls = append(ls, k.label())
	}
	return strings.Join(ls, ", ")
}

// keyHints joins "key: what" for the commands that have a key.
func (a *bro) keyHints(pairs ...string) string {
	var hs []string
	for i := 0; i+1 < len(pairs); i += 2 {
		if k := a.keyLabel(pairs[i]); k != "" {
			hs = append(hs, k+": "+pairs[i+1])
		}
	}
	return strings.Join(hs, "  ·  ")
}

// --- what the commands do ---

const lightStep = 10

func (a *bro) stepLight(d int) {
	if l, ok := a.Light(); ok {
		a.SetLight(l + d)
	}
}

// toggleLight turns the light off, or back on as it was.
func (a *bro) toggleLight() {
	l, ok := a.Light()
	switch {
	case !ok:
	case l > 0:
		a.lightWas = l
		a.SetLight(0)
	default:
		a.SetLight(max(lightStep, a.lightWas))
	}
}

func (a *bro) stepTab(d int) {
	tabs := a.Tabs()
	if i := slices.Index(tabs, a.Tab); i >= 0 {
		a.SwitchTab(tabs[(i+d+len(tabs))%len(tabs)])
	}
}

// stepSession moves the focus to the next session (d -1: the one before)
// whose panel is open; with them all folded, to the next one, opened.
func (a *bro) stepSession(d int) {
	g := a.termGroup()
	if g == nil || len(g.sessions) < 2 {
		return
	}
	n := len(g.sessions)
	open := a.termOpen(a.Tab) > 0
	for k := 1; k < n; k++ {
		if j := (g.cur + d*k + n*k) % n; !open || !g.sessions[j].folded {
			a.switchSession(j)
			return
		}
	}
}

// radioToggle turns Wi-Fi or Bluetooth on or off, as it is now (which
// the panel may not know yet: it's read first). Settings shows it going.
func (a *bro) radioToggle(panel int) {
	a.SwitchTab(tabRadio)
	a.radioStart()
	r := &a.rs
	wifi, bt := r.wifi, r.bt
	go func() {
		ws := wifi.Status()
		bs, _ := bt.Status()
		a.Do(func() {
			r.ws, r.bs = ws, bs
			if panel == panelWiFi {
				a.wifiPower()
			} else {
				a.btPower()
			}
		})
	}()
}
