package wm

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Host is what a command needs of the app: closing windows, running
// programs, modes, and its own commands.
type Host interface {
	// Kill asks a window to close; it's Removed once it has.
	Kill(n *Node)
	// Exec runs a program (the app says where, and if at all).
	Exec(cmd string) error
	// Mode goes to the binding mode named.
	Mode(name string) error
	// Command runs a command the tree doesn't know (exit, restart, the
	// app's own), on n; false if the app doesn't know it either.
	Command(n *Node, name string, args []string) (bool, error)
}

// Result is one command's outcome, as i3's IPC returns it.
type Result struct {
	Success    bool   `json:"success"`
	ParseError bool   `json:"parse_error,omitempty"`
	Error      string `json:"error,omitempty"`
}

// Run runs a line of i3 commands: "[criteria] command args, command;
// command". Commands after a comma act on the criteria's windows too,
// after a semicolon on the focus again. Each command has its result.
func (t *Tree) Run(line string, h Host) []Result {
	var out []Result
	var crit *criteria
	for _, s := range segments(line) {
		body := strings.TrimSpace(s.body)
		if strings.HasPrefix(body, "[") {
			end := closing(body)
			if end < 0 {
				out = append(out, Result{ParseError: true, Error: "criteria without a ]"})
				break
			}
			c, err := parseCriteria(body[1:end])
			if err != nil {
				out = append(out, Result{ParseError: true, Error: err.Error()})
				break
			}
			crit = c
			body = strings.TrimSpace(body[end+1:])
		}
		if body != "" {
			err := t.run(body, crit, h)
			var pe parseError
			switch {
			case errors.As(err, &pe):
				out = append(out, Result{ParseError: true, Error: err.Error()})
			case err != nil:
				out = append(out, Result{Error: err.Error()})
			default:
				out = append(out, Result{Success: true})
			}
		}
		if s.sep == ';' {
			crit = nil
		}
	}
	return out
}

type parseError struct{ msg string }

func (e parseError) Error() string { return e.msg }

func bad(f string, a ...any) error { return parseError{fmt.Sprintf(f, a...)} }

type segment struct {
	body string
	sep  byte // what ended it: ',' ';' or 0 at the end
}

// segments cuts a line at the commas and semicolons outside quotes and
// criteria.
func segments(line string) []segment {
	var out []segment
	start, quote, bracket := 0, false, false
	for i := 0; i < len(line); i++ {
		switch c := line[i]; {
		case c == '\\' && quote:
			i++
		case c == '"':
			quote = !quote
		case quote:
		case c == '[':
			bracket = true
		case c == ']':
			bracket = false
		case !bracket && (c == ',' || c == ';'):
			out = append(out, segment{line[start:i], c})
			start = i + 1
		}
	}
	return append(out, segment{line[start:], 0})
}

// closing is the index of the ] that closes s's [, quotes skipped.
func closing(s string) int {
	quote := false
	for i := 1; i < len(s); i++ {
		switch {
		case s[i] == '\\' && quote:
			i++
		case s[i] == '"':
			quote = !quote
		case s[i] == ']' && !quote:
			return i
		}
	}
	return -1
}

type word struct {
	s      string
	at     int // where it starts in the command
	quoted bool
}

// words splits a command at spaces, "quoted words" kept whole.
func words(s string) []word {
	var out []word
	for i := 0; i < len(s); {
		if s[i] == ' ' || s[i] == '\t' {
			i++
			continue
		}
		w := word{at: i}
		if s[i] == '"' {
			w.quoted = true
			var b strings.Builder
			for i++; i < len(s) && s[i] != '"'; i++ {
				if s[i] == '\\' && i+1 < len(s) {
					i++
				}
				b.WriteByte(s[i])
			}
			i++
			w.s = b.String()
		} else {
			j := strings.IndexAny(s[i:], " \t")
			if j < 0 {
				j = len(s) - i
			}
			w.s = s[i : i+j]
			i += j
		}
		out = append(out, w)
	}
	return out
}

// rest is the command from its word i on: as it was written, or the word
// if it's quoted and alone.
func rest(cmd string, ws []word, i int) string {
	if i >= len(ws) {
		return ""
	}
	if ws[i].quoted && i == len(ws)-1 {
		return ws[i].s
	}
	return strings.TrimSpace(cmd[ws[i].at:])
}

