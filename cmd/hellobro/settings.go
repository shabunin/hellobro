package main

// The Settings tab's third panel, under Wi-Fi and Bluetooth: how hellobro
// sleeps, sharing, the status bar and the tabs. A tap on a setting (or Enter, ←/→) steps
// through its values; they're kept in the settings file.

import (
	"fmt"
	"log"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"hellobro/ai"
	"hellobro/ink"
	"hellobro/kobo"
)

const (
	panelSet = 3 // the settings panel, after Wi-Fi, Bluetooth and Sharing
	setTxt   = "settings"
)

var setRowH, setRowMin = 50, 40 // sizes.go: taller for fingers

// setting is one line of the panel: its values as saved, and as shown.
// A label with a %s gets the key of command cmd there.
type setting struct {
	key, label, cmd string
	title           string // in the switcher, if not the label
	group           string // a heading in the panel, before the first of each
	help            string // a line under it, saying what it's for
	slider          bool   // its values in order, along a track
	values          []string
	shown           []string
}

var settingList = []setting{
	{key: "light-idle", group: "Light", label: "Light off when idle",
		help:   "the front light goes off by itself; a touch or a key puts it back as it was",
		values: []string{"10", "30", "60", "300", "off"},
		shown:  []string{"after 10 s untouched", "after 30 s untouched", "after 1 min untouched", "after 5 min untouched", "stays on"}},
	{key: "light-start", label: "Light at start",
		help:   "the light when hellobro starts: as you left it in hellobro, or Nickel's",
		values: []string{"last", "nickel"},
		shown:  []string{"as you last set it", "as Nickel had it"}},
	{key: "text-size", group: "Screen", label: "Text size", slider: true,
		help:   "the letters of the menus, lists, settings and buttons; not a terminal's, a book's or a notebook's",
		values: []string{"85", "100", "115", "130", "150"},
		shown:  []string{"smaller, 85%", "as it comes, 100%", "larger, 115%", "large, 130%", "largest, 150%"}},
	{key: "auto-rotate", label: "Auto-rotate",
		help:   "turns the screen as the Kobo is held, once it's still for a moment; each rotation is calibrated once, the first time",
		values: []string{"off", "on"},
		shown:  []string{"off: turned by Rotate only", "turns as it's held"}},
	{key: "touch-size", label: "Sizes", title: "Sizes: for fingers or compact",
		help:   "how tall the strips, rows and buttons are",
		values: []string{"fingers", "compact"},
		shown:  []string{"for fingers: taller strips, rows and buttons", "compact: more on the screen"}},
	{key: "sleep-battery", group: "Power", label: "Sleep when idle, on battery",
		help:   "the Kobo sleeps as with the power button; a press wakes it, where it was",
		values: []string{"5", "15", "30", "60", "120", "off"},
		shown:  []string{"after 5 min untouched", "after 15 min untouched", "after 30 min untouched", "after 1 h untouched", "after 2 h untouched", "never by itself"}},
	{key: "sleep-power", label: "… plugged in", title: "Sleep when idle, plugged in",
		values: []string{"off", "5", "15", "30", "60", "120"},
		shown:  []string{"never by itself", "after 5 min untouched", "after 15 min untouched", "after 30 min untouched", "after 1 h untouched", "after 2 h untouched"}},
	{key: "sleep-screen", label: "The sleep screen", title: "Power: the sleep screen",
		help:   "what stays on the screen while it sleeps; the agenda is drawn again at midnight",
		values: []string{"agenda", "plain"},
		shown:  []string{"the calendar's agenda: the days ahead, the tasks", "plain: sleeping, the battery"}},
	{key: "term-calm", label: "A terminal changing by itself", title: "Power: a terminal changing by itself",
		help:   "a clock or a progress bar in a terminal: shown in batches saves the battery",
		values: []string{"2", "5", "10", "off"},
		shown:  []string{"on screen every 2 s, once untouched 5 s", "on screen every 5 s, once untouched 5 s", "on screen every 10 s, once untouched 5 s", "on screen at once, always"}},
	{key: "term-clean", label: "… its quick updates cleaned up", title: "Power: a terminal's quick updates cleaned up",
		help:   "quick updates leave a ghost of what was there; a clean one takes it away",
		values: []string{"stop", "4"},
		shown:  []string{"once its output stops", "when it pauses, or every 4 s"}},
	{key: "mod", group: "Keyboard", label: "Mod key, the window keys'",
		help:   "the key held for the window manager's keys (Mod+Enter, Mod+D…)",
		values: []string{"super", "alt"},
		shown:  []string{"Super, the Win key", "Alt (the Win key is Alt then)"}},
	{key: "kbd-gone", label: "The keyboard gone", title: "The keyboard gone: Bluetooth",
		help:   "when a Bluetooth keyboard drops off, the Bluetooth window comes up to bring it back",
		values: []string{"15", "30", "60", "off"},
		shown:  []string{"Bluetooth full screen after 15 s", "Bluetooth full screen after 30 s", "Bluetooth full screen after 1 min", "nothing comes up"}},
	{key: "vkbd", label: "On-screen keyboard%s", cmd: "keyboard",
		help:   "the keyboard drawn at the bottom of the screen, for fingers",
		values: []string{"fields", "none", "auto", "on"},
		shown:  []string{"for a text field (a name, find, a note's text)", "off", "when no keyboard is connected (after 10 s)", "always"}},
	{key: "languages", label: "Languages to type in%s", cmd: "language", // a tap: ticks to choose them (langs.go)
		help:   "the layouts a language key goes through, on both keyboards",
		values: langValues(false), shown: langValues(true)},
	{key: "touch-marks", group: "Touch", label: "Touch marks",
		help:   "to see where the screen felt a finger or the pen",
		values: []string{"off", "on"},
		shown:  []string{"off", "a ring where it touched, a dot where it lifted, for 3 s"}},
	{key: "edge-swipe", label: "Workspaces by touch",
		help:   "how a finger goes to the next workspace; three fingers down from the top show them all",
		values: []string{"two", "edge"},
		shown:  []string{"two fingers swiped across", "two fingers, or one swiped in from a side"}},
	{key: "note-finger", group: "Notebooks", label: "A finger in a notebook",
		help:   "the pen always writes; this is a finger's part",
		values: []string{"auto", "writes", "turns"},
		shown:  []string{"writes, till a pen has: then it turns pages", "writes, as the pen does", "turns pages: only the pen writes"}},
	{key: "pen-buttons", label: "The pen's buttons",
		help:   "what the pen does while a button on it is held",
		values: []string{"marker", "eraser", "both"},
		shown:  []string{"the first a marker, the second an eraser", "the first an eraser, the second a marker", "both erase"}},
	{key: "note-export", label: "Notebook export",
		help:   "what export makes, in the notebook's folder, for sharing",
		values: []string{"ask", "pdf", "png"},
		shown:  []string{"asks: a PDF of the pages, or a PNG of the page shown", "a PDF of the pages", "a PNG of the page shown"}},
	{key: "note-light", label: "The light by the left edge, in a notebook",
		help:   "a finger up or down the left edge, as in the books",
		values: []string{"on", "off"},
		shown:  []string{"a finger up or down the left edge sets it", "off: a finger there is the page's (full screen)"}},
	{key: "ai-model", group: "AI notebooks", label: "Model",
		help:   "the model an AI notebook asks (the key in ai.conf): the API's list, to search, sort and filter by what they can do",
		values: []string{"conf", ai.DefaultModel}, shown: []string{"as ai.conf says", ai.DefaultModel}}, // the API's list, once it's had (applyModels)
	{key: "ai-format", label: "Answers", title: "AI notebooks: answers",
		help:   "Markdown has headings, tables, code, formulas in LaTeX, diagrams and graphs",
		values: []string{"md", "plain"}, shown: []string{"Markdown: formulas, code, diagrams", "plain text"}},
	{key: "ai-size", label: "Letters of an answer", title: "AI notebooks: letters of an answer", slider: true,
		help:   "how big an answer is written in a notebook (from the next answer)",
		values: diarySizeValues, shown: diarySizeShown},
	{key: "ai-pause", label: "Asks after a pause", title: "AI notebooks: asks after a pause",
		help:   "once the pen rests this long, what's written goes to the model",
		values: []string{"3", "2", "5", "10", "30", "60", "off"},
		shown:  []string{"the pen rests 3 s", "the pen rests 2 s", "the pen rests 5 s", "the pen rests 10 s", "the pen rests 30 s", "the pen rests 1 min", "off: only taps ask"}},
	{key: "ai-taps", label: "Asks on taps", title: "AI notebooks: asks on taps",
		help:   "quick taps of a finger on the page ask at once, finished or not",
		values: []string{"3", "2", "off"},
		shown:  []string{"three quick taps", "two quick taps", "off"}},
	{key: "ai-prompts", label: "Your prompts",
		help:   "the prompts you saved, for every notebook: read them, forget one, write a new one",
		values: []string{""}, shown: []string{""}}, // a window of them: notewizard.go
	{key: "nickel", group: "At start", label: "Nickel while hellobro runs",
		help:   "Nickel is Kobo's own reader; closed frees its memory but takes longer to go back",
		values: []string{"frozen", "closed"},
		shown:  []string{"frozen: back to it at once", "closed, as KOReader does (from the next start)"}},
	{key: "radios", label: "Wi-Fi and Bluetooth at start",
		values: []string{"kobo", "on"},
		shown:  []string{"as Kobo had them, and put back so", "turned on, and left on"}},
	{key: "share", group: "Sharing", label: "File sharing (SFTP, port 2022)",
		help:   "a computer paired once reaches the Kobo's files, and a shell, over Wi-Fi",
		values: []string{"off", "on", "pair"},
		shown:  []string{"off", "on (Sharing, above)", "pair a computer"}},
	{key: "relay", label: "Relay, a way in from afar (relay.conf)",
		help:   "through your own server, when the Kobo isn't on the computer's network",
		values: []string{"off", "on"},
		shown:  []string{"off", "on, while sharing and awake"}},
	{key: "mdns", label: "Show the Kobo as kobo.local (mDNS)",
		values: []string{"on", "off"},
		shown:  []string{"yes, while sharing", "no"}},
	{key: "profile", label: "CPU profiles (make push-profile)",
		help:   "for working on hellobro: what it spends its time on",
		values: []string{"off", "on"},
		shown:  []string{"off", "on, for a paired computer"}},
	{key: "notify", group: "Terminal", label: "Terminal notifications",
		help:   "a program in a terminal asking for attention",
		values: []string{"all", "messages", "off"},
		shown:  []string{"the bell and messages", "messages only (OSC 9, 777)", "off"}},
	{key: "notify-light", label: "… the light, if it's off", title: "Notifications: the light, if it's off",
		values: []string{"1", "3", "off"},
		shown:  []string{"on for 1 s", "on for 3 s", "stays off"}},
	{key: "notify-blink", label: "… the window's strip, if it shows", title: "Notifications: the window's strip, if it shows",
		values: []string{"3", "1", "5", "off"},
		shown:  []string{"blinks 3 times", "blinks once", "blinks 5 times", "stays still"}},
	{key: "notify-note", label: "… a note in the corner, if it doesn't", title: "Notifications: a note in the corner, if it doesn't",
		values: []string{"3", "5", "10", "off"},
		shown:  []string{"for 3 s", "for 5 s", "for 10 s", "none"}},
	{key: "bar", label: "The bar%s", cmd: "statusbar",
		help:   "the strip at the top: workspaces, the time, the battery",
		values: []string{"on", "off"},
		shown:  []string{"shown", "hidden"}},
	{key: "alpine-bus", group: "Alpine", label: "System bus access", title: "Alpine: system bus access",
		help:   "lets Alpine's programs talk to the Kobo's own services",
		values: []string{"off", "on"},
		shown:  []string{"off", "on: e.g. sound (it reaches Bluetooth and Wi-Fi setup too)"}},
	{key: "alpine-remove", label: "Remove it", title: "Alpine: remove it",
		values: []string{""}, shown: []string{""}}, // the question: alpine.go
	{key: "keys", group: "Keys", label: "Keys", help: "the keys of every command, as keys.conf has them", values: []string{""}, shown: []string{""}}, // the Keys menu: keyset.go
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

// valueRenamed are values that mean another now: the on-screen
// keyboard's "off" showed it for a text field; the AI answers' sizes
// were four words, now points.
var valueRenamed = map[string]string{"vkbd=off": "fields", "ai-size=s": "12", "ai-size=m": "15", "ai-size=l": "19", "ai-size=xl": "24"}

// The defaults, by key: the index of a value.
var settingDefault = map[string]int{"light-idle": 1, "sleep-battery": 2, "text-size": 1, "ai-size": 5}

// settingCommands step the settings from the switcher, each to its next
// value, as a tap on its row does, and a toast says what it is now. The
// ones with commands of their own (sharing, the relay, the status bar,
// tabs, keys) are left to those.
func settingCommands() []*command {
	var cs []*command
	for i, st := range settingList {
		switch st.key {
		case "share", "relay", "bar", "tabs", "keys":
			continue
		case "ai-model": // too many to step through: their window
			cs = append(cs, &command{id: "ai.models", title: "AI notebooks: the models", run: func(a *bro) { a.openGUI(modelsKey) }})
			continue
		case "ai-prompts":
			cs = append(cs, &command{id: "ai.prompts", title: "AI notebooks: your prompts", run: func(a *bro) { a.openGUI(promptsKey) }})
			continue
		case "languages":
			cs = append(cs, &command{id: "languages", title: "Setting: Languages to type in, ticked", run: (*bro).openLangs})
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
			if now, ok := valueRenamed[k+"="+v]; ok {
				v = now
			}
			for i, sv := range st.values {
				if sv == v {
					s[k] = i
				}
			}
			if k == "ai-model" && !slices.Contains(st.values, v) && v != "" { // the list's comes later: kept till then
				i := settingIndex(k)
				settingList[i].values = append(settingList[i].values, v)
				settingList[i].shown = append(settingList[i].shown, v)
				s[k] = len(settingList[i].values) - 1
			}
		}
	}
	return s
}

func (s settings) save() {
	var b strings.Builder
	for _, st := range settingList {
		if st.key != "bar" && st.key != "tabs" && st.key != "keys" && st.key != "alpine-remove" && st.key != "ai-prompts" { // kept in their own files, or none
			fmt.Fprintf(&b, "%s=%s\n", st.key, st.values[s[st.key]])
		}
	}
	if err := writeAtomic(kobo.AppFile(setTxt), []byte(b.String())); err != nil { // a flat battery mid-write leaves the old one
		log.Printf("settings: %v", err)
	}
}

// setting is the value of key, as saved.
func (a *bro) setting(key string) string {
	for _, st := range settingList {
		if st.key == key {
			switch key {
			case "bar":
				return map[bool]string{false: "on", true: "off"}[a.BarOff]
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
	n := len(st.values)
	a.pickSetting(i, (a.settingAt(i)+d+n)%n)
}

// settingAt is setting i's value, an index into its values.
func (a *bro) settingAt(i int) int {
	st := settingList[i]
	if st.key == "bar" {
		return indexOf(st.values, a.setting("bar"))
	}
	return a.set[st.key]
}

// pickSetting makes setting i's value its v'th, and does what that
// takes. The ones that are a menu or a question open it.
func (a *bro) pickSetting(i, v int) {
	st := settingList[i]
	switch st.key {
	case "keys":
		a.openKeys(nil)
		return
	case "alpine-remove":
		a.alpineRemoveAsk()
		return
	case "languages":
		a.openLangs()
		return
	case "ai-prompts":
		a.openGUI(promptsKey)
		return
	case "ai-model":
		a.openGUI(modelsKey)
		return
	case "bar":
		if v != a.settingAt(i) {
			a.toggleBar()
		}
	case "share": // off, on, pair: pairing turns it on and opens pairing, and stays "on"
		if st.values[v] == "pair" {
			a.sharePair()
			break
		}
		a.set[st.key] = v
		a.set.save()
		a.shareApply()
	default:
		a.set[st.key] = v
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
		case "mod":
			a.applyMod()
		case "touch-marks":
			a.SetTouchMarks(a.setting(st.key) == "on")
		case "auto-rotate":
			a.SetAutoRotate(a.setting(st.key) == "on")
		case "vkbd":
			a.vk.said = 0 // the setting's say now
			a.vkApply()
		case "touch-size":
			a.applySizes()
			a.Relayout()
		case "text-size":
			a.applyTextSize()
			a.Relayout()
		}
	}
	log.Printf("settings: %s = %s", st.key, a.setting(st.key))
	a.drawSettings()
}

// drawSettings asks the Settings window for a frame: what it shows
// changed.
func (a *bro) drawSettings() { a.inkRedraw("settings") }

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

// settingShown is what setting i's value says, as it is now.
func (a *bro) settingShown(i int) string {
	st := settingList[i]
	switch {
	case st.key == "share" && a.share.code != "":
		return "pairing: code " + a.share.code
	case st.key == "alpine-remove":
		return alpineSummary()
	case st.key == "keys" && len(a.keys.named) == 0:
		return "the defaults (keys.conf)"
	case st.key == "keys":
		return fmt.Sprintf("%d set by you (keys.conf)", len(a.keys.named))
	case st.key == "languages":
		return st.shown[indexOf(st.values, a.setting(st.key))]
	case st.key == "ai-prompts":
		switch n := len(loadPrompts()); n {
		case 0:
			return "none saved yet"
		case 1:
			return "1 saved"
		default:
			return fmt.Sprintf("%d saved", n)
		}
	}
	return st.shown[a.settingAt(i)]
}

func indexOf(vs []string, v string) int {
	for i, s := range vs {
		if s == v {
			return i
		}
	}
	return 0
}

// textSize is the theme's letters as they come, before the setting.
const textSize = 18

// applyTextSize makes the ink windows' letters as big as the setting.
func (a *bro) applyTextSize() {
	n, _ := strconv.Atoi(a.setting("text-size"))
	a.theme().TextSize = textSize * ink.Dp(max(n, 50)) / 100
}

// applyLightIdle tells the App when the light goes off by itself.
func (a *bro) applyLightIdle() {
	n, _ := strconv.Atoi(a.setting("light-idle")) // "off": 0, never
	a.LightIdle = time.Duration(n) * time.Second
}

// applyMod makes the Mod key the setting's, and the hints say so.
func (a *bro) applyMod() {
	modIsAlt = a.setting("mod") == "alt"
	a.tv.mods = termMods{caps: a.tv.mods.caps} // what was held meant the other
	a.applyFooter()
	a.wm.names = nil // the bar's hints again
}

// setSettingTo sets key to the value val, and keeps it.
func (a *bro) setSettingTo(key, val string) {
	st := settingList[settingIndex(key)]
	if i := slices.Index(st.values, val); i >= 0 {
		a.set[key] = i
		a.set.save()
	}
}
