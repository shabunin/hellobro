// Package keyfeed turns what a terminal sends into key presses and
// releases, Linux key codes, for a keyboard made with uinput: keys over
// Wi-Fi (hellobro-keys), typed on another device running hellobro, or in
// a terminal on a computer or a phone. For a device with no Bluetooth or
// USB host, it's the keyboard.
//
// A terminal that speaks kitty's keyboard protocol (kitty, foot,
// ghostty, wezterm, hellobro's own…) says everything: each key going
// down, repeating and up, the modifiers too, Super among them. Any other sends text and
// escape sequences, which are taken as presses with their modifiers:
// Alt (an Esc before the key), but no Super, so hellobro's Mod key is
// best Alt then (Settings, Keyboard).
package keyfeed

import (
	"strconv"
	"strings"
)

// Event is a key going down (1), repeating (2) or up (0).
type Event struct {
	Code  uint16
	Value int32
}

// KittyOn asks the terminal for kitty's protocol: disambiguate (1),
// report event types (2), report all keys as escape codes (8); then for
// the flags it has, which only a terminal that knows the protocol answers.
const KittyOn = "\x1b[>11u\x1b[?u"

// KittyOff gives the terminal back as it was.
const KittyOff = "\x1b[<u"

// Decoder keeps what's held between one read and the next.
type Decoder struct {
	Kitty bool // the terminal said it speaks kitty's protocol
	Quit  bool // Ctrl+] came: the user is done
	down  map[uint16]bool
	synth map[int]bool // modifiers we pressed for kitty's events
	rest  []byte       // an escape sequence cut off at the end of a read
}

// Feed decodes p, as it came off the terminal.
func (d *Decoder) Feed(p []byte) []Event {
	if d.down == nil {
		d.down = map[uint16]bool{}
	}
	b := append(d.rest, p...)
	d.rest = nil
	var out []Event
	for len(b) > 0 && !d.Quit {
		n, evs, ok := d.one(b)
		if !ok { // incomplete: wait for more
			d.rest = append([]byte(nil), b...)
			break
		}
		out = append(out, evs...)
		b = b[n:]
	}
	return out
}

// ReleaseAll lets go of whatever is down: the session ended.
func (d *Decoder) ReleaseAll() []Event {
	var out []Event
	for c, on := range d.down {
		if on {
			out = append(out, Event{c, 0})
		}
	}
	d.down = map[uint16]bool{}
	return out
}

// one decodes what b starts with: how much it took, the events, and
// whether it was all there.
func (d *Decoder) one(b []byte) (int, []Event, bool) {
	c := b[0]
	if c != 0x1b {
		return 1, d.legacyByte(c, 0), true
	}
	if len(b) == 1 { // Esc alone, at the end of what came: the key
		return 1, d.tap(KeyEsc, 0), true
	}
	switch b[1] {
	case '[':
		return d.csi(b)
	case 'O': // SS3: arrows and F1–F4 in application mode
		if len(b) < 3 {
			return 0, nil, false
		}
		if k, ok := csiFinal[b[2]]; ok {
			return 3, d.tap(k, 0), true
		}
		return 3, nil, true
	case 0x1b:
		return 1, d.tap(KeyEsc, 0), true
	}
	// Esc then a key: Alt with it.
	return 2, d.legacyByte(b[1], modAlt), true
}

// csi decodes ESC [ params final.
func (d *Decoder) csi(b []byte) (int, []Event, bool) {
	i := 2
	for i < len(b) && (b[i] < 0x40 || b[i] > 0x7e) {
		i++
	}
	if i == len(b) {
		if len(b) > 32 { // not a sequence of ours: drop it
			return len(b), nil, true
		}
		return 0, nil, false
	}
	params, final := string(b[2:i]), b[i]
	n := i + 1
	if strings.HasPrefix(params, "?") { // a reply: CSI ? flags u says kitty's here
		if final == 'u' {
			d.Kitty = true
		}
		return n, nil, true
	}
	fields := strings.Split(params, ";")
	num := func(f int, part int) int {
		if f >= len(fields) {
			return 0
		}
		parts := strings.Split(fields[f], ":")
		if part >= len(parts) {
			return 0
		}
		v, _ := strconv.Atoi(parts[part])
		return v
	}
	mods, kind := 0, 1
	if m := num(1, 0); m > 0 {
		mods = m - 1
	}
	if k := num(1, 1); k > 0 {
		kind = k
	}
	var key uint16
	switch final {
	case 'u':
		cp := num(0, 0)
		if cp == ']' && mods&modCtrl != 0 && kind != 3 {
			d.Quit = true
			return n, nil, true
		}
		if m, ok := kittyMods[cp]; ok { // a modifier itself
			return n, d.modKey(m, kind), true
		}
		var shift bool
		key, shift = runeKey(rune(cp))
		if shift {
			mods |= modShift
		}
	case '~':
		key = tildeKey[num(0, 0)]
	default:
		key = csiFinal[final]
	}
	switch {
	case key == 0:
		return n, nil, true
	case d.Kitty:
		return n, d.press(key, mods, kind), true
	case kind == 3: // a release before we knew it's kitty's: the press was a tap
		return n, nil, true
	}
	return n, d.tap(key, mods), true
}

