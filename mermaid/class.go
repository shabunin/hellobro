package mermaid

// Class diagrams and entity-relationship diagrams: a graph whose nodes
// are records, a box of rows (a class's name, its attributes, its
// methods; an entity's name and its attributes in columns), laid out
// as a flowchart is. The lines end as UML's (a triangle, diamonds, an
// open arrow) or in crow's feet, with what's written at their ends.

import (
	"regexp"
	"strings"

	"golang.org/x/image/font"

	"hellobro/md"
)

// record is what a record node holds: its head (the class's
// annotation and name), and sections of rows, each row of cells (one,
// or an entity's columns).
type record struct {
	annot, name string
	sections    [][][]string
	cols        bool // its rows in columns, an entity's

	headLabs []*label
	labs     [][][]*label
	colW     []float64
	rowH     float64
}

// recordNode is the graph's node id, a record now.
func (g *graph) recordNode(id string) *node {
	n := g.nodes[g.node(id)]
	if n.rec == nil {
		n.rec = &record{name: generic(id)}
		n.shape = "record"
	}
	return n
}

var (
	classOpen   = regexp.MustCompile(`^class\s+([^\s{\[]+)\s*(\["([^"]*)"\])?\s*(\{)?\s*(\})?$`)
	classRel    = regexp.MustCompile(`^(\S+?)\s*(?:"([^"]*)"\s*)?(<\||\*|o|<)?(--|\.\.)(\|>|\*|o|>)?\s*(?:"([^"]*)"\s*)?(\S+)\s*(?::\s*(.*))?$`)
	classMember = regexp.MustCompile(`^([^\s:]+)\s*:\s*(.+)$`)
	annotRE     = regexp.MustCompile(`^<<\s*([^>]+?)\s*>>\s*(\S+)?$`)
	genericRE   = regexp.MustCompile(`~([^~]*)~`)
)

// generic writes a class's ~T~ as <T>.
func generic(s string) string { return genericRE.ReplaceAllString(s, "<$1>") }

func parseClass(lines []string) *graph {
	g := newGraph("TB")
	var in *node // the class whose body is open
	for _, l := range lines[1:] {
		l = strings.TrimSuffix(l, ";")
		if in != nil {
			switch {
			case l == "}":
				in = nil
			case annotRE.MatchString(l):
				in.rec.annot = annotRE.FindStringSubmatch(l)[1]
			default:
				in.rec.member(l)
			}
			continue
		}
		switch {
		case strings.HasPrefix(l, "direction "):
			g.dir = direction(strings.TrimSpace(l[10:]))
		case classOpen.MatchString(l): // before skipRE, which has "class" (a flowchart's)
			m := classOpen.FindStringSubmatch(l)
			n := g.recordNode(m[1])
			if m[3] != "" {
				n.rec.name = m[3]
			}
			if m[4] != "" && m[5] == "" {
				in = n
			}
		case strings.HasPrefix(l, "namespace "), l == "}", strings.HasPrefix(l, "note"), skipRE.MatchString(l),
			strings.HasPrefix(l, "cssClass"), strings.HasPrefix(l, "style"):
		case annotRE.MatchString(l):
			if m := annotRE.FindStringSubmatch(l); m[2] != "" {
				g.recordNode(m[2]).rec.annot = m[1]
			}
		case classRel.MatchString(l):
			m := classRel.FindStringSubmatch(l)
			e := &edge{from: g.nodeIdx(g.recordNode(m[1])), to: g.nodeIdx(g.recordNode(m[7])), line: '-', minlen: 1,
				text: strings.TrimSpace(m[8]), fromText: m[2], toText: m[6]}
			if m[4] == ".." {
				e.line = '.'
			}
			e.tail, e.head = umlEnd(m[3]), umlEnd(m[5])
			g.edges = append(g.edges, e)
		case classMember.MatchString(l):
			m := classMember.FindStringSubmatch(l)
			g.recordNode(m[1]).rec.member(m[2])
		default:
			if f := strings.Fields(l); len(f) == 1 && isIDByte(f[0][0]) {
				g.recordNode(f[0])
			}
		}
	}
	for _, n := range g.nodes {
		if n.rec != nil && !n.rec.cols && len(n.rec.sections) == 0 {
			n.rec.sections = [][][]string{nil, nil} // UML's empty boxes
		}
	}
	return g
}

