package main

// The workspaces at a glance, for fingers: a tile for each, with its
// windows drawn small as they're laid out, and one more for a new
// workspace (a launcher in it, so it stays: an empty one goes as it's
// left). A tap goes there; Esc or a tap outside closes. A tap on the
// bar's empty part, two fingers swiped down from the screen's top, the
// menu's Workspaces or Meta+O puts it up.
//
// What's in the windows shows too, as it is now, and again every few
// seconds while the tiles are up. The tiles come up at once, with the
// windows as they were drawn small last (the workspace shown's from the
// screen), and what changed is drawn again as soon as the panel's done. A terminal's cells are there whether
// it shows or not, so it's drawn from them (placed, it would be resized,
// and what runs in it). The others are placed where they'd be on their
// workspace, drawn on a canvas of their own and put back: a book has
// its pages laid out for that size already.

import (
	"fmt"
	"hash/fnv"
	"image"
	"log"
	"strconv"
	"time"

	"hellobro/kobo"
	"hellobro/wm"
)

const (
	wsHeadH = 44  // a workspace tile's header: its name, how many windows
	wsTop   = 100 // px down from the screen's top where two fingers swiped down put the tiles up
)

// wsSwipe takes two fingers going down from the screen's top: the
// tiles up, true once they are.
func (a *bro) wsSwipe(f, g *kobo.Gesture) bool {
	top := a.C.Bounds().Min.Y + wsTop
	if a.Locked() || a.picker != nil || f.Start.Y >= top || g.Start.Y >= top {
		return false
	}
	down := func(g *kobo.Gesture) bool { // far enough, and mostly down
		d := g.Last.Sub(g.Start)
		return d.Y >= pairSwipe && d.Y >= 2*kobo.Abs(d.X)
	}
	if !down(f) || !down(g) {
		return false
	}
	log.Print("two-finger swipe: the workspaces")
	a.openWorkspaces()
	return true
}

// openWorkspaces puts the workspaces up, the one shown lit.
func (a *bro) openWorkspaces() {
	t := a.wm.tree
	for w := range a.wm.thumbs {
		if a.nodeOf(w) == nil { // closed
			delete(a.wm.thumbs, w)
		}
	}
	start := time.Now()
	if !a.overlay() {
		a.keepShown()
	}
	var es []pickEntry
	sel := 0
	for _, ws := range t.Workspaces() {
		n := len(t.Find(func(o *wm.Node) bool { return o.Leaf() && wm.WorkspaceOf(o) == ws }))
		cur := ws == t.Current()
		if cur {
			sel = len(es)
		}
		count := fmt.Sprintf("%d windows", n)
		if n == 1 {
			count = "a window"
		}
		var drawn uint64
		es = append(es, pickEntry{kind: "workspace", title: "Workspace " + ws.Name, detail: count,
			paint: func(r image.Rectangle) {
				drawn = a.wsSum(ws)
				a.drawWorkspaceTile(r, ws, count, cur)
			},
			changed: func() bool { return a.wsSum(ws) != drawn },
			do:      func() { a.wmRun("workspace " + strconv.Quote(ws.Name)) }})
	}
	es = append(es, pickEntry{kind: "workspace", title: "New workspace", detail: "+",
		paint: func(r image.Rectangle) { a.drawNewTile(r) }, do: a.newWorkspace})
	p := &picker{all: es, // no title nor hint: the tiles have the whole screen; the keys are in the docs
		tiles: 1, wide: true, keysOnly: true, refresh: a.drawThumbs}
	p.key = func(ch string) {
		switch {
		case ch >= "0" && ch <= "9": // as Meta+1…0
			if ch == "0" {
				ch = "10"
			}
			a.closePicker()
			a.wmRun("workspace number " + ch)
		case ch == "n" || ch == "+" || ch == "=":
			a.closePicker()
			a.newWorkspace()
		}
	}
	a.showPicker(p)
	p.sel = sel
	a.drawPicker()
	p.lookedAt = time.Time{} // the windows drawn again once that's on the panel
	log.Printf("workspaces: up in %v", time.Since(start).Round(time.Millisecond))
}

// newWorkspace goes to the first workspace number not taken, with a
// launcher in it.
func (a *bro) newWorkspace() {
	var names []string
	for _, ws := range a.wm.tree.Workspaces() {
		names = append(names, ws.Name)
	}
	a.wmRun("workspace number " + freeWorkspace(names))
	a.openLauncher()
}

