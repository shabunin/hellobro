package main

// An answer in Markdown, laid out on a notebook's page ("AI answers":
// Markdown): paragraphs and headings in the notebook's font, bold and
// italic as it says; list items under their markers; code in Go Mono,
// framed; formulas drawn as TeX would (package tex), each a picture
// that keeps its source; tables in columns; quotes with a bar. Each is
// written as the hand is, a word (or a formula) at a time, and goes on
// on a new page where the page is full.

import (
	"bytes"
	"encoding/xml"
	"image"
	"image/png"
	"log"
	"strings"
	"unicode/utf8"

	"golang.org/x/image/font"

	"hellobro/md"
	"hellobro/mermaid"
	"hellobro/tex"
)

const (
	texAttr     = "hellobro-tex"     // a formula's source, on its picture
	mermaidAttr = "hellobro-mermaid" // a diagram's
)

// mdAtom is a piece of a line: a word in a font, or a formula drawn.
type mdAtom struct {
	text      string
	font      string
	pt        float64 // the word's size, points
	pic       *notePic
	src       string // the formula's TeX
	w         int    // page units
	asc, desc int
	sw        int  // a space's width, in its font
	space     bool // a space before it
	brk       bool // a new line here
	disp      bool // a formula on a line of its own
}

// mdPlaced is an atom placed in a line: x from the line's start. Words
// of a group in a font, one after another, are one text.
type mdPlaced struct {
	a   mdAtom
	x   int
	grp int
}

type mdLay struct {
	w           *noteWin
	page, y     int
	left, width int
	fam         string
	pt          float64
	talk        []xml.Attr // the answer's talk, for its first stroke
	segs        []*mdSeg   // frames and bars still being drawn down the page
}

// mdSeg is a frame round code, or a quote's bar, being drawn down the
// page from top, x0 to x1; pad: room in it over and under what's in it.
type mdSeg struct {
	frame       bool
	x0, x1, top int
	pad         int
}

// diaryLayMD lays r's answer out, Markdown, under below on page, in
// fam's font, its letters pt points.
func (w *noteWin) diaryLayMD(page int, below image.Rectangle, r diaryReply, fam string, pt float64) {
	if pt == 0 {
		pt = diaryPt
	}
	size := w.pageSize(page)
	l := &mdLay{w: w, page: page, fam: fam, pt: pt, talk: talkAttrs(r)}
	l.left = diaryLeft(below, size)
	l.width = size.X - diaryMargin - l.left
	em := l.em()
	l.y = below.Max.Y + em/2
	if below.Empty() {
		l.y = diaryMargin
	}
	indent := em * 3 / 2
	prev := md.Kind(-1)
	for i, b := range md.Parse(r.Answer) {
		if i > 0 {
			switch {
			case b.Kind == md.Item && prev == md.Item:
				l.y += em / 6
			case b.Kind == md.Heading:
				l.y += em * 3 / 4
			default:
				l.y += em / 2
			}
		}
		prev = b.Kind
		x0 := l.left + (b.Quote+b.Indent)*indent
		width := l.left + l.width - x0
		var bar *mdSeg
		if b.Quote > 0 {
			x := l.left + (b.Quote-1)*indent + em/4
			bar = &mdSeg{x0: x, x1: x, top: l.y}
			l.segs = append(l.segs, bar)
		}
		switch b.Kind {
		case md.Para:
			l.lines(l.wrap(l.atoms(md.Inline(b.Text), 0, pt), width), x0, width)
		case md.Heading:
			k := map[int]float64{1: 1.5, 2: 1.3, 3: 1.15}[b.Level]
			if k == 0 {
				k = 1.05
			}
			l.lines(l.wrap(l.atoms(md.Inline(b.Text), styleBold, pt*k), width), x0, width)
		case md.Item:
			ix := x0 + b.Level*indent
			mk := l.word(b.Marker, fontName(fam, 0), pt)
			mw := max(mk.w+mk.sw, em*6/5)
			lines := l.wrap(l.atoms(md.Inline(b.Text), 0, pt), x0+width-ix-mw)
			if len(lines) == 0 {
				lines = [][]mdPlaced{nil}
			}
			lines[0] = append([]mdPlaced{{a: mk, x: -mw, grp: -1}}, lines[0]...)
			l.lines(lines, ix+mw, x0+width-ix-mw)
		case md.Code:
			if src, ok := diagramSrc(b); !ok || !l.diagram(src, x0, width) {
				l.code(b.Text, x0, width)
			}
		case md.Math:
			l.lines([][]mdPlaced{{{a: l.formula(b.Text, true, pt)}}}, x0, width)
		case md.Table:
			l.table(b, x0, width)
		case md.Rule:
			l.need(em)
			y := l.y + em/2
			l.stroke([]image.Point{{x0, y}, {x0 + width, y}}, 10, "#808080ff")
			l.y += em
		}
		if bar != nil {
			l.close(bar)
		}
	}
}

