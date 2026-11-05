package dict

// The articles: a .dict, or a .dict.dz, which is a gzip file of chunks
// each of which can be read on its own (dictzip): no need to unpack what
// comes before.

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
)

// maxEntry is the longest article that's read: no one reads more.
const maxEntry = 4 << 20

type entryReader interface {
	ReadEntry(off int64, size int) ([]byte, error)
}

func openData(base string) (entryReader, error) {
	if f, err := os.Open(base + ".dict"); err == nil {
		return plainData{f}, nil
	}
	f, err := os.Open(base + ".dict.dz")
	if err != nil {
		return nil, err
	}
	return readDictzip(f)
}

type plainData struct{ f *os.File }

func (p plainData) ReadEntry(off int64, size int) ([]byte, error) {
	b := make([]byte, min(size, maxEntry))
	_, err := p.f.ReadAt(b, off)
	if err == io.EOF {
		err = nil
	}
	return b, err
}

// dictzip is a dictzip file: each chunk's place, and the last chunk read,
// as an article's words are often looked up near each other.
type dictzip struct {
	f      *os.File
	chunk  int
	starts []int64 // of each chunk, and the end of the last
	cached int     // which is in buf; -1 none
	buf    []byte
}

// readDictzip reads the gzip header's "RA" field. A plain gzip file
// hasn't one: then it can only be read whole, and is, if it's not big.
func readDictzip(f *os.File) (entryReader, error) {
	var h [12]byte
	if _, err := io.ReadFull(f, h[:10]); err != nil {
		return nil, err
	}
	if h[0] != 0x1f || h[1] != 0x8b || h[2] != 8 {
		return nil, fmt.Errorf("%s: not gzip", f.Name())
	}
	flags := h[3]
	at := int64(10)
	d := &dictzip{f: f, cached: -1}
	var chunks int
	var sizes []byte
	if flags&4 != 0 {
		if _, err := io.ReadFull(f, h[:2]); err != nil {
			return nil, err
		}
		xlen := int(binary.LittleEndian.Uint16(h[:2]))
		extra := make([]byte, xlen)
		if _, err := io.ReadFull(f, extra); err != nil {
			return nil, err
		}
		at += 2 + int64(xlen)
		for len(extra) >= 4 {
			n := int(binary.LittleEndian.Uint16(extra[2:4]))
			if 4+n > len(extra) {
				break
			}
			if extra[0] == 'R' && extra[1] == 'A' && n >= 6 {
				sub := extra[4 : 4+n]
				d.chunk = int(binary.LittleEndian.Uint16(sub[2:4]))
				chunks = int(binary.LittleEndian.Uint16(sub[4:6]))
				sizes = sub[6:]
			}
			extra = extra[4+n:]
		}
	}
	if d.chunk == 0 || len(sizes) < 2*chunks {
		return readWholeGzip(f)
	}
	// the name and comment, ended by NULs; the header's CRC
	for _, bit := range []byte{8, 16} {
		if flags&bit == 0 {
			continue
		}
		for {
			var b [1]byte
			if _, err := f.ReadAt(b[:], at); err != nil {
				return nil, err
			}
			at++
			if b[0] == 0 {
				break
			}
		}
	}
	if flags&2 != 0 {
		at += 2
	}
	d.starts = make([]int64, chunks+1)
	d.starts[0] = at
	for i := range chunks {
		d.starts[i+1] = d.starts[i] + int64(binary.LittleEndian.Uint16(sizes[2*i:]))
	}
	return d, nil
}

const maxWholeGzip = 64 << 20

type wholeData []byte

func readWholeGzip(f *os.File) (entryReader, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	z, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	b, err := io.ReadAll(io.LimitReader(z, maxWholeGzip+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxWholeGzip {
		return nil, fmt.Errorf("%s: too big, and not dictzip", f.Name())
	}
	return wholeData(b), nil
}

func (w wholeData) ReadEntry(off int64, size int) ([]byte, error) {
	if off < 0 || off > int64(len(w)) {
		return nil, errors.New("article past the end")
	}
	return w[off:min(off+int64(min(size, maxEntry)), int64(len(w)))], nil
}

// ReadEntry reads the article from the chunks it's in.
func (d *dictzip) ReadEntry(off int64, size int) ([]byte, error) {
	size = min(size, maxEntry)
	out := make([]byte, 0, size)
	for size > 0 {
		i := int(off / int64(d.chunk))
		if i >= len(d.starts)-1 {
			return out, errors.New("article past the end")
		}
		b, err := d.read(i)
		if err != nil {
			return out, err
		}
		in := int(off % int64(d.chunk))
		if in >= len(b) {
			return out, errors.New("article past the end")
		}
		n := min(size, len(b)-in)
		out = append(out, b[in:in+n]...)
		off += int64(n)
		size -= n
	}
	return out, nil
}

func (d *dictzip) read(i int) ([]byte, error) {
	if d.cached == i {
		return d.buf, nil
	}
	z := make([]byte, d.starts[i+1]-d.starts[i])
	if _, err := d.f.ReadAt(z, d.starts[i]); err != nil {
		return nil, err
	}
	// Chunks end with a flush, not the stream's end: it reads short then.
	r := flate.NewReader(bytes.NewReader(z))
	defer r.Close()
	b, err := io.ReadAll(io.LimitReader(r, int64(d.chunk)))
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, err
	}
	d.buf, d.cached = b, i
	return b, nil
}
