package wm

import "image"

// Render places what shows of the workspace shown: each node's Rect, and
// for a window its WinRect and, if it has one, its title bar or tab
// (Deco). Visible says what shows. Nothing of other workspaces does.
//
// A window in a split has a title bar (unless its border isn't normal);
// a tabbed container has a row of tabs, one per child, and shows the
// child focused last below them; a stacked one a title bar per child,
// one above another. Full screen, a window takes the whole screen.
func (t *Tree) Render() {
	var reset func(*Node)
	reset = func(n *Node) {
		n.Deco, n.WinRect, n.Visible = image.Rectangle{}, image.Rectangle{}, false
		if n.Type == Con || n.Type == FloatingCon {
			n.Rect = image.Rectangle{}
		}
		for _, c := range n.Nodes {
			reset(c)
		}
		for _, c := range n.Floating {
			reset(c)
		}
	}
	for _, w := range t.content.Nodes {
		reset(w)
		w.Rect = image.Rectangle{}
	}
	reset(t.scratch)
	t.LayOut(t.Current())
}

// LayOut places ws's windows as they'd be with ws shown, and says which
// is fullscreen there (nil: none). For a workspace not shown that lasts
// till the next Render.
func (t *Tree) LayOut(ws *Node) (fullscreen *Node) {
	ws.Rect, ws.Visible = t.content.Rect, true
	if fs := t.fullscreenOn(ws); fs != nil {
		t.place(fs, ws.Rect, false)
		return fs
	}
	t.layout(ws, ws.Rect)
	for _, f := range ws.Floating {
		f.Rect, f.Visible = keepIn(f.FloatRect, ws.Rect), true
		for _, c := range f.Nodes {
			t.place(c, f.Rect, true)
		}
	}
	return nil
}

// place puts n in r, with a title bar if it's a window and bar says so.
func (t *Tree) place(n *Node, r image.Rectangle, bar bool) {
	n.Rect, n.Visible = r, true
	switch {
	case !n.leaf():
		t.layout(n, r)
	case bar && n.Border == BorderNormal && t.TitleH > 0:
		n.Deco = image.Rect(r.Min.X, r.Min.Y, r.Max.X, min(r.Min.Y+t.TitleH, r.Max.Y))
		n.WinRect = image.Rect(r.Min.X, n.Deco.Max.Y, r.Max.X, r.Max.Y)
	default:
		n.WinRect = r
	}
}

// layout places n's children in r.
func (t *Tree) layout(n *Node, r image.Rectangle) {
	k := len(n.Nodes)
	if k == 0 {
		return
	}
	switch n.Layout {
	case SplitH, SplitV:
		total := r.Dx()
		if n.Layout == SplitV {
			total = r.Dy()
		}
		at := 0
		for i, size := range sizes(n.Nodes, total) {
			c := n.Nodes[i]
			cr := image.Rect(r.Min.X+at, r.Min.Y, r.Min.X+at+size, r.Max.Y)
			if n.Layout == SplitV {
				cr = image.Rect(r.Min.X, r.Min.Y+at, r.Max.X, r.Min.Y+at+size)
			}
			at += size
			t.place(c, cr, true)
		}
	case Tabbed, Stacked:
		rows := 1
		if n.Layout == Stacked {
			rows = k
		}
		top := min(r.Min.Y+rows*t.TitleH, r.Max.Y)
		body := image.Rect(r.Min.X, top, r.Max.X, r.Max.Y)
		even := make([]int, k)
		for i := range even {
			even[i] = r.Dx() / k
			if i < r.Dx()%k {
				even[i]++
			}
		}
		x := r.Min.X
		for i, c := range n.Nodes {
			if n.Layout == Tabbed {
				c.Deco = image.Rect(x, r.Min.Y, x+even[i], top)
				x += even[i]
			} else {
				y := r.Min.Y + i*t.TitleH
				c.Deco = image.Rect(r.Min.X, min(y, r.Max.Y), r.Max.X, min(y+t.TitleH, r.Max.Y))
			}
			c.Rect = body
		}
		if f := focusedTiled(n); f != nil {
			deco := f.Deco
			t.place(f, body, false)
			f.Deco = deco
		}
	}
}

// sizes shares total among ns by their Percent, in whole pixels that add
// up to it.
func sizes(ns []*Node, total int) []int {
	sum := 0.0
	for _, c := range ns {
		sum += c.Percent
	}
	out := make([]int, len(ns))
	used := 0
	for i, c := range ns {
		p := 1 / float64(len(ns))
		if sum > 0 {
			p = c.Percent / sum
		}
		out[i] = int(p * float64(total))
		used += out[i]
	}
	for i := 0; used < total; i = (i + 1) % len(out) {
		out[i]++
		used++
	}
	return out
}

// Shown is the windows that show, in the order to draw them: the tiled
// ones, then the floating ones from the bottom up.
func (t *Tree) Shown() []*Node {
	var out []*Node
	var walk func(*Node)
	walk = func(n *Node) {
		if !n.Visible {
			return
		}
		if n.leaf() {
			out = append(out, n)
		}
		for _, c := range n.Nodes {
			walk(c)
		}
	}
	ws := t.Current()
	if fs := t.fullscreenOn(ws); fs != nil {
		walk(fs)
		return out
	}
	walk(ws)
	for _, f := range ws.Floating {
		walk(f)
	}
	return out
}

// Decorated is the nodes whose title bars or tabs show (their Deco).
func (t *Tree) Decorated() []*Node {
	return t.find(t.Current(), func(n *Node) bool { return !n.Deco.Empty() })
}
