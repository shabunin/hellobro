package wm

import (
	"image"
	"slices"
)

// SetFloating floats n (a window, or a container with its windows) above
// the tiled ones on its workspace, or tiles it again where the focus was
// among them: i3's floating enable and disable. It keeps the focus.
func (t *Tree) SetFloating(n *Node, on bool) {
	if n.Type != Con || floating(n) == on {
		return
	}
	ws := workspaceOf(n)
	had := t.focused == n
	if on {
		old := n.Parent
		detach(n)
		t.tidy(old)
		f := t.node(FloatingCon)
		f.Parent, f.Percent, f.Layout = ws, 1, SplitH
		f.FloatRect = n.FloatRect
		if f.FloatRect.Empty() {
			r := ws.Rect
			if r.Empty() {
				r = t.content.Rect
			}
			f.FloatRect = center(image.Rect(0, 0, r.Dx()*2/3, r.Dy()*2/3), r)
		}
		ws.Floating = append(ws.Floating, f)
		ws.Focus = append(ws.Focus, f)
		attach(f, n, -1)
		n.Percent = 1
	} else {
		f := floatOf(n)
		n.FloatRect = f.FloatRect
		detach(n)
		detach(f)
		p, after := t.target(ws)
		attach(p, n, indexAfter(p, after))
	}
	if had {
		t.focus(n)
	}
}

// ToggleFloating floats n or tiles it.
func (t *Tree) ToggleFloating(n *Node) { t.SetFloating(n, !floating(n)) }

// center places r's size in the middle of in.
func center(r, in image.Rectangle) image.Rectangle {
	p := in.Min.Add(image.Pt((in.Dx()-r.Dx())/2, (in.Dy()-r.Dy())/2))
	return image.Rectangle{p, p.Add(r.Size())}
}

// keepIn moves r into in, as much of it as fits.
func keepIn(r, in image.Rectangle) image.Rectangle {
	if r.Dx() > in.Dx() {
		r.Max.X = r.Min.X + in.Dx()
	}
	if r.Dy() > in.Dy() {
		r.Max.Y = r.Min.Y + in.Dy()
	}
	d := image.Point{}
	if r.Min.X < in.Min.X {
		d.X = in.Min.X - r.Min.X
	} else if r.Max.X > in.Max.X {
		d.X = in.Max.X - r.Max.X
	}
	if r.Min.Y < in.Min.Y {
		d.Y = in.Min.Y - r.Min.Y
	} else if r.Max.Y > in.Max.Y {
		d.Y = in.Max.Y - r.Max.Y
	}
	return r.Add(d)
}

// SetFloatRect places n's floating container (a drag, `move position`,
// a resize), kept on the screen.
func (t *Tree) SetFloatRect(n *Node, r image.Rectangle) {
	if f := floatOf(n); f != nil {
		f.FloatRect = keepIn(r.Canon(), t.content.Rect)
		if f.Scratch == ScratchFresh {
			f.Scratch = ScratchChanged
		}
	}
}

// FloatRect is where n's floating container is.
func (t *Tree) FloatRect(n *Node) image.Rectangle {
	if f := floatOf(n); f != nil {
		return f.FloatRect
	}
	return image.Rectangle{}
}

// MoveFloating moves n's floating container px in d.
func (t *Tree) MoveFloating(n *Node, d Dir, px int) {
	f := floatOf(n)
	if f == nil {
		return
	}
	v := map[Dir]image.Point{Left: {-px, 0}, Right: {px, 0}, Up: {0, -px}, Down: {0, px}}[d]
	t.SetFloatRect(n, f.FloatRect.Add(v))
}

// CenterFloating puts n's floating container in the middle.
func (t *Tree) CenterFloating(n *Node) {
	if f := floatOf(n); f != nil {
		f.FloatRect = center(f.FloatRect, t.content.Rect)
	}
}

// fullscreenOn is what's full screen on ws, if anything.
func (t *Tree) fullscreenOn(ws *Node) *Node {
	if ws == nil {
		return nil
	}
	if f := t.find(ws, func(n *Node) bool { return n.Fullscreen }); len(f) > 0 {
		return f[0]
	}
	return nil
}

