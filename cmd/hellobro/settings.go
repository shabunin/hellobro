package main

// The Settings tab's third panel, under Wi-Fi and Bluetooth: how hellobro
// sleeps, sharing, the status bar and the tabs. A tap on a setting (or Enter, ←/→) steps
// through its values; they're kept in the settings file.

import (
	"fmt"
	"image"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"hellobro/kobo"
)

const (
	panelSet  = 3 // the settings panel, after Wi-Fi, Bluetooth and Sharing
	setRowH   = 50
	setRowMin = 40
	setTxt    = "settings"
)

// setting is one line of the panel: its values as saved, and as shown.
// A label with a %s gets the key of command cmd there.
type setting struct {
	key, label, cmd string
	title           string // in the switcher, if not the label
	group           string // a heading in the panel, before the first of each
	values          []string
	shown           []string
}

var settingList = []setting{
	{key: "light-idle", group: "Light", label: "Light off when idle",
		values: []string{"10", "30", "60", "300", "off"},
		shown:  []string{"after 10 s untouched", "after 30 s untouched", "after 1 min untouched", "after 5 min untouched", "stays on"}},
	{key: "light-start", label: "Light at start",
		values: []string{"last", "nickel"},
		shown:  []string{"as you last set it", "as Nickel had it"}},
	{key: "doze-after", group: "Sleep and music", label: "Light sleep while music plays",
		values: []string{"off", "1", "2", "5", "10", "20"},
		shown:  []string{"never by itself", "after 1 min idle", "after 2 min idle", "after 5 min idle", "after 10 min idle", "after 20 min idle"}},
	{key: "sleep-music", label: "Sleep%s with music on", cmd: "sleep",
		values: []string{"light", "deep"},
		shown:  []string{"light: the music plays on", "deep: the music pauses"}},
	{key: "doze-wifi", label: "Wi-Fi in light sleep",
		values: []string{"off", "on"},
		shown:  []string{"off after 3 min (the battery lasts)", "stays on"}},
	{key: "doze-deep", label: "Music stopped in light sleep",
		values: []string{"5", "10", "30", "off"},
		shown:  []string{"deep sleep after 5 min", "deep sleep after 10 min", "deep sleep after 30 min", "stay in light sleep"}},
	{key: "nickel", group: "At start", label: "Nickel while hellobro runs",
		values: []string{"frozen", "closed"},
		shown:  []string{"frozen: back to it at once", "closed, as KOReader does (from the next start)"}},
	{key: "radios", label: "Wi-Fi and Bluetooth at start",
		values: []string{"kobo", "on"},
		shown:  []string{"as Kobo had them, and put back so", "turned on, and left on"}},
	{key: "share", group: "Sharing", label: "File sharing (SFTP, port 2022)",
		values: []string{"off", "on", "pair"},
		shown:  []string{"off", "on (Sharing, above)", "pair a computer"}},
	{key: "relay", label: "Relay, a way in from afar (relay.conf)",
		values: []string{"off", "on"},
		shown:  []string{"off", "on, while sharing: no auto-exit"}},
	{key: "mdns", label: "Show the Kobo as kobo.local (mDNS)",
		values: []string{"on", "off"},
		shown:  []string{"yes, while sharing", "no"}},
	{key: "profile", label: "CPU profiles (make push-profile)",
		values: []string{"off", "on"},
		shown:  []string{"off", "on, for a paired computer"}},
	{key: "notify", group: "Terminal", label: "Terminal notifications",
		values: []string{"all", "messages", "off"},
		shown:  []string{"the bell and messages", "messages only (OSC 9, 777)", "off"}},
	{key: "notify-light", label: "… the light, if it's off", title: "Notifications: the light, if it's off",
		values: []string{"1", "3", "off"},
		shown:  []string{"on for 1 s", "on for 3 s", "stays off"}},
	{key: "notify-blink", label: "… the session's strip, if it shows", title: "Notifications: the session's strip, if it shows",
		values: []string{"3", "1", "5", "off"},
		shown:  []string{"blinks 3 times", "blinks once", "blinks 5 times", "stays still"}},
	{key: "notify-note", label: "… a note in the corner, if it doesn't", title: "Notifications: a note in the corner, if it doesn't",
		values: []string{"3", "5", "10", "off"},
		shown:  []string{"for 3 s", "for 5 s", "for 10 s", "none"}},
	{key: "status-bar", label: "Status bar%s", cmd: "statusbar",
		values: []string{"on", "off"},
		shown:  []string{"shown on the terminals", "hidden"}},
	{key: "shell", group: "Shell", label: "Shell tab runs",
		values: []string{"kobo", "alpine"},
		shown:  []string{"the Kobo's /bin/sh, as root", "Alpine Linux, in a jail"}},
	{key: "alpine-bus", label: "… Alpine: system bus access", title: "Alpine: system bus access",
		values: []string{"off", "on"},
		shown:  []string{"off", "on: e.g. sound (it reaches Bluetooth and Wi-Fi setup too)"}},
	{key: "alpine-remove", label: "… Alpine: remove it", title: "Alpine: remove it",
		values: []string{""}, shown: []string{""}}, // the question: alpine.go
	{key: "tabs", group: "Tabs and keys", label: "Tabs", values: []string{""}, shown: []string{""}}, // the Tabs dialog: tabs.go
	{key: "keys", label: "Keys", values: []string{""}, shown: []string{""}},                         // the Keys menu: keyset.go
}

