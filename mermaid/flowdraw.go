package mermaid

// A graph drawn: its subgraphs' boxes, its edges (smooth, through
// their points, cut where they meet a node's outline), their labels on
// white, and its nodes in their shapes.

import "math"

func (g *graph) draw(d *drawer) *picture {
	em := d.em
	g.size(d)
	L := g.lay(em)
	p := &picture{}
	lw := max(1.2, em*0.07)
	maxY := 0.0
	if len(L.rankY) > 0 {
		maxY = L.rankY[len(L.rankY)-1]
	}
	at := func(n *node) pt { return g.real(n.x, n.y, maxY) }
	hor := g.horizontal()

	// Subgraphs, the outermost first: a box round their nodes and the
	// boxes in them, a title at the top.
	type box struct{ x0, y0, x1, y1 float64 }
	boxes := make([]*box, len(g.subs))
	var boxOf func(s int) *box
	boxOf = func(s int) *box {
		if boxes[s] != nil {
			return boxes[s]
		}
		b := &box{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)}
		grow := func(x0, y0, x1, y1 float64) {
			b.x0, b.y0, b.x1, b.y1 = min(b.x0, x0), min(b.y0, y0), max(b.x1, x1), max(b.y1, y1)
		}
		for i, n := range g.nodes {
			if n.sub != s || i >= L.n && n.labels == 0 {
				continue
			}
			c := at(n)
			grow(c.x-n.w/2, c.y-n.h/2, c.x+n.w/2, c.y+n.h/2)
		}
		for c, cs := range g.subs {
			if cs.parent == s {
				cb := boxOf(c)
				grow(cb.x0, cb.y0, cb.x1, cb.y1)
			}
		}
		if math.IsInf(b.x0, 1) {
			b = &box{}
			boxes[s] = b
			return b
		}
		pad := em * 0.6
		b.x0, b.y0, b.x1, b.y1 = b.x0-pad, b.y0-pad, b.x1+pad, b.y1+pad
		c0, c1 := math.Inf(1), math.Inf(-1) // across: as far as its borders
		for _, n := range g.nodes {
			if n.border != 0 && n.sub == s {
				c0, c1 = min(c0, n.x), max(c1, n.x)
			}
		}
		if !math.IsInf(c0, 1) {
			if hor {
				b.y0, b.y1 = c0, c1
			} else {
				b.x0, b.x1 = c0, c1
			}
		}
		switch g.dir {
		case "TB":
			b.y0 -= g.subs[s].lab.h + pad*0.5
		case "BT":
			b.y1 += g.subs[s].lab.h + pad*0.5
		}
		if w := g.subs[s].lab.w + 2*pad; b.x1-b.x0 < w {
			cx := (b.x0 + b.x1) / 2
			b.x0, b.x1 = cx-w/2, cx+w/2
		}
		boxes[s] = b
		return b
	}
	var order []int
	var walk func(parent int)
	walk = func(parent int) {
		for i, s := range g.subs {
			if s.parent == parent {
				order = append(order, i)
				walk(i)
			}
		}
	}
	walk(-1)
	for _, s := range order {
		b := boxOf(s)
		if b.x0 == b.x1 {
			continue
		}
		r := []pt{{b.x0, b.y0}, {b.x1, b.y0}, {b.x1, b.y1}, {b.x0, b.y1}}
		gray := uint8(0xf2 - 0x0c*min(3, len(g.path(s))-1))
		p.fill(r, gray)
		p.stroke(closed(r), lw, 0, 0x60)
		ty := b.y0 + em*0.3 + g.subs[s].lab.h/2
		if !hor && g.dir == "BT" {
			ty = b.y1 - em*0.3 - g.subs[s].lab.h/2
		}
		p.label(g.subs[s].lab, pt{(b.x0 + b.x1) / 2, ty})
	}

	// Edges.
	outlines := make([][]pt, L.n)
	for i, n := range g.nodes[:L.n] {
		outlines[i], _ = shapeOf(n, at(n), em)
	}
	hs := em * 0.6
	type tag struct {
		lab *label
		at  pt
	}
	var tags []tag
	for i, e := range g.edges {
		chain := L.chains[i]
		w := lw
		dash := 0.0
		switch e.line {
		case '~':
			continue
		case '=':
			w = lw * 2.4
		case '.':
			dash = em * 0.3
		}
		if len(chain) == 1 { // to itself: a loop at the side
			n := g.nodes[chain[0]]
			c := at(n)
			r := c.x + n.w/2
			ps := spline([]pt{{r - em*0.2, c.y - n.h*0.3}, {r + em*1.4, c.y - n.h*0.6}, {r + em*1.4, c.y + n.h*0.6}, {r - em*0.2, c.y + n.h*0.3}})
			ps = clipStart(ps, outlines[chain[0]])
			ps = reverse(clipStart(reverse(ps), outlines[chain[0]]))
			g.drawEdge(p, e, ps, w, dash, hs, lw)
			if e.lab != nil {
				tags = append(tags, tag{e.lab, pt{r + em*1.6 + e.lab.w/2, c.y}})
			}
			continue
		}
		up := g.nodes[chain[0]].rank > g.nodes[chain[len(chain)-1]].rank
		ch := chain
		if up {
			ch = reverse2(chain)
		}
		var lps []pt // layout's points, down the ranks
		for k, u := range ch {
			n := g.nodes[u]
			a := n.h
			if hor {
				a = n.w
			}
			if k > 0 && !n.dummy {
				lps = append(lps, pt{n.x, n.y - a/2 - em*0.45})
			}
			lps = append(lps, pt{n.x, n.y})
			if k < len(ch)-1 && !n.dummy {
				lps = append(lps, pt{n.x, n.y + a/2 + em*0.45})
			}
			if n.labels == i+1 {
				tags = append(tags, tag{e.lab, at(n)})
			}
		}
		ps := make([]pt, len(lps))
		for k, q := range lps {
			ps[k] = g.real(q.x, q.y, maxY)
		}
		if up {
			ps = reverse(ps)
		}
		ps = spline(dedup(ps))
		from, to := outlines[e.from], outlines[e.to]
		if e.subFrom > 0 {
			b := boxOf(e.subFrom - 1)
			from = []pt{{b.x0, b.y0}, {b.x1, b.y0}, {b.x1, b.y1}, {b.x0, b.y1}}
		}
		if e.subTo > 0 {
			b := boxOf(e.subTo - 1)
			to = []pt{{b.x0, b.y0}, {b.x1, b.y0}, {b.x1, b.y1}, {b.x0, b.y1}}
		}
		ps = clipStart(ps, from)
		ps = reverse(clipStart(reverse(ps), to))
		g.drawEdge(p, e, ps, w, dash, hs, lw)
		if len(ps) >= 2 { // what's written at its ends, beside them
			for _, end := range []struct {
				lab *label
				ps  []pt
			}{{e.fromLab, ps}, {e.toLab, reverse(ps)}} {
				if end.lab == nil {
					continue
				}
				tip, dir := endOf(reverse(end.ps), 0)
				dir = dir.mul(-1)
				nrm := pt{-dir.y, dir.x}
				at := tip.add(dir.mul(em*0.6 + end.lab.h/2)).add(nrm.mul(em*0.4 + end.lab.w/2))
				tags = append(tags, tag{end.lab, at})
			}
		}
	}
	for _, t := range tags {
		r := []pt{{t.at.x - t.lab.w/2 - em*0.2, t.at.y - t.lab.h/2 - em*0.1}, {t.at.x + t.lab.w/2 + em*0.2, t.at.y - t.lab.h/2 - em*0.1},
			{t.at.x + t.lab.w/2 + em*0.2, t.at.y + t.lab.h/2 + em*0.1}, {t.at.x - t.lab.w/2 - em*0.2, t.at.y + t.lab.h/2 + em*0.1}}
		p.fill(r, 0xff)
		p.label(t.lab, t.at)
	}

	// Nodes.
	for i, n := range g.nodes[:L.n] {
		c := at(n)
		if n.rec != nil {
			p.drawRecord(n, c, em, lw)
			continue
		}
		fill := uint8(0xff)
		switch n.shape {
		case "start", "fork":
			fill = 0
		}
		p.fill(outlines[i], fill)
		p.stroke(closed(outlines[i]), lw*1.2, 0, 0)
		_, extras := shapeOf(n, c, em)
		for _, x := range extras {
			if n.shape == "end" {
				p.fill(x, 0)
				continue
			}
			p.stroke(x, lw*1.2, 0, 0)
		}
		if n.lab != nil && n.text != "" {
			lc := c
			if n.shape == "cyl" {
				lc.y += em * 0.2
			}
			p.label(n.lab, lc)
		}
	}
	return p
}

