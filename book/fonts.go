package book

// A book's own fonts: its @font-face rules, found as its stylesheets are
// read, and the files they name, made fit to parse. A span in one of
// them has the family in its Style. Parsing and drawing them is the
// reader's.

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// The ways of hiding a font in an EPUB (not DRM: the key is in the book).
const (
	obfIDPF  = "http://www.idpf.org/2008/embedding"
	obfAdobe = "http://ns.adobe.com/pdf/enc#RC"
)

const maxFont = 64 << 20

// fontFace is a @font-face rule as written.
type fontFace struct {
	family string // lower-cased, as the declarations are
	weight int
	italic bool
	urls   []string // of its src, in order
}

// bookFace is a face of a family, in a file in the zip.
type bookFace struct {
	family int // from 1
	weight int
	italic bool
	file   string
}

var (
	faceFamily = regexp.MustCompile(`(?i)font-family\s*:\s*([^;]+)`)
	faceWeight = regexp.MustCompile(`(?i)font-weight\s*:\s*(\w+)`)
	faceStyle  = regexp.MustCompile(`(?i)font-style\s*:\s*(\w+)`)
	faceURL    = regexp.MustCompile(`(?i)url\(\s*["']?([^"')]+?)["']?\s*\)`)
)

// parseFontFace reads a @font-face's body. Not by parseDecls: that
// lower-cases what the zip's paths have in capitals, and a data: URL
// has semicolons.
func parseFontFace(body string) fontFace {
	f := fontFace{weight: 400}
	if m := faceFamily.FindStringSubmatch(body); m != nil {
		f.family = familyName(m[1])
	}
	if m := faceWeight.FindStringSubmatch(body); m != nil {
		switch w := strings.ToLower(m[1]); w {
		case "bold":
			f.weight = 700
		default:
			if n, err := strconv.Atoi(w); err == nil {
				f.weight = n
			}
		}
	}
	if m := faceStyle.FindStringSubmatch(body); m != nil {
		s := strings.ToLower(m[1])
		f.italic = s == "italic" || s == "oblique"
	}
	for _, m := range faceURL.FindAllStringSubmatch(body, -1) {
		f.urls = append(f.urls, m[1])
	}
	return f
}

// familyName is a name in a font-family's list, as a key.
func familyName(s string) string {
	return strings.ToLower(strings.Trim(strings.TrimSpace(s), `"'`))
}

// addFaces takes the faces of a sheet in the file from: each from the
// first of its files the book has, a WOFF2 (which we can't read)
// only if nothing else is. A family with no file isn't one.
func (b *Book) addFaces(from string, s *sheet) {
	if len(s.faces) == 0 {
		return
	}
	b.fontMu.Lock()
	defer b.fontMu.Unlock()
	for _, f := range s.faces {
		file := ""
		for _, u := range f.urls {
			if name := b.fontFile(from, u); name != "" && (file == "" || isWOFF2Name(file)) {
				file = name
			}
		}
		if file == "" || f.family == "" {
			continue
		}
		fam := slices.Index(b.families, f.family) + 1
		if fam == 0 {
			if len(b.families) == maxFamily {
				continue
			}
			b.families = append(b.families, f.family)
			fam = len(b.families)
		}
		if face := (bookFace{fam, f.weight, f.italic, file}); !slices.Contains(b.faces, face) { // a chapter's <style>, read again
			b.faces = append(b.faces, face)
		}
	}
}

// fontFile is the file a url in the file from names, "" if none. Not all
// books say where it is: Calibre's write the path from the top of the
// book, others have the wrong capitals. Then one by that name anywhere.
func (b *Book) fontFile(from, u string) string {
	name, _ := resolve(from, u)
	if b.files[name] != nil {
		return name
	}
	base, found := path.Base(name), ""
	for f := range b.files {
		if strings.EqualFold(path.Base(f), base) && (found == "" || f < found) {
			found = f
		}
	}
	return found
}

func isWOFF2Name(file string) bool { return strings.HasSuffix(strings.ToLower(file), ".woff2") }

// familyOf is the first family of a font-family's value (lower-cased)
// the book has; 0 if it has none of them.
func (b *Book) familyOf(val string) int {
	b.fontMu.Lock()
	defer b.fontMu.Unlock()
	if len(b.families) == 0 {
		return 0
	}
	for _, n := range strings.Split(val, ",") {
		if i := slices.Index(b.families, familyName(n)); i >= 0 {
			return i + 1
		}
	}
	return 0
}

// FontFamilies is how many fonts of its own the book was found to have,
// in the chapters read so far.
func (b *Book) FontFamilies() int {
	b.fontMu.Lock()
	defer b.fontMu.Unlock()
	return len(b.families)
}

// FontFile is the file of the family's face nearest to s's bold and
// italics: its slant first, then its weight, as a browser picks. There's
// no bold made up where the book has none. "" if no such family.
func (b *Book) FontFile(family int, s Style) string {
	b.fontMu.Lock()
	defer b.fontMu.Unlock()
	want := 400
	if s&Bold != 0 {
		want = 700
	}
	file, best := "", 0
	for _, f := range b.faces {
		if f.family != family {
			continue
		}
		score := max(f.weight-want, want-f.weight)
		if f.italic != (s&Italic != 0) {
			score += 10000
		}
		if file == "" || score < best {
			file, best = f.file, score
		}
	}
	return file
}

