package tex

import (
	"math"
	"strings"
)

// hlist lays atoms out in a line, TeX's spaces between them by their
// classes.
func (p *parser) hlist(as []*atom, st int) *box {
	// a binary sign with nothing on its left to work on is ordinary
	prev := -1
	var prevA *atom
	for _, a := range as {
		if a.glue {
			continue
		}
		switch a.class {
		case bin:
			if prev == -1 || prev == bin || prev == op || prev == rel || prev == open || prev == punct {
				a.class = ord
			}
		case rel, clo, punct:
			if prev == bin {
				prevA.class = ord
			}
		}
		prev, prevA = a.class, a
	}
	if prev == bin {
		prevA.class = ord
	}
	out := &box{}
	x := 0.0
	prev = -1
	mu := p.size(st) / 18
	for _, a := range as {
		if a.glue {
			out.put(a.b, x, 0)
			x += a.b.w
			continue
		}
		if prev >= 0 {
			switch s := spacing[prev][a.class]; {
			case s == 1:
				x += 3 * mu
			case s == 2 && st <= styleT:
				x += 4 * mu
			case s == 3 && st <= styleT:
				x += 5 * mu
			case s == 4 && st <= styleT:
				x += 3 * mu
			}
		}
		b := p.attach(a, st)
		out.put(b, x, 0)
		x += b.w
		prev = a.class
		out.ital = b.ital
	}
	out.w = max(0, x)
	return out
}

// attach puts an atom's scripts on it: at its side, or over and under
// it (an operator's limits in display style).
func (p *parser) attach(a *atom, st int) *box {
	nuc := a.b
	if a.sup == nil && a.sub == nil {
		return nuc
	}
	em := p.size(st)
	sup, sub := a.sup, a.sub
	if a.limits > 0 || a.limits == 0 && a.takes && st == styleD {
		gap := 0.15 * em
		w := nuc.w
		for _, s := range []*box{sup, sub} {
			if s != nil {
				w = max(w, s.w)
			}
		}
		b := &box{}
		b.put(nuc, (w-nuc.w)/2, 0)
		if sup != nil {
			b.put(sup, (w-sup.w)/2+nuc.ital/2, -(nuc.h + gap + sup.d))
		}
		if sub != nil {
			b.put(sub, (w-sub.w)/2-nuc.ital/2, nuc.d+gap+sub.h)
		}
		b.w = w
		return b
	}
	var u, v float64
	if sup != nil {
		u0 := 0.0
		if !a.char {
			u0 = nuc.h - 0.25*em
		}
		u = max(u0, map[bool]float64{true: 0.41, false: 0.36}[st == styleD]*em, sup.d+0.12*em)
	}
	if sub != nil {
		v0 := 0.0
		if !a.char {
			v0 = nuc.d + 0.05*em
		}
		if sup == nil {
			v = max(v0, 0.15*em, sub.h-0.36*em)
		} else {
			v = max(v0, 0.25*em)
			if gap := (u - sup.d) - (sub.h - v); gap < 4*p.rule(st) {
				v += 4*p.rule(st) - gap
				if lift := 0.36*em - (u - sup.d); lift > 0 {
					u += lift
					v -= lift
				}
			}
		}
	}
	b := &box{}
	b.put(nuc, 0, 0)
	x, w := nuc.w, nuc.w
	supX, subX := x+nuc.ital, x
	if a.integral { // it leans: the upper limit clear of its top, the lower in under its foot
		supX, subX = x+0.12*em, x-0.12*em*nuc.h/em
	}
	if sup != nil {
		b.put(sup, supX, -u)
		w = max(w, supX+sup.w)
	}
	if sub != nil {
		b.put(sub, subX, v)
		w = max(w, subX+sub.w)
	}
	b.w = w + 0.05*em
	return b
}

// center is b moved up or down to be as far over the axis as under it.
func (p *parser) center(b *box, st int) *box {
	dy := -p.axis(st) - (b.d-b.h)/2
	return &box{w: b.w, h: b.h - dy, d: b.d + dy, ital: b.ital, kids: []kid{{b, 0, dy}}}
}

