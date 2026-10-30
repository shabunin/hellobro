package main

// Commands: what hellobro does on a key, or from the switcher. Each has
// an id for keys.conf, a title for the switcher, and default keys. The
// window manager's are i3's command lines ("focus left"), and keys.conf
// takes any of those, as i3's bindsym does.
//
// keys.conf, next to hellobro, changes them: a line is a key and a
// command ("meta+1 workspace number 1"), a key and "-", which frees it
// (a terminal gets it then), or "none" and a command, which has no key
// then. A command named there loses its defaults. "[resize]" starts the
// keys of a binding mode, i3's: in a mode only its keys count, till one
// runs "mode default". The file is written the first time, every default
// in it commented out, and read again when it's saved; Settings' Keys row
// writes it too (keyset.go).

import (
	"errors"
	"fmt"
	"log"
	"maps"
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
	keys      []string          // the defaults
	mode      string            // the binding mode they're in: "" is the default
	when      func(*bro) bool   // nil: always
	detail    func(*bro) string // what the switcher says next to the keys, or nil
	run       func(*bro)
	hidden    bool // not in the switcher
	repeat    bool // held, it goes on
}

func onTerm(a *bro) bool { return a.term() != nil }

var commands []*command

func init() { commands = makeCommands() } // the switcher, one of them, lists them

// wmCommand is i3's command line as a command: its id is the line.
func wmCommand(line, title string, repeat bool, keys ...string) *command {
	return &command{id: line, title: title, keys: keys, repeat: repeat, run: func(a *bro) { a.wmRun(line) }}
}

