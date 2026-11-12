// Package mermaid draws the diagrams a language model writes in
// Mermaid's language: flowcharts (graph, flowchart), state, sequence,
// class and entity-relationship diagrams, Gantt charts, timelines,
// mind maps, pies and xychart-beta's bars and lines; and plots of
// functions in a little language of our own (chart.go). Not Mermaid
// itself: its language as
// models write it, laid out by our own rules (a flowchart in ranks, as
// dagre lays them out) and drawn black on white, for e-ink. Labels are
// Markdown (bold, italic, code, <br>) with formulas in TeX (package
// tex).
package mermaid

import (
	"errors"
	"image"
	"image/color"
	"image/draw"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"

	"hellobro/md"
	"hellobro/tex"
	"hellobro/third_party/fonts/gentium"
)

// Options are how it's drawn: letters Em px, in Face's fonts.
type Options struct {
	Em   float64
	Face func(st md.Style, px float64) font.Face // nil: Gentium Book, Go Mono
}

// Result is a diagram drawn, black on white; Kind is what it was:
// "flowchart", "state", "sequence", "class", "er", "gantt",
// "timeline", "mindmap", "pie", "xychart", "plot".
type Result struct {
	Img  *image.Gray
	Kind string
}

// ErrUnknown is a diagram it doesn't draw (a quadrant chart, a
// journey...) or nothing it can read.
var ErrUnknown = errors.New("mermaid: not a diagram it draws")

// maxSide is the most pixels a side has: a runaway diagram is cut there.
const maxSide = 4000

// Render draws src.
func Render(src string, o Options) (Result, error) {
	if o.Em <= 0 {
		o.Em = 20
	}
	if o.Face == nil {
		o.Face = defaultFace
	}
	lines := prepare(src)
	if len(lines) == 0 {
		return Result{}, ErrUnknown
	}
	d := &drawer{em: o.Em, faces: o.Face}
	kw := strings.Fields(lines[0])[0]
	var p *picture
	var kind string
	switch {
	case kw == "graph" || kw == "flowchart" || kw == "flowchart-elk":
		g := parseFlow(lines)
		if len(g.nodes) == 0 {
			return Result{}, ErrUnknown
		}
		kind, p = "flowchart", g.draw(d)
	case strings.HasPrefix(kw, "stateDiagram"):
		g := parseState(lines)
		if len(g.nodes) == 0 {
			return Result{}, ErrUnknown
		}
		kind, p = "state", g.draw(d)
	case kw == "sequenceDiagram":
		s := parseSeq(lines)
		if len(s.parts) == 0 {
			return Result{}, ErrUnknown
		}
		kind, p = "sequence", s.draw(d)
	case kw == "classDiagram" || kw == "classDiagram-v2":
		g := parseClass(lines)
		if len(g.nodes) == 0 {
			return Result{}, ErrUnknown
		}
		kind, p = "class", g.draw(d)
	case kw == "erDiagram":
		g := parseER(lines)
		if len(g.nodes) == 0 {
			return Result{}, ErrUnknown
		}
		kind, p = "er", g.draw(d)
	case kw == "plot":
		pl := parsePlot(lines)
		if len(pl.fns) == 0 && len(pl.pts) == 0 {
			return Result{}, ErrUnknown
		}
		kind, p = "plot", pl.draw(d)
	case kw == "xychart-beta" || kw == "xychart":
		c := parseXY(lines)
		if len(c.series) == 0 {
			return Result{}, ErrUnknown
		}
		kind, p = "xychart", c.draw(d)
	case kw == "gantt":
		g := parseGantt(lines)
		if len(g.tasks) == 0 {
			return Result{}, ErrUnknown
		}
		kind, p = "gantt", g.draw(d)
	case kw == "timeline":
		tl := parseTimeline(lines)
		if len(tl.periods) == 0 {
			return Result{}, ErrUnknown
		}
		kind, p = "timeline", tl.draw(d)
	case kw == "mindmap":
		r := parseMindmap(rawLines(src))
		if r == nil {
			return Result{}, ErrUnknown
		}
		kind, p = "mindmap", r.draw(d)
	case kw == "pie":
		pc := parsePie(lines)
		if len(pc.slices) == 0 {
			return Result{}, ErrUnknown
		}
		kind, p = "pie", pc.draw(d)
	default:
		return Result{}, ErrUnknown
	}
	return Result{Img: p.render(o.Em), Kind: kind}, nil
}

