package widget

import (
	"image"
	"slices"
	"strings"
	"unicode"

	"golang.org/x/image/font"

	"hellobro/ink"
	"hellobro/ink/layout"
	"hellobro/ink/text"
)

// Para is a paragraph's kind, as its first letters say, markdown's way.
type Para uint8

const (
	Plain  Para = iota
	Title       // "# "
	Head        // "## "
	Bullet      // "- "
	Todo        // "- [ ] "
	Done        // "- [x] "
	Quote       // "> "
	nParas
)

// Mark is what a paragraph of kind p starts with.
func (p Para) Mark() string { return paraMarks[p] }

var paraMarks = [nParas]string{"", "# ", "## ", "- ", "- [ ] ", "- [x] ", "> "}

// paraOf is the kind rs's paragraph is, and how long its mark.
func paraOf(rs []rune) (Para, int) {
	for _, p := range []Para{Todo, Done, Head, Title, Bullet, Quote} {
		m := paraMarks[p]
		if len(rs) >= len(m) && string(rs[:len(m)]) == m {
			return p, len(m)
		}
	}
	return Plain, 0
}

// TextArea is text of many paragraphs to edit, kept as markdown's
// simplest (a title, headings, bullets, to-dos, quotes, each by its
// first letters) and shown as they'd print: the marks hidden, a title
// big, a bullet's dot, a to-do's box to tap. In focus it takes the keys:
// the arrows, Home End (Ctrl: the whole text's), PageUp PageDown, Shift
// with them selects; Back, Delete, Enter (a bullet goes on to the next
// line; on an empty one, it ends the list); Ctrl+A, Ctrl+X C V, Ctrl+Z;
// Esc lets go of the keys. Back at a paragraph's start takes its kind off.
type TextArea struct {
	Scroll layout.Scroll     // what shows of it, under the finger
	Kinds  [nParas]Clickable // a toolbar's: each makes the paragraph its kind (again: plain)

	rs            []rune
	caret, anchor int // the selection, anchor to caret; the same: none
	wantX         int // where ↑ ↓ aim, across
	keepX         bool
	focused       bool
	changed       bool
	undo          []snap
	lastKind      int
	reveal        bool // the keys moved the caret: the program's scroll may follow it

	down     bool
	downAt   image.Point
	moved    bool
	tapPos   image.Point
	tapped   bool
	lines    []vline
	paras    []span
	caretR   image.Rectangle
	viewH    int
	boxes    []box
	width    int
	lastLook AreaLook
}

type snap struct {
	rs     []rune
	caret  int
	anchor int
}

// A paragraph as laid out: its runes (the mark from start to cs), where.
type span struct {
	start, cs, end int
	kind           Para
}

// A line as shown: runes start to end of paragraph p.
type vline struct {
	start, end int
	p          int
	x, y, h    int
	base       int
	face       font.Face
}

// A to-do's box, to tap.
type box struct {
	r image.Rectangle
	p int
}

// clipboard is what Ctrl+C or X took, for Ctrl+V.
var clipboard []rune

// AreaLook is how a TextArea shows: a face for each kind of paragraph,
// its grays, a list's indent, the room between paragraphs.
type AreaLook struct {
	Faces       [nParas]font.Face
	Fg, Quiet   uint8
	Indent, Gap int
	Stroke      int // a line's width: the caret, a box's frame
}

// Text is the text, marks and all.
func (t *TextArea) Text() string { return string(t.rs) }

// SetText replaces it, the caret at the end.
func (t *TextArea) SetText(s string) {
	t.rs = []rune(s)
	t.caret, t.anchor = len(t.rs), len(t.rs)
	t.undo = nil
}

// SetCaret puts the caret at i (in runes), nothing selected.
func (t *TextArea) SetCaret(i int) {
	t.caret = min(max(i, 0), len(t.rs))
	t.anchor = t.caret
}

// Focused says it has the keys.
func (t *TextArea) Focused() bool { return t.focused }

