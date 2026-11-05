package dict

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
)

// makeDict writes a StarDict of articles by headword, with forms by
// form in its .syn, in dir; dz: its .dict.dz, in chunks of chunk bytes.
func makeDict(t *testing.T, dir, name, seq string, arts map[string]string, forms map[string]string, dz bool, chunk int) string {
	t.Helper()
	var words []string
	for w := range arts {
		words = append(words, w)
	}
	sort.Strings(words)
	var data, idx bytes.Buffer
	at := map[string]uint32{}
	for i, w := range words {
		at[w] = uint32(i)
		idx.WriteString(w)
		idx.WriteByte(0)
		binary.Write(&idx, binary.BigEndian, uint32(data.Len()))
		binary.Write(&idx, binary.BigEndian, uint32(len(arts[w])))
		data.WriteString(arts[w])
	}
	base := filepath.Join(dir, name)
	ifo := fmt.Sprintf("StarDict's dict ifo file\nversion=2.4.2\nbookname=%s\nwordcount=%d\nidxfilesize=%d\nsametypesequence=%s\n", name+" title", len(words), idx.Len(), seq)
	write := func(ext string, b []byte) {
		if err := os.WriteFile(base+ext, b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(".ifo", []byte(ifo))
	write(".idx", idx.Bytes())
	if len(forms) > 0 {
		var syn bytes.Buffer
		for f, w := range forms {
			syn.WriteString(f)
			syn.WriteByte(0)
			binary.Write(&syn, binary.BigEndian, at[w])
		}
		write(".syn", syn.Bytes())
	}
	if dz {
		write(".dict.dz", dictzipOf(t, data.Bytes(), chunk))
	} else {
		write(".dict", data.Bytes())
	}
	return base + ".ifo"
}

// dictzipOf packs data as dictzip does: chunks that don't lean on the
// ones before, each ended with a flush.
func dictzipOf(t *testing.T, data []byte, chunk int) []byte {
	t.Helper()
	var body bytes.Buffer
	var sizes []uint16
	last := 0
	for p := 0; p < len(data); p += chunk {
		z, _ := flate.NewWriter(&body, 9)
		z.Write(data[p:min(p+chunk, len(data))])
		z.Flush()
		sizes = append(sizes, uint16(body.Len()-last))
		last = body.Len()
	}
	ra := []byte{1, 0}
	ra = binary.LittleEndian.AppendUint16(ra, uint16(chunk))
	ra = binary.LittleEndian.AppendUint16(ra, uint16(len(sizes)))
	for _, s := range sizes {
		ra = binary.LittleEndian.AppendUint16(ra, s)
	}
	extra := []byte{'R', 'A'}
	extra = binary.LittleEndian.AppendUint16(extra, uint16(len(ra)))
	extra = append(extra, ra...)
	out := []byte{0x1f, 0x8b, 8, 4 | 8, 0, 0, 0, 0, 2, 255}
	out = binary.LittleEndian.AppendUint16(out, uint16(len(extra)))
	out = append(out, extra...)
	out = append(out, "a name\x00"...)
	return append(out, body.Bytes()...)
}

func TestFold(t *testing.T) {
	for in, want := range map[string]string{
		"Hello":      "hello",
		"σκῠ́λος":    "σκυλοσ",
		"Σκύλος":     "σκυλοσ",
		"καλὸς":      "καλοσ",
		"καλός":      "καλοσ",
		"ἀλλ᾽":       "αλλ'",
		"ἀλλ’":       "αλλ'",
		"сло́во":     "слово",
		"ёлка":       "елка",
		"ЁЛКА":       "елка",
		"pêche":      "peche",
		"L’Homme":    "l'homme",
		"be­ginning": "beginning",
		"":           "",
	} {
		if got := Fold(in); got != want {
			t.Errorf("Fold(%q): %q, want %q", in, got, want)
		}
	}
}

func TestCandidates(t *testing.T) {
	for in, want := range map[string][]string{
		"word":       {"word"},
		"l'homme":    {"l'homme", "homme"},
		"qu’il":      {"qu’il", "il"},
		"ἀλλ᾽":       {"ἀλλ᾽", "ἀλλ"},
		"well-known": {"well-known", "wellknown", "well", "known"},
	} {
		if got := Candidates(in); !slices.Equal(got, want) {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
}

func TestLookup(t *testing.T) {
	arts := map[string]string{
		"σκῠ́λος":               "skin, hide",
		"σκύλος":                "another",
		"Hund":                  "dog",
		"hund":                  "lower",
		"мой":                   "my",
		"мои":                   "my, plural",
		"homme":                 "man",
		strings.Repeat("x", 40): strings.Repeat("long article ", 2000), // over chunks
	}
	forms := map[string]string{"σκύλεος": "σκῠ́λος", "собаки": "Hund"}
	for _, dz := range []bool{false, true} {
		t.Run(fmt.Sprintf("dz=%v", dz), func(t *testing.T) {
			d, err := Open(makeDict(t, t.TempDir(), "t", "m", arts, forms, dz, 512))
			if err != nil {
				t.Fatal(err)
			}
			if d.Name != "t title" || d.Base != "t" {
				t.Errorf("%q %q", d.Name, d.Base)
			}
			for _, c := range []struct {
				word string
				want []string
			}{
				{"σκύλος", []string{"σκύλος", "σκῠ́λος"}}, // as written first
				{"ΣΚΥΛΟΣ", []string{"σκύλος", "σκῠ́λος"}}, // the same, no nearer one
				{"σκύλεος", []string{"σκῠ́λος"}},          // a form
				{"σκῡ́λεος", []string{"σκῠ́λος"}},         // a form, in other marks
				{"hund", []string{"hund", "Hund"}},
				{"Hund", []string{"Hund", "hund"}},
				{"собаки", []string{"Hund"}},
				{"мои", []string{"мои", "мой"}},
				{"l'homme", []string{"homme"}},
				{"nothing", nil},
			} {
				got, err := d.Lookup(c.word, 5)
				if err != nil {
					t.Fatal(err)
				}
				var words []string
				for _, e := range got {
					words = append(words, e.Word)
					if e.Dict != "t title" || len(e.Fields) != 1 || e.Fields[0].Type != 'm' || e.Fields[0].Text != arts[e.Word] {
						t.Errorf("%s: %s: %+v", c.word, e.Word, e.Fields)
					}
				}
				if !slices.Equal(words, c.want) {
					t.Errorf("%s: %q, want %q", c.word, words, c.want)
				}
			}
			long, err := d.Lookup(strings.Repeat("x", 40), 5)
			if err != nil || len(long) != 1 || long[0].Fields[0].Text != strings.Repeat("long article ", 2000) {
				t.Errorf("over chunks: %v, %d", err, len(long))
			}
			if got, _ := d.Lookup("σκύλος", 1); len(got) != 1 || got[0].Word != "σκύλος" {
				t.Errorf("the limit: %+v", got)
			}
		})
	}
}

func TestParseEntry(t *testing.T) {
	type fs = []Field
	for _, c := range []struct {
		data, seq string
		want      fs
	}{
		{"plain", "m", fs{{'m', "plain"}}},
		{"<b>x</b>", "h", fs{{'h', "<b>x</b>"}}},
		{"sound\x00text", "tm", fs{{'t', "sound"}, {'m', "text"}}},
		{"mone\x00hmore\x00", "", fs{{'m', "one"}, {'h', "more"}}},
		{"W\x00\x00\x00\x02ABmtext\x00", "", fs{{'W', ""}, {'m', "text"}}},
		{"W\x00\x00\xff\xff", "", fs{{'W', ""}}}, // a size past the end
		{"m", "", fs{{'m', ""}}},
		{"", "m", fs{{'m', ""}}},
	} {
		got := parseEntry([]byte(c.data), c.seq)
		if !slices.Equal(got, c.want) {
			t.Errorf("%q %q: %+v, want %+v", c.data, c.seq, got, c.want)
		}
	}
}

func TestFind(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "sub"), 0o755)
	makeDict(t, dir, "b", "m", map[string]string{"a": "b"}, nil, false, 0)
	makeDict(t, filepath.Join(dir, "sub"), "a", "m", map[string]string{"a": "b"}, nil, false, 0)
	os.WriteFile(filepath.Join(dir, "bad.ifo"), []byte("idxoffsetbits=7\n"), 0o644)
	ds, err := Find(dir)
	if err == nil || !strings.Contains(err.Error(), "bad.ifo") {
		t.Errorf("err %v", err)
	}
	var names []string
	for _, d := range ds {
		names = append(names, d.Base)
	}
	if !slices.Equal(names, []string{"b", "a"}) {
		t.Errorf("%q", names)
	}
	if ds, err := Find(filepath.Join(dir, "none")); len(ds) != 0 || err != nil {
		t.Errorf("no directory: %v %v", ds, err)
	}
}

// A broken index or data says so, and doesn't crash.
func TestLookupBroken(t *testing.T) {
	dir := t.TempDir()
	ifo := makeDict(t, dir, "t", "m", map[string]string{"a": "b"}, nil, false, 0)
	base := strings.TrimSuffix(ifo, ".ifo")
	os.Truncate(base+".idx", 4)
	d, _ := Open(ifo)
	if _, err := d.Lookup("a", 1); err == nil {
		t.Error("cut index: no error")
	}
	ifo = makeDict(t, dir, "u", "m", map[string]string{"a": "b"}, nil, false, 0)
	os.Remove(strings.TrimSuffix(ifo, ".ifo") + ".dict")
	d, _ = Open(ifo)
	if _, err := d.Lookup("a", 1); err == nil {
		t.Error("no data: no error")
	}
}

func TestIndexKept(t *testing.T) {
	dir := t.TempDir()
	arts := map[string]string{"a": "first", "b": "second"}
	ifo := makeDict(t, dir, "t", "m", arts, map[string]string{"formofb": "b"}, false, 0)
	hbx := strings.TrimSuffix(ifo, ".ifo") + ".hbx"
	look := func(word string) string {
		t.Helper()
		d, err := Open(ifo)
		if err != nil {
			t.Fatal(err)
		}
		es, err := d.Lookup(word, 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(es) == 0 {
			return ""
		}
		return es[0].Fields[0].Text
	}
	if got := look("formofb"); got != "second" {
		t.Errorf("first look: %q", got)
	}
	st, err := os.Stat(hbx)
	if err != nil {
		t.Fatalf("not kept: %v", err)
	}
	made := st.ModTime()
	if got := look("a"); got != "first" { // the kept one, as it is
		t.Errorf("second look: %q", got)
	}
	if st, _ := os.Stat(hbx); !st.ModTime().Equal(made) {
		t.Error("made again, for nothing")
	}
	// the dictionary's changed: its sizes aren't the kept one's
	os.Remove(strings.TrimSuffix(ifo, ".ifo") + ".syn")
	makeDict(t, dir, "t", "m", map[string]string{"a": "first", "b": "second", "cc": "third"}, nil, false, 0)
	if got := look("cc"); got != "third" {
		t.Errorf("new dictionary: %q", got)
	}
	if got := look("formofb"); got != "" {
		t.Errorf("its old form: %q", got)
	}
	// a kept one that's broken is made again
	os.WriteFile(hbx, []byte("hbx1 and then nothing good, of no use at all"), 0o644)
	if got := look("a"); got != "first" {
		t.Errorf("broken: %q", got)
	}
	os.WriteFile(hbx, nil, 0o644)
	if got := look("a"); got != "first" {
		t.Errorf("empty: %q", got)
	}
	// and if it can't be written, it works anyway
	os.Remove(hbx)
	os.Mkdir(hbx+".new", 0o755)
	if got := look("a"); got != "first" {
		t.Errorf("not writable: %q", got)
	}
}

func TestWriteIndex(t *testing.T) {
	ifo := makeDict(t, t.TempDir(), "t", "m", map[string]string{"σκῠ́λος": "x"}, map[string]string{"σκύλεος": "σκῠ́λος"}, false, 0)
	if err := WriteIndex(ifo); err != nil {
		t.Fatal(err)
	}
	base := strings.TrimSuffix(ifo, ".ifo")
	os.Remove(base + ".syn") // the kept one no longer fits: it's the size that tells
	d, _ := Open(ifo)
	if es, err := d.Lookup("σκυλεος", 1); err != nil || len(es) != 0 {
		t.Errorf("%+v %v", es, err)
	}
}

// Two at once, as a second word's held while the first is looked for.
func TestLookupTogether(t *testing.T) {
	arts := map[string]string{}
	for i := range 3000 {
		arts[fmt.Sprintf("w%d", i)] = strings.Repeat(fmt.Sprintf("article %d ", i), 40)
	}
	d, err := Open(makeDict(t, t.TempDir(), "t", "m", arts, nil, true, 2048))
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for g := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 200 {
				w := fmt.Sprintf("w%d", (i*37+g*11)%3000)
				es, err := d.Lookup(w, 1)
				if err != nil || len(es) != 1 || es[0].Fields[0].Text != arts[w] {
					t.Errorf("%s: %v", w, err)
					return
				}
			}
		}()
	}
	wg.Wait()
}