// prepare is src's lines, trimmed, without comments, front matter and
// init directives: the first is the diagram's kind.
func prepare(src string) []string {
	var out []string
	front := false
	for i, l := range strings.Split(strings.ReplaceAll(src, "\r", ""), "\n") {
		t := strings.TrimSpace(l)
		if t == "---" && (i == 0 || front) {
			front = !front
			continue
		}
		if front || t == "" || strings.HasPrefix(t, "%%") {
			continue
		}
		out = append(out, t)
	}
	return out
}

// defaultFace is Gentium Book in a style, or Go Mono for code.
func defaultFace(st md.Style, px float64) font.Face {
	data := [4][]byte{gentium.TTF, gentium.Bold, gentium.Italic, gentium.BoldItalic}[int(st&(md.Bold|md.Italic))]
	if st&md.Mono != 0 {
		data = gomono.TTF
	}
	return cachedFace(data, px)
}

var (
	facesMu sync.Mutex
	faces   = map[[2]uintptr]font.Face{}
	parsed  = map[uintptr]*opentype.Font{}
)

func cachedFace(data []byte, px float64) font.Face {
	facesMu.Lock()
	defer facesMu.Unlock()
	id := uintptr(len(data)) // the fonts are told apart by their length
	k := [2]uintptr{id, uintptr(math.Round(px * 4))}
	if f, ok := faces[k]; ok {
		return f
	}
	fnt, ok := parsed[id]
	if !ok {
		var err error
		if fnt, err = opentype.Parse(data); err != nil {
			return nil
		}
		parsed[id] = fnt
	}
	f, err := opentype.NewFace(fnt, &opentype.FaceOptions{Size: float64(k[1]) / 4, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		return nil
	}
	faces[k] = f
	return f
}

// pt is a point, px.
type pt struct{ x, y float64 }

func (p pt) add(q pt) pt         { return pt{p.x + q.x, p.y + q.y} }
func (p pt) sub(q pt) pt         { return pt{p.x - q.x, p.y - q.y} }
func (p pt) mul(k float64) pt    { return pt{p.x * k, p.y * k} }
func (p pt) len() float64        { return math.Hypot(p.x, p.y) }
func mid(p, q pt) pt             { return pt{(p.x + q.x) / 2, (p.y + q.y) / 2} }
func lerp(p, q pt, t float64) pt { return pt{p.x + (q.x-p.x)*t, p.y + (q.y-p.y)*t} }
func (p pt) unit() pt {
	l := p.len()
	if l == 0 {
		return pt{}
	}
	return pt{p.x / l, p.y / l}
}

// drawer measures labels: letters em px in faces.
type drawer struct {
	em    float64
	faces func(st md.Style, px float64) font.Face
}

func (d *drawer) face(st md.Style) font.Face { return d.faces(st, d.em) }

// atom is a word in a face, or a formula drawn, at x in its line.
type atom struct {
	text      string
	face      font.Face
	img       *image.Gray
	x, w      float64
	asc, desc float64
}

type labLine struct {
	atoms     []atom
	w         float64
	asc, desc float64
}

// label is a label laid out in lines.
type label struct {
	lines []labLine
	w, h  float64
	lead  float64
}

var (
	entityRE = regexp.MustCompile(`#(\d+|[a-z]+);`)
	newlRE   = regexp.MustCompile(`\\n([^a-zA-Z]|$)`)
	entities = map[string]string{"quot": `"`, "amp": "&", "lt": "<", "gt": ">", "nbsp": " ", "apos": "'"}
)

// clean is a label's text as written: its quotes off, its entities
// and \n read.
func clean(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = strings.TrimSpace(s[1 : len(s)-1])
	}
	if len(s) >= 2 && s[0] == '`' && s[len(s)-1] == '`' { // a Markdown string
		s = s[1 : len(s)-1]
	}
	s = entityRE.ReplaceAllStringFunc(s, func(m string) string {
		k := m[1 : len(m)-1]
		if n, err := strconv.Atoi(k); err == nil && n > 0 && n < 0x110000 {
			return string(rune(n))
		}
		if e, ok := entities[k]; ok {
			return e
		}
		return m
	})
	return newlRE.ReplaceAllString(s, "<br>$1")
}

