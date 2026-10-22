package wm

import (
	"errors"
	"image"
	"slices"
)

// Mark marks n m: a name only one container has at a time. Without add,
// n's other marks go; with toggle, a mark n has already goes instead.
func (t *Tree) Mark(n *Node, m string, add, toggle bool) {
	if toggle && slices.Contains(n.Marks, m) {
		n.Marks = slices.DeleteFunc(n.Marks, func(s string) bool { return s == m })
		return
	}
	t.Unmark(nil, m)
	if !add {
		n.Marks = nil
	}
	n.Marks = append(n.Marks, m)
}

// Unmark takes mark m (all, for "") off n (any, for nil).
func (t *Tree) Unmark(n *Node, m string) {
	for _, o := range t.Find(func(o *Node) bool { return len(o.Marks) > 0 && (n == nil || o == n) }) {
		o.Marks = slices.DeleteFunc(o.Marks, func(s string) bool { return m == "" || s == m })
		if len(o.Marks) == 0 {
			o.Marks = nil
		}
	}
}

// Marks are all the marks, as GET_MARKS has them.
func (t *Tree) Marks() []string {
	var out []string
	for _, n := range t.Find(func(o *Node) bool { return len(o.Marks) > 0 }) {
		out = append(out, n.Marks...)
	}
	return out
}

// MoveToMark moves n next to the container marked (after it), or into it
// if it's a split container, tiled; to its workspace if it floats.
func (t *Tree) MoveToMark(n, to *Node) error {
	if t.within(to, n) {
		return errors.New("can't move a container into itself")
	}
	if n.Type == Workspace || n.Type != Con {
		return errors.New("can't move that")
	}
	if floating(to) || to.Type == Workspace {
		ws := workspaceOf(to)
		if floating(n) {
			t.MoveToWorkspace(n, ws.Name)
			return nil
		}
		if to.Type == Workspace {
			t.MoveToWorkspace(n, ws.Name)
			return nil
		}
		return errors.New("can't move a tiled window next to a floating one")
	}
	if floating(n) {
		t.SetFloating(n, false)
	}
	from := workspaceOf(n)
	had := t.within(t.focused, n)
	if had {
		t.focused = t.cur
	}
	old := n.Parent
	detach(n)
	if to.leaf() {
		p := to.Parent
		attach(p, n, slices.Index(p.Nodes, to)+1)
	} else {
		attach(to, n, -1)
	}
	t.tidy(old)
	flatten(from)
	if had {
		if workspaceOf(n) == t.cur {
			t.focus(n)
		} else {
			t.focus(descend(t.cur))
		}
	}
	return nil
}

func pt(x, y int) image.Point { return image.Pt(x, y) }