func makeCommands() []*command {
	cs := []*command{
		{id: "launcher", title: "A new window", keys: []string{"meta+enter"}, run: (*bro).openLauncher},
		{id: "palette", title: "Go to…", keys: []string{"meta+d", "meta+space"}, run: (*bro).openPicker, hidden: true},
		wmCommand("kill", "Close the window in focus", false, "meta+shift+q"),
	}
	for _, d := range []struct{ dir, arrow, letter string }{
		{"left", "left", "h"}, {"down", "down", "j"}, {"up", "up", "k"}, {"right", "right", "l"},
	} {
		cs = append(cs,
			wmCommand("focus "+d.dir, "Focus: the window "+d.dir+"wards", true, "meta+"+d.arrow, "meta+"+d.letter),
			wmCommand("move "+d.dir, "Move the window "+d.dir, true, "meta+shift+"+d.arrow, "meta+shift+"+d.letter))
	}
	cs = append(cs,
		wmCommand("focus next", "Focus: the next window", true, "meta+tab"),
		wmCommand("focus prev", "Focus: the window before", true, "meta+shift+tab"),
		wmCommand("focus parent", "Focus: the container around", false, "meta+a"),
		wmCommand("focus child", "Focus: back into the container", false),
		wmCommand("focus mode_toggle", "Focus: floating / tiled windows", false),
		wmCommand("split h", "Split: the next window to the right", false, "meta+b"),
		wmCommand("split v", "Split: the next window below", false, "meta+v"),
		wmCommand("layout toggle split", "Layout: side by side / one above another", false, "meta+e"),
		wmCommand("layout stacking", "Layout: stacked", false, "meta+s"),
		wmCommand("layout tabbed", "Layout: tabbed", false, "meta+w"),
		wmCommand("fullscreen toggle", "Fullscreen: the window in focus", false, "meta+f"),
		&command{id: "grab", title: "Grab: every key to the window, till this again", keys: []string{"meta+shift+g"}, run: (*bro).toggleGrab},
		wmCommand("floating toggle", "Floating: the window in focus", false, "meta+shift+space"),
		wmCommand("move scratchpad", "Put the window away (the scratchpad)", false, "meta+shift+minus"),
		wmCommand("scratchpad show", "Bring back a window put away", false, "meta+minus"),
		wmCommand("mode resize", "Resize the windows: the arrows", false, "meta+r"),
		wmCommand("workspace back_and_forth", "Workspace: the one before", false),
		wmCommand("workspace next", "Workspace: the next", false),
		wmCommand("workspace prev", "Workspace: the one before in the bar", false),
	)
	for n := 1; n <= 10; n++ {
		k := strconv.Itoa(n % 10)
		ws := wmCommand(fmt.Sprintf("workspace number %d", n), fmt.Sprintf("Workspace %d", n), false, "meta+"+k)
		mv := wmCommand(fmt.Sprintf("move container to workspace number %d", n), fmt.Sprintf("Move the window to workspace %d", n), false, "meta+shift+"+k)
		ws.hidden, mv.hidden = true, true
		cs = append(cs, ws, mv)
	}
	// The resize mode's.
	for _, d := range []struct{ how, title, arrow, letter string }{
		{"shrink width", "narrower", "left", "h"}, {"grow height", "taller", "down", "j"},
		{"shrink height", "shorter", "up", "k"}, {"grow width", "wider", "right", "l"},
	} {
		c := wmCommand("resize "+d.how+" 50 px or 5 ppt", "Resize: "+d.title, true, d.arrow, d.letter)
		c.mode, c.hidden = "resize", true
		cs = append(cs, c)
	}
	back := wmCommand("mode default", "Resize: done", false, "enter", "esc", "meta+r")
	back.mode, back.hidden = "resize", true
	cs = append(cs, back)

	cs = append(cs,
		&command{id: "copy", title: "Copy the selection", keys: []string{"ctrl+shift+c"}, when: onTerm, run: (*bro).copySel},
		&command{id: "copy.screen", title: "Copy the screen's text", when: onTerm, run: func(a *bro) {
			if t := a.term(); t != nil && t.connected() {
				a.copyText(a.screenText(t), "")
			}
		}},
		&command{id: "paste", title: "Paste", keys: []string{"ctrl+shift+v"}, run: func(a *bro) { a.paste(readClip(), "") }},
		&command{id: "copy.key", title: "Copy the Kobo's SSH key", run: (*bro).copyKoboKey},
		&command{id: "paste.key", title: "Paste the Kobo's SSH key", run: (*bro).pasteKoboKey},
		&command{id: "session.end", title: "Terminal in focus: disconnect or kill", when: onTerm, run: (*bro).endSession},
		&command{id: "font.bigger", title: "Bigger font", keys: []string{"ctrl+="}, when: onTerm, repeat: true,
			run: func(a *bro) { a.termFontBy(termFontStep) }},
		&command{id: "font.smaller", title: "Smaller font", keys: []string{"ctrl+-"}, when: onTerm, repeat: true,
			run: func(a *bro) { a.termFontBy(-termFontStep) }},
		&command{id: "font.reset", title: "Font size back", keys: []string{"ctrl+0"}, when: onTerm,
			run: func(a *bro) { a.termFontBy(0) }},
		&command{id: "music.play", title: "Play / pause (the Linux player)", keys: []string{"ctrl+shift+space"},
			run: func(a *bro) { a.mediaKey(kobo.KeyPlayPause) }},
		&command{id: "music.next", title: "Next track", keys: []string{"ctrl+shift+."},
			run: func(a *bro) { a.mediaKey(kobo.KeyNextSong) }},
		&command{id: "music.prev", title: "Previous track", keys: []string{"ctrl+shift+,"},
			run: func(a *bro) { a.mediaKey(kobo.KeyPreviousSong) }},
		&command{id: "music.stop", title: "Stop the music", run: func(a *bro) { a.mediaKey(kobo.KeyStopCD) }},
		&command{id: "music.louder", title: "Louder (the player's volume)", repeat: true,
			run: func(a *bro) { a.mediaKey(kobo.KeyVolumeUp) }},
		&command{id: "music.quieter", title: "Quieter (the player's volume)", repeat: true,
			run: func(a *bro) { a.mediaKey(kobo.KeyVolumeDown) }},
		&command{id: "keyboard", title: "On-screen keyboard: show / hide", run: (*bro).vkToggle},
		&command{id: "language", title: "Keyboard: type in the next language (Settings, Languages)", keys: []string{"ctrl+alt+space"},
			run: func(a *bro) { a.typeIn(a.vk.lang + 1) }},
		&command{id: "books", title: "Books: the shelf, to read one", run: (*bro).openShelf},
		&command{id: "notebooks", title: "Notebooks: all of them, to open, rename, put in folders", run: (*bro).openNotes},
		&command{id: "statusbar", title: "Status bar: show / hide", keys: []string{"ctrl+shift+b"}, run: (*bro).toggleStat},
		&command{id: "sleep", title: "Sleep", keys: []string{"meta+shift+s"}, run: func(a *bro) { a.App.Sleep() }},
		&command{id: "lock", title: "Lock the screen", keys: []string{"meta+shift+x"}, run: (*bro).lock},
		&command{id: "wifi", title: "Wi-Fi on / off", run: func(a *bro) { a.radioToggle(panelWiFi) }},
		&command{id: "bluetooth", title: "Bluetooth on / off", run: func(a *bro) { a.radioToggle(panelBT) }},
		&command{id: "airplane", title: "Airplane mode: Wi-Fi and Bluetooth off (again: back on)", run: (*bro).airplane},
		&command{id: "wifi.scan", title: "Wi-Fi: look for networks", run: onRadio("wifi", func(a *bro) { a.radioPoll(true) })},
		&command{id: "wifi.join", title: "Wi-Fi: join a network by its name", run: onRadio("wifi", func(a *bro) {
			a.radioAsk(&radioPrompt{panel: panelWiFi, kind: "ssid", text: "The network's name (SSID):"})
		})},
		&command{id: "wifi.info", title: "Wi-Fi: the network's details", run: onRadio("wifi", (*bro).wifiInfo)},
		&command{id: "bt.scan", title: "Bluetooth: look for devices (again: stop)", run: onRadio("bluetooth", (*bro).btScan)},
		&command{id: "sharing", title: "File sharing on / off", run: (*bro).shareToggle},
		&command{id: "sharing.pair", title: "Pair a computer", run: (*bro).sharePair},
		&command{id: "relay", title: "Relay on / off", run: func(a *bro) { a.stepSetting(settingIndex("relay"), 1) }},
		&command{id: "keys", title: "Keys: set a command's", run: func(a *bro) { a.openKeys(nil) }},
		wmCommand("reload", "Keys: read keys.conf again", false, "meta+shift+c"),
	)
	cs = append(cs, settingCommands()...)
	cs = append(cs,
		&command{id: "light.up", title: "Light: brighter", keys: []string{"meta+]", "brightnessup"}, repeat: true,
			run: func(a *bro) { a.stepLight(lightStep) }},
		&command{id: "light.down", title: "Light: dimmer", keys: []string{"meta+[", "brightnessdown"}, repeat: true,
			run: func(a *bro) { a.stepLight(-lightStep) }},
		&command{id: "light.toggle", title: "Light: on / off (the backlight)", keys: []string{"meta+\\"}, run: (*bro).toggleLight},
		&command{id: "rotate", title: "Rotate the screen", run: func(a *bro) { a.Rotate() }},
	)
	for r, name := range []string{"Portrait", "Landscape", "Portrait, upside down", "Landscape, the other way"} {
		cs = append(cs, &command{id: "screen." + []string{"portrait", "landscape", "portrait2", "landscape2"}[r], title: "Screen: " + name,
			when: func(a *bro) bool { return a.Rotation() != r }, run: func(a *bro) { a.SetRotation(r) }})
	}
	cs = append(cs,
		&command{id: "calibrate", title: "Touch calibration", run: func(a *bro) { a.Calibrate() }},
		&command{id: "quit", title: "Quit, back to Nickel", keys: []string{"meta+shift+e"}, run: func(a *bro) { a.Quit() }}, // i3's exit
	)
	return cs
}

