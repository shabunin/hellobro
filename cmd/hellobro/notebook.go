package main

// The notebook: pages to write on with the pen, sums worked out by hand,
// say. Its window shows a page, fitted in, under a strip of tools: the
// pen or the eraser, undo and redo, the paper
// (squares, lines, plain) and the pages: two side by side where there's
// room for them (a landscape screen). The pen writes; a button on it
// held, it erases; a finger swiped across turns the page, and turning
// past the last page that has something on it makes a new one. The
// select tool takes what's written, and pictures, to move (notesel.go).
//
// Till a pen has been seen, fingers write: a stylus without a battery
// (or a Kobo Stylus whose battery is flat) is a finger to the panel. A
// pen once seen, it's kept (hellobro.json), and fingers turn pages.
//
// A notebook is a file in Notebooks/ on the Kobo's storage, as
// Xournal++ keeps them (NAME.xopp, notexopp.go): kept a few seconds
// after the last change and on closing, reached from the switcher and
// the launcher. Its window comes back with the others. The first ones
// were NAME.hbnote in notebooks/ next to hellobro, a line of points per
// stroke: they become .xopp when they're opened.
//
// The ink shows as it's written with the fast waveform, black on white;
// once the pen lifts, what it went over gets a grayscale update, which
// brings back the paper's gray lines under it.

import (
	"bufio"
	"cmp"
	"encoding/xml"
	"fmt"
	"image"
	"io"
	"io/fs"
	"log"
	"math"
	"os"
	"path"
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
	noteExt              = ".xopp"
	noteOldExt           = ".hbnote"
	noteDepth            = 4 // folders in folders, at most
	notePapers           = "squares lines plain"

	ownInk  = 7 // the pen writing or erasing in a notebook
	ownLeaf = 8 // a finger on a notebook's page: maybe turning it
)

// notePt is a point of a stroke: where, in page units, and the pen's
// radius there, in tenths of a unit (from how hard it pressed).
type notePt struct{ x, y, r int16 }

type noteStroke struct {
	pts    []notePt
	box    image.Rectangle // in page units, with the pen's width
	attrs  []xml.Attr      // from its file: the tool, its color… (nil: ours, black)
	pic    *notePic        // a picture in box, not a stroke (notepic.go)
	txt    *noteText       // a text from box's top left, not a stroke (notetext.go)
	gray   uint8           // its ink: black, or a gray (notepen.go)
	marker bool            // a highlighter's: under the ink
}

func (s *noteStroke) bound() {
	if s.txt != nil {
		s.box = s.txt.box(s.box.Min)
		return
	}
	if s.pic != nil {
		return
	}
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
	meta  []notePage // what else the pages have, as their file had it
	attrs []xml.Attr // the file's own
	old   string     // the .hbnote it was, to go once it's kept as .xopp
	disk  fileStamp  // its file as it was read or written: another change is the computer's
}

// fileStamp is a file's time and size: changed by something else, they
// differ (FAT's two seconds won't tell two writes apart that close,
// but the size most likely will).
type fileStamp struct {
	mod  time.Time
	size int64
}

// stampOf is path's stamp; false if there's no file.
func stampOf(path string) (fileStamp, bool) {
	fi, err := os.Stat(path)
	if err != nil {
		return fileStamp{}, false
	}
	return fileStamp{fi.ModTime(), fi.Size()}, true
}

// --- the file ---

