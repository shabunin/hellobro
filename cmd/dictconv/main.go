// dictconv makes a StarDict dictionary for the reader (dict, bookdict.go)
// of Wiktionary's words as kaikki.org gives them: one JSON object a line.
// On the PC, as the files are big. Each word's article is its parts of
// speech with their meanings; its other forms go in the .syn, so a word
// in a book finds its entry as it's written there.
//
//	go run ./cmd/dictconv -name "Ancient Greek – English" kaikki.jsonl out/grc-en
//
// Wiktionary's text is CC BY-SA: the .ifo says so, whoever gets the file.
package main

import (
	"bufio"
	"bytes"
	"compress/flate"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"hash/crc32"
	"html"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"hellobro/dict"
)

// dzChunk is dictzip's own size: compressed, it fits the 16 bits it has.
const dzChunk = 58315

// maxWord is the longest word StarDict takes, in bytes.
const maxWord = 255

// A line of kaikki's, what's used of it.
type sourceEntry struct {
	Word  string `json:"word"`
	Pos   string `json:"pos"`
	Heads []struct {
		Expansion string `json:"expansion"`
	} `json:"head_templates"`
	Forms []struct {
		Form string   `json:"form"`
		Tags []string `json:"tags"`
	} `json:"forms"`
	Senses []struct {
		Glosses []string `json:"glosses"`
		Tags    []string `json:"tags"`
	} `json:"senses"`
}

// article is a word's entries so far.
type article struct {
	html  strings.Builder
	forms map[string]bool
}

// skipForm says what's in forms but no form of the word in a book.
func skipForm(form string, tags []string) bool {
	if form == "" || form == "-" || form == "—" || strings.ContainsAny(form, " \t") || len(form) > maxWord {
		return true
	}
	for _, t := range tags {
		switch t {
		case "romanization", "table-tags", "inflection-template", "class", "transliteration":
			return true
		}
	}
	return false
}

// entryHTML is one entry as an article's part: how it's headed (its form
// with its accents, its gender, its genitive…) or its part of speech, and
// its meanings. "" if it has none.
func entryHTML(e *sourceEntry) string {
	var ms []string
	for _, s := range e.Senses {
		if len(s.Glosses) == 0 || slices.Contains(s.Tags, "no-gloss") {
			continue
		}
		ms = append(ms, html.EscapeString(strings.Join(s.Glosses, ": ")))
	}
	if len(ms) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("<p><i>")
	b.WriteString(html.EscapeString(e.Pos))
	b.WriteString("</i>")
	if len(e.Heads) > 0 && e.Heads[0].Expansion != "" {
		b.WriteString(" ")
		b.WriteString(html.EscapeString(e.Heads[0].Expansion))
	}
	b.WriteString("</p>")
	if len(ms) == 1 {
		b.WriteString("<p>" + ms[0] + "</p>")
		return b.String()
	}
	b.WriteString("<ol>")
	for _, m := range ms {
		b.WriteString("<li>" + m + "</li>")
	}
	b.WriteString("</ol>")
	return b.String()
}

