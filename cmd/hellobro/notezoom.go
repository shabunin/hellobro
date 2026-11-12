package main

// A notebook zoomed, by hand: two fingers apart make its pages bigger
// (up to noteZoomMax times as big as they fit), together smaller again;
// a finger then moves them about. Swipes turn the pages only at their
// fit. The pen writes on the pages zoomed as on any.

import (
	"fmt"
	"image"
	"log"
	"math"
	"time"

	"hellobro/kobo"
)

const noteZoomMax = 3.0

// notePinch is two fingers zooming a notebook.
type notePinch struct {
	w     *noteWin
	d0    float64     // the fingers' distance at first
	from  float64     // the zoom then
	z     float64     // and now
	at    image.Point // round here
	shown time.Time
	drawn image.Rectangle // the frame and the zoom said, over the pages
}

// notePan is a finger moving a zoomed notebook's pages.
type notePan struct {
	g     *kobo.Gesture
	from  image.Point // the pan it started from
	drawn time.Time
}

// zoomed says its pages are bigger than they fit.
func (w *noteWin) zoomed() bool { return w.zoom > 1 }

// canZoom says fingers may zoom it: its pages shown, not small, not
// its settings.
func (w *noteWin) canZoom() bool { return !w.overview && !w.settings && len(w.slots) > 0 }

// applyZoom makes the slots, laid out to fit, zoom times as big, round
// the paper's top left, and moves them by the pan: kept so the pages
// fill the paper, or in its middle where they're smaller.
func (w *noteWin) applyZoom() {
	if w.zoom <= 1 || len(w.slots) == 0 || w.overview {
		w.zoom, w.pan = 1, image.Point{}
		return
	}
	area := w.paperArea()
	o := area.Min
	sc := func(p image.Point) image.Point {
		return o.Add(image.Pt(int(math.Round(float64(p.X-o.X)*w.zoom)), int(math.Round(float64(p.Y-o.Y)*w.zoom))))
	}
	var all image.Rectangle
	for i, s := range w.slots {
		w.slots[i] = image.Rectangle{sc(s.Min), sc(s.Max)}
		all = all.Union(w.slots[i])
	}
	w.scale *= w.zoom
	keep := func(p, lo, hi, a0, a1 int) int {
		if hi-lo <= a1-a0 {
			return (lo+hi)/2 - (a0+a1)/2
		}
		return max(lo-a0, min(p, hi-a1))
	}
	w.pan = image.Pt(keep(w.pan.X, all.Min.X, all.Max.X, area.Min.X, area.Max.X), keep(w.pan.Y, all.Min.Y, all.Max.Y, area.Min.Y, area.Max.Y))
	for i := range w.slots {
		w.slots[i] = w.slots[i].Sub(w.pan)
	}
}

// zoomAt zooms to z, what's at p on the screen staying there, as near
// as it can.
func (w *noteWin) zoomAt(a *bro, z float64, p image.Point) {
	z = max(1, min(noteZoomMax, z))
	o := w.paperArea().Min
	z0 := max(1, w.zoom)
	// p on the pages as they fit, then where that goes zoomed.
	fx := float64(p.X+w.pan.X-o.X) / z0
	fy := float64(p.Y+w.pan.Y-o.Y) / z0
	w.zoom = z
	w.pan = image.Pt(int(fx*z)-(p.X-o.X), int(fy*z)-(p.Y-o.Y))
	w.place(a, w.p)
	log.Printf("notebook %s: zoom %.0f%%", w.nb.name, z*100)
	w.show(a)
}