// label lays s out, its lines no wider than maxW (0: as long as they
// are).
func (d *drawer) label(s string, maxW float64) *label {
	lb := &label{lead: d.em * 0.15}
	var cur labLine
	push := func() {
		if len(cur.atoms) == 0 {
			m := d.face(0).Metrics()
			cur.asc, cur.desc = fx(m.Ascent), fx(m.Descent)
		}
		lb.lines = append(lb.lines, cur)
		cur = labLine{}
	}
	add := func(a atom, space bool) {
		sw := 0.0
		if space && len(cur.atoms) > 0 {
			f := a.face
			if f == nil {
				f = d.face(0)
			}
			sw = fx(font.MeasureString(f, " "))
		}
		if maxW > 0 && len(cur.atoms) > 0 && cur.w+sw+a.w > maxW {
			push()
			sw = 0
		}
		a.x = cur.w + sw
		cur.w = a.x + a.w
		cur.asc, cur.desc = max(cur.asc, a.asc), max(cur.desc, a.desc)
		cur.atoms = append(cur.atoms, a)
	}
	s = clean(s)
	if s != "" {
		space := false
		for _, sp := range md.Inline(s) {
			switch {
			case sp.Break:
				push()
				space = false
			case sp.Math:
				r := tex.Render(sp.Text, false, d.em)
				b := r.Img.Bounds()
				add(atom{img: r.Img, w: float64(b.Dx()), asc: float64(r.Baseline), desc: float64(b.Dy() - r.Baseline)}, space)
				space = false
			default:
				f := d.face(sp.Style & (md.Bold | md.Italic | md.Mono))
				m := f.Metrics()
				t := sp.Text
				for i, w := range strings.Fields(t) {
					sp := space || i > 0 || strings.HasPrefix(t, " ")
					add(atom{text: w, face: f, w: fx(font.MeasureString(f, w)), asc: fx(m.Ascent), desc: fx(m.Descent)}, sp)
				}
				space = strings.HasSuffix(t, " ") || strings.HasSuffix(t, "\n")
			}
		}
	}
	if len(cur.atoms) > 0 || len(lb.lines) == 0 {
		push()
	}
	for i, ln := range lb.lines {
		lb.w = max(lb.w, ln.w)
		lb.h += ln.asc + ln.desc
		if i > 0 {
			lb.h += lb.lead
		}
	}
	return lb
}

func fx(v fixed.Int26_6) float64 { return float64(v) / 64 }

// picture is a diagram's drawing, kept until it's all laid out and its
// size is known.
type picture struct {
	ops            []func(c *canvas)
	x0, y0, x1, y1 float64
	any            bool
}

func (p *picture) grow(ps ...pt) {
	for _, q := range ps {
		if !p.any {
			p.x0, p.y0, p.x1, p.y1, p.any = q.x, q.y, q.x, q.y, true
			continue
		}
		p.x0, p.y0, p.x1, p.y1 = min(p.x0, q.x), min(p.y0, q.y), max(p.x1, q.x), max(p.y1, q.y)
	}
}

// fill fills the polygon ps in gray g.
func (p *picture) fill(ps []pt, g uint8) {
	if len(ps) < 3 {
		return
	}
	p.grow(ps...)
	p.ops = append(p.ops, func(c *canvas) { c.paint([][]pt{ps}, g) })
}

