package main

// A PDF's zoom, each PDF its own (kept with its place), and its
// gestures: two fingers pinched apart or together zoom in or out round
// the point between them (the zoom shows while they move; the page is
// drawn again when they lift); a finger dragged on a page bigger than
// the window moves it, anywhere. Taps, keys and swipes still go a
// screenful at a time, from where it is. + and - and the style panel
// zoom too, a step at a time, round the middle.

import (
	"fmt"
	"image"
	"log"
	"math"
	"slices"
	"time"

	"hellobro/book"
	"hellobro/kobo"
)

const (
	ownPan     = 11  // a finger moving a zoomed PDF's page
	pdfZoomMin = 100 // % of the fit
	pdfZoomMax = 400
	pinchStart = 60 // px the fingers' distance changes before it's a pinch
)

// pinch is two fingers zooming a PDF.
type pinch struct {
	w     *bookWin
	d0    float64     // the fingers' distance at first
	from  int         // the zoom then
	z     int         // the zoom now
	at    image.Point // round here
	shown time.Time
}

// canZoom is whether w's page zooms: a PDF's, one page at a time.
func (w *bookWin) canZoom() bool {
	return w.pdf != nil && !w.list && !w.styling && !w.laid.two && w.err == ""
}

// canPan is whether a finger moves w's page: it zooms, and it's bigger
// than the window.
func (w *bookWin) canPan() bool {
	if !w.canZoom() || w.selecting {
		return false
	}
	s, err := w.pdfPage(w.spine, w.laid)
	return err == nil && (s.img.Bounds().Dx() > w.area.Dx() || s.img.Bounds().Dy() > w.area.Dy())
}

// loadZoom takes the PDF's zoom, as kept.
func (w *bookWin) loadZoom(a *bro) {
	w.zoom = pdfZoomMin
	if pl, ok := a.mem.Books[w.path]; ok && pl.Zoom >= pdfZoomMin && pl.Zoom <= pdfZoomMax {
		w.zoom = pl.Zoom
	}
}

// zoomStep zooms a step in (d 1) or out (-1), round the middle.
func (w *bookWin) zoomStep(a *bro, d int) {
	z := w.zoom
	if d > 0 {
		i := slices.IndexFunc(pdfZooms, func(v int) bool { return v > z })
		if i >= 0 {
			z = pdfZooms[i]
		}
	} else {
		for _, v := range pdfZooms {
			if v < w.zoom {
				z = v
			}
		}
	}
	w.setZoom(a, z, w.area.Min.Add(w.area.Max).Div(2))
}

// setZoom zooms to z (% of the fit), what's at p on the screen staying
// there, as near as it can.
func (w *bookWin) setZoom(a *bro, z int, p image.Point) {
	z = max(pdfZoomMin, min(pdfZoomMax, z))
	if z == w.zoom || w.pdf == nil {
		return
	}
	fx, fy := 0.5, 0.5 // where p is on the page, as fractions of it
	if s, err := w.pdfPage(w.spine, w.laid); err == nil && !w.laid.two {
		b, v := s.img.Bounds(), w.view
		if p.In(v.dst) {
			fx = float64(p.X-v.dst.Min.X+v.sp.X-b.Min.X) / float64(b.Dx())
			fy = float64(p.Y-v.dst.Min.Y+v.sp.Y-b.Min.Y) / float64(b.Dy())
		}
	}
	w.zoom = z
	w.layout(a)
	if s, err := w.pdfPage(w.spine, w.laid); err == nil {
		b := s.img.Bounds()
		x := int(fx*float64(b.Dx())) - (p.X - w.area.Min.X)
		y := int(fy*float64(b.Dy())) - (p.Y - w.area.Min.Y)
		w.panAt(b, image.Pt(x, y))
	}
	log.Printf("pdf: zoom %d%%", z)
	a.Notify(fmt.Sprintf("zoom %d%%", z))
	w.save(a)
	w.draw(a)
}

// panAt puts the window's top left at q of the page drawn (b), kept
// in it: its place, in thousandths, as a screenful's is.
func (w *bookWin) panAt(b image.Rectangle, q image.Point) {
	q.X = max(0, min(q.X, b.Dx()-w.area.Dx()))
	q.Y = max(0, min(q.Y, b.Dy()-w.area.Dy()))
	w.at = book.Pos{Block: q.Y * 1000 / max(1, b.Dy()), Off: q.X * 1000 / max(1, b.Dx())}
	w.panned = true
	w.page = book.PageAt(w.pages, w.at)
}

