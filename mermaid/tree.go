package mermaid

// Timelines and mind maps. A timeline runs down the page, as a narrow
// page wants: a line, a dot for each period, its name on the left and
// its events on the right, the sections across it. A mind map grows
// from its root to both sides (to the right only, for one or two
// branches), the first branches boxed, the rest underlined, each
// subtree as high as its leaves.

import (
	"math"
	"regexp"
	"strings"
)

type tlPeriod struct {
	name    string
	section string
	events  []string
}

type timeline struct {
	title   string
	periods []*tlPeriod
}

func parseTimeline(lines []string) *timeline {
	tl := &timeline{}
	section := ""
	var last *tlPeriod
	for _, l := range lines[1:] {
		switch {
		case strings.HasPrefix(l, "title "):
			tl.title = strings.TrimSpace(l[6:])
		case strings.HasPrefix(l, "section "):
			section, last = strings.TrimSpace(l[8:]), nil
		case strings.HasPrefix(l, ":"):
			if last != nil {
				for _, e := range strings.Split(l[1:], ":") {
					if e = strings.TrimSpace(e); e != "" {
						last.events = append(last.events, e)
					}
				}
			}
		case strings.HasPrefix(l, "accTitle"), strings.HasPrefix(l, "accDescr"):
		default:
			parts := strings.Split(l, " : ")
			if len(parts) == 1 {
				parts = strings.Split(l, ":")
			}
			pd := &tlPeriod{name: strings.TrimSpace(parts[0]), section: section}
			for _, e := range parts[1:] {
				if e = strings.TrimSpace(e); e != "" {
					pd.events = append(pd.events, e)
				}
			}
			tl.periods = append(tl.periods, pd)
			last = pd
		}
	}
	return tl
}

func (tl *timeline) draw(d *drawer) *picture {
	em := d.em
	lw := max(1.2, em*0.07)
	p := &picture{}
	type laid struct {
		name   *label
		events []*label
		h      float64
	}
	var ls []laid
	nameW, evW := 0.0, 0.0
	for _, pd := range tl.periods {
		l := laid{name: d.label("**"+pd.name+"**", em*9)}
		h := l.name.h
		eh := 0.0
		for _, e := range pd.events {
			lb := d.label(e, em*16)
			l.events = append(l.events, lb)
			evW = max(evW, lb.w)
			eh += lb.h + em*0.7
		}
		l.h = max(h, eh-em*0.2) + em*0.5
		nameW = max(nameW, l.name.w)
		ls = append(ls, l)
	}
	spine := nameW + em*1.2
	right := spine + em*1.2 + evW + em*0.8
	y := 0.0
	sec := ""
	var dots []pt
	top := y
	for i, pd := range tl.periods {
		if pd.section != sec && pd.section != "" {
			sec = pd.section
			lb := d.label("**"+sec+"**", right)
			h := lb.h + em*0.6
			p.fill([]pt{{0, y}, {right, y}, {right, y + h}, {0, y + h}}, 0xe4)
			p.label(lb, pt{em*0.3 + lb.w/2, y + h/2})
			y += h + em*0.5
		}
		l := ls[i]
		dot := pt{spine, y + l.name.h/2}
		dots = append(dots, dot)
		p.label(l.name, pt{spine - em*0.8 - l.name.w/2, dot.y})
		ey := y
		for _, lb := range l.events {
			x0, x1 := spine+em*1.2, spine+em*1.2+lb.w+em*0.8
			box := rounded(x0, ey-em*0.15, x1, ey+lb.h+em*0.3, em*0.3)
			p.fill(box, 0xff)
			p.stroke(closed(box), lw, 0, 0x40)
			p.label(lb, pt{(x0 + x1) / 2, ey + lb.h/2 + em*0.07})
			p.stroke([]pt{{spine, dot.y}, {x0, ey + lb.h/2}}, lw*0.8, em*0.2, 0x90)
			ey += lb.h + em*0.7
		}
		y += l.h + em*0.3
	}
	if len(dots) > 0 {
		p.stroke([]pt{{spine, top + em*0.2}, {spine, y}}, lw*2, 0, 0)
		for _, c := range dots {
			p.fill(circle(c, em*0.32, 20), 0)
			p.fill(circle(c, em*0.16, 20), 0xff)
		}
	}
	if tl.title != "" {
		lb := d.label("**"+tl.title+"**", right)
		p.label(lb, pt{right / 2, -em*0.6 - lb.h/2})
	}
	return p
}

