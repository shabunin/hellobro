package wm

import "strings"

// Dump writes a workspace's tree short, for tests and the log:
//
//	1:splith[a, splitv[b*, c]] float[d]
//
// a window by its title, a container by its layout; * is the focus, ! full
// screen.
func (t *Tree) Dump(ws *Node) string {
	var b strings.Builder
	b.WriteString(ws.Name)
	b.WriteString(":")
	t.dump(&b, ws)
	if len(ws.Floating) > 0 {
		b.WriteString(" float[")
		for i, f := range ws.Floating {
			if i > 0 {
				b.WriteString(", ")
			}
			for _, c := range f.Nodes {
				t.dump(&b, c)
			}
		}
		b.WriteString("]")
	}
	return b.String()
}

// DumpAll is every workspace's Dump, the one shown in <>.
func (t *Tree) DumpAll() string {
	var out []string
	for _, w := range t.content.Nodes {
		d := t.Dump(w)
		if w == t.Current() {
			d = "<" + d + ">"
		}
		out = append(out, d)
	}
	if len(t.scratch.Floating) > 0 {
		out = append(out, t.Dump(t.scratch))
	}
	return strings.Join(out, " ")
}

func (t *Tree) dump(b *strings.Builder, n *Node) {
	if n.leaf() {
		b.WriteString(n.Window.Title())
	} else {
		b.WriteString(n.Layout.String())
		b.WriteString("[")
		for i, c := range n.Nodes {
			if i > 0 {
				b.WriteString(", ")
			}
			t.dump(b, c)
		}
		b.WriteString("]")
	}
	if n == t.focused && n.Type != Workspace {
		b.WriteString("*")
	}
	if n.Fullscreen {
		b.WriteString("!")
	}
}

// Name is how i3 names a container in a tab or title bar: a window by
// its title, a container by its layout's letter and what's in it.
func Name(n *Node) string {
	if n.leaf() {
		return n.Window.Title()
	}
	var b strings.Builder
	b.WriteString(map[Layout]string{SplitH: "H", SplitV: "V", Stacked: "S", Tabbed: "T"}[n.Layout])
	b.WriteString("[")
	for i, c := range n.Nodes {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(Name(c))
	}
	b.WriteString("]")
	return b.String()
}
