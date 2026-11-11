// Package md reads the Markdown a language model answers in: its blocks
// (paragraphs, headings, list items, quotes, code, formulas, tables,
// rules) and, in their text, its spans (bold, italic, code, formulas).
// What models write, not all of CommonMark: a new line in a paragraph
// is kept, as chats keep it.
package md

import (
	"regexp"
	"strings"
)

// Kind is a block's kind.
type Kind int

const (
	Para Kind = iota
	Heading
	Item
	Code
	Math
	Table
	Rule
)

// Block is a block of the text.
type Block struct {
	Kind   Kind
	Text   string     // a paragraph's, heading's or item's (Markdown in it); code; a formula's TeX
	Level  int        // a heading's, 1-6; an item's depth (0 the outermost)
	Marker string     // an item's: "•", "1."
	Lang   string     // code's language
	Quote  int        // how deep in quotes ("> ") it is
	Indent int        // a block under a list item: as deep as its text
	Rows   [][]string // a table's cells (Markdown in them), its head first
	Align  []byte     // a table's columns: 'l', 'c', 'r'
}

var (
	headingRE = regexp.MustCompile(`^(#{1,6})(?:\s+(.*?))?\s*#*\s*$`)
	ruleRE    = regexp.MustCompile(`^(?:(?:\*\s*){3,}|(?:-\s*){3,}|(?:_\s*){3,})$`)
	setextRE  = regexp.MustCompile(`^(=+|-+)$`)
	itemRE    = regexp.MustCompile(`^( *)([-*+•]|\d{1,3}[.)])(?:\s+(.*))?$`)
	sepRE     = regexp.MustCompile(`^\|?\s*:?-+:?\s*(\|\s*:?-+:?\s*)*\|?$`)
	envRE     = regexp.MustCompile(`^\\begin\{(equation|align|aligned|gather|gathered|multline|eqnarray|cases|matrix|pmatrix|bmatrix|vmatrix|split|alignat)\*?\}`)
)

// Parse reads src's blocks.
func Parse(src string) []Block {
	return parse(strings.Split(strings.ReplaceAll(src, "\r", ""), "\n"), 0)
}

