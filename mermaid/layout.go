package mermaid

// A graph laid out in ranks, as dagre (Mermaid's) does it: cycles
// broken, each node a rank (an edge at least a rank down, two where it
// has a label: its label's place between), long edges cut by points in
// the ranks they cross, the ranks' orders found by barycenters (each
// subgraph's nodes kept together), then the nodes placed across, each
// as near its neighbours as the order lets it, and the edges drawn
// through their points. Laid out as top to bottom: across is x, along
// the ranks y; a chart left to right is turned after.

import (
	"math"
	"sort"
)

func (g *graph) horizontal() bool { return g.dir == "LR" || g.dir == "RL" }

// size measures the nodes and labels: w and h are the drawn box's.
func (g *graph) size(d *drawer) {
	em := d.em
	for _, n := range g.nodes {
		n.lab = d.label(n.text, em*14)
		lw, lh := n.lab.w, n.lab.h
		px, py := em*0.8, em*0.5
		switch n.shape {
		case "circle":
			r := math.Hypot(lw, lh)/2 + em*0.35
			n.w, n.h = 2*r, 2*r
		case "dcircle":
			r := math.Hypot(lw, lh)/2 + em*0.6
			n.w, n.h = 2*r, 2*r
		case "rhombus":
			a := lw/2 + lh + em*0.4
			b := max(a*0.6, lh/2+lw*0.25+em*0.4)
			n.w, n.h = 2*a, 2*b
		case "choice":
			n.w, n.h = em*1.6, em*1.6
		case "start":
			n.w, n.h = em*0.9, em*0.9
		case "end":
			n.w, n.h = em*1.0, em*1.0
		case "fork":
			n.w, n.h = em*4, em*0.4
			if g.horizontal() {
				n.w, n.h = n.h, n.w
			}
		default:
			n.w, n.h = lw+2*px, lh+2*py
			switch n.shape {
			case "stadium":
				n.w += n.h * 0.4
			case "sub":
				n.w += em * 0.8
			case "cyl":
				n.h += em * 0.7
			case "hex", "para", "para2", "trap", "trap2":
				n.w += n.h * 0.5
			case "asym":
				n.w += n.h * 0.35
			}
			n.w = max(n.w, em*3)
		}
	}
	for _, e := range g.edges {
		if e.text != "" {
			e.lab = d.label(e.text, em*12)
		}
	}
	for _, s := range g.subs {
		s.lab = d.label("**"+s.title+"**", 0)
	}
}

// lay lays the graph out: nodes' ranks, orders and places, edges'
// points (by dummies). Dummies are added to g.nodes after the real
// ones; chains are each edge's nodes, first to last.
type laid struct {
	n      int     // real nodes
	chains [][]int // each edge's: its from, its dummies, its to (from as written, even if it was turned)
	layers [][]int
	ranks  int
	rankY  []float64
	succ   [][]int
	pred   [][]int
}

