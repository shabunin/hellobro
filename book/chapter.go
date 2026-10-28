package book

// A chapter's XHTML as blocks: paragraphs of styled spans, headings,
// pictures, rules. Styles come from the tags (b, i, h1, pre…), the
// book's CSS as far as css.go follows it, and style attributes.

import (
	"encoding/xml"
	"strconv"
	"strings"
)

// Style is a span's font: bold, italic, monospace, raised, lowered.
type Style uint8

const (
	Bold Style = 1 << iota
	Italic
	Mono
	Sup
	Sub
)

// Span is text in one style. "\n" alone is a line break.
type Span struct {
	Text  string
	Style Style
	Scale float32 // its size, of the body text's
}

type Kind uint8

const (
	Text Kind = iota
	Picture
	Rule
)

type Align uint8

const (
	Justify Align = iota
	Left
	Center
	Right
)

// Block is a paragraph, a heading, a picture or a rule.
type Block struct {
	Kind   Kind
	Spans  []Span
	Src    string // a picture: its path in the book
	Align  Align
	Pre    bool    // spaces and lines kept as they are
	Head   int     // 1-6: a heading
	Indent float32 // the left margin, ems
	First  float32 // the first line's indent, ems
	Above  float32 // space above it, ems
	Bullet string  // a list item's mark
	Float  Float   // a picture: text goes beside it
}

// Float is where a picture floats: the text goes by it on the other side.
type Float uint8

const (
	NoFloat Float = iota
	FloatLeft
	FloatRight
)

// Chapter is a spine item's blocks, and where its anchors are.
type Chapter struct {
	Blocks []Block
	IDs    map[string]int // id: the block it's in (or the one after)
}

// cstyle is what an element passes on to what's in it.
type cstyle struct {
	style  Style
	scale  float32
	align  Align
	indent float32
	first  float32
	pre    bool
}

type frame struct {
	tag   string
	st    cstyle
	block bool
	below float32 // space after it, ems
	list  int     // a list: its items so far (ol: numbered; -1 ul)
	float Float
}

var blockTags = map[string]bool{
	"p": true, "div": true, "h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"blockquote": true, "li": true, "pre": true, "dt": true, "dd": true, "dl": true, "ul": true, "ol": true,
	"table": true, "tr": true, "section": true, "article": true, "aside": true, "header": true, "footer": true,
	"figure": true, "figcaption": true, "body": true, "center": true, "address": true, "main": true, "nav": true,
	"caption": true, "hr": true,
}

var headScale = [7]float32{1, 1.6, 1.4, 1.2, 1.1, 1, 0.9}

// Chapter reads the i-th spine item.
func (b *Book) Chapter(i int) (*Chapter, error) {
	name := b.Spine[i]
	data, err := b.read(name)
	if err != nil {
		return nil, err
	}
	p := &parser{b: b, name: name, ch: &Chapter{IDs: map[string]int{}}}
	p.parse(data)
	return p.ch, nil
}

type parser struct {
	b      *Book
	name   string
	ch     *Chapter
	sheets []*sheet
	stack  []frame
	cur    *Block
	gap    float32 // the most space asked for since the last block
	bullet string  // the next block's
	skip   int     // inside something not shown: how deep
	style  bool    // in a <style>: its text is CSS
	css    strings.Builder
	decls  []decl
}

func (p *parser) top() *frame { return &p.stack[len(p.stack)-1] }

func (p *parser) parse(data []byte) {
	d := newDecoder(data)
	p.stack = []frame{{tag: "", st: cstyle{scale: 1}}}
	for {
		tok, err := d.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			p.start(t)
		case xml.EndElement:
			p.end(strings.ToLower(t.Name.Local))
		case xml.CharData:
			if p.style {
				p.css.Write(t)
			} else if p.skip == 0 {
				p.text(string(t))
			}
		}
	}
	p.flush()
}