// panTile is where the window's top left is in the page drawn (b),
// moved there by a finger.
func (w *bookWin) panTile(b image.Rectangle) image.Point {
	x := (w.at.Off*b.Dx() + 999) / 1000
	y := (w.at.Block*b.Dy() + 999) / 1000
	return image.Pt(max(0, min(x, b.Dx()-w.area.Dx())), max(0, min(y, b.Dy()-w.area.Dy())))
}

// panStep goes a screenful on (d 1) or back (-1) from where a finger
// left the page: along, then down to the next row's start, as the
// screenfuls go; false at the page's end, for the next page.
func (w *bookWin) panStep(d int) bool {
	s, err := w.pdfPage(w.spine, w.laid)
	if err != nil {
		return false
	}
	b := s.img.Bounds()
	q, sz := w.panTile(b), w.area.Size()
	end := image.Pt(max(0, b.Dx()-sz.X), max(0, b.Dy()-sz.Y))
	step := sz.Sub(sz.Div(pdfOverlap))
	switch {
	case d > 0 && q.X < end.X:
		q.X += step.X
	case d > 0 && q.Y < end.Y:
		q.X, q.Y = 0, q.Y+step.Y
	case d < 0 && q.X > 0:
		q.X -= step.X
	case d < 0 && q.Y > 0:
		q.X, q.Y = end.X, q.Y-step.Y
	default:
		return false
	}
	w.panAt(b, q)
	w.dropCut()
	return true
}

// panTo moves the page so its point sp (in the page drawn) is at the
// window's top left; done: drawn whole, and kept.
func (w *bookWin) panTo(a *bro, sp image.Point, done bool) {
	s, err := w.pdfPage(w.spine, w.laid)
	if err != nil {
		return
	}
	b := s.img.Bounds()
	w.panAt(b, sp.Sub(b.Min))
	if done {
		w.save(a)
		w.draw(a)
		return
	}
	w.views = nil // the page only, in the fast black and white: the rest when it's done
	a.C.Fill(w.area, kobo.White)
	w.drawPDFIn(a, w.spine, w.area, w.panTile(b), 0)
	w.view = w.views[0]
	a.Mark(w.area, false)
}

// pairMove follows two fingers together: on a PDF's page, apart or
// together, a pinch; else maybe a workspace swipe.
func (a *bro) pairMove() {
	f, g := a.pair[0], a.pair[1]
	if f == nil || g == nil {
		return
	}
	d1 := dist(f.Last, g.Last)
	if a.pinch == nil {
		w, ok := a.windowAt(f.Start).(*bookWin)
		d0 := dist(f.Start, g.Start)
		if !ok || !w.canZoom() || a.windowAt(g.Start) != window(w) || math.Abs(d1-d0) < pinchStart {
			a.pairSwipe()
			return
		}
		if w.cut != (image.Rectangle{}) || w.corner != nil {
			w.dropCut()
		}
		a.pinch = &pinch{w: w, d0: max(1, d0), from: w.zoom, z: w.zoom, at: f.Start.Add(g.Start).Div(2)}
		log.Printf("pdf: pinch from %d%%", w.zoom)
	}
	p := a.pinch
	z := int(math.Round(float64(p.from)*d1/p.d0/5)) * 5
	z = max(pdfZoomMin, min(pdfZoomMax, z))
	if z == p.z {
		return
	}
	p.z = z // where it ends: the label only now and then
	if time.Since(p.shown) >= 150*time.Millisecond {
		p.shown = time.Now()
		p.w.drawPinch(a, z)
	}
}

// pinchEnd zooms as the pinch left it, once a finger lifts.
func (a *bro) pinchEnd() {
	p := a.pinch
	a.pinch = nil
	log.Printf("pdf: pinched to %d%%", p.z)
	if p.z == p.w.zoom {
		p.w.draw(a) // the label away
		return
	}
	p.w.setZoom(a, p.z, p.at)
}

// drawPinch shows the zoom a pinch is at, in the page's middle.
func (w *bookWin) drawPinch(a *bro, z int) {
	if !a.shows(w) {
		return
	}
	m := w.area.Min.Add(w.area.Max).Div(2)
	r := image.Rect(m.X-150, m.Y-45, m.X+150, m.Y+45)
	c := a.C
	c.Fill(r, kobo.White)
	c.Frame(r, 3, kobo.Black)
	c.Text(a.F.Body, r, fmt.Sprintf("zoom %d%%", z), kobo.Black, kobo.AlignCenter)
	a.Mark(r, false)
}

func dist(p, q image.Point) float64 {
	d := p.Sub(q)
	return math.Hypot(float64(d.X), float64(d.Y))
}