// Changed says the text changed since it was last asked.
func (t *TextArea) Changed(gtx ink.Ctx) bool {
	t.events(gtx)
	c := t.changed
	t.changed = false
	return c
}

// Selection is the selection, start to end.
func (t *TextArea) Selection() (int, int) { return min(t.caret, t.anchor), max(t.caret, t.anchor) }

// Caret is where it is, in runes.
func (t *TextArea) Caret() int { return t.caret }

// CaretRect is where the caret was drawn, in the area's coordinates; and
// whether the keys moved it since this was last asked (a scroll keeps it
// in sight then).
func (t *TextArea) CaretRect() (image.Rectangle, bool) {
	r := t.reveal
	t.reveal = false
	return t.caretR, r
}

// Kind is the kind of the paragraph the caret's in.
func (t *TextArea) Kind() Para {
	p, _ := paraOf(t.rs[t.paraStart(t.caret):])
	return p
}

// SetKind makes the paragraphs the selection's in (the caret's) kind p.
func (t *TextArea) SetKind(p Para) {
	t.save(0)
	a, b := t.Selection()
	starts := []int{}
	for i := t.paraStart(a); ; {
		starts = append(starts, i)
		e := t.paraEnd(i)
		if e >= b || e >= len(t.rs) {
			break
		}
		i = e + 1
	}
	for k := len(starts) - 1; k >= 0; k-- {
		t.setMark(starts[k], p)
	}
	t.changed = true
}

// setMark sets the mark of the paragraph starting at s, the caret and
// anchor kept in their places.
func (t *TextArea) setMark(s int, p Para) {
	_, m := paraOf(t.rs[s:])
	nm := []rune(paraMarks[p])
	t.rs = slices.Concat(t.rs[:s], nm, t.rs[s+m:])
	d := len(nm) - m
	fix := func(i int) int {
		if i >= s+m {
			return i + d
		}
		if i > s {
			return s + len(nm)
		}
		return i
	}
	t.caret, t.anchor = fix(t.caret), fix(t.anchor)
}

func (t *TextArea) paraStart(i int) int {
	for i > 0 && t.rs[i-1] != '\n' {
		i--
	}
	return i
}

func (t *TextArea) paraEnd(i int) int {
	for i < len(t.rs) && t.rs[i] != '\n' {
		i++
	}
	return i
}

// contentStart is where paragraph-at-i's words start, past its mark.
func (t *TextArea) contentStart(i int) int {
	s := t.paraStart(i)
	_, m := paraOf(t.rs[s:])
	return s + m
}

// clamp keeps i out of a mark.
func (t *TextArea) clamp(i int) int {
	i = min(max(i, 0), len(t.rs))
	return max(i, t.contentStart(i))
}

const (
	opNone = iota
	opType
	opDelete
)

// save keeps the text for Ctrl+Z, before an edit of kind k: typing in a
// row is one undo.
func (t *TextArea) save(k int) {
	if k != 0 && k == t.lastKind && len(t.undo) > 0 {
		return
	}
	t.lastKind = k
	t.undo = append(t.undo, snap{slices.Clone(t.rs), t.caret, t.anchor})
	if len(t.undo) > 100 {
		t.undo = t.undo[1:]
	}
}

func (t *TextArea) deleteSel() bool {
	a, b := t.Selection()
	if a == b {
		return false
	}
	t.rs = slices.Delete(t.rs, a, b)
	t.caret, t.anchor = t.clamp(a), t.clamp(a)
	t.changed = true
	return true
}

func (t *TextArea) insert(s []rune) {
	t.deleteSel()
	t.rs = slices.Insert(t.rs, t.caret, s...)
	t.caret += len(s)
	t.anchor = t.caret
	t.changed = true
}

// enter breaks the paragraph at the caret: a list goes on, an empty
// item ends it, a title or heading is followed by plain text.
func (t *TextArea) enter() {
	t.deleteSel()
	s := t.paraStart(t.caret)
	p, m := paraOf(t.rs[s:])
	switch p {
	case Bullet, Todo, Done, Quote:
		if t.paraEnd(s) == s+m { // empty: no more of it
			t.setMark(s, Plain)
			t.changed = true
			return
		}
		next := p
		if p == Done {
			next = Todo
		}
		t.insert(append([]rune{'\n'}, []rune(paraMarks[next])...))
	default:
		t.insert([]rune{'\n'})
	}
}