// parseHbnote reads a notebook as it was first kept: a line saying what
// it is, its paper, then each page, a line of points per stroke.
func parseHbnote(name string, r io.Reader) (*notebook, error) {
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

// notesDir is where the notebooks are: Notebooks/ on the Kobo's storage,
// to be seen over USB and file sharing (Nickel leaves .xopp be).
func notesDir() string { return filepath.Join(shareRoot, "Notebooks") }

// oldNotesDir is where they were, as .hbnote.
var oldNotesDir = func() string { return kobo.AppFile("notebooks") }

func notePath(name string) string { return filepath.Join(notesDir(), name+noteExt) }

// loadNotebook reads the notebook name: its .xopp, or the .hbnote it
// was; a new one if there's neither.
func loadNotebook(name string) (*notebook, error) {
	f, err := os.Open(notePath(name))
	if err == nil {
		defer f.Close()
		st, _ := stampOf(notePath(name))
		nb, err := parseXopp(name, f)
		if nb != nil {
			nb.disk = st
		}
		return nb, err
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	old := filepath.Join(oldNotesDir(), name+noteOldExt)
	if f, err = os.Open(old); err == nil {
		defer f.Close()
		nb, err := parseHbnote(name, f)
		if nb != nil {
			nb.old = old
		}
		return nb, err
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	return &notebook{name: name, paper: "squares", pages: [][]*noteStroke{nil}}, nil
}

// save writes nb, a new file over the old one: a crash halfway leaves
// the old one whole. The .hbnote it was goes, once it's written. If its
// file changed since it was read (Xournal++ on a computer, over sftp),
// that's left be and nb goes beside it, as NAME (Kobo): what both wrote
// is kept, to be put together there.
func (nb *notebook) save() error {
	if st, ok := stampOf(notePath(nb.name)); ok && st != nb.disk {
		nb.name = keptBeside(nb.name)
		log.Printf("notebook: its file changed meanwhile: kept as %s", nb.name)
	}
	if err := os.MkdirAll(filepath.Dir(notePath(nb.name)), 0o755); err != nil {
		return err
	}
	b, err := nb.xopp()
	if err != nil {
		return err
	}
	if err := writeAtomic(notePath(nb.name), b); err != nil {
		return err
	}
	nb.disk, _ = stampOf(notePath(nb.name))
	if nb.old != "" {
		if err := os.Remove(nb.old); err != nil {
			log.Printf("notebook %s: %v", nb.name, err)
		}
		nb.old = ""
	}
	return nil
}

// keptBeside is a name beside name's for this device's notebook, one no
// file has: "plan (Kobo)", "plan (Kobo 2)"…
func keptBeside(name string) string {
	for i := 1; ; i++ {
		n := name + " (Kobo)"
		if i > 1 {
			n = fmt.Sprintf("%s (Kobo %d)", name, i)
		}
		if !fileExists(notePath(n)) {
			return n
		}
	}
}

// noteNames are the notebooks there are, the last written first: a
// name is its path under Notebooks/, its folders and all ("Work/plan").
func noteNames() []string {
	es, _ := noteEntries()
	slices.SortStableFunc(es, func(a, b libEntry) int { return cmp.Compare(b.at, a.at) })
	var names []string
	for _, e := range es {
		names = append(names, e.key)
	}
	return names
}

// noteEntries are the notebooks there are, in Notebooks/ and its folders
// (not the dot ones), and the .hbnote ones they were, for the list of
// them (notelist.go); and the folders.
func noteEntries() ([]libEntry, []string) {
	var es []libEntry
	var dirs []string
	entry := func(name string, d fs.DirEntry) libEntry {
		e := libEntry{key: name, rel: name + noteExt, name: path.Base(name)}
		if fi, err := d.Info(); err == nil {
			e.at, e.note = fi.ModTime().Unix(), fi.ModTime().Format("2 Jan 15:04")
		}
		return e
	}
	root := notesDir()
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || p == root {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") || strings.Count(rel, "/") >= noteDepth {
				return filepath.SkipDir
			}
			dirs = append(dirs, rel)
			return nil
		}
		if name, ok := strings.CutSuffix(rel, noteExt); ok {
			es = append(es, entry(name, d))
		}
		return nil
	})
	old, _ := os.ReadDir(oldNotesDir())
	for _, d := range old {
		name, ok := strings.CutSuffix(d.Name(), noteOldExt)
		if ok && !d.IsDir() && !slices.ContainsFunc(es, func(e libEntry) bool { return e.key == name }) {
			es = append(es, entry(name, d))
		}
	}
	return es, dirs
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
// taken away; or the page put in (its strokes add) or taken out
// (notepages.go).
type noteOp struct {
	kind     int // pageStrokes, pageIns, pageDel
	page     int
	add, del []*noteStroke
	meta     notePage // a page put in or taken out: the rest of it
}

// apply makes op on nb, or undoes it.
func (nb *notebook) apply(op noteOp, undo bool) {
	if op.kind != pageStrokes {
		if (op.kind == pageIns) != undo {
			nb.insertPage(op.page, op.add, op.meta)
		} else {
			nb.removePage(op.page)
		}
		return
	}
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
	if s.pic != nil || s.txt != nil || !p.In(s.box.Inset(-noteEraser)) { // a picture or a text, as in Xournal++: the eraser leaves it
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
	cur        int // the page shown, the left one of two
	erasing    bool
	selecting  bool          // the select tool in hand (notesel.go)
	texting    bool          // the text tool (notetext.go)
	typing     *noteTyping   // a text being typed
	sel        []*noteStroke // what it's taken, on selPage
	selPage    int
	mark       *noteMark // where the next paste goes
	penMenu    bool      // the pen's choices out (notepen.go)
	penOpts    []barBtn
	pdf        *notePDF    // the PDF the pages are on, open (notepdf.go)
	slotSize   image.Point // the page size the slots are for
	exporting  bool        // a PDF of it being written (noteexport.go)
	clearAsked time.Time   // "clear" tapped once: again soon, and it clears
	overview   bool        // the pages shown small (notepages.go)
	ovSel      int         // the page chosen there
	ovFirst    int         // the first shown
	undo, redo [][]noteOp  // each step's changes
	changed    time.Time   // not kept since; zero: kept
	keepErr    string      // the last write's error, said once

	p     kobo.Panel
	bar   image.Rectangle // the tools' strip
	side  bool            // it stands along the left: a narrow window (layBar)
	ex    *exLine         // a ":" command being typed (noteex.go)
	btns  []barBtn
	slots []image.Rectangle // the pages on screen, left to right: cur, cur+1
	scale float64           // screen px a page unit
}

func (w *noteWin) Title() string    { return "Notebook " + w.nb.name }
func (w *noteWin) Class() string    { return "hellobro" }
func (w *noteWin) Instance() string { return "notebook" }

// noteInk is a stroke going on: written, or erasing.
type noteInk struct {
	w      *noteWin
	g      *kobo.Gesture
	erase  bool
	page   int             // the page it's on
	at     image.Rectangle // and where that is on screen
	s      *noteStroke     // what's written
	del    []*noteStroke   // what's erased
	last   image.Point     // where it was, on screen
	marked image.Rectangle
}

type noteState struct {
	ink      *noteInk
	drag     *noteDrag   // the select tool's
	unsaved  []*notebook // closed but not kept (a full card, say): kept when they can be; opened, from here
	triedAt  time.Time   // when they were last tried
	lookedAt time.Time   // when the open ones' files were last looked at
}

const (
	noteGap    = 24  // between two pages side by side
	noteSpread = 0.8 // two pages side by side, if they're this big as one alone would be, or more
)

// noteSlots fits pages of size (page units) into area: one, or two side
// by side if there's room for two nearly as big (a landscape screen).
// It gives where they go, and the scale.
func noteSlots(area image.Rectangle, size image.Point) ([]image.Rectangle, float64) {
	h := float64(area.Dy()) / float64(size.Y)
	one := math.Min(float64(area.Dx())/float64(size.X), h)
	two := math.Min(float64(area.Dx()-noteGap)/float64(2*size.X), h)
	n, scale := 1, one
	if two >= noteSpread*one {
		n, scale = 2, two
	}
	pw, ph := int(float64(size.X)*scale), int(float64(size.Y)*scale)
	x := area.Min.X + (area.Dx()-n*pw-(n-1)*noteGap)/2
	y := area.Min.Y + (area.Dy()-ph)/2
	var slots []image.Rectangle
	for i := 0; i < n; i++ {
		slots = append(slots, image.Rect(x, y, x+pw, y+ph))
		x += pw + noteGap
	}
	return slots, scale
}

func (w *noteWin) place(a *bro, p kobo.Panel) {
	w.p, w.btns, w.bar, w.slots, w.side = p, nil, image.Rectangle{}, nil, false
	if p.Outer.Empty() {
		return
	}
	h := barHeight()
	b := p.Body.Inset(kobo.Pad / 2)
	w.bar = image.Rect(b.Min.X, b.Min.Y, b.Max.X, b.Min.Y+h)
	if tools, _ := barTools(false); w.barNeed(a, tools) > w.barRoom(a) && b.Dy() > b.Dx() { // not all on top: down the side (the pens' tools say, so it stays put for the overview's)
		w.side = true
		cw := max(h+8, kobo.TextWidth(a.F.Small, "99 / 99")+20)
		w.bar = image.Rect(b.Min.X, b.Min.Y, b.Min.X+cw, b.Max.Y)
	}
	if !w.overview && a.barOff(w, "note-bar", false) { // the pen and the keys only (the overview keeps it: its buttons are all it has)
		w.side, w.bar = false, image.Rect(b.Min.X, p.Body.Min.Y, b.Max.X, p.Body.Min.Y)
	}
	w.slotSize = w.pageSize(w.cur)
	w.slots, w.scale = noteSlots(w.paperArea().Inset(kobo.Pad/2), w.slotSize)
	if !w.bar.Empty() {
		w.layBar(a)
	}
}

// pages is where the pages show, all told.
func (w *noteWin) pages() image.Rectangle {
	var r image.Rectangle
	for _, s := range w.slots {
		r = r.Union(s)
	}
	return r
}

// shown is how many pages show (one, even while the window doesn't).
func (w *noteWin) shown() int { return max(1, len(w.slots)) }

// strokes are page n's, none if it's not been written on yet.
func (w *noteWin) strokes(n int) []*noteStroke {
	if n < len(w.nb.pages) {
		return w.nb.pages[n]
	}
	return nil
}

// counter says which pages show, of how many; in the overview, which
// is chosen.
func (w *noteWin) counter() string {
	if w.overview {
		return fmt.Sprintf("%d / %d", w.ovSel+1, len(w.nb.pages))
	}
	n := w.shown()
	all := max(len(w.nb.pages), w.cur+n)
	if n == 2 {
		return fmt.Sprintf("%d–%d / %d", w.cur+1, w.cur+2, all)
	}
	return fmt.Sprintf("%d / %d", w.cur+1, all)
}

// barTools are the tools on the strip (the overview's, or the pen's
// and the rest), All Notebooks first, and those that go first where
// there's no room for all.
func barTools(overview bool) (tools []barBtn, drops []string) {
	all := barBtn{id: "all", label: "‹ All Notebooks"} // at the left always, as the books' All Books (notelist.go)
	if overview {
		return []barBtn{all, {id: "open"}, {id: "new before"}, {id: "new after"}, {id: "copy page"}, {id: "clear", label: "clear page"}, {id: "delete"}, {id: "earlier"}, {id: "later"}, {id: "undo"}},
			[]string{"undo", "copy page", "new before"}
	}
	return []barBtn{all, {id: "pen"}, {id: "eraser"}, {id: "select"}, {id: "text"}, {id: "undo"}, {id: "redo"}, {id: "paste"}, {id: "paper"}, {id: "export"}},
		[]string{"paper", "export", "redo", "paste"}
}

// barNeed is how wide tools are along the top.
func (w *noteWin) barNeed(a *bro, tools []barBtn) int { return barNeed(a, w.bar, tools) }

// barCounter is how wide the page counter is.
func (w *noteWin) barCounter(a *bro) int {
	if w.shown() == 2 {
		return kobo.TextWidth(a.F.Small, "999–999 / 999")
	}
	return kobo.TextWidth(a.F.Small, "999 / 999")
}

// barRoom is the room along the top for the tools: the page arrows and
// counter are at its end.
func (w *noteWin) barRoom(a *bro) int { return barRoom(w.bar, w.barCounter(a)) }

// layBar lays the tools along the strip, as many as fit: what's left
// out goes from the least needed. Down the side, the page arrows and
// counter are at its foot.
func (w *noteWin) layBar(a *bro) {
	tools, drops := barTools(w.overview)
	pages := []barBtn{{id: "prev"}, {id: "next"}}
	if w.side {
		w.laySide(a, tools, drops, pages)
		w.layPenMenu(a)
		return
	}
	w.btns = layTop(a, w.bar, tools, drops, w.barCounter(a))
	w.layPenMenu(a)
}

// laySide lays the tools down the strip along the left, one under
// another, and at its foot the page before, the counter and the next.
// A short window still leaves some out, as along the top.
func (w *noteWin) laySide(a *bro, tools []barBtn, drops []string, pages []barBtn) {
	bh := noteSideH(w.bar.Dx())
	room := w.bar.Dy() - 3*(bh+barGap)
	tools[0].label = "‹"
	if w.overview {
		tools[slices.IndexFunc(tools, func(t barBtn) bool { return t.id == "clear" })].label = "clear"
	}
	for _, drop := range drops {
		if len(tools)*(bh+barGap) <= room {
			break
		}
		tools = slices.DeleteFunc(tools, func(t barBtn) bool { return t.id == drop })
	}
	for len(tools) > 0 && len(tools)*(bh+barGap) > room {
		tools = tools[:len(tools)-1]
	}
	in := w.bar.Inset(4)
	y := in.Min.Y
	for _, t := range tools {
		t.r = image.Rect(in.Min.X, y, in.Max.X, y+bh)
		w.btns = append(w.btns, t)
		y = t.r.Max.Y + barGap
	}
	y = in.Max.Y
	for _, t := range []barBtn{pages[1], pages[0]} { // the next at the foot; the counter between (counterRect)
		t.r = image.Rect(in.Min.X, y-bh, in.Max.X, y)
		w.btns = append(w.btns, t)
		y = t.r.Min.Y - bh - 2*barGap
	}
}

// noteSideH is a button's height down a strip cw wide: about square.
func noteSideH(cw int) int { return min(cw-8, 76) }

func (w *noteWin) panel(a *bro) kobo.Panel { return a.winPanel(w.p, a.focusedWin() == window(w)) }

func (w *noteWin) draw(a *bro) {
	if w.p.Outer.Empty() {
		return
	}
	w.fitSlots(a)
	pn := w.panel(a)
	a.DrawPanel(pn, "")
	w.drawBar(a)
	w.drawPages(a, w.paperArea())
	if w.ex != nil {
		a.drawEx(a.exStrip(w.p.Body), w.ex, w.exCmds(a))
	}
	a.Mark(pn.Outer, true)
}

func (w *noteWin) drawTitle(a *bro) {
	if !w.p.Outer.Empty() {
		a.DrawPanelTitle(w.panel(a), "")
	}
}

// drawBar draws the tools, the one in hand black, and which pages.
func (w *noteWin) drawBar(a *bro) {
	a.C.Fill(w.bar, kobo.White)
	for _, b := range w.btns {
		on := b.id == "pen" && !w.erasing && !w.selecting && !w.texting || b.id == "eraser" && w.erasing || b.id == "select" && w.selecting || b.id == "text" && w.texting
		off := b.id == "undo" && len(w.undo) == 0 || b.id == "redo" && len(w.redo) == 0 ||
			b.id == "prev" && w.cur == 0 || b.id == "next" && w.turnTo(1) < 0
		if w.overview && b.id != "undo" && b.id != "all" {
			off = w.ovOff(b.id)
		}
		if b.id == "clear" && w.clearAsking() {
			b.label, on, off = "tap again", true, false
		}
		ink, bg := drawBarBtn(a, b, on, off, func(r image.Rectangle, ink, bg uint8) bool {
			if icon := noteIcon(b.id, w.nb.paper); icon != nil {
				drawIcon(a.C, icon, r, ink, bg)
				return true
			}
			return false
		})
		if b.id == "pen" { // what it writes, under it
			w.drawPenSample(a, b.r, ink, bg)
		}
	}
	drawBarCounter(a, w.counterRect(), w.counter(), w.overview) // a button too: the pages, small
}

// counterRect is the page counter's, between the arrows.
func (w *noteWin) counterRect() image.Rectangle { return barCounter(w.btns, w.bar, w.side) }

// drawPages draws the pages where they're in clip: the paper, then the
// strokes over it.
func (w *noteWin) drawPages(a *bro, clip image.Rectangle) {
	if w.overview {
		w.drawOverview(a, clip)
		return
	}
	c := a.C
	area := w.paperArea().Intersect(clip)
	c.Fill(area, kobo.White)
	for i, pg := range w.slots {
		if cl := clip.Intersect(pg); !cl.Empty() {
			w.drawPage(a, w.cur+i, pg, cl, true) // notepdf.go
		}
	}
	if all := w.pages(); len(w.slots) > 1 || all.Max.X < w.p.Body.Max.X-kobo.Pad || all.Min.X > w.p.Body.Min.X+kobo.Pad { // two, or narrower than the window: the pages' edges
		for _, pg := range w.slots {
			c.Fill(image.Rect(pg.Min.X-2, pg.Min.Y, pg.Min.X, pg.Max.Y).Intersect(area), kobo.Mid)
			c.Fill(image.Rect(pg.Max.X, pg.Min.Y, pg.Max.X+2, pg.Max.Y).Intersect(area), kobo.Mid)
		}
	}
	w.drawSel(a, clip)
	w.drawPenMenu(a, clip)
}

// strokeLayer is which s is drawn with: the pictures under the
// markers, under the ink.
func strokeLayer(s *noteStroke) int {
	switch {
	case s.pic != nil:
		return 0
	case s.marker:
		return 1
	}
	return 2
}

// toScreen is p of the page at on screen.
func (w *noteWin) toScreen(p notePt, at image.Rectangle) image.Point {
	return image.Pt(at.Min.X+int(float64(p.x)*w.scale+0.5), at.Min.Y+int(float64(p.y)*w.scale+0.5))
}

func (w *noteWin) toScreenRect(r, at image.Rectangle) image.Rectangle {
	return image.Rect(at.Min.X+int(float64(r.Min.X)*w.scale), at.Min.Y+int(float64(r.Min.Y)*w.scale),
		at.Min.X+int(math.Ceil(float64(r.Max.X)*w.scale))+1, at.Min.Y+int(math.Ceil(float64(r.Max.Y)*w.scale))+1)
}

func (w *noteWin) toPage(p image.Point, at image.Rectangle) image.Point {
	return image.Pt(int(float64(p.X-at.Min.X)/w.scale), int(float64(p.Y-at.Min.Y)/w.scale))
}

// radius is the pen's on screen at p.
func (w *noteWin) radius(p notePt) int { return max(1, int(float64(p.r)/10*w.scale+0.5)) }

func (w *noteWin) drawStroke(a *bro, s *noteStroke, at, clip image.Rectangle) {
	if s.pic != nil {
		w.drawPic(a, s, at, clip)
		return
	}
	if s.txt != nil {
		pos := -1
		if k := w.typing; k != nil && k.s == s {
			pos = k.pos
		}
		drawNoteText(a.C, s, at.Min, w.scale, pos, clip)
		return
	}
	prev := w.toScreen(s.pts[0], at)
	a.C.Disc(prev, w.radius(s.pts[0]), s.gray, clip)
	for _, p := range s.pts[1:] {
		q := w.toScreen(p, at)
		a.C.Line(prev, q, w.radius(p), s.gray, clip)
		prev = q
	}
}

// show draws the pages again, and the strip, where they changed.
func (w *noteWin) show(a *bro) {
	if !a.shows(w) {
		return
	}
	w.fitSlots(a)
	w.drawBar(a)
	w.drawPages(a, w.paperArea()) // the pen's choices too, by the pages
	if w.ex != nil {
		a.drawEx(a.exStrip(w.p.Body), w.ex, w.exCmds(a))
	}
	a.Mark(image.Rect(w.p.Body.Min.X, w.bar.Min.Y, w.p.Body.Max.X, w.p.Body.Max.Y), true)
}

// fitSlots lays the pages out again if the page shown is another size
// than they're laid out for (a PDF's).
func (w *noteWin) fitSlots(a *bro) {
	if !w.overview && w.pageSize(w.cur) != w.slotSize {
		w.place(a, w.p)
	}
}

// edited notes a change made: to undo, and to keep.
func (w *noteWin) edited(ops ...noteOp) {
	w.undo = append(w.undo, ops)
	w.redo = nil
	w.changed = time.Now()
}

// turnTo is the page to show first, the pages turned by (on, or back if
// negative); -1 if they can't be. On goes as far as one page past the
// last that's written on: a new one.
func (w *noteWin) turnTo(by int) int {
	n := w.cur + by*w.shown()
	pages := w.nb.pages
	switch {
	case by < 0 && w.cur == 0:
		return -1
	case by < 0:
		return max(0, n)
	case n < len(pages), n == len(pages) && len(pages[n-1]) > 0:
		return n
	}
	return -1
}

func (w *noteWin) turn(a *bro, by int) {
	w.endTyping(a)
	if w.overview {
		w.ovTurn(a, by)
		return
	}
	n := w.turnTo(by)
	if n < 0 {
		return
	}
	w.cur = n
	w.show(a)
	if a.shows(w) {
		a.MarkFlash(w.pages()) // other pages: no ghost of these
	}
}

func (w *noteWin) doUndo(a *bro, undo bool) {
	w.endTyping(a)
	from, to := &w.undo, &w.redo
	if !undo {
		from, to = to, from
	}
	if len(*from) == 0 {
		return
	}
	ops := (*from)[len(*from)-1]
	*from = (*from)[:len(*from)-1]
	w.sel, w.mark = nil, nil // what it took may be gone, or back
	*to = append(*to, ops)
	for i := range ops {
		if undo {
			w.nb.apply(ops[len(ops)-1-i], true)
		} else {
			w.nb.apply(ops[i], false)
		}
	}
	if pg := min(ops[0].page, len(w.nb.pages)-1); w.overview { // where it was: in sight
		w.ovSel = pg
		w.ovShow()
		w.place(a, w.p)
	} else if pg < w.cur || pg >= w.cur+w.shown() {
		w.cur = pg
	}
	w.changed = time.Now()
	w.show(a)
}

const noteClearAsk = 4 * time.Second // how long "clear" waits for its second tap

// clearAsking is whether "clear" was tapped once, and waits.
func (w *noteWin) clearAsking() bool { return time.Since(w.clearAsked) < noteClearAsk }

// clearUnask puts "clear" back as it was, if it was asking.
func (w *noteWin) clearUnask(a *bro) {
	if w.clearAsked.IsZero() {
		return
	}
	w.clearAsked = time.Time{}
	w.showBar(a)
}

// showBar draws the bar again, if the window shows.
func (w *noteWin) showBar(a *bro) {
	if a.shows(w) {
		w.drawBar(a)
		a.Mark(w.bar, true)
	}
}

func (w *noteWin) tap(a *bro, p image.Point) {
	w.endTyping(a)
	if w.ex != nil { // a tap leaves the line
		w.ex = nil
		a.vkAsk(false)
		w.draw(a)
		return
	}
	if kobo.Hit(p, w.counterRect()) {
		w.overviewOn(a, !w.overview)
		return
	}
	for _, b := range w.btns {
		if !kobo.Hit(p, b.r) {
			continue
		}
		if b.id != "clear" {
			w.clearUnask(a)
		}
		switch b.id {
		case "all":
			a.notesFrom(w)
		case "pen", "eraser", "select", "text":
			if b.id == "pen" && !w.erasing && !w.selecting && !w.texting { // in hand: its choices
				w.penMenu = !w.penMenu
				w.show(a)
				break
			}
			w.closePenMenu(a)
			w.setTool(a, b.id)
		case "undo", "redo":
			w.doUndo(a, b.id == "undo")
		case "clear":
			if w.ovOff("clear") {
				break
			}
			if !w.clearAsking() { // it asks first
				asked := time.Now()
				w.clearAsked = asked
				w.showBar(a)
				time.AfterFunc(noteClearAsk, func() {
					a.Do(func() {
						if w.clearAsked == asked {
							w.clearUnask(a)
						}
					})
				})
				break
			}
			w.clearUnask(a)
			w.pageAct(a, "clear")
		case "paper":
			ps := strings.Fields(notePapers)
			w.nb.paper = ps[(slices.Index(ps, w.nb.paper)+1)%len(ps)]
			w.changed = time.Now()
			w.place(a, w.p) // its button says another word
			w.show(a)
		case "paste":
			w.paste(a)
		case "export":
			w.export(a)
		case "open", "new before", "new after", "copy page", "delete", "earlier", "later":
			w.pageAct(a, b.id)
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
	keyS = 31
	keyC = 46
	keyX = 45
	keyD = 32
	keyO = 24
	keyY = 21
	keyV = 47
)

func (w *noteWin) press(a *bro, code uint16, value int32) bool {
	if value == 0 {
		return false
	}
	m := a.tv.mods
	ch, _ := kobo.KeyChar(code, m.shift, m.caps)
	if w.typing != nil && w.typeKey(a, code, ch) {
		return true
	}
	if e := w.ex; e != nil {
		done, ok := exKey(e, code, ch)
		if !done {
			a.drawEx(a.exStrip(w.p.Body), e, w.exCmds(a))
			return true
		}
		w.ex = nil
		a.vkAsk(false)
		if ok {
			a.exRun(w.exCmds(a), e.text, func(text string) { w.exOpen(a, text) })
		}
		if a.shows(w) && !a.overlay() { // :help's menu stays
			w.draw(a)
		}
		return true
	}
	if ch == ":" && !m.ctrl {
		w.exOpen(a, "")
		return true
	}
	if ch == "q" && !m.ctrl { // the notebook put away: All Notebooks
		a.notesFrom(w)
		return true
	}
	if ch == "?" && !m.ctrl {
		a.showSheet(w.Title()+": keys and commands", noteKeys(), w.exCmds(a))
		return true
	}
	if !m.ctrl && code == keyO {
		w.overviewOn(a, !w.overview)
		return true
	}
	if w.overview && !m.ctrl && w.ovKey(a, code) {
		return true
	}
	switch {
	case code == keyLeft || code == keyPgUp || code == keyUp:
		w.turn(a, -1)
	case code == keyRight || code == keyPgDn || code == keyDown:
		w.turn(a, 1)
	case m.ctrl && code == kobo.KeyZ:
		w.doUndo(a, !m.shift)
	case m.ctrl && code == keyY:
		w.doUndo(a, false)
	case m.ctrl && code == keyV:
		w.paste(a)
	case m.ctrl && code == keyE:
		w.export(a)
	case len(w.sel) > 0 && (code == keyDelete || code == kobo.KeyBackspace):
		w.deleteSel(a)
	case len(w.sel) > 0 && m.ctrl && (code == keyC || code == keyX || code == keyD):
		w.selAct(a, map[uint16]string{keyC: "copy", keyX: "cut", keyD: "duplicate"}[code])
	case value == 1 && !m.ctrl && (code == keyE || code == keyP || code == keyS || code == keyT):
		w.setTool(a, map[uint16]string{keyE: "eraser", keyP: "pen", keyS: "select", keyT: "text"}[code])
	default:
		return false
	}
	return true
}

func (w *noteWin) closing(a *bro) {
	w.endTyping(a)
	if w.ex != nil {
		w.ex = nil
		a.vkAsk(false)
	}
	if a.nt.ink != nil && a.nt.ink.w == w {
		a.nt.ink = nil
		a.Ink(false)
	}
	if d := a.nt.drag; d != nil && d.w == w {
		a.nt.drag = nil
		a.Ink(false)
	}
	if !w.keep(a) {
		a.nt.unsaved = append(a.nt.unsaved, w.nb)
		a.Notify("notebook " + w.nb.name + " not kept: it's held, and kept once it can be")
	}
	w.closePDF()
	w.place(a, kobo.Panel{})
}

// keep writes the notebook, if it changed; false if it couldn't, and
// it's tried again in a while.
func (w *noteWin) keep(a *bro) bool {
	if w.changed.IsZero() {
		return true
	}
	name := w.nb.name
	err := w.nb.save()
	if w.nb.name != name { // its file changed meanwhile: this one's beside it now
		a.Notify(name + " changed on the computer: this is " + path.Base(w.nb.name) + " now, beside it")
		w.drawTitle(a)
		a.wm.changed = time.Now()
	}
	if err != nil {
		log.Printf("notebook %s: %v", w.nb.name, err)
		if err.Error() != w.keepErr { // said once, not every try
			a.Notify("notebook not kept: " + err.Error())
		}
		w.changed, w.keepErr = time.Now(), err.Error()
		return false
	}
	w.changed, w.keepErr = time.Time{}, ""
	log.Printf("notebook %s kept", w.nb.name)
	return true
}

// unsavedNote is the notebook name closed unkept, if it's one.
func (a *bro) unsavedNote(name string) *notebook {
	for _, nb := range a.nt.unsaved {
		if nb.name == name {
			return nb
		}
	}
	return nil
}

// keepUnsaved tries the notebooks closed unkept again.
func (a *bro) keepUnsaved() {
	a.nt.triedAt = time.Now()
	a.nt.unsaved = slices.DeleteFunc(a.nt.unsaved, func(nb *notebook) bool {
		if err := nb.save(); err != nil {
			log.Printf("notebook %s: still not kept: %v", nb.name, err)
			return false
		}
		log.Printf("notebook %s kept, at last", nb.name)
		return true
	})
}

// --- the pen and fingers ---

// noteTouch takes a contact coming down on a notebook's page: the pen
// writes or erases, a finger may turn the page.
func (a *bro) noteTouch(ev kobo.TouchEv, p image.Point, g *kobo.Gesture) bool {
	w, ok := a.windowAt(p).(*noteWin)
	if !ok || !a.shows(w) {
		return false
	}
	if w.penMenu { // its choices: by the edge too
		if w.penMenuTap(a, p) {
			g.Owner = ownLeaf
			return true
		}
		w.closePenMenu(a)
	}
	if p.In(w.paperArea()) {
		w.clearUnask(a)
	}
	if w.overview { // the pages, small: a tap or a swipe
		if !p.In(w.paperArea()) || !ev.Pen && !a.Full && (p.X < kobo.EdgeWidth || p.X >= a.C.Bounds().Dx()-kobo.EdgeWidth) {
			return false
		}
		g.Owner = ownNoteOv
		return true
	}
	if w.selecting && p.In(w.selArea(a)) { // what's taken: its dots and buttons, by the edges too
		return a.noteSelTouch(w, -1, p, g)
	}
	slot := slices.IndexFunc(w.slots, func(r image.Rectangle) bool { return p.In(r) })
	if slot < 0 {
		return false
	}
	if ev.Pen && !a.mem.Pen {
		log.Print("notebook: a pen, so fingers turn pages from now on")
		a.mem.Pen = true
		a.mem.save()
	}
	if !ev.Pen && !a.Full && (p.X < kobo.EdgeWidth || p.X >= a.C.Bounds().Dx()-kobo.EdgeWidth) { // the edges' swipes (none full screen)
		return false
	}
	if !ev.Pen && !a.vk.on && p.Y >= a.C.Bounds().Max.Y-vkFoot { // the foot's: two fingers there bring the keyboard
		return false
	}
	if w.selecting {
		return a.noteSelTouch(w, slot, p, g)
	}
	if w.texting { // a tap types there (noteLift); a finger swiped turns the page
		g.Owner = ownLeaf
		return true
	}
	if !ev.Pen && a.fingerTurns() {
		g.Owner = ownLeaf
		return true
	}
	if a.nt.ink != nil {
		return false
	}
	g.Owner = ownInk
	k := &noteInk{w: w, g: g, erase: w.erasing || ev.Button, page: w.cur + slot, at: w.slots[slot], last: p}
	a.nt.ink = k
	if k.erase {
		a.noteErase(k, p)
		return true
	}
	k.s = a.newStroke()
	a.Ink(true)
	a.noteInkTo(k, p, a.notePressure(ev))
	return true
}

// fingerTurns is whether a finger on a notebook's page turns it,
// rather than writing: as the settings say, or by itself once a pen has
// written (a stylus without a battery is a finger to the screen).
func (a *bro) fingerTurns() bool {
	switch a.setting("note-finger") {
	case "writes":
		return false
	case "turns":
		return true
	}
	return a.mem.Pen
}

// notePressure is how hard the pen pressed; a finger's is a middling
// line, whatever it says.
func (a *bro) notePressure(ev kobo.TouchEv) int32 {
	if !ev.Pen {
		return a.PressureMax() * 2 / 5
	}
	return ev.Pressure
}

// noteMove follows the pen.
func (a *bro) noteMove(ev kobo.TouchEv, p image.Point, g *kobo.Gesture) bool {
	k := a.nt.ink
	switch g.Owner {
	case ownLeaf, ownNoteOv:
		return true
	case ownNoteSel:
		a.noteDragTo(p, g)
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
	a.noteInkTo(k, p, a.notePressure(ev))
	return true
}

// noteInkTo adds p to the stroke, and shows it at once: black on white,
// the fast waveform. Off its page it goes on, unseen, as a pen goes off
// the paper's edge.
func (a *bro) noteInkTo(k *noteInk, p image.Point, pressure int32) {
	w := k.w
	pp := w.toPage(p, k.at)
	frac := math.Min(1, float64(pressure)/float64(a.PressureMax()))
	pt := notePt{int16(pp.X), int16(pp.Y), a.penRadius(frac)}
	k.s.pts = append(k.s.pts, pt)
	r := w.radius(pt)
	from := k.last
	if len(k.s.pts) == 1 {
		from = p
	}
	ink := uint8(kobo.Black)
	if k.s.gray != kobo.Black { // the fast waveform has no grays: dark till it lifts
		ink = kobo.Dark
	}
	a.C.Line(from, p, r, ink, k.at)
	box := image.Rectangle{from, from}.Union(image.Rectangle{p, p}).Inset(-r - 1).Intersect(k.at)
	k.marked = k.marked.Union(box)
	k.last = p
	a.Mark(box, false)
}

// noteErase takes the strokes the eraser touches at p off the page.
func (a *bro) noteErase(k *noteInk, p image.Point) {
	w := k.w
	pp := w.toPage(p, k.at)
	var gone []*noteStroke
	for _, s := range w.strokes(k.page) {
		if s.hits(pp) {
			gone = append(gone, s)
		}
	}
	if len(gone) == 0 {
		return
	}
	w.nb.apply(noteOp{page: k.page, del: gone}, false)
	k.del = append(k.del, gone...)
	var r image.Rectangle
	for _, s := range gone {
		r = r.Union(w.toScreenRect(s.box, k.at))
	}
	w.drawPages(a, r)
	a.Mark(r.Intersect(k.at), true)
}

// noteLift ends what the pen or the finger did.
func (a *bro) noteLift(g *kobo.Gesture) bool {
	switch g.Owner {
	case ownLeaf:
		if w, ok := a.windowAt(g.Start).(*noteWin); ok {
			a.noteFocus(w)
			dx, dy := g.Last.X-g.Start.X, g.Last.Y-g.Start.Y
			if i := slices.IndexFunc(w.slots, func(r image.Rectangle) bool { return g.Start.In(r) }); w.texting && i >= 0 && kobo.Abs(dx) < noteSwipe && kobo.Abs(dy) < noteSwipe {
				w.textAt(a, w.cur+i, w.toPage(g.Start, w.slots[i]))
			} else if kobo.Abs(dx) > noteSwipe && kobo.Abs(dx) > 2*kobo.Abs(dy) {
				by := 1 // leftward: the next, as pages turn
				if dx > 0 {
					by = -1
				}
				w.turn(a, by)
			}
		}
		return true
	case ownNoteSel:
		a.noteDragEnd(g)
		return true
	case ownNoteOv:
		if w, ok := a.windowAt(g.Start).(*noteWin); ok && w.overview {
			a.noteFocus(w)
			dx, dy := g.Last.X-g.Start.X, g.Last.Y-g.Start.Y
			switch {
			case kobo.Abs(dx) > noteSwipe && kobo.Abs(dx) > 2*kobo.Abs(dy):
				w.ovTurn(a, map[bool]int{true: 1, false: -1}[dx < 0])
			case g.Moved < noteSelStill:
				w.ovTap(a, g.Last)
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
	a.Ink(false)
	w := k.w
	switch {
	case k.erase && len(k.del) > 0:
		w.edited(noteOp{page: k.page, del: k.del})
	case !k.erase && len(k.s.pts) > 0:
		k.s.bound()
		for len(w.nb.pages) <= k.page { // written on a page that wasn't yet
			w.nb.pages = append(w.nb.pages, nil)
		}
		w.nb.pages[k.page] = append(w.nb.pages[k.page], k.s)
		w.edited(noteOp{page: k.page, add: []*noteStroke{k.s}})
		if k.s.gray != kobo.Black { // in its own gray, a marker under the ink
			w.drawPages(a, k.marked)
		}
		a.Mark(k.marked, true) // the cleanup: grays (the paper) back under it
	}
	if a.shows(w) {
		w.drawBar(a) // undo can now, and there may be a page more
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
	if nb := a.unsavedNote(name); nb != nil { // newer than its file
		a.nt.unsaved = slices.DeleteFunc(a.nt.unsaved, func(n *notebook) bool { return n == nb })
		return &noteWin{nb: nb, changed: time.Now()}
	}
	nb, err := loadNotebook(name)
	if err != nil {
		log.Printf("notebook %s: %v", name, err)
		a.Notify("can't read notebook " + name + ": " + err.Error())
		return nil
	}
	w := &noteWin{nb: nb}
	if nb.old != "" { // as .xopp, soon
		w.changed = time.Now()
	}
	return w
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
	if a.nt.ink != nil || a.nt.drag != nil {
		return
	}
	for _, w := range a.noteWins() {
		if !w.changed.IsZero() && time.Since(w.changed) > noteKeepAfter {
			w.keep(a)
		}
	}
	if len(a.nt.unsaved) > 0 && time.Since(a.nt.triedAt) > noteKeepAfter {
		a.keepUnsaved()
	}
	if time.Since(a.nt.lookedAt) > noteLookEvery {
		a.nt.lookedAt = time.Now()
		for _, w := range a.noteWins() {
			if w.changed.IsZero() && w.readAgain(a) {
				a.Notify(path.Base(w.nb.name) + ": changed on the computer, read again")
			}
		}
	}
}

// noteLookEvery is how often an open notebook's file is looked at, for
// a change from a computer.
const noteLookEvery = 3 * time.Second

// readAgain reads w's notebook again if its file changed (w has
// nothing unkept): written on a computer meanwhile, that's what it is
// now. True if it did.
func (w *noteWin) readAgain(a *bro) bool {
	st, ok := stampOf(notePath(w.nb.name))
	if !ok || st == w.nb.disk {
		return false
	}
	nb, err := loadNotebook(w.nb.name)
	if err != nil {
		log.Printf("notebook %s: changed, but: %v", w.nb.name, err)
		w.nb.disk = st // not again till it changes again
		return false
	}
	log.Printf("notebook %s: changed on its file, read again", nb.name)
	w.nb = nb
	w.undo, w.redo, w.sel, w.mark = nil, nil, nil, nil
	w.cur, w.ovSel = min(w.cur, len(nb.pages)-1), min(w.ovSel, len(nb.pages)-1)
	w.closePDF() // its background may be another now
	w.place(a, w.p)
	if a.shows(w) {
		w.draw(a)
	}
	return true
}

func (a *bro) noteExit() {
	for _, w := range a.noteWins() {
		w.keep(a)
	}
	a.keepUnsaved()
}