func (p *parser) start(t xml.StartElement) {
	tag := strings.ToLower(t.Name.Local)
	if p.skip > 0 {
		p.skip++
		return
	}
	switch tag {
	case "head", "script", "title":
		if tag == "head" {
			p.stack = append(p.stack, frame{tag: tag, st: p.top().st})
			return
		}
		p.skip = 1
		return
	case "style":
		p.style = true
		p.css.Reset()
		return
	case "link":
		if strings.Contains(attr(t, "rel"), "stylesheet") {
			p.addSheet(attr(t, "href"))
		}
		return
	case "meta", "base":
		return
	}
	parent := p.top()
	f := frame{tag: tag, st: parent.st, block: blockTags[tag]}
	st := &f.st
	var above float32
	switch tag {
	case "b", "strong", "th":
		st.style |= Bold
	case "i", "em", "cite", "var", "dfn":
		st.style |= Italic
	case "code", "tt", "kbd", "samp":
		st.style |= Mono
	case "sup":
		st.style |= Sup
	case "sub":
		st.style |= Sub
	case "small":
		st.scale *= 0.85
	case "big":
		st.scale *= 1.2
	case "h1", "h2", "h3", "h4", "h5", "h6":
		n := int(tag[1] - '0')
		st.style |= Bold
		st.scale *= headScale[n]
		st.first = 0
		above, f.below = 1.2, 0.6
	case "p":
		above, f.below = 0.4, 0.4
	case "blockquote":
		st.indent += 1.5
		above, f.below = 0.5, 0.5
	case "ul", "ol":
		st.indent += 1.5
		above, f.below = 0.3, 0.3
		f.list = -1
		if tag == "ol" {
			f.list = 0
		}
	case "dd":
		st.indent += 1.5
	case "pre":
		st.style |= Mono
		st.pre = true
		st.align = Left
		above, f.below = 0.5, 0.5
	case "center":
		st.align = Center
	case "figure", "figcaption", "caption":
		st.align = Center
	case "li":
		mark := "•"
		for i := len(p.stack) - 1; i >= 0; i-- {
			if l := &p.stack[i]; l.tag == "ul" || l.tag == "ol" {
				if l.list >= 0 {
					l.list++
					mark = strconv.Itoa(l.list) + "."
				}
				break
			}
		}
		p.bullet = mark
	}
	hidden := p.applyCSS(t, tag, &f, &above)
	if hidden {
		p.skip = 1
		return
	}
	if id := attr(t, "id"); id != "" {
		p.ch.IDs[id] = len(p.ch.Blocks)
	}
	if f.block {
		p.flush()
		p.gap = max(p.gap, above)
	}
	switch tag {
	case "img", "image":
		src := attr(t, "src")
		if src == "" {
			src = attr(t, "href")
		}
		if src != "" {
			p.flush()
			path, _ := resolve(p.name, src)
			fl := f.float // its own, or its block's: <div class="figleft"><img>
			for i := len(p.stack) - 1; i > 0 && fl == NoFloat; i-- {
				fl = p.stack[i].float
				if p.stack[i].block {
					break
				}
			}
			p.add(Block{Kind: Picture, Src: path, Align: Center, Float: fl})
		}
	case "hr":
		p.add(Block{Kind: Rule, Indent: st.indent})
	case "br":
		p.span("\n", st)
	case "td":
		if p.cur != nil {
			p.span(" ", st)
		}
	}
	p.stack = append(p.stack, f)
}

func (p *parser) end(tag string) {
	if p.skip > 0 {
		p.skip--
		return
	}
	if tag == "style" && p.style {
		p.style = false
		p.sheets = append(p.sheets, parseCSS(p.css.String()))
		return
	}
	// pop to the element that ends (tags left open end with it)
	for i := len(p.stack) - 1; i > 0; i-- {
		if p.stack[i].tag != tag {
			continue
		}
		for len(p.stack) > i {
			f := p.stack[len(p.stack)-1]
			p.stack = p.stack[:len(p.stack)-1]
			if f.block {
				p.flush()
				p.gap = max(p.gap, f.below)
			}
		}
		return
	}
}

func (p *parser) addSheet(href string) {
	name, _ := resolve(p.name, href)
	s := p.b.css[name]
	if s == nil {
		data, err := p.b.read(name)
		if err != nil {
			return
		}
		s = parseCSS(string(data))
		p.b.css[name] = s
	}
	p.sheets = append(p.sheets, s)
}

