package mermaid

// Sequence diagrams: participants (and actors) along the top, each a
// line down; messages as arrows between the lines, in turn down the
// page; notes; loops, alternatives and the like as frames round what's
// in them.

import (
	"fmt"
	"regexp"
	"strings"
)

type part struct {
	id, name string
	actor    bool
	lab      *label
	w, h, x  float64
}

// sevent is a message ('m'), a note ('n'), a frame's start ('['), a
// line in one ('|') or its end (']').
type sevent struct {
	kind       byte
	from, to   int
	line       byte // '-' solid, '.' dotted
	head, tail byte
	where      string // a note's: left, right, over
	text       string
	lab, cond  *label
}

type seqDiag struct {
	parts  []*part
	ids    map[string]int
	events []*sevent
	auto   bool
	title  string
}

var (
	partRE  = regexp.MustCompile(`^(?:create\s+)?(participant|actor)\s+(.+?)(?:\s+as\s+(.+))?$`)
	msgRE   = regexp.MustCompile(`^(.+?)\s*(<<-->>|<<->>|-->>|->>|--x|-x|--\)|-\)|-->|->)\s*([+-]?)\s*(.+?)\s*(?::\s*(.*))?$`)
	snoteRE = regexp.MustCompile(`^[Nn]ote\s+(left of|right of|over)\s+([^:]+?)\s*:\s*(.*)$`)
	blockRE = regexp.MustCompile(`^(loop|alt|opt|par|par_over|critical|break|rect|box)\b\s*(.*)$`)
	divRE   = regexp.MustCompile(`^(else|and|option)\b\s*(.*)$`)
)

func (s *seqDiag) part(id string) int {
	id = strings.Trim(strings.TrimSpace(id), `"`)
	if i, ok := s.ids[id]; ok {
		return i
	}
	s.parts = append(s.parts, &part{id: id, name: id})
	s.ids[id] = len(s.parts) - 1
	return len(s.parts) - 1
}

func parseSeq(lines []string) *seqDiag {
	s := &seqDiag{ids: map[string]int{}}
	for _, l := range lines[1:] {
		l = strings.TrimSuffix(l, ";")
		switch {
		case l == "autonumber" || strings.HasPrefix(l, "autonumber "):
			s.auto = true
		case strings.HasPrefix(l, "title ") || strings.HasPrefix(l, "title:"):
			s.title = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(l, "title"), ":"))
		case partRE.MatchString(l):
			m := partRE.FindStringSubmatch(l)
			i := s.part(m[2])
			s.parts[i].actor = m[1] == "actor"
			if m[3] != "" {
				s.parts[i].name = m[3]
			}
		case l == "end":
			s.events = append(s.events, &sevent{kind: ']'})
		case blockRE.MatchString(l):
			m := blockRE.FindStringSubmatch(l)
			s.events = append(s.events, &sevent{kind: '[', where: m[1], text: m[2]})
		case divRE.MatchString(l):
			m := divRE.FindStringSubmatch(l)
			s.events = append(s.events, &sevent{kind: '|', where: m[1], text: m[2]})
		case snoteRE.MatchString(l):
			m := snoteRE.FindStringSubmatch(l)
			ps := strings.Split(m[2], ",")
			e := &sevent{kind: 'n', where: strings.Fields(m[1])[0], text: m[3], from: s.part(ps[0])}
			e.to = e.from
			if len(ps) > 1 {
				e.to = s.part(ps[1])
			}
			s.events = append(s.events, e)
		case strings.HasPrefix(l, "activate ") || strings.HasPrefix(l, "deactivate ") || strings.HasPrefix(l, "destroy ") ||
			strings.HasPrefix(l, "link ") || strings.HasPrefix(l, "links "):
		case msgRE.MatchString(l):
			m := msgRE.FindStringSubmatch(l)
			e := &sevent{kind: 'm', from: s.part(m[1]), to: s.part(m[4]), text: m[5], line: '-'}
			a := m[2]
			if strings.Contains(a, "--") {
				e.line = '.'
			}
			switch {
			case strings.HasPrefix(a, "<<"):
				e.head, e.tail = '>', '>'
			case strings.HasSuffix(a, ">>"):
				e.head = '>'
			case strings.HasSuffix(a, "x"):
				e.head = 'x'
			case strings.HasSuffix(a, ")"):
				e.head = 'v'
			}
			s.events = append(s.events, e)
		}
	}
	return s
}

