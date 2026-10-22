package wm

import (
	"errors"
	"slices"
	"strconv"
	"strings"
)

// Move moves n in d: i3's tree_move. Within a container along d it swaps
// places with the one next to it, or goes into it if that's a container;
// at the container's edge it goes out, next to it in the first container
// above along d. If nothing above goes along d, the workspace turns to,
// with what was on it in a container. A floating window moves px.
func (t *Tree) Move(n *Node, d Dir, px int) {
	if n.Type != Con {
		return
	}
	if floating(n) {
		t.MoveFloating(n, d, px)
		return
	}
	ws := workspaceOf(n)
	if n.Fullscreen || n.Parent == ws && len(ws.Nodes) == 1 {
		return // i3 would take it to the next screen
	}
	o := d.horizontal()
	same := parentAlong(n, o)
	for {
		if same == nil {
			t.encapsulate(ws, ws.Layout)
			ws.Layout = SplitV
			if o {
				ws.Layout = SplitH
			}
			same = parentAlong(n, o)
		}
		if same != n.Parent {
			break
		}
		p := n.Parent
		i := slices.Index(p.Nodes, n)
		if j := i + step(d); j >= 0 && j < len(p.Nodes) {
			swap := p.Nodes[j]
			if !swap.leaf() {
				t.moveInto(n, swap, d)
				return
			}
			p.Nodes[i], p.Nodes[j] = swap, n
			return
		}
		if p == ws {
			return // the edge: i3 would take it to the next screen
		}
		if same = parentAlong(p, o); same != nil {
			break
		}
	}
	above := n
	for above.Parent != same {
		above = above.Parent
	}
	var next *Node
	if j := slices.Index(same.Nodes, above) + step(d); j >= 0 && j < len(same.Nodes) {
		next = same.Nodes[j]
	}
	switch {
	case next != nil && !next.leaf():
		t.moveInto(n, next, d)
		return
	case next == nil && n.Parent.Parent == ws && len(n.Parent.Nodes) == 1:
		return // alone at the workspace's edge
	}
	t.insert(n, above, d.back())
}

// moveInto puts n into the container c, next to the window in it
// nearest to where n comes from.
func (t *Tree) moveInto(n, c *Node, d Dir) {
	target := descendDir(c, d)
	before := target.Parent.Layout.horizontal() == d.horizontal() && !d.back()
	t.insert(n, target, before)
}

// insert moves n to target's container, before or after it.
func (t *Tree) insert(n, target *Node, before bool) {
	if t.within(target, n) {
		return
	}
	ws := workspaceOf(n)
	old := n.Parent
	detach(n)
	p := target.Parent
	i := slices.Index(p.Nodes, target)
	if !before {
		i++
	}
	attach(p, n, i)
	t.tidy(old)
	flatten(ws)
	if t.within(t.focused, n) {
		t.focus(t.focused)
	}
}

// parentAlong is the first container above n laid out along o
// (horizontal or not), up to its workspace; nil if there's none, or n
// floats.
func parentAlong(n *Node, horizontal bool) *Node {
	p := n.Parent
	for p != nil && p.Type != FloatingCon && (p.Type == Con || p.Type == Workspace) {
		if p.Layout.horizontal() == horizontal {
			return p
		}
		p = p.Parent
	}
	return nil
}

// MoveToWorkspace moves n to the workspace named name (made if need be):
// next to what's focused there, if it's tiled. A workspace moves all of
// its tiled windows, in a container. The focus stays on the workspace
// shown.
func (t *Tree) MoveToWorkspace(n *Node, name string) {
	if name == "" || name == t.scratch.Name {
		return
	}
	if n.Type == Workspace {
		if len(n.Nodes) == 0 {
			return
		}
		n = t.encapsulate(n, n.Layout)
	}
	if n.Type != Con {
		return
	}
	if f := floatOf(n); f != nil {
		n = f
	}
	if workspaceOf(n).Name == name {
		return
	}
	t.toWorkspace(n, t.workspace(name))
}

// WorkspaceByNum is the first workspace numbered num, if any.
func (t *Tree) WorkspaceByNum(num int) *Node {
	for _, w := range t.content.Nodes {
		if w.Num == num {
			return w
		}
	}
	return nil
}

// RenameWorkspace names the workspace named from (the one shown if "")
// to.
func (t *Tree) RenameWorkspace(from, to string) error {
	w := t.Current()
	if from != "" {
		w = nil
		for _, o := range t.content.Nodes {
			if o.Name == from {
				w = o
			}
		}
		if w == nil {
			return errors.New("no workspace " + strconv.Quote(from))
		}
	}
	if to == "" || to == t.scratch.Name || strings.HasPrefix(to, "__") {
		return errors.New("can't name a workspace " + strconv.Quote(to))
	}
	for _, o := range t.content.Nodes {
		if o.Name == to && o != w {
			return errors.New("there's a workspace " + strconv.Quote(to) + " already")
		}
	}
	if t.prevWS == w.Name {
		t.prevWS = to
	}
	// out and back in, so it's in order of its number
	i := slices.Index(t.content.Nodes, w)
	t.content.Nodes = slices.Delete(t.content.Nodes, i, i+1)
	w.Name, w.Num = to, -1
	if num, err := strconv.Atoi(to); err == nil && num >= 0 {
		w.Num = num
	}
	i = slices.IndexFunc(t.content.Nodes, func(o *Node) bool {
		return w.Num >= 0 && (o.Num < 0 || o.Num > w.Num)
	})
	if i < 0 {
		i = len(t.content.Nodes)
	}
	t.content.Nodes = slices.Insert(t.content.Nodes, i, w)
	return nil
}

// Swap swaps a and b, two windows or containers neither in the other:
// their places, shares, floating or not, and full screen.
func (t *Tree) Swap(a, b *Node) {
	if a == b || a.Type != Con || b.Type != Con || t.within(a, b) || t.within(b, a) {
		return
	}
	pa, pb := a.Parent, b.Parent
	slot := func(p, n *Node) (*[]*Node, int) {
		if i := slices.Index(p.Nodes, n); i >= 0 {
			return &p.Nodes, i
		}
		return &p.Floating, slices.Index(p.Floating, n)
	}
	la, ia := slot(pa, a)
	lb, ib := slot(pb, b)
	(*la)[ia], (*lb)[ib] = b, a
	fa, fb := slices.Index(pa.Focus, a), slices.Index(pb.Focus, b)
	pa.Focus[fa], pb.Focus[fb] = b, a
	a.Parent, b.Parent = pb, pa
	a.Percent, b.Percent = b.Percent, a.Percent
	a.Fullscreen, b.Fullscreen = b.Fullscreen, a.Fullscreen
	if t.focused == a || t.focused == b {
		t.focus(t.focused)
	}
}