// em is the letters' size, page units.
func (l *mdLay) em() int { return int(l.pt/notePt72 + 0.5) }

func (l *mdLay) face(name string, pt float64) font.Face {
	return noteFontFace(name, int(pt/notePt72+0.5))
}

func (l *mdLay) bottom() int { return l.w.pageSize(l.page).Y - diaryMargin/2 }

// need makes room for h more: a new page if there's none on this one.
func (l *mdLay) need(h int) {
	if l.y+h <= l.bottom() || l.y <= diaryMargin || l.w.rollRoom(l.page, l.y+h) {
		return
	}
	for _, s := range l.segs {
		l.drawSeg(s, l.y+s.pad)
	}
	l.page = l.w.diaryPage(l.page)
	l.y = diaryMargin
	for _, s := range l.segs {
		s.top = l.y
		l.y += s.pad
	}
}

// close ends a frame or a bar where the writing's got to.
func (l *mdLay) close(s *mdSeg) {
	l.drawSeg(s, l.y)
	for i, o := range l.segs {
		if o == s {
			l.segs = append(l.segs[:i], l.segs[i+1:]...)
			break
		}
	}
}

func (l *mdLay) drawSeg(s *mdSeg, bot int) {
	if s.frame {
		l.stroke([]image.Point{{s.x0, s.top}, {s.x1, s.top}, {s.x1, bot}, {s.x0, bot}, {s.x0, s.top}}, 10, "#808080ff")
	} else if bot > s.top {
		l.stroke([]image.Point{{s.x0, s.top}, {s.x0, bot}}, 25, "#808080ff")
	}
}

// first is as, and the talk if it's the answer's first stroke.
func (l *mdLay) first(as []xml.Attr) []xml.Attr {
	if l.talk != nil {
		as, l.talk = append(as, l.talk...), nil
	}
	return as
}

// stroke is a pen's line through ps (radius r tenths of a unit), in
// color: a frame, a rule.
func (l *mdLay) stroke(ps []image.Point, r int16, color string) {
	s := &noteStroke{gray: colorGray(color, false), attrs: l.first([]xml.Attr{
		{Name: xml.Name{Local: "tool"}, Value: "pen"}, {Name: xml.Name{Local: "color"}, Value: color},
		{Name: xml.Name{Local: "capStyle"}, Value: "round"}, {Name: xml.Name{Local: diaryAttr}, Value: "diary"}})}
	for _, p := range ps {
		s.pts = append(s.pts, notePt{int16(p.X), int16(p.Y), r})
	}
	s.bound()
	l.w.diary.writing = append(l.w.diary.writing, diaryWord{page: l.page, strokes: []*noteStroke{s}})
}

// text is a text in a font from x, top: its words written one after
// another, its text each of texts in turn.
func (l *mdLay) text(name string, pt float64, x, top int, texts []string) {
	as := l.first([]xml.Attr{{Name: xml.Name{Local: "font"}, Value: name}, {Name: xml.Name{Local: "color"}, Value: "#000000ff"},
		{Name: xml.Name{Local: diaryAttr}, Value: "diary"}})
	s := textStroke(&noteText{size: pt, attrs: as}, image.Pt(x, top))
	for i, t := range texts {
		l.w.diary.writing = append(l.w.diary.writing, diaryWord{page: l.page, line: s, text: t, first: i == 0})
	}
}