// --- mind maps ---

type mmNode struct {
	text, shape string
	kids        []*mmNode
	depth       int
	lab         *label
	w, h        float64
	sub         float64 // its subtree's height
	x, y        float64 // its middle
	left        bool
}

var mmShapeRE = regexp.MustCompile(`^(?:[^\s(\[{)]*?)\s*(\(\((.*)\)\)|\)\)(.*)\(\(|\)(.*)\(|\[(.*)\]|\((.*)\)|\{\{(.*)\}\})\s*$`)

// mmText is a mind map's node: its text and shape, as written.
func mmText(s string) (string, string) {
	if i := strings.Index(s, ":::"); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	if m := mmShapeRE.FindStringSubmatch(s); m != nil {
		for k, shape := range []string{"", "", "circle", "bang", "cloud", "square", "rounded", "hex"} {
			if k >= 2 && m[k] != "" {
				return clean(m[k]), shape
			}
		}
	}
	return clean(s), ""
}

// rawLines are src's lines as prepare has them, but with their
// indentation: a mind map's is its tree.
func rawLines(src string) []string {
	var out []string
	front := false
	for i, l := range strings.Split(strings.ReplaceAll(strings.ReplaceAll(src, "\r", ""), "\t", "    "), "\n") {
		t := strings.TrimSpace(l)
		if t == "---" && (i == 0 || front) {
			front = !front
			continue
		}
		if front || t == "" || strings.HasPrefix(t, "%%") {
			continue
		}
		out = append(out, strings.TrimRight(l, " "))
	}
	return out
}

func parseMindmap(lines []string) *mmNode {
	type open struct {
		n      *mmNode
		indent int
	}
	var root *mmNode
	var stack []open
	for _, l := range lines[1:] {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "::icon") || strings.HasPrefix(t, ":::") {
			continue
		}
		indent := len(l) - len(strings.TrimLeft(l, " "))
		text, shape := mmText(t)
		n := &mmNode{text: text, shape: shape}
		if root == nil {
			root = n
			stack = []open{{n, indent}}
			continue
		}
		for len(stack) > 1 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}
		par := stack[len(stack)-1].n
		n.depth = par.depth + 1
		par.kids = append(par.kids, n)
		stack = append(stack, open{n, indent})
	}
	return root
}

func (r *mmNode) leaves() int {
	if len(r.kids) == 0 {
		return 1
	}
	n := 0
	for _, k := range r.kids {
		n += k.leaves()
	}
	return n
}

