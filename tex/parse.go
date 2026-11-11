package tex

import (
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

// tok is a token of TeX: a command (its name), a character, or one of
// { } ^ _ & (as kind), \\ ('n'), a space.
type tok struct {
	kind byte // 'c', 'x', '{', '}', '^', '_', '&', 'n', ' '; 0 at the end
	s    string
}

func tokenize(src string) []tok {
	rs := []rune(src)
	var ts []tok
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch {
		case r == '\\' && i+1 < len(rs):
			j := i + 1
			switch {
			case isASCIILetter(rs[j]):
				for j < len(rs) && isASCIILetter(rs[j]) {
					j++
				}
				name := string(rs[i+1 : j])
				if j < len(rs) && rs[j] == '*' && starred[name] {
					name += "*"
					j++
				}
				ts = append(ts, tok{'c', name})
				i = j - 1
			case rs[j] == '\\':
				ts = append(ts, tok{'n', ""})
				i = j
			default:
				ts = append(ts, tok{'c', string(rs[j])})
				i = j
			}
		case r == '{' || r == '}' || r == '^' || r == '_' || r == '&':
			ts = append(ts, tok{byte(r), ""})
		case unicode.IsSpace(r):
			if len(ts) == 0 || ts[len(ts)-1].kind != ' ' {
				ts = append(ts, tok{' ', ""})
			}
		case r == '~':
			ts = append(ts, tok{'c', " "})
		default:
			ts = append(ts, tok{'x', string(r)})
		}
	}
	return ts
}

// starred are the commands with a starred kind, the star theirs.
var starred = map[string]bool{"operatorname": true, "tag": true, "hspace": true, "vspace": true}

