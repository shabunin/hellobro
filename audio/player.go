package audio

// The player: one Source into one bluealsa PCM, converted to the PCM's
// rate and channels (linear resampling), with software volume. Pausing
// and stopping go through the PCM's control socket too, so they're heard
// at once rather than after what bluealsa has buffered.

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
)

// Player plays one source; Play starts it.
type Player struct {
	Length time.Duration // 0 if unknown

	gain   atomic.Int32 // percent
	paused atomic.Bool
	pos    atomic.Int64 // what's been sent to the PCM, as a time.Duration
	seek   chan time.Duration
	stop   chan struct{}
	once   sync.Once
}

// Play starts playing src into pcm, at gain percent. Then tick gets the
// position about twice a second, and done is called once at the end: how
// is "finished", "stopped", or what went wrong. Both are called from the
// player's goroutine.
func Play(bus *dbus.Conn, pcm PCM, src Source, gain int, tick func(time.Duration), done func(how string)) *Player {
	p := &Player{Length: src.Length(), seek: make(chan time.Duration, 4), stop: make(chan struct{})}
	p.gain.Store(int32(gain))
	go p.run(bus, pcm, src, tick, done)
	return p
}

// Stop ends playback; done is told "stopped".
func (p *Player) Stop() { p.once.Do(func() { close(p.stop) }) }

func (p *Player) SetPaused(on bool)   { p.paused.Store(on) }
func (p *Player) Paused() bool        { return p.paused.Load() }
func (p *Player) SetGain(percent int) { p.gain.Store(int32(percent)) }
func (p *Player) Pos() time.Duration  { return time.Duration(p.pos.Load()) }

// SeekTo jumps to d into the track, if the source can.
func (p *Player) SeekTo(d time.Duration) {
	d = max(0, d)
	if p.Length > 0 {
		d = min(d, p.Length-time.Second)
	}
	select {
	case p.seek <- d:
	default:
	}
}

func (p *Player) run(bus *dbus.Conn, pcm PCM, src Source, tick func(time.Duration), done func(how string)) {
	defer src.Close()
	var out0 output
	var err error
	if pcm.service == fakeService {
		out0, err = openFake(pcm)
	} else {
		out0, err = openBluealsa(bus, pcm)
	}
	if err != nil {
		done(err.Error())
		return
	}
	defer out0.Close()
	rate, channels := pcm.Rate, pcm.Channels
	if rate == 0 {
		rate = 44100
	}
	if channels == 0 {
		channels = 2
	}
	log.Printf("audio: %d Hz source to %s at %d Hz, %d ch", src.Rate(), pcm.Path, rate, channels)

	r := &resampler{src: src, in: make([]int16, 4096), step: float64(src.Rate()) / float64(rate)}
	var base time.Duration // where the resampler started in the track
	out := make([]byte, 0, 8192)
	var tickAt time.Time
	paused := false
	for {
		select {
		case <-p.stop:
			out0.command("Drop", time.Second)
			done("stopped")
			return
		case to := <-p.seek:
			if s, ok := src.(Seeker); ok {
				if err := s.SeekTo(to); err != nil {
					log.Printf("audio: seek to %v: %v", to, err)
					continue
				}
				out0.command("Drop", time.Second) // what's buffered is from before
				*r = resampler{src: src, in: r.in, step: r.step}
				base = to
				p.pos.Store(int64(to))
				tick(to)
			}
			continue
		default:
		}
		if p.paused.Load() {
			if !paused {
				paused = true
				out0.command("Pause", time.Second)
			}
			select {
			case <-p.stop:
			case to := <-p.seek:
				p.seek <- to // handled above, on the next turn
			case <-time.After(100 * time.Millisecond):
			}
			continue
		}
		if paused {
			paused = false
			out0.command("Resume", time.Second)
		}

		var more bool
		out, more = r.fill(out[:0], channels, float64(p.gain.Load())/100)
		if len(out) > 0 {
			if _, err := out0.Write(out); err != nil {
				done(err.Error())
				return
			}
		}
		pos := base + time.Duration(r.pos/float64(src.Rate())*float64(time.Second))
		p.pos.Store(int64(pos))
		if !more {
			out0.command("Drain", 5*time.Second) // replies once it's all been played
			done("finished")
			return
		}
		if time.Since(tickAt) > 500*time.Millisecond {
			tickAt = time.Now()
			tick(pos)
		}
	}
}

// output is where samples go: a bluealsa PCM, or the fake one.
type output interface {
	io.WriteCloser
	// command sends a control command (Drain, Drop, Pause, Resume) and
	// waits up to wait for the reply.
	command(cmd string, wait time.Duration)
}

type bluealsaOut struct{ data, ctrl *os.File }

func openBluealsa(bus *dbus.Conn, pcm PCM) (output, error) {
	data, ctrl, err := openPCM(bus, pcm)
	if err != nil {
		return nil, err
	}
	return &bluealsaOut{data, ctrl}, nil
}

// writeWait is how long bluealsa may leave a write waiting: it takes
// sound as fast as it plays, so longer means the device is gone without
// bluealsa noticing (its chip lost power, say), and it never will.
const writeWait = 6 * time.Second

func (o *bluealsaOut) Write(b []byte) (int, error) {
	o.data.SetWriteDeadline(time.Now().Add(writeWait))
	n, err := o.data.Write(b)
	if errors.Is(err, os.ErrDeadlineExceeded) {
		err = fmt.Errorf("the Bluetooth device took no sound for %v: gone?", writeWait)
	}
	return n, err
}

