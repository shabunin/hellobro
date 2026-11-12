package mermaid

// Flowcharts (graph, flowchart) and state diagrams, read into a graph:
// its nodes, its edges and its subgraphs. What it doesn't know (styles,
// classes, clicks) it skips.

import (
	"regexp"
	"strings"
	"unicode"
)

// node is a box of the chart, or a point an edge passes in the layout
// (dummy).
type node struct {
	id, text string
	shape    string
	sub      int // the innermost subgraph it's in; -1 none
	lab      *label
	w, h     float64

	dummy  bool
	labels int  // a dummy that's edge labels' place: the edge's index+1
	border int  // a dummy at a subgraph's left (-1) or right (1)
	set    bool // its shape or text written
	rank   int
	order  int
	x, y   float64 // the layout's: x across the ranks, y along them
}

// edge is an arrow (or a line) from a node to another.
type edge struct {
	from, to   int
	text       string
	line       byte // '-' solid, '.' dotted, '=' thick, '~' unseen
	head, tail byte // 0, '>', 'o', 'x'
	minlen     int
	lab        *label
	subFrom    int // from a subgraph (its box), not a node: its index+1
	subTo      int
}

type subgraph struct {
	id, title string
	parent    int
	lab       *label
}

type graph struct {
	dir   string // TB, BT, LR, RL
	nodes []*node
	ids   map[string]int
	edges []*edge
	subs  []*subgraph
	stack []int // the subgraphs open as it's read
}

func newGraph(dir string) *graph {
	return &graph{dir: dir, ids: map[string]int{}}
}

// direction is a header's direction, TB by default.
func direction(s string) string {
	switch strings.ToUpper(s) {
	case "LR", "RL", "BT":
		return strings.ToUpper(s)
	}
	return "TB"
}

func (g *graph) cur() int {
	if len(g.stack) == 0 {
		return -1
	}
	return g.stack[len(g.stack)-1]
}

// node is the node id, new if it's not known; in the subgraph open if
// it isn't in one.
func (g *graph) node(id string) int {
	i, ok := g.ids[id]
	if !ok {
		i = len(g.nodes)
		g.nodes = append(g.nodes, &node{id: id, text: id, shape: "rect", sub: g.cur()})
		g.ids[id] = i
	} else if g.nodes[i].sub == -1 && g.cur() >= 0 {
		g.nodes[i].sub = g.cur()
	}
	return i
}

var (
	subRE   = regexp.MustCompile(`^subgraph\s*(.*)$`)
	skipRE  = regexp.MustCompile(`^(classDef|class|style|linkStyle|click|accTitle|accDescr|title|callback|link)\b`)
	idTitle = regexp.MustCompile(`^([^\s\[]+)\s*\[(.*)\]$`)
)

func parseFlow(lines []string) *graph {
	f := strings.Fields(lines[0])
	g := newGraph("TB")
	if len(f) > 1 {
		g.dir = direction(strings.TrimSuffix(f[1], ";"))
	}
	for _, l := range lines[1:] {
		for _, st := range statements(l) {
			g.statement(st)
		}
	}
	g.resolveSubEdges()
	return g
}

// statements are a line's, cut at semicolons that aren't in a label.
func statements(l string) []string {
	var out []string
	depth, quote, from := 0, false, 0
	for i := 0; i < len(l); i++ {
		switch c := l[i]; {
		case c == '"':
			quote = !quote
		case quote:
		case c == '[' || c == '(' || c == '{':
			depth++
		case c == ']' || c == ')' || c == '}':
			depth = max(0, depth-1)
		case c == ';' && depth == 0:
			out = append(out, strings.TrimSpace(l[from:i]))
			from = i + 1
		}
	}
	out = append(out, strings.TrimSpace(l[from:]))
	return out
}

func (g *graph) statement(st string) {
	switch {
	case st == "":
	case st == "end":
		if len(g.stack) > 0 {
			g.stack = g.stack[:len(g.stack)-1]
		}
	case subRE.MatchString(st) && (len(st) == 8 || st[8] == ' '):
		rest := strings.TrimSpace(subRE.FindStringSubmatch(st)[1])
		s := &subgraph{parent: g.cur()}
		if m := idTitle.FindStringSubmatch(rest); m != nil {
			s.id, s.title = m[1], m[2]
		} else {
			s.id, s.title = rest, rest
		}
		g.subs = append(g.subs, s)
		g.stack = append(g.stack, len(g.subs)-1)
	case strings.HasPrefix(st, "direction "):
	case skipRE.MatchString(st):
	default:
		g.chain(st)
	}
}