// freeWorkspace is the smallest number from 1 that no name is.
func freeWorkspace(names []string) string {
	taken := map[string]bool{}
	for _, s := range names {
		taken[s] = true
	}
	n := 1
	for taken[strconv.Itoa(n)] {
		n++
	}
	return strconv.Itoa(n)
}

// drawWorkspaceTile draws ws in r: its name and count over its windows,
// small, where they are; the one shown in a thicker frame.
func (a *bro) drawWorkspaceTile(r image.Rectangle, ws *wm.Node, count string, cur bool) {
	c := a.C
	c.Fill(r, kobo.White)
	head := image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+wsHeadH)
	c.Fill(head, kobo.Black)
	c.Text(a.F.Panel, head.Inset(8), ws.Name, kobo.White, kobo.AlignLeft)
	c.Text(a.F.Small, head.Inset(8), count, kobo.Light, kobo.AlignRight)
	body := image.Rect(r.Min.X, head.Max.Y, r.Max.X, r.Max.Y)
	k := float64(body.Dx()) / float64(max(1, a.wm.area.Dx()))
	a.miniSplit(ws, body)
	for _, f := range ws.Floating { // over the tiled ones, where they float
		fr := f.FloatRect.Sub(a.wm.area.Min)
		box := image.Rect(body.Min.X+int(float64(fr.Min.X)*k), body.Min.Y+int(float64(fr.Min.Y)*k),
			body.Min.X+int(float64(fr.Max.X)*k), body.Min.Y+int(float64(fr.Max.Y)*k)).Intersect(body)
		c.Fill(box, kobo.White)
		a.miniSplit(f, box)
	}
	if len(ws.Nodes) == 0 && len(ws.Floating) == 0 {
		c.Text(a.F.Small, body, "empty", kobo.Mid, kobo.AlignCenter)
	}
	w := 2
	if cur {
		w = 6
	}
	c.Frame(r, w, kobo.Black)
}

// miniSplit lays n's children in r as the tree would: side by side or
// one over another by their shares, or (tabbed, stacked) the one in
// front; a window is a box with its title.
func (a *bro) miniSplit(n *wm.Node, r image.Rectangle) {
	c := a.C
	if n.Leaf() {
		box := r.Inset(3)
		c.Frame(box, 2, kobo.Black)
		bar := image.Rect(box.Min.X, box.Min.Y, box.Max.X, min(box.Max.Y, box.Min.Y+28))
		if in := image.Rect(box.Min.X+2, bar.Max.Y, box.Max.X-2, box.Max.Y-2); !in.Empty() {
			if th := a.thumbOf(a.wm.wins[n]); th != nil {
				scaleGray(c.Gray, in.Intersect(c.Rect), th)
			}
		}
		c.Fill(bar, kobo.Mid)
		c.Text(a.F.Small, bar.Inset(2), n.Window.Title(), kobo.White, kobo.AlignLeft)
		return
	}
	kids := n.Nodes
	if len(kids) == 0 {
		return
	}
	switch n.Layout {
	case wm.Tabbed, wm.Stacked:
		front := kids[0]
		if len(n.Focus) > 0 && n.Focus[0].Parent == n {
			front = n.Focus[0] // the one focused last
		}
		a.miniSplit(front, r)
		return
	}
	total := r.Dx()
	if n.Layout == wm.SplitV {
		total = r.Dy()
	}
	sum := 0.0
	for _, ch := range kids {
		sum += ch.Percent
	}
	at := 0
	for i, ch := range kids {
		size := int(float64(total) * ch.Percent / max(sum, 0.0001))
		if i == len(kids)-1 {
			size = total - at
		}
		cr := image.Rect(r.Min.X+at, r.Min.Y, r.Min.X+at+size, r.Max.Y)
		if n.Layout == wm.SplitV {
			cr = image.Rect(r.Min.X, r.Min.Y+at, r.Max.X, r.Min.Y+at+size)
		}
		at += size
		a.miniSplit(ch, cr)
	}
}

