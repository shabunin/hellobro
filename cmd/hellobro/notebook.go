package main

// The notebook: pages to write on with the pen, sums worked out by hand,
// say. Its window shows a page, fitted in, under a strip of tools: the
// pen or the eraser, undo and redo, clearing the page, the paper
// (squares, lines, plain) and the pages. The pen writes; a button on it
// held, it erases; a finger swiped across turns the page, and turning
// past the last page that has something on it makes a new one.
//
// A notebook is a file in notebooks/ next to hellobro (NAME.hbnote, its
// strokes as text, kept a few seconds after the last change and on
// closing), reached from the switcher and the launcher. Its window comes
// back with the others.
//
// The ink shows as it's written with the fast waveform, black on white;
// once the pen lifts, what it went over gets a grayscale update, which
// brings back the paper's gray lines under it.

import (
	"bufio"
	"bytes"
	"fmt"
	"image"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"hellobro/kobo"
)

const (
	notePageW, notePageH = 1404, 1872 // a page, in its own units: the Elipsa's screen, upright
	noteRule             = 45         // the squares' side and the lines' spacing: 5 mm on the Elipsa
	noteEraser           = 16         // how near the eraser takes a stroke, in page units
	noteKeepAfter        = 3 * time.Second
	noteSwipe            = 120 // px across a finger goes, to turn the page
	noteExt              = ".hbnote"
	notePapers           = "squares lines plain"

	ownInk  = 7 // the pen writing or erasing in a notebook
	ownLeaf = 8 // a finger on a notebook's page: maybe turning it
)

// notePt is a point of a stroke: where, in page units, and the pen's
// radius there, in tenths of a unit (from how hard it pressed).
type notePt struct{ x, y, r int16 }

type noteStroke struct {
	pts []notePt
	box image.Rectangle // in page units, with the pen's width
}

func (s *noteStroke) bound() {
	s.box = image.Rectangle{}
	for _, p := range s.pts {
		r := int(p.r)/10 + 1
		s.box = s.box.Union(image.Rect(int(p.x)-r, int(p.y)-r, int(p.x)+r+1, int(p.y)+r+1))
	}
}

type notebook struct {
	name  string
	paper string // one of notePapers
	pages [][]*noteStroke
}

// --- the file ---

// parseNotebook reads a notebook as noteText writes it: a line saying
// what it is, its paper, then each page, a line of points per stroke.
func parseNotebook(name string, r io.Reader) (*notebook, error) {
	nb := &notebook{name: name, paper: "squares"}
	sc := bufio.NewScanner(r)
	sc.Buffer(nil, 16<<20) // a stroke is a line, and may be long
	if !sc.Scan() || !strings.HasPrefix(sc.Text(), "hellobro notebook") {
		if err := sc.Err(); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("not a notebook")
	}
	for sc.Scan() {
		word, rest, _ := strings.Cut(sc.Text(), " ")
		switch word {
		case "paper":
			if strings.Contains(notePapers, rest) && rest != "" {
				nb.paper = rest
			}
		case "page":
			nb.pages = append(nb.pages, nil)
		case "s":
			if len(nb.pages) == 0 {
				nb.pages = append(nb.pages, nil)
			}
			s := &noteStroke{}
			for _, f := range strings.Fields(rest) {
				var v [3]int
				parts := strings.Split(f, ",")
				if len(parts) != 3 {
					return nil, fmt.Errorf("a point that isn't: %q", f)
				}
				for i, p := range parts {
					n, err := strconv.Atoi(p)
					if err != nil {
						return nil, fmt.Errorf("a point that isn't: %q", f)
					}
					v[i] = max(-1<<15, min(1<<15-1, n))
				}
				s.pts = append(s.pts, notePt{int16(v[0]), int16(v[1]), int16(v[2])})
			}
			if len(s.pts) > 0 {
				s.bound()
				nb.pages[len(nb.pages)-1] = append(nb.pages[len(nb.pages)-1], s)
			}
		}
	}
	if len(nb.pages) == 0 {
		nb.pages = [][]*noteStroke{nil}
	}
	return nb, sc.Err()
}

