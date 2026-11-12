package layout

import (
	"image"
	"math"
	"time"

	"hellobro/ink"
)

// fling is what a flick leaves: the content coasting on at the speed the
// finger left it, slowing to a stop. Each of its frames is a quick one;
// the one after it stops puts the grays back.
type fling struct {
	samples []sample   // the finger's last places, for its speed
	v       [2]float32 // px/s, the way the content goes
	rem     [2]float32 // what's gone less than a pixel
	at      time.Time  // its last step
}

type sample struct {
	t time.Time
	p image.Point
}

const (
	flingMin      ink.Dp = 400  // Dp/s: slower is no flick, the content stays where it's let go
	flingStop     ink.Dp = 80   // Dp/s: it stops, slower
	flingMax      ink.Dp = 5000 // Dp/s at most
	flingFriction        = 3.5  // per second: it loses this share of its speed each second, as e^-kt
	flingWindow          = 100 * time.Millisecond
	flingFrame           = 40 * time.Millisecond
)

func (f *fling) stop() {
	f.v, f.rem = [2]float32{}, [2]float32{}
	f.samples = f.samples[:0]
}

func (f *fling) active() bool { return f.v != [2]float32{} }

// track keeps p, the finger's place at t, with the ones just before.
func (f *fling) track(p image.Point, t time.Time) {
	f.samples = append(f.samples, sample{t, p})
	k := 0
	for k < len(f.samples)-1 && t.Sub(f.samples[k].t) > flingWindow {
		k++
	}
	f.samples = f.samples[k:]
}

// start sets it off as the finger lifts at t, if it was moving fast
// enough; axes says which ways it may go (x, y).
func (f *fling) start(gtx ink.Ctx, t time.Time, axes [2]bool) bool {
	defer func() { f.samples = f.samples[:0] }()
	if len(f.samples) < 2 {
		return false
	}
	a, b := f.samples[0], f.samples[len(f.samples)-1]
	dt := float32(b.t.Sub(a.t).Seconds())
	if dt < 0.01 || t.Sub(b.t) > flingWindow { // held still before it lifted
		return false
	}
	v := [2]float32{-float32(b.p.X-a.p.X) / dt, -float32(b.p.Y-a.p.Y) / dt}
	for i := range v {
		if !axes[i] {
			v[i] = 0
		}
	}
	speed := float32(math.Hypot(float64(v[0]), float64(v[1])))
	if speed < float32(gtx.Dp(flingMin)) {
		return false
	}
	if m := float32(gtx.Dp(flingMax)); speed > m {
		v[0], v[1] = v[0]*m/speed, v[1]*m/speed
	}
	f.v, f.rem, f.at = v, [2]float32{}, t
	gtx.InvalidateAt(t.Add(flingFrame))
	return true
}

// step is how far it went since its last step, asking for the next frame.
func (f *fling) step(gtx ink.Ctx) image.Point {
	if !f.active() {
		return image.Point{}
	}
	dt := float32(gtx.Now.Sub(f.at).Seconds())
	if dt <= 0 {
		gtx.InvalidateAt(f.at.Add(flingFrame))
		return image.Point{}
	}
	dt = min(dt, 0.2)
	f.at = gtx.Now
	decay := float32(math.Exp(float64(-flingFriction * dt)))
	var d [2]int
	for i := range f.v {
		moved := f.v[i]*(1-decay)/flingFriction + f.rem[i]
		d[i] = int(moved)
		f.rem[i] = moved - float32(d[i])
		f.v[i] *= decay
	}
	if float32(math.Hypot(float64(f.v[0]), float64(f.v[1]))) < float32(gtx.Dp(flingStop)) {
		f.v = [2]float32{}
		gtx.InvalidateAt(gtx.Now) // the frame after: grays
	} else {
		gtx.InvalidateAt(gtx.Now.Add(flingFrame))
	}
	return image.Pt(d[0], d[1])
}
