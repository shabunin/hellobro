package main

// Audio for the Sound tab: a test tone, MP3 and WAV files from the Kobo's
// storage, converted to what the Bluetooth PCM wants and written into it.

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hajimehoshi/go-mp3"

	"hellobro/kobo"
)

const toneTrack = "test tone (a little arpeggio)"

var musicDir = kobo.EnvOr("HELLOBRO_MUSIC_DIR", "/mnt/onboard")

// findTracks lists MP3 and WAV files a few folders deep, skipping hidden ones.
func findTracks() []string {
	var tracks []string
	filepath.WalkDir(musicDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(musicDir, p)
		if d.IsDir() {
			if p != musicDir && (strings.HasPrefix(d.Name(), ".") || strings.Count(rel, "/") >= 3) {
				return filepath.SkipDir
			}
			return nil
		}
		switch strings.ToLower(filepath.Ext(p)) {
		case ".mp3", ".wav":
			tracks = append(tracks, rel)
		}
		if len(tracks) >= 60 {
			return fs.SkipAll
		}
		return nil
	})
	sort.Strings(tracks)
	return tracks
}

// stereoSource yields interleaved 16-bit stereo frames at its own rate.
type stereoSource interface {
	rate() int
	read(buf []int16) (int, error) // fills whole frames (2 samples each)
	length() time.Duration         // 0 if unknown
	close()
}

func openTrack(name string, pcmRate int) (stereoSource, error) {
	if name == toneTrack {
		return &tone{sr: pcmRate}, nil
	}
	f, err := os.Open(filepath.Join(musicDir, name))
	if err != nil {
		return nil, err
	}
	var src stereoSource
	if strings.EqualFold(filepath.Ext(name), ".wav") {
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

// --- test tone ---

type tone struct {
	sr int
	n  int // frames so far
}

var toneNotes = []float64{261.63, 329.63, 392.00, 523.25, 392.00, 329.63, 261.63} // C E G C' G E C

const toneNote = 0.35 // seconds

func (t *tone) rate() int { return t.sr }
func (t *tone) length() time.Duration {
	return time.Duration(float64(len(toneNotes)) * toneNote * float64(time.Second))
}
func (t *tone) close() {}
func (t *tone) read(buf []int16) (int, error) {
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

func openMP3(f *os.File) (stereoSource, error) {
	d, err := mp3.NewDecoder(f)
	if err != nil {
		return nil, fmt.Errorf("mp3: %w", err)
	}
	return &mp3Source{f: f, d: d}, nil
}

func (m *mp3Source) rate() int { return m.d.SampleRate() }
func (m *mp3Source) length() time.Duration {
	if m.d.Length() <= 0 {
		return 0
	}
	return time.Duration(m.d.Length() / 4 * int64(time.Second) / int64(m.d.SampleRate()))
}
func (m *mp3Source) close() { m.f.Close() }
func (m *mp3Source) read(buf []int16) (int, error) {
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
	raw      []byte
}

func openWAV(f *os.File) (stereoSource, error) {
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
			return w, nil
		default:
			r.Discard(int(size + size%2))
		}
	}
}

func (w *wavSource) rate() int { return w.sr }
func (w *wavSource) length() time.Duration {
	return time.Duration(w.dataSize / int64(2*w.ch) * int64(time.Second) / int64(w.sr))
}
func (w *wavSource) close() { w.f.Close() }
func (w *wavSource) read(buf []int16) (int, error) {
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

// --- conversion & playback ---

// writePCM converts src to rate/channels (linear resampling), applies gain,
// and writes 16-bit little-endian samples to w until src ends or stop closes.
// tick gets the playback position now and then.
func writePCM(w io.Writer, src stereoSource, rate, channels int, gain func() float64, stop <-chan struct{}, tick func(time.Duration)) error {
	in := make([]int16, 4096)
	var cur, next [2]float64 // the input frames around the output position
	var have, pos float64    // have: input frames in buf consumed; pos: output position in input frames
	step := float64(src.rate()) / float64(rate)
	n, idx := 0, 0
	readFrame := func() bool {
		if idx >= n {
			var err error
			if n, err = src.read(in); err != nil || n == 0 {
				return false
			}
			idx = 0
		}
		cur = next
		next = [2]float64{float64(in[idx]), float64(in[idx+1])}
		idx += 2
		have++
		return true
	}
	if !readFrame() || !readFrame() {
		return nil
	}
	have = 1 // cur is input frame 0, next frame 1
	out := make([]byte, 0, 8192)
	written := 0
	lastTick := time.Now()
	for {
		select {
		case <-stop:
			return nil
		default:
		}
		out = out[:0]
		g := gain()
		for len(out) < cap(out)-8 {
			for pos >= have {
				if !readFrame() {
					_, err := w.Write(out)
					return err
				}
			}
			frac := pos - (have - 1)
			l := cur[0] + (next[0]-cur[0])*frac
			r := cur[1] + (next[1]-cur[1])*frac
			if channels == 1 {
				out = binary.LittleEndian.AppendUint16(out, uint16(clip16((l+r)/2*g)))
			} else {
				out = binary.LittleEndian.AppendUint16(out, uint16(clip16(l*g)))
				out = binary.LittleEndian.AppendUint16(out, uint16(clip16(r*g)))
			}
			pos += step
			written++
		}
		if _, err := w.Write(out); err != nil {
			return err
		}
		if time.Since(lastTick) > 500*time.Millisecond {
			lastTick = time.Now()
			tick(time.Duration(written) * time.Second / time.Duration(rate))
		}
	}
}

func clip16(v float64) int16 {
	return int16(math.Max(-32768, math.Min(32767, v)))
}