func (o *bluealsaOut) Close() error {
	o.data.Close()
	return o.ctrl.Close()
}

func (o *bluealsaOut) command(cmd string, wait time.Duration) {
	if _, err := o.ctrl.Write([]byte(cmd)); err != nil {
		log.Printf("audio: %s: %v", cmd, err)
		return
	}
	buf := make([]byte, 32)
	o.ctrl.SetReadDeadline(time.Now().Add(wait))
	n, err := o.ctrl.Read(buf)
	if reply := strings.TrimSpace(string(buf[:n])); err != nil || reply != "OK" {
		log.Printf("audio: %s: %q %v", cmd, reply, err)
	}
}

// --- a stand-in for testing on the PC ---

// fakeService marks the fake PCM, which HELLOBRO_FAKE_PCM=FILE asks for:
// Probe finds a connected "speaker" whose samples go to FILE, taking as
// long as playing them would.
const fakeService = "fake"

func fakeReport() (Report, bool) {
	if os.Getenv("HELLOBRO_FAKE_PCM") == "" {
		return Report{}, false
	}
	dev := Device{Path: "/fake/speaker", Name: "Fake Speaker", Addr: "00:00:00:00:00:00", Paired: true, Connected: true, IsAudio: true}
	pcm := PCM{service: fakeService, Path: "/fake/speaker/a2dp", Device: dev.Path, Mode: "sink", Codec: "SBC", Format: 0x8210, Channels: 2, Rate: 48000}
	return Report{Lines: []string{"fake audio (HELLOBRO_FAKE_PCM)"}, Devices: []Device{dev}, PCMs: []PCM{pcm}}, true
}

type fakeOut struct {
	f     *os.File
	start time.Time
	sent  time.Duration // audio written since start
	bps   float64       // bytes per second
}

func openFake(pcm PCM) (output, error) {
	f, err := os.OpenFile(os.Getenv("HELLOBRO_FAKE_PCM"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	return &fakeOut{f: f, start: time.Now(), bps: float64(pcm.Rate * pcm.Channels * 2)}, nil
}

func (o *fakeOut) Write(b []byte) (int, error) {
	o.sent += time.Duration(float64(len(b)) / o.bps * float64(time.Second))
	if ahead := o.sent - time.Since(o.start) - 200*time.Millisecond; ahead > 0 { // a buffer's worth ahead, like bluealsa
		time.Sleep(ahead)
	}
	return o.f.Write(b)
}

func (o *fakeOut) Close() error { return o.f.Close() }

func (o *fakeOut) command(cmd string, wait time.Duration) {
	log.Printf("audio: fake PCM: %s", cmd)
	if cmd == "Drop" || cmd == "Resume" { // the clock starts again from what's written next
		o.start, o.sent = time.Now(), 0
	}
}

// pollable turns a descriptor from D-Bus into a file whose reads and
// writes can time out and be interrupted.
func pollable(fd int, name string) *os.File {
	if err := syscall.SetNonblock(fd, true); err != nil {
		log.Printf("audio: %s: %v", name, err)
	}
	return os.NewFile(uintptr(fd), name)
}

// resampler turns the source's frames into the PCM's rate, by linear
// interpolation between the two input frames around each output one.
type resampler struct {
	src       Source
	in        []int16
	n, idx    int        // samples in in, and how many are used
	cur, next [2]float64 // the input frames around the output position
	have      float64    // the index of next
	pos       float64    // the output position, in input frames
	step      float64    // input frames per output frame
	primed    bool
}

func (r *resampler) read() bool {
	if r.idx >= r.n {
		var err error
		if r.n, err = r.src.Read(r.in); err != nil || r.n == 0 {
			return false
		}
		r.idx = 0
	}
	r.cur = r.next
	r.next = [2]float64{float64(r.in[r.idx]), float64(r.in[r.idx+1])}
	r.idx += 2
	r.have++
	return true
}

// fill appends 16-bit little-endian output to out, up to its capacity,
// with gain applied. It says false once the source has ended.
func (r *resampler) fill(out []byte, channels int, gain float64) ([]byte, bool) {
	if !r.primed {
		if !r.read() || !r.read() {
			return out, false
		}
		r.have, r.primed = 1, true // cur is input frame 0, next frame 1
	}
	for len(out) < cap(out)-8 {
		for r.pos >= r.have {
			if !r.read() {
				return out, false
			}
		}
		frac := r.pos - (r.have - 1)
		l := r.cur[0] + (r.next[0]-r.cur[0])*frac
		rt := r.cur[1] + (r.next[1]-r.cur[1])*frac
		if channels == 1 {
			out = binary.LittleEndian.AppendUint16(out, uint16(clip16((l+rt)/2*gain)))
		} else {
			out = binary.LittleEndian.AppendUint16(out, uint16(clip16(l*gain)))
			out = binary.LittleEndian.AppendUint16(out, uint16(clip16(rt*gain)))
		}
		r.pos += r.step
	}
	return out, true
}

func clip16(v float64) int16 {
	return int16(math.Max(-32768, math.Min(32767, v)))
}

// Clock is d as m:ss, or -:-- when it isn't known.
func Clock(d time.Duration) string {
	if d <= 0 {
		return "-:--"
	}
	s := int(d.Seconds())
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}
