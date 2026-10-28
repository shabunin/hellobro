package main

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/image/font/gofont/goregular"
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
