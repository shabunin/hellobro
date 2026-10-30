package main

// Kitty's keyboard protocol (sw.kovidgoyal.net/kitty/keyboard-protocol):
// a program asks for it (CSI > flags u), and the keys then come as CSI …
// u, each saying what it is: Super among the modifiers, repeats and
// releases if it wants them, the modifier keys themselves if it wants
// every key. vt has none of it, so we answer the asking and say the keys
// ourselves; with no flags on, vt's SendKey says them the old way.
//
// hellobro-keys (keyfeed) asks for it: so Super, and every key, cross to
// the hellobro it types for.

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"

	"hellobro/kobo"
)

const (
	kittyDisambiguate = 1
	kittyEvents       = 2 // repeats and releases
	kittyAlternates   = 4 // the shifted key too
	kittyAllKeys      = 8 // text keys and the modifiers as CSI u too
	kittyText         = 16
	kittyFlags        = 31
	kittyStackMax     = 16
)

// kittyKeys is a terminal's flags: a stack per screen, the main and the
// alternate, as the protocol has them. The emulator changes them as the
// program's output comes (under t.mu); the loop reads them for each key.
type kittyKeys struct {
	mu    sync.Mutex
	stack [2][]int
	down  map[uint16]bool // pressed the kitty way: their release goes too
}

// flags are the flags on for the screen (1: the alternate).
func (k *kittyKeys) flags(screen int) int {
	k.mu.Lock()
	defer k.mu.Unlock()
	if s := k.stack[screen]; len(s) > 0 {
		return s[len(s)-1]
	}
	return 0
}

// csi does what CSI prefix n[;m] u asks for: ? the flags, > push, < pop,
// = set. It returns the reply, if there's one.
func (k *kittyKeys) csi(screen int, prefix byte, n, m int) string {
	k.mu.Lock()
	defer k.mu.Unlock()
	s := k.stack[screen]
	top := 0
	if len(s) > 0 {
		top = s[len(s)-1]
	}
	switch prefix {
	case '?':
		return fmt.Sprintf("\x1b[?%du", top)
	case '>':
		if len(s) == kittyStackMax { // full: the oldest goes
			s = s[1:]
		}
		s = append(s, n&kittyFlags)
	case '<':
		s = s[:max(len(s)-max(n, 1), 0)]
	case '=':
		if len(s) == 0 {
			s = append(s, 0)
		}
		switch m {
		case 2:
			top |= n
		case 3:
			top &^= n
		default:
			top = n
		}
		s[len(s)-1] = top & kittyFlags
	}
	k.stack[screen] = s
	return ""
}

// kittyRegister has emu answer the protocol for t.
func (t *termSession) kittyRegister(emu *vt.SafeEmulator) {
	k := &t.kitty
	k.mu.Lock()
	k.stack, k.down = [2][]int{}, map[uint16]bool{}
	k.mu.Unlock()
	for _, prefix := range []byte{'?', '>', '<', '='} {
		emu.RegisterCsiHandler(ansi.Command(prefix, 0, 'u'), func(p ansi.Params) bool {
			n, _, _ := p.Param(0, 0)
			m, _, _ := p.Param(1, 1)
			screen := 0
			if emu.Emulator.IsAltScreen() { // under its lock already: not emu's own
				screen = 1
			}
			if r := k.csi(screen, prefix, n, m); r != "" {
				io.WriteString(emu.InputPipe(), r)
			}
			return true
		})
	}
}

// kittySend says key code going down (1), repeating (2) or up (0) the
// kitty way, if the program asked for it: false if it didn't, and the
// old way says it.
func (t *termSession) kittySend(code uint16, value int32, m termMods) bool {
	if t == nil || t.emu == nil || !t.connected() {
		return false
	}
	screen := 0
	if t.emu.IsAltScreen() {
		screen = 1
	}
	f := t.kitty.flags(screen)
	if f == 0 {
		return false
	}
	k := &t.kitty
	k.mu.Lock()
	if value == 0 && !k.down[code] { // its press wasn't ours, or went the old way
		k.mu.Unlock()
		return value == 0
	}
	s, ok := kittyKey(code, value, m, f)
	switch {
	case !ok:
	case value == 0:
		delete(k.down, code)
	case s != "":
		k.down[code] = true
	}
	k.mu.Unlock()
	if s != "" {
		io.WriteString(t.emu.InputPipe(), s)
	}
	return ok
}

