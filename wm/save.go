package wm

// The tree as kept between runs: the workspaces, their containers and
// layouts, the windows (as the app's word for each), the floating ones'
// places, the scratchpad, the marks and where the focus was. Load builds
// it again in a new tree; a window the app can't make again is left out,
// and what's empty then goes.

import (
	"image"
	"slices"
)

// Saved is a tree, kept.
type Saved struct {
	Workspaces []SavedNode `json:"workspaces"`
	Scratch    []SavedNode `json:"scratch,omitempty"` // floating containers
}

// SavedNode is a workspace, a container or a window.
type SavedNode struct {
	Name       string      `json:"name,omitempty"` // a workspace's
	Layout     string      `json:"layout,omitempty"`
	LastSplit  string      `json:"last_split,omitempty"`
	Percent    float64     `json:"percent,omitempty"`
	Window     string      `json:"window,omitempty"` // the app's word for it
	Border     string      `json:"border,omitempty"`
	Marks      []string    `json:"marks,omitempty"`
	Fullscreen bool        `json:"fullscreen,omitempty"`
	Rect       []int       `json:"rect,omitempty"`    // a floating container's: x, y, width, height
	Scratch    string      `json:"scratch,omitempty"` // a floating container's, from the scratchpad
	Nodes      []SavedNode `json:"nodes,omitempty"`
	Floating   []SavedNode `json:"floating,omitempty"`
	Focus      []int       `json:"focus,omitempty"` // the children (tiled, then floating), the last focused first
	Focused    bool        `json:"focused,omitempty"`
	Current    bool        `json:"current,omitempty"` // the workspace shown
}

// Save keeps t; word is the app's word for a window ("" leaves it out).
func (t *Tree) Save(word func(Window) string) Saved {
	var s Saved
	for _, w := range t.content.Nodes {
		s.Workspaces = append(s.Workspaces, t.save(w, word))
	}
	for _, f := range t.scratch.Floating {
		s.Scratch = append(s.Scratch, t.save(f, word))
	}
	return s
}

func (t *Tree) save(n *Node, word func(Window) string) SavedNode {
	s := SavedNode{
		Layout: n.Layout.String(), LastSplit: n.LastSplit.String(), Percent: n.Percent,
		Border: n.Border.String(), Marks: slices.Clone(n.Marks), Fullscreen: n.Fullscreen,
		Focused: n == t.focused,
	}
	switch n.Type {
	case Workspace:
		s.Name, s.Current, s.Percent = n.Name, n == t.cur, 0
	case FloatingCon:
		r := n.FloatRect
		s.Rect = []int{r.Min.X, r.Min.Y, r.Dx(), r.Dy()}
		if n.Scratch != ScratchNone {
			s.Scratch = n.Scratch.String()
		}
	}
	if n.leaf() {
		s.Window = word(n.Window)
		s.Layout, s.LastSplit = "", ""
	}
	all := append(slices.Clone(n.Nodes), n.Floating...)
	for _, c := range n.Nodes {
		s.Nodes = append(s.Nodes, t.save(c, word))
	}
	for _, c := range n.Floating {
		s.Floating = append(s.Floating, t.save(c, word))
	}
	for _, c := range n.Focus {
		if i := slices.Index(all, c); i >= 0 {
			s.Focus = append(s.Focus, i)
		}
	}
	return s
}