// bigOp is a large operator: larger in display style, on the axis.
func (p *parser) bigOp(r rune, st int) *atom {
	scale := 1.0
	if st == styleD {
		scale = 1.4
		if integrals[r] {
			scale = 2
		}
	}
	return &atom{class: op, b: p.center(glyph(r, p.size(st)*scale), st), takes: !integrals[r], integral: integrals[r], char: true}
}

// frac is num over den, with a bar if rule, in st.
func (p *parser) frac(num, den *box, st int, rule bool) *box {
	em, axis, t := p.size(st), p.axis(st), p.rule(st)
	numUp, denDown, gap := 0.4*em, 0.35*em, t
	if st == styleD {
		numUp, denDown, gap = 0.68*em, 0.69*em, 3*t
	}
	if rule {
		numUp = max(numUp, axis+t/2+gap+num.d)
		denDown = max(denDown, den.h+gap+t/2-axis)
	} else if clear := (numUp - num.d) - (den.h - denDown); clear < 3*gap {
		numUp += (3*gap - clear) / 2
		denDown += (3*gap - clear) / 2
	}
	pad, over := 0.12*em, 0.08*em
	w := max(num.w, den.w) + 2*over
	b := &box{}
	b.put(num, pad+(w-num.w)/2, -numUp)
	b.put(den, pad+(w-den.w)/2, denDown)
	b.w = w + 2*pad
	if rule {
		b.paint = func(c *canvas, x, y float64) { c.rect(x+pad, y-axis-t/2, x+pad+w, y-axis+t/2) }
	}
	return b
}

// sqrt is body under a root sign, idx (if any) in its crook.
func (p *parser) sqrt(body, idx *box, st int) *box {
	em, t := p.size(st), p.rule(st)
	clr := t + 0.1*em
	if st == styleD {
		clr = t + 0.2*em
	}
	h := body.h + body.d + clr + t // from the bar's top to the body's bottom
	rw := min(0.9*em, 0.5*em+0.05*h)
	off := 0.0
	if idx != nil {
		off = max(0, idx.w-0.55*rw+0.05*em)
	}
	b := &box{}
	b.put(body, off+rw, 0)
	b.h = max(b.h, body.h+clr+t+0.03*em)
	b.d = max(b.d, body.d+0.03*em)
	b.w = off + rw + body.w + 0.1*em
	b.paint = func(c *canvas, x, y float64) {
		x += off
		top, bot := y-body.h-clr-t, y+body.d
		tick := bot - min(0.5*em, 0.45*(bot-top))
		a := pt{x + 0.06*em, tick + 0.05*em}
		k := pt{x + 0.22*rw, tick - 0.04*em}
		v := pt{x + 0.48*rw, bot}
		c.line(t, a, k)
		c.line(1.7*t, k, v)
		c.line(t, v, pt{x + rw, top + t/2})
		c.rect(x+rw-t/4, top, x+rw+body.w+0.1*em, top+t)
	}
	if idx != nil {
		lift := 0.6 * (h) // the index's baseline over the body's bottom
		b.put(idx, off+0.55*rw-0.05*em-idx.w, body.d-lift)
	}
	return b
}

// delim is a delimiter's sign, after \left, \big...: '.' for none.
func (p *parser) delim() rune {
	p.skipSpace()
	t := p.next()
	switch t.kind {
	case 'x':
		switch r := []rune(t.s)[0]; r {
		case '<':
			return '⟨'
		case '>':
			return '⟩'
		default:
			return r
		}
	case 'c':
		if s, ok := symbols[t.s]; ok {
			return s.r
		}
	}
	return '.'
}

// leftRight is \left, what's up to \right (\middle in it), \right: the
// delimiters as tall as what they hold.
func (p *parser) leftRight(st int) *atom {
	l := p.delim()
	var parts []*box
	var mids []rune
	r := '.'
	for {
		as := p.list(st, func(t tok) bool { return t.kind == 'c' && (t.s == "right" || t.s == "middle") })
		parts = append(parts, p.hlist(as, st))
		t := p.peek()
		if t.kind != 'c' || t.s != "right" && t.s != "middle" {
			break // no \right: none
		}
		p.i++
		if t.s == "right" {
			r = p.delim()
			break
		}
		mids = append(mids, p.delim())
	}
	h, d := 0.0, 0.0
	for _, b := range parts {
		h, d = max(h, b.h), max(d, b.d)
	}
	size := p.delimSize(h, d, st)
	bs := []*box{p.makeDelim(l, size, st)}
	mu := p.size(st) / 18
	for i, b := range parts {
		if i > 0 {
			bs = append(bs, space(4*mu), p.makeDelim(mids[i-1], size, st), space(4*mu))
		}
		bs = append(bs, b)
	}
	bs = append(bs, p.makeDelim(r, size, st))
	return &atom{class: inner, b: hbox(bs...)}
}