func parse(lines []string, quote int) []Block {
	var bs []Block
	var para []string
	paraIndent := 0
	var cols []int // the open list items' indents, outermost first
	flush := func() {
		if len(para) > 0 {
			bs = append(bs, Block{Kind: Para, Text: strings.Join(para, "\n"), Quote: quote, Indent: paraIndent})
			para = nil
		}
	}
	under := func(ind int) int { // how deep in the list a block indented ind is
		n := 0
		for _, c := range cols {
			if c < ind {
				n++
			}
		}
		return n
	}
	for i := 0; i < len(lines); i++ {
		line := strings.ReplaceAll(lines[i], "\t", "    ")
		t := strings.TrimSpace(line)
		ind := len(line) - len(strings.TrimLeft(line, " "))
		if t == "" {
			flush()
			continue
		}
		if strings.HasPrefix(t, ">") {
			flush()
			var ql []string
			for ; i < len(lines); i++ {
				q := strings.TrimSpace(lines[i])
				if !strings.HasPrefix(q, ">") {
					break
				}
				ql = append(ql, strings.TrimPrefix(strings.TrimPrefix(q, ">"), " "))
			}
			i--
			bs = append(bs, parse(ql, quote+1)...)
			continue
		}
		if f := fence(t); f != "" {
			flush()
			var code []string
			for i++; i < len(lines); i++ {
				l := strings.ReplaceAll(lines[i], "\t", "    ")
				if c := strings.TrimSpace(l); strings.HasPrefix(c, f) && strings.Trim(c, f[:1]) == "" {
					break
				}
				code = append(code, dedent(l, ind))
			}
			bs = append(bs, Block{Kind: Code, Text: strings.Join(code, "\n"), Lang: strings.TrimSpace(t[len(f):]), Quote: quote, Indent: under(ind)})
			continue
		}
		if open, close, keep := mathOpen(t); open != "" {
			flush()
			var b strings.Builder
			rest, after := t[len(open):], ""
			for {
				if j := strings.Index(rest, close); j >= 0 {
					b.WriteString(rest[:j])
					after = strings.TrimSpace(rest[j+len(close):])
					break
				}
				b.WriteString(rest + "\n")
				if i+1 >= len(lines) {
					break
				}
				i++
				rest = lines[i]
			}
			tex := strings.TrimSpace(b.String())
			if keep {
				tex = open + "\n" + tex + "\n" + close
			}
			bs = append(bs, Block{Kind: Math, Text: tex, Quote: quote, Indent: under(ind)})
			if after != "" {
				para, paraIndent = []string{after}, under(ind)
			}
			continue
		}
		if m := headingRE.FindStringSubmatch(t); m != nil {
			flush()
			bs = append(bs, Block{Kind: Heading, Level: len(m[1]), Text: m[2], Quote: quote})
			cols = nil
			continue
		}
		if len(para) > 0 && setextRE.MatchString(t) {
			bs = append(bs, Block{Kind: Heading, Level: map[byte]int{'=': 1, '-': 2}[t[0]], Text: strings.Join(para, " "), Quote: quote})
			para = nil
			continue
		}
		if ruleRE.MatchString(t) {
			flush()
			bs = append(bs, Block{Kind: Rule, Quote: quote})
			continue
		}
		if strings.Contains(t, "|") && i+1 < len(lines) && sepRE.MatchString(strings.TrimSpace(lines[i+1])) {
			flush()
			rows := [][]string{cells(t)}
			align := aligns(strings.TrimSpace(lines[i+1]))
			for i += 2; i < len(lines); i++ {
				r := strings.TrimSpace(lines[i])
				if r == "" || !strings.Contains(r, "|") {
					break
				}
				rows = append(rows, cells(r))
			}
			i--
			bs = append(bs, Block{Kind: Table, Rows: rows, Align: align, Quote: quote, Indent: under(ind)})
			continue
		}
		if m := itemRE.FindStringSubmatch(line); m != nil {
			flush()
			col := len(m[1])
			for len(cols) > 0 && cols[len(cols)-1] > col {
				cols = cols[:len(cols)-1]
			}
			if len(cols) == 0 || cols[len(cols)-1] < col {
				cols = append(cols, col)
			}
			level := len(cols) - 1
			marker, text := m[2], m[3]
			switch {
			case strings.ContainsAny(marker, "-*+•"):
				marker = []string{"•", "◦", "▪"}[level%3]
			default:
				marker = strings.TrimRight(marker, ".)") + "."
			}
			switch {
			case strings.HasPrefix(text, "[ ] "):
				marker, text = "☐", text[4:]
			case strings.HasPrefix(text, "[x] "), strings.HasPrefix(text, "[X] "):
				marker, text = "☑", text[4:]
			}
			bs = append(bs, Block{Kind: Item, Level: level, Marker: marker, Text: text, Quote: quote})
			continue
		}
		if len(para) == 0 && len(bs) > 0 && bs[len(bs)-1].Kind == Item && ind > 0 && strings.TrimSpace(lines[i-1]) != "" {
			bs[len(bs)-1].Text += "\n" + t // the item's next line
			continue
		}
		if ind == 0 {
			cols = nil // out of the list
		}
		if len(para) == 0 {
			paraIndent = under(ind)
		}
		para = append(para, t)
	}
	flush()
	return bs
}

// fence is a code fence's backticks (or tildes), if t opens one.
func fence(t string) string {
	for _, c := range []string{"`", "~"} {
		n := len(t) - len(strings.TrimLeft(t, c))
		if n >= 3 {
			return t[:n]
		}
	}
	return ""
}

// mathOpen says how a formula on its own begins and ends ($$ and $$,
// \[ and \]), if t begins one; keep, if what begins and ends it is the
// formula's own (\begin{align} and \end{align}).
func mathOpen(t string) (open, close string, keep bool) {
	switch {
	case strings.HasPrefix(t, "$$"):
		return "$$", "$$", false
	case strings.HasPrefix(t, `\[`):
		return `\[`, `\]`, false
	}
	if m := envRE.FindStringSubmatch(t); m != nil {
		return m[0], `\end{` + m[0][len(`\begin{`):], true
	}
	return "", "", false
}