// drawNewTile is the tile for a new workspace: a big +.
func (a *bro) drawNewTile(r image.Rectangle) {
	c := a.C
	c.Fill(r, kobo.White)
	c.Frame(r, 2, kobo.Black)
	m := image.Pt((r.Min.X+r.Max.X)/2, (r.Min.Y+r.Max.Y)/2-20)
	s := min(r.Dx(), r.Dy()) / 6
	c.Fill(image.Rect(m.X-s, m.Y-4, m.X+s, m.Y+4), kobo.Black)
	c.Fill(image.Rect(m.X-4, m.Y-s, m.X+4, m.Y+s), kobo.Black)
	c.Text(a.F.Body, image.Rect(r.Min.X, m.Y+s+10, r.Max.X, m.Y+s+50), "new workspace", kobo.Black, kobo.AlignCenter)
}

// wsThumb is a window drawn small; sum, a terminal's: its cells' when
// drawn.
type wsThumb struct {
	img *image.Gray
	sum uint64
}

// thumbScale is how much smaller a window's kept: about a tile's share.
const thumbScale = 3 // shrink3's

// drawThumbs draws every window but the terminals (termThumb) small:
// each placed where it would be on its workspace and drawn on a canvas
// that isn't the screen's, then put back, the shown ones where they
// were, the others nowhere. Nothing reaches the panel meanwhile (Mark).
func (a *bro) drawThumbs() {
	if a.C.Gray == nil {
		return
	}
	if a.wm.thumbs == nil {
		a.wm.thumbs = map[window]*wsThumb{}
	}
	t := a.wm.tree
	start := time.Now()
	if a.wm.off == nil || a.wm.off.Rect != a.C.Rect { // kept: each window's part is cleared as it's drawn
		a.wm.off = kobo.Blank(a.C.Rect.Size())
	}
	off := a.wm.off
	screen := a.C.Gray
	a.C.Gray, a.wm.offscreen = off, true
	defer func() {
		a.C.Gray, a.wm.offscreen = screen, false
		t.Render() // the workspace shown's places, as they were
	}()
	cur := t.Current()
	took := map[string]time.Duration{}
	defer func() {
		if d := time.Since(start); d > 300*time.Millisecond {
			log.Printf("workspaces: the windows drawn small in %v: %v", d.Round(time.Millisecond), took)
		}
	}()
	for _, ws := range t.Workspaces() {
		fs := t.LayOut(ws)
		for _, n := range t.Find(func(o *wm.Node) bool { return o.Leaf() && o.Visible && wm.WorkspaceOf(o) == ws }) {
			w := a.wm.wins[n]
			if w == nil {
				continue
			}
			if tm, ok := w.(*termSession); ok && tm.connected() {
				continue
			}
			p := a.panelFor(n, w, fs)
			g, gui := w.(*guiWin)
			if gui && ws != cur {
				g.shown = true // not coming up for real: no scan, no sample
			}
			at := time.Now()
			a.C.Fill(p.Outer, kobo.White) // not what the one before left
			w.place(a, p)
			w.draw(a)
			if ws != cur {
				w.place(a, kobo.Panel{})
			}
			took[fmt.Sprintf("%T", w)] += time.Since(at).Round(time.Millisecond)
			a.keepThumb(w, off, p.Body)
		}
	}
}

// keepShown keeps the windows of the workspace shown small, as they are
// on the screen: nothing drawn, so the tiles come up at once.
func (a *bro) keepShown() {
	if a.C.Gray == nil {
		return
	}
	for _, n := range a.wm.tree.Shown() {
		if w := a.wm.wins[n]; w != nil {
			if t, ok := w.(*termSession); !ok || !t.connected() {
				a.keepThumb(w, a.C.Gray, w.panel(a).Body)
			}
		}
	}
}

// keepThumb keeps r of m as w's thumb.
func (a *bro) keepThumb(w window, m *image.Gray, r image.Rectangle) {
	r = r.Intersect(m.Rect)
	if r.Dx() < thumbScale || r.Dy() < thumbScale {
		return
	}
	img := shrink3(m.SubImage(r).(*image.Gray))
	h := fnv.New64a()
	h.Write(img.Pix)
	if a.wm.thumbs == nil {
		a.wm.thumbs = map[window]*wsThumb{}
	}
	a.wm.thumbs[w] = &wsThumb{img: img, sum: h.Sum64()}
}