// stroke draws a line w thick through ps, in gray g, dashed if dash is
// over 0 (its dashes as long, the gaps 0.7 of it).
func (p *picture) stroke(ps []pt, w float64, dash float64, g uint8) {
	if len(ps) < 2 {
		return
	}
	p.grow(ps...)
	var polys [][]pt
	if dash > 0 {
		for _, seg := range dashes(ps, dash, dash*0.7) {
			polys = append(polys, thick(seg, w)...)
		}
	} else {
		polys = thick(ps, w)
	}
	p.ops = append(p.ops, func(c *canvas) { c.paint(polys, g) })
}

// label draws lb centered on at.
func (p *picture) label(lb *label, at pt) {
	top := pt{at.x - lb.w/2, at.y - lb.h/2}
	p.grow(top, pt{top.x + lb.w, top.y + lb.h})
	p.ops = append(p.ops, func(c *canvas) {
		y := top.y
		for i, ln := range lb.lines {
			if i > 0 {
				y += lb.lead
			}
			base := y + ln.asc
			x := at.x - ln.w/2
			for _, a := range ln.atoms {
				if a.img != nil {
					c.image(a.img, pt{x + a.x, base - a.asc})
				} else {
					c.text(a.face, a.text, pt{x + a.x, base})
				}
			}
			y = base + ln.desc
		}
	})
}

// render draws it all, a margin round it.
func (p *picture) render(em float64) *image.Gray {
	m := math.Ceil(em * 0.3)
	ox, oy := math.Floor(p.x0)-m, math.Floor(p.y0)-m
	w := min(maxSide, int(math.Ceil(p.x1-ox+m)))
	h := min(maxSide, int(math.Ceil(p.y1-oy+m)))
	c := &canvas{img: image.NewGray(image.Rect(0, 0, max(1, w), max(1, h))), off: pt{-ox, -oy}}
	for i := range c.img.Pix {
		c.img.Pix[i] = 0xff
	}
	for _, op := range p.ops {
		op(c)
	}
	return c.img
}

// canvas is where a picture's drawn: off from its points.
type canvas struct {
	img *image.Gray
	off pt
}

// paint fills polys, each a polygon, in g: a rasterizer as big as they
// are.
func (c *canvas) paint(polys [][]pt, g uint8) {
	x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, ps := range polys {
		for _, q := range ps {
			q = q.add(c.off)
			x0, y0, x1, y1 = min(x0, q.x), min(y0, q.y), max(x1, q.x), max(y1, q.y)
		}
	}
	r := image.Rect(int(math.Floor(x0)), int(math.Floor(y0)), int(math.Ceil(x1))+1, int(math.Ceil(y1))+1).Intersect(c.img.Bounds())
	if r.Empty() {
		return
	}
	ras := vector.NewRasterizer(r.Dx(), r.Dy())
	ras.DrawOp = draw.Over
	for _, ps := range polys {
		if len(ps) < 3 {
			continue
		}
		area := 0.0
		for i, q := range ps {
			n := ps[(i+1)%len(ps)]
			area += q.x*n.y - n.x*q.y
		}
		at := func(q pt) (float32, float32) {
			return float32(q.x + c.off.x - float64(r.Min.X)), float32(q.y + c.off.y - float64(r.Min.Y))
		}
		if area < 0 { // all one way round: where they overlap, they don't cancel
			ras.MoveTo(at(ps[len(ps)-1]))
			for i := len(ps) - 2; i >= 0; i-- {
				ras.LineTo(at(ps[i]))
			}
		} else {
			ras.MoveTo(at(ps[0]))
			for _, q := range ps[1:] {
				ras.LineTo(at(q))
			}
		}
		ras.ClosePath()
	}
	ras.Draw(c.img, r, image.NewUniform(color.Gray{g}), image.Point{})
}