// notePinchStart makes two fingers on a notebook's pages a pinch, if
// they've gone apart or together far enough.
func (a *bro) notePinchStart(f, g *kobo.Gesture, d0, d1 float64) bool {
	w, ok := a.windowAt(f.Start).(*noteWin)
	if !ok || !w.canZoom() || a.windowAt(g.Start) != window(w) || math.Abs(d1-d0) < pinchStart || a.vkFromFoot(f, g) {
		return false
	}
	a.nt.pan = nil
	a.nt.pinch = &notePinch{w: w, d0: max(1, d0), from: max(1, w.zoom), z: max(1, w.zoom), at: f.Start.Add(g.Start).Div(2)}
	log.Printf("notebook %s: pinch from %.0f%%", w.nb.name, w.zoom*100)
	return true
}

// notePinchMove follows it: the zoom it'd be, and where, now and then.
func (a *bro) notePinchMove(d1 float64) {
	p := a.nt.pinch
	z := max(1, min(noteZoomMax, math.Round(p.from*d1/p.d0*10)/10))
	if z == p.z {
		return
	}
	p.z = z
	if time.Since(p.shown) >= 150*time.Millisecond {
		p.shown = time.Now()
		p.w.drawPinch(a, p)
	}
}

// notePinchEnd zooms as the pinch left it, once a finger lifts.
func (a *bro) notePinchEnd() {
	p := a.nt.pinch
	a.nt.pinch = nil
	if !a.shows(p.w) {
		return
	}
	if p.z == p.w.zoom || p.z == 1 && !p.w.zoomed() {
		p.w.show(a) // the frame away
		return
	}
	p.w.zoomAt(a, p.z, p.at)
	a.Notify(fmt.Sprintf("zoom %.0f%%", p.z*100))
}

// drawPinch shows where a pinch is: a frame round what the paper will
// show, zoomed so, and the zoom in its middle; the pages again where
// it drew before.
func (w *noteWin) drawPinch(a *bro, p *notePinch) {
	if !a.shows(w) {
		return
	}
	area, old := w.paperArea(), p.drawn
	if !old.Empty() {
		w.drawPages(a, old)
	}
	k := p.z / p.from // as big as the paper, it's this much of it now
	sz := image.Pt(int(float64(area.Dx())/k), int(float64(area.Dy())/k))
	at := p.at.Sub(image.Pt(int(float64(p.at.X-area.Min.X)/k), int(float64(p.at.Y-area.Min.Y)/k)))
	zone := image.Rectangle{at, at.Add(sz)}.Intersect(area)
	c := a.C
	if zone.Dx() > 12 && zone.Dy() > 12 && zone != area {
		c.Frame(zone, 5, kobo.Black)
		c.Frame(zone.Inset(5), 3, kobo.White)
	}
	m := area.Min.Add(area.Max).Div(2)
	if !zone.Empty() {
		m = zone.Min.Add(zone.Max).Div(2)
	}
	r := image.Rect(m.X-150, m.Y-45, m.X+150, m.Y+45)
	r = r.Add(image.Pt(max(0, area.Min.X-r.Min.X)+min(0, area.Max.X-r.Max.X), max(0, area.Min.Y-r.Min.Y)+min(0, area.Max.Y-r.Max.Y)))
	a.inkPaint(&w.zoomPane, r, a.inkButton(fmt.Sprintf("zoom %.0f%%", p.z*100), false))
	p.drawn = r.Union(zone).Intersect(area)
	a.Mark(old.Union(p.drawn), false)
}

// notePanMove moves a zoomed notebook's pages with the finger g, at p:
// drawn in the fast black and white now and then; done, whole.
func (a *bro) notePanMove(w *noteWin, g *kobo.Gesture, p image.Point, done bool) {
	if a.nt.pan == nil || a.nt.pan.g != g {
		a.nt.pan = &notePan{g: g, from: w.pan}
	}
	pn := a.nt.pan
	if !done && time.Since(pn.drawn) < 150*time.Millisecond {
		return
	}
	pn.drawn = time.Now()
	w.pan = pn.from.Add(g.Start.Sub(p))
	w.place(a, w.p)
	if done {
		a.nt.pan = nil
		w.show(a)
		return
	}
	w.drawPages(a, w.paperArea())
	a.Mark(w.paperArea(), false)
}