// drawEdge draws e's line through ps, its heads at the ends.
func (g *graph) drawEdge(p *picture, e *edge, ps []pt, w, dash, hs, lw float64) {
	p.arrow(ps, w, dash, e.head, e.tail, hs, lw)
}

// arrow draws a line through ps, w thick (dashed if dash > 0), heads
// of kind head and tail at its ends, size hs.
func (p *picture) arrow(ps []pt, w, dash float64, head, tail byte, hs, lw float64) {
	if len(ps) < 2 {
		return
	}
	if head != 0 {
		ps = trimEnd(ps, headRoom(head, hs))
	}
	if tail != 0 {
		ps = reverse(trimEnd(reverse(ps), headRoom(tail, hs)))
	}
	p.stroke(ps, w, dash, 0)
	if head != 0 {
		tip, dir := endOf(ps, headRoom(head, hs))
		p.head(head, tip, dir, hs, lw)
	}
	if tail != 0 {
		tip, dir := endOf(reverse(ps), headRoom(tail, hs))
		p.head(tail, tip, dir, hs, lw)
	}
}

// endOf is the point room past ps's end, and the way it goes there.
func endOf(ps []pt, room float64) (pt, pt) {
	n := len(ps)
	last := ps[n-1]
	var from pt
	for k := n - 2; k >= 0; k-- {
		from = ps[k]
		if last.sub(from).len() > 1 {
			break
		}
	}
	dir := last.sub(from).unit()
	return last.add(dir.mul(room)), dir
}

