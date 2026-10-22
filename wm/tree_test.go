package wm

import (
	"image"
	"math"
	"strings"
	"testing"
)

type win string

func (w win) Title() string    { return string(w) }
func (w win) Class() string    { return "test" }
func (w win) Instance() string { return string(w) }

var screen = image.Rect(0, 0, 1200, 900)

// build makes a tree from Dumps joined by " | ": * is the focus, + was
// focused before it (so it's first among the tiled ones), ! full screen.
// The first workspace with the focus (else the first) is shown.
func build(t *testing.T, spec string) *Tree {
	t.Helper()
	tr := New(screen, 20)
	var star *Node
	var plus []*Node
	shown := ""
	for _, ws := range strings.Split(spec, " | ") {
		i := strings.LastIndex(ws[:strings.Index(ws, "[")], ":")
		name, rest := ws[:i], ws[i+1:]
		w := tr.workspace(name)
		if shown == "" {
			shown = name
		}
		p := &parser{s: rest}
		l, kids, marks := p.node(tr)
		w.Layout, w.LastSplit = layoutOf(t, l), layoutOf(t, l)
		for _, k := range kids {
			attach(w, k, -1)
		}
		_ = marks
		if strings.HasPrefix(p.s, " float[") {
			p.s = p.s[len(" float["):]
			for !strings.HasPrefix(p.s, "]") {
				l, kids, m := p.node(tr)
				n := p.made(tr, l, kids, m)
				f := tr.node(FloatingCon)
				f.Parent, f.Percent = w, 1
				f.FloatRect = image.Rect(100, 100, 500, 400)
				w.Floating = append(w.Floating, f)
				w.Focus = append(w.Focus, f)
				attach(f, n, -1)
				p.s = strings.TrimPrefix(p.s, ", ")
			}
			p.s = p.s[1:]
		}
		if p.s != "" {
			t.Fatalf("build %q: left %q", ws, p.s)
		}
		if p.star != nil {
			star, shown = p.star, name
		}
		plus = append(plus, p.plus...)
	}
	for _, n := range plus {
		tr.focus(n)
	}
	cur := tr.workspace(shown)
	if star != nil {
		tr.focus(star)
	} else {
		tr.focus(descend(cur))
	}
	if shown != "1" {
		tr.tidy(tr.workspace("1"))
	}
	return tr
}

type parser struct {
	s    string
	star *Node
	plus []*Node
}

// node reads a title or layout[...] with its marks.
func (p *parser) node(tr *Tree) (string, []*Node, string) {
	i := strings.IndexAny(p.s, "[],*+! ")
	if i < 0 {
		i = len(p.s)
	}
	name := p.s[:i]
	p.s = p.s[i:]
	var kids []*Node
	if strings.HasPrefix(p.s, "[") {
		p.s = p.s[1:]
		for !strings.HasPrefix(p.s, "]") {
			l, k, m := p.node(tr)
			kids = append(kids, p.made(tr, l, k, m))
			p.s = strings.TrimPrefix(p.s, ", ")
		}
		p.s = p.s[1:]
		if kids == nil {
			kids = []*Node{}
		}
	}
	j := strings.IndexFunc(p.s, func(r rune) bool { return !strings.ContainsRune("*+!", r) })
	if j < 0 {
		j = len(p.s)
	}
	marks := p.s[:j]
	p.s = p.s[j:]
	if kids == nil {
		return name, nil, marks
	}
	return name, kids, marks
}

func (p *parser) made(tr *Tree, name string, kids []*Node, marks string) *Node {
	n := tr.node(Con)
	if kids == nil {
		n.Window = win(name)
	} else {
		l := map[string]Layout{"splith": SplitH, "splitv": SplitV, "stacked": Stacked, "tabbed": Tabbed}[name]
		n.Layout, n.LastSplit = l, l
		if !l.split() {
			n.LastSplit = SplitH
		}
		for _, k := range kids {
			attach(n, k, -1)
		}
	}
	if strings.Contains(marks, "*") {
		p.star = n
	}
	if strings.Contains(marks, "+") {
		p.plus = append(p.plus, n)
	}
	n.Fullscreen = strings.Contains(marks, "!")
	return n
}