// dedent takes up to n spaces off l's front.
func dedent(l string, n int) string {
	for n > 0 && strings.HasPrefix(l, " ") {
		l, n = l[1:], n-1
	}
	return l
}

// cells are a table row's cells: between |s not in code or a formula,
// nor written \|.
func cells(row string) []string {
	row = strings.TrimSpace(row)
	row = strings.TrimPrefix(row, "|")
	if strings.HasSuffix(row, "|") && !strings.HasSuffix(row, `\|`) {
		row = row[:len(row)-1]
	}
	var cs []string
	var b strings.Builder
	code, math := false, false
	for i := 0; i < len(row); i++ {
		c := row[i]
		switch {
		case c == '\\' && i+1 < len(row) && row[i+1] == '|':
			b.WriteByte('|')
			i++
			continue
		case c == '`' && !math:
			code = !code
		case c == '$' && !code:
			math = !math
		case c == '|' && !code && !math:
			cs = append(cs, strings.TrimSpace(b.String()))
			b.Reset()
			continue
		}
		b.WriteByte(c)
	}
	return append(cs, strings.TrimSpace(b.String()))
}

// aligns are a table's columns' alignments, from the line under its
// head: :-- left, --: right, :-: centered.
func aligns(sep string) []byte {
	var as []byte
	for _, c := range cells(sep) {
		l, r := strings.HasPrefix(c, ":"), strings.HasSuffix(c, ":")
		switch {
		case l && r:
			as = append(as, 'c')
		case r:
			as = append(as, 'r')
		default:
			as = append(as, 'l')
		}
	}
	return as
}

// Style is a span's: bold, italic, code (in a mono font), struck out.
type Style uint8

const (
	Bold Style = 1 << iota
	Italic
	Mono
	Strike
)

// Span is a piece of a block's text, in a style; or a formula; or a
// new line.
type Span struct {
	Text    string
	Style   Style
	Math    bool // Text is TeX: $...$, \(...\)
	Display bool // a formula on its own line: $$...$$ in a paragraph
	Break   bool
}

// Inline reads the spans of a block's text.
func Inline(s string) []Span {
	var p inline
	p.parse(s, 0, 0)
	return p.spans
}

type inline struct{ spans []Span }

var tagRE = regexp.MustCompile(`^<(/?)([a-zA-Z]+)[^<>]*>`)

func (p *inline) parse(s string, st Style, depth int) {
	var buf strings.Builder
	flush := func() {
		if buf.Len() > 0 {
			p.spans = append(p.spans, Span{Text: buf.String(), Style: st})
			buf.Reset()
		}
	}
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '\n':
			flush()
			p.spans = append(p.spans, Span{Break: true})
			i++
			continue
		case c == '\\' && i+1 < len(s):
			switch n := s[i+1]; {
			case n == '(' || n == '[':
				end := map[byte]string{'(': `\)`, '[': `\]`}[n]
				if j := strings.Index(s[i+2:], end); j >= 0 {
					flush()
					p.spans = append(p.spans, Span{Text: s[i+2 : i+2+j], Math: true, Display: n == '['})
					i += j + 4
					continue
				}
			case strings.IndexByte("\\`*_{}[]()#+-.!|~$<>", n) >= 0:
				buf.WriteByte(n)
				i += 2
				continue
			}
		case c == '`':
			k := run(s, i, '`')
			if j := closing(s, i+k, k, '`', false); j >= 0 {
				flush()
				code := s[i+k : j]
				if len(code) > 1 && code[0] == ' ' && code[len(code)-1] == ' ' {
					code = code[1 : len(code)-1]
				}
				p.spans = append(p.spans, Span{Text: code, Style: st | Mono})
				i = j + k
				continue
			}
			buf.WriteString(s[i : i+k])
			i += k
			continue
		case c == '$':
			if strings.HasPrefix(s[i:], "$$") {
				if j := strings.Index(s[i+2:], "$$"); j > 0 {
					flush()
					p.spans = append(p.spans, Span{Text: strings.TrimSpace(s[i+2 : i+2+j]), Math: true, Display: true})
					i += j + 4
					continue
				}
			} else if j := mathEnd(s, i+1); j > 0 {
				flush()
				p.spans = append(p.spans, Span{Text: s[i+1 : j], Math: true})
				i = j + 1
				continue
			}
		case (c == '*' || c == '_' || c == '~') && depth < 8:
			k := min(3, run(s, i, c))
			if c == '~' && k != 2 {
				break
			}
			if canOpen(s, i, k, c) {
				if j := closing(s, i+k, k, c, true); j > i+k {
					flush()
					add := map[int]Style{1: Italic, 2: Bold, 3: Bold | Italic}[k]
					if c == '~' {
						add = Strike
					}
					p.parse(s[i+k:j], st|add, depth+1)
					i = j + k
					continue
				}
			}
			buf.WriteString(s[i : i+k])
			i += k
			continue
		case c == '!' && strings.HasPrefix(s[i+1:], "["): // a picture: its text
			if link(s, i+1) > 0 {
				i++
				continue
			}
		case c == '[':
			if e := link(s, i); e > 0 {
				flush()
				p.parse(s[i+1:strings.Index(s[i:], "](")+i], st, depth+1)
				i = e
				continue
			}
		case c == '<':
			if m := tagRE.FindStringSubmatch(s[i:]); m != nil {
				if strings.EqualFold(m[2], "br") {
					flush()
					p.spans = append(p.spans, Span{Break: true})
				}
				i += len(m[0])
				continue
			}
		}
		buf.WriteByte(c)
		i++
	}
	flush()
}

