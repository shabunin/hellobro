package book

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
)

func TestParseFontFace(t *testing.T) {
	for _, c := range []struct {
		body string
		want fontFace
	}{
		{`font-family: "Bell MT"; src: url("../Fonts/Bell.TTF") format("truetype")`,
			fontFace{family: "bell mt", weight: 400, urls: []string{"../Fonts/Bell.TTF"}}},
		{`FONT-FAMILY:Bell;font-weight:bold;font-style:italic;src:url(a.otf),url('b.woff')`,
			fontFace{family: "bell", weight: 700, italic: true, urls: []string{"a.otf", "b.woff"}}},
		{`font-family: 'x'; font-weight: 300; font-style: oblique; src: local("x"), url( a.ttf )`,
			fontFace{family: "x", weight: 300, italic: true, urls: []string{"a.ttf"}}},
		{`font-family: x; font-style: normal; src: url(data:font/woff;base64,AAAA)`,
			fontFace{family: "x", weight: 400, urls: []string{"data:font/woff;base64,AAAA"}}},
	} {
		if got := parseFontFace(c.body); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s:\n got %+v\nwant %+v", c.body, got, c.want)
		}
	}
}

// fontsBook is a book of one chapter, with a family in a sheet and
// another in the chapter's <style>; body what's in the chapter.
func fontsBook(t *testing.T, body string) *Book {
	t.Helper()
	b, err := Open(testEPUB(t, map[string]string{
		"OPS/Fonts/Bell.ttf": string(goregular.TTF), "OPS/Fonts/Bell-B.woff2": "", "OPS/text/in.otf": string(goregular.TTF),
		"OPS/s.css": `@font-face { font-family: "Bell MT"; src: url(Fonts/Bell.ttf) }
@font-face { font-family: "Bell MT"; font-weight: bold; src: url(Fonts/Bell-B.woff2), url(Fonts/Bell.ttf) }
@font-face { font-family: Gone; src: url(Fonts/nothere.ttf) }
@font-face { font-family: Top; src: url(OPS/Fonts/Bell.ttf) }
@font-face { font-family: Caps; src: url(fonts/BELL.TTF) }
.bell { font-family: 'Bell MT', serif } .plain { font-family: serif } .mono { font-family: monospace }
.gone { font-family: Gone, "Courier New" } .inh { font-family: inherit } .in { font-family: Inner } .top { font-family: Top } .caps { font-family: Caps }`,
		"OPS/text/one.xhtml":   `<html><head><link rel="stylesheet" href="../s.css"/><style>@font-face { font-family: Inner; src: url(in.otf) }</style></head><body>` + body + `</body></html>`,
		"OPS/text/two b.xhtml": `<html><body><p>two</p></body></html>`,
	}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })
	return b
}

func TestFamilyInChapter(t *testing.T) {
	for _, c := range []struct {
		body string
		want Style
	}{
		{`<p>x</p>`, 0},
		{`<p class="bell">x</p>`, Style(0).InFamily(1)},
		{`<p class="in">x</p>`, Style(0).InFamily(4)},
		{`<div class="bell"><p><b>x</b></p></div>`, Bold.InFamily(1)},                      // it's inherited
		{`<div class="bell"><p class="plain">x</p></div>`, 0},                              // and left for the reader's
		{`<div class="bell"><p class="inh">x</p></div>`, Style(0).InFamily(1)},             // inherit changes nothing
		{`<div class="bell"><p class="mono">x</p></div>`, Mono},                            // nor do we have it, but it's a mono
		{`<p class="top">x</p>`, Style(0).InFamily(2)},                                     // a path from the top of the book
		{`<p class="caps">x</p>`, Style(0).InFamily(3)},                                    // the wrong capitals
		{`<p class="gone">x</p>`, Mono},                                                    // no file, no family
		{`<p><code class="bell">x</code></p>`, Style(0).InFamily(1)},                       // the book's font over code's
		{`<p style="font-family: &quot;BELL mt&quot;, serif">x</p>`, Style(0).InFamily(1)}, // any case
	} {
		ch, err := fontsBook(t, c.body).Chapter(0)
		if err != nil {
			t.Fatal(err)
		}
		if len(ch.Blocks) != 1 || len(ch.Blocks[0].Spans) != 1 {
			t.Errorf("%s: %+v", c.body, ch.Blocks)
		} else if got := ch.Blocks[0].Spans[0].Style; got != c.want {
			t.Errorf("%s: style %#x, want %#x", c.body, got, c.want)
		}
	}
}