func layoutOf(t *testing.T, s string) Layout {
	l, ok := map[string]Layout{"splith": SplitH, "splitv": SplitV}[s]
	if !ok {
		t.Fatalf("a workspace's layout %q", s)
	}
	return l
}

// w finds a window by its title.
func w(tr *Tree, title string) *Node {
	f := tr.Find(func(n *Node) bool { return n.leaf() && n.Window.Title() == title })
	if len(f) != 1 {
		panic("no window " + title)
	}
	return f[0]
}

func TestBuild(t *testing.T) {
	for _, s := range []string{
		"1:splith[]",
		"1:splith[a*]",
		"1:splith[a, splitv[b*, c]]",
		"1:splitv[tabbed[a, b*], c] float[d, e]",
		"1:splith[a] | <2:splith[b*]>",
	} {
		tr := build(t, strings.NewReplacer("<", "", ">", "").Replace(s))
		want := strings.ReplaceAll(s, " | ", " ")
		if !strings.Contains(s, "<") {
			want = "<" + want + ">"
		}
		if got := tr.DumpAll(); got != want {
			t.Errorf("build %q: dumps %q", s, got)
		}
	}
}

func TestOps(t *testing.T) {
	for _, c := range []struct {
		name, in string
		op       func(*Tree)
		want     string
	}{
		{"open empty", "1:splith[]", func(t *Tree) { t.Open(win("a")) }, "1:splith[a*]"},
		{"open after focus", "1:splith[a*, b]", func(t *Tree) { t.Open(win("c")) }, "1:splith[a, c*, b]"},
		{"open nested", "1:splith[a, splitv[b*, c]]", func(t *Tree) { t.Open(win("d")) }, "1:splith[a, splitv[b, d*, c]]"},
		{"open from floating", "1:splith[a+, b] float[f*]", func(t *Tree) { t.Open(win("c")) }, "1:splith[a, c*, b] float[f]"},
		{"open from parent", "1:splith[a, splitv[b*, c]]", func(t *Tree) { t.FocusParent(); t.Open(win("d")) }, "1:splith[a, splitv[b, c], d*]"},
		{"open unfullscreens", "1:splith[a*!, b]", func(t *Tree) { t.Open(win("c")) }, "1:splith[a, c*, b]"},
		{"open tabbed workspace", "1:splith[]", func(t *Tree) {
			t.SetLayout(t.Current(), Tabbed)
			t.Open(win("a"))
			t.Open(win("b"))
		}, "1:splith[tabbed[a, b*]]"},

		{"remove", "1:splith[a, splitv[b*, c]]", func(t *Tree) { t.Remove(w(t, "b")) }, "1:splith[a, splitv[c*]]"},
		{"remove last in split", "1:splith[a, splitv[b*]]", func(t *Tree) { t.Remove(w(t, "b")) }, "1:splith[a*]"},
		{"remove focus goes back", "1:splith[a+, b, c*]", func(t *Tree) { t.Remove(w(t, "c")) }, "1:splith[a*, b]"},
		{"remove unfocused", "1:splith[a, b*]", func(t *Tree) { t.Remove(w(t, "a")) }, "1:splith[b*]"},
		{"remove container", "1:splith[a, splitv[b, c]*]", func(t *Tree) { t.Remove(t.Focused()) }, "1:splith[a*]"},
		{"remove floating", "1:splith[a] float[f*]", func(t *Tree) { t.Remove(w(t, "f")) }, "1:splith[a*]"},

		{"split", "1:splith[a, b*]", func(t *Tree) { t.Split(t.Focused(), SplitV); t.Open(win("c")) }, "1:splith[a, splitv[b, c*]]"},
		{"split alone turns", "1:splith[a, splitv[b*]]", func(t *Tree) { t.Split(t.Focused(), SplitH) }, "1:splith[a, splith[b*]]"},
		{"split workspace one", "1:splith[a*]", func(t *Tree) { t.Split(t.Focused(), SplitV) }, "1:splitv[a*]"},
		{"split workspace focused", "1:splith[a, b*]", func(t *Tree) {
			t.FocusParent()
			t.Split(t.Focused(), SplitV)
		}, "1:splitv[splith[a, b]]"},
		{"split toggle", "1:splith[a, b*]", func(t *Tree) { t.SplitToggle(t.Focused()) }, "1:splith[a, splitv[b*]]"},

		{"layout workspace", "1:splith[a*, b]", func(t *Tree) { t.SetLayout(t.Focused(), Tabbed) }, "1:splith[tabbed[a*, b]]"},
		{"layout nested", "1:splith[a, splitv[b*, c]]", func(t *Tree) { t.SetLayout(t.Focused(), Stacked) }, "1:splith[a, stacked[b*, c]]"},
		{"toggle split", "1:splith[a, splitv[b*, c]]", func(t *Tree) { t.ToggleLayout(t.Focused(), "split") }, "1:splith[a, splith[b*, c]]"},
		{"toggle round", "1:splith[a, splitv[b*, c]]", func(t *Tree) {
			for range 3 {
				t.ToggleLayout(t.Focused(), "")
			}
		}, "1:splith[a, splitv[b*, c]]"},
		{"toggle twice", "1:splith[a, splitv[b*, c]]", func(t *Tree) {
			t.ToggleLayout(t.Focused(), "")
			t.ToggleLayout(t.Focused(), "")
		}, "1:splith[a, tabbed[b*, c]]"},
		{"toggle split from tabbed", "1:splith[a, splitv[b*, c]]", func(t *Tree) {
			t.SetLayout(t.Focused(), Tabbed)
			t.ToggleLayout(t.Focused(), "split")
		}, "1:splith[a, splitv[b*, c]]"},
		{"toggle all", "1:splith[a, splith[b*, c]]", func(t *Tree) { t.ToggleLayout(t.Focused(), "all") }, "1:splith[a, splitv[b*, c]]"},
		{"toggle list", "1:splith[a, tabbed[b*, c]]", func(t *Tree) {
			t.ToggleLayout(t.Focused(), "list", Stacked, Tabbed, SplitV)
		}, "1:splith[a, splitv[b*, c]]"},

		{"focus right", "1:splith[a*, b]", func(t *Tree) { t.FocusDir(Right) }, "1:splith[a, b*]"},
		{"focus right wraps", "1:splith[a, b*]", func(t *Tree) { t.FocusDir(Right) }, "1:splith[a*, b]"},
		{"focus into focused", "1:splith[a*, splitv[b, c+]]", func(t *Tree) { t.FocusDir(Right) }, "1:splith[a, splitv[b, c*]]"},
		{"focus into near side", "1:splith[a*, splith[b, c+]]", func(t *Tree) { t.FocusDir(Right) }, "1:splith[a, splith[b*, c]]"},
		{"focus left into near side", "1:splith[splith[b+, c], a*]", func(t *Tree) { t.FocusDir(Left) }, "1:splith[splith[b, c*], a]"},
		{"focus out", "1:splith[a, splitv[b*, c]]", func(t *Tree) { t.FocusDir(Left) }, "1:splith[a*, splitv[b, c]]"},
		{"focus up", "1:splith[a, splitv[b, c*]]", func(t *Tree) { t.FocusDir(Up) }, "1:splith[a, splitv[b*, c]]"},
		{"focus wraps inner", "1:splith[a, splitv[b, c*]]", func(t *Tree) { t.FocusDir(Down) }, "1:splith[a, splitv[b*, c]]"},
		{"focus nowhere", "1:splith[a*, b]", func(t *Tree) { t.FocusDir(Down) }, "1:splith[a*, b]"},
		{"focus tabs", "1:splith[tabbed[a*, b, c]]", func(t *Tree) { t.FocusDir(Left) }, "1:splith[tabbed[a, b, c*]]"},
		{"focus floating round", "1:splith[a] float[f*, g]", func(t *Tree) { t.FocusDir(Left) }, "1:splith[a] float[f, g*]"},
		{"focus fullscreen stays", "1:splith[a*!, b]", func(t *Tree) { t.FocusDir(Right) }, "1:splith[a*!, b]"},
		{"focus in fullscreen container", "1:splith[splitv[a*, b]!, c]", func(t *Tree) { t.FocusDir(Down) }, "1:splith[splitv[a, b*]!, c]"},
		{"focus parent child", "1:splith[a, splitv[b, c*]]", func(t *Tree) { t.FocusParent(); t.FocusParent(); t.FocusChild() }, "1:splith[a, splitv[b, c]*]"},
		{"focus next", "1:splith[a, splitv[b*, c]] float[f]", func(t *Tree) { t.FocusStep(1, false) }, "1:splith[a, splitv[b, c*]] float[f]"},
		{"focus next round", "1:splith[a, splitv[b, c]] float[f*]", func(t *Tree) { t.FocusStep(1, false) }, "1:splith[a*, splitv[b, c]] float[f]"},
		{"focus prev", "1:splith[a, splitv[b*, c]]", func(t *Tree) { t.FocusStep(-1, false) }, "1:splith[a*, splitv[b, c]]"},
		{"focus next sibling", "1:splith[a*, splitv[b, c]]", func(t *Tree) { t.FocusStep(1, true) }, "1:splith[a, splitv[b, c]*]"},
		{"focus parent stops", "1:splith[a*]", func(t *Tree) { t.FocusParent(); t.FocusParent() }, "1:splith[a]"},
		{"focus mode toggle", "1:splith[a*] float[f]", func(t *Tree) { t.FocusModeToggle() }, "1:splith[a] float[f*]"},
		{"focus mode back", "1:splith[a, b+] float[f*]", func(t *Tree) { t.FocusModeToggle() }, "1:splith[a, b*] float[f]"},

		{"move swap", "1:splith[a*, b, c]", func(t *Tree) { t.Move(t.Focused(), Right, 10) }, "1:splith[b, a*, c]"},
		{"move into", "1:splith[a*, splitv[b, c]]", func(t *Tree) { t.Move(t.Focused(), Right, 10) }, "1:splith[splitv[b, a*, c]]"},
		{"move into along", "1:splith[a*, splith[b, c]]", func(t *Tree) { t.Move(t.Focused(), Right, 10) }, "1:splith[splith[a*, b, c]]"},
		{"move out right", "1:splith[a, splitv[b*, c]]", func(t *Tree) { t.Move(t.Focused(), Right, 10) }, "1:splith[a, splitv[c], b*]"},
		{"move out left", "1:splith[a, splitv[b*, c]]", func(t *Tree) { t.Move(t.Focused(), Left, 10) }, "1:splith[a, b*, splitv[c]]"},
		{"move up turns", "1:splith[a, b*]", func(t *Tree) { t.Move(t.Focused(), Up, 10) }, "1:splitv[b*, splith[a]]"},
		{"move edge", "1:splith[a*, b]", func(t *Tree) { t.Move(t.Focused(), Left, 10) }, "1:splith[a*, b]"},
		{"move alone", "1:splith[a*]", func(t *Tree) { t.Move(t.Focused(), Up, 10) }, "1:splith[a*]"},
		{"move flattens", "1:splith[splitv[splith[a, b], d*], c]", func(t *Tree) { t.Move(t.Focused(), Right, 10) }, "1:splith[a, b, d*, c]"},
		{"move tab", "1:splith[tabbed[a*, b]]", func(t *Tree) { t.Move(t.Focused(), Right, 10) }, "1:splith[tabbed[b, a*]]"},
		{"move container", "1:splith[splitv[a, b]*, c]", func(t *Tree) { t.Move(t.Focused(), Right, 10) }, "1:splith[c, splitv[a, b]*]"},

		{"float", "1:splith[a, b*]", func(t *Tree) { t.ToggleFloating(t.Focused()) }, "1:splith[a] float[b*]"},
		{"float and back", "1:splith[a+, b*, c]", func(t *Tree) {
			t.ToggleFloating(t.Focused())
			t.ToggleFloating(t.Focused())
		}, "1:splith[a, b*, c]"},
		{"float empties split", "1:splith[a, splitv[b*]]", func(t *Tree) { t.ToggleFloating(t.Focused()) }, "1:splith[a] float[b*]"},
		{"tile into empty", "1:splith[] float[f*]", func(t *Tree) { t.ToggleFloating(t.Focused()) }, "1:splith[f*]"},

		{"fullscreen", "1:splith[a, b*]", func(t *Tree) { t.ToggleFullscreen(w(t, "a")) }, "1:splith[a*!, b]"},
		{"fullscreen one", "1:splith[a!, b*]", func(t *Tree) { t.ToggleFullscreen(t.Focused()) }, "1:splith[a, b*!]"},

		{"swap", "1:splith[a*, splitv[b, c]]", func(t *Tree) { t.Swap(w(t, "a"), w(t, "c")) }, "1:splith[c, splitv[b, a*]]"},
	} {
		tr := build(t, c.in)
		c.op(tr)
		check(t, tr)
		if got := tr.Dump(tr.Current()); got != c.want {
			t.Errorf("%s: %s → %s, want %s", c.name, c.in, got, c.want)
		}
	}
}