// delimSize is how tall delimiters around what's h over the baseline
// and d under it are: TeX's, 90% of its height about the axis, or
// that less 5pt.
func (p *parser) delimSize(h, d float64, st int) float64 {
	axis := p.axis(st)
	delta := max(h-axis, d+axis)
	return max(2*delta*0.901, 2*delta-0.5*p.size(st))
}

// delimited is b between delimiters as tall.
func (p *parser) delimited(l rune, b *box, r rune, st int) *box {
	size := p.delimSize(b.h, b.d, st)
	return hbox(p.makeDelim(l, size, st), b, p.makeDelim(r, size, st))
}

// vectorDelims are the delimiters drawn to any height.
var vectorDelims = "()[]{}|‖⟨⟩⌊⌋⌈⌉/\\"

// makeDelim is the delimiter r, size tall about the axis: the font's
// sign if that's tall enough, else drawn.
func (p *parser) makeDelim(r rune, size float64, st int) *box {
	em := p.size(st)
	if r == '.' || r == 0 {
		return space(0.12 * em)
	}
	g := glyph(r, em)
	if size <= 1.15*em || size <= (g.h+g.d)*1.05 {
		return p.center(g, st)
	}
	if !strings.ContainsRune(vectorDelims, r) { // an arrow, say: the sign, bigger
		return p.center(glyph(r, em*size/max(g.h+g.d, em/2)), st)
	}
	return p.vdelim(r, size, st)
}

// vdelim draws the delimiter r, size tall, centered on the axis.
func (p *parser) vdelim(r rune, size float64, st int) *box {
	em, axis := p.size(st), p.axis(st)
	tn := max(1, 0.05*em)
	tk := min(0.16*em, 0.085*em+0.01*size)
	var wd float64
	switch r {
	case '(', ')':
		wd = min(0.6*em, 0.24*em+0.04*size)
	case '[', ']', '⌊', '⌋', '⌈', '⌉':
		wd = min(0.45*em, 0.24*em+0.02*size)
	case '{', '}':
		wd = min(0.6*em, 0.3*em+0.03*size)
	case '⟨', '⟩':
		wd = min(0.6*em, 0.25*em+0.05*size)
	case '|':
		wd = 1.4 * tn
	case '‖':
		wd = 4 * tn
	case '/', '\\':
		wd = min(size*0.4, 0.3*em+0.1*size)
	}
	sb := 0.08 * em
	b := &box{w: wd + 2*sb, h: size/2 + axis, d: size/2 - axis}
	b.paint = func(c *canvas, x, y float64) {
		top := y - axis - size/2
		bot, mid := top+size, y-axis
		x0, x1 := x+sb, x+sb+wd
		flip := strings.ContainsRune(")]}⟩⌋⌉\\", r)
		m := func(ps []pt) []pt {
			if flip {
				for i := range ps {
					ps[i].x = x0 + x1 - ps[i].x
				}
			}
			return ps
		}
		switch r {
		case '(', ')':
			e := x1 - 0.3*tn
			out := quad(pt{x1, top}, pt{2*x0 - x1, mid}, pt{x1, bot}, 32)
			xi := x0 + tk
			in := quad(pt{e, bot - 0.6*tn}, pt{2*xi - e, mid}, pt{e, top + 0.6*tn}, 32)
			c.poly(m(append(out, in...)))
		case '[', ']':
			c.poly(m([]pt{{x0, top}, {x1, top}, {x1, top + tn}, {x0 + tk, top + tn}, {x0 + tk, bot - tn}, {x1, bot - tn}, {x1, bot}, {x0, bot}}))
		case '⌊', '⌋':
			c.poly(m([]pt{{x0, top}, {x0 + tk, top}, {x0 + tk, bot - tn}, {x1, bot - tn}, {x1, bot}, {x0, bot}}))
		case '⌈', '⌉':
			c.poly(m([]pt{{x0, top}, {x1, top}, {x1, top + tn}, {x0 + tk, top + tn}, {x0 + tk, bot}, {x0, bot}}))
		case '{', '}':
			xm := (x0 + x1) / 2
			k := min(0.35*em, size*0.12, wd)
			ps := quad(pt{x1, top + tn}, pt{xm, top + tn}, pt{xm, top + k}, 10)
			ps = append(ps, quad(pt{xm, mid - k}, pt{xm, mid}, pt{x0, mid}, 10)...)
			ps = append(ps, quad(pt{x0, mid}, pt{xm, mid}, pt{xm, mid + k}, 10)...)
			ps = append(ps, quad(pt{xm, bot - k}, pt{xm, bot - tn}, pt{x1, bot - tn}, 10)...)
			c.line(0.8*tk, m(ps)...)
		case '⟨', '⟩':
			c.line(1.3*tn, m([]pt{{x1, top}, {x0, mid}, {x1, bot}})...)
		case '|':
			c.rect(x0, top, x1, bot)
		case '‖':
			c.rect(x0, top, x0+1.4*tn, bot)
			c.rect(x1-1.4*tn, top, x1, bot)
		case '/', '\\':
			c.line(1.3*tn, m([]pt{{x1, top}, {x0, bot}})...)
		}
	}
	return b
}