func (s *seqDiag) draw(d *drawer) *picture {
	em := d.em
	lw := max(1.2, em*0.07)
	hs := em * 0.6
	n := len(s.parts)
	for _, pa := range s.parts {
		pa.lab = d.label(pa.name, em*10)
		if pa.actor {
			pa.w, pa.h = max(pa.lab.w, em*2), em*2.4+pa.lab.h+em*0.3
		} else {
			pa.w, pa.h = max(pa.lab.w+em*1.6, em*4), pa.lab.h+em*0.9
		}
	}
	// Across: the gaps between the lines wide enough for what's between.
	gap := make([]float64, max(0, n-1))
	for i := range gap {
		gap[i] = (s.parts[i].w+s.parts[i+1].w)/2 + em*1.2
	}
	left, right := s.parts[0].w/2, s.parts[n-1].w/2
	type span struct {
		lo, hi int
		need   float64
	}
	var spans []span
	numW := 0.0
	if s.auto {
		numW = em * 1.4
	}
	for _, e := range s.events {
		switch e.kind {
		case 'm':
			e.lab = d.label(e.text, em*16)
			if e.from == e.to {
				ext := em*1.8 + e.lab.w + em*0.5
				if e.from < n-1 {
					gap[e.from] = max(gap[e.from], ext+em*0.4)
				} else {
					right = max(right, ext)
				}
				continue
			}
			lo, hi := min(e.from, e.to), max(e.from, e.to)
			spans = append(spans, span{lo, hi, e.lab.w + em*1.6 + numW})
		case 'n':
			e.lab = d.label(e.text, em*12)
			nw := e.lab.w + em*1.2
			i := e.from
			switch {
			case e.where == "left":
				if i > 0 {
					gap[i-1] = max(gap[i-1], nw+em*0.9)
				} else {
					left = max(left, nw+em*0.6)
				}
			case e.where == "right":
				if i < n-1 {
					gap[i] = max(gap[i], nw+em*0.9)
				} else {
					right = max(right, nw+em*0.6)
				}
			case e.from != e.to:
				lo, hi := min(e.from, e.to), max(e.from, e.to)
				spans = append(spans, span{lo, hi, nw - em*1.2})
			default:
				if i > 0 {
					gap[i-1] = max(gap[i-1], nw/2+em*0.5)
				} else {
					left = max(left, nw/2)
				}
				if i < n-1 {
					gap[i] = max(gap[i], nw/2+em*0.5)
				} else {
					right = max(right, nw/2)
				}
			}
		case '[', '|':
			if e.text != "" {
				e.cond = d.label("["+e.text+"]", em*16)
			}
			if e.kind == '[' {
				e.lab = d.label("**"+e.where+"**", 0)
			}
		}
	}
	for k := 1; k < n; k++ {
		for _, sp := range spans {
			if sp.hi-sp.lo != k {
				continue
			}
			cur := 0.0
			for i := sp.lo; i < sp.hi; i++ {
				cur += gap[i]
			}
			if cur < sp.need {
				for i := sp.lo; i < sp.hi; i++ {
					gap[i] += (sp.need - cur) / float64(k)
				}
			}
		}
	}
	x := 0.0
	for i, pa := range s.parts {
		if i > 0 {
			x += gap[i-1]
		}
		pa.x = x
	}
	_ = left
	_ = right

	bg, p := &picture{}, &picture{}
	head := 0.0
	for _, pa := range s.parts {
		head = max(head, pa.h)
	}
	if s.title != "" {
		t := d.label("**"+s.title+"**", 0)
		p.label(t, pt{(s.parts[0].x + s.parts[n-1].x) / 2, -em*0.6 - t.h/2})
	}
	y := head + em*0.8
	type frame struct {
		ev     *sevent
		top    float64
		x0, x1 float64
		divs   []float64
		dlabs  []*label
		any    bool
	}
	var open []*frame
	touch := func(a, b float64) {
		for _, f := range open {
			if !f.any {
				f.x0, f.x1, f.any = a, b, true
			}
			f.x0, f.x1 = min(f.x0, a), max(f.x1, b)
		}
	}
	num := 0
	for _, e := range s.events {
		switch e.kind {
		case 'm':
			num++
			a, b := s.parts[e.from].x, s.parts[e.to].x
			dash := 0.0
			if e.line == '.' {
				dash = em * 0.3
			}
			ly := y + e.lab.h + em*0.35
			if e.from == e.to {
				p.label(e.lab, pt{a + em*0.4 + e.lab.w/2, y + e.lab.h/2})
				ps := spline([]pt{{a, ly}, {a + em*1.6, ly}, {a + em*1.6, ly + em*1.1}, {a, ly + em*1.1}})
				p.arrow(ps, lw, dash, e.head, e.tail, hs, lw)
				touch(a-em*0.2, max(a+em*1.8, a+em*0.4+e.lab.w))
				if s.auto {
					s.number(p, d, num, pt{a, ly})
				}
				y = ly + em*1.1 + em*0.9
				continue
			}
			p.label(e.lab, pt{(a + b) / 2, y + e.lab.h/2})
			start := a
			if s.auto {
				s.number(p, d, num, pt{a, ly})
				dir := 1.0
				if b < a {
					dir = -1
				}
				start = a + dir*em*0.55
			}
			p.arrow([]pt{{start, ly}, {b, ly}}, lw, dash, e.head, e.tail, hs, lw)
			touch(min(a, b), max(a, b))
			y = ly + em*0.9
		case 'n':
			a, b := s.parts[e.from].x, s.parts[e.to].x
			nw, nh := e.lab.w+em*1.2, e.lab.h+em*0.7
			var x0 float64
			switch {
			case e.where == "left":
				x0 = a - em*0.6 - nw
			case e.where == "right":
				x0 = a + em*0.6
			default:
				lo, hi := min(a, b), max(a, b)
				nw = max(nw, hi-lo+em*1.2)
				x0 = (lo+hi)/2 - nw/2
			}
			r := []pt{{x0, y}, {x0 + nw, y}, {x0 + nw, y + nh}, {x0, y + nh}}
			p.fill(r, 0xee)
			p.stroke(closed(r), lw, 0, 0x40)
			p.label(e.lab, pt{x0 + nw/2, y + nh/2})
			touch(x0, x0+nw)
			y += nh + em*0.7
		case '[':
			f := &frame{ev: e, top: y}
			open = append(open, f)
			tab := e.lab.h + em*0.5
			if e.cond != nil {
				tab = max(tab, e.cond.h+em*0.5)
			}
			y += tab + em*0.5
		case '|':
			if len(open) == 0 {
				continue
			}
			f := open[len(open)-1]
			y += em * 0.2
			f.divs = append(f.divs, y)
			f.dlabs = append(f.dlabs, e.cond)
			if e.cond != nil {
				y += e.cond.h + em*0.5
			} else {
				y += em * 0.5
			}
		case ']':
			if len(open) == 0 {
				continue
			}
			f := open[len(open)-1]
			open = open[:len(open)-1]
			y += em * 0.2
			if !f.any {
				f.x0, f.x1 = s.parts[0].x, s.parts[n-1].x
			}
			x0, x1 := f.x0-em*0.8, f.x1+em*0.8
			tabW := f.ev.lab.w + em*1.4
			if f.ev.cond != nil {
				x1 = max(x1, x0+tabW+f.ev.cond.w+em*0.8)
			}
			for _, dl := range f.dlabs {
				if dl != nil {
					x1 = max(x1, (x0+x1)/2+dl.w/2+em*0.4)
				}
			}
			r := []pt{{x0, f.top}, {x1, f.top}, {x1, y}, {x0, y}}
			switch f.ev.where {
			case "box":
			case "rect":
				bg.fill(r, 0xf0)
			default:
				bg.stroke(closed(r), lw, 0, 0)
				th := f.ev.lab.h + em*0.5
				tab := []pt{{x0, f.top}, {x0 + tabW, f.top}, {x0 + tabW, f.top + th*0.6}, {x0 + tabW - th*0.4, f.top + th}, {x0, f.top + th}}
				bg.fill(tab, 0xe0)
				bg.stroke(closed(tab), lw, 0, 0)
				bg.label(f.ev.lab, pt{x0 + (tabW-em*0.3)/2, f.top + th/2})
				if f.ev.cond != nil {
					bg.label(f.ev.cond, pt{x0 + tabW + em*0.4 + f.ev.cond.w/2, f.top + th/2})
				}
				for k, dy := range f.divs {
					bg.stroke([]pt{{x0, dy}, {x1, dy}}, lw, em*0.3, 0)
					if dl := f.dlabs[k]; dl != nil {
						bg.label(dl, pt{(x0 + x1) / 2, dy + em*0.25 + dl.h/2})
					}
				}
			}
			for _, o := range open { // the frame's in the one round it
				o.x0, o.x1 = min(o.x0, x0), max(o.x1, x1)
				if !o.any {
					o.x0, o.x1, o.any = x0, x1, true
				}
			}
			y += em * 0.6
		}
	}
	foot := y + em*0.2
	for _, pa := range s.parts {
		bg.stroke([]pt{{pa.x, head}, {pa.x, foot}}, lw, em*0.35, 0x70)
		s.drawPart(p, pa, 0, head, lw, em)
		s.drawPart(p, pa, foot, head, lw, em)
	}
	bg.ops = append(bg.ops, p.ops...)
	if p.any {
		bg.grow(pt{p.x0, p.y0}, pt{p.x1, p.y1})
	}
	return bg
}

