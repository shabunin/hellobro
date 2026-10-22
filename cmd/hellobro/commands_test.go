package main

import (
	"slices"
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
		if len(m.keys[c.id]) != len(c.keys) {
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
}

func TestParseKeys(t *testing.T) {
	m, errs := parseKeys(`
# a comment
f5 tab.next  # a comment too
ctrl+tab tab.next
ctrl+shift+p palette
ctrl+shift+p -
ctrl+shift+f -
ctrl+alt+9 nope
oops
`)
	if len(errs) != 2 {
		t.Errorf("errors: %q, want 2", errs)
	}
	ids := func(s string) []string {
		k, _ := parseKey(s)
		var out []string
		for _, c := range m.bind[k] {
			out = append(out, c.id)
		}
		return out
	}
	for key, want := range map[string]string{
		"f5":           "tab.next",
		"ctrl+tab":     "tab.next panel.next", // the file's first
		"ctrl+shift+p": "palette",             // named, and freed of its default (which it is too)
		"ctrl+shift+f": "",                    // freed
		"ctrl+shift+t": "session.new",         // as by default
		"ctrl+alt+9":   "tab.9",
	} {
		if got := strings.Join(ids(key), " "); got != want {
			t.Errorf("%s: %q, want %q", key, got, want)
		}
	}
	if !slices.Equal(m.keys["fullscreen"], nil) {
		t.Errorf("fullscreen: %v, want no key", m.keys["fullscreen"])
	}
}

func TestParseTabs(t *testing.T) {
	for in, want := range map[string]string{
		"":                                    tabsDefault,
		tabsDefault:                           tabsDefault,
		"music ssh":                           "ssh -shell settings power", // the Music tab is gone
		"-settings -music files ssh shell":    "settings ssh shell power",  // and Files
		"SSH SSH bogus -Files":                "ssh -shell settings power",
		"files settings music shell ssh more": "settings shell ssh power",
	} {
		if got := parseTabs(in).String(); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
	if got := parseTabs(tabsDefault).shown(); !slices.Equal(got, []int{tabSSH, tabRadio, tabPower}) {
		t.Errorf("shown: %v", got)
	}
}

func TestTabHints(t *testing.T) {
	four := "ssh shell settings power"
	for _, c := range []struct {
		conf, tabs string
		hints      []string
		footer     string
	}{
		{"", tabsDefault, []string{"1", "2", "3"}, "tabs: tap or Ctrl+Alt+1–3"},
		{"", four, []string{"1", "2", "3", "4"}, "tabs: tap or Ctrl+Alt+1–4"},
		{"alt+1 tab.1\nalt+2 tab.2\nalt+3 tab.3\nalt+4 tab.4", four, []string{"1", "2", "3", "4"}, "tabs: tap or Alt+1–4"},
		{"f1 tab.shell", four, []string{"1", "F1", "3", "4"}, "tabs: tap or Ctrl+Alt+corner key"},
		{"ctrl+alt+m tab.shell", four, []string{"1", "M", "3", "4"}, "tabs: tap or Ctrl+Alt+corner key"},
		{"f1 tab.1\nf2 tab.2\nctrl+alt+3 -\nctrl+alt+4 -", four, []string{"F1", "F2", "", ""}, "tabs: tap or the corner key"},
		{"ctrl+alt+1 -\nctrl+alt+2 -\nctrl+alt+3 -\nctrl+alt+4 -", four, []string{"", "", "", ""}, "tabs: tap"},
	} {
		a := &bro{tabs: parseTabs(c.tabs)}
		a.keys, _ = parseKeys(c.conf)
		hints, footer := a.tabHints()
		if !slices.Equal(hints, c.hints) || footer != c.footer {
			t.Errorf("%q: %q, %q; want %q, %q", c.conf, hints, footer, c.hints, c.footer)
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
	conf := "# the defaults\n# ctrl+tab session.next\nf5 tab.next\nnone doze\n"
	k := func(s string) combo { c, _ := parseKey(s); return c }
	for _, c := range []struct {
		id    string
		keys  []combo
		reset bool
		want  string // the lines that aren't comments
	}{
		{"tab.next", []combo{k("f6")}, false, "none doze|f6 tab.next"},
		{"tab.next", nil, true, "none doze"},
		{"tab.next", []combo{}, false, "none doze|none tab.next"},
		{"doze", []combo{k("ctrl+alt+d")}, false, "f5 tab.next|ctrl+alt+d doze"},
		// Ctrl+PgDn was session.next's: it keeps its other.
		{"doze", []combo{k("ctrl+pgdn")}, false, "f5 tab.next|ctrl+shift+right session.next|ctrl+pgdn doze"},
		// Ctrl+Tab was panel.next's only key: it has none then.
		{"doze", []combo{k("ctrl+tab")}, false, "f5 tab.next|none panel.next|ctrl+tab doze"},
	} {
		m, _ := parseKeys(conf)
		out := rewriteKeys(conf, m, c.id, c.keys, c.reset)
		var got []string
		for _, l := range strings.Split(strings.TrimSpace(out), "\n") {
			if !strings.HasPrefix(l, "#") {
				got = append(got, l)
			}
		}
		if g := strings.Join(got, "|"); g != c.want {
			t.Errorf("%s %v %v: %q, want %q", c.id, c.keys, c.reset, g, c.want)
		}
		if _, errs := parseKeys(out); len(errs) > 0 {
			t.Errorf("%s: %v", c.id, errs)
		}
	}
}