// Load builds s in t, a new tree; mk is the window for a word, or nil
// if there's none now. It returns the windows' leaves.
func (t *Tree) Load(s Saved, mk func(word string) Window) []*Node {
	l := &loader{t: t, mk: mk}
	cur := ""
	for _, sw := range s.Workspaces {
		if sw.Name == "" || sw.Name == t.scratch.Name {
			continue
		}
		w := t.workspace(sw.Name)
		w.Layout, w.LastSplit = layoutNamed(sw.Layout, w.Layout), layoutNamed(sw.LastSplit, w.LastSplit)
		l.children(w, sw)
		if sw.Current {
			cur = w.Name
		}
	}
	for _, sf := range s.Scratch {
		if f := l.node(sf, FloatingCon); f != nil {
			f.Parent = t.scratch
			t.scratch.Floating = append(t.scratch.Floating, f)
			t.scratch.Focus = append(t.scratch.Focus, f)
		}
	}
	// The focus: where it was, else on the workspace that was shown.
	f := l.focused
	if f == nil || workspaceOf(f) == t.scratch {
		f = t.workspace("1")
		if cur != "" {
			f = t.workspace(cur)
		}
		f = descend(f)
	}
	if ws := workspaceOf(f); len(ws.Nodes) == 0 && len(ws.Floating) == 0 { // all it had is gone: one that has some
		for _, w := range t.content.Nodes {
			if len(w.Nodes) > 0 || len(w.Floating) > 0 {
				f = descend(w)
				break
			}
		}
	}
	t.focus(f)
	for _, w := range slices.Clone(t.content.Nodes) {
		t.tidy(w)
	}
	return l.leaves
}

type loader struct {
	t       *Tree
	mk      func(string) Window
	leaves  []*Node
	focused *Node
}

// children builds s's children under n, with their shares and focus.
func (l *loader) children(n *Node, s SavedNode) {
	var all []*Node // as s has them: tiled, then floating; nil for those left out
	for _, c := range s.Nodes {
		k := l.node(c, Con)
		if k != nil {
			k.Parent = n
			n.Nodes = append(n.Nodes, k)
		}
		all = append(all, k)
	}
	for _, c := range s.Floating {
		k := (*Node)(nil)
		if n.Type == Workspace {
			k = l.node(c, FloatingCon)
		}
		if k != nil {
			k.Parent = n
			n.Floating = append(n.Floating, k)
		}
		all = append(all, k)
	}
	sum := 0.0
	for _, c := range n.Nodes {
		sum += c.Percent
	}
	for _, c := range n.Nodes {
		if sum > 0 && c.Percent > 0 {
			c.Percent /= sum
		} else {
			c.Percent = 1 / float64(len(n.Nodes))
		}
	}
	for _, i := range s.Focus {
		if i >= 0 && i < len(all) && all[i] != nil && !slices.Contains(n.Focus, all[i]) {
			n.Focus = append(n.Focus, all[i])
		}
	}
	for _, c := range all {
		if c != nil && !slices.Contains(n.Focus, c) {
			n.Focus = append(n.Focus, c)
		}
	}
}

// node builds s as a node of type ty; nil if nothing of it is left.
func (l *loader) node(s SavedNode, ty Type) *Node {
	t := l.t
	n := t.node(ty)
	n.Layout, n.LastSplit = layoutNamed(s.Layout, SplitH), layoutNamed(s.LastSplit, SplitH)
	if !n.LastSplit.split() {
		n.LastSplit = SplitH
	}
	n.Percent = s.Percent
	n.Marks = slices.Clone(s.Marks)
	n.Fullscreen = s.Fullscreen
	switch s.Border {
	case "pixel":
		n.Border = BorderPixel
	case "none":
		n.Border = BorderNone
	}
	if ty == FloatingCon {
		if len(s.Rect) == 4 && s.Rect[2] > 0 && s.Rect[3] > 0 {
			n.FloatRect = image.Rect(s.Rect[0], s.Rect[1], s.Rect[0]+s.Rect[2], s.Rect[1]+s.Rect[3])
		}
		switch s.Scratch {
		case "fresh":
			n.Scratch = ScratchFresh
		case "changed":
			n.Scratch = ScratchChanged
		}
	}
	if s.Window != "" {
		if ty != Con || len(s.Nodes) > 0 {
			return nil // a window is a leaf of its own
		}
		w := l.mk(s.Window)
		if w == nil {
			return nil
		}
		n.Window = w
		l.leaves = append(l.leaves, n)
	} else {
		l.children(n, s)
		if len(n.Nodes) == 0 {
			return nil
		}
	}
	if s.Focused {
		l.focused = n
	}
	return n
}

func layoutNamed(s string, def Layout) Layout {
	if l, ok := layouts[s]; ok {
		return l
	}
	return def
}