// accent is an accent's sign over its argument: where the font puts
// it for a letter as high as an x, raised by how much higher it is.
func (p *parser) accent(r rune, name string, st int) *atom {
	em := p.size(st)
	as := p.argAtoms(st)
	body := p.hlist(as, st)
	single := len(as) == 1 && as[0].letter
	switch {
	case name == "bar" && body.w > 0.8*em:
		return &atom{class: ord, b: p.rule2(body, true, st)}
	case (name == "hat" || name == "tilde") && body.w > 1.1*em:
		return &atom{class: ord, b: p.wide(body, "wide"+name, false, st)}
	}
	g := glyph(r, em)
	skew := 0.0
	if single {
		skew = body.ital*0.5 + 0.03*em
	}
	raise := max(0, body.h-0.47*em)
	b := &box{}
	b.put(body, 0, 0)
	b.put(g, (body.w-g.w)/2+skew, -raise)
	b.w, b.ital = body.w, body.ital
	return &atom{class: ord, b: b, letter: single}
}

// rule2 is body overlined, or underlined.
func (p *parser) rule2(body *box, over bool, st int) *box {
	t := p.rule(st)
	b := &box{}
	b.put(body, 0, 0)
	b.w = body.w
	if over {
		b.h = body.h + 4*t
		b.paint = func(c *canvas, x, y float64) { c.rect(x, y-body.h-4*t, x+body.w, y-body.h-3*t) }
	} else {
		b.d = body.d + 4*t
		b.paint = func(c *canvas, x, y float64) { c.rect(x, y+body.d+2*t, x+body.w, y+body.d+3*t) }
	}
	return b
}