// setLine is a line of the panel: a group's heading, or setting i.
type setLine struct {
	head string
	i    int
}

// settingLines are the panel's lines: each group's heading, then its
// settings.
var settingLines = func() []setLine {
	var ls []setLine
	for i, st := range settingList {
		if st.group != "" {
			ls = append(ls, setLine{head: st.group, i: -1})
		}
		ls = append(ls, setLine{i: i})
	}
	return ls
}()

// settingLine is the line setting i is on.
func settingLine(i int) int {
	for l, sl := range settingLines {
		if sl.head == "" && sl.i == i {
			return l
		}
	}
	return 0
}

// settingRenamed are keys as they were once, by what they are now.
var settingRenamed = map[string]string{"alpine-sound": "alpine-bus"}

// The defaults, by key: the index of a value.
var settingDefault = map[string]int{"doze-after": 3, "doze-deep": 1, "light-idle": 1}

// settingCommands step the settings from the switcher, each to its next
// value, as a tap on its row does, and a toast says what it is now. The
// ones with commands of their own (sharing, the relay, the status bar,
// tabs, keys) are left to those.
func settingCommands() []*command {
	var cs []*command
	for i, st := range settingList {
		switch st.key {
		case "share", "relay", "status-bar", "tabs", "keys":
			continue
		case "alpine-remove":
			cs = append(cs, &command{id: "alpine.remove", title: "Alpine: remove it…", run: (*bro).alpineRemoveAsk})
			continue
		}
		title := st.title
		if title == "" {
			title = strings.ReplaceAll(st.label, "%s", "")
		}
		shown := func(a *bro) string { return st.shown[a.set[st.key]] }
		cs = append(cs, &command{id: "set." + st.key, title: "Setting: " + title, detail: shown, run: func(a *bro) {
			a.stepSetting(i, 1)
			a.Toast(title+": "+shown(a), 4*time.Second)
		}})
	}
	return cs
}

func settingIndex(key string) int {
	for i, st := range settingList {
		if st.key == key {
			return i
		}
	}
	return -1
}

type settingsView struct {
	p     kobo.Panel
	rows  []image.Rectangle
	hint  image.Rectangle
	sel   int
	top   int // the first row shown, when not all fit
	shown string
}

type settings map[string]int // by key, the value's index

func loadSettings() settings {
	s := settings{}
	for k, v := range settingDefault {
		s[k] = v
	}
	b, _ := os.ReadFile(kobo.AppFile(setTxt))
	for _, line := range strings.Split(string(b), "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		if now, ok := settingRenamed[k]; ok {
			k = now
		}
		for _, st := range settingList {
			if st.key != k {
				continue
			}
			for i, sv := range st.values {
				if sv == v {
					s[k] = i
				}
			}
		}
	}
	return s
}

func (s settings) save() {
	var b strings.Builder
	for _, st := range settingList {
		if st.key != "status-bar" && st.key != "tabs" && st.key != "keys" && st.key != "alpine-remove" { // kept in their own files, or none
			fmt.Fprintf(&b, "%s=%s\n", st.key, st.values[s[st.key]])
		}
	}
	if err := os.WriteFile(kobo.AppFile(setTxt), []byte(b.String()), 0o644); err != nil {
		log.Printf("settings: %v", err)
	}
}