func (t *TextArea) back() {
	if t.deleteSel() {
		return
	}
	c := t.caret
	s := t.paraStart(c)
	_, m := paraOf(t.rs[s:])
	switch {
	case c == s+m && m > 0: // at the words' start: the kind goes
		t.setMark(s, Plain)
	case c == s && s > 0: // joins the paragraph before
		t.rs = slices.Delete(t.rs, s-1, s)
		t.caret = s - 1
	case c > 0:
		t.rs = slices.Delete(t.rs, c-1, c)
		t.caret = c - 1
	default:
		return
	}
	t.anchor = t.caret
	t.changed = true
}

func (t *TextArea) del() {
	if t.deleteSel() {
		return
	}
	c := t.caret
	if c >= len(t.rs) {
		return
	}
	if t.rs[c] == '\n' { // joins the next, its mark gone
		_, m := paraOf(t.rs[c+1:])
		t.rs = slices.Delete(t.rs, c, c+1+m)
	} else {
		t.rs = slices.Delete(t.rs, c, c+1)
	}
	t.changed = true
}

// move puts the caret at i, the selection going with it if shift.
func (t *TextArea) move(i int, shift bool) {
	t.lastKind = opNone // typing after a move is a new undo
	t.caret = t.clamp(i)
	if !shift {
		t.anchor = t.caret
	}
	t.reveal = true
}

func (t *TextArea) left() int {
	c := t.caret
	if c == t.contentStart(c) {
		if s := t.paraStart(c); s > 0 {
			return s - 1 // the end of the paragraph before
		}
		return c
	}
	return c - 1
}

func (t *TextArea) right() int {
	c := t.caret
	if c < len(t.rs) && t.rs[c] == '\n' {
		return t.contentStart(c + 1)
	}
	return min(c+1, len(t.rs))
}

// word goes a word on (or back): past the spaces, then the letters.
func (t *TextArea) word(dir int) int {
	c := t.caret
	in := func(i int) bool { return i >= 0 && i < len(t.rs) }
	if dir < 0 {
		c--
		for in(c) && !isWord(t.rs[c]) {
			c--
		}
		for in(c-1) && isWord(t.rs[c-1]) {
			c--
		}
		return max(c, 0)
	}
	for in(c) && !isWord(t.rs[c]) {
		c++
	}
	for in(c) && isWord(t.rs[c]) {
		c++
	}
	return c
}