// trimEnd cuts d off the end of the line through ps.
func trimEnd(ps []pt, d float64) []pt {
	if d <= 0 {
		return ps
	}
	ps = append([]pt(nil), ps...)
	for len(ps) > 1 {
		a, b := ps[len(ps)-2], ps[len(ps)-1]
		l := b.sub(a).len()
		if l > d {
			ps[len(ps)-1] = lerp(b, a, d/l)
			return ps
		}
		d -= l
		ps = ps[:len(ps)-1]
	}
	return ps
}

func reverse(ps []pt) []pt {
	out := make([]pt, len(ps))
	for i, q := range ps {
		out[len(ps)-1-i] = q
	}
	return out
}

func reverse2(xs []int) []int {
	out := make([]int, len(xs))
	for i, x := range xs {
		out[len(xs)-1-i] = x
	}
	return out
}

// dedup is ps without a point on the one before it.
func dedup(ps []pt) []pt {
	out := ps[:0:0]
	for _, q := range ps {
		if len(out) > 0 && q.sub(out[len(out)-1]).len() < 0.5 {
			continue
		}
		out = append(out, q)
	}
	return out
}

// clipStart is the line through ps from where it leaves the outline
// round its start.
func clipStart(ps []pt, outline []pt) []pt {
	if len(outline) < 3 {
		return ps
	}
	for i := 0; i+1 < len(ps); i++ {
		a, b := ps[i], ps[i+1]
		best := math.Inf(1)
		for k := range outline {
			c, d := outline[k], outline[(k+1)%len(outline)]
			if t, ok := cross(a, b, c, d); ok && t < best {
				best = t
			}
		}
		if !math.IsInf(best, 1) {
			return append([]pt{lerp(a, b, best)}, ps[i+1:]...)
		}
	}
	return ps
}

