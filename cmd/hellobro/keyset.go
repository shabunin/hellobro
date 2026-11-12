package main

// Settings' Keys: the commands, in the switcher's menu, each with its
// keys. Picking one asks for its new key, which keys.conf gets in place
// of the ones it had (and another command with that key lets it go).
// Backspace leaves it no key, Delete gives it its defaults back, Esc
// leaves it be.

import (
	"image"
	"log"
	"maps"
	"os"
	"slices"
	"strings"

	"hellobro/kobo"
)

// keySet is the question: the key for c.
type keySet struct {
	c    *command
	note string // why the last key pressed won't do
	ui   *keySetUI
	over *inkOver
}

// openKeys lists the commands to set a key for, sel picked.
func (a *bro) openKeys(sel *command) {
	var es []pickEntry
	for _, c := range commands {
		detail := a.keyLabels(c.id)
		if detail == "" {
			detail = "no key"
		}
		kind := ""
		if a.keys.named[c.id] {
			kind = "yours"
		}
		es = append(es, pickEntry{kind: kind, title: c.title, detail: detail, do: func() { a.askKeyFor(c) }})
	}
	a.openMenu("Keys", "type to find a command, Enter to set its key", es, nil)
	if i := slices.Index(commands, sel); i >= 0 {
		a.picker.sel = i
		a.drawPicker()
	}
}

func (a *bro) askKeyFor(c *command) {
	k := &keySet{c: c}
	k.ui = &keySetUI{a: a, k: k}
	k.over = newOver(k.ui.Layout, 1100)
	k.over.take = func() {
		act := k.ui.act
		k.ui.act = ""
		switch act {
		case "none":
			a.setKeys(k.c.id, []combo{}, false)
		case "default":
			a.setKeys(k.c.id, nil, true)
		case "back":
		default:
			return
		}
		a.keySetDone()
	}
	a.keyset = k
	a.drawKeySet()
}

func (a *bro) drawKeySet() { a.drawOver(a.keyset.over) }

// keySetWords are what the question says: the keys c has, its defaults.
func (a *bro) keySetWords(c *command) (now, def string) {
	now = a.keyLabels(c.id)
	if now == "" {
		now = "no key"
	}
	var defs []string
	for _, s := range c.keys {
		if d, err := parseKey(s); err == nil {
			defs = append(defs, d.label())
		}
	}
	def = strings.Join(defs, ", ")
	if def == "" {
		def = "none"
	}
	return now, def
}

// keySetDone takes the question down, and goes back to the list.
func (a *bro) keySetDone() {
	c := a.keyset.c
	a.keyset.over.gone()
	a.keyset = nil
	a.DrawPage()
	a.openKeys(c)
}

func (a *bro) keySetKey(code uint16, value int32) {
	if a.termMod(code, value) || value != 1 {
		return
	}
	k, mods := a.keyset, a.mods()
	_, named := keyConfNames[code]
	switch {
	case mods == 0 && code == kobo.KeyEsc:
	case mods == 0 && code == kobo.KeyBackspace:
		a.setKeys(k.c.id, []combo{}, false)
	case mods == 0 && code == keyDelete:
		a.setKeys(k.c.id, nil, true)
	case mods&^modShift == 0 && named && !isFKey(code):
		k.note = combo{mods, code}.label() + " alone is for typing: with Ctrl, Alt or Mod, please."
		a.drawKeySet()
		return
	default:
		a.setKeys(k.c.id, []combo{{mods, code}}, false)
	}
	a.keySetDone()
}

func isFKey(code uint16) bool {
	return code >= kobo.KeyF1 && code < kobo.KeyF1+10 || code == keyF11 || code == keyF12
}

// keySetTap: a tap outside the question leaves the key as it is. One on
// it was its pane's.
func (a *bro) keySetTap(p image.Point) {
	if !p.In(a.keyset.over.box) {
		a.keySetDone()
	}
}

// setKeys has keys.conf give command id keys (empty: none), or with
// reset its defaults back. Then the keys are read again.
func (a *bro) setKeys(id string, keys []combo, reset bool) {
	path := kobo.AppFile(keysTxt)
	b, err := os.ReadFile(path)
	if err != nil {
		b = []byte(defaultKeysConf())
	}
	if err := writeAtomic(path, []byte(rewriteKeys(string(b), a.keys, id, keys, reset))); err != nil {
		log.Printf("keys: %v", err)
		return
	}
	log.Printf("keys: %s: %v (back to the defaults: %v)", id, keys, reset)
	a.keys = loadKeys()
	a.keysChanged()
}

// rewriteKeys is conf, as m has it, with id's lines in place of what it
// said of id. Another command with one of those keys lets it go.
func rewriteKeys(conf string, m keyMap, id string, keys []combo, reset bool) string {
	set := map[string][]combo{}
	if !reset {
		set[id] = keys
	}
	for _, k := range keys {
		for _, o := range m.bind[k] {
			if o.id != id {
				set[o.id] = slices.DeleteFunc(slices.Clone(m.keys[o.id]), func(x combo) bool { return x == k })
			}
		}
	}
	// The default mode's lines go; the new ones come where they were, before
	// the first other mode's.
	var lines []string
	at, mode := -1, ""
	for _, l := range strings.Split(strings.TrimRight(conf, "\n"), "\n") {
		line, _, _ := strings.Cut(l, "#")
		f := strings.Fields(line)
		if name, ok := strings.CutPrefix(strings.Join(f, " "), "["); ok && len(f) == 1 && strings.HasSuffix(name, "]") {
			if mode = strings.TrimSuffix(name, "]"); mode == "default" {
				mode = ""
			} else if at < 0 {
				at = len(lines)
			}
		}
		if mode == "" && len(f) >= 2 {
			_, cid := keyID(strings.Join(f[1:], " "))
			if _, changed := set[cid]; cid == id || changed {
				continue
			}
		}
		lines = append(lines, l)
	}
	if at < 0 {
		at = len(lines)
	}
	var add []string
	ids := []string{}
	for _, c := range commands {
		ids = append(ids, c.id)
	}
	for _, o := range slices.Sorted(maps.Keys(set)) { // keys.conf's own i3 lines
		if !slices.Contains(ids, o) {
			ids = append(ids, o)
		}
	}
	for _, cid := range ids {
		ks, ok := set[cid]
		switch {
		case !ok:
		case len(ks) == 0:
			if commandByID(cid) != nil { // keys.conf's own just goes
				add = append(add, "none "+cid)
			}
		default:
			for _, k := range ks {
				add = append(add, k.String()+" "+cid)
			}
		}
	}
	lines = slices.Insert(lines, at, add...)
	return strings.Join(lines, "\n") + "\n"
}