func isWord(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' }

// lineOf is the line the caret's on, as last laid out.
func (t *TextArea) lineOf(c int) int {
	for k, l := range t.lines {
		if c >= l.start && (c < l.end || c == l.end && (k == len(t.lines)-1 || t.lines[k+1].start > c || t.lines[k+1].p != l.p)) {
			return k
		}
	}
	return max(len(t.lines)-1, 0)
}

// at is the rune nearest x on line k.
func (t *TextArea) at(k, x int) int {
	l := t.lines[k]
	best, bestD := l.start, 1<<30
	for i := l.start; i <= l.end; i++ {
		d := abs(l.x + text.Width(l.face, string(t.rs[l.start:i])) - x)
		if d < bestD {
			best, bestD = i, d
		}
	}
	return best
}

func (t *TextArea) caretX() int {
	if len(t.lines) == 0 {
		return 0
	}
	l := t.lines[t.lineOf(t.caret)]
	return l.x + text.Width(l.face, string(t.rs[l.start:min(max(t.caret, l.start), l.end)]))
}

// vertical moves the caret lines lines down (up if negative).
func (t *TextArea) vertical(lines int, shift bool) {
	if len(t.lines) == 0 {
		return
	}
	if !t.keepX {
		t.wantX = t.caretX()
	}
	k := min(max(t.lineOf(t.caret)+lines, 0), len(t.lines)-1)
	t.move(t.at(k, t.wantX), shift)
	t.keepX = true
}

// Update takes the events since the last frame: the keys, the taps,
// the toolbar's.
func (t *TextArea) Update(gtx ink.Ctx) { t.events(gtx) }

func (t *TextArea) events(gtx ink.Ctx) {
	t.Scroll.Axis = layout.Vertical
	for p := range t.Kinds {
		t.Kinds[p].KeepFocus = true
		for t.Kinds[p].Clicked(gtx) {
			k := Para(p)
			if cur := t.Kind(); cur == k || cur == Done && k == Todo {
				k = Plain
			}
			t.SetKind(k)
			t.reveal = true
		}
	}
	for _, ev := range gtx.Events(t) {
		switch ev := ev.(type) {
		case ink.Focus:
			t.focused = ev.On
		case ink.Pointer:
			switch ev.Kind {
			case ink.Press:
				t.down, t.downAt, t.moved = true, ev.Pos, false
			case ink.Move:
				if d := ev.Pos.Sub(t.downAt); d.X*d.X+d.Y*d.Y > 400 {
					t.moved = true
				}
			case ink.Release:
				if t.down && !t.moved {
					t.tapped, t.tapPos = true, ev.Pos
				}
				t.down = false
			case ink.Cancel:
				t.down = false
			}
		case ink.Text:
			t.save(opType)
			t.insert([]rune(ev.Text))
			t.keepX, t.reveal = false, true
		case ink.Key:
			t.key(gtx, ev)
		}
	}
}

func (t *TextArea) key(gtx ink.Ctx, k ink.Key) {
	shift, ctrl := k.Mods&ink.Shift != 0, k.Mods&ink.Ctrl != 0
	keep := false
	switch k.Name {
	case "Left":
		if ctrl {
			t.move(t.word(-1), shift)
		} else if a, b := t.Selection(); a != b && !shift {
			t.move(a, false)
		} else {
			t.move(t.left(), shift)
		}
	case "Right":
		if ctrl {
			t.move(t.word(1), shift)
		} else if a, b := t.Selection(); a != b && !shift {
			t.move(b, false)
		} else {
			t.move(t.right(), shift)
		}
	case "Up":
		t.vertical(-1, shift)
		keep = true
	case "Down":
		t.vertical(1, shift)
		keep = true
	case "PageUp", "PageDown":
		n := max(t.viewH/max(t.lineH(), 1)-1, 1)
		if k.Name == "PageUp" {
			n = -n
		}
		t.vertical(n, shift)
		keep = true
	case "Home":
		if ctrl {
			t.move(0, shift)
		} else if len(t.lines) > 0 {
			t.move(t.lines[t.lineOf(t.caret)].start, shift)
		}
	case "End":
		if ctrl {
			t.move(len(t.rs), shift)
		} else if len(t.lines) > 0 {
			t.move(t.lines[t.lineOf(t.caret)].end, shift)
		}
	case "Back":
		t.save(opDelete)
		t.back()
		t.reveal = true
	case "Delete":
		t.save(opDelete)
		t.del()
		t.reveal = true
	case "Enter":
		t.save(0)
		t.enter()
		t.reveal = true
	case "Space":
		t.save(opType)
		t.insert([]rune{' '})
		t.reveal = true
	case "Esc":
		gtx.Focus(nil)
	case "a":
		if ctrl {
			t.anchor, t.caret = 0, len(t.rs)
		}
	case "c", "x":
		if a, b := t.Selection(); ctrl && a != b {
			clipboard = slices.Clone(t.rs[a:b])
			if k.Name == "x" {
				t.save(0)
				t.deleteSel()
			}
		}
	case "v":
		if ctrl && len(clipboard) > 0 {
			t.save(0)
			t.insert(clipboard)
			t.reveal = true
		}
	case "0", "1", "2", "8", "9", "t", "q":
		if ctrl {
			t.SetKind(map[string]Para{"0": Plain, "1": Title, "2": Head, "8": Bullet, "9": Todo, "t": Todo, "q": Quote}[k.Name])
			t.reveal = true
		}
	case "z":
		if ctrl && len(t.undo) > 0 {
			s := t.undo[len(t.undo)-1]
			t.undo = t.undo[:len(t.undo)-1]
			t.rs, t.caret, t.anchor = s.rs, s.caret, s.anchor
			t.lastKind, t.changed, t.reveal = opNone, true, true
		}
	}
	if !keep {
		t.keepX = false
	}
}

func (t *TextArea) lineH() int {
	if len(t.lines) == 0 {
		return 1
	}
	return t.lines[0].h
}

// Layout lays the text out as wide as it's given, as tall as it is (at
// least the room's Min), in look; viewH is how much shows at a time (a
// page for PageDown).
func (t *TextArea) Layout(gtx ink.Ctx, look AreaLook, viewH int) ink.Dims {
	t.events(gtx)
	t.lastLook, t.viewH = look, viewH
	w := gtx.Constraints.Max.X
	t.width = w
	t.caret, t.anchor = t.clamp(t.caret), t.clamp(t.anchor)

	// The paragraphs, wrapped into lines.
	t.lines, t.paras, t.boxes = t.lines[:0], t.paras[:0], t.boxes[:0]
	y := 0
	for s := 0; ; {
		e := t.paraEnd(s)
		kind, m := paraOf(t.rs[s:e])
		f := look.Faces[kind]
		x := 0
		switch kind {
		case Bullet, Todo, Done, Quote:
			x = look.Indent
		case Title:
			if s > 0 {
				y += look.Gap * 2
			}
		case Head:
			if s > 0 {
				y += look.Gap
			}
		}
		pi := len(t.paras)
		t.paras = append(t.paras, span{s, s + m, e, kind})
		lh := text.Height(f)
		cs := s + m
		pieces := wrapRunes(f, t.rs[cs:e], max(w-x, 1))
		for _, pc := range pieces {
			t.lines = append(t.lines, vline{cs + pc[0], cs + pc[1], pi, x, y, lh, text.Ascent(f), f})
			y += lh
		}
		if kind == Todo || kind == Done {
			bs := text.Ascent(f)
			by := t.lines[len(t.lines)-len(pieces)].y + (lh-bs)/2
			t.boxes = append(t.boxes, box{image.Rect(x-look.Indent+(look.Indent-bs)/2, by, x-look.Indent+(look.Indent-bs)/2+bs, by+bs), pi})
		}
		y += look.Gap
		if e >= len(t.rs) {
			break
		}
		s = e + 1
	}
	size := image.Pt(w, max(y, gtx.Constraints.Min.Y))

	// A tap: on a to-do's box, it's ticked; elsewhere the caret goes there.
	if t.tapped {
		t.tapped = false
		hit := false
		for _, b := range t.boxes {
			if t.tapPos.In(b.r.Inset(-look.Indent / 3)) {
				p := t.paras[b.p]
				t.save(0)
				if p.kind == Todo {
					t.setMark(p.start, Done)
				} else {
					t.setMark(p.start, Todo)
				}
				t.changed, hit = true, true
				break
			}
		}
		if !hit && len(t.lines) > 0 {
			k := len(t.lines) - 1
			for i, l := range t.lines {
				if t.tapPos.Y < l.y+l.h {
					k = i
					break
				}
			}
			t.move(t.at(k, t.tapPos.X), false)
			t.reveal, t.keepX = false, false
		}
		if hit {
			return t.Layout(gtx, look, viewH) // the marks changed: again
		}
	}

	// Paint: the selection under the words, the words, the marks, the caret.
	ink.Fill(gtx.Ops, image.Rectangle{Max: size}, ink.White)
	a, b := t.Selection()
	for _, l := range t.lines {
		if a < b && a <= l.end && b >= l.start && !(b == l.start && l.start != l.end) {
			x0 := l.x + text.Width(l.face, string(t.rs[l.start:max(a, l.start)]))
			x1 := l.x + text.Width(l.face, string(t.rs[l.start:min(b, l.end)]))
			if b > l.end {
				x1 = max(x1, l.x+text.Width(l.face, " "))
			}
			ink.Fill(gtx.Ops, image.Rect(x0, l.y, x1, l.y+l.h), ink.Light)
		}
		p := t.paras[l.p]
		g := look.Fg
		if p.kind == Done || p.kind == Quote {
			g = look.Quiet
		}
		s := string(t.rs[l.start:l.end])
		ink.TextAt(gtx.Ops, l.face, l.x, l.y+l.base, s, g)
		if p.kind == Done && s != "" {
			my := l.y + l.base - l.base/3
			ink.Fill(gtx.Ops, image.Rect(l.x, my, l.x+text.Width(l.face, s), my+max(look.Stroke/2, 1)), look.Quiet)
		}
		first := l.start == p.cs
		switch {
		case p.kind == Bullet && first:
			r := max(text.Ascent(l.face)/7, 2)
			ink.Disc(gtx.Ops, image.Pt(l.x-look.Indent/2, l.y+l.base-text.Ascent(l.face)/3), r, look.Fg)
		case p.kind == Quote:
			ink.Fill(gtx.Ops, image.Rect(look.Indent/3, l.y, look.Indent/3+look.Stroke*2, l.y+l.h), look.Quiet)
		}
	}
	for _, bx := range t.boxes {
		ink.Frame(gtx.Ops, bx.r, look.Stroke, look.Fg)
		if t.paras[bx.p].kind == Done {
			s := bx.r.Dx()
			ink.Line(gtx.Ops, look.Stroke*2, look.Fg, bx.r.Min.Add(image.Pt(s/5, s/2)), bx.r.Min.Add(image.Pt(s*2/5, s*3/4)), bx.r.Min.Add(image.Pt(s*4/5, s/4)))
		}
	}
	if len(t.lines) > 0 {
		l := t.lines[t.lineOf(t.caret)]
		x := l.x + text.Width(l.face, string(t.rs[l.start:min(max(t.caret, l.start), l.end)]))
		t.caretR = image.Rect(x, l.y, x+max(look.Stroke, 2), l.y+l.h)
		if t.focused && !gtx.IsDisabled() {
			ink.Fill(gtx.Ops, t.caretR, look.Fg)
		}
	}
	if !gtx.IsDisabled() {
		ink.Input(gtx.Ops, ink.Area{Rect: image.Rectangle{Max: size}, Tag: t, Focus: true, Keys: []string{"*"}})
	}
	t.focused = gtx.Focused(t)
	return ink.Dims{Size: size}
}

// wrapRunes breaks rs into lines w wide at most, at spaces where it can:
// each [start, end) in rs, the space at a break left out of both.
func wrapRunes(f font.Face, rs []rune, w int) [][2]int {
	if len(rs) == 0 {
		return [][2]int{{0, 0}}
	}
	var out [][2]int
	start := 0
	for start < len(rs) {
		// As many as fit.
		end, x := start, 0
		for end < len(rs) {
			aw := text.Width(f, string(rs[end]))
			if x+aw > w && end > start {
				break
			}
			x += aw
			end++
		}
		if end >= len(rs) {
			out = append(out, [2]int{start, len(rs)})
			break
		}
		// Back to the last space, if there's one.
		brk := -1
		for i := end; i > start; i-- {
			if rs[i] == ' ' || i < end && rs[i] == ' ' {
				brk = i
				break
			}
		}
		if brk > start {
			out = append(out, [2]int{start, brk})
			start = brk + 1
		} else {
			out = append(out, [2]int{start, end})
			start = end
		}
	}
	return out
}

// Words is the text without its marks, a paragraph a line: what it says.
func (t *TextArea) Words() string {
	var b strings.Builder
	for i, p := range strings.Split(string(t.rs), "\n") {
		if i > 0 {
			b.WriteByte('\n')
		}
		_, m := paraOf([]rune(p))
		b.WriteString(string([]rune(p)[m:]))
	}
	return b.String()
}