// legacyByte is one byte of text or a control character, with mods.
func (d *Decoder) legacyByte(c byte, mods int) []Event {
	switch {
	case c == 0x1d: // Ctrl+]
		d.Quit = true
		return nil
	case c == '\r' || c == '\n':
		return d.tap(KeyEnter, mods)
	case c == '\t':
		return d.tap(KeyTab, mods)
	case c == 0x7f || c == 0x08:
		return d.tap(KeyBackspace, mods)
	case c == 0:
		return d.tap(KeySpace, mods|modCtrl)
	case c < 0x1b: // Ctrl+a…z
		return d.tap(letterKeys[c-1], mods|modCtrl)
	case c < 0x20: // Ctrl+\ ] ^ _
		k, _ := runeKey(rune(c + 0x40))
		return d.tap(k, mods|modCtrl)
	case c < 0x80:
		k, shift := runeKey(rune(c))
		if k == 0 {
			return nil
		}
		if shift {
			mods |= modShift
		}
		return d.tap(k, mods)
	}
	return nil // UTF-8 beyond ASCII: no key for it on a US keyboard
}

// The modifier bits, as the terminal encodes them (less one).
const (
	modShift = 1
	modAlt   = 2
	modCtrl  = 4
	modSuper = 8
)

// modOrder: the modifiers, their keys on the left and the right.
var modOrder = []struct {
	bit         int
	left, right uint16
}{{modCtrl, KeyLeftCtrl, KeyRightCtrl}, {modShift, KeyLeftShift, KeyRightShift},
	{modAlt, KeyLeftAlt, KeyRightAlt}, {modSuper, KeyLeftMeta, KeyRightMeta}}

// tap is a key pressed and let go, its modifiers around it.
func (d *Decoder) tap(key uint16, mods int) []Event {
	return append(d.press(key, mods, 1), d.press(key, mods, 3)...)
}

// press is one kitty event (kind 1 press, 2 repeat, 3 release) of key
// with mods. In a plain terminal (tap) the modifiers go down with the
// key and up after it. With kitty's protocol they're keys of their own;
// one the event says is held but never came down is pressed for it, and
// let go once an event says it's not held any more.
func (d *Decoder) press(key uint16, mods, kind int) []Event {
	var out []Event
	held := func(code uint16, on bool) {
		if d.down[code] != on {
			d.down[code] = on
			out = append(out, Event{code, b2v(on)})
		}
	}
	if d.synth == nil {
		d.synth = map[int]bool{}
	}
	if d.Kitty {
		for _, m := range modOrder {
			switch on := mods&m.bit != 0; {
			case on && !d.down[m.left] && !d.down[m.right]:
				held(m.left, true)
				d.synth[m.bit] = true
			case !on && d.synth[m.bit]:
				held(m.left, false)
				delete(d.synth, m.bit)
			}
		}
	} else if kind != 3 {
		for _, m := range modOrder {
			if mods&m.bit != 0 {
				held(m.left, true)
			}
		}
	}
	switch {
	case kind == 3:
		held(key, false)
	case kind == 2 && d.down[key]:
		out = append(out, Event{key, 2})
	default:
		if d.down[key] { // pressed again without letting go
			out = append(out, Event{key, 0})
		}
		d.down[key] = true
		out = append(out, Event{key, 1})
	}
	if !d.Kitty && kind == 3 {
		for i := len(modOrder) - 1; i >= 0; i-- {
			if m := modOrder[i]; mods&m.bit != 0 {
				held(m.left, false)
			}
		}
	}
	return out
}

// modKey is a modifier key itself going down or up (kitty's protocol):
// the real one, so nothing of ours stands in for it any more.
func (d *Decoder) modKey(code uint16, kind int) []Event {
	for _, m := range modOrder {
		if code == m.left || code == m.right {
			delete(d.synth, m.bit)
		}
	}
	on := kind != 3
	if kind == 2 || d.down[code] == on {
		return nil
	}
	d.down[code] = on
	return []Event{{code, b2v(on)}}
}

func b2v(on bool) int32 {
	if on {
		return 1
	}
	return 0
}