func (c *canvas) text(f font.Face, s string, at pt) {
	at = at.add(c.off)
	dr := font.Drawer{Dst: c.img, Src: image.Black, Face: f,
		Dot: fixed.Point26_6{X: fixed.Int26_6(math.Round(at.x * 64)), Y: fixed.Int26_6(math.Round(at.y * 64))}}
	dr.DrawString(s)
}

// image puts a formula's picture, its top left at: darker wins.
func (c *canvas) image(img *image.Gray, at pt) {
	at = at.add(c.off)
	ox, oy := int(math.Round(at.x)), int(math.Round(at.y))
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			p := image.Pt(ox+x-b.Min.X, oy+y-b.Min.Y)
			if !p.In(c.img.Bounds()) {
				continue
			}
			i := c.img.PixOffset(p.X, p.Y)
			c.img.Pix[i] = min(c.img.Pix[i], img.GrayAt(x, y).Y)
		}
	}
}

// thick is a line w thick through ps: a quad a segment, a disc a
// joint.
func thick(ps []pt, w float64) [][]pt {
	r := w / 2
	var out [][]pt
	for i := 1; i < len(ps); i++ {
		q, p := ps[i-1], ps[i]
		d := p.sub(q)
		l := d.len()
		if l == 0 {
			continue
		}
		n := pt{-d.y / l * r, d.x / l * r}
		out = append(out, []pt{q.add(n), p.add(n), p.sub(n), q.sub(n)})
		if i < len(ps)-1 {
			out = append(out, circle(p, r, 8))
		}
	}
	return out
}

// dashes cuts the line through ps into dashes on long, gaps off.
func dashes(ps []pt, on, off float64) [][]pt {
	var out [][]pt
	cur := []pt{ps[0]}
	drawing, left := true, on
	for i := 1; i < len(ps); i++ {
		a, b := ps[i-1], ps[i]
		l := b.sub(a).len()
		pos := 0.0
		for l-pos > left {
			pos += left
			p := lerp(a, b, pos/l)
			if drawing {
				out = append(out, append(cur, p))
			}
			cur = []pt{p}
			drawing = !drawing
			left = map[bool]float64{true: on, false: off}[drawing]
		}
		left -= l - pos
		cur = append(cur, b)
	}
	if drawing && len(cur) > 1 {
		out = append(out, cur)
	}
	return out
}

// circle is a circle's outline, n points.
func circle(c pt, r float64, n int) []pt {
	ps := make([]pt, n)
	for i := range ps {
		a := 2 * math.Pi * float64(i) / float64(n)
		ps[i] = pt{c.x + r*math.Cos(a), c.y + r*math.Sin(a)}
	}
	return ps
}

// ellipse is an ellipse's outline, n points.
func ellipse(c pt, rx, ry float64, n int) []pt {
	ps := make([]pt, n)
	for i := range ps {
		a := 2 * math.Pi * float64(i) / float64(n)
		ps[i] = pt{c.x + rx*math.Cos(a), c.y + ry*math.Sin(a)}
	}
	return ps
}

// arc is n+1 points along the circle round c, r, from angle a0 to a1.
func arc(c pt, rx, ry, a0, a1 float64, n int) []pt {
	ps := make([]pt, n+1)
	for i := range ps {
		a := a0 + (a1-a0)*float64(i)/float64(n)
		ps[i] = pt{c.x + rx*math.Cos(a), c.y + ry*math.Sin(a)}
	}
	return ps
}

// closed is ps and its first point again.
func closed(ps []pt) []pt { return append(append([]pt(nil), ps...), ps[0]) }