// shrink3 is m a third of its size, each pixel the average of the 3×3
// under it: what scaleGray does, without its general case's work per
// pixel, which on the Kobo took a few hundred ms for a screenful.
func shrink3(m *image.Gray) *image.Gray {
	w, h := m.Rect.Dx()/3, m.Rect.Dy()/3 // what's left over is left out
	img := image.NewGray(image.Rect(0, 0, w, h))
	acc := make([]int, w)
	for y := range h {
		clear(acc)
		for sy := y * 3; sy < y*3+3; sy++ {
			row := m.Pix[m.PixOffset(m.Rect.Min.X, m.Rect.Min.Y+sy):][: w*3 : w*3]
			for x := range acc {
				acc[x] += int(row[x*3]) + int(row[x*3+1]) + int(row[x*3+2])
			}
		}
		out := img.Pix[y*img.Stride:][:w]
		for x, s := range acc {
			out[x] = uint8(s / 9)
		}
	}
	return img
}

// thumbOf is w small: a terminal as it is now, another as it was kept;
// nil, nothing to show.
func (a *bro) thumbOf(w window) *image.Gray {
	if t, ok := w.(*termSession); ok && t.connected() {
		return a.termThumb(t)
	}
	if th := a.wm.thumbs[w]; th != nil {
		return th.img
	}
	return nil
}

// wsSum is what ws's windows show, summed: another sum, another tile.
func (a *bro) wsSum(ws *wm.Node) uint64 {
	h := fnv.New64a()
	for _, n := range a.wm.tree.Find(func(o *wm.Node) bool { return o.Leaf() && wm.WorkspaceOf(o) == ws }) {
		w := a.wm.wins[n]
		if t, ok := w.(*termSession); ok && t.connected() {
			_, sum := a.termCells(t, false)
			fmt.Fprint(h, sum, ";")
		} else if th := a.wm.thumbs[w]; th != nil {
			fmt.Fprint(h, th.sum, ";")
		}
	}
	return h.Sum64()
}

// termCells are t's cells as the panel would show them (if keep), and
// their sum. Read under t.mu, as renderTerm does.
func (a *bro) termCells(t *termSession, keep bool) ([][]cellKey, uint64) {
	h := fnv.New64a()
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.emu == nil {
		return nil, 0
	}
	emu := t.emu.Emulator
	cur, showCursor := emu.CursorPosition(), !emu.CursorHidden()
	var rows [][]cellKey
	var k cellKey
	var b []byte
	for y := range t.size.Y {
		var row []cellKey
		if keep {
			row = make([]cellKey, t.size.X)
		}
		for x := range t.size.X {
			k.set(emu.CellAt(x, y), showCursor && cur.X == x && cur.Y == y)
			if k.img != 0 {
				k.place = t.gfx.places[k.img]
			}
			cursor := byte(0)
			if k.cursor {
				cursor = 1
			}
			b = append(append(b[:0], k.content...), 0, k.fg, k.bg, k.attrs, k.ul, cursor, byte(k.img), byte(k.img>>8), byte(k.ix), byte(k.iy))
			h.Write(b)
			if keep {
				row[x] = k
			}
		}
		rows = append(rows, row)
	}
	return rows, h.Sum64()
}

// termThumb is t drawn from its cells, small: drawn again only once
// they've changed.
func (a *bro) termThumb(t *termSession) *image.Gray {
	cells, sum := a.termCells(t, true)
	if len(cells) == 0 || len(cells[0]) == 0 {
		return nil
	}
	if th := a.wm.thumbs[t]; th != nil && th.sum == sum {
		return th.img
	}
	// The cells are drawn whole into a canvas of their own, as drawCell
	// does on the panel, then made small.
	f := a.face(t)
	full := kobo.Blank(image.Pt(len(cells[0])*f.cw, len(cells)*f.ch))
	pn := &termPane{grid: full.Rect}
	screen := a.C.Gray
	a.C.Gray = full
	for y, row := range cells {
		for x, k := range row {
			a.drawCell(pn, f, x, y, k)
		}
	}
	a.C.Gray = screen
	img := shrink3(full)
	if a.wm.thumbs == nil {
		a.wm.thumbs = map[window]*wsThumb{}
	}
	a.wm.thumbs[t] = &wsThumb{img: img, sum: sum}
	return img
}
