package main

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"

	"hellobro/book"
)

// TestBookFamilies: the Kobo's fonts as families of four styles, Georgia
// first; one-style fonts (CJK, the UI's) left out; the Go fonts last.
func TestBookFamilies(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"georgia.ttf", "georgiab.ttf", "georgiai.ttf", "georgiaz.ttf",
		"KoboNickel.ttf", "KoboNickel-Bold.ttf", "KoboNickel-Italic.ttf", "KoboNickel-BoldItalic.ttf",
		"RakutenSerifApp-Regular.ttf", "RakutenSerifApp-Bold.ttf", "RakutenSerifApp-Italic.ttf",
		"Ryumin.otf", "Gothic MB101.otf", "readme.txt"} {
		os.WriteFile(filepath.Join(dir, n), nil, 0o644)
	}
	t.Setenv("HELLOBRO_FONTS", dir)
	var got []string
	for _, f := range bookFamilies() {
		var files []string
		for _, p := range f.files {
			files = append(files, filepath.Base(p))
		}
		got = append(got, f.name+" "+fmt.Sprint(files))
	}
	want := []string{
		"Georgia [georgia.ttf georgiab.ttf georgiai.ttf georgiaz.ttf]",
		"Kobo Nickel [KoboNickel.ttf KoboNickel-Bold.ttf KoboNickel-Italic.ttf KoboNickel-BoldItalic.ttf]",
		"Rakuten Serif [RakutenSerifApp-Regular.ttf RakutenSerifApp-Bold.ttf RakutenSerifApp-Italic.ttf .]",
		"Go [. . . .]",
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("families:\n%v\nwant\n%v", got, want)
	}
}

// TestParseFontQTD: the Kobo's fonts are zlib'd behind "QTD" and a size;
// KOBO_FONTS=dir (copies of the Kobo's) reads each of its book fonts.
func TestParseFontQTD(t *testing.T) {
	var z bytes.Buffer
	z.WriteString("QTD")
	binary.Write(&z, binary.BigEndian, uint32(len(goregular.TTF)))
	w := zlib.NewWriter(&z)
	w.Write(goregular.TTF)
	w.Close()
	f, err := parseFont(z.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if n, _ := f.Name(nil, 1); n != "Go" {
		t.Errorf("family %q", n)
	}
	if _, err := parseFont(goregular.TTF); err != nil {
		t.Errorf("a plain TTF: %v", err)
	}
	dir := os.Getenv("KOBO_FONTS")
	if dir == "" {
		return
	}
	t.Setenv("HELLOBRO_FONTS", dir)
	for _, fam := range bookFamilies() {
		for _, p := range fam.files {
			if p == "" {
				continue
			}
			b, _ := os.ReadFile(p)
			f, err := parseFont(b)
			if err != nil {
				t.Errorf("%s: %v", p, err)
				continue
			}
			n, _ := f.Name(nil, 4)
			t.Logf("%s: %s", filepath.Base(p), n)
		}
	}
}

func TestMainFamily(t *testing.T) {
	in := func(f int) book.Style { return book.Style(0).InFamily(f) }
	for _, c := range []struct {
		name  string
		spans []book.Span
		want  int
	}{
		{"none", nil, 0},
		{"the reader's", []book.Span{{Text: "text"}}, 0},
		{"a book's", []book.Span{{Text: "head", Style: in(1)}, {Text: "and a lot of it", Style: in(2)}}, 2},
		{"bold's the same", []book.Span{{Text: "text", Style: book.Bold | in(2)}, {Text: "tex", Style: in(1)}}, 2},
		{"a heading in it isn't", []book.Span{{Text: "head", Style: in(1)}, {Text: "and a lot of it"}}, 0},
	} {
		ch := &book.Chapter{Blocks: []book.Block{{Spans: c.spans}}}
		if got := mainFamily(ch); got != c.want {
			t.Errorf("%s: %d, want %d", c.name, got, c.want)
		}
	}
	if got := mainFamily(nil); got != 0 {
		t.Errorf("no chapter: %d", got)
	}
}

// What the book's font hasn't got comes from the reader's, not a box.
func TestOwnFaceFallsBack(t *testing.T) {
	face := func(data []byte) (*opentype.Font, font.Face) {
		o, err := opentype.Parse(data)
		if err != nil {
			t.Fatal(err)
		}
		f, err := opentype.NewFace(o, &opentype.FaceOptions{Size: 30, DPI: 72})
		if err != nil {
			t.Fatal(err)
		}
		return o, f
	}
	o, reg := face(goregular.TTF)
	_, mono := face(gomono.TTF)
	own := &ownFace{Face: reg, f: o, base: mono}
	const missing = '一' // the Go fonts have no CJK; .notdef in both
	if own.has(missing) || !own.has('i') {
		t.Fatalf("has: %v %v", own.has(missing), own.has('i'))
	}
	want, _ := reg.GlyphAdvance('i')
	if got, _ := own.GlyphAdvance('i'); got != want {
		t.Errorf("i: %v, want its own %v", got, want)
	}
	want, _ = mono.GlyphAdvance(missing)
	if got, _ := own.GlyphAdvance(missing); got != want {
		t.Errorf("missing: %v, want the reader's %v", got, want)
	}
	if k := own.Kern('A', missing); k != 0 {
		t.Errorf("kern across fonts: %v", k)
	}
}