// wide is a hat, a tilde or an arrow drawn over (or under) all of
// body.
func (p *parser) wide(body *box, name string, letter bool, st int) *box {
	em, t := p.size(st), p.rule(st)
	tn := max(1, 0.85*t)
	under := strings.HasPrefix(name, "under")
	w, x0 := body.w, 0.0
	if name == "vec" && body.w < 0.9*em {
		w = max(0.45*em, body.w*0.9)
		x0 = (body.w - w) / 2
		if letter {
			x0 += body.ital*0.5 + 0.04*em
		}
	}
	hgt := 0.22 * em
	switch name {
	case "widehat", "widetilde":
		hgt = min(0.3*em, 0.1*em+0.04*w)
	}
	gap := 0.1 * em
	b := &box{}
	b.put(body, 0, 0)
	b.w = body.w
	if under {
		b.d = body.d + gap + hgt
	} else {
		b.h = max(body.h, 0.45*em) + gap + hgt
	}
	b.paint = func(c *canvas, x, y float64) {
		lo := y - max(body.h, 0.45*em) - gap // the drawing's bottom
		if under {
			lo = y + body.d + gap + hgt
		}
		hi, l, r := lo-hgt, x+x0, x+x0+w
		switch name {
		case "widehat":
			c.line(tn, pt{l, lo}, pt{(l + r) / 2, hi}, pt{r, lo})
		case "widetilde":
			var ps []pt
			for i := 0; i <= 24; i++ {
				f := float64(i) / 24
				ps = append(ps, pt{l + f*w, (lo+hi)/2 - math.Sin(f*2*math.Pi)*hgt*0.4})
			}
			c.line(tn, ps...)
		default: // arrows
			mid := (lo + hi) / 2
			c.line(tn, pt{l, mid}, pt{r, mid})
			hd := min(0.2*em, w/2)
			if strings.Contains(name, "right") || name == "vec" {
				c.line(tn, pt{r - hd, mid - hd*0.6}, pt{r, mid}, pt{r - hd, mid + hd*0.6})
			}
			if strings.Contains(name, "left") {
				c.line(tn, pt{l + hd, mid - hd*0.6}, pt{l, mid}, pt{l + hd, mid + hd*0.6})
			}
		}
	}
	return b
}

// brace is body with a brace over or under it.
func (p *parser) brace(body *box, over bool, st int) *box {
	em := p.size(st)
	tk := max(1, 0.07*em)
	hgt, gap := 0.3*em, 0.1*em
	b := &box{}
	b.put(body, 0, 0)
	b.w = body.w
	if over {
		b.h = body.h + gap + hgt
	} else {
		b.d = body.d + gap + hgt
	}
	b.paint = func(c *canvas, x, y float64) {
		e, tip := y-body.h-gap, y-body.h-gap-hgt // its ends, its tip
		if !over {
			e, tip = y+body.d+gap, y+body.d+gap+hgt
		}
		m := (e + tip) / 2
		l, r := x+tk/2, x+body.w-tk/2
		xm := (l + r) / 2
		k := min(hgt/2, (r-l)/4)
		ps := quad(pt{l, e}, pt{l, m}, pt{l + k, m}, 8)
		ps = append(ps, quad(pt{xm - k, m}, pt{xm, m}, pt{xm, tip}, 8)...)
		ps = append(ps, quad(pt{xm, tip}, pt{xm, m}, pt{xm + k, m}, 8)...)
		ps = append(ps, quad(pt{r - k, m}, pt{r, m}, pt{r, e}, 8)...)
		c.line(tk, ps...)
	}
	return b
}

// stack is over centered over base (or under it).
func (p *parser) stack(base, over *box, above bool, st int) *box {
	gap := 0.1 * p.size(st)
	w := max(base.w, over.w)
	b := &box{}
	b.put(base, (w-base.w)/2, 0)
	if above {
		b.put(over, (w-over.w)/2, -(base.h + gap + over.d))
	} else {
		b.put(over, (w-over.w)/2, base.d+gap+over.h)
	}
	b.w = w
	return b
}

// xarrow is an arrow as long as what's written over it (and under).
func (p *parser) xarrow(name string, above, below *box, st int) *box {
	em, axis := p.size(st), p.axis(st)
	tn := max(1, 0.85*p.rule(st))
	w := max(1.6*em, above.w+0.8*em)
	if below != nil {
		w = max(w, below.w+0.8*em)
	}
	gap := 0.15 * em
	b := &box{h: axis + 0.2*em, d: max(0, 0.2*em-axis)}
	b.put(above, (w-above.w)/2, -(axis + gap + above.d))
	if below != nil {
		b.put(below, (w-below.w)/2, -axis+gap+below.h)
	}
	b.w = w
	double := name == "xRightarrow" || name == "xLeftarrow" || name == "xlongequal"
	b.paint = func(c *canvas, x, y float64) {
		mid, l, r := y-axis, x+0.1*em, x+w-0.1*em
		hd := 0.22 * em
		if double {
			c.line(tn, pt{l, mid - 0.1*em}, pt{r - 0.08*em, mid - 0.1*em})
			c.line(tn, pt{l, mid + 0.1*em}, pt{r - 0.08*em, mid + 0.1*em})
		} else {
			c.line(tn, pt{l, mid}, pt{r, mid})
		}
		if name == "xlongequal" {
			return
		}
		if strings.Contains(name, "ight") || name == "xmapsto" {
			c.line(tn, pt{r - hd, mid - hd*0.7}, pt{r, mid}, pt{r - hd, mid + hd*0.7})
		}
		if strings.Contains(name, "eft") {
			c.line(tn, pt{l + hd, mid - hd*0.7}, pt{l, mid}, pt{l + hd, mid + hd*0.7})
		}
		if name == "xmapsto" {
			c.line(tn, pt{l, mid - hd*0.6}, pt{l, mid + hd*0.6})
		}
	}
	return b
}