// spline is a smooth curve by ps (a B-spline's, quadratic): through
// the first and the last, near the rest.
func spline(ps []pt) []pt {
	if len(ps) < 3 {
		return ps
	}
	out := []pt{ps[0]}
	for i := 1; i < len(ps)-1; i++ {
		a := mid(ps[i-1], ps[i])
		if i == 1 {
			a = ps[0]
		}
		b := mid(ps[i], ps[i+1])
		if i == len(ps)-2 {
			b = ps[len(ps)-1]
		}
		for k := 1; k <= 8; k++ {
			t := float64(k) / 8
			u := 1 - t
			out = append(out, pt{u*u*a.x + 2*u*t*ps[i].x + t*t*b.x, u*u*a.y + 2*u*t*ps[i].y + t*t*b.y})
		}
	}
	return out
}

// head draws an arrow's head of kind ('>', 'o', 'x', 'v' an open one)
// at tip, coming from dir (a unit vector along the line), size s.
func (p *picture) head(kind byte, tip, dir pt, s, lw float64) {
	n := pt{-dir.y, dir.x}
	switch kind {
	case '>':
		b := tip.sub(dir.mul(s))
		p.fill([]pt{tip, b.add(n.mul(s * 0.42)), b.sub(n.mul(s * 0.42))}, 0)
	case 'v':
		b := tip.sub(dir.mul(s))
		p.stroke([]pt{b.add(n.mul(s * 0.42)), tip, b.sub(n.mul(s * 0.42))}, lw, 0, 0)
	case 'o':
		c := tip.sub(dir.mul(s * 0.3))
		p.fill(circle(c, s*0.3, 16), 0xff)
		p.stroke(closed(circle(c, s*0.3, 16)), lw, 0, 0)
	case 'x':
		c := tip.sub(dir.mul(s * 0.3))
		k := s * 0.3
		p.stroke([]pt{c.add(dir.add(n).mul(k)), c.sub(dir.add(n).mul(k))}, lw*1.3, 0, 0)
		p.stroke([]pt{c.add(dir.sub(n).mul(k)), c.sub(dir.sub(n).mul(k))}, lw*1.3, 0, 0)
	case 't': // UML's inheritance: a hollow triangle
		b := tip.sub(dir.mul(s))
		tri := []pt{tip, b.add(n.mul(s * 0.5)), b.sub(n.mul(s * 0.5))}
		p.fill(tri, 0xff)
		p.stroke(closed(tri), lw, 0, 0)
	case 'D', 'd': // composition, filled; aggregation, hollow
		m := tip.sub(dir.mul(s * 0.65))
		dia := []pt{tip, m.add(n.mul(s * 0.36)), tip.sub(dir.mul(s * 1.3)), m.sub(n.mul(s * 0.36))}
		p.fill(dia, map[byte]uint8{'D': 0, 'd': 0xff}[kind])
		p.stroke(closed(dia), lw, 0, 0)
	case '1', 'z', 'M', 'm': // crow's feet: one, zero or one, one or more, zero or more
		bar := func(at float64) {
			c := tip.sub(dir.mul(at))
			p.stroke([]pt{c.add(n.mul(s * 0.4)), c.sub(n.mul(s * 0.4))}, lw, 0, 0)
		}
		ring := func(at float64) {
			c := tip.sub(dir.mul(at))
			p.fill(circle(c, s*0.22, 16), 0xff)
			p.stroke(closed(circle(c, s*0.22, 16)), lw, 0, 0)
		}
		if kind == 'M' || kind == 'm' {
			f := tip.sub(dir.mul(s * 0.7))
			p.stroke([]pt{tip.add(n.mul(s * 0.42)), f, tip.sub(n.mul(s * 0.42))}, lw, 0, 0)
		}
		switch kind {
		case '1':
			bar(s * 0.35)
			bar(s * 0.6)
		case 'z':
			bar(s * 0.35)
			ring(s * 0.85)
		case 'M':
			bar(s * 0.95)
		case 'm':
			ring(s * 1.0)
		}
	}
}

// headRoom is how much of a line an arrow's head of kind takes.
func headRoom(kind byte, s float64) float64 {
	switch kind {
	case '>':
		return s * 0.8
	case 'o':
		return s * 0.6
	case 't':
		return s
	case 'D', 'd':
		return s * 1.3
	}
	return 0
}