func (g *graph) lay(em float64) *laid {
	L := &laid{n: len(g.nodes)}
	hor := g.horizontal()
	// across and along: a node's size across the ranks and along them
	across := func(n *node) float64 {
		if hor {
			return n.h
		}
		return n.w
	}
	along := func(n *node) float64 {
		if hor {
			return n.w
		}
		return n.h
	}

	// Cycles broken: an edge back to a node on the way is turned.
	type arc struct{ u, v, minlen int }
	arcs := make([]arc, len(g.edges))
	state := make([]int, L.n)
	out := make([][]int, L.n)
	for i, e := range g.edges {
		if e.from != e.to {
			out[e.from] = append(out[e.from], i)
		}
	}
	turned := make([]bool, len(g.edges))
	var dfs func(u int)
	dfs = func(u int) {
		state[u] = 1
		for _, i := range out[u] {
			v := g.edges[i].to
			switch state[v] {
			case 0:
				dfs(v)
			case 1:
				turned[i] = true
			}
		}
		state[u] = 2
	}
	for u := range L.n {
		if state[u] == 0 {
			dfs(u)
		}
	}
	for i, e := range g.edges {
		a := arc{e.from, e.to, 2 * e.minlen}
		if turned[i] {
			a.u, a.v = a.v, a.u
		}
		arcs[i] = a
	}

	// Ranks: the longest path from the top, then a node with no edges in
	// pulled down to just over the nodes it points at.
	rank := make([]int, L.n)
	indeg := make([]int, L.n)
	outs := make([][]int, L.n)
	ins := make([][]int, L.n)
	for i, a := range arcs {
		if a.u == a.v {
			continue
		}
		indeg[a.v]++
		outs[a.u] = append(outs[a.u], i)
		ins[a.v] = append(ins[a.v], i)
	}
	var topo []int
	q := []int{}
	for u := range L.n {
		if indeg[u] == 0 {
			q = append(q, u)
		}
	}
	for len(q) > 0 {
		u := q[0]
		q = q[1:]
		topo = append(topo, u)
		for _, i := range outs[u] {
			v := arcs[i].v
			rank[v] = max(rank[v], rank[u]+arcs[i].minlen)
			if indeg[v]--; indeg[v] == 0 {
				q = append(q, v)
			}
		}
	}
	for k := len(topo) - 1; k >= 0; k-- {
		u := topo[k]
		if len(ins[u]) > 0 || len(outs[u]) == 0 {
			continue
		}
		r := math.MaxInt
		for _, i := range outs[u] {
			r = min(r, rank[arcs[i].v]-arcs[i].minlen)
		}
		rank[u] = r
	}
	lo := math.MaxInt
	for _, r := range rank {
		lo = min(lo, r)
	}
	for u := range rank {
		rank[u] -= lo
		g.nodes[u].rank = rank[u]
		L.ranks = max(L.ranks, rank[u]+1)
	}

	// Dummies: a point in each rank an edge crosses; a labeled edge's
	// label sits on the one in the middle.
	L.chains = make([][]int, len(g.edges))
	L.succ = make([][]int, L.n)
	L.pred = make([][]int, L.n)
	link := func(u, v int) {
		L.succ[u] = append(L.succ[u], v)
		L.pred[v] = append(L.pred[v], u)
	}
	for i, a := range arcs {
		e := g.edges[i]
		if a.u == a.v {
			L.chains[i] = []int{a.u}
			continue
		}
		chain := []int{a.u}
		ru, rv := rank[a.u], rank[a.v]
		midR := (ru + rv) / 2
		sub := g.common(g.nodes[a.u].sub, g.nodes[a.v].sub)
		for r := ru + 1; r < rv; r++ {
			d := &node{dummy: true, rank: r, sub: sub}
			if e.lab != nil && r == midR {
				d.labels = i + 1
				d.w, d.h = e.lab.w+em*0.4, e.lab.h+em*0.2
			}
			g.nodes = append(g.nodes, d)
			L.succ = append(L.succ, nil)
			L.pred = append(L.pred, nil)
			chain = append(chain, len(g.nodes)-1)
		}
		chain = append(chain, a.v)
		for k := 1; k < len(chain); k++ {
			link(chain[k-1], chain[k])
		}
		if turned[i] {
			for l, r := 0, len(chain)-1; l < r; l, r = l+1, r-1 {
				chain[l], chain[r] = chain[r], chain[l]
			}
		}
		L.chains[i] = chain
	}
	for u, n := range g.nodes {
		n.rank = max(n.rank, 0)
		if u < L.n {
			n.rank = rank[u]
		}
	}

	// Borders: a subgraph has a node at its left and one at its right in
	// each rank it's in, joined down the ranks: they keep what isn't in it
	// out, and its box's sides straight.
	for si := range g.subs {
		lo, hi := math.MaxInt, -1
		for _, n := range g.nodes {
			if n.border == 0 && n.sub >= 0 && g.within(n.sub, si) {
				lo, hi = min(lo, n.rank), max(hi, n.rank)
			}
		}
		for _, side := range []int{-1, 1} {
			prev := -1
			for r := lo; r <= hi; r++ {
				g.nodes = append(g.nodes, &node{dummy: true, border: side, rank: r, sub: si})
				L.succ = append(L.succ, nil)
				L.pred = append(L.pred, nil)
				u := len(g.nodes) - 1
				if prev >= 0 {
					link(prev, u)
				}
				prev = u
			}
		}
	}

	// The order in each rank: first as a walk from the top finds them,
	// then by barycenters, down and up, the best kept.
	L.layers = make([][]int, L.ranks)
	seen := make([]bool, len(g.nodes))
	var walk func(u int)
	walk = func(u int) {
		if seen[u] {
			return
		}
		seen[u] = true
		L.layers[g.nodes[u].rank] = append(L.layers[g.nodes[u].rank], u)
		for _, v := range L.succ[u] {
			walk(v)
		}
	}
	for _, u := range topo {
		walk(u)
	}
	for u := range g.nodes {
		walk(u)
	}
	L.setOrder(g)
	for _, ly := range L.layers {
		g.sortLayer(ly, func(u int) float64 { return float64(g.nodes[u].order) })
	}
	L.setOrder(g)
	best := L.copyLayers()
	bestX := L.crossings(g)
	for it, stale := 0, 0; it < 24 && stale < 6 && bestX > 0; it++ {
		down := it%2 == 0
		if down {
			for r := 1; r < L.ranks; r++ {
				L.byBary(g, r, L.pred)
			}
		} else {
			for r := L.ranks - 2; r >= 0; r-- {
				L.byBary(g, r, L.succ)
			}
		}
		if x := L.crossings(g); x < bestX {
			best, bestX, stale = L.copyLayers(), x, 0
		} else {
			stale++
		}
	}
	L.layers = best
	L.setOrder(g)

	// Across: each node as near the mean of its neighbours as the order
	// and the gaps let it be (isotonic regression, one rank at a time).
	gap := func(a, b *node) float64 {
		gp := em * 1.4
		if hor {
			gp = em * 0.9
		}
		switch {
		case a.border != 0 || b.border != 0:
			gp = em * 0.7
			if a.border == 1 && b.border == -1 {
				gp = em
			}
			if hor && a.border == -1 { // a subgraph's title, over what's in it
				gp += g.subs[a.sub].lab.h + em*0.2
			}
		case a.dummy || b.dummy:
			gp = em * 0.5
			if a.labels > 0 || b.labels > 0 {
				gp = em * 0.8
			}
		}
		return gp + (across(a)+across(b))/2
	}
	for _, ly := range L.layers {
		x := 0.0
		for i, u := range ly {
			if i > 0 {
				x += gap(g.nodes[ly[i-1]], g.nodes[u])
			}
			g.nodes[u].x = x
		}
	}
	weight := func(u, v int) float64 {
		du, dv := g.nodes[u].dummy, g.nodes[v].dummy
		switch {
		case du && dv:
			return 8
		case du || dv:
			return 2
		}
		return 1
	}
	members := make([][]int, len(g.subs)) // what's in each subgraph, but its borders
	for u, n := range g.nodes {
		if n.border == 0 && n.sub >= 0 && (u < L.n || n.labels > 0) {
			for s := n.sub; s >= 0; s = g.subs[s].parent {
				members[s] = append(members[s], u)
			}
		}
	}
	place := func(r int, nbrs ...[][]int) {
		ly := L.layers[r]
		if len(ly) == 0 {
			return
		}
		target := make([]float64, len(ly))
		w := make([]float64, len(ly))
		for i, u := range ly {
			if n := g.nodes[u]; n.border != 0 { // tight round what's in it
				lo, hi := math.Inf(1), math.Inf(-1)
				for _, v := range members[n.sub] {
					m := g.nodes[v]
					lo, hi = min(lo, m.x-across(m)/2), max(hi, m.x+across(m)/2)
				}
				if !math.IsInf(lo, 1) {
					target[i], w[i] = lo-em*0.7, 20
					if hor {
						target[i] -= g.subs[n.sub].lab.h + em*0.2
					}
					if n.border == 1 {
						target[i] = hi + em*0.7
					}
					continue
				}
			}
			sum, ws := 0.0, 0.0
			for _, nb := range nbrs {
				for _, v := range nb[u] {
					k := weight(u, v)
					sum += k * g.nodes[v].x
					ws += k
				}
			}
			if ws == 0 {
				target[i], w[i] = g.nodes[u].x, 0.1
			} else {
				target[i], w[i] = sum/ws, ws
			}
		}
		seps := make([]float64, len(ly))
		for i := 1; i < len(ly); i++ {
			seps[i] = seps[i-1] + gap(g.nodes[ly[i-1]], g.nodes[ly[i]])
		}
		for i := range target {
			target[i] -= seps[i]
		}
		xs := isotonic(target, w)
		for i, u := range ly {
			g.nodes[u].x = xs[i] + seps[i]
		}
	}
	for it := 0; it < 12; it++ {
		for r := 1; r < L.ranks; r++ {
			place(r, L.pred)
		}
		for r := L.ranks - 2; r >= 0; r-- {
			place(r, L.succ)
		}
	}
	for it := 0; it < 4; it++ {
		for r := 0; r < L.ranks; r++ {
			place(r, L.pred, L.succ)
		}
	}

	// Along: each rank as thick as its thickest node, a gap between, and
	// room for subgraphs' titles where they start.
	thick := make([]float64, L.ranks)
	before := make([]float64, L.ranks)
	after := make([]float64, L.ranks)
	for _, n := range g.nodes {
		thick[n.rank] = max(thick[n.rank], along(n))
	}
	pad := em * 0.6
	for si, s := range g.subs {
		lo, hi := math.MaxInt, -1
		for _, n := range g.nodes[:L.n] {
			if g.within(n.sub, si) {
				lo, hi = min(lo, n.rank), max(hi, n.rank)
			}
		}
		if hi < 0 {
			continue
		}
		title := s.lab.h + pad
		switch g.dir {
		case "TB":
			before[lo] += title + pad
			after[hi] += pad
		case "BT":
			before[lo] += pad
			after[hi] += title + pad
		default:
			before[lo] += pad
			after[hi] += pad
		}
	}
	step := em * 1.3
	if hor {
		step = em * 1.6
	}
	L.rankY = make([]float64, L.ranks)
	y := 0.0
	for r := range L.ranks {
		if r > 0 {
			y += thick[r-1]/2 + after[r-1] + step + before[r] + thick[r]/2
			if thick[r] == 0 && thick[r-1] == 0 {
				y -= step / 2
			}
		}
		L.rankY[r] = y
	}
	for _, n := range g.nodes {
		n.y = L.rankY[n.rank]
	}
	return L
}

