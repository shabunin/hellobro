package main

// The windows. hellobro is a tiling window manager, after i3 (the wm
// package has the tree and i3's commands): workspaces, in the bar where
// the tabs were, and on each a tree of windows, side by side, one above
// another, tabbed or stacked, or floating above them. A window is a
// terminal (an SSH session, a shell, Alpine; a launcher till it runs
// something) or one of the settings' panels (Wi-Fi, Bluetooth, Sharing,
// Settings, Battery, What uses it).
//
// The keys are i3's with Meta (the Win key): Meta+Enter a launcher,
// Meta+D or Meta+Space the switcher, Meta+arrows the focus,
// Meta+Shift+arrows the window, Meta+1…0 the workspaces (commands.go).
// The window in focus gets the rest. A window's strip has its name (black
// when it has the focus), + for a window next to it and × to close it.

import (
	"fmt"
	"image"
	"log"
	"slices"
	"strconv"
	"strings"

	"hellobro/kobo"
	"hellobro/wm"
)

// window is what a leaf of the tree holds.
type window interface {
	wm.Window
	// place puts it at p, laid out for it (with a title strip, or not);
	// an empty p.Outer: it doesn't show.
	place(a *bro, p kobo.Panel)
	panel(a *bro) kobo.Panel                     // as drawn: its boxes, for taps
	draw(a *bro)                                 // all of it
	drawTitle(a *bro)                            // its title strip: the focus came or went
	press(a *bro, code uint16, value int32) bool // a key, true if it took it
	tap(a *bro, p image.Point)
	closing(a *bro) // it's being closed: end what runs in it
}

const (
	wmGap   = kobo.Pad // between the windows
	wmTabH  = kobo.FoldH
	maxWins = 24
)

type wmState struct {
	tree      *wm.Tree
	wins      map[*wm.Node]window
	area      image.Rectangle   // where the windows go
	mode      string            // the keys' binding mode: "" is the default
	tabs      []wmTab           // tabbed and stacked containers' tabs, as drawn
	frames    []image.Rectangle // the frames around tabbed windows, as drawn
	laid      string            // what was placed last: another layout is a new screen
	focus     window            // the focus as drawn
	names     []string          // the workspaces in the bar
	empty     image.Rectangle   // an empty workspace's hint, for a tap
	drag      *wmDrag           // a floating window being moved by its strip
	composing bool              // drawing the floating windows over what was drawn
}

type wmTab struct {
	r image.Rectangle
	n *wm.Node
}

type wmDrag struct {
	g    *kobo.Gesture
	n    *wm.Node
	from image.Rectangle // where it was
}

// wmInit makes the tree, with a launcher on workspace 1.
func (a *bro) wmInit() {
	a.wm.tree = wm.New(image.Rect(0, 0, 1, 1), wmTabH)
	a.wm.wins = map[*wm.Node]window{}
	a.openWin(&termSession{})
}

func (a *bro) winOf(n *wm.Node) window {
	if n == nil {
		return nil
	}
	return a.wm.wins[n]
}

func (a *bro) nodeOf(w window) *wm.Node {
	for n, o := range a.wm.wins {
		if o == w {
			return n
		}
	}
	return nil
}

// focusedWin is the window in focus, if a window is (not a container,
// nor an empty workspace).
func (a *bro) focusedWin() window { return a.winOf(a.wm.tree.Focused()) }

// windows are all of them, in the tree's order.
func (a *bro) windows() []window {
	var ws []window
	for _, n := range a.wm.tree.Windows() {
		ws = append(ws, a.wm.wins[n])
	}
	return ws
}

// terms are the terminal windows.
func (a *bro) terms() []*termSession {
	var ts []*termSession
	for _, w := range a.windows() {
		if t, ok := w.(*termSession); ok {
			ts = append(ts, t)
		}
	}
	return ts
}

// shows says whether w is on screen, with nothing over it.
func (a *bro) shows(w window) bool {
	return w != nil && !w.panel(a).Outer.Empty() && !a.Calibrating() && !a.overlay() && !a.Dozing()
}

// guiShows says whether the settings' window key is on screen.
func (a *bro) guiShows(key string) bool {
	g := a.gui(key)
	return g != nil && a.shows(g) // not a nil *guiWin as a window: that's not nil
}

// gui is the settings' window key, if it's open.
func (a *bro) gui(key string) *guiWin {
	for _, w := range a.wm.wins {
		if g, ok := w.(*guiWin); ok && g.key == key {
			return g
		}
	}
	return nil
}