// criteria pick windows: i3's [class="..." title="..." con_mark=...].
type criteria struct {
	class, instance, title, workspace, mark *regexp.Regexp
	conID                                   int64
	focusedClass, focusedInstance           bool
	focusedWS, focusedID                    bool
	floating, tiling, urgent, all           bool
}

func parseCriteria(s string) (*criteria, error) {
	c := &criteria{}
	for _, w := range criteriaWords(s) {
		k, v, _ := strings.Cut(w, "=")
		var err error
		re := func() *regexp.Regexp {
			r, e := regexp.Compile(v)
			if e != nil {
				err = bad("bad %s %q: %v", k, v, e)
			}
			return r
		}
		switch k {
		case "class", "app_id":
			if v == "__focused__" {
				c.focusedClass = true
			} else {
				c.class = re()
			}
		case "instance":
			if v == "__focused__" {
				c.focusedInstance = true
			} else {
				c.instance = re()
			}
		case "title":
			c.title = re()
		case "workspace":
			if v == "__focused__" {
				c.focusedWS = true
			} else {
				c.workspace = re()
			}
		case "con_mark":
			c.mark = re()
		case "con_id", "id":
			if v == "__focused__" {
				c.focusedID = true
			} else if c.conID, err = strconv.ParseInt(v, 10, 64); err != nil {
				err = bad("bad %s %q", k, v)
			}
		case "floating":
			c.floating = true
		case "tiling":
			c.tiling = true
		case "urgent":
			c.urgent = true
		case "all":
			c.all = true
		default:
			return nil, bad("unknown criterion %q", k)
		}
		if err != nil {
			return nil, err
		}
	}
	return c, nil
}

// criteriaWords splits key=value pairs, quoted values unquoted.
func criteriaWords(s string) []string {
	var out []string
	var b strings.Builder
	quote, any := false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\\' && quote && i+1 < len(s):
			i++
			b.WriteByte(s[i])
		case c == '"':
			quote, any = !quote, true
		case !quote && (c == ' ' || c == '\t'):
			if any || b.Len() > 0 {
				out = append(out, b.String())
			}
			b.Reset()
			any = false
		default:
			b.WriteByte(c)
		}
	}
	if any || b.Len() > 0 {
		out = append(out, b.String())
	}
	return out
}

// match is the containers c picks: windows, unless it asks for a
// container by its id or mark.
func (t *Tree) match(c *criteria) []*Node {
	f := t.focused
	return t.Find(func(n *Node) bool {
		if n.Type != Con {
			return false
		}
		if !n.leaf() && c.conID == 0 && !c.focusedID && c.mark == nil {
			return false
		}
		w := n.Window
		str := func(re *regexp.Regexp, s func(Window) string) bool {
			return re == nil || w != nil && re.MatchString(s(w))
		}
		switch {
		case !str(c.class, Window.Class), !str(c.instance, Window.Instance), !str(c.title, Window.Title):
			return false
		case c.focusedClass && (w == nil || f.Window == nil || w.Class() != f.Window.Class()):
			return false
		case c.focusedInstance && (w == nil || f.Window == nil || w.Instance() != f.Window.Instance()):
			return false
		case c.workspace != nil && !c.workspace.MatchString(workspaceOf(n).Name):
			return false
		case c.focusedWS && workspaceOf(n) != t.Current():
			return false
		case c.mark != nil && !slices.ContainsFunc(n.Marks, c.mark.MatchString):
			return false
		case c.conID != 0 && n.ID != c.conID, c.focusedID && n != f:
			return false
		case c.floating && !floating(n), c.tiling && floating(n), c.urgent && !n.Urgent:
			return false
		}
		return true
	})
}

