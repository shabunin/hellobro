package wm

import "slices"

// Open puts a new window next to the one focused (after it, in its
// container), or on the workspace if it's empty, and focuses it: i3's
// tree_open_con. With a floating window focused it goes where the focus
// was among the tiled ones. A window full screen gives way.
func (t *Tree) Open(w Window) *Node {
	n := t.node(Con)
	n.Window = w
	ws := t.Current()
	t.unFullscreen(ws)
	p, after := t.target(t.focused)
	if p == ws && ws.wsLayout && (ws.WSLayout == Stacked || ws.WSLayout == Tabbed) {
		// a stacked or tabbed workspace: the windows go in its one container
		if len(ws.Nodes) == 0 {
			c := t.node(Con)
			c.Layout, c.LastSplit = ws.WSLayout, ws.LastSplit
			attach(ws, c, -1)
		}
		p, after = ws.Nodes[0], focusedTiled(ws.Nodes[0])
	}
	attach(p, n, indexAfter(p, after))
	t.focus(n)
	return n
}

// target is where a window opened (or tiled) from f goes: into p, after
// its child after (nil: at the end).
func (t *Tree) target(f *Node) (p, after *Node) {
	ws := workspaceOf(f)
	if f.Type == Workspace || floating(f) {
		f = descendTiled(ws)
	}
	if f.Type == Workspace {
		return f, nil
	}
	return f.Parent, f
}

func indexAfter(p, c *Node) int {
	if i := slices.Index(p.Nodes, c); i >= 0 {
		return i + 1
	}
	return -1
}

// Remove takes n out, a window's leaf or a container with all in it, as
// when the windows closed. If the focus was in it, it goes to what was
// focused before it there.
func (t *Tree) Remove(n *Node) {
	if n.Type != Con || n.Parent == nil {
		return
	}
	had := t.within(t.focused, n)
	p := n.Parent
	if p.Type == FloatingCon {
		n, p = p, p.Parent
	}
	detach(n)
	left := t.tidy(p)
	if had {
		if left == nil {
			left = t.Current()
		}
		t.focus(descend(left))
	}
}

// within says whether n is c or in it.
func (t *Tree) within(n, c *Node) bool {
	for ; n != nil; n = n.Parent {
		if n == c {
			return true
		}
	}
	return false
}

// Split makes n's place split o (SplitH or SplitV): i3's tree_split. A
// window alone in a split container just turns it; otherwise it goes
// into a new container of its own, laid out o, for the next window to
// share. A workspace with a window or none turns; with more, they go in
// a container first.
func (t *Tree) Split(n *Node, o Layout) {
	if !o.split() || floating(n) {
		return
	}
	if n.Type == Workspace {
		if len(n.Nodes) < 2 {
			n.Layout, n.wsLayout = o, false
			return
		}
		n = t.encapsulate(n, n.Layout)
	}
	p := n.Parent
	if len(p.Nodes) == 1 && p.Layout.split() {
		p.Layout = o
		return
	}
	c := t.node(Con)
	c.Layout, c.LastSplit = o, o
	replace(n, c)
	n.Parent = nil
	attach(c, n, -1)
}

// SplitToggle splits across n's parent.
func (t *Tree) SplitToggle(n *Node) {
	p := n
	if n.Type != Workspace {
		p = n.Parent
	}
	if p.Layout.horizontal() {
		t.Split(n, SplitV)
	} else {
		t.Split(n, SplitH)
	}
}

// encapsulate moves all of ws's tiled windows into a new container laid
// out l, its only child, and returns that. It takes their place in the
// workspace's focus list, among the floating windows.
func (t *Tree) encapsulate(ws *Node, l Layout) *Node {
	c := t.node(Con)
	c.Layout, c.LastSplit = l, ws.LastSplit
	if l.split() {
		c.LastSplit = l
	}
	var focus []*Node
	for _, o := range ws.Focus {
		switch {
		case o.Type == FloatingCon:
			focus = append(focus, o)
		case len(c.Focus) == 0:
			focus = append(focus, c)
			fallthrough
		default:
			c.Focus = append(c.Focus, o)
		}
	}
	for _, o := range ws.Nodes {
		o.Parent = c
	}
	c.Nodes = ws.Nodes
	c.Parent, c.Percent = ws, 1
	ws.Nodes, ws.Focus = []*Node{c}, focus
	return c
}

// SetLayout lays out n's container (n's parent, or n if it's a
// workspace) as l: i3's con_set_layout. A workspace's windows go in a
// container laid out l; an empty workspace keeps it for the windows to
// come.
func (t *Tree) SetLayout(n *Node, l Layout) {
	if floating(n) {
		return
	}
	if n.Type != Workspace {
		n = n.Parent
	}
	if n.Layout.split() {
		n.LastSplit = n.Layout
	}
	if n.Type == Workspace {
		switch {
		case len(n.Nodes) == 0:
			n.WSLayout, n.wsLayout = l, !l.split()
			if l.split() {
				n.Layout = l
			}
		case l.split() && n.Layout.split():
			n.Layout = l // as i3 would after encapsulating and flattening
		default:
			t.encapsulate(n, l)
		}
		return
	}
	n.Layout = l
}