// resolveSubEdges sends an edge to or from a subgraph to one of its
// nodes: to its first, from its last.
func (g *graph) resolveSubEdges() {
	for si, s := range g.subs {
		i, ok := g.ids[s.id]
		if !ok || len(g.edges) == 0 {
			continue
		}
		if g.nodes[i].set {
			continue // a node of its own name
		}
		var in []int
		for j, n := range g.nodes {
			if j != i && g.within(n.sub, si) {
				in = append(in, j)
			}
		}
		if len(in) == 0 {
			continue
		}
		for _, e := range g.edges {
			if e.to == i {
				e.to, e.subTo = in[0], si+1
			}
			if e.from == i {
				e.from, e.subFrom = in[len(in)-1], si+1
			}
		}
		g.nodes[i].dummy = true // gone: drawn as nothing, laid out as nothing
	}
	var keep []*node
	idx := map[int]int{}
	for i, n := range g.nodes {
		if !n.dummy {
			idx[i] = len(keep)
			keep = append(keep, n)
		}
	}
	if len(keep) == len(g.nodes) {
		return
	}
	var es []*edge
	for _, e := range g.edges {
		f, ok1 := idx[e.from]
		t, ok2 := idx[e.to]
		if ok1 && ok2 {
			e.from, e.to = f, t
			es = append(es, e)
		}
	}
	g.nodes, g.edges = keep, es
}

// within says if subgraph s is sub or in it.
func (g *graph) within(s, sub int) bool {
	for ; s >= 0; s = g.subs[s].parent {
		if s == sub {
			return true
		}
	}
	return false
}

// scan reads a statement.
type scan struct {
	s string
	i int
}

func (s *scan) space() {
	for s.i < len(s.s) && (s.s[s.i] == ' ' || s.s[s.i] == '\t') {
		s.i++
	}
}

func (s *scan) rest() string { return s.s[s.i:] }

// chain reads nodes and the edges between them: a & b --> c -- text --> d.
func (g *graph) chain(st string) {
	s := &scan{s: st}
	from := g.group(s)
	if from == nil {
		return
	}
	for {
		e := s.edge()
		if e == nil {
			return
		}
		to := g.group(s)
		if to == nil {
			return
		}
		for _, a := range from {
			for _, b := range to {
				ne := *e
				ne.from, ne.to = a, b
				g.edges = append(g.edges, &ne)
			}
		}
		from = to
	}
}

// group is nodes joined by &.
func (g *graph) group(s *scan) []int {
	var out []int
	for {
		n, ok := g.readNode(s)
		if !ok {
			return out
		}
		out = append(out, n)
		s.space()
		if !strings.HasPrefix(s.rest(), "&") {
			return out
		}
		s.i++
	}
}

// shapes are the brackets round a node's text, the longer first.
var shapes = []struct{ open, close, shape string }{
	{"(((", ")))", "dcircle"},
	{"((", "))", "circle"},
	{"([", "])", "stadium"},
	{"[[", "]]", "sub"},
	{"[(", ")]", "cyl"},
	{"{{", "}}", "hex"},
	{"[/", "/]", "para"},
	{"[/", `\]`, "trap"},
	{`[\`, `\]`, "para2"},
	{`[\`, "/]", "trap2"},
	{"(", ")", "round"},
	{"[", "]", "rect"},
	{"{", "}", "rhombus"},
	{">", "]", "asym"},
}

var atShape = map[string]string{
	"rect": "rect", "rounded": "round", "stadium": "stadium", "subproc": "sub", "fr-rect": "sub", "cyl": "cyl", "db": "cyl",
	"circle": "circle", "circ": "circle", "dbl-circ": "dcircle", "diam": "rhombus", "diamond": "rhombus", "decision": "rhombus",
	"hex": "hex", "lean-r": "para", "lean-l": "para2", "trap-b": "trap", "trap-t": "trap2", "doc": "rect", "notch-rect": "asym",
	"sm-circ": "start", "start": "start", "fr-circ": "end", "stop": "end", "fork": "fork", "join": "fork",
}

// readNode reads a node: its id, and its shape and text if they're
// written.
func (g *graph) readNode(s *scan) (int, bool) {
	s.space()
	start := s.i
	for s.i < len(s.s) {
		r := rune(s.s[s.i])
		c := s.s[s.i]
		switch {
		case c >= 0x80:
			r = []rune(s.s[s.i:])[0]
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				s.i += len(string(r))
				continue
			}
		case c == '_' || unicode.IsLetter(r) || unicode.IsDigit(r):
			s.i++
			continue
		case (c == '-' || c == '.') && s.i > start && s.i+1 < len(s.s):
			n := s.s[s.i+1]
			if n != '-' && n != '.' && n != '>' && n != '=' && n != ' ' && n != 'x' && n != 'o' || (n == 'x' || n == 'o') && s.i+2 < len(s.s) && isIDByte(s.s[s.i+2]) {
				s.i++
				continue
			}
		}
		break
	}
	id := s.s[start:s.i]
	if id == "" {
		return 0, false
	}
	n := g.node(id)
	nd := g.nodes[n]
	rest := s.rest()
	if strings.HasPrefix(rest, "@{") {
		end := strings.IndexByte(rest, '}')
		if end < 0 {
			end = len(rest) - 1
		}
		for _, kv := range strings.Split(rest[2:end], ",") {
			k, v, _ := strings.Cut(kv, ":")
			k, v = strings.TrimSpace(k), strings.Trim(strings.TrimSpace(v), `"'`)
			switch k {
			case "shape":
				if sh, ok := atShape[v]; ok {
					nd.shape, nd.set = sh, true
				}
			case "label":
				nd.text, nd.set = v, true
			}
		}
		s.i += end + 1
	} else {
		for _, sh := range shapes {
			if !strings.HasPrefix(rest, sh.open) {
				continue
			}
			text, n, ok := bracketed(rest[len(sh.open):], sh.close)
			if !ok {
				continue
			}
			nd.shape, nd.text, nd.set = sh.shape, text, true
			s.i += len(sh.open) + n
			break
		}
	}
	if strings.HasPrefix(s.rest(), ":::") { // a class
		s.i += 3
		for s.i < len(s.s) && isIDByte(s.s[s.i]) {
			s.i++
		}
	}
	return n, true
}

