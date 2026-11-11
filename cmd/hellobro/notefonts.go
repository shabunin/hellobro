package main

// The fonts of a notebook's text: a family by its name, as Xournal++
// writes it, "Bold" and "Italic" after it ("Gentium Book Bold Italic",
// "Go Mono"): Gentium Book's and Go's, ours, or one of the reader's. A
// letter the font lacks (the Kobo's have no Greek, some no Cyrillic) is
// drawn in the first that has it: Gentium Book, Go, STIX Two Math (the
// signs).

import (
	"image"
	"log"
	"os"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gobolditalic"
	"golang.org/x/image/font/gofont/goitalic"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/gofont/gomonobolditalic"
	"golang.org/x/image/font/gofont/gomonoitalic"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"hellobro/third_party/fonts/gentium"
	"hellobro/third_party/fonts/stix"
)

// The styles, as bookFamily's files are: regular, bold, italic, both.
const (
	styleBold   = 1
	styleItalic = 2
)

// Our own families, in their four styles.
const (
	famGo      = "Go"
	famGoMono  = "Go Mono"
	famGentium = "Gentium Book"
)

var ourFonts = map[string][4][]byte{
	famGo:      {goregular.TTF, gobold.TTF, goitalic.TTF, gobolditalic.TTF},
	famGoMono:  {gomono.TTF, gomonobold.TTF, gomonoitalic.TTF, gomonobolditalic.TTF},
	famGentium: {gentium.TTF, gentium.Bold, gentium.Italic, gentium.BoldItalic},
}

// fontName is a family and a style as a text's font says them.
func fontName(fam string, style int) string {
	return fam + [4]string{"", " Bold", " Italic", " Bold Italic"}[style&3]
}

// splitFont is the family a font's name says, as we know it ("" for
// none: Go's), and its style.
func splitFont(name string) (string, int) {
	style := 0
	for {
		switch {
		case strings.HasSuffix(name, " Bold"):
			name, style = strings.TrimSuffix(name, " Bold"), style|styleBold
			continue
		case strings.HasSuffix(name, " Italic"), strings.HasSuffix(name, " Oblique"):
			name, style = name[:strings.LastIndexByte(name, ' ')], style|styleItalic
			continue
		}
		break
	}
	return noteFamily(name), style
}

// noteFamily is the family a text's font names, as we have it ("" for
// none: Go's).
func noteFamily(name string) string {
	for fam := range ourFonts {
		if strings.EqualFold(fam, name) {
			return fam
		}
	}
	if strings.EqualFold(name, "Gentium") {
		return famGentium
	}
	for _, f := range noteFamilies() {
		if f.files[0] != "" && strings.EqualFold(f.name, name) {
			return f.name
		}
	}
	return ""
}

// noteFamilies are the reader's fonts, looked for once.
var noteFamilies = sync.OnceValue(bookFamilies)

// noteTextFaces are the texts' fonts at each size they're drawn (px),
// by name. An export draws in its own goroutine while the loop draws
// pages: the map is locked, and so is each face, as opentype's keep a
// buffer of their own for measuring.
var noteTextFaces struct {
	sync.Mutex
	m     map[noteFaceKey]font.Face
	fonts map[string]*opentype.Font // a file's or our font's, read: by its path, or family and style
}

type noteFaceKey struct {
	name string
	px   int
}

// noteTextFace is Go's regular at px.
func noteTextFace(px int) font.Face { return noteFontFace("", px) }

// noteFontFace is the font name names at px, with the others for what
// it lacks.
func noteFontFace(name string, px int) font.Face {
	px = max(4, px)
	fam, style := splitFont(name)
	if fam == "" {
		fam = famGo
	}
	noteTextFaces.Lock()
	defer noteTextFaces.Unlock()
	k := noteFaceKey{fontName(fam, style), px}
	f, ok := noteTextFaces.m[k]
	if ok {
		return f
	}
	if noteTextFaces.m == nil {
		noteTextFaces.m, noteTextFaces.fonts = map[noteFaceKey]font.Face{}, map[string]*opentype.Font{}
	}
	var chain []font.Face
	for _, o := range []*opentype.Font{noteFont(fam, style), noteFont(famGentium, style), noteFont(famGo, style), noteFont("stix", 0)} {
		if o == nil {
			continue
		}
		if fc, err := opentype.NewFace(o, &opentype.FaceOptions{Size: float64(px), DPI: 72, Hinting: font.HintingFull}); err == nil {
			chain = append(chain, fc)
		}
	}
	f = &lockedFace{f: &fallbackFace{chain}}
	noteTextFaces.m[k] = f
	return f
}

// noteFont is fam's font in style, read once (noteTextFaces locked):
// the regular if it hasn't that style; nil if there's none.
func noteFont(fam string, style int) *opentype.Font {
	var key string
	var data func() ([]byte, error)
	switch {
	case fam == "stix":
		key, data = fam, func() ([]byte, error) { return stix.OTF, nil }
	case ourFonts[fam][0] != nil:
		key, data = fontName(fam, style), func() ([]byte, error) { return ourFonts[fam][style&3], nil }
	default:
		for _, bf := range noteFamilies() {
			if bf.name == fam && bf.files[0] != "" {
				file := bf.files[style&3]
				if file == "" && style&3 == styleBold|styleItalic {
					file = bf.files[styleBold]
				}
				if file == "" {
					file = bf.files[0]
				}
				key, data = file, func() ([]byte, error) { return os.ReadFile(file) }
			}
		}
	}
	if data == nil {
		return nil
	}
	if o, ok := noteTextFaces.fonts[key]; ok {
		return o
	}
	var o *opentype.Font
	b, err := data()
	if err == nil {
		o, err = parseFont(b)
	}
	if err != nil {
		log.Printf("notebook: font %s: %v", key, err)
	}
	noteTextFaces.fonts[key] = o
	return o
}

// fallbackFace is the first of its faces, and for a letter it lacks,
// the first of the others that has it. Its metrics are the first's.
type fallbackFace struct {
	faces []font.Face
}

// of is the face r is drawn in.
func (f *fallbackFace) of(r rune) font.Face {
	for _, fc := range f.faces {
		if _, ok := fc.GlyphAdvance(r); ok {
			return fc
		}
	}
	return f.faces[0]
}

func (f *fallbackFace) Close() error { return nil }

func (f *fallbackFace) Glyph(dot fixed.Point26_6, r rune) (image.Rectangle, image.Image, image.Point, fixed.Int26_6, bool) {
	return f.of(r).Glyph(dot, r)
}

func (f *fallbackFace) GlyphBounds(r rune) (fixed.Rectangle26_6, fixed.Int26_6, bool) {
	return f.of(r).GlyphBounds(r)
}

func (f *fallbackFace) GlyphAdvance(r rune) (fixed.Int26_6, bool) {
	return f.of(r).GlyphAdvance(r)
}

func (f *fallbackFace) Kern(r0, r1 rune) fixed.Int26_6 {
	if a := f.of(r0); a == f.of(r1) {
		return a.Kern(r0, r1)
	}
	return 0
}

func (f *fallbackFace) Metrics() font.Metrics { return f.faces[0].Metrics() }
