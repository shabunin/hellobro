package main

// A PDF's pages, small: the contents' third tab (Pages; o, or Tab from
// the bookmarks), a screenful at a time, the page being read framed.
// Tap one, or Enter, to read it; the arrows move the choice, PgUp and
// PgDn or a finger swiped up and down go a screenful. Each is drawn by
// MuPDF away from the main loop, one at a time, those showing first,
// then the next screenful's; a few screenfuls are kept.

import (
	"fmt"
	"image"
	"log"

	"hellobro/book"
	"hellobro/kobo"
)

const (
	thumbW   = 240 // px a page is at least, small
	thumbGap = 24
	thumbLab = 36 // px under it for its number
)

// pdfThumbs are the pages drawn small.
type pdfThumbs struct {
	width  int // px wide they're drawn
	drawn  map[int]*image.Gray
	busy   bool // one being drawn
	aspect float64
}

// thumbLayout is where the pages show small in area: a tile each, as
// many as fit, and how many across.
func (w *bookWin) thumbLayout(area image.Rectangle) ([]image.Rectangle, int) {
	if w.thumbs == nil {
		w.thumbs = &pdfThumbs{drawn: map[int]*image.Gray{}, aspect: 1.3}
		if sz, err := w.pdf.Size(0); err == nil && sz.W > 0 {
			w.thumbs.aspect = sz.H / sz.W
		}
	}
	k := w.thumbs.aspect
	cols := max(2, (area.Dx()+thumbGap)/(thumbW+thumbGap))
	tw := (area.Dx() - (cols-1)*thumbGap) / cols
	th := int(float64(tw) * k)
	if th+thumbLab > area.Dy() { // a short window: one row, as tall as it goes
		th = area.Dy() - thumbLab
		tw = int(float64(th) / k)
		cols = max(1, (area.Dx()+thumbGap)/(tw+thumbGap))
	}
	rows := max(1, (area.Dy()+thumbGap)/(th+thumbLab+thumbGap))
	x0 := area.Min.X + (area.Dx()-cols*tw-(cols-1)*thumbGap)/2
	var tiles []image.Rectangle
	for r := range rows {
		for c := range cols {
			x, y := x0+c*(tw+thumbGap), area.Min.Y+r*(th+thumbLab+thumbGap)
			tiles = append(tiles, image.Rect(x, y, x+tw, y+th))
		}
	}
	return tiles, cols
}

// drawThumbs draws the pages small in area, the choice (w.sel) in the
// screenful (from w.top).
func (w *bookWin) drawThumbs(a *bro, area image.Rectangle) {
	tiles, cols := w.thumbLayout(area)
	per, n := len(tiles), len(w.b.Spine)
	w.listN, w.listCols, w.thumbTiles = per, cols, tiles
	if per == 0 {
		return
	}
	if w.thumbs.width != tiles[0].Dx() { // another size: drawn again
		w.thumbs.width, w.thumbs.drawn = tiles[0].Dx(), map[int]*image.Gray{}
	}
	w.sel = max(0, min(w.sel, n-1))
	w.top = w.sel / per * per
	for i, t := range tiles {
		pg := w.top + i
		if pg >= n {
			break
		}
		w.drawThumb(a, pg, t)
		lab := image.Rect(t.Min.X-thumbGap/2, t.Min.Y-thumbGap/2, t.Max.X+thumbGap/2, t.Max.Y+thumbLab)
		w.rows = append(w.rows, bookRow{lab, func() { w.listOpen(a, pg) }})
	}
	w.thumbNext(a)
}

// drawThumb draws page pg small in t: drawn, or blank till it is; its
// frame, thick if it's the choice; its number under it.
func (w *bookWin) drawThumb(a *bro, pg int, t image.Rectangle) {
	c := a.C
	c.Fill(image.Rect(t.Min.X-8, t.Min.Y-8, t.Max.X+8, t.Max.Y+thumbLab), kobo.White)
	if m := w.thumbs.drawn[pg]; m != nil {
		kobo.CopyGray(c.Gray, t.Intersect(m.Bounds().Add(t.Min)), m, image.Point{})
	}
	frame, ink := 1, uint8(kobo.Mid)
	if pg == w.sel {
		frame, ink = 5, kobo.Black
	}
	for _, s := range frameStrips(t.Inset(-frame), frame) {
		c.Fill(s, ink)
	}
	num := fmt.Sprint(pg + 1)
	if pg == w.spine {
		num += " · reading"
	}
	c.Text(a.F.Small, image.Rect(t.Min.X, t.Max.Y, t.Max.X, t.Max.Y+thumbLab), num, kobo.Black, kobo.AlignCenter)
}

// thumbNext draws the next page wanted small, away from the main loop:
// those showing, then the next screenful's.
func (w *bookWin) thumbNext(a *bro) {
	th := w.thumbs
	if th == nil || th.busy || !w.list || !w.listPages {
		return
	}
	per, n := len(w.thumbTiles), len(w.b.Spine)
	pg := -1
	for i := w.top; i < min(n, w.top+2*per); i++ {
		if th.drawn[i] == nil {
			pg = i
			break
		}
	}
	if pg < 0 {
		return
	}
	th.busy = true
	d, width := w.pdf, th.width
	go func() {
		m, err := renderPDFPage(d, pg, width)
		a.Do(func() {
			th.busy = false
			if err != nil {
				log.Printf("pdf: page %d small: %v", pg+1, err)
				return
			}
			if w.thumbs != th || th.width != width {
				return
			}
			th.drawn[pg] = m
			for k := range th.drawn { // a few screenfuls kept
				if k < w.top-per || k >= w.top+2*per {
					delete(th.drawn, k)
				}
			}
			if i := pg - w.top; i >= 0 && i < per && w.list && w.listPages && a.shows(w) {
				t := w.thumbTiles[i]
				w.drawThumb(a, pg, t)
				a.Mark(image.Rect(t.Min.X-8, t.Min.Y-8, t.Max.X+8, t.Max.Y+thumbLab), true)
			}
			w.thumbNext(a)
		})
	}()
}

// showPages shows the PDF's pages small, the one being read chosen.
func (w *bookWin) showPages(a *bro) {
	if w.pdf == nil {
		return
	}
	w.list, w.listMks, w.listPages, w.sel = true, false, true, w.spine
	w.draw(a)
}

// openThumb reads page pg, from the pages small.
func (w *bookWin) openThumb(a *bro, pg int) {
	if pg < 0 || pg >= len(w.b.Spine) {
		return
	}
	w.list = false
	w.goTo(pg, book.Pos{})
	w.draw(a)
	a.MarkFlash(w.p.Body)
	w.save(a)
}