func (t *Tree) unFullscreen(ws *Node) {
	if f := t.fullscreenOn(ws); f != nil {
		f.Fullscreen = false
	}
}

// SetFullscreen puts n full screen, its workspace's only one (focused, if
// it's on the one shown), or back.
func (t *Tree) SetFullscreen(n *Node, on bool) {
	if n.Type != Con || n.Fullscreen == on {
		return
	}
	ws := workspaceOf(n)
	if on {
		t.unFullscreen(ws)
		if ws == t.Current() {
			t.focus(n)
		}
	}
	n.Fullscreen = on
}

// ToggleFullscreen puts n full screen or back.
func (t *Tree) ToggleFullscreen(n *Node) { t.SetFullscreen(n, !n.Fullscreen) }

// Fullscreen is what's full screen on the workspace shown, if anything.
func (t *Tree) Fullscreen() *Node { return t.fullscreenOn(t.Current()) }

// MoveToScratchpad hides n away, floating, for ScratchpadShow to bring
// back.
func (t *Tree) MoveToScratchpad(n *Node) {
	if n.Type != Con || workspaceOf(n) == t.scratch {
		return
	}
	t.SetFloating(n, true)
	n.Fullscreen = false
	f := floatOf(n)
	if f.Scratch == ScratchNone {
		f.Scratch = ScratchFresh
	}
	t.toWorkspace(f, t.scratch)
}

// ScratchpadShow is i3's scratchpad show: with n nil, it hides the
// scratchpad window focused, or focuses one shown, or shows the one
// hidden longest. With n (found by criteria), it hides n if it shows,
// and shows it if not.
func (t *Tree) ScratchpadShow(n *Node) bool {
	cur := t.Current()
	if n == nil {
		if f := floatOf(t.focused); f != nil && f.Scratch != ScratchNone {
			t.toWorkspace(f, t.scratch)
			return true
		}
		for _, f := range cur.Floating {
			if f.Scratch != ScratchNone && f != floatOf(t.focused) {
				t.focus(descendTiled(f))
				return true
			}
		}
		if len(t.scratch.Floating) == 0 {
			return false
		}
		t.showScratch(t.scratch.Floating[0])
		return true
	}
	f := floatOf(n)
	if f == nil || f.Scratch == ScratchNone {
		return false
	}
	if workspaceOf(f) == cur {
		t.toWorkspace(f, t.scratch)
		return true
	}
	t.showScratch(f)
	return true
}

// showScratch brings scratchpad's (or another workspace's) f here, in the
// middle while it's fresh, and focuses it.
func (t *Tree) showScratch(f *Node) {
	cur := t.Current()
	t.unFullscreen(cur)
	if workspaceOf(f) != cur {
		t.toWorkspace(f, cur)
	}
	if f.Scratch == ScratchFresh {
		r := t.content.Rect
		f.FloatRect = center(image.Rect(0, 0, r.Dx()/2, r.Dy()*3/4), r)
	}
	t.focus(descend(f))
}

// toWorkspace moves n (a container, tiled or floating) to ws: tiled, next
// to where the focus was among ws's tiled ones; floating, on top. On ws,
// it's what's focused; the focus stays where it was, unless it was in n:
// then it goes to what was focused before it.
func (t *Tree) toWorkspace(n, ws *Node) {
	from := workspaceOf(n)
	if from == ws {
		return
	}
	had := t.within(t.focused, n)
	n.Fullscreen = false
	p := n.Parent
	detach(n)
	if n.Type == FloatingCon {
		n.Parent = ws
		ws.Floating = append(ws.Floating, n)
		ws.Focus = slices.Insert(ws.Focus, 0, n)
	} else {
		q, after := t.target(ws)
		attach(q, n, indexAfter(q, after))
		front(n, ws)
	}
	if ws == t.cur && !had {
		t.focus(t.focused) // the shown workspace's focus is where it was
	}
	left := t.tidy(p)
	if had {
		if from != t.Current() || left == nil {
			left = from
		}
		t.focus(descend(left))
	}
}
