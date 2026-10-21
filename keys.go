package main

// Keyboard: US QWERTY key map and a tiny text editor model.

import (
	"fmt"
	"strings"
)

const (
	keyEsc        = 1
	keyBackspace  = 14
	keyTab        = 15
	keyEnter      = 28
	keyLeftCtrl   = 29
	keyA          = 30
	keyLeftShift  = 42
	keyZ          = 44
	keyRightShift = 54
	keySpace      = 57
	keyCapsLock   = 58
	keyRightCtrl  = 97
)

var (
	plainKeys   = map[uint16]string{}
	shiftedKeys = map[uint16]string{}
	keyNames    = map[uint16]string{
		1: "Esc", 14: "Backspace", 15: "Tab", 28: "Enter", 29: "LCtrl", 42: "LShift",
		54: "RShift", 56: "LAlt", 57: "Space", 58: "CapsLock", 97: "RCtrl", 100: "RAlt",
		102: "Home", 103: "Up", 104: "PgUp", 105: "Left", 106: "Right", 107: "End",
		108: "Down", 109: "PgDn", 110: "Insert", 111: "Delete", 113: "Mute", 114: "VolDown",
		115: "VolUp", 125: "LMeta", 126: "RMeta", 87: "F11", 88: "F12",
	}
)

func init() {
	rows := []struct {
		first          uint16
		plain, shifted string
	}{
		{2, "1234567890-=", "!@#$%^&*()_+"},
		{16, "qwertyuiop[]", "QWERTYUIOP{}"},
		{30, "asdfghjkl;'`", "ASDFGHJKL:\"~"},
		{43, `\zxcvbnm,./`, "|ZXCVBNM<>?"},
	}
	for _, r := range rows {
		shifted := []rune(r.shifted)
		for i, c := range []rune(r.plain) {
			plainKeys[r.first+uint16(i)] = string(c)
			shiftedKeys[r.first+uint16(i)] = string(shifted[i])
		}
	}
	plainKeys[keySpace], shiftedKeys[keySpace] = " ", " "
	for i := uint16(0); i < 10; i++ {
		keyNames[59+i] = fmt.Sprintf("F%d", i+1)
	}
}

func keyChar(code uint16, shift, caps bool) (string, bool) {
	ch, ok := plainKeys[code]
	if !ok {
		return "", false
	}
	isLetter := ch >= "a" && ch <= "z"
	if shift != (caps && isLetter) {
		ch = shiftedKeys[code]
	}
	return ch, true
}

func keyName(code uint16) string {
	if n, ok := keyNames[code]; ok {
		return n
	}
	if ch, ok := plainKeys[code]; ok {
		return strings.ToUpper(ch)
	}
	return fmt.Sprintf("key %d", code)
}

// editor is an append-only text box: type, Enter, Backspace.
type editor struct {
	lines             []string
	shift, caps, ctrl bool
}

// key handles one EV_KEY event. It returns a description for the
// "last key" line ("" for releases) and whether the text changed.
func (e *editor) key(code uint16, value int32) (desc string, changed bool) {
	switch code {
	case keyLeftShift, keyRightShift:
		e.shift = value != 0
	case keyLeftCtrl, keyRightCtrl:
		e.ctrl = value != 0
	}
	if value == 0 {
		return "", false
	}
	desc = fmt.Sprintf("%s (code %d)", keyName(code), code)
	if value == 2 {
		desc += " repeat"
	}

	if len(e.lines) == 0 {
		e.lines = []string{""}
	}
	last := &e.lines[len(e.lines)-1]
	switch ch, ok := keyChar(code, e.shift, e.caps); {
	case code == keyCapsLock && value == 1:
		e.caps = !e.caps
		desc += fmt.Sprintf(", caps lock %v", map[bool]string{true: "on", false: "off"}[e.caps])
	case e.ctrl && ok:
		desc = "Ctrl+" + strings.ToUpper(ch) + "  " + desc
	case ok:
		*last += ch
		changed = true
	case code == keyTab:
		*last += "    "
		changed = true
	case code == keyEnter:
		e.lines = append(e.lines, "")
		changed = true
	case code == keyBackspace:
		if r := []rune(*last); len(r) > 0 {
			*last = string(r[:len(r)-1])
			changed = true
		} else if len(e.lines) > 1 {
			e.lines = e.lines[:len(e.lines)-1]
			changed = true
		}
	}
	return desc, changed
}

// wrapped returns the text hard-wrapped at cols.
func (e *editor) wrapped(cols int) []string {
	var out []string
	for _, l := range e.lines {
		r := []rune(l)
		for len(r) > cols {
			out = append(out, string(r[:cols]))
			r = r[cols:]
		}
		out = append(out, string(r))
	}
	if len(out) == 0 {
		out = []string{""}
	}
	return out
}
