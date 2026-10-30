package kobo

// Languages to type in: what the keys type in another layout, by their
// US places. Only text typed follows it (Type); the keys' names and the
// commands' keys stay the US ones, as Ctrl+C is Ctrl+C in any. Greek's
// accents are dead keys: ; (΄) and Shift+; (¨) type nothing, and the
// next letter comes with it.

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
	"el": {
		{"1234567890-=", "!@#$%^&*()_+"},
		{";ςερτυθιοπ[]", ":ΣΕΡΤΥΘΙΟΠ{}"},
		{"ασδφγηξκλ΄'`", "ΑΣΔΦΓΗΞΚΛ¨\"~"},
		{`\ζχψωβνμ,./`, "|ΖΧΨΩΒΝΜ<>?"},
	},
	"de": {
		{"1234567890ß'", "!\"§$%&/()=?`"},
		{"qwertzuiopü+", "QWERTZUIOPÜ*"},
		{"asdfghjklöä^", "ASDFGHJKLÖÄ°"},
		{"#yxcvbnm,.-", "'YXCVBNM;:_"},
	},
}

// Layouts are the ones there are, "en" first.
func Layouts() []string { return []string{"en", "ru", "uk", "de", "el"} }

// deadKeys are the accents typed before their letters, and what they
// make of each.
var deadKeys = map[string]map[rune]rune{
	"΄": {'α': 'ά', 'ε': 'έ', 'η': 'ή', 'ι': 'ί', 'ο': 'ό', 'υ': 'ύ', 'ω': 'ώ',
		'Α': 'Ά', 'Ε': 'Έ', 'Η': 'Ή', 'Ι': 'Ί', 'Ο': 'Ό', 'Υ': 'Ύ', 'Ω': 'Ώ'},
	"¨": {'ι': 'ϊ', 'υ': 'ϋ', 'Ι': 'Ϊ', 'Υ': 'Ϋ'},
}

var (
	layout   string            // "" or "en": US
	retyped  map[string]string // a US character by the key's: the layout's
	dead     string            // an accent typed, waiting for its letter
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
	layout, retyped, dead = name, layoutBy[name], ""
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

// Type is what the US characters ch (a key's, KeyChar's) type in the
// layout: an accent's dead key nothing, and the letter after it with
// it (an accent and a space: the accent).
func Type(ch string) string {
	s := Retype(ch)
	if _, ok := deadKeys[s]; ok {
		s, dead = dead, s // two in a row: the first as it is
		return s
	}
	if dead == "" {
		return s
	}
	acc := dead
	dead = ""
	r := []rune(s)
	if len(r) == 1 {
		if c, ok := deadKeys[acc][r[0]]; ok {
			return string(c)
		}
		if r[0] == ' ' {
			return acc
		}
	}
	return acc + s
}

// Retype is what the US characters ch (KeyChar's) are in the layout,
// the dead keys their accents: a key's label.
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