// kittyMods are the modifier keys' codes in the protocol.
var kittyMods = map[uint16]int{
	kobo.KeyLeftShift: 57441, kobo.KeyLeftCtrl: 57442, keyLeftAlt: 57443, keyLeftMeta: 57444,
	kobo.KeyRightShift: 57447, kobo.KeyRightCtrl: 57448, keyRightAlt: 57449, keyRightMeta: 57450,
	kobo.KeyCapsLock: 57358,
}

// kittyFunc is a key with no text: CSI 1;mods final, or CSI n;mods ~.
type kittyFunc struct {
	n     int
	final byte
}

var kittyFuncs = map[uint16]kittyFunc{
	keyUp: {1, 'A'}, keyDown: {1, 'B'}, keyRight: {1, 'C'}, keyLeft: {1, 'D'}, keyHome: {1, 'H'}, keyEnd: {1, 'F'},
	59: {1, 'P'}, 60: {1, 'Q'}, 61: {13, '~'}, 62: {1, 'S'}, 63: {15, '~'}, 64: {17, '~'}, 65: {18, '~'},
	66: {19, '~'}, 67: {20, '~'}, 68: {21, '~'}, keyF11: {23, '~'}, keyF12: {24, '~'},
	110: {2, '~'}, 111: {3, '~'}, keyPgUp: {5, '~'}, keyPgDn: {6, '~'},
}

// kittyCtl are the keys that are control characters the old way.
var kittyCtl = map[uint16]int{kobo.KeyEsc: 27, kobo.KeyEnter: 13, 96: 13, kobo.KeyTab: 9, kobo.KeyBackspace: 127}

// kittyKey is code going down (1), repeating (2) or up (0), with m held
// (after it, if it's a modifier), as flags f ask: "" for nothing; false
// if it's the old way's.
func kittyKey(code uint16, value int32, m termMods, f int) (string, bool) {
	all := f&kittyAllKeys != 0
	ev := 1
	switch value {
	case 0:
		ev = 3
	case 2:
		ev = 2
	}
	if ev != 1 && f&kittyEvents == 0 {
		if ev == 3 {
			return "", true
		}
		ev = 1 // a repeat is a press again
	}
	mods := 1
	for _, b := range []struct {
		on  bool
		bit int
	}{{m.shift, 1}, {m.alt, 2}, {m.ctrl, 4}, {m.meta, 8}, {m.caps && all, 64}} {
		if b.on {
			mods += b.bit
		}
	}
	held := m.ctrl || m.alt || m.meta
	oldWay := func() (string, bool) {
		if ev == 3 {
			return "", true // released, the old way never says
		}
		return "", false
	}
	if c, ok := kittyMods[code]; ok {
		if !all {
			return "", true
		}
		return kittyCSI(strconv.Itoa(c), mods, ev, "", 'u'), true
	}
	if fn, ok := kittyFuncs[code]; ok {
		if mods == 1 && ev == 1 && !all && f&kittyEvents == 0 {
			return oldWay() // as vt has it: application cursor keys and all
		}
		return kittyCSI(strconv.Itoa(fn.n), mods, ev, "", fn.final), true
	}
	if c, ok := kittyCtl[code]; ok {
		if all || f&kittyDisambiguate != 0 && (mods != 1 || c == 27) {
			return kittyCSI(strconv.Itoa(c), mods, ev, "", 'u'), true
		}
		return oldWay()
	}
	ch, printable := kobo.KeyChar(code, m.shift, m.caps)
	if !printable {
		return oldWay()
	}
	if !all && !(f&kittyDisambiguate != 0 && held) {
		return oldWay()
	}
	base, _ := kobo.KeyChar(code, false, false)
	key := strconv.Itoa(int([]rune(base)[0]))
	if r := []rune(ch)[0]; f&kittyAlternates != 0 && m.shift && string(r) != base {
		key += ":" + strconv.Itoa(int(r))
	}
	var text string
	if all && f&kittyText != 0 && ev != 3 && !held {
		var cps []string
		for _, r := range kobo.Type(ch) {
			cps = append(cps, strconv.Itoa(int(r)))
		}
		text = strings.Join(cps, ":")
	}
	return kittyCSI(key, mods, ev, text, 'u'), true
}

// kittyCSI is CSI key ; mods:event ; text final, what's the default left out.
func kittyCSI(key string, mods, ev int, text string, final byte) string {
	s := "\x1b["
	if key != "1" || mods != 1 || ev != 1 || final == 'u' || final == '~' {
		s += key
	}
	if mods != 1 || ev != 1 || text != "" {
		s += ";" + strconv.Itoa(mods)
		if ev != 1 {
			s += ":" + strconv.Itoa(ev)
		}
	}
	if text != "" {
		s += ";" + text
	}
	return s + string(final)
}