// framed is body in a frame.
func (p *parser) framed(body *box, st int) *box {
	pad, t := 0.25*p.size(st), p.rule(st)
	b := &box{}
	b.put(body, pad+t, 0)
	b.w, b.h, b.d = body.w+2*(pad+t), body.h+pad+t, body.d+pad+t
	b.paint = func(c *canvas, x, y float64) {
		l, r, top, bot := x, x+b.w, y-b.h, y+b.d
		c.rect(l, top, r, top+t)
		c.rect(l, bot-t, r, bot)
		c.rect(l, top, l+t, bot)
		c.rect(r-t, top, r, bot)
	}
	return b
}

// cancel is body struck through.
func (p *parser) cancel(body *box, name string, st int) *box {
	tn := max(1, 0.85*p.rule(st))
	b := &box{}
	b.put(body, 0, 0)
	b.w = body.w
	b.paint = func(c *canvas, x, y float64) {
		l, r, top, bot := x, x+body.w, y-body.h, y+body.d
		switch name {
		case "sout":
			mid := y - p.axis(st)
			c.line(tn, pt{l, mid}, pt{r, mid})
			return
		case "cancel", "xcancel":
			c.line(tn, pt{l, bot}, pt{r, top})
		}
		if name != "cancel" {
			c.line(tn, pt{l, top}, pt{r, bot})
		}
	}
	return b
}

// env is an environment: \begin{name}, to \end.
func (p *parser) env(name string, st int) *atom {
	em := p.size(st)
	end := func(t tok) bool { return t.kind == 'c' && t.s == "end" }
	finish := func() {
		if t := p.peek(); t.kind == 'c' && t.s == "end" {
			p.i++
			p.rawGroup()
		}
	}
	gap := func(g float64) func(int) float64 { return func(int) float64 { return g } }
	cellSt := max(st, styleT)
	switch name {
	case "matrix", "pmatrix", "bmatrix", "Bmatrix", "vmatrix", "Vmatrix", "smallmatrix", "array", "subarray",
		"matrix*", "pmatrix*", "bmatrix*":
		aligns := "c"
		if name == "array" || name == "subarray" {
			aligns = strings.Map(func(r rune) rune {
				if r == 'l' || r == 'c' || r == 'r' {
					return r
				}
				return -1
			}, p.rawGroup())
		} else if strings.HasSuffix(name, "*") {
			if b, ok := p.optArg(st); ok && b != nil {
				aligns = "c" // its l or r isn't read: centered
			}
		}
		g := 1.0
		if name == "smallmatrix" || name == "subarray" {
			cellSt, g = styleS, 0.5
		}
		rows, hl := p.cells(cellSt, end, false)
		finish()
		b := p.grid(rows, aligns, gap(g*em), 0.25, st, hl)
		lr := map[string][2]rune{"pmatrix": {'(', ')'}, "bmatrix": {'[', ']'}, "Bmatrix": {'{', '}'},
			"vmatrix": {'|', '|'}, "Vmatrix": {'‖', '‖'}, "pmatrix*": {'(', ')'}, "bmatrix*": {'[', ']'}}[name]
		l, r := lr[0], lr[1]
		if l != 0 {
			b = p.delimited(l, b, r, st)
		}
		return &atom{class: inner, b: b}
	case "cases", "dcases", "rcases", "cases*":
		if name == "dcases" {
			cellSt = styleD
		}
		rows, hl := p.cells(cellSt, end, false)
		finish()
		b := p.grid(rows, "ll", gap(em), 0.2, st, hl)
		if name == "rcases" {
			return &atom{class: inner, b: p.delimited('.', b, '}', st)}
		}
		return &atom{class: inner, b: p.delimited('{', b, '.', st)}
	case "aligned", "align", "align*", "alignat", "alignat*", "alignedat", "split", "flalign", "flalign*", "eqnarray", "eqnarray*":
		if strings.HasPrefix(name, "alignat") || name == "alignedat" {
			p.rawGroup()
		}
		rows, hl := p.cells(styleD, end, !strings.HasPrefix(name, "eqnarray"))
		finish()
		if strings.HasPrefix(name, "eqnarray") {
			return &atom{class: ord, b: p.grid(rows, "rcl", gap(0.3*em), 0.3, st, hl)}
		}
		return &atom{class: ord, b: p.grid(rows, "rl", alignedGap(em), 0.3, st, hl)}
	case "gathered", "gather", "gather*", "multline", "multline*", "equation", "equation*", "displaymath", "math":
		rows, hl := p.cells(styleD, end, false)
		finish()
		if len(rows) == 1 && len(rows[0]) == 1 {
			return &atom{class: ord, b: rows[0][0]}
		}
		return &atom{class: ord, b: p.grid(rows, "c", gap(em), 0.3, st, hl)}
	}
	p.unknownCmd("begin{" + name + "}")
	rows, hl := p.cells(cellSt, end, false)
	finish()
	return &atom{class: ord, b: p.grid(rows, "c", gap(em), 0.3, st, hl)}
}