// run runs one command on crit's windows, or the focus.
func (t *Tree) run(cmd string, crit *criteria, h Host) error {
	ws := words(cmd)
	targets := []*Node{t.focused}
	if crit != nil {
		targets = t.match(crit)
	}
	arg := func(i int) string {
		if i < len(ws) {
			return ws[i].s
		}
		return ""
	}
	each := func(f func(n *Node) error) error {
		var err error
		for _, n := range targets {
			if e := f(n); e != nil {
				err = e
			}
		}
		return err
	}
	switch name := arg(0); name {
	case "nop":
		return nil
	case "exec":
		i := 1
		if arg(1) == "--no-startup-id" {
			i = 2
		}
		c := rest(cmd, ws, i)
		if c == "" {
			return bad("exec what?")
		}
		return h.Exec(c)
	case "mode":
		return h.Mode(rest(cmd, ws, 1))
	case "kill":
		return each(func(n *Node) error {
			for _, l := range t.find(n, (*Node).leaf) {
				h.Kill(l)
			}
			return nil
		})
	case "focus":
		return t.focusCmd(ws, arg, targets, crit != nil)
	case "split":
		var o Layout
		switch arg(1) {
		case "h", "horizontal":
			o = SplitH
		case "v", "vertical":
			o = SplitV
		case "t", "toggle":
			return each(func(n *Node) error { t.SplitToggle(n); return nil })
		default:
			return bad("split h, v or toggle")
		}
		return each(func(n *Node) error { t.Split(n, o); return nil })
	case "layout":
		return t.layoutCmd(ws, arg, each)
	case "floating":
		return each(func(n *Node) error {
			switch arg(1) {
			case "enable":
				t.SetFloating(n, true)
			case "disable":
				t.SetFloating(n, false)
			case "toggle":
				t.ToggleFloating(n)
			default:
				return bad("floating enable, disable or toggle")
			}
			return nil
		})
	case "fullscreen":
		return each(func(n *Node) error {
			switch arg(1) {
			case "enable":
				t.SetFullscreen(n, true)
			case "disable":
				t.SetFullscreen(n, false)
			case "", "toggle":
				t.ToggleFullscreen(n)
			default:
				return bad("fullscreen enable, disable or toggle")
			}
			return nil
		})
	case "border":
		return each(func(n *Node) error {
			b, ok := map[string]Border{"normal": BorderNormal, "pixel": BorderPixel, "none": BorderNone}[arg(1)]
			if arg(1) == "toggle" {
				b, ok = (n.Border+1)%3, true
			}
			if !ok {
				return bad("border normal, pixel, none or toggle")
			}
			for _, l := range t.find(n, (*Node).leaf) {
				l.Border = b
			}
			return nil
		})
	case "workspace":
		return t.workspaceCmd(cmd, ws, arg)
	case "rename":
		return t.renameCmd(cmd, ws, arg)
	case "move":
		return t.moveCmd(cmd, ws, arg, each)
	case "swap":
		// swap container with id|con_id|mark X
		i := 1
		for arg(i) == "container" || arg(i) == "window" || arg(i) == "with" {
			i++
		}
		var other []*Node
		switch arg(i) {
		case "id", "con_id":
			id, err := strconv.ParseInt(arg(i+1), 10, 64)
			if err != nil {
				return bad("swap with id what?")
			}
			if n := t.ByID(id); n != nil {
				other = []*Node{n}
			}
		case "mark":
			other = t.Find(func(n *Node) bool { return slices.Contains(n.Marks, arg(i+1)) })
		default:
			return bad("swap container with id or mark")
		}
		if len(other) == 0 {
			return errors.New("nothing to swap with")
		}
		return each(func(n *Node) error { t.Swap(n, other[0]); return nil })
	case "resize":
		return t.resizeCmd(ws, arg, each)
	case "mark":
		add, toggle := false, false
		i := 1
		for ; strings.HasPrefix(arg(i), "--"); i++ {
			switch arg(i) {
			case "--add":
				add = true
			case "--replace":
				add = false
			case "--toggle":
				toggle = true
			}
		}
		m := arg(i)
		if m == "" {
			return bad("mark what?")
		}
		return each(func(n *Node) error { t.Mark(n, m, add, toggle); return nil })
	case "unmark":
		if crit == nil {
			t.Unmark(nil, arg(1))
			return nil
		}
		return each(func(n *Node) error { t.Unmark(n, arg(1)); return nil })
	case "scratchpad":
		if arg(1) != "show" {
			return bad("scratchpad show")
		}
		if crit == nil {
			t.ScratchpadShow(nil)
			return nil
		}
		return each(func(n *Node) error {
			if !t.ScratchpadShow(n) {
				return errors.New("not a scratchpad window")
			}
			return nil
		})
	default:
		var args []string
		for _, w := range ws[1:] {
			args = append(args, w.s)
		}
		return each(func(n *Node) error {
			ok, err := h.Command(n, name, args)
			if !ok {
				return bad("unknown command %q", name)
			}
			return err
		})
	}
}

