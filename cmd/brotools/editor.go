package main

// The Keys tab's text box.

import (
	"fmt"
	"strings"

	"hellobro/kobo"
)

// editor is an append-only text box: type, Enter, Backspace.
type editor struct {
	lines             []string
	shift, caps, ctrl bool
}

// key handles one EV_KEY event. It returns a description for the
// "last key" line ("" for releases) and whether the text changed.
func (e *editor) key(code uint16, value int32) (desc string, changed bool) {
	switch code {
	case kobo.KeyLeftShift, kobo.KeyRightShift:
		e.shift = value != 0
	case kobo.KeyLeftCtrl, kobo.KeyRightCtrl:
		e.ctrl = value != 0
	}
	if value == 0 {
		return "", false
	}
	desc = fmt.Sprintf("%s (code %d)", kobo.KeyName(code), code)
	if value == 2 {
		desc += " repeat"
	}

	if len(e.lines) == 0 {
		e.lines = []string{""}
	}
	last := &e.lines[len(e.lines)-1]
	switch ch, ok := kobo.KeyChar(code, e.shift, e.caps); {
	case code == kobo.KeyCapsLock && value == 1:
		e.caps = !e.caps
		desc += fmt.Sprintf(", caps lock %v", map[bool]string{true: "on", false: "off"}[e.caps])
	case e.ctrl && ok:
		desc = "Ctrl+" + strings.ToUpper(ch) + "  " + desc
	case ok:
		*last += ch
		changed = true
	case code == kobo.KeyTab:
		*last += "    "
		changed = true
	case code == kobo.KeyEnter:
		e.lines = append(e.lines, "")
		changed = true
	case code == kobo.KeyBackspace:
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