// FontData is the font in file: not hidden, and if it was a WOFF,
// the plain font in it.
func (b *Book) FontData(file string) ([]byte, error) {
	data, err := b.read(file)
	if err != nil {
		return nil, err
	}
	if algo := b.hidden[file]; algo != "" && !isFont(data) { // some are listed, and aren't
		if data, err = unhide(data, algo, b.ids); err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
	}
	switch {
	case bytes.HasPrefix(data, []byte("wOFF")):
		return unWOFF(data)
	case bytes.HasPrefix(data, []byte("wOF2")):
		return nil, fmt.Errorf("%s: WOFF2, which we can't read", file)
	}
	return data, nil
}

// readEncryption reads which files are hidden, and how. Other
// algorithms are DRM: such a font stays as it is and won't parse.
func (b *Book) readEncryption() {
	data, err := b.read("META-INF/encryption.xml")
	if err != nil {
		return
	}
	var e struct {
		Data []struct {
			Method struct {
				Algorithm string `xml:"Algorithm,attr"`
			} `xml:"EncryptionMethod"`
			Ref struct {
				URI string `xml:"URI,attr"`
			} `xml:"CipherData>CipherReference"`
		} `xml:"EncryptedData"`
	}
	if xml.Unmarshal(data, &e) != nil {
		return
	}
	for _, d := range e.Data {
		if a := d.Method.Algorithm; a == obfIDPF || a == obfAdobe {
			name, _ := resolve("", d.Ref.URI)
			if b.hidden == nil {
				b.hidden = map[string]string{}
			}
			b.hidden[name] = a
		}
	}
}

// isFont says if data starts as a font does.
func isFont(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	switch string(data[:4]) {
	case "\x00\x01\x00\x00", "OTTO", "true", "ttcf", "wOFF", "wOF2":
		return true
	}
	return false
}

// unhide undoes a font's obfuscation: its start, XORed with a key made
// of the book's identifier. The one that should be is first in ids, but
// not all books hold to that: the one that gives a font is the one.
func unhide(data []byte, algo string, ids []string) ([]byte, error) {
	for _, id := range ids {
		key, n := hideKey(algo, id)
		if key == nil {
			continue
		}
		head := slices.Clone(data[:min(n, len(data))])
		for i := range head {
			head[i] ^= key[i%len(key)]
		}
		if isFont(head) {
			return append(head, data[len(head):]...), nil
		}
	}
	return nil, errors.New("hidden, and no identifier in the book opens it")
}

// hideKey is the key for the id, and how many bytes of the file it
// hides; nil if the id can't be one.
func hideKey(algo, id string) ([]byte, int) {
	if algo == obfIDPF {
		h := sha1.Sum([]byte(strings.Map(func(r rune) rune {
			if strings.ContainsRune(" \t\r\n", r) {
				return -1
			}
			return r
		}, id)))
		return h[:], 1040
	}
	u := strings.ReplaceAll(strings.TrimPrefix(strings.TrimSpace(id), "urn:uuid:"), "-", "")
	key, err := hex.DecodeString(u)
	if err != nil || len(key) != 16 {
		return nil, 0
	}
	return key, 1024
}

// unWOFF is the font in a WOFF: its tables, each zlib'd or not, put back
// as a plain font has them. The checksums aren't looked at.
func unWOFF(w []byte) ([]byte, error) {
	if len(w) < 44 {
		return nil, errors.New("WOFF: too short")
	}
	be := binary.BigEndian
	n := int(be.Uint16(w[12:]))
	if n == 0 || 44+20*n > len(w) {
		return nil, errors.New("WOFF: bad table count")
	}
	size := uint64(12 + 16*n)
	for i := range n {
		size += (uint64(be.Uint32(w[44+20*i+12:])) + 3) &^ 3
	}
	if size > maxFont {
		return nil, errors.New("WOFF: too big")
	}
	out := make([]byte, 12+16*n, size)
	copy(out, w[4:8]) // its flavour: TrueType, or OpenType's
	log2 := 0
	for 2<<log2 <= n {
		log2++
	}
	be.PutUint16(out[4:], uint16(n))
	be.PutUint16(out[6:], uint16(16<<log2))
	be.PutUint16(out[8:], uint16(log2))
	be.PutUint16(out[10:], uint16(16*n-16<<log2))
	for i := range n {
		e := w[44+20*i:]
		off, comp, orig := uint64(be.Uint32(e[4:])), uint64(be.Uint32(e[8:])), uint64(be.Uint32(e[12:]))
		if off+comp > uint64(len(w)) || comp > orig {
			return nil, errors.New("WOFF: bad table")
		}
		table := w[off : off+comp]
		if comp < orig {
			z, err := zlib.NewReader(bytes.NewReader(table))
			if err != nil {
				return nil, fmt.Errorf("WOFF: %w", err)
			}
			if table, err = io.ReadAll(io.LimitReader(z, int64(orig))); err != nil {
				return nil, fmt.Errorf("WOFF: %w", err)
			}
			if uint64(len(table)) != orig {
				return nil, errors.New("WOFF: a table came out short")
			}
		}
		at := len(out)
		dir := out[12+16*i:]
		copy(dir, e[:4])        // its tag
		copy(dir[4:], e[16:20]) // its checksum
		be.PutUint32(dir[8:], uint32(at))
		be.PutUint32(dir[12:], uint32(orig))
		out = append(out, table...)
		out = append(out, make([]byte, -len(out)&3)...)
	}
	return out, nil
}
