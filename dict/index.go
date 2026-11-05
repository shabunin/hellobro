package dict

// A dictionary's words as they're looked up: folded, and all its forms.
// Folding millions of forms takes a Kobo a good while, and holding them a
// lot of memory, so it's done once and kept beside the dictionary, in a
// .hbx (dictconv writes it; for a dictionary without one, the first look
// up does), and that's searched where it is, on disk.
//
// A .hbx: "hbx1", the sizes of the .idx and .syn it's of (another size
// and it's made again), how many entries and keys; then the entries in
// the .idx's order (where the word is, where its article is, how long),
// then the keys, sorted by hash (a hash, an entry). All big-endian.

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"slices"
)

const (
	hbxMagic  = "hbx1"
	hbxHead   = 4 + 8 + 8 + 4 + 4
	entrySize = 12
	keySize   = 12
)

// entry is a headword: where it's written in the .idx, and where its
// article is.
type entry struct{ word, off, size uint32 }

// key is a folded word (a hash of it, so millions stay small) and the
// entry it leads to.
type key struct {
	hash  uint64
	entry uint32
}

// index is a .hbx open, or in memory if it couldn't be written.
type index struct {
	r    io.ReaderAt
	ents uint32
	keys uint32
}

// hbxHeader is what a .hbx starts with.
type hbxHeader struct {
	idx, syn   uint64
	ents, keys uint32
}

// sizes are the .idx's and the .syn's (0 if it has none).
func sizes(base string) (idx, syn uint64, err error) {
	st, err := os.Stat(base + ".idx")
	if err != nil {
		return 0, 0, err
	}
	idx = uint64(st.Size())
	if st, err := os.Stat(base + ".syn"); err == nil {
		syn = uint64(st.Size())
	}
	return idx, syn, nil
}

// openIndex opens the dictionary's .hbx, making it first if need be.
func openIndex(d *Dict) (*index, error) {
	idx, syn, err := sizes(d.base)
	if err != nil {
		return nil, err
	}
	if ix := readIndex(d.base+".hbx", idx, syn); ix != nil {
		return ix, nil
	}
	data, err := buildIndex(d.base, d.offBits, d.words)
	if err != nil {
		return nil, err
	}
	if err := writeFile(d.base+".hbx", data); err != nil { // it works anyway, in memory, for now
		log.Printf("dict: %s: %v", d.Base, err)
	}
	return parseIndex(bytes.NewReader(data), int64(len(data)), idx, syn)
}

// readIndex is the .hbx if it's there and of these sizes.
func readIndex(name string, idx, syn uint64) *index {
	f, err := os.Open(name)
	if err != nil {
		return nil
	}
	st, err := f.Stat()
	if err == nil {
		var ix *index
		if ix, err = parseIndex(f, st.Size(), idx, syn); err == nil {
			return ix
		}
		log.Printf("dict: %s: %v: made again", name, err)
	}
	f.Close()
	return nil
}

func parseIndex(r io.ReaderAt, size int64, idx, syn uint64) (*index, error) {
	var h [hbxHead]byte
	if _, err := r.ReadAt(h[:], 0); err != nil {
		return nil, err
	}
	if string(h[:4]) != hbxMagic {
		return nil, errors.New("not a .hbx")
	}
	be := binary.BigEndian
	if be.Uint64(h[4:]) != idx || be.Uint64(h[12:]) != syn {
		return nil, errors.New("of another dictionary")
	}
	ix := &index{r: r, ents: be.Uint32(h[20:]), keys: be.Uint32(h[24:])}
	if want := hbxHead + int64(ix.ents)*entrySize + int64(ix.keys)*keySize; size != want {
		return nil, fmt.Errorf("%d bytes, not %d", size, want)
	}
	return ix, nil
}

// entry is entry i.
func (ix *index) entry(i uint32) (entry, error) {
	if i >= ix.ents {
		return entry{}, fmt.Errorf("no entry %d", i)
	}
	var b [entrySize]byte
	if _, err := ix.r.ReadAt(b[:], hbxHead+int64(i)*entrySize); err != nil {
		return entry{}, err
	}
	be := binary.BigEndian
	return entry{be.Uint32(b[:]), be.Uint32(b[4:]), be.Uint32(b[8:])}, nil
}

// keyAt is key i.
func (ix *index) keyAt(i uint32) (key, error) {
	var b [keySize]byte
	if _, err := ix.r.ReadAt(b[:], hbxHead+int64(ix.ents)*entrySize+int64(i)*keySize); err != nil {
		return key{}, err
	}
	be := binary.BigEndian
	return key{be.Uint64(b[:]), be.Uint32(b[8:])}, nil
}

