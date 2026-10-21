package kobo

// Keyboard: key codes (linux/input-event-codes.h) and the US QWERTY key map.

import (
	"fmt"
	"strings"
)

const (
	KeyEsc        = 1
	Key1          = 2 // to 9 (10), then 0 (11)
	KeyBackspace  = 14
	KeyTab        = 15
	KeyEnter      = 28
	KeyLeftCtrl   = 29
	KeyA          = 30
	KeyLeftShift  = 42
	KeyZ          = 44
	KeyRightShift = 54
	KeySpace      = 57
	KeyCapsLock   = 58
	KeyRightCtrl  = 97
	KeyF1         = 59
	keyLeftAlt    = 56
	keyRightAlt   = 100
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
	plainKeys[KeySpace], shiftedKeys[KeySpace] = " ", " "
	for i := uint16(0); i < 10; i++ {
		keyNames[59+i] = fmt.Sprintf("F%d", i+1)
	}
}

// KeyChar is the character code types, if any.
func KeyChar(code uint16, shift, caps bool) (string, bool) {
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

// KeyName is what code is called on a keyboard: "Esc", "F1", "Q".
func KeyName(code uint16) string {
	if n, ok := keyNames[code]; ok {
		return n
	}
	if ch, ok := plainKeys[code]; ok {
		return strings.ToUpper(ch)
	}
	return fmt.Sprintf("key %d", code)
}

// Media keys, from headphones or a keyboard.
const (
	KeyNextSong     = 163
	KeyPlayPause    = 164
	KeyPreviousSong = 165
	KeyStopCD       = 166
	KeyPlayCD       = 200
	KeyPauseCD      = 201
)

func IsMediaKey(code uint16) bool {
	switch code {
	case KeyNextSong, KeyPlayPause, KeyPreviousSong, KeyStopCD, KeyPlayCD, KeyPauseCD:
		return true
	}
	return false
}
