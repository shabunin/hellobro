package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"hellobro/book"
	"hellobro/dict"
	"hellobro/kobo"
)

func TestWordsIn(t *testing.T) {
	for _, c := range []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"one two", []string{"one", "two"}},
		{"«Да,» он—сказал.", []string{"Да", "он", "сказал"}},
		{"кто-то пришёл", []string{"кто-то", "пришёл"}},
		{"be-", []string{"be"}},
		{"l'homme", []string{"l'homme"}},
		{"ἀλλ᾽ ὁ", []string{"ἀλλ᾽", "ὁ"}},
		{"ἄνδρα μοι ἔννεπε, Μοῦσα,", []string{"ἄνδρα", "μοι", "ἔννεπε", "Μοῦσα"}},
		{"‘quoted’ dogs’", []string{"quoted’", "dogs’"}},
		{"σκῠ́λος", []string{"σκῠ́λος"}},
		{"(1984)", []string{"1984"}},
		{"-x- --", []string{"x"}},
	} {
		var got []string
		for _, w := range wordsIn(c.in) {
			got = append(got, c.in[w[0]:w[1]])
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("%q: %q, want %q", c.in, got, c.want)
		}
	}
}

func TestPageWord(t *testing.T) {
	item := func(x, y, w int, s string) book.Item {
		return book.Item{Kind: book.ItemText, X: x, Y: y, W: w, Text: s}
	}
	items := []book.Item{
		item(0, 10, 50, "a word be-"), // 0: cut at the line's end
		item(0, 20, 30, "ginning and"),
		item(40, 20, 20, "spl"), // 2: touching the next
		item(60, 20, 20, "it"),
		item(90, 20, 20, "apart"), // 4: not touching
		item(0, 30, 20, "plain-"), // 5: its end's a hyphen, the next line says
		item(0, 40, 20, "text."),
		{Kind: book.ItemPicture, X: 0, Y: 50},
	}
	for _, c := range []struct {
		i, from, to int
		want        string
	}{
		{0, 7, 9, "beginning"}, // the first half
		{1, 0, 7, "beginning"}, // the second
		{1, 8, 11, "and"},      // on, not touching
		{0, 2, 6, "word"},      // in the middle
		{2, 0, 3, "split"},     // a span's end in the word
		{3, 0, 2, "split"},     // and from its other side
		{4, 0, 5, "apart"},     // not touching
		{5, 0, 5, "plaintext"}, // a real hyphen's as good as lost at a line's end
		{6, 0, 4, "plaintext"},
	} {
		if got := pageWord(items, c.i, c.from, c.to); got != c.want {
			t.Errorf("item %d [%d,%d]: %q, want %q", c.i, c.from, c.to, got, c.want)
		}
	}
}

func TestEntryBlocks(t *testing.T) {
	texts := func(bs []book.Block) []string {
		var out []string
		for _, b := range bs {
			s := ""
			for _, sp := range b.Spans {
				s += sp.Text
			}
			out = append(out, s)
		}
		return out
	}
	e := dict.Entry{Dict: "D", Word: "w", Fields: []dict.Field{
		{Type: 't', Text: "wɜː"},
		{Type: 'm', Text: "one\n\n two \n"},
		{Type: 'h', Text: "<p>three <b>four</b></p><img src='a.png'/>"},
		{Type: 'P', Text: ""},
	}}
	want := []string{"w  D", "wɜː", "one", "two", "three four"}
	got := entryBlocks(e, true)
	if !slices.Equal(texts(got), want) {
		t.Errorf("%q, want %q", texts(got), want)
	}
	if got[0].Above != 0 {
		t.Errorf("first: above %v", got[0].Above)
	}
	if got := entryBlocks(e, false); got[0].Above == 0 {
		t.Error("not first: no space above")
	}
}

func TestDictsForLang(t *testing.T) {
	dir := t.TempDir()
	var ds []*dict.Dict
	for _, n := range []string{"el-en", "fr-en", "grc-en", "webster"} {
		ds = append(ds, dictNamed(t, dir, n))
	}
	names := func(ds []*dict.Dict) []string {
		var out []string
		for _, d := range ds {
			out = append(out, d.Base)
		}
		return out
	}
	for lang, want := range map[string][]string{
		"":      {"el-en", "fr-en", "grc-en", "webster"},
		"grc":   {"grc-en", "el-en", "fr-en", "webster"},
		"fr-CA": {"fr-en", "el-en", "grc-en", "webster"},
		"FR_ca": {"fr-en", "el-en", "grc-en", "webster"},
		"ru":    {"el-en", "fr-en", "grc-en", "webster"},
	} {
		if got := names(dictsForLang(ds, lang)); !slices.Equal(got, want) {
			t.Errorf("%q: %q, want %q", lang, got, want)
		}
	}
}

func dictNamed(t *testing.T, dir, name string) *dict.Dict {
	t.Helper()
	p := filepath.Join(dir, name+".ifo")
	if err := os.WriteFile(p, []byte("bookname="+name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := dict.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestHoldTick(t *testing.T) {
	long := time.Now().Add(-time.Second)
	for _, tt := range []struct {
		name string
		g    kobo.Gesture
		kept bool
	}{
		{"not long enough yet", kobo.Gesture{At: time.Now()}, true},
		{"moved off", kobo.Gesture{At: long, Moved: holdSlop}, false},
		{"another's", kobo.Gesture{At: long, Owner: ownPair}, false},
	} {
		a := &bro{}
		a.bk.hold = &tt.g
		a.holdTick()
		if (a.bk.hold != nil) != tt.kept {
			t.Errorf("%s: kept %v, want %v", tt.name, a.bk.hold != nil, tt.kept)
		}
		if tt.g.Owner == ownHold {
			t.Errorf("%s: took the finger", tt.name)
		}
	}
}
