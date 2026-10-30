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
		{"el", "ghbdtn", "γηβδτν"},
		{"xx", "abc", "abc"},
	} {
		SetTyping(c.layout)
		if got := Retype(c.in); got != c.want {
			t.Errorf("%s %q: %q, want %q", c.layout, c.in, got, c.want)
		}
	}
	SetTyping("el")
	for _, c := range []struct{ in, want string }{
		{"kalhm;era", "καλημέρα"},
		{";A:i", "Άϊ"},
		{"; x", "΄χ"},
		{";;a", "΄ά"},
		{";\n", "΄\n"},
	} {
		got := ""
		for _, r := range c.in {
			got += Type(string(r))
		}
		if got != c.want {
			t.Errorf("el %q: %q, want %q", c.in, got, c.want)
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