// guiFocused says whether the settings' window key has the focus.
func (a *bro) guiFocused(key string) bool {
	g, ok := a.focusedWin().(*guiWin)
	return ok && g.key == key
}

// openWin opens w next to the window in focus, and focuses it.
func (a *bro) openWin(w window) {
	if len(a.wm.wins) >= maxWins {
		a.Notify(fmt.Sprintf("%d windows are enough", maxWins))
		return
	}
	n := a.wm.tree.Open(w)
	a.wm.wins[n] = w
	log.Printf("wm: opened %s on %s", w.Title(), a.wm.tree.Current().Name)
	a.wmApply()
}

// openLauncher opens a launcher: a terminal that asks what to run.
func (a *bro) openLauncher() { a.openWin(&termSession{}) }

// openGUI focuses the settings' window key, or opens it.
func (a *bro) openGUI(key string) {
	if g := a.gui(key); g != nil {
		a.wm.tree.Focus(a.nodeOf(g))
		a.wmApply()
		return
	}
	if guiName(key) == "" {
		return
	}
	a.openWin(&guiWin{key: key})
}

// replaceWin puts w where old is (a launcher, say, that picked a settings
// window), and closes old.
func (a *bro) replaceWin(old, w window) {
	n := a.nodeOf(old)
	if n == nil {
		return
	}
	if g, ok := w.(*guiWin); ok && a.gui(g.key) != nil { // it's open already: there
		a.closeWin(old)
		a.openGUI(g.key)
		return
	}
	old.closing(a)
	n.Window = w
	a.wm.wins[n] = w
	a.wm.laid = "" // a new window there: draw it all
	a.wmApply()
}

// closeWin closes w: what runs in it ends, and it's gone.
func (a *bro) closeWin(w window) {
	n := a.nodeOf(w)
	if n == nil {
		return
	}
	log.Printf("wm: closing %s", w.Title())
	w.closing(a)
	delete(a.wm.wins, n)
	a.wm.tree.Remove(n)
	a.wmApply()
}

// wmRun runs a line of i3 commands (a key's, the switcher's, the IPC's)
// and shows what came of it.
func (a *bro) wmRun(line string) []wm.Result {
	rs := a.wm.tree.Run(line, wmHost{a})
	for _, r := range rs {
		if !r.Success {
			log.Printf("wm: %q: %s", line, r.Error)
		}
	}
	a.wmApply()
	return rs
}

// wmHost is what i3's commands need of the app.
type wmHost struct{ a *bro }

func (h wmHost) Kill(n *wm.Node) {
	if w := h.a.winOf(n); w != nil {
		w.closing(h.a)
		delete(h.a.wm.wins, n)
		h.a.wm.tree.Remove(n)
	}
}

// Exec runs cmd in a new Alpine window: never on the Kobo itself, which
// would be root for whoever reaches the IPC from a jail.
func (h wmHost) Exec(cmd string) error {
	t := &termSession{}
	h.a.openWin(t)
	h.a.termStartAlpine(t, cmd)
	return nil
}

func (h wmHost) Mode(name string) error {
	if name == "default" {
		name = ""
	}
	if name != "" && !slices.Contains(h.a.keyModes(), name) {
		return fmt.Errorf("no mode %q", name)
	}
	h.a.setMode(name)
	return nil
}

// Command runs hellobro's own: a command by its id (music.play, sleep),
// exit, reload, and open (a window: launcher, alpine, sh, ssh NAME, or a
// settings' one: wifi, bluetooth, sharing, settings, battery, usage).
func (h wmHost) Command(n *wm.Node, name string, args []string) (bool, error) {
	a := h.a
	switch name {
	case "exit":
		a.Quit()
		return true, nil
	case "reload":
		a.keys = loadKeys()
		a.keysChanged()
		return true, nil
	case "open":
		if len(args) == 0 {
			return true, fmt.Errorf("open what?")
		}
		switch args[0] {
		case "launcher":
			a.openLauncher()
		case "alpine", "sh":
			t := &termSession{}
			a.openWin(t)
			a.termStartShell(t, args[0] == "alpine")
		case "ssh":
			if len(args) < 2 {
				return true, fmt.Errorf("open ssh which host?")
			}
			i := slices.IndexFunc(a.tv.hosts, func(h sshHost) bool { return h.name == args[1] })
			if i < 0 {
				return true, fmt.Errorf("no host %q", args[1])
			}
			a.goHost(a.tv.hosts[i])
		default:
			if guiName(args[0]) == "" {
				return true, fmt.Errorf("open what? %q", args[0])
			}
			a.openGUI(args[0])
		}
		return true, nil
	}
	if c := commandByID(name); c != nil && len(args) == 0 && c.run != nil {
		if c.when != nil && !c.when(a) {
			return true, fmt.Errorf("%s can't now", name)
		}
		c.run(a)
		return true, nil
	}
	return false, nil
}