// applyCSS gives f what the book's CSS and the element's style say;
// true if it's not to be shown.
func (p *parser) applyCSS(t xml.StartElement, tag string, f *frame, above *float32) bool {
	classes := strings.Fields(strings.ToLower(attr(t, "class")))
	ds := p.decls[:0]
	for _, s := range p.sheets {
		ds = s.matching(tag, classes, ds)
	}
	if style := attr(t, "style"); style != "" {
		ds = append(ds, parseDecls(strings.ToLower(style))...)
	}
	p.decls = ds
	parent := p.top().st
	st := &f.st
	for _, d := range ds {
		switch d.prop {
		case "display":
			switch d.val {
			case "none":
				return true
			case "block":
				f.block = true
			case "inline", "inline-block":
				f.block = false
			}
		case "float":
			switch d.val {
			case "left":
				f.float = FloatLeft
			case "right":
				f.float = FloatRight
			}
		case "text-align":
			switch d.val {
			case "center":
				st.align = Center
			case "right", "end":
				st.align = Right
			case "left", "start":
				st.align = Left
			case "justify":
				st.align = Justify
			}
		case "font-style":
			if d.val == "italic" || d.val == "oblique" {
				st.style |= Italic
			} else if d.val == "normal" {
				st.style &^= Italic
			}
		case "font-weight":
			n, _ := strconv.Atoi(d.val)
			if d.val == "bold" || d.val == "bolder" || n >= 600 {
				st.style |= Bold
			} else if d.val == "normal" || d.val == "lighter" || (n > 0 && n < 600) {
				st.style &^= Bold
			}
		case "font-family":
			if strings.Contains(d.val, "mono") || strings.Contains(d.val, "courier") {
				st.style |= Mono
			}
		case "font-size":
			if s, ok := fontScale(d.val); ok {
				st.scale = min(2.5, max(0.6, parent.scale*s))
			}
		case "vertical-align":
			if d.val == "super" {
				st.style |= Sup
			} else if d.val == "sub" {
				st.style |= Sub
			}
		case "white-space":
			st.pre = strings.HasPrefix(d.val, "pre")
		case "text-indent":
			if v, ok := length(d.val); ok {
				st.first = min(4, max(0, v))
			}
		case "margin-left", "padding-left":
			if v, ok := length(d.val); ok {
				st.indent = parent.indent + min(6, max(0, v))
			}
		case "margin-top":
			if v, ok := length(d.val); ok {
				*above = min(3, max(0, v))
			}
		case "margin-bottom":
			if v, ok := length(d.val); ok {
				f.below = min(3, max(0, v))
			}
		case "margin":
			vs := strings.Fields(d.val)
			var ems []float32
			for _, v := range vs {
				e, ok := length(v)
				if !ok {
					e = 0
				}
				ems = append(ems, min(6, max(0, e)))
			}
			// top, right, bottom, left; those missing as the one across
			for len(ems) > 0 && len(ems) < 4 {
				ems = append(ems, ems[max(0, len(ems)-2)])
			}
			if len(ems) == 4 {
				*above, f.below = min(3, ems[0]), min(3, ems[2])
				st.indent = parent.indent + ems[3]
			}
		}
	}
	return false
}

// text adds character data to the block being built.
func (p *parser) text(s string) {
	st := &p.top().st
	if !st.pre {
		s = collapse(s)
		if p.cur == nil && strings.TrimSpace(s) == "" {
			return
		}
	}
	p.span(s, st)
}

// collapse makes each run of white space one space.
func collapse(s string) string {
	var b strings.Builder
	space := false
	for _, r := range s {
		if r == ' ' || r == '\n' || r == '\t' || r == '\r' || r == '\f' {
			space = true
			continue
		}
		if space {
			b.WriteByte(' ')
			space = false
		}
		b.WriteRune(r)
	}
	if space {
		b.WriteByte(' ')
	}
	return b.String()
}

func (p *parser) span(s string, st *cstyle) {
	if p.cur == nil { // laid out as the block it's in says
		bst := p.stack[0].st
		p.cur = &Block{}
		for i := len(p.stack) - 1; i >= 0; i-- {
			if f := p.stack[i]; f.block {
				if len(f.tag) == 2 && f.tag[0] == 'h' && f.tag[1] >= '1' && f.tag[1] <= '6' {
					p.cur.Head = int(f.tag[1] - '0')
				}
				bst = f.st
				break
			}
		}
		p.cur.Align, p.cur.Pre, p.cur.Indent, p.cur.First = bst.align, bst.pre, bst.indent, bst.first
	}
	sp := p.cur.Spans
	if n := len(sp); n > 0 && s != "\n" && sp[n-1].Text != "\n" && sp[n-1].Style == st.style && sp[n-1].Scale == st.scale {
		sp[n-1].Text += s
		return
	}
	p.cur.Spans = append(sp, Span{s, st.style, st.scale})
}

// flush ends the block being built, if it has anything to show.
func (p *parser) flush() {
	c := p.cur
	p.cur = nil
	if c == nil {
		return
	}
	// trim the white space at its ends
	for len(c.Spans) > 0 && strings.TrimSpace(c.Spans[0].Text) == "" && !c.Pre {
		c.Spans = c.Spans[1:]
	}
	for len(c.Spans) > 0 && strings.TrimSpace(c.Spans[len(c.Spans)-1].Text) == "" && !(c.Pre && c.Spans[len(c.Spans)-1].Text != "\n") {
		c.Spans = c.Spans[:len(c.Spans)-1]
	}
	if len(c.Spans) == 0 {
		return
	}
	if !c.Pre {
		c.Spans[0].Text = strings.TrimLeft(c.Spans[0].Text, " ")
		n := len(c.Spans) - 1
		c.Spans[n].Text = strings.TrimRight(c.Spans[n].Text, " ")
	}
	p.add(*c)
}

func (p *parser) add(b Block) {
	b.Above = p.gap
	if b.Kind == Text {
		b.Bullet, p.bullet = p.bullet, ""
	}
	p.gap = 0
	p.ch.Blocks = append(p.ch.Blocks, b)
}