// find is the entries of a word's hash.
func (ix *index) find(h uint64) ([]uint32, error) {
	lo, hi := uint32(0), ix.keys
	for lo < hi {
		m := lo + (hi-lo)/2
		k, err := ix.keyAt(m)
		if err != nil {
			return nil, err
		}
		if k.hash < h {
			lo = m + 1
		} else {
			hi = m
		}
	}
	var out []uint32
	for ; lo < ix.keys; lo++ {
		k, err := ix.keyAt(lo)
		if err != nil {
			return nil, err
		}
		if k.hash != h {
			break
		}
		out = append(out, k.entry)
	}
	return out, nil
}

// buildIndex reads the .idx and .syn, and makes a .hbx's contents.
func buildIndex(base string, offBits, words int) ([]byte, error) {
	idxSize, synSize, err := sizes(base)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(base + ".idx")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	name := base[max(0, len(base)-40):]
	ents := make([]entry, 0, words)
	keys := make([]key, 0, words)
	r := bufio.NewReaderSize(f, 1<<20)
	var at uint32
	n := offBits / 8
	for {
		word, err := r.ReadBytes(0)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		var b [12]byte
		if _, err := io.ReadFull(r, b[:n+4]); err != nil {
			return nil, fmt.Errorf("%s.idx: %w", name, err)
		}
		off := uint64(binary.BigEndian.Uint32(b[:4]))
		if n == 8 {
			off = binary.BigEndian.Uint64(b[:8])
		}
		if off > 1<<32-1 {
			return nil, fmt.Errorf("%s: too big", name)
		}
		keys = append(keys, key{hashOf(Fold(string(word[:len(word)-1]))), uint32(len(ents))})
		ents = append(ents, entry{at, uint32(off), binary.BigEndian.Uint32(b[n:])})
		at += uint32(len(word) + n + 4)
	}
	if keys, err = readSyn(base, keys, len(ents)); err != nil {
		return nil, err
	}
	slices.SortFunc(keys, func(a, b key) int {
		switch {
		case a.hash < b.hash:
			return -1
		case a.hash > b.hash:
			return 1
		}
		return int(a.entry) - int(b.entry)
	})
	keys = slices.Compact(keys)

	out := make([]byte, hbxHead, hbxHead+len(ents)*entrySize+len(keys)*keySize)
	copy(out, hbxMagic)
	be := binary.BigEndian
	be.PutUint64(out[4:], idxSize)
	be.PutUint64(out[12:], synSize)
	be.PutUint32(out[20:], uint32(len(ents)))
	be.PutUint32(out[24:], uint32(len(keys)))
	for _, e := range ents {
		out = be.AppendUint32(out, e.word)
		out = be.AppendUint32(out, e.off)
		out = be.AppendUint32(out, e.size)
	}
	for _, k := range keys {
		out = be.AppendUint64(out, k.hash)
		out = be.AppendUint32(out, k.entry)
	}
	return out, nil
}

// readSyn adds the .syn's keys, if there is one: the other forms of a
// word, and the entry each leads to.
func readSyn(base string, keys []key, ents int) ([]key, error) {
	f, err := os.Open(base + ".syn")
	if errors.Is(err, os.ErrNotExist) {
		return keys, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := bufio.NewReaderSize(f, 1<<20)
	for {
		word, err := r.ReadBytes(0)
		if err == io.EOF {
			return keys, nil
		}
		if err != nil {
			return nil, err
		}
		var b [4]byte
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return nil, fmt.Errorf("%s.syn: %w", base, err)
		}
		if i := binary.BigEndian.Uint32(b[:]); int(i) < ents {
			keys = append(keys, key{hashOf(Fold(string(word[:len(word)-1]))), i})
		}
	}
}

// writeFile writes a new file and moves it over the old one, so nothing
// half-written is ever opened.
func writeFile(name string, data []byte) error {
	tmp := name + ".new"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, name)
}

// hashOf is FNV-1a.
func hashOf(s string) uint64 {
	h := uint64(14695981039346656037)
	for i := 0; i < len(s); i++ {
		h = (h ^ uint64(s[i])) * 1099511628211
	}
	return h
}

// WriteIndex makes the .hbx of the dictionary at ifo, ahead of time, so
// the Kobo needn't: a PC does it in a moment.
func WriteIndex(ifo string) error {
	d, err := Open(ifo)
	if err != nil {
		return err
	}
	data, err := buildIndex(d.base, d.offBits, d.words)
	if err != nil {
		return err
	}
	return writeFile(d.base+".hbx", data)
}
