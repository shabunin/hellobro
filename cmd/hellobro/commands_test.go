package main

import (
	"strings"
	"testing"
)

func TestParseKey(t *testing.T) {
	for _, c := range []struct {
		in    string
		label string // "" for an error
	}{
		{"ctrl+shift+p", "Ctrl+Shift+P"},
		{"Ctrl+Alt+1", "Ctrl+Alt+1"},
		{"f11", "F11"},
		{"f1", "F1"},
		{"ctrl+=", "Ctrl+="},
		{"ctrl+-", "Ctrl+-"},
		{"ctrl+shift+,", "Ctrl+Shift+<"},
		{"ctrl+shift+.", "Ctrl+Shift+>"},
		{"ctrl+shift+space", "Ctrl+Shift+Space"},
		{"ctrl+pgdn", "Ctrl+PgDn"},
		{"meta+tab", "Meta+Tab"},
		{"alt+shift+ctrl+x", "Ctrl+Shift+Alt+X"},
		{"ctrl+shift+1", "Ctrl+Shift+1"},
		{"brightnessup", "Brightness+"},
		{"ctrl+key183", "Ctrl+key 183"},
		{"ctrl+minus", "Ctrl+-"},
		{"key0", ""},
		{"ctrl++", ""},
		{"hyper+x", ""},
		{"ctrl+", ""},
		{"ctrl+nope", ""},
	} {
		k, err := parseKey(c.in)
		switch {
		case c.label == "" && err == nil:
			t.Errorf("%q: %v, want an error", c.in, k)
		case c.label != "" && err != nil:
			t.Errorf("%q: %v", c.in, err)
		case c.label != "" && k.label() != c.label:
			t.Errorf("%q: %q, want %q", c.in, k.label(), c.label)
		}
	}
}

func TestDefaultKeys(t *testing.T) {
	m, errs := parseKeys(defaultKeysConf()) // all commented out: the defaults
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	for _, c := range commands {
		if c.mode == "" && len(m.keys[c.id]) != len(c.keys) {
			t.Errorf("%s: keys %v, want %v", c.id, m.keys[c.id], c.keys)
		}
	}
	ids := map[string]bool{}
	for _, c := range commands {
		if ids[c.id] {
			t.Errorf("%s twice", c.id)
		}
		ids[c.id] = true
	}
	// No key runs two commands by default: the first would hide the other.
	for k, cs := range m.bind {
		if len(cs) > 1 && cs[0].when == nil {
			t.Errorf("%s: %s and %s", k, cs[0].id, cs[1].id)
		}
	}
	if b := m.modes["resize"]; len(b) != 11 {
		t.Errorf("resize mode: %d keys, want 11", len(b))
	}
	// Uncommented, every line of the defaults reads as it says.
	var lines []string
	for _, l := range strings.Split(defaultKeysConf(), "\n") {
		if l, ok := strings.CutPrefix(l, "# "); ok && (strings.Contains(l, " # ") || l == "[resize]") {
			lines = append(lines, l)
		}
	}
	if _, errs := parseKeys(strings.Join(lines, "\n")); len(errs) > 0 {
		t.Errorf("uncommented: %v", errs)
	}
}

func TestParseKeys(t *testing.T) {
	m, errs := parseKeys(`
# a comment
f5 workspace next  # a comment too
meta+tab workspace next
ctrl+shift+p palette
meta+d -
meta+f -
ctrl+alt+9 nope
oops
ctrl+alt+1 tab.1
ctrl+shift+t session.new
meta+t layout tabbed; focus parent
[resize]
x mode default
meta+r -
[default]
f6 [class="terminal"] focus
`)
	if len(errs) != 2 {
		t.Errorf("errors: %q, want 2", errs)
	}
	ids := func(mode, s string) []string {
		k, _ := parseKey(s)
		b := m.bind
		if mode != "" {
			b = m.modes[mode]
		}
		var out []string
		for _, c := range b[k] {
			out = append(out, c.id)
		}
		return out
	}
	for key, want := range map[string]string{
		"f5":            "workspace next",
		"meta+tab":      "workspace next focus next", // the file's first
		"ctrl+shift+p":  "palette",                   // named: its defaults go
		"meta+d":        "",                          // freed
		"meta+space":    "",                          // palette's default, gone
		"meta+f":        "",                          // freed
		"meta+enter":    "",                          // session.new is the launcher: named, its default goes
		"meta+shift+e":  "quit",                      // as by default, as i3's exit
		"ctrl+q":        "",                          // not ours: the terminal's
		"ctrl+alt+1":    "workspace number 1",        // the old id
		"meta+1":        "",                          // named in the file: its default goes
		"ctrl+shift+t":  "launcher",
		"meta+t":        "layout tabbed; focus parent",
		"f6":            `[class="terminal"] focus`,
		"resize x":      "mode default",
		"resize meta+r": "",
		"resize esc":    "",            // named in resize: its defaults there go
		"meta+r":        "mode resize", // but not the default mode's
		"resize left":   "resize shrink width 50 px or 5 ppt",
	} {
		mode, k, ok := strings.Cut(key, " ")
		if !ok {
			mode, k = "", key
		}
		if got := strings.Join(ids(mode, k), " "); got != want {
			t.Errorf("%s: %q, want %q", key, got, want)
		}
	}
}

