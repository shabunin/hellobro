package main

// The notebook's pages, all of them: tapping the page counter (or O)
// shows them small, a screenful at a time (the arrows, or a finger
// swiped across, for more), and a tile after the last for a new one.
// Tap a page to choose it, again to open it. The strip then has: open,
// new before, new after, copy page, clear page (asking first), delete,
// earlier and later (moving it). Each is a change to undo, as strokes
// are.

import (
	"fmt"
	"image"
	"log"
	"math"
	"slices"

	"hellobro/kobo"
)

const (
	ownNoteOv    = 10  // a contact on the pages, small
	noteThumbW   = 260 // px a page is at least, small
	noteThumbGap = 24
	noteThumbLab = 36 // px under it for its number
)

// What a noteOp does: to a page's strokes, or a page put in or taken out.
const (
	pageStrokes = iota
	pageIns
	pageDel
)

// defaultMeta is a page as a new one is, in the file: the Elipsa's
// screen, in points.
func defaultMeta() notePage {
	return notePage{w: math.Round(notePageW*notePt72*100) / 100, h: math.Round(notePageH*notePt72*100) / 100}
}

// padMeta gives each page what it has besides its strokes, so pages
// can be put in and taken out with it.
func (nb *notebook) padMeta() {
	for len(nb.meta) < len(nb.pages) {
		nb.meta = append(nb.meta, defaultMeta())
	}
}

func (nb *notebook) insertPage(at int, ss []*noteStroke, m notePage) {
	nb.padMeta()
	nb.pages = slices.Insert(nb.pages, at, slices.Clone(ss))
	nb.meta = slices.Insert(nb.meta, at, m)
}

func (nb *notebook) removePage(at int) {
	nb.padMeta()
	nb.pages = slices.Delete(nb.pages, at, at+1)
	nb.meta = slices.Delete(nb.meta, at, at+1)
	if len(nb.pages) == 0 {
		nb.pages, nb.meta = [][]*noteStroke{nil}, []notePage{defaultMeta()}
	}
}

// pageOp is page at taken out, to put back on undo.
func (nb *notebook) delPageOp(at int) noteOp {
	nb.padMeta()
	return noteOp{kind: pageDel, page: at, add: slices.Clone(nb.pages[at]), meta: nb.meta[at]}
}

// ovLayout is where the pages show small: a tile each, as many as fit,
// and how many across.
func (w *noteWin) ovLayout() ([]image.Rectangle, int) {
	area := w.paperArea().Inset(noteThumbGap)
	if area.Empty() {
		return nil, 1
	}
	cols := max(2, (area.Dx()+noteThumbGap)/(noteThumbW+noteThumbGap))
	tw := (area.Dx() - (cols-1)*noteThumbGap) / cols
	size := w.pageSize(0)
	th := tw * size.Y / size.X
	if th+noteThumbLab > area.Dy() { // a short window: one row, as tall as it goes
		th = area.Dy() - noteThumbLab
		tw = th * size.X / size.Y
		cols = max(1, (area.Dx()+noteThumbGap)/(tw+noteThumbGap))
	}
	rows := max(1, (area.Dy()+noteThumbGap)/(th+noteThumbLab+noteThumbGap))
	x0 := area.Min.X + (area.Dx()-cols*tw-(cols-1)*noteThumbGap)/2
	var tiles []image.Rectangle
	for r := range rows {
		for c := range cols {
			x, y := x0+c*(tw+noteThumbGap), area.Min.Y+r*(th+noteThumbLab+noteThumbGap)
			tiles = append(tiles, image.Rect(x, y, x+tw, y+th))
		}
	}
	return tiles, cols
}

// ovShow makes the chosen page one that shows.
func (w *noteWin) ovShow() {
	tiles, _ := w.ovLayout()
	per := max(1, len(tiles))
	w.ovSel = max(0, min(w.ovSel, len(w.nb.pages)-1))
	if w.ovSel < w.ovFirst || w.ovSel >= w.ovFirst+per {
		w.ovFirst = w.ovSel / per * per
	}
}

// overviewOn shows the pages small, or the page again.
func (w *noteWin) overviewOn(a *bro, on bool) {
	w.overview, w.penMenu, w.sel, w.mark = on, false, nil, nil
	if on {
		w.ovSel = min(w.cur, len(w.nb.pages)-1)
		w.ovShow()
	}
	w.place(a, w.p) // the strip has other buttons
	w.show(a)
}

// openPage shows page n, from the overview.
func (w *noteWin) openPage(a *bro, n int) {
	w.cur = n
	log.Printf("notebook %s: page %d opened", w.nb.name, n+1)
	w.overviewOn(a, false)
	if a.shows(w) {
		a.MarkFlash(w.pages())
	}
}

// drawOverview draws the pages small, in clip.
func (w *noteWin) drawOverview(a *bro, clip image.Rectangle) {
	c := a.C
	clip = clip.Intersect(w.paperArea())
	c.Fill(clip, kobo.White)
	tiles, _ := w.ovLayout()
	scale := w.scale
	defer func() { w.scale = scale }()
	for i, t := range tiles {
		n := w.ovFirst + i
		if n > len(w.nb.pages) || !t.Inset(-noteThumbGap).Overlaps(clip) {
			continue
		}
		lab := image.Rect(t.Min.X, t.Max.Y, t.Max.X, t.Max.Y+noteThumbLab)
		if n == len(w.nb.pages) { // the tile for a new one
			for _, s := range frameStrips(t, 2) {
				c.Fill(s.Intersect(clip), kobo.Mid)
			}
			c.Text(a.F.Small, t, "+ new page", kobo.Dark, kobo.AlignCenter)
			continue
		}
		w.scale = float64(t.Dx()) / float64(w.pageSize(n).X)
		w.drawPage(a, n, t, t.Intersect(clip), false)
		frame, ink := 1, uint8(kobo.Mid)
		if n == w.ovSel {
			frame, ink = 5, kobo.Black
		}
		for _, s := range frameStrips(t.Inset(-frame), frame) {
			c.Fill(s.Intersect(clip), ink)
		}
		num := fmt.Sprint(n + 1)
		if n == w.ovSel {
			num = fmt.Sprintf("page %d: tap again to open", n+1)
		}
		c.Text(a.F.Small, lab, num, kobo.Black, kobo.AlignCenter)
	}
}

