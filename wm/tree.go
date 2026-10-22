// Package wm is a tiling window manager's tree, after i3's: a root, an
// output (the screen), its workspaces, and in each workspace a tree of
// containers. A container holds a window, or lays its children out: side
// by side (splith), one above another (splitv), as tabs (tabbed) or under
// a stack of title bars (stacked). Floating windows hang off their
// workspace, above the tiled ones; the scratchpad is a hidden workspace.
//
// It knows nothing of drawing or keys: the app asks it to open, move or
// resize, and Render says where each window goes. The rules are i3's, so
// that i3's commands (and its IPC's tree) mean here what they mean there.
package wm

import (
	"image"
	"slices"
	"strconv"
)

// Type is a node's kind, named as i3's IPC names them.
type Type int

const (
	Root Type = iota
	Output
	Content
	Workspace
	Con
	FloatingCon
)

func (t Type) String() string {
	return [...]string{"root", "output", "con", "workspace", "con", "floating_con"}[t]
}

// Layout is how a container places its children.
type Layout int

const (
	SplitH Layout = iota
	SplitV
	Stacked
	Tabbed
)

func (l Layout) String() string { return [...]string{"splith", "splitv", "stacked", "tabbed"}[l] }

// horizontal says whether the children go left to right: splith, and
// tabbed's tabs.
func (l Layout) horizontal() bool { return l == SplitH || l == Tabbed }

func (l Layout) split() bool { return l == SplitH || l == SplitV }

// Border is a window's frame: normal has a title bar, pixel and none
// don't (on e-ink a pixel's border is none).
type Border int

const (
	BorderNormal Border = iota
	BorderPixel
	BorderNone
)

func (b Border) String() string { return [...]string{"normal", "pixel", "none"}[b] }

// Scratch is a floating container's state as a scratchpad window: fresh
// is put there and not moved or resized since, so it's placed when shown.
type Scratch int

const (
	ScratchNone Scratch = iota
	ScratchFresh
	ScratchChanged
)

func (s Scratch) String() string { return [...]string{"none", "fresh", "changed"}[s] }

// Dir is a direction to focus, move or resize in.
type Dir int

const (
	Left Dir = iota
	Right
	Up
	Down
)

func (d Dir) String() string { return [...]string{"left", "right", "up", "down"}[d] }

// back says whether d goes to the earlier children: left, up.
func (d Dir) back() bool { return d == Left || d == Up }

func (d Dir) horizontal() bool { return d == Left || d == Right }

// Window is what a leaf holds: the app's terminal, settings, launcher….
type Window interface {
	Title() string
	Class() string    // what kind: "terminal", "settings"…
	Instance() string // which of the kind: "ssh", "alpine", "wifi"…
}

// Node is a container: the root, an output, a workspace, a split
// container, or a leaf with a window.
type Node struct {
	ID        int64
	Type      Type
	Name      string // a workspace's, an output's
	Num       int    // a workspace's number; -1 if its name isn't one
	Layout    Layout
	LastSplit Layout  // the split layout to go back to from tabbed or stacked
	Percent   float64 // its share of its parent, tiled
	Window    Window  // a leaf's
	Parent    *Node
	Nodes     []*Node // tiled children
	Floating  []*Node // a workspace's floating containers, the top one last
	// Focus is the children, tiled and floating, the last focused first:
	// following the first at each level leads to the focused window.
	Focus      []*Node
	Fullscreen bool
	Urgent     bool
	Marks      []string
	Border     Border
	Scratch    Scratch         // a floating container's, from the scratchpad
	FloatRect  image.Rectangle // a floating container's place, or where a window floated last
	// WSLayout is an empty workspace's layout for the windows to come:
	// Stacked or Tabbed puts them in a container laid out so.
	WSLayout Layout
	wsLayout bool

	// Set by Render: all of it, its title bar or tab, and the window's
	// part; Visible if it shows.
	Rect, Deco, WinRect image.Rectangle
	Visible             bool
}

func (n *Node) leaf() bool { return n.Window != nil }

// Leaf says whether n holds a window.
func (n *Node) Leaf() bool { return n.leaf() }

