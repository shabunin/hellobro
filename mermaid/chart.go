package mermaid

// Charts on axes: plots of functions (our own little language, a
// ```plot block) and Mermaid's xychart-beta (bars and lines over
// categories). The axes have round numbers on them, a light grid, and
// the zero lines darker.
//
// A plot:
//
//	title Damped wave
//	x: -10 .. 10          (the x range; -10 .. 10 if it's not said)
//	y: -1.5 .. 1.5        (else as high and low as the functions go)
//	y = sin(x)/x : sinc   (a function, and its name in the legend)
//	g(x) = exp(-x/5)
//	x = 2                 (an upright line)
//	points: (0, 1) (3.14, 0) : roots
//	xlabel: t, s
//	ylabel: amplitude

import (
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// axes are a chart's: where its box is in the picture, and the values
// at its edges.
type axes struct {
	x0, y0, x1, y1         float64
	xlo, xhi, ylo, yhi     float64
	cats                   []string // the x axis's categories, if it has them
	xlabel, ylabel, ytitle string
}

func (a *axes) X(v float64) float64 { return a.x0 + (v-a.xlo)/(a.xhi-a.xlo)*(a.x1-a.x0) }
func (a *axes) Y(v float64) float64 { return a.y1 - (v-a.ylo)/(a.yhi-a.ylo)*(a.y1-a.y0) }

// niceTicks are round numbers from lo to hi, about n of them: steps of
// 1, 2 or 5 times a power of ten.
func niceTicks(lo, hi float64, n int) []float64 {
	if !(hi > lo) || math.IsInf(hi-lo, 0) {
		return nil
	}
	raw := (hi - lo) / float64(max(1, n))
	p := math.Pow(10, math.Floor(math.Log10(raw)))
	step := p
	for _, k := range []float64{1, 2, 2.5, 5, 10} {
		if k*p >= raw {
			step = k * p
			break
		}
	}
	var ts []float64
	for v := math.Ceil(lo/step-1e-9) * step; v <= hi+step*1e-9 && len(ts) < 50; v += step {
		if math.Abs(v) < step*1e-9 {
			v = 0
		}
		ts = append(ts, v)
	}
	return ts
}

// tickText is v as a tick says it: no more figures than it needs.
func tickText(v float64) string {
	if v == 0 {
		return "0"
	}
	if a := math.Abs(v); a >= 1e6 || a < 1e-4 {
		return strconv.FormatFloat(v, 'g', 3, 64)
	}
	s := strconv.FormatFloat(v, 'f', 6, 64)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	return strings.Replace(s, "-", "−", 1)
}

// drawAxes draws the grid, the zero lines, the box and the ticks'
// numbers (or the categories under their bands).
func (a *axes) draw(p *picture, d *drawer) {
	em := d.em
	lw := max(1.2, em*0.07)
	small := func(s string) *label { return d.label(s, 0) }
	box := []pt{{a.x0, a.y0}, {a.x1, a.y0}, {a.x1, a.y1}, {a.x0, a.y1}}
	p.fill(box, 0xff)
	for _, v := range niceTicks(a.ylo, a.yhi, 6) {
		y := a.Y(v)
		g := uint8(0xd0)
		if v == 0 && a.ylo < 0 {
			g = 0x30
		}
		p.stroke([]pt{{a.x0, y}, {a.x1, y}}, lw*0.8, 0, g)
		lb := small(tickText(v))
		p.label(lb, pt{a.x0 - em*0.35 - lb.w/2, y})
	}
	if a.cats != nil {
		bw := (a.x1 - a.x0) / float64(len(a.cats))
		for i, c := range a.cats {
			lb := d.label(c, bw*0.95)
			p.label(lb, pt{a.x0 + bw*(float64(i)+0.5), a.y1 + em*0.3 + lb.h/2})
		}
	} else {
		for _, v := range niceTicks(a.xlo, a.xhi, 7) {
			x := a.X(v)
			g := uint8(0xd0)
			if v == 0 && a.xlo < 0 {
				g = 0x30
			}
			p.stroke([]pt{{x, a.y0}, {x, a.y1}}, lw*0.8, 0, g)
			lb := small(tickText(v))
			p.label(lb, pt{x, a.y1 + em*0.3 + lb.h/2})
		}
	}
	p.stroke(closed(box), lw, 0, 0x50)
	if a.xlabel != "" {
		lb := d.label(a.xlabel, a.x1-a.x0)
		p.label(lb, pt{(a.x0 + a.x1) / 2, a.y1 + em*1.9 + lb.h/2})
	}
	if a.ylabel != "" {
		lb := d.label(a.ylabel, 0)
		p.label(lb, pt{a.x0 + lb.w/2, a.y0 - em*0.4 - lb.h/2})
	}
}

// --- plots ---

type plotFn struct {
	f     fn
	label string
	vert  bool    // x = c: an upright line at c
	c     float64 // its c
}

type plotPts struct {
	ps    []pt
	label string
}

type plot struct {
	title          string
	xlo, xhi       float64
	ylo, yhi       float64
	yset           bool
	fns            []plotFn
	pts            []plotPts
	xlabel, ylabel string
}

var (
	rangeRE  = regexp.MustCompile(`^([xy])\s*(?:-?range|from|in|:|=|∈)?\s*[\[(]?\s*(.+?)\s*(?:\.\.\.?|…|\bto\b|,|;)\s*(.+?)\s*[\])]?$`)
	fnRE     = regexp.MustCompile(`^(?:y|[a-zA-Z]\w*\s*\(\s*x\s*\))\s*=\s*(.+)$`)
	vertRE   = regexp.MustCompile(`^x\s*=\s*(.+)$`)
	pointsRE = regexp.MustCompile(`^points?\s*:?\s*(.*)$`)
	pairRE   = regexp.MustCompile(`\(\s*([^,()]+(?:\([^()]*\))?[^,()]*)\s*,\s*([^()]+?(?:\([^()]*\))?[^()]*?)\s*\)`)
	labelRE  = regexp.MustCompile(`^(.*?)\s*(?:(?:[:;]|,\s*label\s*[:=]?)\s*"?([^":;]*)"?|"([^"]*)")\s*$`)
	xlabRE   = regexp.MustCompile(`(?i)^([xy])[- ]?(?:label|axis)\s*:?\s*"?(.*?)"?$`)
)

// points reads a line of points: (x, y) pairs, and a name after them.
func (pl *plot) points(l string) {
	body, name := l, ""
	if i := strings.LastIndex(l, ")"); i >= 0 {
		body = l[:i+1]
		_, name = cutLabel("x" + l[i+1:])
	}
	var ps []pt
	for _, m := range pairRE.FindAllStringSubmatch(body, -1) {
		x, ok1 := number(m[1])
		y, ok2 := number(m[2])
		if ok1 && ok2 {
			ps = append(ps, pt{x, y})
		}
	}
	if len(ps) > 0 {
		pl.pts = append(pl.pts, plotPts{ps, name})
	}
}

// number is a bound written as a formula: -2pi, 1e3, 3/2.
func number(s string) (float64, bool) {
	f, err := parseFormula(s)
	if err != nil {
		return 0, false
	}
	v := f(0)
	return v, !math.IsNaN(v) && !math.IsInf(v, 0)
}

// cutLabel is a line less the name at its end (": sinc", "; sinc",
// "sinc" in quotes), and the name.
func cutLabel(s string) (string, string) {
	if m := labelRE.FindStringSubmatch(s); m != nil && strings.TrimSpace(m[1]) != "" {
		return strings.TrimSpace(m[1]), strings.TrimSpace(m[2] + m[3])
	}
	return s, ""
}

func parsePlot(lines []string) *plot {
	pl := &plot{xlo: -10, xhi: 10}
	if f := strings.Fields(lines[0]); len(f) > 1 && f[1] == "title" {
		pl.title = strings.Join(f[2:], " ")
	}
	for _, l := range lines[1:] {
		l = strings.TrimSuffix(strings.TrimSpace(l), ";")
		low := strings.ToLower(l)
		if strings.HasPrefix(low, "title") {
			pl.title = strings.Trim(strings.TrimSpace(strings.TrimLeft(l[5:], " :")), `"`)
			continue
		}
		if m := xlabRE.FindStringSubmatch(l); m != nil {
			if strings.EqualFold(m[1], "x") {
				pl.xlabel = m[2]
			} else {
				pl.ylabel = m[2]
			}
			continue
		}
		if pointsRE.MatchString(low) && strings.Contains(l, "(") {
			pl.points(l)
			continue
		}
		ranged := strings.Contains(l, "..") || strings.Contains(l, "…") || strings.Contains(low, " to ")
		if m := rangeRE.FindStringSubmatch(l); m != nil && (ranged || !strings.Contains(l, "=")) {
			lo, ok1 := number(m[2])
			hi, ok2 := number(m[3])
			if ok1 && ok2 && hi > lo {
				if strings.EqualFold(m[1], "x") {
					pl.xlo, pl.xhi = lo, hi
				} else {
					pl.ylo, pl.yhi, pl.yset = lo, hi, true
				}
				continue
			}
		}
		body := l
		if m := fnRE.FindStringSubmatch(l); m != nil {
			body = m[1]
		} else if m := vertRE.FindStringSubmatch(l); m != nil {
			b, name := cutLabel(m[1])
			if c, ok := number(b); ok {
				pl.fns = append(pl.fns, plotFn{vert: true, c: c, label: name})
			}
			continue
		}
		b, name := cutLabel(body)
		if f, err := parseFormula(b); err == nil {
			pl.fns = append(pl.fns, plotFn{f: f, label: name})
		}
	}
	return pl
}

// plotLooks are the curves' looks in turn: gray, and dashes (0: none,
// <0: dots).
var plotLooks = []struct {
	g    uint8
	dash float64
}{{0, 0}, {0, 0.45}, {0x70, 0}, {0, -1}, {0x70, 0.45}, {0x40, 0.2}}

const plotSamples = 900

func (pl *plot) draw(d *drawer) *picture {
	em := d.em
	lw := max(1.2, em*0.07)
	p := &picture{}
	// the curves' points, sampled
	curves := make([][]pt, len(pl.fns))
	ylo, yhi := math.Inf(1), math.Inf(-1)
	for i, f := range pl.fns {
		if f.vert {
			continue
		}
		var ys []float64
		for k := 0; k <= plotSamples; k++ {
			x := pl.xlo + (pl.xhi-pl.xlo)*float64(k)/plotSamples
			y := f.f(x)
			curves[i] = append(curves[i], pt{x, y})
			if !math.IsNaN(y) && !math.IsInf(y, 0) {
				ys = append(ys, y)
			}
		}
		if len(ys) > 0 {
			lo, hi := curveRange(ys)
			ylo, yhi = min(ylo, lo), max(yhi, hi)
		}
	}
	for _, pp := range pl.pts {
		for _, q := range pp.ps {
			ylo, yhi = min(ylo, q.y), max(yhi, q.y)
		}
	}
	if pl.yset {
		ylo, yhi = pl.ylo, pl.yhi
	} else {
		ylo, yhi = padRange(ylo, yhi)
	}
	a := &axes{x0: 0, y0: 0, x1: em * 24, y1: em * 15, xlo: pl.xlo, xhi: pl.xhi, ylo: ylo, yhi: yhi, xlabel: pl.xlabel, ylabel: pl.ylabel}
	a.draw(p, d)
	clip := func(ps []pt) [][]pt { return clipCurve(ps, a) }
	for i, f := range pl.fns {
		look := plotLooks[i%len(plotLooks)]
		w := lw * 1.7
		segs := clip(curves[i])
		if f.vert {
			segs = nil
			if f.c >= a.xlo && f.c <= a.xhi {
				segs = [][]pt{{{a.X(f.c), a.y0}, {a.X(f.c), a.y1}}}
			}
		}
		for _, seg := range segs {
			if look.dash < 0 {
				p.stroke(seg, w*1.3, em*0.12, look.g)
			} else {
				p.stroke(seg, w, look.dash*em, look.g)
			}
		}
	}
	for _, pp := range pl.pts {
		for _, q := range pp.ps {
			if q.x < a.xlo || q.x > a.xhi || q.y < a.ylo || q.y > a.yhi {
				continue
			}
			c := pt{a.X(q.x), a.Y(q.y)}
			p.fill(circle(c, em*0.2, 16), 0)
		}
	}
	// the legend, top right in the box: each curve's look and name
	var rows []legendRow
	for i, f := range pl.fns {
		if f.label != "" {
			rows = append(rows, legendRow{lab: d.label(f.label, em*10), look: i})
		}
	}
	for _, pp := range pl.pts {
		if pp.label != "" {
			rows = append(rows, legendRow{lab: d.label(pp.label, em*10), look: -1})
		}
	}
	drawLegend(p, a, rows, em, lw)
	if pl.title != "" {
		t := d.label("**"+pl.title+"**", a.x1-a.x0)
		top := a.y0 - em*0.5
		if pl.ylabel != "" {
			top -= em * 1.5
		}
		p.label(t, pt{(a.x0 + a.x1) / 2, top - t.h/2})
	}
	return p
}

type legendRow struct {
	lab   *label
	look  int // the curve's: plotLooks; -1 points
	bar   uint8
	isBar bool
}

func drawLegend(p *picture, a *axes, rows []legendRow, em, lw float64) {
	if len(rows) == 0 {
		return
	}
	sw := em * 1.8
	w, h := 0.0, em*0.3
	for _, r := range rows {
		w = max(w, r.lab.w)
		h += max(r.lab.h, em) + em*0.15
	}
	w += sw + em*0.9
	x1, y0 := a.x1-em*0.3, a.y0+em*0.3
	x0 := x1 - w
	box := []pt{{x0, y0}, {x1, y0}, {x1, y0 + h}, {x0, y0 + h}}
	p.fill(box, 0xff)
	p.stroke(closed(box), lw*0.8, 0, 0x80)
	y := y0 + em*0.3
	for _, r := range rows {
		rh := max(r.lab.h, em)
		cy := y + rh/2
		sx := x0 + em*0.3
		switch {
		case r.isBar:
			sq := []pt{{sx + sw*0.25, cy - em*0.35}, {sx + sw*0.75, cy - em*0.35}, {sx + sw*0.75, cy + em*0.35}, {sx + sw*0.25, cy + em*0.35}}
			p.fill(sq, r.bar)
			p.stroke(closed(sq), lw*0.8, 0, 0)
		case r.look < 0:
			p.fill(circle(pt{sx + sw/2, cy}, em*0.2, 16), 0)
		default:
			look := plotLooks[r.look%len(plotLooks)]
			seg := []pt{{sx, cy}, {sx + sw, cy}}
			switch {
			case look.dash < 0:
				p.stroke(seg, lw*2.2, em*0.12, look.g)
			default:
				p.stroke(seg, lw*1.7, look.dash*em*0.6, look.g)
			}
		}
		p.label(r.lab, pt{sx + sw + em*0.3 + r.lab.w/2, cy})
		y += rh + em*0.15
	}
}

// curveRange is how high and low a curve goes: but for the few far
// out (a tan's near its poles), if they'd squash the rest.
func curveRange(vs []float64) (float64, float64) {
	s := slices.Clone(vs)
	slices.Sort(s)
	lo, hi := s[0], s[len(s)-1]
	plo, phi := s[len(s)*5/100], s[len(s)-1-len(s)*5/100]
	if hi-lo > 4*(phi-plo) && phi > plo {
		return plo - (phi-plo)*0.25, phi + (phi-plo)*0.25
	}
	return lo, hi
}

// padRange is lo to hi with room round it; from zero if it's near.
func padRange(lo, hi float64) (float64, float64) {
	if math.IsInf(lo, 0) || math.IsInf(hi, 0) {
		return -1, 1
	}
	if hi-lo < 1e-12 {
		return lo - 1, hi + 1
	}
	pad := (hi - lo) * 0.08
	lo, hi = lo-pad, hi+pad
	if lo > 0 && lo < (hi-lo)*0.3 {
		lo = 0
	}
	if hi < 0 && -hi < (hi-lo)*0.3 {
		hi = 0
	}
	return lo, hi
}

// clipCurve is a curve's points in the picture, cut where it leaves the
// box (on the edge) and where it's not defined, or jumps from far
// above to far below (tan's poles).
func clipCurve(ps []pt, a *axes) [][]pt {
	var segs [][]pt
	var cur []pt
	end := func() {
		if len(cur) > 1 {
			segs = append(segs, cur)
		}
		cur = nil
	}
	ok := func(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
	edge := func(p, q pt, y float64) pt { // where p to q crosses y
		t := (y - p.y) / (q.y - p.y)
		return pt{p.x + (q.x-p.x)*t, y}
	}
	in := func(v float64) bool { return v >= a.ylo && v <= a.yhi }
	put := func(q pt) { cur = append(cur, pt{a.X(q.x), a.Y(q.y)}) }
	for i := 1; i < len(ps); i++ {
		p, q := ps[i-1], ps[i]
		if !ok(p.y) || !ok(q.y) || p.y > a.yhi && q.y < a.ylo || p.y < a.ylo && q.y > a.yhi {
			end()
			continue
		}
		switch {
		case in(p.y) && in(q.y):
			if len(cur) == 0 {
				put(p)
			}
			put(q)
		case in(p.y): // leaves
			if len(cur) == 0 {
				put(p)
			}
			put(edge(p, q, min(a.yhi, max(a.ylo, q.y))))
			end()
		case in(q.y): // comes in
			put(edge(p, q, min(a.yhi, max(a.ylo, p.y))))
			put(q)
		}
	}
	end()
	return segs
}

// --- xychart-beta ---

type xySeries struct {
	bar  bool
	name string
	vs   []float64
}

type xyChart struct {
	title          string
	cats           []string
	xlo, xhi       float64
	xnum           bool
	xtitle, ytitle string
	ylo, yhi       float64
	yset           bool
	series         []xySeries
}

var (
	xyAxis     = regexp.MustCompile(`^([xy])-axis\s*(?:"([^"]*)"|([^\s\[\d"-][^\[]*?))?\s*(?:\[(.*)\]|(-?[\d.eE+-]+)\s*-->\s*(-?[\d.eE+-]+))?\s*$`)
	xySeriesRE = regexp.MustCompile(`^(bar|line)\s*(?:"([^"]*)")?\s*\[(.*)\]\s*$`)
)

func splitList(s string) []string {
	var out []string
	for _, f := range strings.Split(s, ",") {
		out = append(out, clean(strings.TrimSpace(f)))
	}
	return out
}

func parseXY(lines []string) *xyChart {
	c := &xyChart{}
	for _, l := range lines[1:] {
		switch {
		case strings.HasPrefix(l, "title"):
			c.title = clean(strings.TrimSpace(l[5:]))
		case xyAxis.MatchString(l):
			m := xyAxis.FindStringSubmatch(l)
			title := strings.TrimSpace(m[2] + m[3])
			lo, err1 := strconv.ParseFloat(m[5], 64)
			hi, err2 := strconv.ParseFloat(m[6], 64)
			if m[1] == "x" {
				c.xtitle = title
				if m[4] != "" {
					c.cats = splitList(m[4])
				} else if err1 == nil && err2 == nil && hi > lo {
					c.xlo, c.xhi, c.xnum = lo, hi, true
				}
			} else {
				c.ytitle = title
				if err1 == nil && err2 == nil && hi > lo {
					c.ylo, c.yhi, c.yset = lo, hi, true
				}
			}
		case xySeriesRE.MatchString(l):
			m := xySeriesRE.FindStringSubmatch(l)
			s := xySeries{bar: m[1] == "bar", name: m[2]}
			for _, f := range splitList(m[3]) {
				if v, err := strconv.ParseFloat(f, 64); err == nil && !math.IsInf(v, 0) {
					s.vs = append(s.vs, v)
				}
			}
			if len(s.vs) > 0 {
				c.series = append(c.series, s)
			}
		}
	}
	return c
}

var barGrays = []uint8{0x60, 0xb8, 0x30, 0xd8}

func (c *xyChart) draw(d *drawer) *picture {
	em := d.em
	lw := max(1.2, em*0.07)
	p := &picture{}
	n := 0
	var vs []float64
	bars := 0
	for _, s := range c.series {
		n = max(n, len(s.vs))
		vs = append(vs, s.vs...)
		if s.bar {
			bars++
		}
	}
	cats := c.cats
	for len(cats) < n {
		if c.xnum && n > 1 {
			cats = append(cats, tickText(c.xlo+(c.xhi-c.xlo)*float64(len(cats))/float64(n-1)))
		} else {
			cats = append(cats, strconv.Itoa(len(cats)+1))
		}
	}
	n = len(cats)
	ylo, yhi := c.ylo, c.yhi
	if !c.yset {
		lo, hi := slices.Min(vs), slices.Max(vs)
		ylo, yhi = min(0, lo), max(0, hi)
		pad := (yhi - ylo) * 0.08
		if hi > 0 {
			yhi += pad
		}
		if lo < 0 {
			ylo -= pad
		}
		if yhi == ylo {
			yhi++
		}
	}
	width := max(em*20, min(em*34, float64(n)*em*2.6))
	a := &axes{x0: 0, y0: 0, x1: width, y1: em * 14, xlo: 0, xhi: float64(n), ylo: ylo, yhi: yhi, cats: cats, xlabel: c.xtitle, ylabel: c.ytitle}
	a.draw(p, d)
	band := (a.x1 - a.x0) / float64(n)
	bi := 0
	var rows []legendRow
	for _, s := range c.series {
		if !s.bar {
			continue
		}
		bw := band * 0.7 / float64(bars)
		g := barGrays[bi%len(barGrays)]
		for i, v := range s.vs {
			x := a.x0 + band*float64(i) + band*0.15 + bw*float64(bi)
			y0, y1 := a.Y(max(a.ylo, min(a.yhi, v))), a.Y(max(a.ylo, min(a.yhi, 0)))
			r := []pt{{x, y0}, {x + bw, y0}, {x + bw, y1}, {x, y1}}
			p.fill(r, g)
			p.stroke(closed(r), lw*0.8, 0, 0)
		}
		if s.name != "" {
			rows = append(rows, legendRow{lab: d.label(s.name, em*10), isBar: true, bar: g})
		}
		bi++
	}
	li := 0
	for _, s := range c.series {
		if s.bar {
			continue
		}
		look := plotLooks[li%len(plotLooks)]
		var ps []pt
		for i, v := range s.vs {
			ps = append(ps, pt{a.x0 + band*(float64(i)+0.5), a.Y(max(a.ylo, min(a.yhi, v)))})
		}
		p.stroke(ps, lw*2, look.dash*em, look.g)
		for _, q := range ps {
			p.fill(circle(q, em*0.18, 12), look.g)
		}
		if s.name != "" {
			rows = append(rows, legendRow{lab: d.label(s.name, em*10), look: li})
		}
		li++
	}
	drawLegend(p, a, rows, em, lw)
	if c.title != "" {
		t := d.label("**"+c.title+"**", a.x1-a.x0)
		top := a.y0 - em*0.5
		if c.ytitle != "" {
			top -= em * 1.5
		}
		p.label(t, pt{(a.x0 + a.x1) / 2, top - t.h/2})
	}
	return p
}
