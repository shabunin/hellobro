package audio

// Decoding: MP3 and WAV files, and a test tone, as 16-bit stereo.

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hajimehoshi/go-mp3"
)

// Source yields interleaved 16-bit stereo frames at its own rate.
type Source interface {
	Rate() int
	Read(buf []int16) (int, error) // fills whole frames (2 samples each)
	Length() time.Duration         // 0 if unknown
	Close()
}

// Seeker is a Source that can jump.
type Seeker interface {
	SeekTo(d time.Duration) error
}

// Playable says whether Open can play the file, by its name.
func Playable(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".mp3", ".wav":
		return true
	}
	return false
}

// Open opens an MP3 or WAV file.
func Open(path string) (Source, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	var src Source
	if strings.EqualFold(filepath.Ext(path), ".wav") {
		src, err = openWAV(f)
	} else {
		src, err = openMP3(f)
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return src, nil
}

// Tone is a little arpeggio, a test sound at rate.
func Tone(rate int) Source { return &tone{sr: rate} }

// --- test tone ---

type tone struct {
	sr int
	n  int // frames so far
}

var toneNotes = []float64{261.63, 329.63, 392.00, 523.25, 392.00, 329.63, 261.63} // C E G C' G E C

const toneNote = 0.35 // seconds

func (t *tone) Rate() int { return t.sr }
func (t *tone) Length() time.Duration {
	return time.Duration(float64(len(toneNotes)) * toneNote * float64(time.Second))
}
func (t *tone) Close() {}
func (t *tone) Read(buf []int16) (int, error) {
	noteLen := int(toneNote * float64(t.sr))
	total := noteLen * len(toneNotes)
	i := 0
	for ; i+1 < len(buf) && t.n < total; i += 2 {
		k, pos := t.n/noteLen, t.n%noteLen
		env := math.Min(1, float64(pos)/(0.01*float64(t.sr))) * math.Min(1, float64(noteLen-pos)/(0.05*float64(t.sr)))
		v := int16(0.6 * 32767 * env * math.Sin(2*math.Pi*toneNotes[k]*float64(t.n)/float64(t.sr)))
		buf[i], buf[i+1] = v, v
		t.n++
	}
	if i == 0 {
		return 0, io.EOF
	}
	return i, nil
}

// --- MP3 ---

type mp3Source struct {
	f   *os.File
	d   *mp3.Decoder
	raw []byte
}

func openMP3(f *os.File) (Source, error) {
	d, err := mp3.NewDecoder(f)
	if err != nil {
		return nil, fmt.Errorf("mp3: %w", err)
	}
	return &mp3Source{f: f, d: d}, nil
}

func (m *mp3Source) Rate() int { return m.d.SampleRate() }
func (m *mp3Source) Length() time.Duration {
	if m.d.Length() <= 0 {
		return 0
	}
	return time.Duration(m.d.Length() / 4 * int64(time.Second) / int64(m.d.SampleRate()))
}
func (m *mp3Source) Close() { m.f.Close() }
func (m *mp3Source) SeekTo(d time.Duration) error {
	frame := int64(d.Seconds() * float64(m.d.SampleRate()))
	_, err := m.d.Seek(max(0, min(frame*4, m.d.Length())), io.SeekStart)
	return err
}
func (m *mp3Source) Read(buf []int16) (int, error) {
	n := len(buf) &^ 1
	if cap(m.raw) < n*2 {
		m.raw = make([]byte, n*2)
	}
	got, err := io.ReadFull(m.d, m.raw[:n*2])
	got &^= 3
	for i := 0; i < got/2; i++ {
		buf[i] = int16(binary.LittleEndian.Uint16(m.raw[2*i:]))
	}
	if got > 0 {
		return got / 2, nil
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		err = io.EOF
	}
	return 0, err
}

// --- WAV (16-bit PCM only) ---

type wavSource struct {
	f        *os.File
	r        *bufio.Reader
	sr, ch   int
	left     int64 // data bytes left
	dataSize int64
	dataAt   int64 // where the data starts in the file
	raw      []byte
}

func openWAV(f *os.File) (Source, error) {
	r := bufio.NewReaderSize(f, 64<<10)
	var hdr [12]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil || string(hdr[:4]) != "RIFF" || string(hdr[8:]) != "WAVE" {
		return nil, errors.New("wav: not a RIFF/WAVE file")
	}
	w := &wavSource{f: f, r: r}
	for {
		var ch [8]byte
		if _, err := io.ReadFull(r, ch[:]); err != nil {
			return nil, errors.New("wav: no data chunk")
		}
		size := int64(binary.LittleEndian.Uint32(ch[4:]))
		switch string(ch[:4]) {
		case "fmt ":
			b := make([]byte, size)
			if _, err := io.ReadFull(r, b); err != nil || size < 16 {
				return nil, errors.New("wav: bad fmt chunk")
			}
			format, bits := binary.LittleEndian.Uint16(b), binary.LittleEndian.Uint16(b[14:])
			if format != 1 || bits != 16 {
				return nil, fmt.Errorf("wav: only 16-bit PCM is supported (format %d, %d bits)", format, bits)
			}
			w.ch, w.sr = int(binary.LittleEndian.Uint16(b[2:])), int(binary.LittleEndian.Uint32(b[4:]))
			if size%2 == 1 {
				r.Discard(1)
			}
		case "data":
			if w.sr == 0 || w.ch < 1 || w.ch > 2 {
				return nil, errors.New("wav: unsupported channel layout")
			}
			w.left, w.dataSize = size, size
			at, err := f.Seek(0, io.SeekCurrent)
			w.dataAt = at - int64(r.Buffered())
			return w, err
		default:
			r.Discard(int(size + size%2))
		}
	}
}

func (w *wavSource) Rate() int { return w.sr }
func (w *wavSource) Length() time.Duration {
	return time.Duration(w.dataSize / int64(2*w.ch) * int64(time.Second) / int64(w.sr))
}
func (w *wavSource) Close() { w.f.Close() }
func (w *wavSource) SeekTo(d time.Duration) error {
	frame := int64(2 * w.ch)
	off := max(0, min(int64(d.Seconds()*float64(w.sr))*frame, w.dataSize/frame*frame))
	if _, err := w.f.Seek(w.dataAt+off, io.SeekStart); err != nil {
		return err
	}
	w.r.Reset(w.f)
	w.left = w.dataSize - off
	return nil
}
func (w *wavSource) Read(buf []int16) (int, error) {
	frames := min(int64(len(buf)/2), w.left/int64(2*w.ch))
	if frames == 0 {
		return 0, io.EOF
	}
	need := int(frames) * 2 * w.ch
	if cap(w.raw) < need {
		w.raw = make([]byte, need)
	}
	got, err := io.ReadFull(w.r, w.raw[:need])
	got -= got % (2 * w.ch)
	w.left -= int64(got)
	n := 0
	for i := 0; i < got; i += 2 * w.ch {
		l := int16(binary.LittleEndian.Uint16(w.raw[i:]))
		r := l
		if w.ch == 2 {
			r = int16(binary.LittleEndian.Uint16(w.raw[i+2:]))
		}
		buf[n], buf[n+1] = l, r
		n += 2
	}
	if n == 0 && err != nil {
		return 0, io.EOF
	}
	return n, nil
}