// ToggleLayout is `layout toggle [split|all]`: split turns splith and
// splitv (and takes stacked or tabbed back to the split they were);
// "all" goes stacked, tabbed, splith, splitv, round; "" goes stacked,
// tabbed, the split they were. Or a list of layouts to go round.
func (t *Tree) ToggleLayout(n *Node, how string, list ...Layout) {
	if floating(n) {
		return
	}
	p := n
	if n.Type != Workspace {
		p = n.Parent
	}
	cur := p.Layout
	var next Layout
	switch how {
	case "split":
		switch cur {
		case SplitH:
			next = SplitV
		case SplitV:
			next = SplitH
		default:
			next = p.LastSplit
		}
	case "all":
		next = [...]Layout{SplitH: SplitV, SplitV: Stacked, Stacked: Tabbed, Tabbed: SplitH}[cur]
	case "list":
		if len(list) == 0 {
			return
		}
		next = list[0]
		if i := slices.Index(list, cur); i >= 0 {
			next = list[(i+1)%len(list)]
		}
	default:
		switch cur {
		case Stacked:
			next = Tabbed
		case Tabbed:
			next = p.LastSplit
		default:
			next = Stacked
		}
	}
	t.SetLayout(n, next)
}

// FocusDir moves the focus to the next window in d: i3's tree_next. It
// goes up to the first container laid out along d with something there,
// and then down into what's there, on the side facing us; past the edge
// it wraps round. Floating windows go round left and right among
// themselves. Full screen, it stays in what's full screen.
func (t *Tree) FocusDir(d Dir) {
	n := t.focused
	fs := t.fullscreenOn(workspaceOf(n))
	var wrap *Node
	for n.Type != Workspace && n != fs {
		p := n.Parent
		if n.Type == FloatingCon {
			if !d.horizontal() {
				return
			}
			i := slices.Index(p.Floating, n)
			k := len(p.Floating)
			if d.back() {
				t.focus(descend(p.Floating[(i-1+k)%k]))
			} else {
				t.focus(descend(p.Floating[(i+1)%k]))
			}
			return
		}
		if len(p.Nodes) > 1 && p.Layout.horizontal() == d.horizontal() {
			i := slices.Index(p.Nodes, n)
			if j := i + step(d); j >= 0 && j < len(p.Nodes) {
				t.focus(descendDir(p.Nodes[j], d))
				return
			}
			if wrap == nil {
				if d.back() {
					wrap = p.Nodes[len(p.Nodes)-1]
				} else {
					wrap = p.Nodes[0]
				}
			}
		}
		n = p
	}
	if wrap != nil {
		t.focus(descendDir(wrap, d))
	}
}

func step(d Dir) int {
	if d.back() {
		return -1
	}
	return 1
}

// descendDir goes down from n to a window, toward where we came from:
// i3's con_descend_direction. Along a container laid out across d it
// takes the side facing us; otherwise the one focused last.
func descendDir(n *Node, d Dir) *Node {
	for len(n.Nodes) > 0 {
		switch {
		case n.Layout.horizontal() == d.horizontal() && d.back():
			n = n.Nodes[len(n.Nodes)-1]
		case n.Layout.horizontal() == d.horizontal():
			n = n.Nodes[0]
		default:
			n = focusedTiled(n)
		}
	}
	return n
}

// FocusParent focuses n's container, up to the workspace: then commands
// act on all of it.
func (t *Tree) FocusParent() {
	n := t.focused
	switch {
	case n.Type == Workspace:
	case n.Parent.Type == FloatingCon:
		t.focus(n.Parent.Parent)
	default:
		t.focus(n.Parent)
	}
}

// FocusChild goes back down from FocusParent.
func (t *Tree) FocusChild() {
	if len(t.focused.Focus) == 0 {
		return
	}
	c := t.focused.Focus[0]
	if c.Type == FloatingCon {
		c = c.Focus[0]
	}
	t.focus(c)
}

// FocusModeToggle goes between the floating windows and the tiled ones.
func (t *Tree) FocusModeToggle() {
	if floating(t.focused) {
		t.FocusTiling()
	} else {
		t.FocusFloating()
	}
}

// FocusTiling focuses the tiled window focused last.
func (t *Tree) FocusTiling() { t.focus(descendTiled(t.Current())) }

// FocusFloating focuses the floating window focused last.
func (t *Tree) FocusFloating() {
	ws := t.Current()
	if i := slices.IndexFunc(ws.Focus, func(c *Node) bool { return c.Type == FloatingCon }); i >= 0 {
		t.focus(descend(ws.Focus[i]))
	}
}