func isIDByte(c byte) bool {
	return c == '_' || c == '-' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80
}

// bracketed is the text up to close (brackets in it matched, a quoted
// text as it is), and how much it took with close.
func bracketed(s, close string) (string, int, bool) {
	t := strings.TrimLeft(s, " ")
	if strings.HasPrefix(t, `"`) {
		if e := strings.IndexByte(t[1:], '"'); e >= 0 {
			after := strings.TrimLeft(t[e+2:], " ")
			if strings.HasPrefix(after, close) {
				return t[1 : e+1], len(s) - len(after) + len(close), true
			}
		}
	}
	depth := 0
	for i := 0; i < len(s); i++ {
		if depth == 0 && strings.HasPrefix(s[i:], close) {
			if close == ")" && strings.HasPrefix(s[i:], "))") && i+2 <= len(s) && !strings.Contains(s[i+2:], ")") {
				// "(f(x))": its last ) closes
			}
			return strings.TrimSpace(s[:i]), i + len(close), true
		}
		switch s[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
			if depth < 0 {
				return "", 0, false
			}
		}
	}
	return "", 0, false
}

var (
	linkRE = regexp.MustCompile(`^(<|o|x)?(-{3,}|-{2,}>|-{2,}[ox]|={3,}|={2,}[>ox]|-\.+->?|-\.+-[ox]|~{3,})`)
	pipeRE = regexp.MustCompile(`^\|("[^"]*"|[^|]*)\|`)
	openRE = regexp.MustCompile(`^(<|o|x)?(--|==|-\.)`)
	closes = map[string]*regexp.Regexp{
		"--": regexp.MustCompile(`-{2,}>|-{3,}|--[ox]\b`),
		"==": regexp.MustCompile(`={2,}>|={3,}|==[ox]\b`),
		"-.": regexp.MustCompile(`\.+->|\.+-[ox]?`),
	}
)

// edge reads an edge: --> --- -.-> ==> ~~~, its text written in it or
// between bars.
func (s *scan) edge() *edge {
	s.space()
	rest := s.rest()
	e := &edge{line: '-', minlen: 1}
	var body string
	if m := linkRE.FindStringSubmatch(rest); m != nil && !(len(m[2]) > 0 && strings.ContainsAny(m[2][len(m[2])-1:], "ox") && len(rest) > len(m[0]) && isIDByte(rest[len(m[0])])) {
		s.i += len(m[0])
		body = m[2]
		if m[1] != "" {
			e.tail = tailOf(m[1][0])
		}
	} else if m := openRE.FindStringSubmatch(rest); m != nil {
		after := rest[len(m[0]):]
		loc := closes[m[2]].FindStringIndex(after)
		if loc == nil {
			return nil
		}
		e.text = strings.TrimSpace(after[:loc[0]])
		body = m[2] + after[loc[0]:loc[1]]
		if m[2] == "-." {
			body = "-" + after[loc[0]:loc[1]]
		}
		if m[1] != "" {
			e.tail = tailOf(m[1][0])
		}
		s.i += len(m[0]) + loc[1]
	} else {
		return nil
	}
	switch last := body[len(body)-1]; last {
	case '>', 'o', 'x':
		e.head = last
		body = body[:len(body)-1]
	}
	switch {
	case strings.Contains(body, "."):
		e.line = '.'
		e.minlen = strings.Count(body, ".")
	case strings.HasPrefix(body, "="):
		e.line = '='
		e.minlen = len(body) - 1
		if e.head == 0 {
			e.minlen = len(body) - 2
		}
	case strings.HasPrefix(body, "~"):
		e.line = '~'
		e.minlen = len(body) - 2
	default:
		e.minlen = len(body) - 1
		if e.head == 0 {
			e.minlen = len(body) - 2
		}
	}
	e.minlen = min(3, max(1, e.minlen))
	s.space()
	if m := pipeRE.FindStringSubmatch(s.rest()); m != nil {
		e.text = strings.TrimSpace(m[1])
		s.i += len(m[0])
	}
	e.text = strings.Trim(e.text, `"`)
	return e
}