// cross is where segment ab meets cd, along ab (0 to 1).
func cross(a, b, c, d pt) (float64, bool) {
	r, s := b.sub(a), d.sub(c)
	den := r.x*s.y - r.y*s.x
	if math.Abs(den) < 1e-9 {
		return 0, false
	}
	q := c.sub(a)
	t := (q.x*s.y - q.y*s.x) / den
	u := (q.x*r.y - q.y*r.x) / den
	if t < 0 || t > 1 || u < 0 || u > 1 {
		return 0, false
	}
	return t, true
}

// shapeOf is a node's outline at c, and what's drawn in it (a
// subroutine's bars, a cylinder's rim, a double circle's inner one).
func shapeOf(n *node, c pt, em float64) ([]pt, [][]pt) {
	x0, y0, x1, y1 := c.x-n.w/2, c.y-n.h/2, c.x+n.w/2, c.y+n.h/2
	rect := []pt{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}}
	switch n.shape {
	case "round":
		return rounded(x0, y0, x1, y1, min(em*0.45, n.h/2)), nil
	case "stadium":
		return rounded(x0, y0, x1, y1, n.h/2), nil
	case "sub":
		k := em * 0.4
		return rect, [][]pt{{{x0 + k, y0}, {x0 + k, y1}}, {{x1 - k, y0}, {x1 - k, y1}}}
	case "cyl":
		ry := em * 0.35
		rx := n.w / 2
		var ps []pt
		ps = append(ps, arc(pt{c.x, y0 + ry}, rx, ry, math.Pi, 2*math.Pi, 16)...)
		ps = append(ps, arc(pt{c.x, y1 - ry}, rx, ry, 0, math.Pi, 16)...)
		return ps, [][]pt{arc(pt{c.x, y0 + ry}, rx, ry, 0, math.Pi, 16)}
	case "circle":
		return circle(c, n.w/2, 40), nil
	case "dcircle":
		return circle(c, n.w/2, 40), [][]pt{closed(circle(c, n.w/2-em*0.2, 40))}
	case "start":
		return circle(c, n.w/2, 24), nil
	case "end":
		return circle(c, n.w/2, 24), [][]pt{circle(c, n.w/2-em*0.18, 24)}
	case "rhombus", "choice":
		return []pt{{c.x, y0}, {x1, c.y}, {c.x, y1}, {x0, c.y}}, nil
	case "hex":
		k := n.h / 4
		return []pt{{x0, c.y}, {x0 + k, y0}, {x1 - k, y0}, {x1, c.y}, {x1 - k, y1}, {x0 + k, y1}}, nil
	case "para":
		k := n.h * 0.5
		return []pt{{x0 + k, y0}, {x1, y0}, {x1 - k, y1}, {x0, y1}}, nil
	case "para2":
		k := n.h * 0.5
		return []pt{{x0, y0}, {x1 - k, y0}, {x1, y1}, {x0 + k, y1}}, nil
	case "trap":
		k := n.h * 0.5
		return []pt{{x0 + k, y0}, {x1 - k, y0}, {x1, y1}, {x0, y1}}, nil
	case "trap2":
		k := n.h * 0.5
		return []pt{{x0, y0}, {x1, y0}, {x1 - k, y1}, {x0 + k, y1}}, nil
	case "asym":
		return []pt{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}, {x0 + n.h*0.3, c.y}}, nil
	}
	return rect, nil
}

// rounded is a rectangle's outline, its corners round, radius r.
func rounded(x0, y0, x1, y1, r float64) []pt {
	var ps []pt
	ps = append(ps, arc(pt{x1 - r, y0 + r}, r, r, -math.Pi/2, 0, 6)...)
	ps = append(ps, arc(pt{x1 - r, y1 - r}, r, r, 0, math.Pi/2, 6)...)
	ps = append(ps, arc(pt{x0 + r, y1 - r}, r, r, math.Pi/2, math.Pi, 6)...)
	ps = append(ps, arc(pt{x0 + r, y0 + r}, r, r, math.Pi, 3*math.Pi/2, 6)...)
	return ps
}