func (L *laid) setOrder(g *graph) {
	for _, ly := range L.layers {
		for i, u := range ly {
			g.nodes[u].order = i
		}
	}
}

func (L *laid) copyLayers() [][]int {
	out := make([][]int, len(L.layers))
	for i, ly := range L.layers {
		out[i] = append([]int(nil), ly...)
	}
	return out
}

// byBary sorts rank r by its nodes' neighbours' (nb's) mean place.
func (L *laid) byBary(g *graph, r int, nb [][]int) {
	ly := L.layers[r]
	bary := map[int]float64{}
	for _, u := range ly {
		if len(nb[u]) == 0 {
			bary[u] = float64(g.nodes[u].order)
			continue
		}
		s := 0.0
		for _, v := range nb[u] {
			s += float64(g.nodes[v].order)
		}
		bary[u] = s / float64(len(nb[u]))
	}
	g.sortLayer(ly, func(u int) float64 { return bary[u] })
	for i, u := range ly {
		g.nodes[u].order = i
	}
}

// crossings counts the edges that cross between ranks.
func (L *laid) crossings(g *graph) int {
	n := 0
	for r := 0; r+1 < L.ranks; r++ {
		type seg struct{ a, b int }
		var ss []seg
		for _, u := range L.layers[r] {
			for _, v := range L.succ[u] {
				ss = append(ss, seg{g.nodes[u].order, g.nodes[v].order})
			}
		}
		for i := range ss {
			for j := i + 1; j < len(ss); j++ {
				if (ss[i].a-ss[j].a)*(ss[i].b-ss[j].b) < 0 {
					n++
				}
			}
		}
	}
	return n
}