// --- placing and drawing ---

// wmApply shows the tree as it is now: the bar's workspaces, full screen
// or not, and the windows, drawn again where the layout changed, or only
// their title strips where only the focus moved.
func (a *bro) wmApply() {
	t := a.wm.tree
	a.wmBar()
	if full := t.Fullscreen() != nil; full != a.Full && !a.Calibrating() {
		a.wm.laid = ""
		a.SetFull(full) // lays out and draws it all
		a.wmFocusMoved()
		return
	}
	if a.C.Gray == nil || a.wm.area.Empty() {
		return
	}
	if a.wmPlace() {
		if !a.overlay() && !a.Calibrating() {
			a.DrawPage()
			a.MarkFlash(a.lay.Page)
		}
		a.wmFocusMoved()
		return
	}
	was := a.wm.focus
	a.wmFocusMoved()
	if now := a.wm.focus; now != was && !a.overlay() && !a.Calibrating() {
		for _, w := range []window{was, now} {
			if w != nil && a.nodeOf(w) != nil && !w.panel(a).Outer.Empty() {
				w.drawTitle(a)
			}
		}
		a.drawTabs()
	}
}

// wmFocusMoved is the focus as drawn now: what goes with it.
func (a *bro) wmFocusMoved() {
	w := a.focusedWin()
	if w == a.wm.focus {
		return
	}
	a.wm.focus = w
	a.tv.sel = nil // a terminal's selection is the focused one's
	a.fs.at = ""
	if g, ok := w.(*guiWin); ok {
		switch g.key {
		case "wifi", "bluetooth", "sharing", "settings":
			if a.rs.prompt == nil {
				a.rs.focus = radioPanelOf(g.key)
			}
		case "battery", "usage":
			a.pv.useFocus = g.key == "usage"
		}
	}
	if w != nil {
		log.Printf("wm: focus on %s", w.Title())
	}
}

// wmBar puts the workspaces in the bar.
func (a *bro) wmBar() {
	var names, hints []string
	cur := 0
	for i, w := range a.wm.tree.Workspaces() {
		names = append(names, w.Name)
		hint := ""
		if w.Num >= 0 && w.Num <= 10 {
			hint = a.keyLabel(fmt.Sprintf("workspace number %d", w.Num))
			hint = strings.TrimPrefix(hint, "Meta+")
		}
		hints = append(hints, hint)
		if w == a.wm.tree.Current() {
			cur = i
		}
	}
	if slices.Equal(names, a.wm.names) && cur == a.Tab {
		return
	}
	a.wm.names = names
	a.SetTabNames(names, hints, cur)
}

// TabTap goes to the workspace tapped in the bar.
func (a *bro) TabTap(i int) {
	if i < len(a.wm.names) {
		a.wmRun("workspace " + strconv.Quote(a.wm.names[i]))
	}
}

// wmPlace lays the windows out as the tree has them, and says whether
// that's another layout than the last (else only the focus may have
// moved).
func (a *bro) wmPlace() bool {
	t := a.wm.tree
	t.SetRect(a.wm.area)
	t.Render()
	var sig strings.Builder
	fmt.Fprint(&sig, a.wm.area, a.lay.Full)
	a.wm.tabs, a.wm.frames = nil, nil
	placed := map[window]bool{}
	fs := t.Fullscreen()
	var walk func(n *wm.Node)
	walk = func(n *wm.Node) {
		if !n.Visible {
			return
		}
		fmt.Fprint(&sig, n.ID, n.Rect, n.Layout, ";")
		if w := a.wm.wins[n]; w != nil {
			placed[w] = true
			r := n.Rect.Inset(wmGap / 2)
			switch {
			case n == fs:
				w.place(a, kobo.Panel{Name: w.Title(), Outer: n.Rect, Body: n.Rect})
			case n.Parent != nil && (n.Parent.Layout == wm.Tabbed || n.Parent.Layout == wm.Stacked) && n.Parent.Type != wm.FloatingCon:
				w.place(a, kobo.NewPanel(a.F, w.Title(), a.tabBody(n.Parent, n.Rect), ""))
			case n.Border == wm.BorderNone:
				w.place(a, kobo.Panel{Name: w.Title(), Outer: r, Body: r})
			default:
				w.place(a, kobo.NewPanel(a.F, w.Title(), r, ""))
			}
		}
		if n.Layout == wm.Tabbed || n.Layout == wm.Stacked {
			a.placeTabs(n)
		}
		for _, c := range n.Nodes {
			walk(c)
		}
		for _, c := range n.Floating {
			walk(c)
		}
	}
	if fs != nil {
		walk(fs)
	} else {
		walk(t.Current())
	}
	for _, w := range a.wm.wins {
		if !placed[w] && !w.panel(a).Outer.Empty() {
			w.place(a, kobo.Panel{})
		}
	}
	if s := sig.String(); s != a.wm.laid {
		a.wm.laid = s
		return true
	}
	return false
}