func (g *graph) nodeIdx(n *node) int { return g.ids[n.id] }

// member puts a class's member in its section: a method (it has
// brackets) in the second, an attribute in the first.
func (r *record) member(s string) {
	s = generic(strings.TrimSpace(s))
	if s == "" {
		return
	}
	for len(r.sections) < 2 {
		r.sections = append(r.sections, nil)
	}
	k := 0
	if strings.Contains(s, "(") {
		k = 1
	}
	r.sections[k] = append(r.sections[k], []string{s})
}

// umlEnd is the head a class relation's end marker draws.
func umlEnd(m string) byte {
	switch m {
	case "<|", "|>":
		return 't'
	case "*":
		return 'D'
	case "o":
		return 'd'
	case "<", ">":
		return 'v'
	}
	return 0
}

var (
	erRel    = regexp.MustCompile(`^("[^"]+"|[^\s"]+)\s*([|}][|o]|[|o][|}])(--|\.\.)([|o][|{]|[|{][|o])\s*("[^"]+"|[^\s"]+)\s*(?::\s*(.*))?$`)
	erOpen   = regexp.MustCompile(`^("[^"]+"|[^\s"{\[]+)\s*(\[\s*"?([^"\]]*)"?\s*\])?\s*\{\s*(\})?$`)
	erAttrRE = regexp.MustCompile(`^(\S+)\s+(\S+)((?:\s+(?:PK|FK|UK)\s*,?)*)\s*("([^"]*)")?$`)
)

func parseER(lines []string) *graph {
	g := newGraph("TB")
	ent := func(id string) *node {
		n := g.recordNode(strings.Trim(id, `"`))
		n.rec.cols = true
		return n
	}
	var in *node
	for _, l := range lines[1:] {
		if in != nil {
			if l == "}" {
				in = nil
				continue
			}
			if m := erAttrRE.FindStringSubmatch(l); m != nil {
				keys := strings.Join(strings.Fields(strings.ReplaceAll(m[3], ",", " ")), ", ")
				row := []string{generic(m[1]), m[2], keys, m[5]}
				if len(in.rec.sections) == 0 {
					in.rec.sections = append(in.rec.sections, nil)
				}
				in.rec.sections[0] = append(in.rec.sections[0], row)
			}
			continue
		}
		switch {
		case strings.HasPrefix(l, "direction "):
			g.dir = direction(strings.TrimSpace(l[10:]))
		case erRel.MatchString(l):
			m := erRel.FindStringSubmatch(l)
			e := &edge{from: g.nodeIdx(ent(m[1])), to: g.nodeIdx(ent(m[5])), line: '-', minlen: 1, text: clean(m[6])}
			if m[3] == ".." {
				e.line = '.'
			}
			e.tail, e.head = crowEnd(m[2]), crowEnd(m[4])
			g.edges = append(g.edges, e)
		case erOpen.MatchString(l):
			m := erOpen.FindStringSubmatch(l)
			n := ent(m[1])
			if m[3] != "" {
				n.rec.name = m[3]
			}
			if m[4] == "" {
				in = n
			}
		case skipRE.MatchString(l), strings.HasPrefix(l, "style"):
		default:
			if f := strings.Fields(l); len(f) == 1 && isIDByte(f[0][0]) {
				ent(f[0])
			}
		}
	}
	return g
}

// crowEnd is the head an ER relation's cardinality draws, written
// either way round: exactly one, zero or one, one or more, zero or
// more.
func crowEnd(m string) byte {
	switch m {
	case "||":
		return '1'
	case "|o", "o|":
		return 'z'
	case "}|", "|{":
		return 'M'
	case "}o", "o{":
		return 'm'
	}
	return 0
}

