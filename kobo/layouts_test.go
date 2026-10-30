package kobo

import "testing"

func TestRetype(t *testing.T) {
	defer SetTyping("en")
	for _, c := range []struct{ layout, in, want string }{
		{"en", "Hello, world!", "Hello, world!"},
		{"ru", "ghbdtn", "привет"},
		{"ru", "Ghbdtn? ", "Привет, "},
		{"ru", "`~", "ёЁ"},
		{"uk", "ghbdsn", "привіт"},
		{"uk", `\|`, "ґҐ"},
		{"de", "Grz/[e^", "Gry-üe&"},
		{"xx", "abc", "abc"},
	} {
		SetTyping(c.layout)
		if got := Retype(c.in); got != c.want {
			t.Errorf("%s %q: %q, want %q", c.layout, c.in, got, c.want)
		}
	}
	for name, rows := range layoutRows {
		for i, r := range rows {
			if p, s := len([]rune(r[0])), len([]rune(r[1])); p != s || p != []int{12, 12, 12, 11}[i] {
				t.Errorf("%s row %d: %d and %d keys", name, i, p, s)
			}
		}
	}
}