func TestFooter(t *testing.T) {
	for conf, want := range map[string]string{
		"":                         "workspaces: tap or Meta+1–9",
		"alt+1 workspace number 1": "workspaces: tap",
		"none workspace number 9":  "workspaces: tap",
	} {
		m, _ := parseKeys(conf)
		got := m.footer()
		if got != want {
			t.Errorf("%q: %q, want %q", conf, got, want)
		}
	}
}

func TestKeyString(t *testing.T) {
	for name, code := range keyCodes {
		for _, mods := range []uint8{0, modCtrl | modShift, modAlt | modMeta} {
			c := combo{mods, code}
			back, err := parseKey(c.String())
			if err != nil || back != c {
				t.Errorf("%s (%v): %q gives %v, %v", name, c, c.String(), back, err)
			}
		}
	}
	if s := (combo{modCtrl, 183}).String(); s != "ctrl+key183" {
		t.Errorf("key183: %q", s)
	}
}

func TestRewriteKeys(t *testing.T) {
	conf := "# the defaults\n# meta+tab focus next\nf5 workspace next\nnone music.stop\nf7 layout tabbed; focus parent\n[resize]\nx mode default\n"
	k := func(s string) combo { c, _ := parseKey(s); return c }
	for _, c := range []struct {
		id    string
		keys  []combo
		reset bool
		want  string // the lines that aren't comments
	}{
		{"workspace next", []combo{k("f6")}, false, "none music.stop|f7 layout tabbed; focus parent|f6 workspace next|[resize]|x mode default"},
		{"workspace next", nil, true, "none music.stop|f7 layout tabbed; focus parent|[resize]|x mode default"},
		{"workspace next", []combo{}, false, "none music.stop|f7 layout tabbed; focus parent|none workspace next|[resize]|x mode default"},
		{"music.stop", []combo{k("ctrl+alt+d")}, false, "f5 workspace next|f7 layout tabbed; focus parent|ctrl+alt+d music.stop|[resize]|x mode default"},
		// Meta+K was focus up's: it keeps its other.
		{"music.stop", []combo{k("meta+k")}, false, "f5 workspace next|f7 layout tabbed; focus parent|focus up's|meta+k music.stop|[resize]|x mode default"},
		// Meta+A was focus parent's only key: it has none then.
		{"music.stop", []combo{k("meta+a")}, false, "f5 workspace next|f7 layout tabbed; focus parent|none focus parent|meta+a music.stop|[resize]|x mode default"},
		// F7 was keys.conf's own line: it goes.
		{"music.stop", []combo{k("f7")}, false, "f5 workspace next|f7 music.stop|[resize]|x mode default"},
	} {
		m, _ := parseKeys(conf)
		out := rewriteKeys(conf, m, c.id, c.keys, c.reset)
		var got []string
		for _, l := range strings.Split(strings.TrimSpace(out), "\n") {
			if !strings.HasPrefix(l, "#") {
				got = append(got, l)
			}
		}
		want := strings.ReplaceAll(c.want, "focus up's", "meta+up focus up")
		if g := strings.Join(got, "|"); g != want {
			t.Errorf("%s %v %v: %q, want %q", c.id, c.keys, c.reset, g, want)
		}
		if _, errs := parseKeys(out); len(errs) > 0 {
			t.Errorf("%s: %v", c.id, errs)
		}
	}
}

func TestKeysOf(t *testing.T) {
	m, _ := parseKeys("meta+left workspace prev\nf1 workspace number 1")
	a := &bro{keys: m}
	for _, c := range []struct {
		ids  []string
		want string
	}{
		{[]string{"launcher"}, "Meta+Enter"},
		{[]string{"focus left", "focus down", "focus up", "focus right"}, "Meta+H/Down/Up/Right"},
		{[]string{"split h", "split v"}, "Meta+B/V"},
		{[]string{"workspace number 2", "workspace number 3", "workspace number 4", "workspace number 5", "workspace number 10"}, "Meta+2…0"},
		{[]string{"workspace number 1", "workspace number 2"}, "F1, Meta+2"},
		{[]string{"copy", "launcher"}, "Ctrl+Shift+C, Meta+Enter"},
		{[]string{"quit"}, "Shift+Meta+E"},
		{[]string{"focus child"}, ""},
	} {
		if got := a.keysOf(c.ids...); got != c.want {
			t.Errorf("keysOf(%v) = %q, want %q", c.ids, got, c.want)
		}
	}
}

// With Alt as the Mod key, Alt runs keys.conf's meta keys, the Win key
// its alt ones, and the hints say what to press.
func TestModIsAlt(t *testing.T) {
	defer func() { modIsAlt = false }()
	c, err := parseKey("meta+enter")
	if err != nil {
		t.Fatal(err)
	}
	if got := c.label(); got != "Meta+Enter" {
		t.Errorf("Super: label %q", got)
	}
	modIsAlt = true
	if got := c.label(); got != "Alt+Enter" {
		t.Errorf("Alt: label %q", got)
	}
	if got := c.String(); got != "meta+enter" {
		t.Errorf("Alt: keys.conf has %q, still meta's", got)
	}
	if got := swapMod(modAlt | modShift); got != modMeta|modShift {
		t.Errorf("Alt+Shift held is %b, want meta+shift", got)
	}
	if got := swapMod(modMeta); got != modAlt {
		t.Errorf("the Win key held is %b, want alt", got)
	}
}