func TestWorkspaces(t *testing.T) {
	for _, c := range []struct {
		name, in string
		op       func(*Tree)
		want     string
	}{
		{"show new", "1:splith[a*]", func(t *Tree) { t.ShowWorkspace("2") }, "1:splith[a] <2:splith[]>"},
		{"show and back", "1:splith[a*]", func(t *Tree) { t.ShowWorkspace("2"); t.ShowWorkspace("1") }, "<1:splith[a*]>"},
		{"back and forth", "1:splith[a*] | 3:splith[b]", func(t *Tree) {
			t.ShowWorkspace("3")
			t.WorkspaceBackAndForth()
		}, "<1:splith[a*]> 3:splith[b]"},
		{"in order", "1:splith[a*]", func(t *Tree) {
			t.ShowWorkspace("mail")
			t.Open(win("m"))
			t.ShowWorkspace("10")
			t.Open(win("x"))
			t.ShowWorkspace("2")
			t.Open(win("b"))
		}, "1:splith[a] <2:splith[b*]> 10:splith[x] mail:splith[m]"},
		{"step", "1:splith[a*] | 2:splith[b] | 3:splith[c]", func(t *Tree) { t.WorkspaceStep(-1) }, "1:splith[a] 2:splith[b] <3:splith[c*]>"},
		{"move to", "1:splith[a, b*]", func(t *Tree) { t.MoveToWorkspace(t.Focused(), "2") }, "<1:splith[a*]> 2:splith[b]"},
		{"move to, next to focus", "1:splith[a*] | 2:splith[b+, c]", func(t *Tree) {
			t.MoveToWorkspace(t.Focused(), "2")
		}, "<1:splith[]> 2:splith[b, a, c]"},
		{"move floating to", "1:splith[a] float[f*]", func(t *Tree) { t.MoveToWorkspace(t.Focused(), "2") }, "<1:splith[a*]> 2:splith[] float[f]"},
		{"move workspace", "1:splith[a, b*] | 2:splith[c]", func(t *Tree) {
			t.FocusParent()
			t.FocusParent()
			t.MoveToWorkspace(t.Focused(), "2")
		}, "<1:splith[]> 2:splith[c, splith[a, b]]"},
		{"empty goes", "1:splith[a] | 2:splith[b*]", func(t *Tree) { t.Remove(w(t, "a")) }, "<2:splith[b*]>"},
		{"rename", "1:splith[a*] | 3:splith[b]", func(t *Tree) { t.RenameWorkspace("3", "0") }, "0:splith[b] <1:splith[a*]>"},
		{"focus elsewhere", "1:splith[a*] | 2:splith[b, c]", func(t *Tree) { t.Focus(w(t, "c")) }, "1:splith[a] <2:splith[b, c*]>"},
		{"scratchpad", "1:splith[a, b*]", func(t *Tree) { t.MoveToScratchpad(t.Focused()) }, "<1:splith[a*]> __i3_scratch:splith[] float[b]"},
		{"scratchpad show", "1:splith[a, b*]", func(t *Tree) {
			t.MoveToScratchpad(t.Focused())
			t.ScratchpadShow(nil)
		}, "<1:splith[a] float[b*]>"},
		{"scratchpad hide", "1:splith[a, b*]", func(t *Tree) {
			t.MoveToScratchpad(t.Focused())
			t.ScratchpadShow(nil)
			t.ScratchpadShow(nil)
		}, "<1:splith[a*]> __i3_scratch:splith[] float[b]"},
		{"scratchpad round", "1:splith[a, b, c*]", func(t *Tree) {
			t.MoveToScratchpad(w(t, "c"))
			t.MoveToScratchpad(w(t, "b"))
			t.ScratchpadShow(nil)
			t.ScratchpadShow(nil)
			t.ScratchpadShow(nil)
		}, "<1:splith[a] float[b*]> __i3_scratch:splith[] float[c]"},
		{"scratchpad focus shown", "1:splith[a, b*]", func(t *Tree) {
			t.MoveToScratchpad(t.Focused())
			t.ScratchpadShow(nil)
			t.FocusTiling()
			t.ScratchpadShow(nil)
		}, "<1:splith[a] float[b*]>"},
		{"scratchpad by criteria", "1:splith[a, b*] | 2:splith[c]", func(t *Tree) {
			t.MoveToScratchpad(w(t, "c"))
			t.ScratchpadShow(w(t, "c"))
		}, "<1:splith[a, b] float[c*]>"},
	} {
		tr := build(t, c.in)
		c.op(tr)
		check(t, tr)
		if got := tr.DumpAll(); got != c.want {
			t.Errorf("%s: %s → %s, want %s", c.name, c.in, got, c.want)
		}
	}
}

