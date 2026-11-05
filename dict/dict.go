// Package dict reads StarDict dictionaries (.ifo, .idx, .syn, .dict or
// .dict.dz) to look a book's words up. Words are matched folded, without
// their accents and capitals (Fold), as a book's words aren't spelled as
// a dictionary's headwords are. No drawing here: the reader's box shows
// what's found.
package dict

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
)

// maxWord is the longest headword StarDict allows, its NUL with it.
const maxWord = 256

// Dict is a dictionary. Nothing but its .ifo is read till the first look
// up, so one not used costs nothing.
type Dict struct {
	Name string // its title, the .ifo's bookname
	Base string // the file's name without .ifo: "grc-en"
	base string // with its directory

	seq     string // sametypesequence: what each entry's made of
	offBits int
	words   int

	once  sync.Once
	err   error
	idx   *os.File
	data  entryReader
	index *index // index.go

	mu sync.Mutex // the data's last chunk
}

// Entry is an article found.
type Entry struct {
	Dict   string // the dictionary's title
	Word   string // its headword
	Fields []Field
}

// Field is a part of an article, in its StarDict type: m is plain text,
// h HTML, and so on.
type Field struct {
	Type byte
	Text string
}

// Find opens the dictionaries in dir and the directories in it, by name.
// What can't be opened is left out, and said in the error.
func Find(dir string) ([]*Dict, error) {
	var ifos []string
	err := filepath.WalkDir(dir, func(p string, e os.DirEntry, err error) error {
		if err != nil {
			if p == dir {
				return err
			}
			return nil
		}
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(p), ".ifo") {
			ifos = append(ifos, p)
		}
		return nil
	})
	slices.Sort(ifos)
	var ds []*Dict
	errs := []error{}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		errs = append(errs, err)
	}
	for _, p := range ifos {
		d, err := Open(p)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		ds = append(ds, d)
	}
	return ds, errors.Join(errs...)
}

// Open reads a dictionary's .ifo. The rest waits for the first look up.
func Open(ifo string) (*Dict, error) {
	f, err := os.Open(ifo)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	base := ifo[:len(ifo)-len(filepath.Ext(ifo))]
	d := &Dict{base: base, Base: filepath.Base(base), offBits: 32}
	s := bufio.NewScanner(f)
	for s.Scan() {
		k, v, ok := strings.Cut(s.Text(), "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(k) {
		case "bookname":
			d.Name = strings.TrimSpace(v)
		case "sametypesequence":
			d.seq = strings.TrimSpace(v)
		case "wordcount":
			d.words, _ = strconv.Atoi(strings.TrimSpace(v))
		case "idxoffsetbits":
			d.offBits, _ = strconv.Atoi(strings.TrimSpace(v))
		}
	}
	if err := s.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", ifo, err)
	}
	if d.offBits != 32 && d.offBits != 64 {
		return nil, fmt.Errorf("%s: idxoffsetbits %d", ifo, d.offBits)
	}
	if d.Name == "" {
		d.Name = d.Base
	}
	return d, nil
}

// Path says where it is, which is what tells it from another.
func (d *Dict) Path() string { return d.base }

// load reads the index, the first time.
func (d *Dict) load() error {
	d.once.Do(func() { d.err = d.read() })
	return d.err
}

func (d *Dict) read() error {
	ix, err := openIndex(d)
	if err != nil {
		return err
	}
	d.index = ix
	if d.idx, err = os.Open(d.base + ".idx"); err != nil {
		return err
	}
	d.data, err = openData(d.base)
	return err
}

// headword is the word at this place in the .idx.
func (d *Dict) headword(at uint32) (string, error) {
	var b [maxWord]byte
	n, err := d.idx.ReadAt(b[:], int64(at))
	if err != nil && err != io.EOF {
		return "", err
	}
	if k := bytes.IndexByte(b[:n], 0); k >= 0 {
		n = k
	}
	return string(b[:n]), nil
}

// Lookup finds word's articles, at most limit: its headword as written
// first, then one that differs in its capitals, then in its accents, and
// then what it's a form of.
func (d *Dict) Lookup(word string, limit int) ([]Entry, error) {
	if err := d.load(); err != nil {
		return nil, err
	}
	type hit struct {
		e    entry
		rank int
		word string
	}
	var hits []hit
	seen := map[uint32]bool{}
	for c, cand := range Candidates(word) {
		folded := Fold(cand)
		found, err := d.index.find(hashOf(folded))
		if err != nil {
			return nil, err
		}
		for _, i := range found {
			if seen[i] {
				continue
			}
			seen[i] = true
			e, err := d.index.entry(i)
			if err != nil {
				return nil, err
			}
			w, err := d.headword(e.word)
			if err != nil {
				return nil, err
			}
			hits = append(hits, hit{e, 10*c + rankOf(cand, w, folded), w})
		}
	}
	slices.SortStableFunc(hits, func(a, b hit) int { return a.rank - b.rank })
	hits = hits[:min(limit, len(hits))]
	out := make([]Entry, 0, len(hits))
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, h := range hits {
		data, err := d.data.ReadEntry(int64(h.e.off), int(h.e.size))
		if err != nil {
			return out, fmt.Errorf("%s: %s: %w", d.Base, h.word, err)
		}
		out = append(out, Entry{Dict: d.Name, Word: h.word, Fields: parseEntry(data, d.seq)})
	}
	return out, nil
}

// rankOf says how near the headword is to the word as written.
func rankOf(cand, head, folded string) int {
	switch {
	case head == cand:
		return 0
	case strings.EqualFold(head, cand):
		return 1
	case Fold(head) == folded:
		return 2
	}
	return 3 // it's a form of it
}

// parseEntry splits an article by its types: the .ifo's sequence if it
// has one (and the types aren't written then), or each field's own.
// Lower-case types are text, NUL-ended; upper-case are data with their
// size first. The last field of a sequence goes to the end.
func parseEntry(data []byte, seq string) []Field {
	var out []Field
	if seq != "" {
		for i := 0; i < len(seq); i++ {
			var f Field
			f, data = readField(seq[i], data, i == len(seq)-1)
			out = append(out, f)
		}
		return out
	}
	for len(data) > 0 {
		t := data[0]
		var f Field
		f, data = readField(t, data[1:], false)
		out = append(out, f)
	}
	return out
}

func readField(t byte, data []byte, last bool) (Field, []byte) {
	if t >= 'A' && t <= 'Z' { // data: pictures, sounds
		if last {
			return Field{t, ""}, nil
		}
		if len(data) < 4 {
			return Field{t, ""}, nil
		}
		n := int(binary.BigEndian.Uint32(data))
		return Field{t, ""}, data[min(len(data), 4+max(0, n)):]
	}
	if last {
		return Field{t, string(data)}, nil
	}
	if k := bytes.IndexByte(data, 0); k >= 0 {
		return Field{t, string(data[:k])}, data[k+1:]
	}
	return Field{t, string(data)}, nil
}