// word is a word's atom.
func (l *mdLay) word(s, name string, pt float64) mdAtom {
	f := l.face(name, pt)
	m := f.Metrics()
	return mdAtom{text: s, font: name, pt: pt, w: font.MeasureString(f, s).Ceil(), asc: m.Ascent.Ceil(), desc: m.Descent.Ceil(),
		sw: font.MeasureString(f, " ").Ceil()}
}

// atoms are spans' words and formulas, in the notebook's font in style
// (and the spans' own), pt points.
func (l *mdLay) atoms(spans []md.Span, style int, pt float64) []mdAtom {
	var as []mdAtom
	space := false
	for _, sp := range spans {
		switch {
		case sp.Break:
			as, space = append(as, mdAtom{brk: true}), false
		case sp.Math:
			a := l.formula(sp.Text, sp.Display, pt)
			a.space = space
			as, space = append(as, a), false
		default:
			fam, st, apt := l.fam, style, pt
			if sp.Style&md.Bold != 0 {
				st |= styleBold
			}
			if sp.Style&md.Italic != 0 {
				st |= styleItalic
			}
			txt := sp.Text
			if sp.Style&md.Mono != 0 {
				fam, apt = famGoMono, pt*0.9
			} else {
				txt = superscripts(txt, l.face(fontName(fam, st), apt))
			}
			name := fontName(fam, st)
			for i, wd := range strings.Split(txt, " ") {
				if i > 0 {
					space = true
				}
				if wd == "" {
					continue
				}
				a := l.word(wd, name, apt)
				a.space = space
				as, space = append(as, a), false
			}
		}
	}
	return as
}

// formula is src drawn, in display style or in a line's.
func (l *mdLay) formula(src string, display bool, pt float64) mdAtom {
	r := tex.Render(src, display, pt/notePt72)
	if len(r.Unknown) > 0 {
		log.Printf("diary: TeX it doesn't know: %v in %q", r.Unknown, src)
	}
	ink := image.NewNRGBA(r.Img.Bounds()) // black, as transparent as the drawing's white: on any paper
	for i, g := range r.Img.Pix {
		ink.Pix[4*i+3] = 255 - g
	}
	var buf bytes.Buffer
	png.Encode(&buf, ink)
	b := r.Img.Bounds()
	sw := font.MeasureString(l.face(fontName(l.fam, 0), pt), " ").Ceil() // a space before it is the text's
	return mdAtom{pic: &notePic{png: buf.Bytes(), gray: r.Img, ink: true}, src: src, w: b.Dx(), asc: r.Baseline,
		desc: b.Dy() - r.Baseline, disp: display, sw: sw}
}

// wrap breaks atoms into lines width wide: between words, a word too
// long cut, a formula too wide made smaller.
func (l *mdLay) wrap(as []mdAtom, width int) [][]mdPlaced {
	width = max(width, l.em()*4)
	var lines [][]mdPlaced
	var cur []mdPlaced
	x := 0
	next := func() { lines, cur, x = append(lines, cur), nil, 0 }
	for i := 0; i < len(as); {
		a := as[i]
		switch {
		case a.brk:
			next()
			i++
			continue
		case a.disp:
			if len(cur) > 0 {
				next()
			}
			cur = []mdPlaced{{a: l.fit(a, width)}}
			next()
			i++
			continue
		}
		j, cw := i+1, a.w // a word and what's on it: the formula, the comma after
		for j < len(as) && !as[j].space && !as[j].brk && !as[j].disp {
			cw += as[j].w
			j++
		}
		gap := 0
		if len(cur) > 0 && a.space {
			gap = a.sw
		}
		if len(cur) > 0 && x+gap+cw > width {
			next()
			gap = 0
		}
		for _, b := range as[i:j] {
			for b.text != "" && x+gap+b.w > width { // too long for a line: what fits on it, and on
				n := l.fits(b, width-x-gap)
				if n == 0 && len(cur) == 0 {
					n = len(string([]rune(b.text)[:1]))
				}
				if n > 0 {
					c := l.word(b.text[:n], b.font, b.pt)
					cur = append(cur, mdPlaced{a: c, x: x + gap})
				}
				next()
				gap = 0
				b = l.word(b.text[n:], b.font, b.pt)
			}
			if b.pic != nil && b.w > width-x-gap {
				if len(cur) > 0 {
					next()
					gap = 0
				}
				b = l.fit(b, width)
			}
			cur = append(cur, mdPlaced{a: b, x: x + gap})
			x += gap + b.w
			gap = 0
		}
		i = j
	}
	if len(cur) > 0 {
		next()
	}
	return lines
}