func tailOf(c byte) byte {
	if c == '<' {
		return '>'
	}
	return c
}

// State diagrams: states, [*] at the start and the end, transitions with
// their labels after a colon, composite states as subgraphs.

var (
	stateAs    = regexp.MustCompile(`^state\s+"([^"]*)"\s+as\s+(\S+)\s*\{?$`)
	stateSt    = regexp.MustCompile(`^state\s+(\S+?)\s*(<<(\w+)>>)?\s*(\{)?$`)
	stateTrans = regexp.MustCompile(`^(\S+)\s*-->\s*([^:]+?)\s*(?::\s*(.*))?$`)
	stateDesc  = regexp.MustCompile(`^([^\s:]+)\s*:\s*(.*)$`)
	noteRE     = regexp.MustCompile(`^note\s+(left|right)\s+of\s+[^:]+(:.*)?$`)
)

func parseState(lines []string) *graph {
	g := newGraph("TB")
	inNote := false
	starts := map[int]int{}
	ends := map[int]int{}
	state := func(id string, src bool) int {
		id = strings.TrimSuffix(strings.TrimSpace(id), ";")
		if i := strings.Index(id, ":::"); i > 0 {
			id = id[:i]
		}
		if id != "[*]" {
			n := g.node(id)
			if nd := g.nodes[n]; nd.shape == "rect" {
				nd.shape = "round"
			}
			return n
		}
		m, kind := ends, "end"
		if src {
			m, kind = starts, "start"
		}
		if n, ok := m[g.cur()]; ok {
			return n
		}
		n := g.node(kind + "\x00" + strings.Repeat("·", len(m)) + g.curID())
		g.nodes[n].shape, g.nodes[n].text = kind, ""
		m[g.cur()] = n
		return n
	}
	for _, l := range lines[1:] {
		switch {
		case inNote:
			inNote = !strings.HasPrefix(l, "end note")
		case strings.HasPrefix(l, "note "):
			inNote = !noteRE.MatchString(l) || !strings.Contains(l, ":")
		case strings.HasPrefix(l, "direction "):
			g.dir = direction(strings.TrimPrefix(l, "direction "))
		case l == "}":
			if len(g.stack) > 0 {
				g.stack = g.stack[:len(g.stack)-1]
			}
		case l == "--" || skipRE.MatchString(l) || strings.HasPrefix(l, "hide "):
		case stateAs.MatchString(l):
			m := stateAs.FindStringSubmatch(l)
			if strings.HasSuffix(l, "{") {
				g.subs = append(g.subs, &subgraph{id: m[2], title: m[1], parent: g.cur()})
				g.stack = append(g.stack, len(g.subs)-1)
				continue
			}
			n := state(m[2], false)
			g.nodes[n].text, g.nodes[n].set = m[1], true
		case stateSt.MatchString(l):
			m := stateSt.FindStringSubmatch(l)
			if m[4] == "{" {
				g.subs = append(g.subs, &subgraph{id: m[1], title: m[1], parent: g.cur()})
				g.stack = append(g.stack, len(g.subs)-1)
				continue
			}
			n := state(m[1], false)
			switch m[3] {
			case "fork", "join":
				g.nodes[n].shape, g.nodes[n].text, g.nodes[n].set = "fork", "", true
			case "choice":
				g.nodes[n].shape, g.nodes[n].text, g.nodes[n].set = "choice", "", true
			}
		case stateTrans.MatchString(l):
			m := stateTrans.FindStringSubmatch(l)
			a, b := state(m[1], true), state(m[2], false)
			g.edges = append(g.edges, &edge{from: a, to: b, text: m[3], line: '-', head: '>', minlen: 1})
		case stateDesc.MatchString(l):
			m := stateDesc.FindStringSubmatch(l)
			n := state(m[1], false)
			g.nodes[n].set = true
			if nd := g.nodes[n]; nd.text == nd.id {
				nd.text = "**" + nd.id + "**<br>" + m[2]
			} else {
				nd.text += "<br>" + m[2]
			}
		default:
			if f := strings.Fields(l); len(f) == 1 {
				state(f[0], false)
			}
		}
	}
	g.resolveSubEdges()
	return g
}

func (g *graph) curID() string {
	if c := g.cur(); c >= 0 {
		return g.subs[c].id
	}
	return ""
}
