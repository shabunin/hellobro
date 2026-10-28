package book

// A chapter laid out in pages: lines broken at spaces (and after
// dashes), justified where the book says, pictures scaled to the page,
// small floated ones with the text going by them. A page starts at a
// place in the chapter (Pos): kept, it finds the page again after the
// font or its size changed.

import (
	"image"
	"strings"
	"unicode/utf8"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// Faces gives the font for a style, size px.
type Faces interface {
	Face(s Style, px int) font.Face
}

// Settings are the page's: its text area, the body text's size.
type Settings struct {
	W, H    int
	Px      int     // the body text's size
	Leading float64 // a line's height, of the size
	Ragged  bool    // no justified text: the right edge as it comes
}

type ItemKind uint8

const (
	ItemText ItemKind = iota
	ItemPicture
	ItemRule
)

// Item is something on a page, in its text area's pixels.
type Item struct {
	Kind  ItemKind
	X, Y  int    // text: where its baseline starts; else the top left
	W, H  int    // a picture's, a rule's
	Text  string // text: what; a picture: its Src
	Style Style
	Px    int // text: its size
}

// Pos is a place in a chapter: a block, a byte in its text.
type Pos struct{ Block, Off int }

func (p Pos) before(q Pos) bool { return p.Block < q.Block || p.Block == q.Block && p.Off < q.Off }

// Page is what's on one page, and where it starts.
type Page struct {
	Items []Item
	Start Pos
}

// PageAt is the page that shows pos.
func PageAt(pages []Page, pos Pos) int {
	n := 0
	for i, p := range pages {
		if pos.before(p.Start) {
			break
		}
		n = i
	}
	return n
}

// frag is a piece of a line: a word or part of one in one style, or a
// space.
type frag struct {
	text  string
	style Style
	px    int
	off   int // in the block's text
	space bool
	br    bool // a line break
	w     fixed.Int26_6
}

type layout struct {
	s     Settings
	faces Faces
	size  func(src string) image.Point
	pages []Page
	page  *Page
	y     int // the next line's top
	fl    struct {
		bottom, w int
		side      Float
	}
	placed bool // the page has something on it
}

// Layout lays ch out in pages. size gives a picture's size in its own
// pixels (0×0 if it can't).
func Layout(ch *Chapter, s Settings, faces Faces, size func(src string) image.Point) []Page {
	if s.Leading == 0 {
		s.Leading = 1.35
	}
	l := &layout{s: s, faces: faces, size: size}
	l.newPage(Pos{})
	for i := range ch.Blocks {
		b := &ch.Blocks[i]
		gap := int(b.Above * float32(s.Px))
		if l.placed {
			l.y += gap
		}
		switch b.Kind {
		case Text:
			l.text(b, i)
		case Picture:
			l.picture(b, i)
		case Rule:
			l.rule(i)
		}
	}
	if len(l.pages) > 1 && len(l.page.Items) == 0 {
		l.pages = l.pages[:len(l.pages)-1]
	}
	return l.pages
}

func (l *layout) newPage(start Pos) {
	l.pages = append(l.pages, Page{Start: start})
	l.page = &l.pages[len(l.pages)-1]
	l.y, l.placed = 0, false
	l.fl.bottom = 0
}

// room makes room for h more pixels, on a new page if need be.
func (l *layout) room(h int, at Pos) {
	if l.placed && l.y+h > l.s.H {
		l.newPage(at)
	}
	if !l.placed {
		l.page.Start = at
	}
}

func (l *layout) rule(i int) {
	h := l.s.Px
	l.room(h, Pos{i, 0})
	w := l.s.W / 3
	l.page.Items = append(l.page.Items, Item{Kind: ItemRule, X: (l.s.W - w) / 2, Y: l.y + h/2, W: w, H: 2})
	l.y += h
	l.placed = true
}

func (l *layout) picture(b *Block, i int) {
	var sz image.Point
	if l.size != nil {
		sz = l.size(b.Src)
	}
	if sz.X <= 0 || sz.Y <= 0 {
		return
	}
	// a book's pixels are a browser's: about 16 to its text's em
	w, h := sz.X*l.s.Px/18, sz.Y*l.s.Px/18
	if w > l.s.W {
		w, h = l.s.W, h*l.s.W/w
	}
	if h > l.s.H {
		w, h = w*l.s.H/h, l.s.H
	}
	w, h = max(1, w), max(1, h)
	if b.Float != NoFloat && w <= l.s.W*45/100 && h <= l.s.H/2 {
		l.room(min(h, 3*l.lineH(l.s.Px)), Pos{i, 0})
		x := 0
		if b.Float == FloatRight {
			x = l.s.W - w
		}
		l.page.Items = append(l.page.Items, Item{Kind: ItemPicture, X: x, Y: l.y, W: w, H: h, Text: b.Src})
		l.fl.bottom, l.fl.w, l.fl.side = l.y+h, w+l.s.Px/2, b.Float
		l.placed = true
		return
	}
	l.clearFloat()
	if l.placed && l.y+h > l.s.H {
		// what's left of the page, if the picture's no more than a
		// third smaller in it; else the next page
		if left := l.s.H - l.y; left*3 >= h*2 {
			w, h = w*left/h, left
		} else {
			l.newPage(Pos{i, 0})
		}
	}
	if !l.placed {
		l.page.Start = Pos{i, 0}
	}
	l.page.Items = append(l.page.Items, Item{Kind: ItemPicture, X: (l.s.W - w) / 2, Y: l.y, W: w, H: h, Text: b.Src})
	l.y += h
	l.placed = true
}

// clearFloat goes below a floated picture.
func (l *layout) clearFloat() {
	if l.fl.bottom > l.y {
		l.y = l.fl.bottom
	}
	l.fl.bottom = 0
}

func (l *layout) lineH(px int) int { return int(float64(px)*l.s.Leading + 0.5) }

// sizeOf is the size a span's text is set in.
func (l *layout) sizeOf(sp *Span) int {
	px := float32(l.s.Px) * sp.Scale
	if sp.Style&(Sup|Sub) != 0 {
		px *= 0.7
	}
	return max(8, int(px+0.5))
}

// frags are b's words, spaces and breaks, measured.
func (l *layout) frags(b *Block) []frag {
	var fs []frag
	off := 0
	for si := range b.Spans {
		sp := &b.Spans[si]
		px := l.sizeOf(sp)
		face := l.faces.Face(sp.Style&^(Sup|Sub), px)
		add := func(text string, at int, space, br bool) {
			f := frag{text: text, style: sp.Style, px: px, off: at, space: space, br: br}
			if !br {
				f.w = font.MeasureString(face, text)
			}
			fs = append(fs, f)
		}
		t := sp.Text
		if t == "\n" {
			add("", off, false, true)
			off += len(t)
			continue
		}
		if b.Pre { // lines as they are; long ones broken anywhere
			for i, line := range strings.Split(t, "\n") {
				if i > 0 {
					add("", off, false, true)
					off++
				}
				if line != "" {
					add(line, off, false, false)
				}
				off += len(line)
			}
			continue
		}
		start := 0
		for i := 0; i < len(t); {
			r, n := utf8.DecodeRuneInString(t[i:])
			switch {
			case r == ' ':
				if i > start {
					add(t[start:i], off+start, false, false)
				}
				add(" ", off+i, true, false)
				start = i + n
			case r == '-' || r == '—' || r == '–' || r == '/':
				// a break may come after it
				if i+n < len(t) && t[i+n] != ' ' {
					add(t[start:i+n], off+start, false, false)
					fs[len(fs)-1].space = false
					fs = append(fs, frag{off: off + i + n, style: sp.Style, px: px}) // a break that takes no room
					fs[len(fs)-1].text = "​"
					start = i + n
				}
			}
			i += n
		}
		if start < len(t) {
			add(t[start:], off+start, false, false)
		}
		off += len(t)
	}
	return fs
}

// text lays out a paragraph.
func (l *layout) text(b *Block, bi int) {
	fs := l.frags(b)
	if len(fs) == 0 {
		return
	}
	px := l.s.Px
	indent := min(l.s.W/3, int(b.Indent*float32(px)))
	if b.Bullet != "" && b.Indent == 0 {
		indent = px * 3 / 2
	}
	first := min(l.s.W/4, int(b.First*float32(px)))
	align := b.Align
	if align == Justify && (l.s.Ragged || b.Pre) {
		align = Left
	}
	if b.Head > 0 { // a heading keeps with two lines after it
		maxPx := px
		for _, f := range fs {
			maxPx = max(maxPx, f.px)
		}
		l.room(l.lineH(maxPx)+2*l.lineH(px), Pos{bi, 0})
	}
	firstLine := true
	for i := 0; i < len(fs); {
		// the line's room, by a float if it's beside one
		left, width := indent, l.s.W-indent
		if l.fl.bottom > l.y {
			width -= l.fl.w
			if l.fl.side == FloatLeft {
				left = max(left, l.fl.w)
				width = l.s.W - left
			}
		} else {
			l.fl.bottom = 0
		}
		if firstLine {
			left += first
			width -= first
		}
		width = max(width, px*4)
		// skip the spaces it would start with
		for i < len(fs) && fs[i].space && !b.Pre {
			i++
		}
		if i == len(fs) {
			break
		}
		// as many as fit
		var w fixed.Int26_6
		j, lastBreak, lineEnd := i, -1, -1
		limit := fixed.I(width)
		for j < len(fs) {
			f := &fs[j]
			if f.br {
				lineEnd = j
				break
			}
			if f.space || f.text == "​" {
				lastBreak = j
			}
			if w+f.w > limit && !f.space && j > i {
				break
			}
			w += f.w
			j++
		}
		end, next, broke := j, j, false
		switch {
		case lineEnd == i: // an empty line
			next, broke = i+1, true
		case fs[i].w > limit: // a word wider than the line: as much of it as fits
			fs = splitFrag(fs, i, l.faces.Face(fs[i].style&^(Sup|Sub), fs[i].px), limit)
			end, next = i+1, i+1
		case lineEnd >= 0:
			end, next, broke = lineEnd, lineEnd+1, true
		case j < len(fs) && lastBreak > i:
			end, next = lastBreak, lastBreak+1
		}
		// trailing spaces don't count
		line := fs[i:end]
		for len(line) > 0 && line[len(line)-1].space && !b.Pre {
			line = line[:len(line)-1]
		}
		lastOfPara := next >= len(fs) || broke
		l.setLine(line, b, bi, left, width, align, lastOfPara, firstLine)
		firstLine = false
		i = next
	}
}

// setLine puts a line on the page, at y, as it's aligned.
func (l *layout) setLine(line []frag, b *Block, bi, left, width int, align Align, last, firstLine bool) {
	maxPx := l.s.Px
	if len(line) > 0 {
		maxPx = 0
	}
	var w fixed.Int26_6
	spaces := 0
	for _, f := range line {
		px := f.px
		if f.style&(Sup|Sub) != 0 {
			px = px * 10 / 7
		}
		maxPx = max(maxPx, px)
		w += f.w
		if f.space {
			spaces++
		}
	}
	h := l.lineH(maxPx)
	at := Pos{bi, 0}
	if len(line) > 0 {
		at.Off = line[0].off
	}
	l.room(h, at)
	if !l.placed {
		l.page.Start = at
	}
	face := l.faces.Face(0, maxPx)
	m := face.Metrics()
	base := l.y + (h-(m.Ascent+m.Descent).Ceil())/2 + m.Ascent.Ceil()
	x := fixed.I(left)
	var extra fixed.Int26_6
	switch {
	case align == Center:
		x += (fixed.I(width) - w) / 2
	case align == Right:
		x += fixed.I(width) - w
	case align == Justify && !last && spaces > 0:
		extra = (fixed.I(width) - w) / fixed.Int26_6(spaces)
		if extra > fixed.I(l.s.Px) { // a line with a few long words: left as it is
			extra = 0
		}
	}
	if firstLine && b.Bullet != "" {
		bf := l.faces.Face(0, l.s.Px)
		bw := font.MeasureString(bf, b.Bullet+" ")
		l.page.Items = append(l.page.Items, Item{Kind: ItemText, X: (x - bw).Round(), Y: base, Text: b.Bullet, Px: l.s.Px})
	}
	items := l.page.Items
	for _, f := range line {
		if f.space {
			if n := len(items); extra == 0 && n > len(l.page.Items) && items[n-1].Style == f.style&^(Sup|Sub) && items[n-1].W == x.Round() {
				items[n-1].Text += f.text // with the words around it, when it isn't stretched
				items[n-1].W = (x + f.w).Round()
			}
			x += f.w + extra
			continue
		}
		if f.text == "​" || f.text == "" {
			continue
		}
		y := base
		if f.style&Sup != 0 {
			y -= maxPx * 35 / 100
		} else if f.style&Sub != 0 {
			y += maxPx * 15 / 100
		}
		// one item for a run in one style, up to a stretched space
		if n := len(items); n > len(l.page.Items) && extra == 0 && items[n-1].Kind == ItemText && items[n-1].Y == y &&
			items[n-1].Style == f.style&^(Sup|Sub) && items[n-1].Px == f.px && items[n-1].W == x.Round() {
			items[n-1].Text += f.text
			items[n-1].W = (x + f.w).Round()
		} else {
			items = append(items, Item{Kind: ItemText, X: x.Round(), Y: y, Text: f.text, Style: f.style &^ (Sup | Sub), Px: f.px, W: (x + f.w).Round()})
		}
		x += f.w
	}
	// W is where it ends, while it's set; a text item's W is its width
	for k := len(l.page.Items); k < len(items); k++ {
		items[k].W -= items[k].X
	}
	l.page.Items = items
	l.y += h
	l.placed = true
}

// splitFrag breaks fs[i] where it stops fitting in room: as much as
// fits (one character at least), then the rest.
func splitFrag(fs []frag, i int, face font.Face, room fixed.Int26_6) []frag {
	f := fs[i]
	cut, w := 0, fixed.Int26_6(0)
	for k, r := range f.text {
		a, _ := face.GlyphAdvance(r)
		if w+a > room && k > 0 {
			break
		}
		w += a
		cut = k + utf8.RuneLen(r)
	}
	if room <= 0 {
		cut, w = 0, 0
	}
	if cut >= len(f.text) {
		return fs
	}
	a, b := f, f
	a.text, a.w = f.text[:cut], w
	b.text, b.off = f.text[cut:], f.off+cut
	b.w = font.MeasureString(face, b.text)
	out := append(fs[:i:i], a, b)
	return append(out, fs[i+1:]...)
}
