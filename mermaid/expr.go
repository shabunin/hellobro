package mermaid

// Formulas of x, as a plot's functions are written: numbers, x, pi, e;
// + - * / ^ (and **), brackets, and the functions of a calculator.
// Written as people write them too: 2x, 3(x+1), (x+1)(x-1), sin x,
// x², −, ×, ·.

import (
	"errors"
	"math"
	"strconv"
	"strings"
	"unicode"
)

// fn is a formula ready to work out, for x.
type fn func(x float64) float64

var funcs1 = map[string]func(float64) float64{
	"sin": math.Sin, "cos": math.Cos, "tan": math.Tan, "asin": math.Asin, "acos": math.Acos, "atan": math.Atan,
	"arcsin": math.Asin, "arccos": math.Acos, "arctan": math.Atan, "sinh": math.Sinh, "cosh": math.Cosh, "tanh": math.Tanh,
	"sqrt": math.Sqrt, "cbrt": math.Cbrt, "exp": math.Exp, "ln": math.Log, "log": math.Log, "log10": math.Log10,
	"log2": math.Log2, "lg": math.Log10, "abs": math.Abs, "floor": math.Floor, "ceil": math.Ceil, "round": math.Round,
	"sign": func(x float64) float64 {
		switch {
		case x > 0:
			return 1
		case x < 0:
			return -1
		}
		return x
	},
	"sec": func(x float64) float64 { return 1 / math.Cos(x) }, "csc": func(x float64) float64 { return 1 / math.Sin(x) },
	"cot": func(x float64) float64 { return 1 / math.Tan(x) }, "tg": math.Tan, "ctg": func(x float64) float64 { return 1 / math.Tan(x) },
}

var funcs2 = map[string]func(a, b float64) float64{
	"min": math.Min, "max": math.Max, "pow": math.Pow, "atan2": math.Atan2, "mod": math.Mod,
	"log": func(a, b float64) float64 { return math.Log(b) / math.Log(a) }, // log(base, x)
}

var consts = map[string]float64{"pi": math.Pi, "π": math.Pi, "e": math.E, "tau": 2 * math.Pi, "τ": 2 * math.Pi}

type token struct {
	kind byte // 'n' number, 'i' a name, 'o' an operator or a bracket
	num  float64
	s    string
}

var errFormula = errors.New("mermaid: not a formula")

// lexFormula cuts s into its tokens.
func lexFormula(s string) ([]token, error) {
	r := strings.NewReplacer("Math.", "", "math.", "", "−", "-", "×", "*", "·", "*", "⋅", "*", "÷", "/", "**", "^",
		"²", "^2", "³", "^3", "\\pi", "pi", "\\", "", "{", "(", "}", ")", "[", "(", "]", ")")
	s = r.Replace(s)
	var ts []token
	rs := []rune(s)
	for i := 0; i < len(rs); {
		c := rs[i]
		switch {
		case unicode.IsSpace(c):
			i++
		case unicode.IsDigit(c) || c == '.':
			j := i
			for j < len(rs) && (unicode.IsDigit(rs[j]) || rs[j] == '.') {
				j++
			}
			if j < len(rs) && (rs[j] == 'e' || rs[j] == 'E') && j+1 < len(rs) && (unicode.IsDigit(rs[j+1]) || (rs[j+1] == '-' || rs[j+1] == '+') && j+2 < len(rs) && unicode.IsDigit(rs[j+2])) {
				j += 2
				for j < len(rs) && unicode.IsDigit(rs[j]) {
					j++
				}
			}
			v, err := strconv.ParseFloat(string(rs[i:j]), 64)
			if err != nil {
				return nil, errFormula
			}
			ts = append(ts, token{kind: 'n', num: v})
			i = j
		case unicode.IsLetter(c):
			j := i
			for j < len(rs) && (unicode.IsLetter(rs[j]) || unicode.IsDigit(rs[j]) || rs[j] == '_') {
				j++
			}
			ts = append(ts, splitName(string(rs[i:j]))...)
			i = j
		case strings.ContainsRune("+-*/^(),", c):
			ts = append(ts, token{kind: 'o', s: string(c)})
			i++
		default:
			return nil, errFormula
		}
	}
	return ts, nil
}

// splitName reads a run of letters: a function's or a constant's name,
// x, or several of them written together (xsin: x sin; 2pix: pi x).
func splitName(s string) []token {
	if _, ok := funcs1[s]; ok {
		return []token{{kind: 'i', s: s}}
	}
	if _, ok := funcs2[s]; ok {
		return []token{{kind: 'i', s: s}}
	}
	if _, ok := consts[s]; ok || s == "x" {
		return []token{{kind: 'i', s: s}}
	}
	for n := len(s) - 1; n > 0; n-- { // the longest known name first
		head := s[:n]
		_, f1 := funcs1[head]
		_, f2 := funcs2[head]
		_, c := consts[head]
		if f1 || f2 || c || head == "x" {
			return append([]token{{kind: 'i', s: head}}, splitName(s[n:])...)
		}
	}
	return []token{{kind: 'i', s: s}} // unknown: an error when it's read
}