// setting is the value of key, as saved.
func (a *bro) setting(key string) string {
	for _, st := range settingList {
		if st.key == key {
			switch key {
			case "status-bar":
				return map[bool]string{false: "on", true: "off"}[a.stat.off]
			case "tabs":
				return a.tabs.String()
			case "keys":
				return fmt.Sprint(a.keys.named)
			}
			return st.values[a.set[key]]
		}
	}
	return ""
}

// settingMinutes is key's value as a time; 0 for off.
func (a *bro) settingMinutes(key string) time.Duration {
	n, _ := strconv.Atoi(a.setting(key))
	return time.Duration(n) * time.Minute
}

// stepSetting moves setting i on (or back) a value.
func (a *bro) stepSetting(i, d int) {
	st := settingList[i]
	switch st.key {
	case "tabs":
		a.openTabs()
		return
	case "keys":
		a.openKeys(nil)
		return
	case "alpine-remove":
		a.alpineRemoveAsk()
		return
	case "status-bar":
		a.toggleStat()
	case "share": // off, on, pair: pairing turns it on and opens pairing, and stays "on"
		was := a.set[st.key]
		n := len(st.values)
		next := (was + d + n) % n
		if st.values[next] == "pair" {
			a.sharePair()
			break
		}
		a.set[st.key] = next
		a.set.save()
		a.shareApply()
	default:
		n := len(st.values)
		a.set[st.key] = (a.set[st.key] + d + n) % n
		a.set.save()
		switch st.key {
		case "mdns", "relay":
			a.shareApply()
		case "profile":
			profileOn.Store(a.setting(st.key) == "on")
		case "alpine-bus":
			alpineBus.Store(a.setting(st.key) == "on")
		case "radios":
			a.rs.onDone = false
			a.radiosOn()
		case "light-idle":
			a.applyLightIdle()
		}
	}
	log.Printf("settings: %s = %s", st.key, a.setting(st.key))
	a.drawSettings()
}

func (a *bro) layoutSettings(K kobo.Layout, r image.Rectangle, folded bool) {
	v := &a.sv
	v.p = a.pagePanel(K, "Settings", r, folded)
	b := v.p.Body.Inset(kobo.Pad)
	v.rows = v.rows[:0]
	if folded {
		v.hint, v.shown = image.Rectangle{}, ""
		return
	}
	h := max(setRowMin, min(setRowH, (b.Dy()-40)/len(settingLines))) // closer, if that makes room for all
	for i := range settingLines {
		y := b.Min.Y + i*h
		if y+h > b.Max.Y-40 {
			break // no room: a short screen
		}
		v.rows = append(v.rows, image.Rect(b.Min.X, y, b.Max.X, y+h))
	}
	v.hint = image.Rect(b.Min.X, b.Max.Y-40, b.Max.X, b.Max.Y)
	v.shown = ""
}

