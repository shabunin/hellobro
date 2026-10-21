package main

// Settings' Keys: the commands, in the switcher's menu, each with its
// keys. Picking one asks for its new key, which keys.conf gets in place
// of the ones it had (and another command with that key lets it go).
// Backspace leaves it no key, Delete gives it its defaults back, Esc
// leaves it be.

import (
	"image"
	"log"
	"os"
	"slices"
	"strings"

	"hellobro/kobo"
)

// keySet is the question: the key for c.
type keySet struct {
	c                      *command
	note                   string // why the last key pressed won't do
	box, noneR, defR, back image.Rectangle
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
	b := a.C.Bounds()
	w := min(1100, b.Dx()-80)
	h := 400
	top := b.Min.Y + (b.Dy()-h)/2
	k := &keySet{c: c}
	k.box = image.Rect((b.Dx()-w)/2, top, (b.Dx()+w)/2, top+h)
	bw, by := (w-64-40)/3, k.box.Max.Y-100
	k.back = image.Rect(k.box.Max.X-32-bw, by, k.box.Max.X-32, by+68)
	k.defR = image.Rect(k.back.Min.X-20-bw, by, k.back.Min.X-20, by+68)
	k.noneR = image.Rect(k.defR.Min.X-20-bw, by, k.defR.Min.X-20, by+68)
	a.keyset = k
	a.drawKeySet()
}

func (a *bro) drawKeySet() {
	k, c := a.keyset, a.C
	c.Fill(k.box, kobo.White)
	c.Frame(k.box, 4, kobo.Black)
	in := k.box.Inset(32)
	c.Text(a.F.Panel, image.Rect(in.Min.X, in.Min.Y, in.Max.X, in.Min.Y+56), "Key for: "+k.c.title, kobo.Black, kobo.AlignLeft)
	now := a.keyLabels(k.c.id)
	if now == "" {
		now = "no key"
	}
	var defs []string
	for _, s := range k.c.keys {
		if d, err := parseKey(s); err == nil {
			defs = append(defs, d.label())
		}
	}
	def := strings.Join(defs, ", ")
	if def == "" {
		def = "none"
	}
	c.Text(a.F.Small, image.Rect(in.Min.X, in.Min.Y+66, in.Max.X, in.Min.Y+106), "now: "+now+"   ·   by default: "+def, kobo.Dark, kobo.AlignLeft)
	say := "Press the new key, with Ctrl, Alt or Meta (an F-key alone will do)."
	if k.note != "" {
		say = k.note
	}
	c.Text(a.F.Body, image.Rect(in.Min.X, in.Min.Y+130, in.Max.X, in.Min.Y+190), say, kobo.Black, kobo.AlignLeft)
	for _, b := range []struct {
		r     image.Rectangle
		label string
	}{{k.noneR, "no key  (Backspace)"}, {k.defR, "default  (Delete)"}, {k.back, "as it is  (Esc)"}} {
		c.Frame(b.r, 2, kobo.Black)
		c.Text(a.F.Small, b.r, b.label, kobo.Black, kobo.AlignCenter)
	}
	a.Mark(k.box, true)
}

// keySetDone takes the question down, and goes back to the list.
func (a *bro) keySetDone() {
	c := a.keyset.c
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
		k.note = combo{mods, code}.label() + " alone is for typing: with Ctrl, Alt or Meta, please."
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

func (a *bro) keySetTap(p image.Point) {
	k := a.keyset
	switch {
	case kobo.Hit(p, k.noneR):
		a.setKeys(k.c.id, []combo{}, false)
	case kobo.Hit(p, k.defR):
		a.setKeys(k.c.id, nil, true)
	case kobo.Hit(p, k.back), !p.In(k.box):
	default:
		return
	}
	a.keySetDone()
}

// setKeys has keys.conf give command id keys (empty: none), or with
// reset its defaults back. Then the keys are read again.
func (a *bro) setKeys(id string, keys []combo, reset bool) {
	path := kobo.AppFile(keysTxt)
	b, err := os.ReadFile(path)
	if err != nil {
		b = []byte(defaultKeysConf())
	}
	if err := os.WriteFile(path, []byte(rewriteKeys(string(b), a.keys, id, keys, reset)), 0o644); err != nil {
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
	lines := slices.DeleteFunc(strings.Split(strings.TrimRight(conf, "\n"), "\n"), func(l string) bool {
		line, _, _ := strings.Cut(l, "#")
		f := strings.Fields(line)
		if len(f) != 2 {
			return false
		}
		_, changed := set[f[1]]
		return f[1] == id || changed
	})
	for _, c := range commands {
		ks, ok := set[c.id]
		switch {
		case !ok:
		case len(ks) == 0:
			lines = append(lines, "none "+c.id)
		default:
			for _, k := range ks {
				lines = append(lines, k.String()+" "+c.id)
			}
		}
	}
	return strings.Join(lines, "\n") + "\n"
}
