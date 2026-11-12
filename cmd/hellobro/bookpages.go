package main

// A PDF's pages, small: the contents' third tab (Pages; o, or Tab from
// the bookmarks), in the list's grid (libui.go), the page being read
// framed. Tap one, or Enter, to read it; the arrows move the choice,
// PgUp and PgDn go a screenful, a finger scrolls them. Each is drawn by
// MuPDF away from the main loop, one at a time, those showing first,
// then the next screenful's; a few screenfuls are kept.

import (
	"fmt"
	"image"
	"log"

	"hellobro/book"
)

// pdfThumbs are the pages drawn small.
type pdfThumbs struct {
	width  int // px wide they're drawn
	drawn  map[int]*image.Gray
	busy   bool // one being drawn
	aspect float64
}

// pageGrid is the pages small, for the list (libui.go's grid): as it
// lays them out, those wanted are drawn (thumbNext).
func (w *bookWin) pageGrid(a *bro) *libGrid {
	if w.thumbs == nil {
		w.thumbs = &pdfThumbs{drawn: map[int]*image.Gray{}, aspect: 1.3}
		if sz, err := w.pdf.Size(0); err == nil && sz.W > 0 {
			w.thumbs.aspect = sz.H / sz.W
		}
	}
	th := w.thumbs
	return &libGrid{n: len(w.b.Spine), aspect: th.aspect,
		thumb: func(i int) *image.Gray { return th.drawn[i] },
		label: func(i int) string {
			if i == w.spine {
				return fmt.Sprint(i+1, " · reading")
			}
			return fmt.Sprint(i + 1)
		},
		laid: func(width, cols, first, per int) {
			w.listCols, w.listN, w.top = cols, per, first
			if th.width != width { // another size: drawn again
				th.width, th.drawn = width, map[int]*image.Gray{}
			}
			w.thumbNext(a)
		}}
}

// thumbNext draws the next page wanted small, away from the main loop:
// those showing, then the next screenful's.
func (w *bookWin) thumbNext(a *bro) {
	th := w.thumbs
	if th == nil || th.busy || !w.list || !w.listPages {
		return
	}
	per, n := max(1, w.listN), len(w.b.Spine)
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
			if w.list && w.listPages && a.shows(w) {
				w.update(a) // it's laid out again: the next is asked for then
				return
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