// tabBody is where a tabbed or stacked container's child c (at r, under
// the tabs) shows: in the container's gaps, under the tabs.
func (a *bro) tabBody(p *wm.Node, r image.Rectangle) image.Rectangle {
	in := p.Rect.Inset(wmGap / 2)
	return image.Rect(in.Min.X, r.Min.Y+wmGap/2, in.Max.X, in.Max.Y)
}

// placeTabs lays out the tabs (or the stack's strips) of n.
func (a *bro) placeTabs(n *wm.Node) {
	in := n.Rect.Inset(wmGap / 2)
	k := len(n.Nodes)
	for i, c := range n.Nodes {
		var r image.Rectangle
		if n.Layout == wm.Tabbed {
			x0, x1 := in.Min.X+in.Dx()*i/k, in.Min.X+in.Dx()*(i+1)/k
			r = image.Rect(x0, in.Min.Y, x1, in.Min.Y+wmTabH)
		} else {
			y := in.Min.Y + i*wmTabH
			r = image.Rect(in.Min.X, y, in.Max.X, y+wmTabH)
		}
		a.wm.tabs = append(a.wm.tabs, wmTab{r.Intersect(in), c})
	}
}

// Draw draws the workspace: the tabs, the windows, the floating ones
// over them, and the status bar.
func (a *bro) Draw() {
	if a.wm.tree == nil || a.Calibrating() {
		return
	}
	t := a.wm.tree
	a.wm.empty = image.Rectangle{}
	ws := t.Current()
	if len(ws.Nodes) == 0 && len(ws.Floating) == 0 {
		a.drawEmpty()
	}
	a.drawTabs()
	for _, n := range t.Shown() {
		if w := a.wm.wins[n]; w != nil && !floatingNode(n) {
			w.draw(a)
		}
	}
	a.drawFloats(image.Rectangle{})
	a.drawStat(true)
}

func floatingNode(n *wm.Node) bool {
	for ; n != nil; n = n.Parent {
		if n.Type == wm.FloatingCon {
			return true
		}
	}
	return false
}

// drawFloats draws the floating windows over r (all of them, for an empty
// r), from the bottom up.
func (a *bro) drawFloats(r image.Rectangle) {
	if a.wm.composing {
		return
	}
	a.wm.composing = true
	defer func() { a.wm.composing = false }()
	for _, n := range a.wm.tree.Shown() {
		if w := a.wm.wins[n]; w != nil && floatingNode(n) && (r.Empty() || r.Overlaps(w.panel(a).Outer)) {
			w.draw(a)
			r = r.Union(w.panel(a).Outer) // what's over this one is drawn again too
		}
	}
}

// Mark is the App's, but what a tiled window draws under a floating one
// has the floating one drawn again over it, before it reaches the panel.
func (a *bro) Mark(r image.Rectangle, ui bool) {
	a.App.Mark(r, ui)
	a.overFloats(r)
}

func (a *bro) MarkFlash(r image.Rectangle) {
	a.App.MarkFlash(r)
	a.overFloats(r)
}

func (a *bro) overFloats(r image.Rectangle) {
	if a.wm.tree == nil || a.wm.composing || r.Empty() || a.overlay() {
		return
	}
	for _, f := range a.wm.tree.Current().Floating {
		if f.Visible && f.Rect.Overlaps(r) {
			a.drawFloats(r)
			return
		}
	}
}

// drawTabs draws the tabs of the tabbed and stacked containers that
// show: the one shown open into it, black if it has the focus.
func (a *bro) drawTabs() {
	if a.overlay() || a.Calibrating() {
		return
	}
	c := a.C
	focus := a.wm.tree.Focused()
	for _, tb := range a.wm.tabs {
		r, n := tb.r, tb.n
		shown := n.Visible
		in := focus != nil && within(focus, n)
		bg, ink := uint8(kobo.Light), uint8(kobo.Dark)
		switch {
		case shown && in:
			bg, ink = kobo.Black, kobo.White
		case shown:
			bg, ink = kobo.White, kobo.Black
		}
		c.Fill(r, bg)
		c.Frame(r, 2, kobo.Black)
		name := wm.Name(n)
		if w := a.wm.wins[n]; w != nil {
			name = w.Title()
		}
		f := a.F.Small
		if shown {
			f = a.F.Panel
		}
		c.Text(f, r.Inset(10), name, ink, kobo.AlignCenter)
		a.Mark(r, true)
	}
}

