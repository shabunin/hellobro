package kobo

// Languages to type in: what the keys type in another layout, by their
// US places. Only text typed follows it (Retype); the keys' names and
// the commands' keys stay the US ones, as Ctrl+C is Ctrl+C in any.

import "strings"

// layoutRows are each layout's four rows of characters, as init's in
// keys.go: plain, then shifted.
var layoutRows = map[string][4][2]string{
	"ru": {
		{"1234567890-=", "!\"№;%:?*()_+"},
		{"йцукенгшщзхъ", "ЙЦУКЕНГШЩЗХЪ"},
		{"фывапролджэё", "ФЫВАПРОЛДЖЭЁ"},
		{`\ячсмитьбю.`, "/ЯЧСМИТЬБЮ,"},
	},
	"uk": {
		{"1234567890-=", "!\"№;%:?*()_+"},
		{"йцукенгшщзхї", "ЙЦУКЕНГШЩЗХЇ"},
		{"фівапролджє'", "ФІВАПРОЛДЖЄ₴"},
		{"ґячсмитьбю.", "ҐЯЧСМИТЬБЮ,"},
	},
	"de": {
		{"1234567890ß'", "!\"§$%&/()=?`"},
		{"qwertzuiopü+", "QWERTZUIOPÜ*"},
		{"asdfghjklöä^", "ASDFGHJKLÖÄ°"},
		{"#yxcvbnm,.-", "'YXCVBNM;:_"},
	},
}

// Layouts are the ones there are, besides "en".
func Layouts() []string { return []string{"ru", "uk", "de"} }

var (
	layout   string            // "" or "en": US
	retyped  map[string]string // a US character by the key's: the layout's
	layoutBy = map[string]map[string]string{}
)

func init() {
	firsts := []uint16{2, 16, 30, 43}
	for name, rows := range layoutRows {
		m := map[string]string{}
		for i, r := range rows {
			plain, shifted := []rune(r[0]), []rune(r[1])
			for j := range plain {
				code := firsts[i] + uint16(j)
				m[plainKeys[code]] = string(plain[j])
				m[shiftedKeys[code]] = string(shifted[j])
			}
		}
		layoutBy[name] = m
	}
}

// SetTyping has text typed in name's layout from now on: "en", or one
// of Layouts (else it's "en").
func SetTyping(name string) {
	layout, retyped = name, layoutBy[name]
	if retyped == nil {
		layout = "en"
	}
}

// Typing is the layout typed in.
func Typing() string {
	if layout == "" {
		return "en"
	}
	return layout
}

// Retype is what the US characters ch (KeyChar's) type in the layout.
func Retype(ch string) string {
	if retyped == nil {
		return ch
	}
	var b strings.Builder
	for _, r := range ch {
		if t, ok := retyped[string(r)]; ok {
			b.WriteString(t)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
