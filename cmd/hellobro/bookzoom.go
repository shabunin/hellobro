package main

// A PDF's zoom, each PDF its own (kept with its place), and its
// gestures: two fingers pinched apart or together zoom in or out round
// the point between them (while they move, a frame shows what will fill
// the window, and the zoom; the page is drawn again when they lift); a finger dragged on a page bigger than
// the window moves it, anywhere. Taps, keys and swipes still go a
// screenful at a time, from where it is. + and - and the style panel
// zoom too, a step at a time, round the middle; and a cut's zoom button,
// the cut as big as the window takes it, in its middle.

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
	drawn image.Rectangle // what it drew over the page: the frame, the zoom
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
	fx, fy := w.pageFrac(p)
	w.zoomTo(a, z, fx, fy, p)
}

// pageFrac is where p on the screen is on the page shown, as fractions
// of it: its middle if p's off it.
func (w *bookWin) pageFrac(p image.Point) (float64, float64) {
	s, err := w.pdfPage(w.spine, w.laid)
	if err != nil || w.laid.two || !p.In(w.view.dst) {
		return 0.5, 0.5
	}
	b, v := s.img.Bounds(), w.view
	return float64(p.X-v.dst.Min.X+v.sp.X-b.Min.X) / float64(b.Dx()), float64(p.Y-v.dst.Min.Y+v.sp.Y-b.Min.Y) / float64(b.Dy())
}

// zoomCut zooms so the cut fills the window as far as it can, in its
// middle; the cut goes.
func (w *bookWin) zoomCut(a *bro) {
	r := w.cut
	if r.Empty() || !w.canZoom() {
		return
	}
	fx, fy := w.pageFrac(r.Min.Add(r.Max).Div(2))
	k := min(float64(w.area.Dx())/float64(r.Dx()), float64(w.area.Dy())/float64(r.Dy()))
	z := max(pdfZoomMin, min(pdfZoomMax, int(float64(w.zoom)*k)/5*5))
	w.dropCut()
	w.zoomTo(a, z, fx, fy, w.area.Min.Add(w.area.Max).Div(2))
}

// zoomTo zooms to z, the page's point at fractions fx, fy of it at p on
// the screen, as near as it can.
func (w *bookWin) zoomTo(a *bro, z int, fx, fy float64, p image.Point) {
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
		if !ok || !w.canZoom() || a.windowAt(g.Start) != window(w) || math.Abs(d1-d0) < pinchStart || a.vkFromFoot(f, g) {
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
		p.w.drawPinch(a, p)
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

// drawPinch shows where a pinch is: a frame round what the window
// will show, zoomed so, and the zoom in its middle; what it drew
// before, the page again.
func (w *bookWin) drawPinch(a *bro, p *pinch) {
	if !a.shows(w) {
		return
	}
	c, old := a.C, p.drawn
	w.pageUnder(a, old)
	zone := w.pinchZone(p.z, p.at).Intersect(w.area)
	if zone.Dx() > 12 && zone.Dy() > 12 && zone != w.area {
		c.Frame(zone, 5, kobo.Black)
		c.Frame(zone.Inset(5), 3, kobo.White)
	}
	m := w.area.Min.Add(w.area.Max).Div(2)
	if !zone.Empty() {
		m = zone.Min.Add(zone.Max).Div(2)
	}
	r := image.Rect(m.X-150, m.Y-45, m.X+150, m.Y+45)
	r = r.Add(image.Pt(max(0, w.area.Min.X-r.Min.X)+min(0, w.area.Max.X-r.Max.X), max(0, w.area.Min.Y-r.Min.Y)+min(0, w.area.Max.Y-r.Max.Y)))
	c.Fill(r, kobo.White)
	c.Frame(r, 3, kobo.Black)
	c.Text(a.F.Body, r, fmt.Sprintf("zoom %d%%", p.z), kobo.Black, kobo.AlignCenter)
	p.drawn = r.Union(zone)
	a.Mark(old.Union(p.drawn), false)
}

// pinchZone is where on the screen what the window would show zoomed
// to z round p is now: as setZoom will place it.
func (w *bookWin) pinchZone(z int, p image.Point) image.Rectangle {
	s, err := w.pdfPage(w.spine, w.laid)
	if err != nil || w.zoom <= 0 {
		return image.Rectangle{}
	}
	b, v, sz := s.img.Bounds(), w.view, w.area.Size()
	k := float64(z) / float64(w.zoom) // the page that much bigger
	q := image.Pt(b.Dx()/2, b.Dy()/2) // p's point on the page now
	if p.In(v.dst) {
		q = p.Sub(v.dst.Min).Add(v.sp).Sub(b.Min)
	}
	scale := func(q image.Point, k float64) image.Point {
		return image.Pt(int(float64(q.X)*k), int(float64(q.Y)*k))
	}
	big := scale(b.Size(), k)
	o := scale(q, k).Sub(p.Sub(w.area.Min)) // the window's top left on the page zoomed
	o.X = max(0, min(o.X, big.X-sz.X))
	o.Y = max(0, min(o.Y, big.Y-sz.Y))
	at := scale(o, 1/k)
	r := image.Rectangle{at, at.Add(scale(sz, 1/k))}
	return r.Add(b.Min).Sub(v.sp).Add(v.dst.Min)
}

// pageUnder draws the page shown again in r, white off it.
func (w *bookWin) pageUnder(a *bro, r image.Rectangle) {
	r = r.Intersect(w.area)
	if r.Empty() {
		return
	}
	a.C.Fill(r, kobo.White)
	s, err := w.pdfPage(w.spine, w.laid)
	if err != nil {
		return
	}
	v := w.view
	in := r.Intersect(v.dst)
	kobo.CopyGray(a.C.Gray, in, s.img, in.Min.Sub(v.dst.Min).Add(v.sp))
}

func dist(p, q image.Point) float64 {
	d := p.Sub(q)
	return math.Hypot(float64(d.X), float64(d.Y))
}