func (a *bro) drawSettings() {
	if !a.showingRadio() {
		return
	}
	v, c := &a.sv, a.C
	focused := a.rs.focus == panelSet
	var vals []string
	for _, st := range settingList {
		vals = append(vals, a.setting(st.key))
	}
	n, sl := len(v.rows), settingLine(v.sel)
	top := sl // the selected one in sight, and its group's heading if that's right above
	if sl > 0 && settingLines[sl-1].head != "" {
		top--
	}
	if top < v.top {
		v.top = top
	}
	if sl >= v.top+n {
		v.top = sl - n + 1
	}
	v.top = max(0, min(v.top, len(settingLines)-n))
	key := fmt.Sprint(vals, v.sel, v.top, n, focused, a.share.code)
	if key == v.shown {
		return
	}
	v.shown = key
	p := v.p
	if focused {
		p.Name = "► " + p.Name
	}
	a.DrawPanel(p, "")
	for k, rr := range v.rows {
		if l := settingLines[v.top+k]; l.head != "" {
			t := rr.Inset(4)
			t.Min.X += 12
			c.Text(a.F.Small, t, l.head, kobo.Dark, kobo.AlignLeft)
			c.Fill(image.Rect(t.Min.X, rr.Max.Y-6, t.Max.X, rr.Max.Y-4), kobo.Mid)
			continue
		}
		i := settingLines[v.top+k].i
		st := settingList[i]
		fg, dim := uint8(kobo.Black), uint8(kobo.Dark)
		if i == v.sel {
			if focused {
				c.Fill(rr, kobo.Black)
				fg, dim = kobo.White, kobo.Light
			} else {
				c.Frame(rr, 2, kobo.Black)
			}
		}
		t := rr.Inset(4)
		t.Min.X += 12
		t.Max.X -= 12
		shown := st.shown[indexOf(st.values, vals[i])]
		switch {
		case st.key == "share" && a.share.code != "":
			shown = "pairing: code " + a.share.code
		case st.key == "tabs":
			shown = a.tabsSummary()
		case st.key == "alpine-remove":
			shown = alpineSummary()
		case st.key == "keys" && len(a.keys.named) == 0:
			shown = "the defaults (keys.conf)"
		case st.key == "keys":
			shown = fmt.Sprintf("%d set by you (keys.conf)", len(a.keys.named))
		}
		value := "‹ " + shown + " ›"
		vw := kobo.TextWidth(a.F.Small, value)
		c.Text(a.F.Small, image.Rect(t.Max.X-vw, t.Min.Y, t.Max.X, t.Max.Y), value, dim, kobo.AlignRight)
		t.Max.X -= vw + 20
		c.Text(a.F.Body, t, a.settingLabel(st), fg, kobo.AlignLeft)
	}
	hint := "Tap a setting to change it, or ↑ ↓ and ← → (Enter); Ctrl+Tab: the next panel."
	switch {
	case v.top+n < len(settingLines): // a short screen
		hint = fmt.Sprintf("▼ %d more: tap here, or ↓", len(settingLines)-v.top-n)
	case v.top > 0:
		hint = "▲ back to the top: tap here, or ↑"
	}
	c.Text(a.F.Small, v.hint, hint, kobo.Dark, kobo.AlignLeft)
	a.Mark(p.Outer, true)
}

// settingLabel is st's label, with its key in.
func (a *bro) settingLabel(st setting) string {
	if st.cmd == "" {
		return st.label
	}
	k := a.keyLabel(st.cmd)
	if k != "" {
		k = " (" + k + ")"
	}
	return fmt.Sprintf(st.label, k)
}

func indexOf(vs []string, v string) int {
	for i, s := range vs {
		if s == v {
			return i
		}
	}
	return 0
}

// settingsTap changes the setting tapped.
func (a *bro) settingsTap(pt image.Point) {
	v := &a.sv
	if pt.In(v.hint) && len(v.rows) < len(settingLines) { // the rest of a list that doesn't fit, or back
		if v.top+len(v.rows) < len(settingLines) {
			v.top = len(settingLines) - len(v.rows)
		} else {
			v.top = 0
		}
		l := v.top
		for settingLines[l].head != "" {
			l++
		}
		v.sel = settingLines[l].i
		a.drawSettings()
		return
	}
	for k, rr := range v.rows {
		if l := settingLines[v.top+k]; pt.In(rr) && l.head == "" {
			v.sel = l.i
			a.stepSetting(v.sel, 1)
			return
		}
	}
}

// settingsKey takes the keys while the panel has the focus.
func (a *bro) settingsKey(code uint16) {
	v := &a.sv
	switch code {
	case keyUp:
		v.sel = max(0, v.sel-1)
	case keyDown:
		v.sel = min(len(settingList)-1, v.sel+1)
	case keyPgUp:
		v.sel = max(0, v.sel-len(v.rows))
	case keyPgDn:
		v.sel = min(len(settingList)-1, v.sel+len(v.rows))
	case keyHome:
		v.sel = 0
	case keyEnd:
		v.sel = len(settingList) - 1
	case keyLeft:
		a.stepSetting(v.sel, -1)
		return
	case keyRight, kobo.KeyEnter, kobo.KeySpace:
		a.stepSetting(v.sel, 1)
		return
	}
	a.drawSettings()
}

// applyLightIdle tells the App when the light goes off by itself.
func (a *bro) applyLightIdle() {
	n, _ := strconv.Atoi(a.setting("light-idle")) // "off": 0, never
	a.LightIdle = time.Duration(n) * time.Second
}