// ovTap takes a tap on the pages small: one chosen, or opened if it
// was, or a new one at the end.
func (w *noteWin) ovTap(a *bro, p image.Point) {
	tiles, _ := w.ovLayout()
	for i, t := range tiles {
		n := w.ovFirst + i
		if n > len(w.nb.pages) || !p.In(t.Inset(-noteThumbGap/2).Union(t.Add(image.Pt(0, noteThumbLab)))) {
			continue
		}
		switch {
		case n == len(w.nb.pages):
			w.ovSel = n - 1
			w.pageAct(a, "new after")
		case n == w.ovSel:
			w.openPage(a, n)
		default:
			w.ovSel = n
			w.show(a)
		}
		return
	}
}

// ovOff is whether an overview's button can't do anything now.
func (w *noteWin) ovOff(id string) bool {
	tiles, _ := w.ovLayout()
	switch id {
	case "clear":
		return w.ovSel >= len(w.nb.pages) || len(w.nb.pages[w.ovSel]) == 0
	case "earlier":
		return w.ovSel == 0
	case "later":
		return w.ovSel >= len(w.nb.pages)-1
	case "prev":
		return w.ovFirst == 0
	case "next":
		return w.ovFirst+len(tiles) > len(w.nb.pages)
	}
	return false
}

// ovTurn shows the screenful of pages before or after.
func (w *noteWin) ovTurn(a *bro, by int) {
	tiles, _ := w.ovLayout()
	per := max(1, len(tiles))
	first := w.ovFirst + by*per
	if first < 0 || first > len(w.nb.pages) {
		return
	}
	w.ovFirst = first
	w.ovSel = max(first, min(w.ovSel, first+per-1, len(w.nb.pages)-1))
	w.show(a)
	if a.shows(w) {
		a.MarkFlash(w.paperArea())
	}
}

// ovKey moves the choice by keys: arrows, Enter opens it.
func (w *noteWin) ovKey(a *bro, code uint16) bool {
	_, cols := w.ovLayout()
	by := map[uint16]int{keyLeft: -1, keyRight: 1, keyUp: -cols, keyDown: cols}
	switch {
	case code == kobo.KeyEnter:
		w.openPage(a, w.ovSel)
	case code == keyDelete:
		w.pageAct(a, "delete")
	case by[code] != 0:
		w.ovSel = max(0, min(len(w.nb.pages)-1, w.ovSel+by[code]))
		w.ovShow()
		w.show(a)
	case code == keyPgUp || code == keyPgDn:
		w.ovTurn(a, map[bool]int{true: -1, false: 1}[code == keyPgUp])
	default:
		return false
	}
	return true
}

// pageAct does what an overview's button says, to the page chosen.
func (w *noteWin) pageAct(a *bro, id string) {
	if id == "open" {
		w.openPage(a, w.ovSel)
		return
	}
	if !w.pageEdit(id) {
		return
	}
	w.ovShow()
	w.place(a, w.p) // the counter
	w.show(a)
}

// pageEdit changes the pages as id says, the page chosen, as one change
// to undo; false if it can't.
func (w *noteWin) pageEdit(id string) bool {
	nb, n := w.nb, w.ovSel
	nb.padMeta()
	var ops []noteOp
	switch id {
	case "new before":
		ops = []noteOp{{kind: pageIns, page: n, meta: defaultMeta()}}
	case "new after":
		ops, w.ovSel = []noteOp{{kind: pageIns, page: n + 1, meta: defaultMeta()}}, n+1
	case "copy page":
		var copies []*noteStroke
		for _, s := range nb.pages[n] {
			copies = append(copies, s.moved(image.Point{}))
		}
		ops, w.ovSel = []noteOp{{kind: pageIns, page: n + 1, add: copies, meta: nb.meta[n]}}, n+1
	case "clear":
		if len(nb.pages[n]) == 0 {
			return false
		}
		ops = []noteOp{{page: n, del: slices.Clone(nb.pages[n])}}
	case "delete":
		ops = []noteOp{nb.delPageOp(n)}
		if len(nb.pages) == 1 { // the last: cleared, not gone
			if len(nb.pages[0]) == 0 {
				return false
			}
			ops = []noteOp{{page: 0, del: slices.Clone(nb.pages[0])}}
		}
	case "earlier", "later":
		to := n - 1
		if id == "later" {
			to = n + 1
		}
		if to < 0 || to >= len(nb.pages) {
			return false
		}
		del := nb.delPageOp(n)
		ops, w.ovSel = []noteOp{del, {kind: pageIns, page: to, add: del.add, meta: del.meta}}, to
	default:
		return false
	}
	for _, op := range ops {
		nb.apply(op, false)
	}
	w.edited(ops...)
	log.Printf("notebook %s: page %d: %s; %d pages", nb.name, n+1, id, len(nb.pages))
	return true
}
