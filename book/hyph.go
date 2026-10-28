package book

// Hyphenation, Liang's way (TeX's): patterns of letters with numbers
// between them, the odd ones where a word may break, the highest
// number at a place winning. American English's patterns, for books in
// English (or that don't say); others break only where they have soft
// hyphens.

import (
	_ "embed"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

//go:embed hyph/en-us.txt
var enUS string

type hyphenator struct {
	pats        map[string][]uint8 // letters: the numbers before, between and after them
	exc         map[string][]int   // words hyphenated by hand: where (in runes)
	maxLen      int
	left, right int // at least this many letters before and after a break
}

var (
	hyphOnce sync.Once
	hyphEN   *hyphenator
)

// hyphenatorFor is the hyphenator for a book in lang (dc:language), nil
// if there's none.
func hyphenatorFor(lang string) *hyphenator {
	lang = strings.ToLower(lang)
	if lang != "" && lang != "en" && !strings.HasPrefix(lang, "en-") && !strings.HasPrefix(lang, "en_") {
		return nil
	}
	hyphOnce.Do(func() { hyphEN = parseHyph(enUS, 2, 3) })
	return hyphEN
}

func parseHyph(src string, left, right int) *hyphenator {
	h := &hyphenator{pats: map[string][]uint8{}, exc: map[string][]int{}, left: left, right: right}
	exceptions := false
	for _, line := range strings.Split(src, "\n") {
		line = strings.TrimSpace(line)
		if line == "% exceptions" {
			exceptions = true
			continue
		}
		if line == "" || line[0] == '%' {
			continue
		}
		if exceptions {
			var word []rune
			var at []int
			for _, r := range line {
				if r == '-' {
					at = append(at, len(word))
				} else {
					word = append(word, r)
				}
			}
			h.exc[string(word)] = at
			continue
		}
		var letters []rune
		vals := []uint8{0}
		for _, r := range line {
			if r >= '0' && r <= '9' {
				vals[len(vals)-1] = uint8(r - '0')
			} else {
				letters = append(letters, r)
				vals = append(vals, 0)
			}
		}
		h.pats[string(letters)] = vals
		h.maxLen = max(h.maxLen, len(letters))
	}
	return h
}

// points are where word may break, as byte offsets into it: a hyphen
// goes before the byte. Words with other than letters in them don't.
func (h *hyphenator) points(word string) []int {
	rs := []rune(strings.ToLower(word))
	n := len(rs)
	if n < h.left+h.right || n > 64 {
		return nil
	}
	for _, r := range rs {
		if !unicode.IsLetter(r) {
			return nil
		}
	}
	var at []int // in runes
	if e, ok := h.exc[string(rs)]; ok {
		at = e
	} else {
		w := make([]rune, 0, n+2)
		w = append(append(append(w, '.'), rs...), '.')
		v := make([]uint8, len(w)+1)
		for i := range w {
			for j := i + 1; j <= min(len(w), i+h.maxLen); j++ {
				if p, ok := h.pats[string(w[i:j])]; ok {
					for k, x := range p {
						v[i+k] = max(v[i+k], x)
					}
				}
			}
		}
		for k := h.left; k <= n-h.right; k++ {
			if v[k+1]%2 == 1 { // before rs[k], which is w[k+1]
				at = append(at, k)
			}
		}
	}
	// runes to bytes, in the word as written
	var out []int
	k, b := 0, 0
	for _, r := range word {
		if len(at) > 0 && at[0] == k {
			out = append(out, b)
			at = at[1:]
		}
		k++
		b += utf8.RuneLen(r)
	}
	return out
}

// hyphenated is word with its breaks shown, for tests: "hy-phen-ation".
func (h *hyphenator) hyphenated(word string) string {
	var b strings.Builder
	last := 0
	for _, p := range h.points(word) {
		b.WriteString(word[last:p] + "-")
		last = p
	}
	b.WriteString(word[last:])
	return b.String()
}