// fits is how many bytes of a's text fit in w.
func (l *mdLay) fits(a mdAtom, w int) int {
	f := l.face(a.font, a.pt)
	n := 0
	for i, r := range a.text {
		if font.MeasureString(f, a.text[:i+utf8.RuneLen(r)]).Ceil() > w {
			break
		}
		n = i + utf8.RuneLen(r)
	}
	return n
}

// fit is a formula made smaller to be no wider than width.
func (l *mdLay) fit(a mdAtom, width int) mdAtom {
	if a.w > width && a.w > 0 {
		k := float64(width) / float64(a.w)
		a.w, a.asc, a.desc = width, int(float64(a.asc)*k+0.5), int(float64(a.desc)*k+0.5)
	}
	return a
}

// lines writes lines, from x0, width wide: a formula on its own line
// centered in it.
func (l *mdLay) lines(lines [][]mdPlaced, x0, width int) {
	for _, ln := range lines {
		l.line(ln, x0, width)
	}
}

func (l *mdLay) line(ln []mdPlaced, x0, width int) {
	m := l.face(fontName(l.fam, 0), l.pt).Metrics()
	asc, desc := m.Ascent.Ceil(), m.Descent.Ceil()
	lead := max(0, m.Height.Ceil()-asc-desc)
	for _, p := range ln {
		asc, desc = max(asc, p.a.asc), max(desc, p.a.desc)
	}
	if len(ln) == 1 && ln[0].a.disp {
		lead = l.em() / 2
		x0 += (width - ln[0].a.w) / 2
	}
	l.need(asc + desc + lead)
	base := l.y + lead/2 + asc
	for i := 0; i < len(ln); {
		p := ln[i]
		if p.a.pic != nil {
			s := picStroke(p.a.pic, image.Rect(x0+p.x, base-p.a.asc, x0+p.x+p.a.w, base+p.a.desc))
			s.attrs = l.first([]xml.Attr{{Name: xml.Name{Local: diaryAttr}, Value: "diary"}, {Name: xml.Name{Local: texAttr}, Value: p.a.src}})
			l.w.diary.writing = append(l.w.diary.writing, diaryWord{page: l.page, strokes: []*noteStroke{s}})
			i++
			continue
		}
		texts := []string{p.a.text}
		j := i + 1
		for ; j < len(ln) && ln[j].a.pic == nil && ln[j].a.font == p.a.font && ln[j].a.pt == p.a.pt && ln[j].grp == p.grp && p.grp >= 0; j++ {
			t := texts[len(texts)-1]
			if ln[j].a.space || ln[j].x > ln[j-1].x+ln[j-1].a.w {
				t += " "
			}
			texts = append(texts, t+ln[j].a.text)
		}
		top := base - l.face(p.a.font, p.a.pt).Metrics().Ascent.Ceil()
		l.text(p.a.font, p.a.pt, x0+p.x, top, texts)
		i = j
	}
	l.y = base + desc + lead - lead/2
}