// number is a message's number, in a circle at c.
func (s *seqDiag) number(p *picture, d *drawer, n int, c pt) {
	r := d.em * 0.5
	p.fill(circle(c, r, 20), 0xff)
	p.stroke(closed(circle(c, r, 20)), max(1.2, d.em*0.07), 0, 0)
	small := &drawer{em: d.em * 0.7, faces: d.faces}
	p.label(small.label(fmt.Sprint(n), 0), c)
}

// drawPart draws a participant's box (an actor's figure), its top at y.
func (s *seqDiag) drawPart(p *picture, pa *part, y, head, lw, em float64) {
	if pa.actor {
		top := y + (head - pa.h)
		c := pt{pa.x, top + em*0.4}
		p.fill(circle(c, em*0.4, 20), 0xff)
		p.stroke(closed(circle(c, em*0.4, 20)), lw*1.2, 0, 0)
		neck, hip := top+em*0.8, top+em*1.6
		p.stroke([]pt{{pa.x, neck}, {pa.x, hip}}, lw*1.2, 0, 0)
		p.stroke([]pt{{pa.x - em*0.6, top + em*1.05}, {pa.x + em*0.6, top + em*1.05}}, lw*1.2, 0, 0)
		p.stroke([]pt{{pa.x - em*0.5, top + em*2.3}, {pa.x, hip}, {pa.x + em*0.5, top + em*2.3}}, lw*1.2, 0, 0)
		p.label(pa.lab, pt{pa.x, top + em*2.4 + em*0.2 + pa.lab.h/2})
		return
	}
	top := y + (head - pa.h)
	r := rounded(pa.x-pa.w/2, top, pa.x+pa.w/2, top+pa.h, em*0.25)
	p.fill(r, 0xff)
	p.stroke(closed(r), lw*1.2, 0, 0)
	p.label(pa.lab, pt{pa.x, top + pa.h/2})
}
