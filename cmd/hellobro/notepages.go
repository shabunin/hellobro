package main

// The notebook's pages, all of them: tapping the page counter (or O)
// shows them small, a screenful at a time (the arrows, or a finger
// swiped across, for more), and a tile after the last for a new one.
// Tap a page to choose it, again to open it. The strip then has: open,
// new before, new after, copy page, clear page (asking first), delete,
// earlier and later (moving it). Each page has a × too, in its corner:
// tapped, it asks, and tapped again soon it deletes the page, as
// workspaces' tiles close. Each is a change to undo, as strokes are.

import (
	"fmt"
	"image"
	"log"
	"slices"
	"time"

	"hellobro/ink"
	"hellobro/ink/text"
	"hellobro/kobo"
)

const (
	ownNoteOv    = 10  // a contact on the pages, small
	noteThumbW   = 260 // px a page is at least, small
	noteThumbGap = 24
	noteThumbLab = 36 // px under it for its number
	noteThumbX   = 52 // the ×'s box, in a page's corner
)

// What a noteOp does: to a page's strokes, or a page put in or taken out.
const (
	pageStrokes = iota
	pageIns
	pageDel
)

// padMeta gives each page what it has besides its strokes, so pages
// can be put in and taken out with it.
func (nb *notebook) padMeta() {
	for len(nb.meta) < len(nb.pages) {
		nb.meta = append(nb.meta, nb.newMeta())
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
		nb.pages, nb.meta = [][]*noteStroke{nil}, []notePage{nb.newMeta()}
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
	w.endTyping(a)
	w.overview, w.optsOf, w.sel, w.mark = on, "", nil, nil
	if on {
		w.ovSel = min(w.cur, len(w.nb.pages)-1)
		w.ovShow()
	}
	w.place(a, w.p) // the strip has other buttons
	w.show(a)
}

// openPage shows page n, from the overview.
func (w *noteWin) openPage(a *bro, n int) {
	w.cur, w.scroll = n, 0
	log.Printf("notebook %s: page %d opened", w.nb.name, n+1)
	w.overviewOn(a, false)
	if a.shows(w) {
		a.MarkFlash(w.pages())
	}
}

// drawOverview paints the pages small in ink, all of them that show
// (clip's the caller's to mark): each page drawn as the notebook draws
// it into an image of its own, then laid out with its frame, its ×
// and its number; the chosen framed thick; a tile for a new one.
func (w *noteWin) drawOverview(a *bro, clip image.Rectangle) {
	area := w.paperArea()
	if area.Empty() {
		return
	}
	tiles, _ := w.ovLayout()
	thumbs := make([]*image.Gray, len(tiles))
	screen, scale := a.C.Gray, w.scale
	for i, t := range tiles {
		n := w.ovFirst + i
		if n >= len(w.nb.pages) {
			break
		}
		img := image.NewGray(t) // where it is on the screen, as drawPage draws
		a.C.Gray = img
		a.C.Fill(t, kobo.White)
		w.scale = float64(t.Dx()) / float64(w.pageSize(n).X)
		w.drawPage(a, n, t, t, false)
		thumbs[i] = img
	}
	a.C.Gray, w.scale = screen, scale
	a.inkPaint(&w.ovPane, area, func(gtx ink.Ctx) ink.Dims {
		th := a.theme()
		size := gtx.Constraints.Max
		ink.Fill(gtx.Ops, image.Rectangle{Max: size}, th.Bg)
		small := th.Face(gtx, th.TextSize*0.85, text.Regular)
		label := func(r image.Rectangle, s string, g uint8) {
			s = text.Fit(small, s, r.Dx())
			ink.TextAt(gtx.Ops, small, r.Min.X+(r.Dx()-text.Width(small, s))/2, r.Min.Y+(r.Dy()-text.Height(small))/2+text.Ascent(small), s, g)
		}
		for i, at := range tiles {
			n, t := w.ovFirst+i, at.Sub(area.Min)
			if n > len(w.nb.pages) {
				break
			}
			if n == len(w.nb.pages) { // the tile for a new one
				ink.Frame(gtx.Ops, t, 2, ink.Mid)
				label(t, "+ new page", ink.Dark)
				continue
			}
			ink.Image(gtx.Ops, t, thumbs[i], thumbs[i].Rect.Min)
			frame, gray := 1, uint8(ink.Mid)
			if n == w.ovSel {
				frame, gray = 5, th.Fg
			}
			ink.Frame(gtx.Ops, t.Inset(-frame), frame, gray)
			num := fmt.Sprint(n + 1)
			if n == w.ovSel {
				num = fmt.Sprintf("page %d: tap again to open", n+1)
			}
			if w.ovDelOK(n) {
				x := ovCloseBox(t)
				fg, bg := th.Bg, th.Fg
				ink.Fill(gtx.Ops, x, th.Fg)
				if w.ovDelArmed(n) { // lit: again deletes
					ink.Fill(gtx.Ops, x.Inset(3), th.Bg)
					fg, bg = th.Fg, th.Bg
					num = "tap × again: delete it"
					if len(w.nb.pages) == 1 {
						num = "tap × again: clear it"
					}
				}
				ink.Draw(gtx.Ops, x, func(dst *image.Gray, r image.Rectangle) {
					drawIcon(kobo.Canvas{Gray: dst}, iconCross, r, fg, bg)
				})
			}
			label(image.Rect(t.Min.X, t.Max.Y, t.Max.X, t.Max.Y+noteThumbLab), num, th.Fg)
		}
		return ink.Dims{Size: size}
	})
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
		armed := w.ovDelArmed(n)
		w.ovDelAt = time.Time{}
		switch {
		case n < len(w.nb.pages) && w.ovDelOK(n) && p.In(ovCloseTap(t)):
			w.ovSel = n
			if armed {
				w.pageAct(a, "delete")
				break
			}
			w.ovDel, w.ovDelAt = n, time.Now()
			at := w.ovDelAt
			time.AfterFunc(noteClearAsk, func() {
				a.Do(func() {
					if w.ovDelAt == at && w.overview {
						w.ovDelAt = time.Time{}
						w.show(a)
					}
				})
			})
			w.show(a)
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

// iconCross is a page's ×.
var iconCross = []iconPart{iconLine(1.6, 6, 6, 18, 18), iconLine(1.6, 18, 6, 6, 18)}

// ovCloseBox is where a page's × is drawn, in its corner at t's top
// right; ovCloseTap where a tap is on it, a finger's more.
func ovCloseBox(t image.Rectangle) image.Rectangle {
	return image.Rect(t.Max.X-noteThumbX, t.Min.Y, t.Max.X, t.Min.Y+noteThumbX)
}

func ovCloseTap(t image.Rectangle) image.Rectangle {
	return image.Rect(t.Max.X-noteThumbX-24, t.Min.Y-noteThumbGap/2, t.Max.X+noteThumbGap/2, t.Min.Y+noteThumbX+24)
}

// ovDelOK is whether page n has a ×: all have but a lone blank one,
// which deleting wouldn't change.
func (w *noteWin) ovDelOK(n int) bool {
	return n < len(w.nb.pages) && (len(w.nb.pages) > 1 || len(w.nb.pages[n]) > 0)
}

// ovDelArmed is whether page n's × was tapped once, lately.
func (w *noteWin) ovDelArmed(n int) bool {
	return w.ovDel == n && time.Since(w.ovDelAt) < noteClearAsk
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
	// The last screenful may hold only the new page's tile: the choice
	// stays on the last page then, before it.
	w.ovSel = min(len(w.nb.pages)-1, max(first, min(w.ovSel, first+per-1)))
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
	w.ovDelAt = time.Time{}
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
	if n < 0 || n >= len(nb.pages) {
		return false
	}
	nb.padMeta()
	var ops []noteOp
	switch id {
	case "new before":
		ops = []noteOp{{kind: pageIns, page: n, meta: nb.newMeta()}}
	case "new after":
		ops, w.ovSel = []noteOp{{kind: pageIns, page: n + 1, meta: nb.newMeta()}}, n+1
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
