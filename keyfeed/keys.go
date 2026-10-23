package keyfeed

// Linux key codes (linux/input-event-codes.h), and where a US keyboard
// has each character.

const (
	KeyEsc        = 1
	KeyMinus      = 12
	KeyEqual      = 13
	KeyBackspace  = 14
	KeyTab        = 15
	KeyLeftBrace  = 26
	KeyRightBrace = 27
	KeyEnter      = 28
	KeyLeftCtrl   = 29
	KeySemicolon  = 39
	KeyApostrophe = 40
	KeyGrave      = 41
	KeyLeftShift  = 42
	KeyBackslash  = 43
	KeyComma      = 51
	KeyDot        = 52
	KeySlash      = 53
	KeyRightShift = 54
	KeyLeftAlt    = 56
	KeySpace      = 57
	KeyCapsLock   = 58
	KeyF1         = 59
	KeyF11        = 87
	KeyF12        = 88
	KeyRightCtrl  = 97
	KeyRightAlt   = 100
	KeyHome       = 102
	KeyUp         = 103
	KeyPageUp     = 104
	KeyLeft       = 105
	KeyRight      = 106
	KeyEnd        = 107
	KeyDown       = 108
	KeyPageDown   = 109
	KeyInsert     = 110
	KeyDelete     = 111
	KeyLeftMeta   = 125
	KeyRightMeta  = 126
	KeyMax        = 248 // the keys the uinput keyboard says it has
)

// letterKeys are a…z.
var letterKeys = [26]uint16{30, 48, 46, 32, 18, 33, 34, 35, 23, 36, 37, 38, 50, 49, 24, 25, 16, 19, 31, 20, 22, 47, 17, 45, 21, 44}

// digitKeys are 0…9.
var digitKeys = [10]uint16{11, 2, 3, 4, 5, 6, 7, 8, 9, 10}

var punctKeys = map[rune]uint16{
	' ': KeySpace, '-': KeyMinus, '=': KeyEqual, '[': KeyLeftBrace, ']': KeyRightBrace, '\\': KeyBackslash,
	';': KeySemicolon, '\'': KeyApostrophe, '`': KeyGrave, ',': KeyComma, '.': KeyDot, '/': KeySlash,
}

// shifted is what each character is shifted from.
var shifted = map[rune]rune{
	'!': '1', '@': '2', '#': '3', '$': '4', '%': '5', '^': '6', '&': '7', '*': '8', '(': '9', ')': '0',
	'_': '-', '+': '=', '{': '[', '}': ']', '|': '\\', ':': ';', '"': '\'', '~': '`', '<': ',', '>': '.', '?': '/',
}

// runeKey is the key for r on a US keyboard, and whether it takes Shift.
// Kitty's protocol also names Enter, Tab, Backspace and Esc this way.
func runeKey(r rune) (uint16, bool) {
	switch {
	case r >= 'a' && r <= 'z':
		return letterKeys[r-'a'], false
	case r >= 'A' && r <= 'Z':
		return letterKeys[r-'A'], true
	case r >= '0' && r <= '9':
		return digitKeys[r-'0'], false
	case r == 13:
		return KeyEnter, false
	case r == 9:
		return KeyTab, false
	case r == 127 || r == 8:
		return KeyBackspace, false
	case r == 27:
		return KeyEsc, false
	}
	if k, ok := punctKeys[r]; ok {
		return k, false
	}
	if base, ok := shifted[r]; ok {
		k, _ := runeKey(base)
		return k, true
	}
	if k, ok := kittyKeys[int(r)]; ok {
		return k, false
	}
	return 0, false
}

// csiFinal: CSI 1;mods X and SS3 X.
var csiFinal = map[byte]uint16{
	'A': KeyUp, 'B': KeyDown, 'C': KeyRight, 'D': KeyLeft, 'H': KeyHome, 'F': KeyEnd,
	'P': KeyF1, 'Q': KeyF1 + 1, 'R': KeyF1 + 2, 'S': KeyF1 + 3, 'Z': KeyTab,
}

// tildeKey: CSI n;mods ~.
var tildeKey = map[int]uint16{
	1: KeyHome, 2: KeyInsert, 3: KeyDelete, 4: KeyEnd, 5: KeyPageUp, 6: KeyPageDown, 7: KeyHome, 8: KeyEnd,
	11: KeyF1, 12: KeyF1 + 1, 13: KeyF1 + 2, 14: KeyF1 + 3, 15: KeyF1 + 4,
	17: KeyF1 + 5, 18: KeyF1 + 6, 19: KeyF1 + 7, 20: KeyF1 + 8, 21: KeyF1 + 9, 23: KeyF11, 24: KeyF12,
}

// kittyMods are the modifier keys' own codes in kitty's protocol.
var kittyMods = map[int]uint16{
	57441: KeyLeftShift, 57442: KeyLeftCtrl, 57443: KeyLeftAlt, 57444: KeyLeftMeta,
	57447: KeyRightShift, 57448: KeyRightCtrl, 57449: KeyRightAlt, 57450: KeyRightMeta,
}

// kittyKeys are other keys kitty's protocol names by code.
var kittyKeys = map[int]uint16{57358: KeyCapsLock}