// run is how many c there are from s[i] on.
func run(s string, i int, c byte) int {
	n := 0
	for i+n < len(s) && s[i+n] == c {
		n++
	}
	return n
}

// canOpen says whether the k c at s[i] can begin a span: something but
// a space after them; for _, not in a word.
func canOpen(s string, i, k int, c byte) bool {
	if i+k >= len(s) || s[i+k] == ' ' || s[i+k] == '\n' {
		return false
	}
	return c != '_' || i == 0 || !isWordByte(s[i-1])
}

func isWordByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b >= 0x80
}

// closing is where the k c that end a span begun before from are: just
// k of them, after something but a space; for _, not in a word. -1 if
// none. Code spans and formulas on the way are skipped (in emphasis).
func closing(s string, from, k int, c byte, emph bool) int {
	for j := from; j < len(s); j++ {
		switch {
		case emph && s[j] == '`':
			n := run(s, j, '`')
			if e := closing(s, j+n, n, '`', false); e >= 0 {
				j = e + n - 1
				continue
			}
		case emph && s[j] == '$':
			if e := mathEnd(s, j+1); e > 0 {
				j = e
				continue
			}
		case s[j] == '\\' && emph:
			j++
			continue
		}
		if s[j] != c {
			continue
		}
		n := run(s, j, c)
		if n == k && (!emph || s[j-1] != ' ' && s[j-1] != '\n') && (c != '_' || j+n >= len(s) || !isWordByte(s[j+n])) {
			return j
		}
		j += n - 1
	}
	return -1
}

// mathEnd is where the $ that ends a formula begun before from is: a
// formula begins with no space after its $, and ends with none before
// its $ and no digit after ("$5 and $6" is none). -1 if none.
func mathEnd(s string, from int) int {
	if from >= len(s) || s[from] == ' ' || s[from] == '$' {
		return -1
	}
	for j := from; j < len(s); j++ {
		switch s[j] {
		case '\\':
			j++
		case '$':
			if s[j-1] != ' ' && (j+1 >= len(s) || s[j+1] < '0' || s[j+1] > '9') {
				return j
			}
			return -1
		}
	}
	return -1
}

// link is where a link ([its text](where)) at s[i] ends; 0 if it isn't
// one.
func link(s string, i int) int {
	if i >= len(s) || s[i] != '[' {
		return 0
	}
	m := strings.Index(s[i:], "](")
	if m <= 0 || strings.Contains(s[i:i+m], "\n") {
		return 0
	}
	e := strings.IndexByte(s[i+m:], ')')
	if e < 0 {
		return 0
	}
	return i + m + e + 1
}