// path is a subgraph's subgraphs, the outermost first, it last.
func (g *graph) path(s int) []int {
	var p []int
	for ; s >= 0; s = g.subs[s].parent {
		p = append([]int{s}, p...)
	}
	return p
}

// common is the innermost subgraph both a and b are in.
func (g *graph) common(a, b int) int {
	pa, pb := g.path(a), g.path(b)
	c := -1
	for i := 0; i < len(pa) && i < len(pb) && pa[i] == pb[i]; i++ {
		c = pa[i]
	}
	return c
}

// sortLayer sorts ly by key, each subgraph's nodes kept together (where
// the mean of their keys puts them).
func (g *graph) sortLayer(ly []int, key func(int) float64) {
	type unit struct {
		key   float64
		nodes []int
		first int
	}
	var sortAt func(us []int, depth int) []int
	sortAt = func(us []int, depth int) []int {
		groups := map[int]*unit{}
		var units []*unit
		for i, u := range us {
			p := g.path(g.nodes[u].sub)
			if len(p) <= depth {
				k := key(u)
				if b := g.nodes[u].border; b != 0 {
					k = float64(b) * math.Inf(1)
				}
				units = append(units, &unit{key: k, nodes: []int{u}, first: i})
				continue
			}
			s := p[depth]
			gr, ok := groups[s]
			if !ok {
				gr = &unit{first: i}
				groups[s] = gr
				units = append(units, gr)
			}
			gr.nodes = append(gr.nodes, u)
			gr.key += key(u)
		}
		for _, gr := range groups {
			gr.key /= float64(len(gr.nodes))
			gr.nodes = sortAt(gr.nodes, depth+1)
		}
		sort.SliceStable(units, func(i, j int) bool { return units[i].key < units[j].key })
		var out []int
		for _, u := range units {
			out = append(out, u.nodes...)
		}
		return out
	}
	copy(ly, sortAt(append([]int(nil), ly...), 0))
}

// isotonic is the non-decreasing xs nearest ys, weights w (least
// squares): pools of neighbours that would go the wrong way merged at
// their mean.
func isotonic(ys, w []float64) []float64 {
	type pool struct {
		sum, w float64
		n      int
	}
	var ps []pool
	for i, y := range ys {
		ps = append(ps, pool{y * w[i], w[i], 1})
		for len(ps) > 1 {
			a, b := ps[len(ps)-2], ps[len(ps)-1]
			if a.sum/a.w <= b.sum/b.w {
				break
			}
			ps = append(ps[:len(ps)-2], pool{a.sum + b.sum, a.w + b.w, a.n + b.n})
		}
	}
	out := make([]float64, 0, len(ys))
	for _, p := range ps {
		for range p.n {
			out = append(out, p.sum/p.w)
		}
	}
	return out
}

// real is a layout point turned as the chart goes: x across, y along.
func (g *graph) real(x, y, maxY float64) pt {
	switch g.dir {
	case "BT":
		return pt{x, maxY - y}
	case "LR":
		return pt{y, x}
	case "RL":
		return pt{maxY - y, x}
	}
	return pt{x, y}
}