type exprParser struct {
	ts []token
	i  int
}

func (p *exprParser) peek() token {
	if p.i < len(p.ts) {
		return p.ts[p.i]
	}
	return token{}
}

func (p *exprParser) is(op string) bool { t := p.peek(); return t.kind == 'o' && t.s == op }

// parseFormula reads s as a formula of x.
func parseFormula(s string) (fn, error) {
	ts, err := lexFormula(s)
	if err != nil {
		return nil, err
	}
	p := &exprParser{ts: ts}
	f, err := p.sum()
	if err != nil {
		return nil, err
	}
	if p.i != len(ts) {
		return nil, errFormula
	}
	return f, nil
}

func (p *exprParser) sum() (fn, error) {
	f, err := p.product()
	if err != nil {
		return nil, err
	}
	for p.is("+") || p.is("-") {
		op := p.peek().s
		p.i++
		g, err := p.product()
		if err != nil {
			return nil, err
		}
		a, b := f, g
		if op == "+" {
			f = func(x float64) float64 { return a(x) + b(x) }
		} else {
			f = func(x float64) float64 { return a(x) - b(x) }
		}
	}
	return f, nil
}

// startsFactor is whether a factor starts at t: a factor next to
// another is a product (2x, (x+1)(x-1)).
func startsFactor(t token) bool {
	return t.kind == 'n' || t.kind == 'i' || t.kind == 'o' && t.s == "("
}

func (p *exprParser) product() (fn, error) {
	f, err := p.unary()
	if err != nil {
		return nil, err
	}
	for {
		op := ""
		var g fn
		var err error
		switch t := p.peek(); {
		case p.is("*") || p.is("/"):
			op = t.s
			p.i++
			g, err = p.unary()
		case startsFactor(t):
			op = "*"
			g, err = p.power()
		default:
			return f, nil
		}
		if err != nil {
			return nil, err
		}
		a, b := f, g
		if op == "*" {
			f = func(x float64) float64 { return a(x) * b(x) }
		} else {
			f = func(x float64) float64 { return a(x) / b(x) }
		}
	}
}

func (p *exprParser) unary() (fn, error) {
	if p.is("-") {
		p.i++
		f, err := p.unary()
		if err != nil {
			return nil, err
		}
		return func(x float64) float64 { return -f(x) }, nil
	}
	if p.is("+") {
		p.i++
		return p.unary()
	}
	return p.power()
}

// power is a factor, raised: right to left (2^3^2 is 2^9), and -x^2 is
// -(x^2), as the unary minus is read before it.
func (p *exprParser) power() (fn, error) {
	f, err := p.factor()
	if err != nil {
		return nil, err
	}
	if p.is("^") {
		p.i++
		g, err := p.unary()
		if err != nil {
			return nil, err
		}
		a, b := f, g
		return func(x float64) float64 { return math.Pow(a(x), b(x)) }, nil
	}
	return f, nil
}

func (p *exprParser) factor() (fn, error) {
	t := p.peek()
	switch {
	case t.kind == 'n':
		p.i++
		v := t.num
		return func(float64) float64 { return v }, nil
	case t.kind == 'o' && t.s == "(":
		p.i++
		f, err := p.sum()
		if err != nil || !p.is(")") {
			return nil, errFormula
		}
		p.i++
		return f, nil
	case t.kind == 'i' && t.s == "x":
		p.i++
		return func(x float64) float64 { return x }, nil
	case t.kind == 'i':
		p.i++
		if v, ok := consts[t.s]; ok {
			return func(float64) float64 { return v }, nil
		}
		f1, ok1 := funcs1[t.s]
		f2, ok2 := funcs2[t.s]
		if !ok1 && !ok2 {
			return nil, errFormula
		}
		if p.is("(") { // its arguments in brackets
			p.i++
			a, err := p.sum()
			if err != nil {
				return nil, err
			}
			if p.is(",") && ok2 {
				p.i++
				b, err := p.sum()
				if err != nil || !p.is(")") {
					return nil, errFormula
				}
				p.i++
				return func(x float64) float64 { return f2(a(x), b(x)) }, nil
			}
			if !p.is(")") || !ok1 {
				return nil, errFormula
			}
			p.i++
			return func(x float64) float64 { return f1(a(x)) }, nil
		}
		if !ok1 {
			return nil, errFormula
		}
		a, err := p.power() // sin x, sin 2x, sin 2πx: no brackets, its numbers and x
		if err != nil {
			return nil, err
		}
		for t := p.peek(); t.kind == 'n' || t.kind == 'i' && (t.s == "x" || consts[t.s] != 0); t = p.peek() {
			b, err := p.power()
			if err != nil {
				return nil, err
			}
			c := a
			a = func(x float64) float64 { return c(x) * b(x) }
		}
		return func(x float64) float64 { return f1(a(x)) }, nil
	}
	return nil, errFormula
}
