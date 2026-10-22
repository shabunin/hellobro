package wm

// The tree as i3's IPC shows it (GET_TREE, GET_WORKSPACES, GET_OUTPUTS,
// and the events' containers), for i3-msg, swaymsg and the scripts and
// bars written for them. What X11 has and we don't (window ids, roles,
// types) is the container's id or null.

import "image"

// IPCRect is a rectangle as i3 has it.
type IPCRect struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

func ipcRect(r image.Rectangle) IPCRect { return IPCRect{r.Min.X, r.Min.Y, r.Dx(), r.Dy()} }

// IPCProps are a window's properties: X11's, from the app's window.
type IPCProps struct {
	Class      string  `json:"class"`
	Instance   string  `json:"instance"`
	Title      string  `json:"title"`
	WindowRole *string `json:"window_role"`
}

// IPCNode is a node in GET_TREE's reply.
type IPCNode struct {
	ID                 int64     `json:"id"`
	Type               string    `json:"type"`
	Orientation        string    `json:"orientation"`
	ScratchpadState    string    `json:"scratchpad_state"`
	Percent            *float64  `json:"percent"`
	Urgent             bool      `json:"urgent"`
	Marks              []string  `json:"marks"`
	Focused            bool      `json:"focused"`
	Output             string    `json:"output,omitempty"`
	Layout             string    `json:"layout"`
	WorkspaceLayout    string    `json:"workspace_layout"`
	LastSplitLayout    string    `json:"last_split_layout"`
	Border             string    `json:"border"`
	CurrentBorderWidth int       `json:"current_border_width"`
	Rect               IPCRect   `json:"rect"`
	DecoRect           IPCRect   `json:"deco_rect"`
	WindowRect         IPCRect   `json:"window_rect"`
	Geometry           IPCRect   `json:"geometry"`
	Name               *string   `json:"name"`
	Num                *int      `json:"num,omitempty"`
	Window             *int64    `json:"window"`
	WindowType         *string   `json:"window_type"`
	WindowProperties   *IPCProps `json:"window_properties,omitempty"`
	Nodes              []IPCNode `json:"nodes"`
	FloatingNodes      []IPCNode `json:"floating_nodes"`
	Focus              []int64   `json:"focus"`
	FullscreenMode     int       `json:"fullscreen_mode"`
	Sticky             bool      `json:"sticky"`
	Floating           string    `json:"floating"`
	Swallows           []any     `json:"swallows"`
}

// IPCWorkspace is a workspace in GET_WORKSPACES' reply.
type IPCWorkspace struct {
	ID      int64   `json:"id"`
	Num     int     `json:"num"`
	Name    string  `json:"name"`
	Visible bool    `json:"visible"`
	Focused bool    `json:"focused"`
	Urgent  bool    `json:"urgent"`
	Rect    IPCRect `json:"rect"`
	Output  string  `json:"output"`
}

// IPCOutput is an output in GET_OUTPUTS' reply.
type IPCOutput struct {
	Name             string  `json:"name"`
	Active           bool    `json:"active"`
	Primary          bool    `json:"primary"`
	CurrentWorkspace *string `json:"current_workspace"`
	Rect             IPCRect `json:"rect"`
}

// IPCTree is the whole tree.
func (t *Tree) IPCTree() IPCNode { return t.IPCNode(t.Root) }

// IPCNode is n and what's under it.
func (t *Tree) IPCNode(n *Node) IPCNode {
	j := IPCNode{
		ID: n.ID, Type: n.Type.String(), Orientation: "none", ScratchpadState: n.Scratch.String(),
		Urgent: n.Urgent || n.Type == Workspace && t.urgentIn(n), Marks: append([]string{}, n.Marks...),
		Focused: n == t.focused, Layout: n.Layout.String(), WorkspaceLayout: "default",
		LastSplitLayout: n.LastSplit.String(), Border: n.Border.String(),
		Rect: ipcRect(n.Rect), Nodes: []IPCNode{}, FloatingNodes: []IPCNode{}, Focus: []int64{},
		Floating: "auto_off", Swallows: []any{},
	}
	if o := outputOf(n); o != nil {
		j.Output = o.Name
	}
	switch n.Type {
	case Root:
		j.Layout = "splith"
	case Output:
		j.Layout = "output"
	case Workspace, Con, FloatingCon:
		if !n.leaf() {
			j.Orientation = "horizontal"
			if !n.Layout.horizontal() {
				j.Orientation = "vertical"
			}
		}
	}
	if n.Parent != nil && n.Type != Workspace && n.Type != Output && n.Type != Content {
		p := n.Percent
		j.Percent = &p
	}
	if n.Border == BorderPixel {
		j.CurrentBorderWidth = 2
	}
	if !n.Deco.Empty() && n.Parent != nil {
		j.DecoRect = ipcRect(n.Deco.Sub(n.Parent.Rect.Min))
	}
	if n.Fullscreen {
		j.FullscreenMode = 1
	}
	if floating(n) && n.Type != FloatingCon {
		j.Floating = "user_on"
	}
	name := n.Name
	if n.leaf() {
		w := n.Window
		name = w.Title()
		id := n.ID
		j.Window = &id
		j.WindowProperties = &IPCProps{Class: w.Class(), Instance: w.Instance(), Title: w.Title()}
		if !n.WinRect.Empty() {
			j.WindowRect = ipcRect(n.WinRect.Sub(n.Rect.Min))
			j.Geometry = IPCRect{Width: n.WinRect.Dx(), Height: n.WinRect.Dy()}
		}
	}
	if n.leaf() || name != "" {
		j.Name = &name
	}
	if n.Type == Workspace {
		num := n.Num
		j.Num = &num
	}
	for _, c := range n.Nodes {
		j.Nodes = append(j.Nodes, t.IPCNode(c))
	}
	for _, c := range n.Floating {
		j.FloatingNodes = append(j.FloatingNodes, t.IPCNode(c))
	}
	for _, c := range n.Focus {
		j.Focus = append(j.Focus, c.ID)
	}
	return j
}

func (t *Tree) urgentIn(n *Node) bool {
	return len(t.find(n, func(c *Node) bool { return c.Urgent })) > 0
}

func outputOf(n *Node) *Node {
	for ; n != nil; n = n.Parent {
		if n.Type == Output {
			return n
		}
	}
	return nil
}

// IPCWorkspaces are the workspaces, in the bar's order.
func (t *Tree) IPCWorkspaces() []IPCWorkspace {
	ws := []IPCWorkspace{}
	for _, w := range t.content.Nodes {
		ws = append(ws, IPCWorkspace{
			ID: w.ID, Num: w.Num, Name: w.Name, Visible: w == t.cur, Focused: w == t.cur,
			Urgent: t.urgentIn(w), Rect: ipcRect(t.content.Rect), Output: t.out.Name,
		})
	}
	return ws
}

// IPCOutputs are the outputs: the screen, and i3's own for the scratchpad.
func (t *Tree) IPCOutputs() []IPCOutput {
	cur := t.cur.Name
	return []IPCOutput{
		{Name: "__i3", Rect: ipcRect(t.out.Rect)},
		{Name: t.out.Name, Active: true, Primary: true, CurrentWorkspace: &cur, Rect: ipcRect(t.out.Rect)},
	}
}
