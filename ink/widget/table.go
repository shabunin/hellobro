package widget

import (
	"hellobro/ink"
	"hellobro/ink/layout"
)

// Table is rows of columns under a row of their heads: a tap on a head
// sorts by it (again: the other way), a tap on a row is Clicked. The
// rows are the program's, sorted and filtered as it likes: Sort and Desc
// say how they're to go. The rows move under the finger (Body).
type Table struct {
	Sort  int  // the column it's sorted by
	Desc  bool // the other way
	Heads []Clickable
	Rows  []Clickable
	Body  layout.Scroll

	sorted  bool
	clicked []int
}

// Update takes the events since the last frame: cols columns, n rows.
func (t *Table) Update(gtx ink.Ctx, cols, n int) {
	t.Body.Axis = layout.Vertical
	for len(t.Heads) < cols {
		t.Heads = append(t.Heads, Clickable{})
	}
	for len(t.Rows) < n {
		t.Rows = append(t.Rows, Clickable{})
	}
	for c := range t.Heads {
		for t.Heads[c].Clicked(gtx) {
			if c == t.Sort {
				t.Desc = !t.Desc
			} else {
				t.Sort, t.Desc = c, false
			}
			t.sorted = true
			t.Body.ScrollTo(0)
		}
	}
	for i := range t.Rows {
		for t.Rows[i].Clicked(gtx) {
			t.clicked = append(t.clicked, i)
		}
	}
}

// Sorted says Sort or Desc changed since it was last asked: the rows
// are to be sorted again.
func (t *Table) Sorted(gtx ink.Ctx) bool {
	t.Update(gtx, len(t.Heads), len(t.Rows))
	s := t.sorted
	t.sorted = false
	return s
}

// Clicked is a row tapped since the last frame, if one was.
func (t *Table) Clicked(gtx ink.Ctx) (int, bool) {
	t.Update(gtx, len(t.Heads), len(t.Rows))
	if len(t.clicked) == 0 {
		return 0, false
	}
	i := t.clicked[0]
	t.clicked = t.clicked[1:]
	return i, true
}

// Node is a tree's: a leaf, or a branch with Kids. Key is its own, among
// all of the tree's.
type Node struct {
	Key, Title, Sub string
	Kids            []Node
	Branch          bool // a branch even with no kids yet
}

// IsBranch says it opens.
func (n *Node) IsBranch() bool { return n.Branch || len(n.Kids) > 0 }

// TreeRow is a node as it shows: how deep, and the guides to its left
// (Last[d]: the node at depth d was its parent's last, no line goes on
// under it).
type TreeRow struct {
	Node  *Node
	Depth int
	Last  []bool
}

// Tree is nodes in rows, a branch's kids under it when it's open: a tap
// on a branch opens or closes it, on a leaf picks it. A row in focus:
// → opens, ← closes, Enter is a tap. The rows move under the finger.
type Tree struct {
	Open map[string]bool
	Body layout.Scroll

	rows   map[string]*Clickable
	branch map[string]bool
	picked []string
}

var treeKeys = []string{"Left", "Right"}

// Row is the clickable of the row keyed key.
func (t *Tree) Row(key string) *Clickable {
	if t.rows == nil {
		t.rows = map[string]*Clickable{}
	}
	c := t.rows[key]
	if c == nil {
		c = &Clickable{Keys: treeKeys}
		t.rows[key] = c
	}
	return c
}

// Visible is the rows that show: the nodes, and the kids of the open ones.
func (t *Tree) Visible(nodes []Node) []TreeRow {
	t.branch = map[string]bool{}
	var rows []TreeRow
	var walk func(ns []Node, depth int, last []bool)
	walk = func(ns []Node, depth int, last []bool) {
		for i := range ns {
			n := &ns[i]
			l := append(append([]bool(nil), last...), i == len(ns)-1)
			rows = append(rows, TreeRow{n, depth, l})
			t.branch[n.Key] = n.IsBranch()
			if n.IsBranch() && t.Open[n.Key] {
				walk(n.Kids, depth+1, l)
			}
		}
	}
	walk(nodes, 0, nil)
	return rows
}

func (t *Tree) setOpen(key string, on bool) {
	if t.Open == nil {
		t.Open = map[string]bool{}
	}
	if on {
		t.Open[key] = true
	} else {
		delete(t.Open, key)
	}
}

// Update takes the events since the last frame.
func (t *Tree) Update(gtx ink.Ctx) {
	t.Body.Axis = layout.Vertical
	for key, c := range t.rows {
		br := t.branch[key]
		for c.Clicked(gtx) {
			if br {
				t.setOpen(key, !t.Open[key])
			} else {
				t.picked = append(t.picked, key)
			}
		}
		for _, k := range c.TakeKeys(gtx) {
			if br {
				t.setOpen(key, k.Name == "Right")
				gtx.Invalidate()
			}
		}
	}
}

// Picked is a leaf tapped since the last frame, if one was.
func (t *Tree) Picked(gtx ink.Ctx) (string, bool) {
	t.Update(gtx)
	if len(t.picked) == 0 {
		return "", false
	}
	k := t.picked[0]
	t.picked = t.picked[1:]
	return k, true
}