// readSource reads the lines of r into articles by word.
func readSource(r io.Reader) (map[string]*article, error) {
	arts := map[string]*article{}
	br := bufio.NewReaderSize(r, 1<<20)
	bad := 0
	for {
		line, err := br.ReadBytes('\n')
		if len(bytes.TrimSpace(line)) > 0 {
			var e sourceEntry
			if jerr := json.Unmarshal(line, &e); jerr != nil {
				if err != io.EOF { // the last may be cut short, a sample's
					bad++
				}
			} else if e.Word != "" && len(e.Word) <= maxWord {
				if h := entryHTML(&e); h != "" {
					a := arts[e.Word]
					if a == nil {
						a = &article{forms: map[string]bool{}}
						arts[e.Word] = a
					}
					a.html.WriteString(h)
					for _, f := range e.Forms {
						if !skipForm(f.Form, f.Tags) && f.Form != e.Word {
							a.forms[f.Form] = true
						}
					}
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	if bad > 0 {
		log.Printf("%d lines not read", bad)
	}
	return arts, nil
}

// compareWords is StarDict's order: ASCII's case not minded, then as bytes.
func compareWords(a, b string) int {
	if c := strings.Compare(asciiLower(a), asciiLower(b)); c != 0 {
		return c
	}
	return strings.Compare(a, b)
}

func asciiLower(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + 32
		}
		return r
	}, s)
}

// write writes the dictionary at base, the path without its extensions.
func write(base, name string, arts map[string]*article) error {
	words := make([]string, 0, len(arts))
	for w := range arts {
		words = append(words, w)
	}
	slices.SortFunc(words, compareWords)
	index := make(map[string]uint32, len(words))
	var idx bytes.Buffer
	var data bytes.Buffer
	for i, w := range words {
		index[w] = uint32(i)
		idx.WriteString(w)
		idx.WriteByte(0)
		text := arts[w].html.String()
		var n [8]byte
		binary.BigEndian.PutUint32(n[:], uint32(data.Len()))
		binary.BigEndian.PutUint32(n[4:], uint32(len(text)))
		idx.Write(n[:])
		data.WriteString(text)
	}
	type syn struct {
		form string
		to   uint32
	}
	var syns []syn
	for w, a := range arts {
		for f := range a.forms {
			syns = append(syns, syn{f, index[w]})
		}
	}
	slices.SortFunc(syns, func(a, b syn) int {
		if c := compareWords(a.form, b.form); c != 0 {
			return c
		}
		return int(a.to) - int(b.to)
	})
	var synb bytes.Buffer
	for _, s := range syns {
		synb.WriteString(s.form)
		synb.WriteByte(0)
		var n [4]byte
		binary.BigEndian.PutUint32(n[:], s.to)
		synb.Write(n[:])
	}
	ifo := fmt.Sprintf("StarDict's dict ifo file\nversion=3.0.0\nbookname=%s\nwordcount=%d\nsynwordcount=%d\nidxfilesize=%d\nsametypesequence=h\ndescription=From Wiktionary, by way of kaikki.org. CC BY-SA.\n",
		name, len(words), len(syns), idx.Len())
	if err := os.MkdirAll(filepath.Dir(base), 0o755); err != nil {
		return err
	}
	for ext, b := range map[string][]byte{".ifo": []byte(ifo), ".idx": idx.Bytes(), ".syn": synb.Bytes()} {
		if err := os.WriteFile(base+ext, b, 0o644); err != nil {
			return err
		}
	}
	dz, err := dictzip(data.Bytes())
	if err != nil {
		return err
	}
	if err := os.WriteFile(base+".dict.dz", dz, 0o644); err != nil {
		return err
	}
	if err := dict.WriteIndex(base + ".ifo"); err != nil { // so the Kobo needn't
		return err
	}
	var all int64
	for _, ext := range []string{".ifo", ".idx", ".syn", ".dict.dz", ".hbx"} {
		if st, err := os.Stat(base + ext); err == nil {
			all += st.Size()
		}
	}
	log.Printf("%s: %d words, %d forms; %.1f MB", filepath.Base(base), len(words), len(syns), float64(all)/1e6)
	return nil
}

// dictzip packs data as dictzip does: a gzip file of chunks each deflated
// on its own, each one's size in the header. gzip itself reads it whole.
func dictzip(data []byte) ([]byte, error) {
	var body bytes.Buffer
	var sizes []uint16
	last := 0
	if len(data) == 0 {
		data = []byte{}
	}
	for p := 0; p == 0 || p < len(data); p += dzChunk {
		z, err := flate.NewWriter(&body, flate.BestCompression)
		if err != nil {
			return nil, err
		}
		if _, err := z.Write(data[p:min(p+dzChunk, len(data))]); err != nil {
			return nil, err
		}
		if p+dzChunk >= len(data) {
			err = z.Close() // the last one ends the stream
		} else {
			err = z.Flush()
		}
		if err != nil {
			return nil, err
		}
		n := body.Len() - last
		if n > 1<<16-1 {
			return nil, fmt.Errorf("a chunk of %d bytes", n)
		}
		sizes = append(sizes, uint16(n))
		last = body.Len()
	}
	if len(sizes) > 1<<16-1 {
		return nil, fmt.Errorf("%d chunks: too big", len(sizes))
	}
	ra := []byte{1, 0}
	ra = binary.LittleEndian.AppendUint16(ra, dzChunk)
	ra = binary.LittleEndian.AppendUint16(ra, uint16(len(sizes)))
	for _, s := range sizes {
		ra = binary.LittleEndian.AppendUint16(ra, s)
	}
	extra := binary.LittleEndian.AppendUint16([]byte{'R', 'A'}, uint16(len(ra)))
	extra = append(extra, ra...)
	out := make([]byte, 0, body.Len()+len(extra)+32)
	out = append(out, 0x1f, 0x8b, 8, 4, 0, 0, 0, 0, 2, 255)
	out = binary.LittleEndian.AppendUint16(out, uint16(len(extra)))
	out = append(out, extra...)
	out = append(out, body.Bytes()...)
	out = binary.LittleEndian.AppendUint32(out, crc32.ChecksumIEEE(data))
	out = binary.LittleEndian.AppendUint32(out, uint32(len(data)))
	return out, nil
}

func main() {
	log.SetFlags(0)
	name := flag.String("name", "", "the dictionary's title")
	flag.Parse()
	if flag.NArg() != 2 || *name == "" {
		fmt.Fprintln(os.Stderr, "usage: dictconv -name TITLE kaikki.jsonl out/grc-en")
		os.Exit(2)
	}
	f, err := os.Open(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	arts, err := readSource(f)
	if err != nil {
		log.Fatal(err)
	}
	if err := write(flag.Arg(1), *name, arts); err != nil {
		log.Fatal(err)
	}
}