// grid lays rows of cells out: each column as wide as its widest
// (aligns says how each is aligned in it, repeated: l, c, r), gap(j)
// before column j, rowGap ems between rows; centered on the axis. A
// rule over each row hlines says.
func (p *parser) grid(rows [][]*box, aligns string, gap func(int) float64, rowGap float64, st int, hlines []int) *box {
	em, t := p.size(st), p.rule(st)
	if aligns == "" {
		aligns = "c"
	}
	ncol := 0
	for _, r := range rows {
		ncol = max(ncol, len(r))
	}
	colW := make([]float64, ncol)
	rowH, rowD := make([]float64, len(rows)), make([]float64, len(rows))
	total := 0.0
	for i, r := range rows {
		rowH[i], rowD[i] = 0.7*em, 0.3*em
		for j, c := range r {
			colW[j] = max(colW[j], c.w)
			rowH[i], rowD[i] = max(rowH[i], c.h), max(rowD[i], c.d)
		}
		total += rowH[i] + rowD[i]
	}
	total += rowGap * em * float64(max(0, len(rows)-1))
	axis := p.axis(st)
	b := &box{}
	y := -axis - total/2
	var lines []float64 // where the rules are, down from the baseline
	width := 0.0
	for j := range colW {
		if j > 0 {
			width += gap(j)
		}
		width += colW[j]
	}
	for i, r := range rows {
		if hasInt(hlines, i) {
			lines = append(lines, y-rowGap*em/2)
		}
		y += rowH[i]
		x := 0.0
		for j := range ncol {
			if j > 0 {
				x += gap(j)
			}
			if j < len(r) {
				c := r[j]
				dx := 0.0
				switch aligns[min(j, len(aligns)-1)%len(aligns)] {
				case 'r':
					dx = colW[j] - c.w
				case 'c':
					dx = (colW[j] - c.w) / 2
				}
				if len(aligns) == 2 { // repeated in pairs: rl, ll
					switch aligns[j%2] {
					case 'r':
						dx = colW[j] - c.w
					case 'l':
						dx = 0
					}
				}
				b.put(c, x+dx, y)
			}
			x += colW[j]
		}
		y += rowD[i] + rowGap*em
	}
	if hasInt(hlines, len(rows)) {
		lines = append(lines, y-rowGap*em/2)
	}
	b.w, b.h, b.d = width, max(b.h, axis+total/2), max(b.d, total/2-axis)
	if len(lines) > 0 {
		b.paint = func(c *canvas, x, by float64) {
			for _, ly := range lines {
				c.rect(x, by+ly-t/2, x+width, by+ly+t/2)
			}
		}
	}
	return b
}

func hasInt(xs []int, x int) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