func (r *mmNode) draw(d *drawer) *picture {
	em := d.em
	lw := max(1.2, em*0.07)
	p := &picture{}
	var size func(n *mmNode)
	size = func(n *mmNode) {
		text := n.text
		if n.depth == 0 {
			text = "**" + text + "**"
		}
		n.lab = d.label(text, em*10)
		n.w, n.h = n.lab.w+em*0.4, n.lab.h+em*0.4
		switch {
		case n.shape == "circle" || n.shape == "bang":
			r := math.Hypot(n.lab.w, n.lab.h)/2 + em*0.4
			n.w, n.h = 2*r, 2*r
		case n.shape == "hex":
			n.w, n.h = n.lab.w+em*1.8, n.lab.h+em*0.8
		case n.shape != "" || n.depth <= 1:
			n.w, n.h = n.lab.w+em*1.2, n.lab.h+em*0.8
		}
		n.sub = 0
		for _, k := range n.kids {
			size(k)
			n.sub += k.sub
		}
		gap := em * 0.35
		if len(n.kids) > 0 {
			n.sub += gap * float64(len(n.kids)-1)
		}
		n.sub = max(n.sub, n.h)
	}
	size(r)
	// sides: the first branches, by their leaves, half each way
	var right, left []*mmNode
	if len(r.kids) >= 3 {
		total, acc := r.leaves(), 0
		for _, k := range r.kids {
			if acc*2 < total {
				right = append(right, k)
				acc += k.leaves()
			} else {
				k.left = true
				left = append(left, k)
			}
		}
	} else {
		right = r.kids
	}
	// the columns: each depth as wide as its widest, on each side
	colW := map[[2]int]float64{}
	var walk func(n *mmNode, side int)
	walk = func(n *mmNode, side int) {
		key := [2]int{side, n.depth}
		colW[key] = max(colW[key], n.w)
		for _, k := range n.kids {
			walk(k, side)
		}
	}
	for _, k := range right {
		walk(k, 1)
	}
	for _, k := range left {
		walk(k, -1)
	}
	gapX := em * 2.2
	colX := func(side, depth int) float64 { // a column's near edge from the root's middle
		x := r.w/2 + gapX
		for dd := 1; dd < depth; dd++ {
			x += colW[[2]int{side, dd}] + gapX
		}
		return x
	}
	var place func(n *mmNode, side int, top float64)
	place = func(n *mmNode, side int, top float64) {
		near := colX(side, n.depth)
		n.x = float64(side) * (near + n.w/2)
		n.y = top + n.sub/2
		var kh float64
		for i, k := range n.kids {
			if i > 0 {
				kh += em * 0.35
			}
			kh += k.sub
		}
		y := n.y - kh/2
		for _, k := range n.kids {
			place(k, side, y)
			y += k.sub + em*0.35
		}
	}
	stack := func(ns []*mmNode, side int) {
		h := 0.0
		for i, k := range ns {
			if i > 0 {
				h += em * 0.6
			}
			h += k.sub
		}
		y := -h / 2
		for _, k := range ns {
			place(k, side, y)
			y += k.sub + em*0.6
		}
	}
	stack(right, 1)
	stack(left, -1)
	// edges, then nodes over them
	var edges func(n *mmNode)
	edges = func(n *mmNode) {
		for _, k := range n.kids {
			side := 1.0
			if k.left {
				side = -1
			}
			a := pt{n.x + side*n.w/2, n.y}
			if n.depth == 0 {
				a = pt{n.x + side*n.w/2*0.9, n.y}
			} else if n.shape == "" && n.depth > 1 {
				a.y = n.y + n.h/2 // from the end of its underline
			}
			b := pt{k.x - side*k.w/2, k.y}
			if k.shape == "" && k.depth > 1 {
				b.y = k.y + k.h/2
			}
			dx := (b.x - a.x) / 2
			w := max(lw, lw*(3.2-0.7*float64(k.depth)))
			p.stroke(spline([]pt{a, {a.x + dx, a.y}, {b.x - dx, b.y}, b}), w, 0, 0x20)
			edges(k)
		}
	}
	edges(r)
	var nodes func(n *mmNode)
	nodes = func(n *mmNode) {
		c := pt{n.x, n.y}
		x0, y0, x1, y1 := n.x-n.w/2, n.y-n.h/2, n.x+n.w/2, n.y+n.h/2
		fill := uint8(0xff)
		if n.depth == 1 {
			fill = 0xe8
		}
		var outline []pt
		switch {
		case n.shape == "circle":
			outline = circle(c, n.w/2, 40)
		case n.shape == "bang":
			outline = circle(c, n.w/2, 40)
			p.fill(outline, fill)
			p.stroke(closed(circle(c, n.w/2+em*0.15, 40)), lw, 0, 0)
		case n.shape == "cloud":
			outline = rounded(x0, y0, x1, y1, n.h/2)
		case n.shape == "hex":
			k := n.h / 3
			outline = []pt{{x0, c.y}, {x0 + k, y0}, {x1 - k, y0}, {x1, c.y}, {x1 - k, y1}, {x0 + k, y1}}
		case n.shape == "square":
			outline = []pt{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}}
		case n.shape == "rounded" || n.depth <= 1:
			outline = rounded(x0, y0, x1, y1, min(em*0.5, n.h/2))
		}
		if outline != nil {
			p.fill(outline, fill)
			w := lw * 1.2
			if n.depth == 0 {
				w = lw * 2
			}
			p.stroke(closed(outline), w, 0, 0)
		} else {
			p.stroke([]pt{{x0, y1}, {x1, y1}}, max(lw, lw*(3.2-0.7*float64(n.depth))), 0, 0x20)
		}
		p.label(n.lab, c)
		for _, k := range n.kids {
			nodes(k)
		}
	}
	nodes(r)
	return p
}