// diagramSrc is a block of code's diagram, if it is one: in Mermaid's
// language, marked so or unmarked and starting as one does; or a plot
// (```plot), which Render knows by its first word.
func diagramSrc(b md.Block) (string, bool) {
	switch {
	case strings.EqualFold(b.Lang, "mermaid"):
		return b.Text, true
	case strings.EqualFold(b.Lang, "plot"):
		return "plot\n" + b.Text, true
	case b.Lang != "":
		return "", false
	}
	f := strings.Fields(b.Text)
	if len(f) == 0 {
		return "", false
	}
	switch f[0] {
	case "graph", "flowchart", "sequenceDiagram", "stateDiagram", "stateDiagram-v2", "pie", "classDiagram", "erDiagram",
		"gantt", "timeline", "mindmap", "xychart-beta":
		return b.Text, true
	}
	return "", false
}

// diagram draws a diagram (package mermaid), centered, a picture that
// keeps its source: its letters a little smaller than the text's, and
// smaller again where it'd be wider than the text or higher than a page.
// False if it's not one it draws: it's code then.
func (l *mdLay) diagram(src string, x0, width int) bool {
	em := float64(l.em()) * 0.85
	maxH := l.bottom() - diaryMargin
	if l.w.nb.rollOn() { // a screen's, not the roll's
		maxH = min(notePageH, l.w.pageSize(l.page).X*4/3) - 2*diaryMargin
	}
	var r mermaid.Result
	for try := 0; try < 4; try++ {
		var err error
		if r, err = mermaid.Render(src, mermaid.Options{Em: em, Face: l.diagramFace}); err != nil {
			log.Printf("diary: a diagram it doesn't draw: %v", err)
			return false
		}
		b := r.Img.Bounds()
		k := min(float64(width)/float64(b.Dx()), float64(maxH)/float64(b.Dy()))
		if k >= 1 || em <= 8 {
			break
		}
		em = max(8, em*k*0.97)
	}
	b := r.Img.Bounds()
	k := min(1, float64(width)/float64(b.Dx()), float64(maxH)/float64(b.Dy())) // at the smallest, the picture made smaller
	w, h := int(float64(b.Dx())*k), int(float64(b.Dy())*k)
	ink := image.NewNRGBA(b) // black, as transparent as the drawing's white: on any paper
	for i, g := range r.Img.Pix {
		ink.Pix[4*i+3] = 255 - g
	}
	var buf bytes.Buffer
	png.Encode(&buf, ink)
	l.need(h)
	x := x0 + (width-w)/2
	s := picStroke(&notePic{png: buf.Bytes(), gray: r.Img, ink: true}, image.Rect(x, l.y, x+w, l.y+h))
	s.attrs = l.first([]xml.Attr{{Name: xml.Name{Local: diaryAttr}, Value: "diary"}, {Name: xml.Name{Local: mermaidAttr}, Value: src}})
	l.w.diary.writing = append(l.w.diary.writing, diaryWord{page: l.page, strokes: []*noteStroke{s}})
	l.y += h
	return true
}

// diagramFace is the notebook's font for a diagram's labels, in a style;
// code in Go Mono.
func (l *mdLay) diagramFace(st md.Style, px float64) font.Face {
	if st&md.Mono != 0 {
		return noteFontFace(famGoMono, int(px+0.5))
	}
	style := 0
	if st&md.Bold != 0 {
		style |= styleBold
	}
	if st&md.Italic != 0 {
		style |= styleItalic
	}
	return noteFontFace(fontName(l.fam, style), int(px+0.5))
}