func isASCIILetter(r rune) bool { return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' }

func tokString(t tok) string {
	switch t.kind {
	case 'x':
		return t.s
	case 'c':
		return `\` + t.s
	case ' ':
		return " "
	case 'n':
		return `\\`
	case 0:
		return ""
	}
	return string(t.kind)
}

type parser struct {
	ts      []tok
	i       int
	em      float64
	font    string // the letters': "it" as math has them, "rm", "bf"...
	depth   int
	unknown []string
}

// maxDepth is how deep groups go before the rest's left out: a formula
// gone wrong mustn't take the stack.
const maxDepth = 40

func newParser(src string, em float64) *parser {
	return &parser{ts: tokenize(src), em: em, font: "it"}
}

func (p *parser) peek() tok {
	if p.i < len(p.ts) {
		return p.ts[p.i]
	}
	return tok{}
}

func (p *parser) next() tok {
	t := p.peek()
	if p.i < len(p.ts) {
		p.i++
	}
	return t
}

func (p *parser) skipSpace() {
	for p.i < len(p.ts) && p.ts[p.i].kind == ' ' {
		p.i++
	}
}

func (p *parser) isX(s string) bool { t := p.peek(); return t.kind == 'x' && t.s == s }

// size is the letters' size in st; axis, where fractions' bars and
// minus signs are, over the baseline; rule, a bar's thickness.
func (p *parser) size(st int) float64 { return p.em * styleScale[st] }
func (p *parser) axis(st int) float64 { return 0.25 * p.size(st) }
func (p *parser) rule(st int) float64 { return max(1, 0.055*p.size(st)) }

func (p *parser) unknownCmd(name string) {
	if !slices.Contains(p.unknown, name) {
		p.unknown = append(p.unknown, name)
	}
}

// atom is a piece of a formula, of a class, with its scripts.
type atom struct {
	class    int
	b        *box
	sup, sub *box
	limits   int  // \limits (1), \nolimits (-1), or as its kind has it (0)
	takes    bool // takes limits over and under in display style
	integral bool
	char     bool // one sign: scripts go by the font's rules, not its size
	letter   bool
	glue     bool // a space: no class
}

// top is the whole formula: one line, or rows of them (\\), aligned at
// & if they have them.
func (p *parser) top(st int) *box {
	rows, hl := p.cells(st, func(t tok) bool { return false }, false)
	if len(rows) == 1 && len(rows[0]) == 1 {
		return rows[0][0]
	}
	cols := 1
	for _, r := range rows {
		cols = max(cols, len(r))
	}
	if cols == 1 {
		return p.grid(rows, "c", func(int) float64 { return 0 }, 0.3, st, hl)
	}
	return p.grid(rows, "rl", alignedGap(p.size(st)), 0.3, st, hl)
}

func alignedGap(em float64) func(int) float64 {
	return func(j int) float64 {
		if j%2 == 1 {
			return 0
		}
		return 2 * em
	}
}

// cells are rows of cells, & between them and \\ after each, till end
// says it's the end (not taken), or the formula's. pad: in aligned
// equations, the cells after an & begin with an empty atom, so a
// relation there is spaced as one.
func (p *parser) cells(st int, end func(tok) bool, pad bool) (rows [][]*box, hlines []int) {
	var row []*box
	hline := func() {
		for {
			p.skipSpace()
			if t := p.peek(); t.kind == 'c' && (t.s == "hline" || t.s == "hdashline" || t.s == "midrule" || t.s == "toprule" || t.s == "bottomrule") {
				p.i++
				hlines = append(hlines, len(rows))
				continue
			}
			return
		}
	}
	hline()
	for {
		as := p.list(st, func(t tok) bool { return t.kind == '&' || t.kind == 'n' || end(t) })
		if pad && len(row)%2 == 1 {
			as = append([]*atom{{class: ord, b: &box{}}}, as...)
		}
		row = append(row, p.hlist(as, st))
		switch p.peek().kind {
		case '&':
			p.i++
		case 'n':
			p.i++
			p.skipOpt()
			rows, row = append(rows, row), nil
			hline()
		default:
			if len(row) > 1 || row[0].w > 0 || len(rows) == 0 {
				rows = append(rows, row)
			}
			return rows, hlines
		}
	}
}

// skipOpt skips an optional argument: \\[2pt]'s.
func (p *parser) skipOpt() {
	p.skipSpace()
	if !p.isX("[") {
		return
	}
	for p.i < len(p.ts) {
		if t := p.next(); t.kind == 'x' && t.s == "]" {
			return
		}
	}
}

// list is the atoms up to stop's token (not taken), or a '}' that's
// not ours, or the end.
func (p *parser) list(st int, stop func(tok) bool) []*atom {
	var as []*atom
	font := p.font
	defer func() { p.font = font }()
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > maxDepth {
		p.i = len(p.ts)
		return nil
	}
	last := func() *atom {
		if len(as) == 0 || as[len(as)-1].glue {
			as = append(as, &atom{class: ord, b: &box{}})
		}
		return as[len(as)-1]
	}
	for p.i < len(p.ts) {
		t := p.peek()
		if t.kind == ' ' {
			p.i++
			continue
		}
		if stop(t) || t.kind == '}' || t.kind == '&' || t.kind == 'n' {
			break
		}
		switch {
		case t.kind == '^' || t.kind == '_':
			p.i++
			a := last()
			s := p.arg(supStyle(st))
			if t.kind == '^' {
				a.sup = joinBox(a.sup, s)
			} else {
				a.sub = joinBox(a.sub, s)
			}
			continue
		case t.kind == 'x' && t.s == "'":
			n := 0
			for p.isX("'") {
				p.i++
				n++
			}
			a := last()
			a.b = hbox(a.b, glyph([]rune("′″‴⁗")[min(n, 4)-1], p.size(st)))
			continue
		case t.kind == 'c':
			switch t.s {
			case "displaystyle", "textstyle", "scriptstyle", "scriptscriptstyle":
				p.i++
				st = map[string]int{"displaystyle": styleD, "textstyle": styleT, "scriptstyle": styleS, "scriptscriptstyle": styleSS}[t.s]
				continue
			case "limits", "nolimits", "displaylimits":
				p.i++
				if len(as) > 0 {
					as[len(as)-1].limits = map[string]int{"limits": 1, "nolimits": -1}[t.s]
				}
				continue
			case "rm", "bf", "it", "cal", "sf", "tt", "mit":
				p.i++
				p.font = map[string]string{"rm": "rm", "bf": "bf", "it": "it", "mit": "it", "cal": "cal", "sf": "sf", "tt": "tt"}[t.s]
				continue
			case "over", "choose", "atop":
				p.i++
				num := p.hlist(as, fracStyle(st))
				den := p.hlist(p.list(fracStyle(st), stop), fracStyle(st))
				b := p.frac(num, den, st, t.s == "over")
				if t.s == "choose" {
					b = p.delimited('(', b, ')', st)
				}
				return []*atom{{class: inner, b: b}}
			}
		}
		if a := p.atom(st); a != nil {
			as = append(as, a)
		}
	}
	return as
}

func joinBox(a, b *box) *box {
	if a == nil {
		return b
	}
	return hbox(a, b)
}

// atom is the next atom, from a token: a group, a character, a command
// and its arguments. nil for none (a command that's only a setting).
func (p *parser) atom(st int) *atom {
	t := p.next()
	switch t.kind {
	case '{':
		as := p.list(st, func(t tok) bool { return false })
		if p.peek().kind == '}' {
			p.i++
		}
		return &atom{class: ord, b: p.hlist(as, st)}
	case 'x':
		return p.char([]rune(t.s)[0], st)
	case 'c':
		return p.command(t.s, st)
	}
	return nil
}

// argAtoms are an argument's atoms: a group's, or one atom's.
func (p *parser) argAtoms(st int) []*atom {
	p.skipSpace()
	switch p.peek().kind {
	case '{':
		p.i++
		as := p.list(st, func(t tok) bool { return false })
		if p.peek().kind == '}' {
			p.i++
		}
		return as
	case 0, '}', '&', 'n', '^', '_':
		return nil
	}
	if a := p.atom(st); a != nil {
		return []*atom{a}
	}
	return nil
}

func (p *parser) arg(st int) *box { return p.hlist(p.argAtoms(st), st) }

// optArg is an optional argument, in [ ].
func (p *parser) optArg(st int) (*box, bool) {
	p.skipSpace()
	if !p.isX("[") {
		return nil, false
	}
	p.i++
	as := p.list(st, func(t tok) bool { return t.kind == 'x' && t.s == "]" })
	if p.isX("]") {
		p.i++
	}
	return p.hlist(as, st), true
}

// rawGroup is an argument as it's written: an environment's name, a
// color.
func (p *parser) rawGroup() string {
	p.skipSpace()
	if p.peek().kind != '{' {
		return tokString(p.next())
	}
	p.i++
	depth := 1
	var b strings.Builder
	for p.i < len(p.ts) {
		t := p.next()
		if t.kind == '{' {
			depth++
		} else if t.kind == '}' {
			if depth--; depth == 0 {
				break
			}
		}
		b.WriteString(tokString(t))
	}
	return strings.TrimSpace(b.String())
}

// char is a character's atom: a letter (italic), a digit, a sign.
func (p *parser) char(r rune, st int) *atom {
	size := p.size(st)
	for _, op := range bigOps {
		if r == op {
			return p.bigOp(r, st)
		}
	}
	switch r {
	case '-':
		r = '−'
	case '*':
		r = '∗'
	}
	cls := ord
	if c, ok := unicodeClass[r]; ok {
		cls = c
	}
	isLetter := unicode.IsLetter(r)
	return &atom{class: cls, b: glyph(letter(r, p.font), size), char: true, letter: isLetter && p.font == "it"}
}

// fontCmds are the commands that set the letters' font for their
// argument.
var fontCmds = map[string]string{
	"mathrm": "rm", "mathup": "rm", "mathit": "it", "mathnormal": "it", "mathbf": "bf", "boldsymbol": "bfit",
	"bm": "bfit", "mathbfit": "bfit", "mathbb": "bb", "Bbb": "bb", "mathcal": "cal", "mathscr": "cal",
	"mathfrak": "frak", "mathsf": "sf", "mathtt": "tt",
}

// textCmds are the commands whose argument is text, in a font.
var textCmds = map[string]string{
	"text": "rm", "textrm": "rm", "textnormal": "rm", "textup": "rm", "mbox": "rm", "hbox": "rm",
	"textbf": "bf", "textit": "it", "emph": "it", "textsl": "it", "texttt": "tt", "textsf": "sf", "fbox": "rm",
}

// spaces are the spacing commands, in mu (an 18th of the letters' size).
var spaces = map[string]float64{
	",": 3, "thinspace": 3, ":": 4, ">": 4, "medspace": 4, ";": 5, "thickspace": 5, "!": -3,
	"negthinspace": -3, "negmedspace": -4, "negthickspace": -5, "quad": 18, "qquad": 36, " ": 6,
	"enspace": 9, "enskip": 9,
}

// ignored are the commands that draw nothing, and take an argument (1)
// or none (0).
var ignored = map[string]int{
	"label": 1, "nonumber": 0, "notag": 0, "displaybreak": 0, "allowbreak": 0, "nolinebreak": 0,
	"hfill": 0, "qedhere": 0, "centering": 0, "noindent": 0, "vspace": 1, "vspace*": 1,
	"color": 1, "nobreak": 0, "relax": 0, "protect": 0,
}

func (p *parser) command(name string, st int) *atom {
	size := p.size(st)
	if r, ok := greek[name]; ok {
		return &atom{class: ord, b: glyph(letter(r, p.font), size), char: true, letter: true}
	}
	if s, ok := symbols[name]; ok {
		return &atom{class: s.class, b: glyph(letter(s.r, p.font), size), char: true}
	}
	if r, ok := bigOps[name]; ok {
		return p.bigOp(r, st)
	}
	if lim, ok := funcs[name]; ok {
		n := name
		if fn, ok := funcNames[name]; ok {
			n = fn
		}
		return &atom{class: op, b: text(n, size, "rm"), takes: lim}
	}
	if r, ok := accents[name]; ok {
		return p.accent(r, name, st)
	}
	if mu, ok := spaces[name]; ok {
		return &atom{glue: true, b: space(mu * size / 18)}
	}
	if f, ok := fontCmds[name]; ok {
		font := p.font
		p.font = f
		as := p.argAtoms(st)
		p.font = font
		cls := ord
		if len(as) == 1 {
			cls = as[0].class
		}
		return &atom{class: cls, b: p.hlist(as, st)}
	}
	if f, ok := textCmds[name]; ok {
		b := p.textArg(st, f)
		if name == "fbox" {
			b = p.framed(b, st)
		}
		return &atom{class: ord, b: b}
	}
	if n, ok := ignored[name]; ok {
		if n == 1 {
			p.rawGroup()
		}
		return nil
	}
	switch name {
	case "frac", "dfrac", "tfrac", "cfrac", "binom", "dbinom", "tbinom":
		fst := st
		switch name {
		case "dfrac", "cfrac", "dbinom":
			fst = styleD
		case "tfrac", "tbinom":
			fst = max(st, styleT)
		}
		num, den := p.arg(fracStyle(fst)), p.arg(fracStyle(fst))
		if strings.HasSuffix(name, "binom") {
			return &atom{class: inner, b: p.delimited('(', p.frac(num, den, fst, false), ')', fst)}
		}
		return &atom{class: inner, b: p.frac(num, den, fst, true)}
	case "sqrt":
		idx, _ := p.optArg(styleSS)
		return &atom{class: ord, b: p.sqrt(p.arg(st), idx, st)}
	case "left":
		return p.leftRight(st)
	case "right", "middle":
		p.delim()
		return nil
	case "big", "Big", "bigg", "Bigg", "bigl", "Bigl", "biggl", "Biggl", "bigr", "Bigr", "biggr", "Biggr", "bigm", "Bigm", "biggm", "Biggm":
		r := p.delim()
		h := map[string]float64{"big": 1.2, "Big": 1.8, "bigg": 2.4, "Bigg": 3}[strings.TrimRight(name, "lrm")]
		cls := map[byte]int{'l': open, 'r': clo, 'm': rel}[name[len(name)-1]] // ord, if none
		return &atom{class: cls, b: p.makeDelim(r, h*size, st)}
	case "operatorname", "operatorname*":
		return &atom{class: op, b: text(p.rawGroup(), size, "rm"), takes: name == "operatorname*"}
	case "mathop":
		return &atom{class: op, b: p.arg(st), takes: true}
	case "mathrel", "mathbin", "mathord", "mathopen", "mathclose", "mathpunct", "mathinner":
		cls := map[string]int{"mathrel": rel, "mathbin": bin, "mathord": ord, "mathopen": open, "mathclose": clo,
			"mathpunct": punct, "mathinner": inner}[name]
		return &atom{class: cls, b: p.arg(st)}
	case "not":
		return p.not(st)
	case "hspace", "hspace*", "kern", "mkern", "hskip", "mskip":
		return &atom{glue: true, b: space(p.dimen(st, name == "mkern" || name == "mskip"))}
	case "phantom", "hphantom", "vphantom", "smash":
		b := p.arg(st)
		switch name {
		case "phantom":
			return &atom{class: ord, b: &box{w: b.w, h: b.h, d: b.d}}
		case "hphantom":
			return &atom{class: ord, b: &box{w: b.w}}
		case "vphantom":
			return &atom{class: ord, b: &box{h: b.h, d: b.d}}
		}
		return &atom{class: ord, b: &box{w: b.w, kids: []kid{{b, 0, 0}}}}
	case "mathstrut", "strut":
		return &atom{class: ord, b: &box{h: 0.75 * size, d: 0.25 * size}}
	case "boxed":
		return &atom{class: ord, b: p.framed(p.arg(st), st)}
	case "cancel", "bcancel", "xcancel", "sout":
		return &atom{class: ord, b: p.cancel(p.arg(st), name, st)}
	case "textcolor":
		p.rawGroup()
		return &atom{class: ord, b: p.arg(st)}
	case "colorbox":
		p.rawGroup()
		return &atom{class: ord, b: p.textArg(st, "rm")}
	case "overline", "underline":
		return &atom{class: ord, b: p.rule2(p.arg(st), name == "overline", st)}
	case "widehat", "widetilde", "vec", "overrightarrow", "overleftarrow", "overleftrightarrow",
		"underrightarrow", "underleftarrow":
		as := p.argAtoms(st)
		return &atom{class: ord, b: p.wide(p.hlist(as, st), name, len(as) == 1 && as[0].letter, st)}
	case "overbrace", "underbrace":
		return &atom{class: op, b: p.brace(p.arg(st), name == "overbrace", st), takes: true, limits: 1}
	case "overset", "underset", "stackrel":
		over := p.arg(supStyle(st))
		as := p.argAtoms(st)
		cls := ord
		if len(as) == 1 {
			cls = as[0].class
		}
		if name == "stackrel" {
			cls = rel
		}
		return &atom{class: cls, b: p.stack(p.hlist(as, st), over, name != "underset", st)}
	case "xrightarrow", "xleftarrow", "xRightarrow", "xLeftarrow", "xleftrightarrow", "xmapsto", "xlongequal":
		below, _ := p.optArg(supStyle(st))
		return &atom{class: rel, b: p.xarrow(name, p.arg(supStyle(st)), below, st)}
	case "substack":
		p.skipSpace()
		if p.peek().kind == '{' {
			p.i++
		}
		rows, _ := p.cells(st, func(t tok) bool { return t.kind == '}' }, false)
		if p.peek().kind == '}' {
			p.i++
		}
		return &atom{class: ord, b: p.grid(rows, "c", func(int) float64 { return 0 }, 0.05, st, nil)}
	case "begin":
		return p.env(p.rawGroup(), st)
	case "end":
		p.rawGroup()
		return nil
	case "pmod", "pod":
		b := p.arg(st)
		lead := 1.0
		if st > styleT {
			lead = 1.0 / 3
		}
		parts := []*box{space(lead * size), text("(", size, "rm")}
		if name == "pmod" {
			parts = append(parts, text("mod", size, "rm"), space(size/3))
		}
		return &atom{class: ord, b: hbox(append(parts, b, text(")", size, "rm"))...)}
	case "bmod":
		return &atom{class: bin, b: text("mod", size, "rm")}
	case "mod":
		return &atom{class: ord, b: hbox(space(size), text("mod", size, "rm"), space(size/3))}
	case "tag", "tag*":
		b := p.textArg(st, "rm")
		if name == "tag" {
			b = hbox(text("(", size, "rm"), b, text(")", size, "rm"))
		}
		return &atom{glue: true, b: hbox(space(2*size), b)}
	case "rule":
		p.optArg(st)
		w, h := p.dimen(st, false), p.dimen(st, false)
		b := &box{w: w, h: h}
		b.paint = func(c *canvas, x, y float64) { c.rect(x, y-h, x+w, y) }
		return &atom{class: ord, b: b}
	case "TeX", "LaTeX":
		return &atom{class: ord, b: text(name, size, "rm")}
	case "sideset", "prescript":
		p.rawGroup()
		p.rawGroup()
		return nil
	}
	p.unknownCmd(name)
	return &atom{class: ord, b: text(`\`+name, size, "rm")}
}

// textArg is \text's argument: text, upright, spaces kept, and math
// in it between $ signs.
func (p *parser) textArg(st int, font string) *box {
	size := p.size(st)
	p.skipSpace()
	if p.peek().kind != '{' {
		return text(strings.TrimPrefix(tokString(p.next()), `\`), size, font)
	}
	p.i++
	var bs []*box
	var run strings.Builder
	flush := func() {
		if run.Len() > 0 {
			bs = append(bs, text(run.String(), size, font))
			run.Reset()
		}
	}
	depth := 1
	for p.i < len(p.ts) {
		t := p.next()
		switch t.kind {
		case '{':
			depth++
		case '}':
			if depth--; depth == 0 {
				flush()
				return hbox(bs...)
			}
		case ' ', 'n':
			run.WriteByte(' ')
		case 'x':
			if t.s == "$" {
				flush()
				as := p.list(styleT, func(t tok) bool { return t.kind == 'x' && t.s == "$" })
				if p.isX("$") {
					p.i++
				}
				bs = append(bs, p.hlist(as, max(st, styleT)))
				continue
			}
			run.WriteString(t.s)
		case 'c':
			if s, ok := symbols[t.s]; ok {
				run.WriteRune(s.r)
			} else if mu, ok := spaces[t.s]; ok {
				flush()
				bs = append(bs, space(mu*size/18))
			} else if f, ok := textCmds[t.s]; ok {
				flush()
				bs = append(bs, p.textArg(st, f))
			} else if r, ok := greek[t.s]; ok {
				run.WriteRune(r)
			} else if t.s != "/" && t.s != "-" {
				run.WriteString(t.s)
			}
		default:
			run.WriteByte(t.kind)
		}
	}
	flush()
	return hbox(bs...)
}

var dimenRE = regexp.MustCompile(`^\s*(-?[0-9]*\.?[0-9]+)\s*(em|ex|pt|mu|mm|cm|in|px|bp|pc)?`)

// dimen is a length: \hspace{1em}'s, \kern3pt's; mu says its unit is
// mu, if it's none.
func (p *parser) dimen(st int, mu bool) float64 {
	size := p.size(st)
	var s string
	p.skipSpace()
	if p.peek().kind == '{' {
		s = p.rawGroup()
	} else {
		var b strings.Builder
		for p.peek().kind == 'x' && b.Len() < 12 {
			b.WriteString(p.next().s)
			if m := dimenRE.FindStringSubmatch(b.String()); m != nil && m[2] != "" {
				break
			}
		}
		s = b.String()
	}
	m := dimenRE.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	v, _ := strconv.ParseFloat(m[1], 64)
	unit := map[string]float64{"em": 1, "ex": 0.45, "pt": 0.1, "mu": 1.0 / 18, "mm": 0.2845, "cm": 2.845,
		"in": 7.227, "px": 0.075, "bp": 0.1, "pc": 1.2}
	k, ok := unit[m[2]]
	if !ok {
		k = map[bool]float64{true: 1.0 / 18, false: 0.1}[mu]
	}
	return math.Max(-5*size, math.Min(20*size, v*k*size))
}

// not is \not and what it's on: the sign with a stroke through it, its
// own if it has one.
func (p *parser) not(st int) *atom {
	p.skipSpace()
	t := p.peek()
	var r rune
	switch {
	case t.kind == 'x':
		r = []rune(t.s)[0]
	case t.kind == 'c':
		r = symbols[t.s].r
	}
	if n, ok := nots[r]; ok {
		p.i++
		return &atom{class: rel, b: glyph(n, p.size(st)), char: true}
	}
	a := p.atom(st)
	if a == nil {
		return nil
	}
	b := a.b
	s := glyph('/', p.size(st))
	out := &box{}
	out.put(b, 0, 0)
	out.put(s, (b.w-s.w)/2, 0)
	out.w = b.w
	return &atom{class: rel, b: out}
}