func within(n, c *wm.Node) bool {
	for ; n != nil; n = n.Parent {
		if n == c {
			return true
		}
	}
	return false
}

// drawEmpty says what to do on an empty workspace.
func (a *bro) drawEmpty() {
	r := a.wm.area.Inset(wmGap)
	keys := a.keyHints("launcher", "a new window", "palette", "the switcher")
	lines := []string{"Nothing here yet.", "Tap here, or " + keys + "."}
	c := a.C
	y := r.Min.Y + r.Dy()/3
	for i, l := range lines {
		f := a.F.Body
		if i == 0 {
			f = a.F.Panel
		}
		c.Text(f, image.Rect(r.Min.X, y, r.Max.X, y+60), l, kobo.Dark, kobo.AlignCenter)
		y += 64
	}
	a.wm.empty = r
}

// --- input ---

// wmTap takes a tap on the workspace: a tab, a window's + or ×, a window
// (the one tapped takes the focus; a settings' window gets the tap too),
// or the empty workspace.
func (a *bro) wmTap(p image.Point) {
	for _, tb := range a.wm.tabs {
		if p.In(tb.r) {
			a.wm.tree.Focus(wmDescend(tb.n))
			a.wmApply()
			return
		}
	}
	w := a.windowAt(p)
	if w == nil {
		if p.In(a.wm.empty) {
			a.openLauncher()
		}
		return
	}
	pn := w.panel(a)
	switch {
	case kobo.Hit(p, pn.Close):
		a.closeWin(w)
		return
	case kobo.Hit(p, pn.New):
		a.openLauncher()
		return
	}
	if w != a.focusedWin() {
		a.wm.tree.Focus(a.nodeOf(w))
		a.wmApply()
		if _, ok := w.(*termSession); ok {
			return // a tap on a terminal: the focus, and that's all
		}
	}
	w.tap(a, p)
}

// wmDescend is the window focused last in n.
func wmDescend(n *wm.Node) *wm.Node {
	for len(n.Focus) > 0 {
		n = n.Focus[0]
	}
	return n
}

// windowAt is the window shown at p: a floating one, top first, else the
// tiled one.
func (a *bro) windowAt(p image.Point) window {
	shown := a.wm.tree.Shown()
	for i := len(shown) - 1; i >= 0; i-- {
		if w := a.wm.wins[shown[i]]; w != nil && p.In(w.panel(a).Outer) {
			return w
		}
	}
	return nil
}

// wmKey gives a key to the window in focus.
func (a *bro) wmKey(code uint16, value int32) bool {
	if w := a.focusedWin(); w != nil {
		return w.press(a, code, value)
	}
	if value == 1 && code == kobo.KeyEnter { // an empty workspace: a window
		a.openLauncher()
		return true
	}
	return false
}

// wmTouch starts a drag on a floating window's strip.
func (a *bro) wmTouch(p image.Point, g *kobo.Gesture) bool {
	w := a.windowAt(p)
	n := a.nodeOf(w)
	if w == nil || !floatingNode(n) || !p.In(w.panel(a).Title) || kobo.Hit(p, w.panel(a).Close) || kobo.Hit(p, w.panel(a).New) {
		return false
	}
	a.wm.drag = &wmDrag{g: g, n: n, from: a.wm.tree.FloatRect(n)}
	g.Owner = ownDrag
	return true
}

// wmDragTo moves it with the finger, when it lifts: e-ink can't follow.
func (a *bro) wmDragLift(g *kobo.Gesture) bool {
	d := a.wm.drag
	if d == nil || d.g != g {
		return false
	}
	a.wm.drag = nil
	if g.Tap {
		return false
	}
	a.wm.tree.SetFloatRect(d.n, d.from.Add(g.Last.Sub(g.Start)))
	a.wm.tree.Focus(d.n)
	a.wm.laid = ""
	a.wmApply()
	return true
}

const ownDrag = 3

// setMode goes to the keys' binding mode name ("": the default).
func (a *bro) setMode(name string) {
	a.wm.mode = name
	log.Printf("wm: mode %q", name)
	if name == "" {
		a.SetStatus("")
	} else {
		a.SetStatus("mode: " + name)
	}
}
