package dict

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Fold is a word as it's matched: lower case, no accents or other marks,
// one apostrophe and one Greek sigma. In running text Greek's accents
// change (καλός is καλὸς before another word), Wiktionary marks vowels'
// length (σκῠ́λος), Russian's books have no stress marks and few ёs.
// It makes some words one (мой, мои; pêche, péché): Lookup puts the one
// written as the word is first.
func Fold(s string) string {
	ascii := true
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			ascii = false
			break
		}
	}
	if ascii { // most words of most dictionaries, and a lot faster
		return strings.ToLower(s)
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range norm.NFD.String(s) {
		switch {
		case unicode.Is(unicode.Mn, r), r == '­', r == '‍', r == '‌':
			// accents, breathings, soft hyphens, joiners
		case IsApostrophe(r):
			b.WriteByte('\'')
		case r == 'ς':
			b.WriteRune('σ')
		default:
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

// IsApostrophe says if r closes a word cut short: l'homme, ἀλλ᾽.
func IsApostrophe(r rune) bool {
	switch r {
	case '\'', '’', 'ʼ', '᾽', '᾿', '′', '＇':
		return true
	}
	return false
}

// Candidates are what to look a word up as, the likeliest first: as it
// is, then what's after an apostrophe (l'homme: homme), or before it if
// that's how it ends (ἀλλ᾽: ἀλλ); a hyphened word as one, and its parts.
func Candidates(word string) []string {
	out := []string{word}
	add := func(s string) {
		if s != "" && !contains(out, s) {
			out = append(out, s)
		}
	}
	if i := strings.IndexFunc(word, IsApostrophe); i >= 0 {
		_, n := utf8.DecodeRuneInString(word[i:])
		if after := word[i+n:]; after != "" { // l'homme: what's before is the little word
			add(after)
		} else { // ἀλλ᾽
			add(word[:i])
		}
	}
	if strings.ContainsAny(word, "-‐‑") { // well-known: as one, and each part
		add(strings.NewReplacer("-", "", "‐", "", "‑", "").Replace(word))
		for _, part := range strings.FieldsFunc(word, func(r rune) bool { return r == '-' || r == '‐' || r == '‑' }) {
			add(part)
		}
	}
	return out
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
