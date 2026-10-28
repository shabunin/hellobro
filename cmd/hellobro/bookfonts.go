package main

// The reader's fonts: the Kobo's own (Georgia, Caecilia, Malabar…),
// read where Nickel keeps them, each with its bold and italics; the Go
// fonts when there are none (the harness). Code is set in Go Mono.

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gobolditalic"
	"golang.org/x/image/font/gofont/goitalic"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"

	"hellobro/book"
)

const koboFonts = "/usr/local/Trolltech/QtEmbedded-4.6.2-arm/lib/fonts"

// bookFamily is a font in its four styles: regular, bold, italic, both.
type bookFamily struct {
	name  string
	files [4]string // "" for the Go fonts
}

// bookFamilies are the fonts there are, Georgia first if it's there;
// the Go fonts last, always.
func bookFamilies() []bookFamily {
	dir := os.Getenv("HELLOBRO_FONTS")
	if dir == "" {
		dir = koboFonts
	}
	ents, _ := os.ReadDir(dir)
	files := map[string]string{} // lower-cased name, no extension: the file
	for _, e := range ents {
		n := e.Name()
		if ext := strings.ToLower(filepath.Ext(n)); ext == ".ttf" || ext == ".otf" {
			files[strings.ToLower(strings.TrimSuffix(n, filepath.Ext(n)))] = filepath.Join(dir, n)
		}
	}
	var fams []bookFamily
	for key, f := range files {
		base, ok := strings.CutSuffix(key, "-regular")
		if !ok {
			base = key
			if strings.Contains(key, "-") {
				continue
			}
		}
		bold, italic := files[base+"-bold"], files[base+"-italic"]
		if bold == "" || italic == "" {
			continue // not a book's font: one style only (CJK, the UI's)
		}
		fams = append(fams, bookFamily{familyName(filepath.Base(f)), [4]string{f, bold, italic, files[base+"-bolditalic"]}})
	}
	slices.SortFunc(fams, func(a, b bookFamily) int { return strings.Compare(a.name, b.name) })
	if g := files["georgia"]; g != "" && files["georgiab"] != "" && files["georgiai"] != "" {
		fams = append([]bookFamily{{"Georgia", [4]string{g, files["georgiab"], files["georgiai"], files["georgiaz"]}}}, fams...)
	}
	return append(fams, bookFamily{name: "Go"})
}

// familyName is a font file's family as said: KoboNickel-Bold.ttf is
// "Kobo Nickel", RakutenSerifApp-Regular.otf "Rakuten Serif".
func familyName(file string) string {
	n := strings.TrimSuffix(file, filepath.Ext(file))
	n, _, _ = strings.Cut(n, "-")
	n = strings.TrimSuffix(n, "App")
	var b strings.Builder
	rs := []rune(n)
	for i, r := range rs {
		if i > 0 && unicode.IsUpper(r) && (unicode.IsLower(rs[i-1]) || i+1 < len(rs) && unicode.IsLower(rs[i+1])) {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// bookFaces are a family's faces in the sizes asked for, for book.Layout
// and for drawing.
type bookFaces struct {
	fam   bookFamily
	fonts [5]*opentype.Font // the four styles, then mono
	faces map[[2]int]font.Face
}

func newBookFaces(fam bookFamily) *bookFaces {
	f := &bookFaces{fam: fam, faces: map[[2]int]font.Face{}}
	gos := [5][]byte{goregular.TTF, gobold.TTF, goitalic.TTF, gobolditalic.TTF, gomono.TTF}
	for i := range f.fonts {
		var data []byte
		if i < 4 && fam.files[i] != "" {
			b, err := os.ReadFile(fam.files[i])
			if err != nil {
				log.Printf("book: %v", err)
			}
			data = b
		}
		if data == nil && i == 3 && fam.files[0] != "" { // no bold italic: bold
			f.fonts[3] = f.fonts[1]
			continue
		}
		if data == nil {
			data = gos[i]
		}
		o, err := parseFont(data)
		if err != nil {
			log.Printf("book: %s: %v", fam.name, err)
			o, _ = opentype.Parse(gos[i])
		}
		f.fonts[i] = o
	}
	return f
}

// parseFont reads a font file as the Kobo keeps them: "QTD", the
// font's size (big-endian) and the font, zlib'd; often a collection, of
// which the first is the one. A plain TTF or OTF is read as it is.
func parseFont(data []byte) (*opentype.Font, error) {
	if len(data) > 7 && string(data[:3]) == "QTD" {
		size := binary.BigEndian.Uint32(data[3:7])
		if size > 32<<20 {
			return nil, errors.New("QTD: too big")
		}
		z, err := zlib.NewReader(bytes.NewReader(data[7:]))
		if err != nil {
			return nil, err
		}
		raw, err := io.ReadAll(io.LimitReader(z, int64(size)+1))
		if err != nil {
			return nil, err
		}
		data = raw
	}
	if len(data) > 4 && string(data[:4]) == "ttcf" {
		c, err := opentype.ParseCollection(data)
		if err != nil {
			return nil, err
		}
		return c.Font(0)
	}
	return opentype.Parse(data)
}

// Face is the font for style s, px pixels.
func (f *bookFaces) Face(s book.Style, px int) font.Face {
	i := int(s & (book.Bold | book.Italic))
	if s&book.Mono != 0 {
		i = 4
		px = px * 9 / 10 // a monospace looks big beside a book's text
	}
	k := [2]int{i, px}
	if fc := f.faces[k]; fc != nil {
		return fc
	}
	fc, err := opentype.NewFace(f.fonts[i], &opentype.FaceOptions{Size: float64(px), DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		fc, _ = opentype.NewFace(f.fonts[0], &opentype.FaceOptions{Size: float64(px), DPI: 72, Hinting: font.HintingFull})
	}
	if len(f.faces) > 32 {
		clear(f.faces)
	}
	f.faces[k] = fc
	return fc
}