// Tree is the whole: the screen's output and its workspaces, and where
// the focus is.
type Tree struct {
	Root    *Node
	out     *Node // the screen
	content *Node // its workspaces
	scratch *Node // __i3_scratch
	focused *Node
	cur     *Node  // the workspace shown
	prevWS  string // for workspace back_and_forth
	lastID  int64

	TitleH int // a title bar's height: a window's, a tab's, a stack's row
}

// New makes a tree for a screen of r (less any bar), with workspace 1.
func New(r image.Rectangle, titleH int) *Tree {
	t := &Tree{TitleH: titleH}
	t.Root = t.node(Root)
	t.Root.Name = "root"
	i3 := t.add(t.Root, Output, "__i3")
	t.scratch = t.add(t.add(i3, Content, "content"), Workspace, "__i3_scratch")
	t.out = t.add(t.Root, Output, "kobo")
	t.content = t.add(t.out, Content, "content")
	t.SetRect(r)
	t.focused = t.workspace("1")
	t.focus(t.focused)
	return t
}

func (t *Tree) node(ty Type) *Node {
	t.lastID++
	return &Node{ID: t.lastID, Type: ty, Num: -1}
}

// add puts a structural node (output, content, workspace) under p.
func (t *Tree) add(p *Node, ty Type, name string) *Node {
	n := t.node(ty)
	n.Name, n.Parent, n.Percent = name, p, 1
	p.Nodes = append(p.Nodes, n)
	p.Focus = append(p.Focus, n)
	return n
}

// SetRect is a new screen: rotated, or the bar's changed.
func (t *Tree) SetRect(r image.Rectangle) {
	t.out.Rect, t.content.Rect = r, r
	t.Root.Rect = r
}

// Focused is the node with the focus: a window's leaf, a container (focus
// parent), or an empty workspace.
func (t *Tree) Focused() *Node { return t.focused }

// Current is the workspace shown.
func (t *Tree) Current() *Node { return t.cur }

// Workspaces are the screen's, in order.
func (t *Tree) Workspaces() []*Node { return t.content.Nodes }

// Scratchpad is the hidden workspace.
func (t *Tree) Scratchpad() *Node { return t.scratch }

func workspaceOf(n *Node) *Node {
	for n != nil && n.Type != Workspace {
		n = n.Parent
	}
	return n
}

// workspace finds the workspace named name, or makes it: numbered ones in
// order of their numbers, then the others in the order they came.
func (t *Tree) workspace(name string) *Node {
	for _, w := range t.content.Nodes {
		if w.Name == name {
			return w
		}
	}
	w := t.node(Workspace)
	w.Name, w.Parent, w.Percent = name, t.content, 1
	w.Num = wsNum(name)
	w.Layout = SplitH
	if r := t.content.Rect; r.Dy() > r.Dx() {
		w.Layout = SplitV // as i3's default_orientation auto: along the longer side
	}
	w.LastSplit = w.Layout
	i := slices.IndexFunc(t.content.Nodes, func(o *Node) bool {
		return w.Num >= 0 && (o.Num < 0 || o.Num > w.Num)
	})
	if i < 0 {
		i = len(t.content.Nodes)
	}
	t.content.Nodes = slices.Insert(t.content.Nodes, i, w)
	t.content.Focus = append(t.content.Focus, w)
	return w
}

// wsNum is the number a workspace's name starts with ("3", "3:mail"),
// or -1.
func wsNum(name string) int {
	i := 0
	for i < len(name) && name[i] >= '0' && name[i] <= '9' {
		i++
	}
	n, err := strconv.Atoi(name[:i])
	if err != nil {
		return -1
	}
	return n
}

// focus makes n the focused node: it goes to the front of its parent's
// focus list, and so on up.
// A floating container focused comes to the top.
func (t *Tree) focus(n *Node) {
	t.focused = n
	if w := workspaceOf(n); w != nil && w != t.scratch {
		t.cur = w
	}
	front(n, nil)
	if f := floatOf(n); f != nil {
		if i := slices.Index(f.Parent.Floating, f); i >= 0 {
			f.Parent.Floating = append(slices.Delete(f.Parent.Floating, i, i+1), f)
		}
	}
}