// sizeRecord lays out n's record: its head over its sections, each a
// rule under the one before; an entity's cells in columns.
func (d *drawer) sizeRecord(n *node) {
	r := n.rec
	em := d.em
	r.headLabs = nil
	if r.annot != "" {
		r.headLabs = append(r.headLabs, d.plain("«"+r.annot+"»", md.Italic))
	}
	r.headLabs = append(r.headLabs, d.plain(r.name, md.Bold))
	w, h := 0.0, em*0.5
	for _, lb := range r.headLabs {
		w, h = max(w, lb.w), h+lb.h
	}
	r.labs, r.colW = nil, nil
	r.rowH = d.plain("Mg", 0).h + em*0.25
	for _, sec := range r.sections {
		var ls [][]*label
		for _, row := range sec {
			var cells []*label
			for c, t := range row {
				st := md.Style(0)
				if r.cols && c == 0 {
					st = md.Italic
				}
				if !r.cols { // UML's classifiers: abstract (*) in italics; static ($) has no underline here
					if i := strings.LastIndexAny(t, "*$"); i >= 0 && (i == len(t)-1 || t[i+1] == ' ') && (i == 0 || t[i-1] == ')' || t[i-1] != ' ') {
						if t[i] == '*' {
							st = md.Italic
						}
						t = t[:i] + t[i+1:]
					}
				}
				lb := d.plain(t, st)
				if r.cols && c == 2 && t != "" {
					lb = d.plain(t, md.Bold)
				}
				cells = append(cells, lb)
				for len(r.colW) <= c {
					r.colW = append(r.colW, 0)
				}
				r.colW[c] = max(r.colW[c], lb.w)
			}
			ls = append(ls, cells)
		}
		r.labs = append(r.labs, ls)
		h += em*0.3 + float64(max(1, len(sec)))*r.rowH
		if len(sec) == 0 {
			h -= r.rowH * 0.6
		}
	}
	rows := 0.0
	for _, cw := range r.colW {
		if cw > 0 {
			rows += cw + em*0.8
		}
	}
	n.w = max(w+em*1.6, rows+em*0.6, em*4)
	n.h = h + em*0.2
}

// plain is s in one line as it's written, no Markdown: a member's
// name may have stars and underscores.
func (d *drawer) plain(s string, st md.Style) *label {
	f := d.face(st)
	m := f.Metrics()
	a := atom{text: s, face: f, w: fx(font.MeasureString(f, s)), asc: fx(m.Ascent), desc: fx(m.Descent)}
	return &label{lines: []labLine{{atoms: []atom{a}, w: a.w, asc: a.asc, desc: a.desc}}, w: a.w, h: a.asc + a.desc}
}

// drawRecord draws n's record at c.
func (p *picture) drawRecord(n *node, c pt, em, lw float64) {
	r := n.rec
	x0, y0, x1, y1 := c.x-n.w/2, c.y-n.h/2, c.x+n.w/2, c.y+n.h/2
	box := []pt{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}}
	p.fill(box, 0xff)
	y := y0 + em*0.25
	for _, lb := range r.headLabs {
		y += lb.h
	}
	y += em * 0.25
	if r.cols { // an entity's name on gray
		p.fill([]pt{{x0, y0}, {x1, y0}, {x1, y}, {x0, y}}, 0xe0)
	}
	hy := y0 + em*0.25
	for _, lb := range r.headLabs {
		p.label(lb, pt{c.x, hy + lb.h/2})
		hy += lb.h
	}
	for _, sec := range r.labs {
		p.stroke([]pt{{x0, y}, {x1, y}}, lw, 0, 0)
		y += em * 0.15
		for _, cells := range sec {
			x := x0 + em*0.5
			for k, lb := range cells {
				p.label(lb, pt{x + lb.w/2, y + r.rowH/2})
				if k < len(r.colW) && r.colW[k] > 0 {
					x += r.colW[k] + em*0.8
				}
			}
			y += r.rowH
		}
		if len(sec) == 0 {
			y += r.rowH * 0.4
		}
		y += em * 0.15
	}
	p.stroke(closed(box), lw*1.2, 0, 0)
}