// noteText is nb as its file has it. Empty pages at the end aren't kept.
func (nb *notebook) noteText() []byte {
	var b bytes.Buffer
	b.WriteString("hellobro notebook 1\npaper " + nb.paper + "\n")
	n := len(nb.pages)
	for n > 1 && len(nb.pages[n-1]) == 0 {
		n--
	}
	for _, pg := range nb.pages[:n] {
		b.WriteString("page\n")
		for _, s := range pg {
			b.WriteString("s")
			for _, p := range s.pts {
				fmt.Fprintf(&b, " %d,%d,%d", p.x, p.y, p.r)
			}
			b.WriteString("\n")
		}
	}
	return b.Bytes()
}

func notesDir() string { return kobo.AppFile("notebooks") }

func notePath(name string) string { return filepath.Join(notesDir(), name+noteExt) }

// loadNotebook reads the notebook name; a new one if there's no such file.
func loadNotebook(name string) (*notebook, error) {
	f, err := os.Open(notePath(name))
	if os.IsNotExist(err) {
		return &notebook{name: name, paper: "squares", pages: [][]*noteStroke{nil}}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parseNotebook(name, f)
}

// save writes nb, a new file over the old one: a crash halfway leaves
// the old one whole.
func (nb *notebook) save() error {
	if err := os.MkdirAll(notesDir(), 0o755); err != nil {
		return err
	}
	tmp := notePath(nb.name) + ".new"
	if err := os.WriteFile(tmp, nb.noteText(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, notePath(nb.name))
}

// noteNames are the notebooks there are, the last written first.
func noteNames() []string {
	es, _ := os.ReadDir(notesDir())
	type named struct {
		name string
		at   time.Time
	}
	var ns []named
	for _, e := range es {
		if n, ok := strings.CutSuffix(e.Name(), noteExt); ok && !e.IsDir() {
			var at time.Time
			if fi, err := e.Info(); err == nil {
				at = fi.ModTime()
			}
			ns = append(ns, named{n, at})
		}
	}
	slices.SortFunc(ns, func(a, b named) int { return b.at.Compare(a.at) })
	var names []string
	for _, n := range ns {
		names = append(names, n.name)
	}
	return names
}

// newNoteName names a new notebook for when it's made.
func newNoteName(now time.Time, taken func(string) bool) string {
	base := now.Format("2006-01-02 15.04")
	name := base
	for i := 2; taken(name); i++ {
		name = fmt.Sprintf("%s (%d)", base, i)
	}
	return name
}

// --- editing ---

// noteOp is a change to a page, to undo and redo: strokes added, or
// taken away.
type noteOp struct {
	page     int
	add, del []*noteStroke
}

// apply makes op on nb, or undoes it.
func (nb *notebook) apply(op noteOp, undo bool) {
	add, del := op.add, op.del
	if undo {
		add, del = del, add
	}
	pg := slices.DeleteFunc(nb.pages[op.page], func(s *noteStroke) bool { return slices.Contains(del, s) })
	nb.pages[op.page] = append(pg, add...)
}

// segDist is how far p is from the segment a–b.
func segDist(p, a, b image.Point) float64 {
	dx, dy := float64(b.X-a.X), float64(b.Y-a.Y)
	t := 0.0
	if l := dx*dx + dy*dy; l > 0 {
		t = math.Max(0, math.Min(1, (float64(p.X-a.X)*dx+float64(p.Y-a.Y)*dy)/l))
	}
	return math.Hypot(float64(p.X-a.X)-t*dx, float64(p.Y-a.Y)-t*dy)
}

// hits says whether the eraser at p (page units) takes s.
func (s *noteStroke) hits(p image.Point) bool {
	if !p.In(s.box.Inset(-noteEraser)) {
		return false
	}
	pt := func(i int) image.Point { return image.Pt(int(s.pts[i].x), int(s.pts[i].y)) }
	for i := range s.pts {
		j := max(0, i-1)
		if segDist(p, pt(j), pt(i)) <= noteEraser+float64(s.pts[i].r)/10 {
			return true
		}
	}
	return false
}

// --- the window ---

type noteWin struct {
	nb         *notebook
	cur        int // the page shown
	erasing    bool
	undo, redo []noteOp
	changed    time.Time // not kept since; zero: kept

	p     kobo.Panel
	bar   image.Rectangle // the tools' strip
	btns  []noteBtn
	page  image.Rectangle // the page, on screen
	scale float64         // screen px a page unit
}

type noteBtn struct {
	id, label string
	r         image.Rectangle
}

func (w *noteWin) Title() string    { return "Notebook " + w.nb.name }
func (w *noteWin) Class() string    { return "hellobro" }
func (w *noteWin) Instance() string { return "notebook" }

// noteInk is a stroke going on: written, or erasing.
type noteInk struct {
	w      *noteWin
	g      *kobo.Gesture
	erase  bool
	s      *noteStroke   // what's written
	del    []*noteStroke // what's erased
	last   image.Point   // where it was, on screen
	marked image.Rectangle
}

type noteState struct {
	ink *noteInk
}

func (w *noteWin) place(a *bro, p kobo.Panel) {
	w.p, w.btns, w.bar, w.page = p, nil, image.Rectangle{}, image.Rectangle{}
	if p.Outer.Empty() {
		return
	}
	h := 56
	if kobo.IsBig() {
		h = 76
	}
	b := p.Body.Inset(kobo.Pad / 2)
	w.bar = image.Rect(b.Min.X, b.Min.Y, b.Max.X, b.Min.Y+h)
	area := image.Rect(b.Min.X, w.bar.Max.Y+kobo.Pad/2, b.Max.X, b.Max.Y)
	w.scale = math.Min(float64(area.Dx())/notePageW, float64(area.Dy())/notePageH)
	pw, ph := int(notePageW*w.scale), int(notePageH*w.scale)
	at := image.Pt(area.Min.X+(area.Dx()-pw)/2, area.Min.Y+(area.Dy()-ph)/2)
	w.page = image.Rectangle{at, at.Add(image.Pt(pw, ph))}
	w.layBar(a)
}

// layBar lays the tools along the strip, as many as fit: what's left
// out goes from the least needed.
func (w *noteWin) layBar(a *bro) {
	tools := []noteBtn{{id: "pen"}, {id: "eraser"}, {id: "undo"}, {id: "redo"}, {id: "clear", label: "clear page"}, {id: "paper", label: w.nb.paper}}
	pages := []noteBtn{{id: "prev"}, {id: "next"}}
	width := func(b noteBtn) int { return max(kobo.TextWidth(a.F.Small, b.label)+36, w.bar.Dy()+8) }
	for i := range tools {
		if tools[i].label == "" {
			tools[i].label = tools[i].id
		}
	}
	const gap = 10
	room := w.bar.Dx() - width(pages[0]) - width(pages[1]) - kobo.TextWidth(a.F.Small, "999 / 999") - 4*gap
	for _, drop := range []string{"paper", "redo", "clear"} {
		need := 0
		for _, t := range tools {
			need += width(t) + gap
		}
		if need <= room {
			break
		}
		tools = slices.DeleteFunc(tools, func(t noteBtn) bool { return t.id == drop })
	}
	x := w.bar.Min.X
	for _, t := range tools {
		t.r = image.Rect(x, w.bar.Min.Y+4, x+width(t), w.bar.Max.Y-4)
		w.btns = append(w.btns, t)
		x = t.r.Max.X + gap
	}
	x = w.bar.Max.X
	for _, t := range []noteBtn{pages[1], pages[0]} {
		t.r = image.Rect(x-width(t), w.bar.Min.Y+4, x, w.bar.Max.Y-4)
		w.btns = append(w.btns, t)
		x = t.r.Min.X - kobo.TextWidth(a.F.Small, "999 / 999") - 2*gap
	}
}

func (w *noteWin) panel(a *bro) kobo.Panel { return a.winPanel(w.p, a.focusedWin() == window(w)) }

func (w *noteWin) draw(a *bro) {
	if w.p.Outer.Empty() {
		return
	}
	pn := w.panel(a)
	a.DrawPanel(pn, "")
	w.drawBar(a)
	w.drawPage(a, w.page)
	a.Mark(pn.Outer, true)
}

func (w *noteWin) drawTitle(a *bro) {
	if !w.p.Outer.Empty() {
		a.DrawPanelTitle(w.panel(a), "")
	}
}

// drawBar draws the tools, the one in hand black, and which page.
func (w *noteWin) drawBar(a *bro) {
	c := a.C
	c.Fill(w.bar, kobo.White)
	for _, b := range w.btns {
		on := b.id == "pen" && !w.erasing || b.id == "eraser" && w.erasing
		off := b.id == "undo" && len(w.undo) == 0 || b.id == "redo" && len(w.redo) == 0 || b.id == "prev" && w.cur == 0
		bg, ink := uint8(kobo.White), uint8(kobo.Black)
		switch {
		case on:
			bg, ink = kobo.Black, kobo.White
		case off:
			ink = kobo.Mid
		}
		c.Fill(b.r, bg)
		c.Frame(b.r, 2, kobo.Black)
		c.Text(a.F.Small, b.r, b.label, ink, kobo.AlignCenter)
		if b.id == "prev" || b.id == "next" { // the font has no arrows: a triangle
			m, h := b.r.Min.Add(b.r.Max).Div(2), b.r.Dy()/4
			for dy := -h; dy <= h; dy++ {
				l := h - kobo.Abs(dy)
				if b.id == "next" {
					c.Fill(image.Rect(m.X-h/2, m.Y+dy, m.X-h/2+l+1, m.Y+dy+1), ink)
				} else {
					c.Fill(image.Rect(m.X+h/2-l, m.Y+dy, m.X+h/2+1, m.Y+dy+1), ink)
				}
			}
		}
	}
	if n := len(w.btns); n >= 2 {
		r := image.Rect(w.btns[n-1].r.Max.X, w.bar.Min.Y, w.btns[n-2].r.Min.X, w.bar.Max.Y)
		c.Text(a.F.Small, r, fmt.Sprintf("%d / %d", w.cur+1, len(w.nb.pages)), kobo.Black, kobo.AlignCenter)
	}
}

// drawPage draws the page where it's in clip: the paper, then the
// strokes over it.
func (w *noteWin) drawPage(a *bro, clip image.Rectangle) {
	c := a.C
	area := image.Rect(w.p.Body.Min.X, w.bar.Max.Y, w.p.Body.Max.X, w.p.Body.Max.Y).Intersect(clip)
	c.Fill(area, kobo.White)
	clip = clip.Intersect(w.page)
	if clip.Empty() {
		return
	}
	pg := w.page
	switch w.nb.paper {
	case "squares":
		for u := noteRule; u < notePageW; u += noteRule {
			x := pg.Min.X + int(float64(u)*w.scale)
			c.Fill(image.Rect(x, pg.Min.Y, x+1, pg.Max.Y).Intersect(clip), kobo.Light)
		}
		fallthrough
	case "lines":
		for u := 2 * noteRule; u < notePageH; u += noteRule {
			y := pg.Min.Y + int(float64(u)*w.scale)
			c.Fill(image.Rect(pg.Min.X, y, pg.Max.X, y+1).Intersect(clip), kobo.Light)
		}
	}
	if pg.Max.X < w.p.Body.Max.X-kobo.Pad || pg.Min.X > w.p.Body.Min.X+kobo.Pad { // a page narrower than the window: its edges
		c.Fill(image.Rect(pg.Min.X-2, pg.Min.Y, pg.Min.X, pg.Max.Y).Intersect(area), kobo.Mid)
		c.Fill(image.Rect(pg.Max.X, pg.Min.Y, pg.Max.X+2, pg.Max.Y).Intersect(area), kobo.Mid)
	}
	for _, s := range w.nb.pages[w.cur] {
		if w.toScreenRect(s.box).Overlaps(clip) {
			w.drawStroke(a, s, clip)
		}
	}
}

func (w *noteWin) toScreen(p notePt) image.Point {
	return image.Pt(w.page.Min.X+int(float64(p.x)*w.scale+0.5), w.page.Min.Y+int(float64(p.y)*w.scale+0.5))
}

func (w *noteWin) toScreenRect(r image.Rectangle) image.Rectangle {
	return image.Rect(w.page.Min.X+int(float64(r.Min.X)*w.scale), w.page.Min.Y+int(float64(r.Min.Y)*w.scale),
		w.page.Min.X+int(math.Ceil(float64(r.Max.X)*w.scale))+1, w.page.Min.Y+int(math.Ceil(float64(r.Max.Y)*w.scale))+1)
}

func (w *noteWin) toPage(p image.Point) image.Point {
	return image.Pt(int(float64(p.X-w.page.Min.X)/w.scale), int(float64(p.Y-w.page.Min.Y)/w.scale))
}

// radius is the pen's on screen at p.
func (w *noteWin) radius(p notePt) int { return max(1, int(float64(p.r)/10*w.scale+0.5)) }

func (w *noteWin) drawStroke(a *bro, s *noteStroke, clip image.Rectangle) {
	prev := w.toScreen(s.pts[0])
	a.C.Disc(prev, w.radius(s.pts[0]), kobo.Black, clip)
	for _, p := range s.pts[1:] {
		q := w.toScreen(p)
		a.C.Line(prev, q, w.radius(p), kobo.Black, clip)
		prev = q
	}
}

// show draws the page again, and its strip, where they changed.
func (w *noteWin) show(a *bro) {
	if !a.shows(w) {
		return
	}
	w.drawBar(a)
	w.drawPage(a, w.page)
	a.Mark(image.Rect(w.p.Body.Min.X, w.bar.Min.Y, w.p.Body.Max.X, w.p.Body.Max.Y), true)
}

// edited notes a change made: to undo, and to keep.
func (w *noteWin) edited(op noteOp) {
	w.undo = append(w.undo, op)
	w.redo = nil
	w.changed = time.Now()
}

func (w *noteWin) turn(a *bro, by int) {
	n := w.cur + by
	if n == len(w.nb.pages) && len(w.nb.pages[w.cur]) > 0 { // past the last page: a new one
		w.nb.pages = append(w.nb.pages, nil)
		w.changed = time.Now()
	}
	if n < 0 || n >= len(w.nb.pages) {
		return
	}
	w.cur = n
	w.show(a)
	if a.shows(w) {
		a.MarkFlash(w.page) // another page: no ghost of this one
	}
}

func (w *noteWin) doUndo(a *bro, undo bool) {
	from, to := &w.undo, &w.redo
	if !undo {
		from, to = to, from
	}
	if len(*from) == 0 {
		return
	}
	op := (*from)[len(*from)-1]
	*from = (*from)[:len(*from)-1]
	*to = append(*to, op)
	w.nb.apply(op, undo)
	w.cur = op.page
	w.changed = time.Now()
	w.show(a)
}

func (w *noteWin) tap(a *bro, p image.Point) {
	for _, b := range w.btns {
		if !kobo.Hit(p, b.r) {
			continue
		}
		switch b.id {
		case "pen", "eraser":
			w.erasing = b.id == "eraser"
			w.drawBar(a)
			a.Mark(w.bar, true)
		case "undo", "redo":
			w.doUndo(a, b.id == "undo")
		case "clear":
			if pg := w.nb.pages[w.cur]; len(pg) > 0 {
				op := noteOp{page: w.cur, del: slices.Clone(pg)}
				w.nb.apply(op, false)
				w.edited(op)
				w.show(a)
				a.MarkFlash(w.page)
			}
		case "paper":
			ps := strings.Fields(notePapers)
			w.nb.paper = ps[(slices.Index(ps, w.nb.paper)+1)%len(ps)]
			w.changed = time.Now()
			w.place(a, w.p) // its button says another word
			w.show(a)
		case "prev":
			w.turn(a, -1)
		case "next":
			w.turn(a, 1)
		}
		return
	}
}

const (
	keyE = 18
	keyP = 25
	keyY = 21
)

func (w *noteWin) press(a *bro, code uint16, value int32) bool {
	if value == 0 {
		return false
	}
	m := a.tv.mods
	switch {
	case code == keyLeft || code == keyPgUp || code == keyUp:
		w.turn(a, -1)
	case code == keyRight || code == keyPgDn || code == keyDown:
		w.turn(a, 1)
	case m.ctrl && code == kobo.KeyZ:
		w.doUndo(a, !m.shift)
	case m.ctrl && code == keyY:
		w.doUndo(a, false)
	case value == 1 && !m.ctrl && (code == keyE || code == keyP):
		w.erasing = code == keyE
		if a.shows(w) {
			w.drawBar(a)
			a.Mark(w.bar, true)
		}
	default:
		return false
	}
	return true
}

func (w *noteWin) closing(a *bro) {
	if a.nt.ink != nil && a.nt.ink.w == w {
		a.nt.ink = nil
	}
	w.keep(a)
	w.place(a, kobo.Panel{})
}

// keep writes the notebook, if it changed.
func (w *noteWin) keep(a *bro) {
	if w.changed.IsZero() {
		return
	}
	w.changed = time.Time{}
	if err := w.nb.save(); err != nil {
		log.Printf("notebook %s: %v", w.nb.name, err)
		a.Notify("notebook not kept: " + err.Error())
		return
	}
	log.Printf("notebook %s kept", w.nb.name)
}

// --- the pen and fingers ---

// noteTouch takes a contact coming down on a notebook's page: the pen
// writes or erases, a finger may turn the page.
func (a *bro) noteTouch(ev kobo.TouchEv, p image.Point, g *kobo.Gesture) bool {
	w, ok := a.windowAt(p).(*noteWin)
	if !ok || !p.In(w.page) || !a.shows(w) {
		return false
	}
	if !ev.Pen {
		if p.X < kobo.EdgeWidth || p.X >= a.C.Bounds().Dx()-kobo.EdgeWidth { // the edges' swipes
			return false
		}
		g.Owner = ownLeaf
		return true
	}
	if a.nt.ink != nil {
		return false
	}
	g.Owner = ownInk
	k := &noteInk{w: w, g: g, erase: w.erasing || ev.Button, last: p}
	a.nt.ink = k
	if k.erase {
		a.noteErase(k, p)
		return true
	}
	k.s = &noteStroke{}
	a.noteInkTo(k, p, ev.Pressure)
	return true
}

// noteMove follows the pen.
func (a *bro) noteMove(ev kobo.TouchEv, p image.Point, g *kobo.Gesture) bool {
	k := a.nt.ink
	if g.Owner == ownLeaf {
		return true
	}
	if g.Owner != ownInk || k == nil || k.g != g {
		return false
	}
	if p == k.last || !a.shows(k.w) {
		return true
	}
	if k.erase {
		// along the way, not only where the pen was seen: it can be quick
		d := p.Sub(k.last)
		steps := max(1, int(math.Hypot(float64(d.X), float64(d.Y))/(noteEraser*k.w.scale)))
		for i := 1; i <= steps; i++ {
			a.noteErase(k, k.last.Add(d.Mul(i).Div(steps)))
		}
		k.last = p
		return true
	}
	a.noteInkTo(k, p, ev.Pressure)
	return true
}

// noteInkTo adds p to the stroke, and shows it at once: black on white,
// the fast waveform. Off the page it goes on, unseen, as a pen goes off
// the paper's edge.
func (a *bro) noteInkTo(k *noteInk, p image.Point, pressure int32) {
	w := k.w
	pp := w.toPage(p)
	frac := math.Min(1, float64(pressure)/float64(a.PressureMax()))
	pt := notePt{int16(pp.X), int16(pp.Y), int16(15 + 25*frac)} // 1.5 to 4 units
	k.s.pts = append(k.s.pts, pt)
	r := w.radius(pt)
	from := k.last
	if len(k.s.pts) == 1 {
		from = p
	}
	a.C.Line(from, p, r, kobo.Black, w.page)
	box := image.Rectangle{from, from}.Union(image.Rectangle{p, p}).Inset(-r - 1).Intersect(w.page)
	k.marked = k.marked.Union(box)
	k.last = p
	a.Mark(box, false)
}

// noteErase takes the strokes the eraser touches at p off the page.
func (a *bro) noteErase(k *noteInk, p image.Point) {
	w := k.w
	pp := w.toPage(p)
	var gone []*noteStroke
	for _, s := range w.nb.pages[w.cur] {
		if s.hits(pp) {
			gone = append(gone, s)
		}
	}
	if len(gone) == 0 {
		return
	}
	w.nb.apply(noteOp{page: w.cur, del: gone}, false)
	k.del = append(k.del, gone...)
	var r image.Rectangle
	for _, s := range gone {
		r = r.Union(w.toScreenRect(s.box))
	}
	w.drawPage(a, r)
	a.Mark(r.Intersect(w.page), true)
}

// noteLift ends what the pen or the finger did.
func (a *bro) noteLift(g *kobo.Gesture) bool {
	switch g.Owner {
	case ownLeaf:
		if w, ok := a.windowAt(g.Start).(*noteWin); ok {
			a.noteFocus(w)
			dx, dy := g.Last.X-g.Start.X, g.Last.Y-g.Start.Y
			if kobo.Abs(dx) > noteSwipe && kobo.Abs(dx) > 2*kobo.Abs(dy) {
				by := 1 // leftward: the next, as pages turn
				if dx > 0 {
					by = -1
				}
				w.turn(a, by)
			}
		}
		return true
	case ownInk:
	default:
		return false
	}
	k := a.nt.ink
	if k == nil || k.g != g {
		return true
	}
	a.nt.ink = nil
	w := k.w
	switch {
	case k.erase && len(k.del) > 0:
		w.edited(noteOp{page: w.cur, del: k.del})
	case !k.erase && len(k.s.pts) > 0:
		k.s.bound()
		w.nb.pages[w.cur] = append(w.nb.pages[w.cur], k.s)
		w.edited(noteOp{page: w.cur, add: []*noteStroke{k.s}})
		a.Mark(k.marked, true) // the cleanup: grays (the paper) back under it
	}
	if a.shows(w) {
		w.drawBar(a) // undo can now
		a.Mark(w.bar, true)
	}
	a.noteFocus(w)
	return true
}

// noteFocus gives w the focus, if it hasn't got it.
func (a *bro) noteFocus(w *noteWin) {
	if a.focusedWin() != window(w) {
		a.wm.tree.Focus(a.nodeOf(w))
		a.wmApply()
	}
}

// --- notebooks: which, and keeping them ---

func (a *bro) noteWins() []*noteWin {
	var ws []*noteWin
	for _, w := range a.windows() {
		if n, ok := w.(*noteWin); ok {
			ws = append(ws, n)
		}
	}
	return ws
}

// noteWindow is the notebook name's window, if it's open.
func (a *bro) noteWindow(name string) *noteWin {
	for _, w := range a.noteWins() {
		if w.nb.name == name {
			return w
		}
	}
	return nil
}

// noteOpen makes the window for the notebook name ("": a new one), or
// says why it can't; it's not opened yet.
func (a *bro) noteOpen(name string) *noteWin {
	if name == "" {
		name = newNoteName(time.Now(), func(n string) bool {
			_, err := os.Stat(notePath(n))
			return err == nil || a.noteWindow(n) != nil
		})
	}
	nb, err := loadNotebook(name)
	if err != nil {
		log.Printf("notebook %s: %v", name, err)
		a.Notify("can't read notebook " + name + ": " + err.Error())
		return nil
	}
	return &noteWin{nb: nb}
}

// openNotebook focuses the notebook name's window, or opens it; "" opens
// a new notebook.
func (a *bro) openNotebook(name string) {
	if w := a.noteWindow(name); w != nil && name != "" {
		a.wm.tree.Focus(a.nodeOf(w))
		a.wmApply()
		return
	}
	if w := a.noteOpen(name); w != nil {
		a.openWin(w)
	}
}

// launchNotebook puts the notebook name ("": a new one) where the
// launcher t is; if it's open elsewhere, it's focused there.
func (a *bro) launchNotebook(t *termSession, name string) {
	if a.noteWindow(name) != nil && name != "" {
		a.closeWin(t)
		a.openNotebook(name)
		return
	}
	if w := a.noteOpen(name); w != nil {
		a.replaceWin(t, w)
	}
}

// noteTick keeps the notebooks that changed, once the pen has rested.
func (a *bro) noteTick() {
	if a.nt.ink != nil {
		return
	}
	for _, w := range a.noteWins() {
		if !w.changed.IsZero() && time.Since(w.changed) > noteKeepAfter {
			w.keep(a)
		}
	}
}

func (a *bro) noteExit() {
	for _, w := range a.noteWins() {
		w.keep(a)
	}
}
