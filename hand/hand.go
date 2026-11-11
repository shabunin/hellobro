// Package hand writes text as a hand would, a single stroke a line, for a
// notebook's ink: in the Hershey fonts' script (fonts/README), Cyrillic
// in theirs (a print hand: there's no Cyrillic script among them).
package hand

import (
	"bufio"
	"bytes"
	"embed"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

//go:embed fonts/*.jhf
var fontFS embed.FS

// Pt is a point, in the units the text was written in, y down.
type Pt struct{ X, Y float64 }

// Line is a stroke: the pen down along it.
type Line []Pt

// glyph is a letter: its strokes in Hershey's units (y down, the
// baseline at 9), and where it starts and ends across.
type glyph struct {
	left, right int
	strokes     [][]Pt
}

// font is a .jhf file's letters, by the runes they're for.
type font map[rune]glyph

// parseJHF reads a .jhf font, its glyphs for runes, in order (a font of
// 96 for ASCII's printable 32 to 127).
func parseJHF(data []byte, runes []rune) (font, error) {
	f := font{}
	sc := bufio.NewScanner(bytes.NewReader(data))
	i := 0
	for sc.Scan() {
		l := sc.Text()
		if strings.TrimSpace(l) == "" {
			continue
		}
		if len(l) < 10 {
			return nil, fmt.Errorf("glyph %d: short", i)
		}
		n, err := strconv.Atoi(strings.TrimSpace(l[5:8]))
		if err != nil {
			return nil, fmt.Errorf("glyph %d: %v", i, err)
		}
		s := l[8:]
		for len(s) < 2*n && sc.Scan() { // a long one goes on on the next lines
			s += sc.Text()
		}
		if len(s) < 2*n {
			return nil, fmt.Errorf("glyph %d: %d of %d points", i, len(s)/2, n)
		}
		g := glyph{left: int(s[0]) - 'R', right: int(s[1]) - 'R'}
		var st []Pt
		for k := 1; k < n; k++ {
			a, b := s[2*k], s[2*k+1]
			if a == ' ' && b == 'R' { // the pen up
				if len(st) > 0 {
					g.strokes = append(g.strokes, st)
				}
				st = nil
				continue
			}
			st = append(st, Pt{float64(int(a) - 'R'), float64(int(b) - 'R')})
		}
		if len(st) > 0 {
			g.strokes = append(g.strokes, st)
		}
		if i < len(runes) {
			f[runes[i]] = g
		}
		i++
	}
	return f, sc.Err()
}

// ascii is the runes of a font of ASCII's printable: 32 to 127.
func ascii() []rune {
	rs := make([]rune, 96)
	for i := range rs {
		rs[i] = rune(32 + i)
	}
	return rs
}

// cyrillicRunes is cyrillic.jhf's letters where ASCII has its own: its
// digits and signs are ASCII's, the letters on the letters' and a few
// signs' places (Й is И there, Ё is Е).
func cyrillicRunes() []rune {
	rs := ascii()
	for i := range rs {
		if c := rs[i]; c >= 'A' && c <= 'Z' || c >= 'a' && c <= '~' || c >= '$' && c <= '&' || c == '[' || c == ']' || c == '^' || c == '_' || c == '`' {
			rs[i] = 0
		}
	}
	set := func(from rune, s string) {
		for i, r := range []rune(s) {
			rs[int(from)-32+i] = r
		}
	}
	set('$', "Ыцы")
	set('A', "АБЭДИФГЖИЧКЛМНОПШРСТЮВЩХУЗЕ")
	set(']', "ЪЯЬЦ")
	set('a', "абэдйфгжичклмнопшрстювщхузеъяь")
	return rs
}

// Hand is the fonts a text is written in: a letter in the first that
// has it.
type Hand struct {
	fonts []font
	k     float64 // a capital's height, in the fonts' units
}

var (
	scriptOnce sync.Once
	script     *Hand
	scriptErr  error
)

// Script is the script hand, Cyrillic after it.
func Script() (*Hand, error) {
	scriptOnce.Do(func() {
		load := func(name string, runes []rune) font {
			if scriptErr != nil {
				return nil
			}
			data, err := fontFS.ReadFile("fonts/" + name)
			if err != nil {
				scriptErr = err
				return nil
			}
			f, err := parseJHF(data, runes)
			if err != nil {
				scriptErr = fmt.Errorf("%s: %v", name, err)
			}
			return f
		}
		lat, cyr := load("scripts.jhf", ascii()), load("cyrillic.jhf", cyrillicRunes())
		delete(cyr, ' ')
		script = &Hand{fonts: []font{lat, cyr}, k: 21}
	})
	return script, scriptErr
}

// glyph is r's, or what stands for it: a letter without its accent, a
// sign ASCII has; ok false if none.
func (h *Hand) glyph(r rune) (glyph, bool) {
	for _, f := range h.fonts {
		if g, ok := f[r]; ok {
			return g, true
		}
	}
	if s, ok := plain[r]; ok && s != r {
		return h.glyph(s)
	}
	if d := []rune(norm.NFD.String(string(r))); len(d) > 1 && d[0] != r { // é: e
		return h.glyph(d[0])
	}
	return glyph{}, false
}

// plain is what a hand writes for signs its fonts don't have.
var plain = map[rune]rune{
	'‘': '\'', '’': '\'', '‚': ',', '“': '"', '”': '"', '„': '"', '«': '"', '»': '"',
	'–': '-', '—': '-', '−': '-', '‐': '-', '•': '*', '·': '.', '×': 'x', '÷': '/',
	' ': ' ', 'Ё': 'Е', 'ё': 'е', 'Й': 'И', 'ʼ': '\'',
}

// Word is a word written: its strokes, in the order they're written, its
// box, and the baseline of its line (the words of a line share it).
type Word struct {
	Strokes []Line
	Min     Pt
	Max     Pt
	Base    float64
}

// Writing is a text written: its words, in order, and how far down it
// goes (from its top, 0).
type Writing struct {
	Words []Word
	H     float64
}

// LineHeight is how far a line is under the one before, for letters
// size high.
func LineHeight(size float64) float64 { return size * 1.7 }

// Write writes text in lines at most width wide, a capital size high,
// from the top left (0, 0): a line break a new line, a blank line a
// paragraph's space. Each word is a little off, as a hand's would be:
// up or down, bigger or smaller, slanted, as seed has it.
func (h *Hand) Write(text string, size, width float64, seed int64) Writing {
	k := size / h.k
	rnd := rand.New(rand.NewSource(seed))
	lineH := LineHeight(size)
	space := h.advance(' ') * k
	width /= 1.05 // room for a word a little bigger, or slanted
	var out Writing
	y := size // the first line's baseline
	x := 0.0
	for para := range strings.SplitSeq(strings.ReplaceAll(text, "\r", ""), "\n") {
		if strings.TrimSpace(para) == "" {
			y += lineH / 2
			continue
		}
		indent := float64(len(para)-len(strings.TrimLeft(para, " "))) * space
		x = indent
		for word := range strings.FieldsSeq(para) {
			for _, part := range h.split(word, (width-indent)/k) {
				w := h.width(part) * k
				if x > indent && x+w > width {
					x, y = indent, y+lineH
				}
				wd := h.word(part, k, x, y, rnd)
				if len(wd.Strokes) > 0 {
					out.Words = append(out.Words, wd)
				}
				x += w + space
			}
		}
		y += lineH
	}
	out.H = y - lineH + size*0.45 // the last line's descenders
	return out
}

// advance is how far r goes across, in the fonts' units.
func (h *Hand) advance(r rune) float64 {
	g, ok := h.glyph(r)
	if !ok {
		g, _ = h.glyph('?')
	}
	return float64(g.right - g.left)
}

func (h *Hand) width(s string) float64 {
	w := 0.0
	for _, l := range h.letters(s) {
		w = max(w, l.x+float64(l.g.right-l.g.left)*l.k)
	}
	return w
}

// letter is a glyph placed in a word: x across it, k its size (1, or a
// superscript's), up how far over the baseline, in the fonts' units.
type letter struct {
	g    glyph
	x, k float64
	up   float64
}

// supK is a superscript's size, supUp how far up it is (in a capital's).
const supK, supUp = 0.62, 0.48

// letters places s's letters: what follows ^ is a superscript, x^2,
// x^-1, e^(i pi), 2^{10} (its braces left out).
func (h *Hand) letters(s string) []letter {
	var out []letter
	rs := []rune(s)
	x := 0.0
	put := func(r rune, k, up float64) {
		g, ok := h.glyph(r)
		if !ok && !unicode.IsSpace(r) {
			g, _ = h.glyph('?')
		}
		out = append(out, letter{g: g, x: x, k: k, up: up})
		x += float64(g.right-g.left) * k
	}
	for i := 0; i < len(rs); i++ {
		if rs[i] != '^' || i+1 == len(rs) {
			put(rs[i], 1, 0)
			continue
		}
		j, end := i+1, i+1 // the superscript: rs[j:end]
		switch rs[j] {
		case '{', '(':
			closing := map[rune]rune{'{': '}', '(': ')'}[rs[j]]
			for end = j + 1; end < len(rs) && rs[end] != closing; end++ {
			}
			if rs[j] == '{' {
				j++
			} else if end < len(rs) {
				end++ // the brackets kept
			}
		default:
			if rs[end] == '-' || rs[end] == '+' {
				end++
			}
			for end < len(rs) && (unicode.IsLetter(rs[end]) || unicode.IsDigit(rs[end]) || rs[end] == '.' && end+1 < len(rs) && unicode.IsDigit(rs[end+1])) {
				end++
			}
		}
		for _, r := range rs[j:min(end, len(rs))] {
			put(r, supK, supUp*h.k)
		}
		i = end
		if i < len(rs) && rs[i] == '}' {
			continue // its closing brace: the loop steps past it
		}
		i--
	}
	return out
}

// split cuts a word too long for a line (max, in the fonts' units) in
// parts that fit.
func (h *Hand) split(word string, max float64) []string {
	if h.width(word) <= max || max <= 0 {
		return []string{word}
	}
	var parts []string
	var cur []rune
	w := 0.0
	for _, r := range word {
		a := h.advance(r)
		if w+a > max && len(cur) > 0 {
			parts, cur, w = append(parts, string(cur)), nil, 0
		}
		cur, w = append(cur, r), w+a
	}
	return append(parts, string(cur))
}

// word writes s with its baseline's start at x, y, k page units a font
// unit, a little off as rnd has it.
func (h *Hand) word(s string, k, x, y float64, rnd *rand.Rand) Word {
	size := h.k * k
	scale := 1 + (rnd.Float64()-0.5)*0.06
	lift := (rnd.Float64() - 0.5) * 0.08 * size
	slant := (rnd.Float64() - 0.5) * 0.08
	tilt := (rnd.Float64() - 0.5) * 0.02 // radians: the line it's on, a touch off level
	sin, cos := math.Sincos(tilt)
	wd := Word{Min: Pt{math.Inf(1), math.Inf(1)}, Max: Pt{math.Inf(-1), math.Inf(-1)}, Base: y}
	for _, lt := range h.letters(s) {
		g := lt.g
		for _, st := range g.strokes {
			l := make(Line, len(st))
			for i, p := range st {
				px, py := (lt.x+(p.X-float64(g.left))*lt.k)*k*scale, ((p.Y-9)*lt.k-lt.up)*k*scale // from the word's start, on its baseline
				px -= py * slant
				l[i] = Pt{x + px*cos - py*sin, y + lift + px*sin + py*cos}
				wd.Min = Pt{math.Min(wd.Min.X, l[i].X), math.Min(wd.Min.Y, l[i].Y)}
				wd.Max = Pt{math.Max(wd.Max.X, l[i].X), math.Max(wd.Max.Y, l[i].Y)}
			}
			wd.Strokes = append(wd.Strokes, l)
		}
	}
	return wd
}