func (t *Tree) focusCmd(ws []word, arg func(int) string, targets []*Node, crit bool) error {
	if arg(1) == "" {
		if !crit {
			return bad("focus what?")
		}
		if len(targets) == 0 {
			return errors.New("no window matches")
		}
		t.Focus(targets[len(targets)-1])
		return nil
	}
	if d, ok := dirs[arg(1)]; ok {
		t.FocusDir(d)
		return nil
	}
	switch arg(1) {
	case "parent":
		t.FocusParent()
	case "child":
		t.FocusChild()
	case "floating":
		t.FocusFloating()
	case "tiling":
		t.FocusTiling()
	case "mode_toggle":
		t.FocusModeToggle()
	case "output":
	default:
		return bad("focus left, right, up, down, parent, child, floating, tiling or mode_toggle")
	}
	return nil
}

var dirs = map[string]Dir{"left": Left, "right": Right, "up": Up, "down": Down}

var layouts = map[string]Layout{
	"splith": SplitH, "splitv": SplitV, "stacked": Stacked, "stacking": Stacked, "tabbed": Tabbed,
}

func (t *Tree) layoutCmd(ws []word, arg func(int) string, each func(func(*Node) error) error) error {
	if arg(1) == "toggle" {
		how := arg(2)
		var list []Layout
		if len(ws) > 3 || how != "" && how != "split" && how != "all" {
			for _, w := range ws[2:] {
				l, ok := layouts[w.s]
				if w.s == "split" {
					l, ok = SplitH, true
				}
				if !ok {
					return bad("layout toggle %q?", w.s)
				}
				list = append(list, l)
			}
			how = "list"
		}
		return each(func(n *Node) error { t.ToggleLayout(n, how, list...); return nil })
	}
	if arg(1) == "default" {
		return each(func(n *Node) error {
			p := n
			if n.Type != Workspace {
				p = n.Parent
			}
			t.SetLayout(n, p.LastSplit)
			return nil
		})
	}
	l, ok := layouts[arg(1)]
	if !ok {
		return bad("layout splith, splitv, stacking, tabbed, default or toggle")
	}
	return each(func(n *Node) error { t.SetLayout(n, l); return nil })
}

// wsName reads a workspace's name from word i: "number N", next,
// prev, back_and_forth, or a name.
func (t *Tree) wsName(cmd string, ws []word, arg func(int) string, i int) (string, error) {
	switch arg(i) {
	case "number":
		n, err := strconv.Atoi(strings.Fields(arg(i+1) + " x")[0])
		if err != nil {
			return "", bad("workspace number what?")
		}
		if w := t.WorkspaceByNum(n); w != nil {
			return w.Name, nil
		}
		return rest(cmd, ws, i+1), nil
	case "next", "next_on_output", "prev", "prev_on_output":
		all := t.content.Nodes
		j := slices.Index(all, t.Current())
		d := 1
		if strings.HasPrefix(arg(i), "prev") {
			d = -1
		}
		return all[(j+d+len(all))%len(all)].Name, nil
	case "back_and_forth":
		if t.prevWS == "" {
			return t.Current().Name, nil
		}
		return t.prevWS, nil
	case "current":
		return t.Current().Name, nil
	case "":
		return "", bad("which workspace?")
	}
	return rest(cmd, ws, i), nil
}

func (t *Tree) workspaceCmd(cmd string, ws []word, arg func(int) string) error {
	i := 1
	for arg(i) == "--no-auto-back-and-forth" {
		i++
	}
	name, err := t.wsName(cmd, ws, arg, i)
	if err != nil {
		return err
	}
	if strings.HasPrefix(name, "__") {
		return errors.New("can't go to " + strconv.Quote(name))
	}
	t.ShowWorkspace(name)
	return nil
}

// rename workspace [OLD] to NEW
func (t *Tree) renameCmd(cmd string, ws []word, arg func(int) string) error {
	if arg(1) != "workspace" {
		return bad("rename workspace [old] to new")
	}
	from, i := "", 2
	if arg(2) != "to" {
		from, i = arg(2), 3
	}
	if arg(i) != "to" || arg(i+1) == "" {
		return bad("rename workspace [old] to new")
	}
	return t.RenameWorkspace(from, rest(cmd, ws, i+1))
}