// check looks the whole tree over: parents, focus lists, shares, and a
// focus that's in it.
func check(t *testing.T, tr *Tree) {
	t.Helper()
	var walk func(n *Node)
	walk = func(n *Node) {
		kids := append(append([]*Node{}, n.Nodes...), n.Floating...)
		if len(kids) != len(n.Focus) {
			t.Errorf("%s: %d children, %d in focus", tr.DumpAll(), len(kids), len(n.Focus))
		}
		sum := 0.0
		for _, c := range kids {
			if c.Parent != n {
				t.Errorf("%s: %s's parent is wrong", tr.DumpAll(), Name(c))
			}
			found := false
			for _, f := range n.Focus {
				found = found || f == c
			}
			if !found {
				t.Errorf("%s: %s not in focus", tr.DumpAll(), Name(c))
			}
			walk(c)
		}
		for _, c := range n.Nodes {
			sum += c.Percent
		}
		if n.Type == Con || n.Type == Workspace {
			if len(n.Nodes) > 0 && math.Abs(sum-1) > 1e-9 {
				t.Errorf("%s: shares in %s add up to %v", tr.DumpAll(), Name(n), sum)
			}
			if n.Type == Con && !n.leaf() && len(n.Nodes) == 0 {
				t.Errorf("%s: an empty container", tr.DumpAll())
			}
		}
	}
	walk(tr.Root)
	if !tr.within(tr.focused, tr.Root) || workspaceOf(tr.focused) == tr.scratch {
		t.Errorf("%s: the focus is lost", tr.DumpAll())
	}
	// the focus lists lead to the focus
	if f := descendFrom(tr.Current(), tr.focused); f != tr.focused {
		t.Errorf("%s: the focus lists lead elsewhere", tr.DumpAll())
	}
}

// descendFrom follows the focus lists down from n till it reaches stop.
func descendFrom(n, stop *Node) *Node {
	for n != stop && len(n.Focus) > 0 {
		n = n.Focus[0]
	}
	return n
}
