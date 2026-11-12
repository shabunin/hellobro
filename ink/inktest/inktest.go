// Package inktest drives an ink UI without a screen: taps, swipes and
// keys in, frames out as images (with the focus ring stamped, as the
// host would), PNGs to look at, and words found where they were drawn.
package inktest

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"time"

	"hellobro/ink"
)

// T is a window being driven.
type T struct {
	W       *ink.Window
	Now     time.Time
	Updates []ink.Update // the last frame's
	contact int
}

// Elipsa is the Kobo Elipsa's screen, upright, and its pixels per Dp.
var (
	Elipsa       = image.Pt(1404, 1872)
	ElipsaMetric = ink.Metric{PxPerDp: 1.42}
)

// New is ui in a window size big, laid out once.
func New(size image.Point, m ink.Metric, ui ink.Widget) *T {
	t := &T{W: ink.NewWindow(size, m, ui), Now: time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)}
	t.Frame()
	return t
}

// Frame lays it out if it's due, a moment on.
func (t *T) Frame() []ink.Update {
	t.Now = t.Now.Add(40 * time.Millisecond)
	t.Updates = t.W.Frame(t.Now)
	return t.Updates
}

// Wait lets d go by, and the frames that come due meanwhile.
func (t *T) Wait(d time.Duration) {
	end := t.Now.Add(d)
	for t.Now.Before(end) {
		if w := t.W.Wake(); !w.IsZero() && w.Before(end) && w.After(t.Now) {
			t.Now = w.Add(-40 * time.Millisecond)
		} else if w.IsZero() || !w.Before(end) {
			t.Now = end.Add(-40 * time.Millisecond)
		}
		t.Frame()
	}
}

func (t *T) pointer(k ink.PointerKind, p image.Point) {
	t.W.Pointer(ink.Pointer{Kind: k, ID: t.contact, Pos: p, Time: t.Now})
}

// Press puts a finger down at p, and a frame.
func (t *T) Press(p image.Point) {
	t.contact++
	t.pointer(ink.Press, p)
	t.Frame()
}

// Release lifts it at p, and a frame.
func (t *T) Release(p image.Point) {
	t.pointer(ink.Release, p)
	t.Frame()
}

// Tap taps p.
func (t *T) Tap(p image.Point) {
	t.Press(p)
	t.Release(p)
}

// Hold presses p, holds it d, lifts it.
func (t *T) Hold(p image.Point, d time.Duration) {
	t.Press(p)
	t.Wait(d)
	t.Release(p)
}

// Swipe goes from a to b in a few steps.
func (t *T) Swipe(a, b image.Point) {
	t.Press(a)
	for i := 1; i <= 4; i++ {
		t.pointer(ink.Move, a.Add(b.Sub(a).Mul(i).Div(4)))
		t.Frame()
	}
	t.Release(b)
}

// Find is where s was drawn, its middle.
func (t *T) Find(s string) (image.Point, error) {
	r, ok := t.W.FindText(s)
	if !ok {
		return image.Point{}, fmt.Errorf("%q isn't on screen; it shows %q", s, t.W.Texts())
	}
	return r.Min.Add(r.Max).Div(2), nil
}

// TapText taps the words s.
func (t *T) TapText(s string) error {
	p, err := t.Find(s)
	if err != nil {
		return err
	}
	t.Tap(p)
	return nil
}

// Key presses a key (with mods), and a frame. It says if anyone took it.
func (t *T) Key(name string, mods ink.Mods) bool {
	ok := t.W.Key(ink.Key{Name: name, Mods: mods})
	t.Frame()
	return ok
}

// Type types s.
func (t *T) Type(s string) {
	for _, r := range s {
		t.W.Text(string(r))
	}
	t.Frame()
}

// Image is what shows: the frame, the focus ring on it.
func (t *T) Image() *image.Gray {
	src := t.W.Image()
	img := image.NewGray(src.Rect)
	copy(img.Pix, src.Pix)
	StampRing(img, t.W.Ring())
	return img
}

// Save writes what shows to path, a PNG.
func (t *T) Save(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, t.Image()); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// StampRing draws a dashed focus ring, its outer edge o, as hellobro's
// is (kobo/ring.go).
func StampRing(img *image.Gray, o image.Rectangle) {
	const w, dash = 3, 12
	if o.Dx() < 2*w || o.Dy() < 2*w {
		return
	}
	strips := []image.Rectangle{
		image.Rect(o.Min.X, o.Min.Y, o.Max.X, o.Min.Y+w),
		image.Rect(o.Min.X, o.Max.Y-w, o.Max.X, o.Max.Y),
		image.Rect(o.Min.X, o.Min.Y+w, o.Min.X+w, o.Max.Y-w),
		image.Rect(o.Max.X-w, o.Min.Y+w, o.Max.X, o.Max.Y-w),
	}
	for i, s := range strips {
		s = s.Intersect(img.Rect)
		for y := s.Min.Y; y < s.Max.Y; y++ {
			for x := s.Min.X; x < s.Max.X; x++ {
				along := x - o.Min.X
				if i >= 2 {
					along = y - o.Min.Y
				}
				if along%(dash*3/2) < dash {
					img.Pix[img.PixOffset(x, y)] = 0
				}
			}
		}
	}
}