func (t *Tree) moveCmd(cmd string, ws []word, arg func(int) string, each func(func(*Node) error) error) error {
	i := 1
	if d, ok := dirs[arg(1)]; ok {
		a, err := amount(ws, 2, Amount{Px: 10})
		if err != nil {
			return err
		}
		return each(func(n *Node) error {
			if a.Px == 0 && floating(n) {
				a.Px = t.floatPx(a, d.horizontal())
			}
			t.Move(n, d, a.Px)
			return nil
		})
	}
	for arg(i) == "window" || arg(i) == "container" || arg(i) == "--no-auto-back-and-forth" {
		i++
	}
	if arg(i) == "to" {
		i++
	}
	switch arg(i) {
	case "workspace":
		name, err := t.wsName(cmd, ws, arg, i+1)
		if err != nil {
			return err
		}
		return each(func(n *Node) error { t.MoveToWorkspace(n, name); return nil })
	case "scratchpad":
		return each(func(n *Node) error { t.MoveToScratchpad(n); return nil })
	case "mark":
		m := arg(i + 1)
		to := t.Find(func(n *Node) bool { return slices.Contains(n.Marks, m) })
		if len(to) == 0 {
			return errors.New("no mark " + strconv.Quote(m))
		}
		return each(func(n *Node) error { return t.MoveToMark(n, to[0]) })
	case "absolute", "position":
		if arg(i) == "absolute" {
			i++
		}
		if arg(i+1) == "center" {
			return each(func(n *Node) error { t.CenterFloating(n); return nil })
		}
		x, errx := strconv.Atoi(arg(i + 1))
		j := i + 2
		if arg(j) == "px" {
			j++
		}
		y, erry := strconv.Atoi(arg(j))
		if errx != nil || erry != nil {
			return bad("move position X px Y px, or center")
		}
		return each(func(n *Node) error {
			r := t.FloatRect(n)
			if r.Empty() {
				return errors.New("not floating")
			}
			t.SetFloatRect(n, r.Sub(r.Min).Add(t.content.Rect.Min).Add(pt(x, y)))
			return nil
		})
	}
	return bad("move left, right, up, down, to workspace, scratchpad, mark or position")
}

// amount reads "N [px|ppt]" from word i, or def.
func amount(ws []word, i int, def Amount) (Amount, error) {
	if i >= len(ws) {
		return def, nil
	}
	n, err := strconv.Atoi(ws[i].s)
	if err != nil {
		return def, bad("how much? %q", ws[i].s)
	}
	if i+1 < len(ws) && ws[i+1].s == "ppt" {
		return Amount{Ppt: n}, nil
	}
	return Amount{Px: n}, nil
}

// resize grow|shrink width|height|left|… [N px] [or N ppt]
// resize set [width] W [px|ppt] [[height] H [px|ppt]]
func (t *Tree) resizeCmd(ws []word, arg func(int) string, each func(func(*Node) error) error) error {
	switch arg(1) {
	case "grow", "shrink":
		var a Amount
		have := false
		for i := 3; i < len(ws); i++ {
			if n, err := strconv.Atoi(arg(i)); err == nil {
				have = true
				if arg(i+1) == "ppt" {
					a.Ppt = n
				} else {
					a.Px = n
				}
			}
		}
		if !have {
			a = Amount{Px: 10, Ppt: 10}
		}
		if arg(1) == "shrink" {
			a.Px, a.Ppt = -a.Px, -a.Ppt
		}
		what := arg(2)
		return each(func(n *Node) error {
			b := a
			if floating(n) && b.Px != 0 {
				b.Ppt = 0
			} else if !floating(n) && b.Ppt != 0 {
				b.Px = 0
			}
			switch what {
			case "width", "height":
				return t.ResizeSize(n, what == "width", b)
			}
			if d, ok := dirs[what]; ok {
				return t.ResizeDir(n, d, b)
			}
			return bad("resize %s width, height, left, right, up or down", arg(1))
		})
	case "set":
		var w, h Amount
		i := 2
		read := func(into *Amount) error {
			n, err := strconv.Atoi(arg(i))
			if err != nil {
				return bad("resize set how much? %q", arg(i))
			}
			i++
			switch arg(i) {
			case "ppt":
				*into = Amount{Ppt: n}
				i++
			case "px":
				*into = Amount{Px: n}
				i++
			default:
				*into = Amount{Px: n}
			}
			return nil
		}
		if arg(i) == "width" {
			i++
		}
		if arg(i) != "height" && arg(i) != "" {
			if err := read(&w); err != nil {
				return err
			}
		}
		if arg(i) == "height" {
			i++
		}
		if arg(i) != "" {
			if err := read(&h); err != nil {
				return err
			}
		}
		return each(func(n *Node) error { return t.ResizeSet(n, w, h) })
	}
	return bad("resize grow, shrink or set")
}