// front puts n first in its parent's focus list, and so on up to top.
func front(n, top *Node) {
	for c := n; c != top && c.Parent != nil; c = c.Parent {
		p := c.Parent
		if i := slices.Index(p.Focus, c); i > 0 {
			p.Focus = slices.Insert(slices.Delete(p.Focus, i, i+1), 0, c)
		}
	}
}

// descend follows the focus lists down from n to the window focused last
// in it (or an empty container).
func descend(n *Node) *Node {
	for len(n.Focus) > 0 {
		n = n.Focus[0]
	}
	return n
}

// descendTiled is descend, but never into a floating container.
func descendTiled(n *Node) *Node {
	for {
		i := slices.IndexFunc(n.Focus, func(c *Node) bool { return c.Type != FloatingCon })
		if i < 0 {
			return n
		}
		n = n.Focus[i]
	}
}

// focusedTiled is n's tiled child focused last.
func focusedTiled(n *Node) *Node {
	if i := slices.IndexFunc(n.Focus, func(c *Node) bool { return c.Type != FloatingCon }); i >= 0 {
		return n.Focus[i]
	}
	return nil
}

// floating says whether n is (in) a floating container.
func floating(n *Node) bool {
	for ; n != nil && n.Type != Workspace; n = n.Parent {
		if n.Type == FloatingCon {
			return true
		}
	}
	return false
}

// floatOf is the floating container n is in.
func floatOf(n *Node) *Node {
	for ; n != nil && n.Type != Workspace; n = n.Parent {
		if n.Type == FloatingCon {
			return n
		}
	}
	return nil
}

// attach puts c in p's tiled children at i (-1: the end); the shares are
// made even for it, as i3's con_fix_percent does: c gets 1/n, the others
// what's left, in their proportions. c isn't focused by it.
func attach(p, c *Node, i int) {
	if i < 0 || i > len(p.Nodes) {
		i = len(p.Nodes)
	}
	c.Parent = p
	n := float64(len(p.Nodes) + 1)
	sum := 0.0
	for _, o := range p.Nodes {
		sum += o.Percent
	}
	for _, o := range p.Nodes {
		if sum > 0 {
			o.Percent = o.Percent / sum * (n - 1) / n
		} else {
			o.Percent = 1 / n
		}
	}
	c.Percent = 1 / n
	p.Nodes = slices.Insert(p.Nodes, i, c)
	p.Focus = append(p.Focus, c)
}

// detach takes c out of its parent, tiled or floating; the others' shares
// grow to fill its place.
func detach(c *Node) {
	p := c.Parent
	if p == nil {
		return
	}
	if i := slices.Index(p.Nodes, c); i >= 0 {
		p.Nodes = slices.Delete(p.Nodes, i, i+1)
		sum := 0.0
		for _, o := range p.Nodes {
			sum += o.Percent
		}
		for _, o := range p.Nodes {
			if sum > 0 {
				o.Percent /= sum
			} else {
				o.Percent = 1 / float64(len(p.Nodes))
			}
		}
	}
	if i := slices.Index(p.Floating, c); i >= 0 {
		p.Floating = slices.Delete(p.Floating, i, i+1)
	}
	if i := slices.Index(p.Focus, c); i >= 0 {
		p.Focus = slices.Delete(p.Focus, i, i+1)
	}
	c.Parent = nil
}

// replace puts n where o was in o's parent: its place, share and focus.
func replace(o, n *Node) {
	p := o.Parent
	n.Parent, n.Percent = p, o.Percent
	if i := slices.Index(p.Nodes, o); i >= 0 {
		p.Nodes[i] = n
	}
	if i := slices.Index(p.Floating, o); i >= 0 {
		p.Floating[i] = n
	}
	if i := slices.Index(p.Focus, o); i >= 0 {
		p.Focus[i] = n
	}
	o.Parent = nil
}