// rel="Stylesheet", as some books have it, is a stylesheet too.
func TestLinkRelAnyCase(t *testing.T) {
	b, err := Open(testEPUB(t, map[string]string{
		"OPS/s.css":            `.big { font-weight: bold }`,
		"OPS/text/one.xhtml":   `<html><head><link rel="Stylesheet" href="../s.css"/></head><body><p class="big">x</p></body></html>`,
		"OPS/text/two b.xhtml": `<html><body><p>two</p></body></html>`,
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	ch, err := b.Chapter(0)
	if err != nil {
		t.Fatal(err)
	}
	if got := ch.Blocks[0].Spans[0].Style; got != Bold {
		t.Errorf("style %#x", got)
	}
}

func TestFamilyStyleKeepsFlags(t *testing.T) {
	s := (Bold | Italic | Sup).InFamily(7)
	if s.Family() != 7 || s&^(Sup|Sub) != (Bold|Italic).InFamily(7) {
		t.Errorf("%#x", s)
	}
	if s = s.InFamily(0); s != Bold|Italic|Sup {
		t.Errorf("%#x", s)
	}
}

func TestFontFile(t *testing.T) {
	b := &Book{faces: []bookFace{
		{1, 400, false, "r"}, {1, 700, false, "b"}, {1, 400, true, "i"},
		{2, 300, false, "light"}, {2, 500, false, "medium"},
		{3, 400, true, "only-i"},
	}}
	for _, c := range []struct {
		fam  int
		s    Style
		want string
	}{
		{1, 0, "r"}, {1, Bold, "b"}, {1, Italic, "i"},
		{1, Bold | Italic, "i"}, // the slant first
		{2, 0, "light"}, {2, Bold, "medium"},
		{3, 0, "only-i"}, {3, Bold, "only-i"},
		{9, 0, ""},
	} {
		if got := b.FontFile(c.fam, c.s); got != c.want {
			t.Errorf("family %d, style %d: %q, want %q", c.fam, c.s, got, c.want)
		}
	}
}

// hide is what a book's maker does to a font, hideKey's way.
func hide(data, key []byte, n int) string {
	out := bytes.Clone(data)
	for i := range n {
		out[i] ^= key[i%len(key)]
	}
	return string(out)
}

// toWOFF packs a plain font in a WOFF, with what's worth it zlib'd.
func toWOFF(t *testing.T, sfnt []byte) []byte {
	t.Helper()
	be := binary.BigEndian
	n := int(be.Uint16(sfnt[4:]))
	head := make([]byte, 44+20*n)
	copy(head, "wOFF")
	copy(head[4:], sfnt[:4])
	be.PutUint16(head[12:], uint16(n))
	var body bytes.Buffer
	for i := range n {
		d := sfnt[12+16*i:]
		off, length := be.Uint32(d[8:]), be.Uint32(d[12:])
		table := sfnt[off : off+length]
		var z bytes.Buffer
		w := zlib.NewWriter(&z)
		w.Write(table)
		w.Close()
		if z.Len() >= len(table) {
			z = *bytes.NewBuffer(table)
		}
		e := head[44+20*i:]
		copy(e, d[:4])
		be.PutUint32(e[4:], uint32(len(head)+body.Len()))
		be.PutUint32(e[8:], uint32(z.Len()))
		be.PutUint32(e[12:], length)
		copy(e[16:], d[4:8])
		body.Write(z.Bytes())
		body.Write(make([]byte, -body.Len()&3))
	}
	return append(head, body.Bytes()...)
}

func TestFontData(t *testing.T) {
	const uuid = "urn:uuid:12345678-90ab-cdef-1234-567890abcdef"
	const uid = "\n  " + uuid + " "
	sum := sha1.Sum([]byte(uuid))
	ad, _ := hex.DecodeString(strings.ReplaceAll(strings.TrimPrefix(uuid, "urn:uuid:"), "-", ""))
	font := goregular.TTF
	opf := func(ids string) string {
		return `<?xml version="1.0"?><package unique-identifier="main" xmlns="http://www.idpf.org/2007/opf" xmlns:dc="http://purl.org/dc/elements/1.1/">
<metadata><dc:title>A</dc:title>` + ids + `</metadata>
<manifest><item id="c1" href="text/one.xhtml"/></manifest><spine><itemref idref="c1"/></spine></package>`
	}
	right := `<dc:identifier>isbn</dc:identifier><dc:identifier id="main">` + uid + `</dc:identifier>`
	enc := func(algo string) string {
		return `<encryption xmlns="urn:oasis:names:tc:opendocument:xmlns:container" xmlns:enc="http://www.w3.org/2001/04/xmlenc#"><enc:EncryptedData>
<enc:EncryptionMethod Algorithm="` + algo + `"/><enc:CipherData><enc:CipherReference URI="OPS/f%20a.ttf"/></enc:CipherData></enc:EncryptedData></encryption>`
	}
	for _, c := range []struct {
		name         string
		ids, enc, in string
		fails        string
	}{
		{name: "plain", ids: right, in: string(font)},
		{name: "IDPF", ids: right, enc: enc(obfIDPF), in: hide(font, sum[:], 1040)},
		{name: "Adobe", ids: right, enc: enc(obfAdobe), in: hide(font, ad, 1024)},
		{name: "IDPF, not by the unique identifier", ids: `<dc:identifier id="main">other</dc:identifier><dc:identifier>` + uuid + `</dc:identifier>`, enc: enc(obfIDPF), in: hide(font, sum[:], 1040)},
		{name: "listed, but not hidden", ids: right, enc: enc(obfIDPF), in: string(font)},
		{name: "WOFF", ids: right, in: string(toWOFF(t, font))},
		{name: "IDPF, a WOFF", ids: right, enc: enc(obfIDPF), in: hide(toWOFF(t, font), sum[:], 1040)},
		{name: "no key opens it", ids: `<dc:identifier id="main">other</dc:identifier>`, enc: enc(obfIDPF), in: hide(font, sum[:], 1040), fails: "no identifier"},
		{name: "DRM", ids: right, enc: enc("http://www.w3.org/2001/04/xmlenc#aes256-cbc"), in: hide(font, sum[:], 1040)},
		{name: "WOFF2", ids: right, in: "wOF2 and the rest", fails: "WOFF2"},
		{name: "short WOFF", ids: right, in: "wOFF", fails: "too short"},
	} {
		t.Run(c.name, func(t *testing.T) {
			files := map[string]string{"OPS/book.opf": opf(c.ids), "OPS/f a.ttf": c.in, "OPS/text/one.xhtml": `<html><body><p>x</p></body></html>`}
			if c.enc != "" {
				files["META-INF/encryption.xml"] = c.enc
			}
			b, err := Open(testEPUB(t, files))
			if err != nil {
				t.Fatal(err)
			}
			defer b.Close()
			data, err := b.FontData("OPS/f a.ttf")
			if c.fails != "" {
				if err == nil || !strings.Contains(err.Error(), c.fails) {
					t.Fatalf("err %v, want %q", err, c.fails)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if c.name == "DRM" { // it reads, but as what it is: not a font
				if _, err := opentype.Parse(data); err == nil {
					t.Fatal("it parsed")
				}
				return
			}
			f, err := opentype.Parse(data)
			if err != nil {
				t.Fatal(err)
			}
			if f.NumGlyphs() < 100 {
				t.Errorf("%d glyphs", f.NumGlyphs())
			}
		})
	}
}

func TestUnWOFFBad(t *testing.T) {
	w := toWOFF(t, goregular.TTF)
	be := binary.BigEndian
	for name, spoil := range map[string]func(b []byte){
		"no tables":          func(b []byte) { be.PutUint16(b[12:], 0) },
		"too many tables":    func(b []byte) { be.PutUint16(b[12:], 60000) },
		"table past the end": func(b []byte) { be.PutUint32(b[44+4:], 1<<31) },
		"huge":               func(b []byte) { be.PutUint32(b[44+12:], 1<<31) },
		"not zlib":           func(b []byte) { o := be.Uint32(b[44+20*5+4:]); b[o] ^= 0xff },
	} {
		b := bytes.Clone(w)
		spoil(b)
		if _, err := unWOFF(b); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}