// onRadio has do done in the settings' window key, which it opens (and
// that starts the radios).
func onRadio(key string, do func(*bro)) func(*bro) {
	return func(a *bro) {
		a.openGUI(key)
		do(a)
	}
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

// modIsAlt says Alt is the window keys' modifier (Settings, "Mod key"),
// as i3's "set $mod Mod1": keys.conf's meta is then Alt, and its alt the
// Win key. What goes to a terminal is the keys as they are.
var modIsAlt bool

// swapMod swaps meta and alt in mods, when Alt is the Mod key.
func swapMod(mods uint8) uint8 {
	if !modIsAlt {
		return mods
	}
	m := mods &^ (modAlt | modMeta)
	if mods&modAlt != 0 {
		m |= modMeta
	}
	if mods&modMeta != 0 {
		m |= modAlt
	}
	return m
}

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
	return modsLabel(swapMod(c.mods)) + name
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

// modsList is mods as i3 names them: "ctrl", "shift", "mod1" (Alt),
// "mod4" (Meta).
func modsList(mods uint8) []string {
	ms := []string{}
	for _, m := range []struct {
		bit  uint8
		name string
	}{{modShift, "shift"}, {modCtrl, "ctrl"}, {modAlt, "mod1"}, {modMeta, "mod4"}} {
		if mods&m.bit != 0 {
			ms = append(ms, m.name)
		}
	}
	return ms
}

func isAlnum(s string) bool { return s >= "a" && s <= "z" || s >= "0" && s <= "9" }

// keyMap is what the keys run, as keys.conf has it.
type keyMap struct {
	bind      map[combo][]*command            // the default mode's: the first that can, runs
	modes     map[string]map[combo][]*command // the other modes', by name
	keys      map[string][]combo              // by command, in the default mode
	named     map[string]bool                 // the commands keys.conf sets, not left at their defaults
	down      map[uint16]bool                 // keys pressed for a command, their releases ours too
	mtime     time.Time
	checkedAt time.Time
}

// loadKeys reads keys.conf; the defaults if there's none, which it writes.
// One with nothing but comments is the defaults of a hellobro before:
// it's written again, with these.
func loadKeys() keyMap {
	path := kobo.AppFile(keysTxt)
	b, err := os.ReadFile(path)
	if def := defaultKeysConf(); errors.Is(err, os.ErrNotExist) || err == nil && string(b) != def && onlyComments(string(b)) {
		if err := os.WriteFile(path, []byte(def), 0o644); err != nil {
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

func onlyComments(conf string) bool {
	for _, line := range strings.Split(conf, "\n") {
		if line, _, _ = strings.Cut(line, "#"); strings.TrimSpace(line) != "" {
			return false
		}
	}
	return true
}

// keyIDs are commands' ids as they were once, by what they are now.
var keyIDs = map[string]string{
	"session.new": "launcher", "fullscreen": "fullscreen toggle",
	"session.next": "focus next", "session.prev": "focus prev", "panel.next": "focus next", "panel.prev": "focus prev",
	"tab.next": "workspace next", "tab.prev": "workspace prev",
}

// keyID is the command a keys.conf line names: one of ours, or i3's, which
// the window manager runs. "" if there's none.
func keyID(line string) (*command, string) {
	if now, ok := keyIDs[line]; ok {
		line = now
	}
	if n, ok := strings.CutPrefix(line, "tab."); ok {
		if _, err := strconv.Atoi(n); err == nil {
			line = "workspace number " + n
		}
	}
	if c := commandByID(line); c != nil {
		return c, line
	}
	w, _, _ := strings.Cut(line, " ")
	if !wmWords[w] && !strings.HasPrefix(w, "[") {
		return nil, line
	}
	return wmCommand(line, line, false), line
}

// wmWords start i3's commands.
var wmWords = map[string]bool{
	"nop": true, "exec": true, "mode": true, "kill": true, "focus": true, "split": true, "layout": true,
	"floating": true, "fullscreen": true, "border": true, "workspace": true, "rename": true, "move": true,
	"swap": true, "resize": true, "mark": true, "unmark": true, "scratchpad": true, "exit": true,
	"reload": true, "open": true,
}

// keyName is how named has command id in mode.
func keyName(mode, id string) string {
	if mode == "" {
		return id
	}
	return mode + ":" + id
}

func parseKeys(conf string) (keyMap, []string) {
	named := map[string]bool{}
	m := keyMap{bind: map[combo][]*command{}, modes: map[string]map[combo][]*command{}, keys: map[string][]combo{},
		named: named, down: map[uint16]bool{}}
	var errs []string
	freed := map[string]bool{} // mode and key
	add := func(mode string, k combo, c *command) {
		b := m.bind
		if mode != "" {
			if m.modes[mode] == nil {
				m.modes[mode] = map[combo][]*command{}
			}
			b = m.modes[mode]
		}
		if !slices.ContainsFunc(b[k], func(o *command) bool { return o.id == c.id }) {
			b[k] = append(b[k], c)
			if mode == "" {
				m.keys[c.id] = append(m.keys[c.id], k)
			}
		}
	}
	mode := ""
	for n, line := range strings.Split(conf, "\n") {
		line, _, _ = strings.Cut(line, "#") // no key is # (it's shift+3)
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		if name, ok := strings.CutPrefix(f[0], "["); ok && len(f) == 1 && strings.HasSuffix(name, "]") {
			mode = strings.TrimSuffix(name, "]")
			if mode == "default" {
				mode = ""
			}
			if mode != "" && m.modes[mode] == nil {
				m.modes[mode] = map[combo][]*command{}
			}
			continue
		}
		if len(f) < 2 {
			errs = append(errs, fmt.Sprintf("line %d: a key and a command, not %q", n+1, line))
			continue
		}
		c, id := keyID(strings.Join(f[1:], " "))
		if f[0] == "none" {
			if c != nil {
				named[keyName(mode, c.id)] = true
			} else {
				errs = append(errs, fmt.Sprintf("line %d: no command %q", n+1, id))
			}
			continue
		}
		k, err := parseKey(f[0])
		if err != nil {
			errs = append(errs, fmt.Sprintf("line %d: %v", n+1, err))
			continue
		}
		if id == "-" {
			freed[keyName(mode, k.String())] = true
			continue
		}
		if c == nil {
			errs = append(errs, fmt.Sprintf("line %d: no command %q", n+1, id))
			continue
		}
		named[keyName(mode, c.id)] = true
		add(mode, k, c)
	}
	for _, c := range commands { // after the file's: those come first
		if named[keyName(c.mode, c.id)] {
			continue
		}
		for _, s := range c.keys {
			k, err := parseKey(s)
			if err != nil {
				panic(err) // a default: a bug
			}
			if !freed[keyName(c.mode, k.String())] {
				add(c.mode, k, c)
			}
		}
	}
	return m, errs
}

// keyModes are the binding modes keys.conf and the defaults have.
func (a *bro) keyModes() []string {
	return slices.Sorted(maps.Keys(a.keys.modes))
}

func defaultKeysConf() string {
	var b strings.Builder
	b.WriteString(`# hellobro's keys. A line is a key and a command, "meta+1 workspace number 1",
# a key and -, "f11 -": the key isn't ours then, a terminal gets it, or
# none and a command, "none launcher": it has no key. A command named
# here loses its default keys; the rest keep theirs. The window manager's
# commands are i3's, any of them: "meta+t layout tabbed; focus parent".
# Settings, Keys, sets them too, and writes here.
#
# [name] starts a binding mode's keys (i3's "mode"): while it's on, only
# those count. [default] goes back to the default's.
#
# Modifiers: ctrl shift alt meta (the Win key; Alt, with Settings' Mod key
# Alt, and then alt is the Win key). Keys: a-z 0-9 f1-f12 tab
# space enter esc backspace up down left right pgup pgdn home end insert
# delete, the marks (minus or -, = [ ] ; ' ` + "`" + ` \ , . /), brightnessup
# brightnessdown, and any other by its Linux code: key183.
# Saved, it's read again at once.
#
# The defaults:
#
`)
	line := func(k string, c *command) {
		fmt.Fprintf(&b, "# %-18s %-38s # %s\n", k, c.id, c.title)
	}
	var none []*command
	modes := map[string][]*command{}
	for _, c := range commands {
		if c.mode != "" {
			modes[c.mode] = append(modes[c.mode], c)
			continue
		}
		if len(c.keys) == 0 {
			none = append(none, c)
		}
		for _, k := range c.keys {
			line(k, c)
		}
	}
	b.WriteString("#\n# With no key, till you give them one:\n#\n")
	for _, c := range none {
		line("none", c)
	}
	for _, mode := range slices.Sorted(maps.Keys(modes)) {
		fmt.Fprintf(&b, "#\n# [%s]\n", mode)
		for _, c := range modes[mode] {
			for _, k := range c.keys {
				line(k, c)
			}
		}
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
	if a.wm.mode != "" && a.keys.modes[a.wm.mode] == nil {
		a.setMode("")
	}
	a.applyFooter()
	a.wm.names = nil // the workspaces' keys
	a.wmBar()
	if !a.overlay() && !a.Calibrating() {
		a.DrawPage()
		a.MarkFlash(a.lay.Page)
	}
}

// applyFooter has the footer say the workspaces', sleep's and quit's keys.
func (a *bro) applyFooter() {
	sleep := a.keyLabel("sleep")
	if sleep == "" {
		sleep = "-"
	}
	quit := a.keyLabel("quit")
	if quit == "" {
		quit = "-"
	}
	a.SetFooter(a.keys.footer(), sleep, quit)
}

// footer is what the footer says of the workspaces' keys.
func (m keyMap) footer() string {
	one, nine := m.keys["workspace number 1"], m.keys["workspace number 9"]
	if len(one) > 0 && len(nine) > 0 && one[0].mods == nine[0].mods && one[0].code == kobo.Key1 && nine[0].code == kobo.Key1+8 {
		return "workspaces: tap or " + modsLabel(swapMod(one[0].mods)) + "1–9"
	}
	return "workspaces: tap"
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
	return swapMod(b)
}

// keyCommand is the command code runs now, with the modifiers held, in
// the binding mode on.
func (a *bro) keyCommand(code uint16) *command {
	b := a.keys.bind
	if a.wm.mode != "" {
		b = a.keys.modes[a.wm.mode]
	}
	for _, c := range b[combo{a.mods(), code}] {
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
			a.ipcSend("binding", map[string]any{"change": "run", "binding": map[string]any{
				"command": c.id, "event_state_mask": modsList(a.mods()), "input_code": 0,
				"symbol": combo{code: code}.String(), "input_type": "keyboard"}})
		}
		c.run(a)
	}
	return true
}

// keyLabel is the first key for command id, as shown; "" if none.
func (a *bro) keyLabel(id string) string {
	if ks := a.keys.reach(id); len(ks) > 0 {
		return ks[0].label()
	}
	return ""
}

// keyLabels are all its keys.
func (a *bro) keyLabels(id string) string {
	var ls []string
	for _, k := range a.keys.reach(id) {
		ls = append(ls, k.label())
	}
	return strings.Join(ls, ", ")
}

// reach is id's keys (the default mode's) that run it: not those another
// command has first, which always can (keys.conf's "meta+left workspace
// prev" over focus left's default).
func (m keyMap) reach(id string) []combo {
	var ks []combo
	for _, k := range m.keys[id] {
		if b := m.bind[k]; len(b) > 0 && b[0].id != id && b[0].when == nil {
			continue
		}
		ks = append(ks, k)
	}
	return ks
}

// keysOf is the first keys of ids, as one: "Meta+Left/Down/Up/Right"
// where they differ in the key alone, "Meta+1…0" for more than four of
// those; or each, after commas.
func (a *bro) keysOf(ids ...string) string {
	var ks []string
	for _, id := range ids {
		if k := a.keyLabel(id); k != "" {
			ks = append(ks, k)
		}
	}
	if len(ks) < 2 {
		return strings.Join(ks, "")
	}
	pre := ks[0][:strings.LastIndex(ks[0][:len(ks[0])-1], "+")+1] // the last char may be a "+" key
	var rest []string
	for _, k := range ks {
		r, ok := strings.CutPrefix(k, pre)
		if !ok || pre == "" || r == "" || strings.Contains(r[:len(r)-1], "+") {
			return strings.Join(ks, ", ")
		}
		rest = append(rest, r)
	}
	if len(rest) > 4 {
		return pre + rest[0] + "…" + rest[len(rest)-1]
	}
	return pre + strings.Join(rest, "/")
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
		a.mem.LightOn = l // Nickel's, maybe, which LightKept never heard of; saved with the 0
		a.SetLight(0)
	default:
		a.SetLight(max(lightStep, a.mem.LightOn))
	}
}

// radioToggle turns Wi-Fi or Bluetooth on or off, as it is now (which
// the window may not know yet: it's read first). Its window shows it going.
func (a *bro) radioToggle(panel int) {
	a.openGUI([]string{"wifi", "bluetooth"}[panel])
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
