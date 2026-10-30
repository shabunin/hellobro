package main

// The workspaces at a glance, for fingers: a tile for each, with its
// windows drawn small as they're laid out, and one more for a new
// workspace (a launcher in it, so it stays: an empty one goes as it's
// left). A tap goes there; Esc or a tap outside closes. A tap on the
// bar's empty part, the menu's Workspaces or Meta+O puts it up.

import (
	"fmt"
	"image"
	"strconv"

	"hellobro/kobo"
	"hellobro/wm"
)

const wsHeadH = 44 // a workspace tile's header: its name, how many windows

// openWorkspaces puts the workspaces up, the one shown lit.
func (a *bro) openWorkspaces() {
	t := a.wm.tree
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
		es = append(es, pickEntry{kind: "workspace", title: "Workspace " + ws.Name, detail: count,
			paint: func(r image.Rectangle) { a.drawWorkspaceTile(r, ws, count, cur) },
			do:    func() { a.wmRun("workspace " + strconv.Quote(ws.Name)) }})
	}
	es = append(es, pickEntry{kind: "workspace", title: "New workspace", detail: "+",
		paint: func(r image.Rectangle) { a.drawNewTile(r) }, do: a.newWorkspace})
	p := &picker{title: "Workspaces", hint: "tap one to go there, or + for a new one", all: es, tiles: 1, wide: true}
	a.showPicker(p)
	p.sel = sel
	a.drawPicker()
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