// tidy goes up from n after something left it: a container left empty
// goes (as i3's on_remove_child), and a workspace left empty goes if it's
// not the one shown. It's the nearest node left that it returns.
func (t *Tree) tidy(n *Node) *Node {
	for n != nil {
		switch {
		case n.Type == Con && len(n.Nodes) == 0 && !n.leaf(),
			n.Type == FloatingCon && len(n.Nodes) == 0:
			p := n.Parent
			detach(n)
			n = p
			continue
		case n.Type == Workspace:
			if len(n.Nodes) == 0 && len(n.Floating) == 0 && n != t.Current() && n != t.scratch {
				detach(n)
				return t.Current()
			}
		}
		return n
	}
	return nil
}

// flatten takes out the split containers that do nothing, as i3's
// tree_flatten: one holding a single split container across it, inside
// a parent along it (as moving across a split leaves behind).
func flatten(n *Node) {
	for _, c := range slices.Clone(n.Nodes) {
		flatten(c)
	}
	p := n.Parent
	if n.Type != Con || n.leaf() || p == nil || len(n.Nodes) != 1 {
		return
	}
	c := n.Nodes[0]
	if !n.Layout.split() || !c.Layout.split() || c.leaf() ||
		n.Layout == c.Layout || c.Layout.horizontal() != p.Layout.horizontal() ||
		(p.Type != Con && p.Type != Workspace) {
		return
	}
	// c's children go in n's place, in its share
	i := slices.Index(p.Nodes, n)
	p.Nodes = slices.Delete(p.Nodes, i, i+1)
	focused := slices.Index(p.Focus, n)
	p.Focus = slices.Delete(p.Focus, focused, focused+1)
	n.Parent = nil
	for j, k := range c.Nodes {
		k.Parent = p
		k.Percent *= n.Percent
		p.Nodes = slices.Insert(p.Nodes, i+j, k)
	}
	order := c.Focus
	p.Focus = slices.Insert(p.Focus, focused, order...)
}

// ShowWorkspace goes to the workspace named name, made if need be; the
// one left goes if it's empty.
func (t *Tree) ShowWorkspace(name string) {
	cur := t.Current()
	if name == cur.Name || name == "" || name == t.scratch.Name {
		return
	}
	w := t.workspace(name)
	t.prevWS = cur.Name
	t.focus(descend(w))
	t.tidy(cur)
}

// WorkspaceBackAndForth goes back to the workspace shown before.
func (t *Tree) WorkspaceBackAndForth() {
	if t.prevWS != "" {
		t.ShowWorkspace(t.prevWS)
	}
}

// WorkspaceStep goes to the next (d 1) or previous (-1) workspace, round.
func (t *Tree) WorkspaceStep(d int) {
	ws := t.content.Nodes
	i := slices.Index(ws, t.Current())
	t.ShowWorkspace(ws[(i+d+len(ws))%len(ws)].Name)
}

// Focus moves the focus to n (a window's leaf, say, found by criteria),
// on its workspace, shown if it isn't; a scratchpad window comes out, and
// a window full screen in front of it goes back.
func (t *Tree) Focus(n *Node) {
	w := workspaceOf(n)
	if w == nil {
		return
	}
	if w == t.scratch {
		t.showScratch(floatOf(n))
		return
	}
	if fs := t.fullscreenOn(w); fs != nil && !t.within(n, fs) {
		fs.Fullscreen = false
	}
	if cur := t.Current(); w != cur {
		t.prevWS = cur.Name
		t.focus(n)
		t.tidy(cur)
		return
	}
	t.focus(n)
}

// Find is every node for which f says so.
func (t *Tree) Find(f func(*Node) bool) []*Node { return t.find(t.Root, f) }

func (t *Tree) find(n *Node, f func(*Node) bool) []*Node {
	var out []*Node
	var walk func(*Node)
	walk = func(n *Node) {
		if f(n) {
			out = append(out, n)
		}
		for _, c := range n.Nodes {
			walk(c)
		}
		for _, c := range n.Floating {
			walk(c)
		}
	}
	walk(n)
	return out
}

// Windows are all the leaves, scratchpad's too.
func (t *Tree) Windows() []*Node { return t.Find((*Node).leaf) }

// ByID finds a node.
func (t *Tree) ByID(id int64) *Node {
	if f := t.Find(func(n *Node) bool { return n.ID == id }); len(f) > 0 {
		return f[0]
	}
	return nil
}