// code is a block of code, in Go Mono, framed: its lines cut where
// they're too long.
func (l *mdLay) code(src string, x0, width int) {
	pt := l.pt * 0.85
	f := l.face(famGoMono, pt)
	lh := f.Metrics().Height.Ceil()
	pad := lh / 3
	cw := max(1, font.MeasureString(f, "M").Ceil())
	cols := max(8, (width-2*pad)/cw)
	var lines []string
	for _, ln := range strings.Split(strings.TrimRight(src, "\n"), "\n") {
		rs := []rune(strings.ReplaceAll(ln, "\t", "    "))
		for len(rs) > cols {
			lines, rs = append(lines, string(rs[:cols])), rs[cols:]
		}
		lines = append(lines, string(rs))
	}
	l.need(lh + 2*pad)
	s := &mdSeg{frame: true, x0: x0, x1: x0 + width, top: l.y, pad: pad}
	l.segs = append(l.segs, s)
	l.y += pad
	for _, ln := range lines {
		l.need(lh + pad)
		if strings.TrimSpace(ln) != "" {
			var texts []string // a word on at a time, its spaces kept
			for i := 1; i < len(ln); i++ {
				if ln[i] == ' ' && ln[i-1] != ' ' {
					texts = append(texts, ln[:i])
				}
			}
			l.text(famGoMono, pt, x0+pad, l.y, append(texts, ln))
		}
		l.y += lh
	}
	l.y += pad
	l.close(s)
}

// table lays a table out in columns: as wide as they'd be, or shared
// out where that's too wide; its head bold, a rule under it.
func (l *mdLay) table(b md.Block, x0, width int) {
	em := l.em()
	gap := em
	ncol := 0
	for _, r := range b.Rows {
		ncol = max(ncol, len(r))
	}
	cells := make([][][]mdAtom, len(b.Rows))
	natural := make([]int, ncol)
	for i, r := range b.Rows {
		style := 0
		if i == 0 {
			style = styleBold
		}
		for j, c := range r {
			as := l.atoms(md.Inline(c), style, l.pt)
			cells[i] = append(cells[i], as)
			w := 0
			for k, a := range as {
				if k > 0 && a.space {
					w += a.sw
				}
				w += a.w
			}
			natural[j] = max(natural[j], w)
		}
	}
	colW := shareOut(natural, width-gap*(ncol-1), 2*em)
	colX := make([]int, ncol)
	for j := 1; j < ncol; j++ {
		colX[j] = colX[j-1] + colW[j-1] + gap
	}
	total := colX[ncol-1] + colW[ncol-1]
	for i, row := range cells {
		var cl [][][]mdPlaced
		n := 0
		for j, as := range row {
			lines := l.wrap(as, colW[j])
			cl = append(cl, lines)
			n = max(n, len(lines))
		}
		for k := range n {
			var ln []mdPlaced
			for j, lines := range cl {
				if k >= len(lines) || len(lines[k]) == 0 {
					continue
				}
				last := lines[k][len(lines[k])-1]
				dx := 0
				al := byte('l')
				if j < len(b.Align) {
					al = b.Align[j]
				}
				switch al {
				case 'r':
					dx = colW[j] - (last.x + last.a.w)
				case 'c':
					dx = (colW[j] - (last.x + last.a.w)) / 2
				}
				for _, p := range lines[k] {
					p.x += colX[j] + dx
					p.grp = j
					ln = append(ln, p)
				}
			}
			l.line(ln, x0, width)
		}
		if i == 0 || i == len(cells)-1 {
			l.y += em / 4
			l.stroke([]image.Point{{x0, l.y}, {x0 + total, l.y}}, 10, "#808080ff")
			l.y += em / 4
		}
	}
}

// shareOut is columns' widths in all: as wide as they'd be, if they
// fit; else the narrow ones so, and the rest share what's left (at
// least least).
func shareOut(natural []int, all, least int) []int {
	w := make([]int, len(natural))
	sum := 0
	for _, n := range natural {
		sum += n
	}
	if sum <= all {
		copy(w, natural)
		return w
	}
	left, rest := all, len(natural)
	done := make([]bool, len(natural))
	for changed := true; changed && rest > 0; {
		changed = false
		share := left / rest
		for j, n := range natural {
			if !done[j] && n <= share {
				w[j], done[j], left, rest, changed = n, true, left-n, rest-1, true
			}
		}
	}
	for j := range w {
		if !done[j] {
			w[j] = max(least, left/max(1, rest))
		}
	}
	return w
}
