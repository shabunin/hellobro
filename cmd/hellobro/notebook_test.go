package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"hellobro/kobo"
	"hellobro/wm"
)

func TestNotebookOldFile(t *testing.T) {
	in := "hellobro notebook 1\npaper lines\npage\ns 10,20,15 30,40,25\ns 5,5,40\npage\npage\ns 1,2,3\npage\npage\n"
	nb, err := parseHbnote("x", strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if nb.paper != "lines" || len(nb.pages) != 5 || len(nb.pages[0]) != 2 || len(nb.pages[2]) != 1 {
		t.Fatalf("paper %q, pages %d", nb.paper, len(nb.pages))
	}
	if s := nb.pages[0][0]; s.pts[1] != (notePt{30, 40, 25}) || s.box != image.Rect(8, 18, 34, 44) {
		t.Errorf("stroke %v, box %v", s.pts, s.box)
	}
	for _, bad := range []string{"", "a shopping list\n", "hellobro notebook 1\ns 1,2\n", "hellobro notebook 1\ns 1,x,3\n"} {
		if _, err := parseHbnote("x", strings.NewReader(bad)); err == nil {
			t.Errorf("%q: no error", bad)
		}
	}
}

func TestNotebookXopp(t *testing.T) {
	nb := &notebook{name: "x", paper: "lines", pages: [][]*noteStroke{
		{{pts: []notePt{{10, 20, 15}, {30, 40, 25}, {1400, 1870, 40}}}, {pts: []notePt{{5, 5, 40}}}},
		nil,
		{{pts: []notePt{{-3, 2000, 20}, {7, 8, 21}}}},
		nil, nil,
	}}
	b, err := nb.xopp()
	if err != nil {
		t.Fatal(err)
	}
	back, err := parseXopp("x", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if back.paper != "lines" || len(back.pages) != 3 {
		t.Fatalf("paper %q, %d pages (the empty ones at the end go)", back.paper, len(back.pages))
	}
	for i, pg := range nb.pages[:3] {
		for j, s := range pg {
			want := s.pts
			if len(want) == 1 { // a dot is two points to Xournal++
				want = append(want, want[0])
			}
			if got := back.pages[i][j].pts; !slices.Equal(got, want) {
				t.Errorf("page %d stroke %d: %v, want %v", i, j, got, want)
			}
		}
	}
	text := string(nb.xoppText())
	for _, want := range []string{`<xournal creator="hellobro"`, `style="ruled"`, `tool="pen"`, `width="0.95 1.59 2.54"`} {
		if !strings.Contains(text, want) {
			t.Errorf("no %s in\n%s", want, text)
		}
	}
}

// What Xournal++ wrote, with what we don't draw: it comes back as it was
// (its text, read and written again).
func TestNotebookXoppKept(t *testing.T) {
	in := `<?xml version="1.0" standalone="no"?>
<xournal creator="Xournal++ 1.2.3" fileversion="4"><title>t</title><preview>AAAA</preview>
<page width="595.28" height="841.89"><background type="solid" color="#ffffffff" style="graph"/>
<layer><stroke tool="pen" ts="0" fn="" color="#ff0000ff" width="2.26 1.5 2.0">10 20 30 40 50 60</stroke>
<stroke tool="highlighter" color="#ffff0080" width="8">1 2 3 4</stroke>
<text font="Sans" size="12" x="100" y="100" color="#000000ff" ts="0" fn="">x² + 1 &amp; so</text></layer>
<layer><stroke tool="pen" color="#000000ff" width="1">5 5 6 6</stroke></layer></page>
<page width="595.28" height="841.89"><background type="pdf" pageno="1" domain="absolute" filename="a.pdf"/><layer/></page>
</xournal>`
	var gz bytes.Buffer
	z := gzip.NewWriter(&gz)
	z.Write([]byte(in))
	z.Close()
	for _, src := range [][]byte{[]byte(in), gz.Bytes()} {
		nb, err := parseXopp("x", bytes.NewReader(src))
		if err != nil {
			t.Fatal(err)
		}
		if nb.paper != "squares" || len(nb.pages) != 2 || len(nb.pages[0]) != 4 || len(nb.meta[0].extra) != 0 { // the text read (notetext.go)
			t.Fatalf("paper %q, %d pages, %d strokes, %d kept", nb.paper, len(nb.pages), len(nb.pages[0]), len(nb.meta[0].extra))
		}
		if p := nb.pages[0][0].pts[2]; p != (notePt{158, 189, 32}) {
			t.Errorf("a point: %v", p)
		}
		if red, hl := nb.pages[0][0], nb.pages[0][1]; red.gray != 0x4C || red.marker || !hl.marker || hl.gray != noteMarker {
			t.Errorf("the red pen %#x, the highlighter %#x %v", red.gray, hl.gray, hl.marker)
		}
		nb.paper = "plain"
		out := string(nb.xoppText())
		for _, want := range []string{`creator="Xournal++ 1.2.3"`, `color="#ff0000ff"`, `tool="highlighter"`, `x² + 1 &amp; so`,
			`width="595.28" height="841.89"`, `style="plain"`, `type="pdf" pageno="1"`} {
			if !strings.Contains(out, want) {
				t.Errorf("no %s in\n%s", want, out)
			}
		}
		if strings.Contains(out, `filename="a.pdf" style`) {
			t.Error("a PDF's background got a style")
		}
	}
	for _, bad := range []string{"", "hellobro notebook 1\n", `<xournal><page><layer><stroke tool="pen" width="1">1 x</stroke></layer></page></xournal>`} {
		if _, err := parseXopp("x", strings.NewReader(bad)); err == nil {
			t.Errorf("%q: no error", bad)
		}
	}
}

func TestNotebookUndo(t *testing.T) {
	a, b, c := &noteStroke{}, &noteStroke{}, &noteStroke{}
	nb := &notebook{pages: [][]*noteStroke{{a, b}}}
	add := noteOp{page: 0, add: []*noteStroke{c}}
	del := noteOp{page: 0, del: []*noteStroke{a, c}}
	nb.apply(add, false)
	nb.apply(del, false)
	if pg := nb.pages[0]; len(pg) != 1 || pg[0] != b {
		t.Fatalf("after: %v", pg)
	}
	nb.apply(del, true)
	nb.apply(add, true)
	if pg := nb.pages[0]; len(pg) != 2 || pg[0] != b || pg[1] != a {
		t.Fatalf("undone: %v", pg)
	}
}

func TestNoteEraser(t *testing.T) {
	s := &noteStroke{pts: []notePt{{100, 100, 20}, {300, 100, 20}}}
	s.bound()
	for _, tc := range []struct {
		p    image.Point
		want bool
	}{
		{image.Pt(200, 100), true},
		{image.Pt(200, 115), true},
		{image.Pt(200, 130), false},
		{image.Pt(90, 100), true},
		{image.Pt(60, 100), false},
		{image.Pt(320, 110), false},
	} {
		if got := s.hits(tc.p); got != tc.want {
			t.Errorf("%v: %v", tc.p, got)
		}
	}
	one := &noteStroke{pts: []notePt{{50, 50, 15}}}
	one.bound()
	if !one.hits(image.Pt(55, 55)) {
		t.Error("a dot isn't erased")
	}
}

func TestNewNoteName(t *testing.T) {
	now := time.Date(2026, 10, 28, 14, 5, 0, 0, time.UTC)
	taken := map[string]bool{"2026-10-28 14.05": true, "2026-10-28 14.05 (2)": true}
	if got := newNoteName(now, func(n string) bool { return taken[n] }); got != "2026-10-28 14.05 (3)" {
		t.Errorf("got %q", got)
	}
}

func TestNoteSlots(t *testing.T) {
	for _, tc := range []struct {
		area image.Rectangle
		n    int
	}{
		{image.Rect(0, 0, 1388, 1760), 1}, // portrait, fullscreen
		{image.Rect(0, 0, 1856, 1290), 2}, // landscape, fullscreen
		{image.Rect(0, 0, 920, 1290), 1},  // landscape, half
		{image.Rect(0, 0, 1388, 860), 2},  // portrait, the top half: wide
		{image.Rect(0, 0, 1388, 1500), 1}, // a little short
	} {
		slots, scale := noteSlots(tc.area, image.Pt(notePageW, notePageH))
		if len(slots) != tc.n {
			t.Errorf("%v: %d pages, want %d (scale %.2f)", tc.area, len(slots), tc.n, scale)
		}
		for _, s := range slots {
			if !s.In(tc.area) {
				t.Errorf("%v: a page at %v", tc.area, s)
			}
		}
		if len(slots) == 2 && slots[0].Overlaps(slots[1]) {
			t.Errorf("%v: the pages overlap", tc.area)
		}
	}
}

func TestNoteTurn(t *testing.T) {
	s := &noteStroke{}
	for _, tc := range []struct {
		pages      [][]*noteStroke
		cur, shown int
		by, want   int
	}{
		{[][]*noteStroke{{s}}, 0, 1, 1, 1},           // past the last, written on: a new page
		{[][]*noteStroke{{s}, nil}, 1, 1, 1, -1},     // past a blank one: no
		{[][]*noteStroke{{s}, nil}, 1, 1, -1, 0},     // back
		{[][]*noteStroke{nil}, 0, 1, -1, -1},         // before the first
		{[][]*noteStroke{{s}}, 0, 2, 1, -1},          // two shown, the second not written on
		{[][]*noteStroke{{s}, {s}}, 0, 2, 1, 2},      // both written on: the next two
		{[][]*noteStroke{{s}, {s}, {s}}, 0, 2, 1, 2}, // the third and a new one
		{[][]*noteStroke{{s}, {s}, {s}}, 1, 2, -1, 0},
	} {
		w := &noteWin{nb: &notebook{pages: tc.pages}, cur: tc.cur}
		for range tc.shown {
			w.slots = append(w.slots, image.Rect(0, 0, 1, 1))
		}
		if got := w.turnTo(tc.by); got != tc.want {
			t.Errorf("%d pages, at %d, %d shown, by %d: %d, want %d", len(tc.pages), tc.cur, tc.shown, tc.by, got, tc.want)
		}
	}
}

func TestNotebookMoves(t *testing.T) {
	root, old := t.TempDir(), t.TempDir()
	defer func(r string, o func() string) { shareRoot, oldNotesDir = r, o }(shareRoot, oldNotesDir)
	shareRoot, oldNotesDir = root, func() string { return old }
	os.WriteFile(filepath.Join(old, "sums.hbnote"), []byte("hellobro notebook 1\npaper plain\npage\ns 1,2,15 3,4,15\n"), 0o644)
	if got := noteNames(); !slices.Equal(got, []string{"sums"}) {
		t.Fatalf("names: %v", got)
	}
	nb, err := loadNotebook("sums")
	if err != nil || nb.old == "" || len(nb.pages[0]) != 1 {
		t.Fatalf("%v, %+v", err, nb)
	}
	if err := nb.save(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(old, "sums.hbnote")); !os.IsNotExist(err) {
		t.Errorf("the old one's still there: %v", err)
	}
	nb, err = loadNotebook("sums")
	if err != nil || nb.old != "" || nb.paper != "plain" || len(nb.pages[0]) != 1 {
		t.Fatalf("as .xopp: %v, %+v", err, nb)
	}
	if got := noteNames(); !slices.Equal(got, []string{"sums"}) {
		t.Errorf("names after: %v", got)
	}
	if _, err := os.Stat(filepath.Join(root, "Notebooks", "sums.xopp")); err != nil {
		t.Error(err)
	}
}

func TestNoteEntries(t *testing.T) {
	root, old := t.TempDir(), t.TempDir()
	defer func(r string, o func() string) { shareRoot, oldNotesDir = r, o }(shareRoot, oldNotesDir)
	shareRoot, oldNotesDir = root, func() string { return old }
	nd := filepath.Join(root, "Notebooks")
	for _, d := range []string{"Work/2026", "Empty", ".trash"} {
		os.MkdirAll(filepath.Join(nd, d), 0o755)
	}
	for _, f := range []string{"plan.xopp", "Work/ideas.xopp", "Work/2026/q4.xopp", ".trash/gone.xopp", "plan.pdf"} {
		os.WriteFile(filepath.Join(nd, f), []byte("x"), 0o644)
	}
	os.WriteFile(filepath.Join(old, "sums.hbnote"), []byte("x"), 0o644)
	es, dirs := noteEntries()
	var keys []string
	for _, e := range es {
		keys = append(keys, e.key)
	}
	slices.Sort(keys)
	if want := []string{"Work/2026/q4", "Work/ideas", "plan", "sums"}; !slices.Equal(keys, want) {
		t.Errorf("notebooks %q, want %q", keys, want)
	}
	slices.Sort(dirs)
	if want := []string{"Empty", "Work", "Work/2026"}; !slices.Equal(dirs, want) {
		t.Errorf("folders %q, want %q", dirs, want)
	}
	// one in a folder is written there
	nb := &notebook{name: "New/first", paper: "plain", pages: [][]*noteStroke{nil}}
	if err := nb.save(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(nd, "New", "first.xopp")); err != nil {
		t.Error(err)
	}
}

func TestNoteNameOK(t *testing.T) {
	for name, ok := range map[string]bool{"plan": true, "2026-10-29 14.05": true, "a b (2)": true, "": false, "a/b": false, `a\b`: false, ".hidden": false, "what?": false} {
		if err := noteNameOK(name); (err == nil) != ok {
			t.Errorf("%q: %v", name, err)
		}
	}
}

// A notebook's file written on a computer meanwhile is never written
// over: unchanged here, it's read again; changed here too, this one goes
// beside it.
func TestNoteChangedOnDisk(t *testing.T) {
	defer func(r string) { shareRoot = r }(shareRoot)
	shareRoot = t.TempDir()
	a := &bro{App: &kobo.App{F: kobo.LoadFonts()}}
	a.wm.tree = wm.New(image.Rect(0, 0, 1000, 800), kobo.FoldH)
	a.wm.wins = map[*wm.Node]window{}
	nb := &notebook{name: "plan", paper: "plain", pages: [][]*noteStroke{nil}}
	if err := nb.save(); err != nil {
		t.Fatal(err)
	}
	ink := func() []*noteStroke {
		s := &noteStroke{pts: []notePt{{10, 10, 20}, {50, 50, 20}}}
		s.bound()
		return []*noteStroke{s}
	}
	pc := &notebook{name: "plan", paper: "plain", pages: [][]*noteStroke{ink(), ink(), ink()}} // the computer's: three pages
	pcWrite := func() {
		b, _ := pc.xopp()
		os.WriteFile(notePath("plan"), b, 0o644)
		os.Chtimes(notePath("plan"), time.Now(), time.Now().Add(time.Minute))
	}
	pcWrite()
	w := &noteWin{nb: nb}
	w.readAgain(a)
	if len(w.nb.pages) != 3 {
		t.Fatalf("not read again: %d pages", len(w.nb.pages))
	}
	w.nb.pages = append(w.nb.pages, ink()) // changed here
	pc.pages = pc.pages[:2]
	pcWrite() // and there
	if err := w.nb.save(); err != nil {
		t.Fatal(err)
	}
	if w.nb.name != "plan (Kobo)" {
		t.Fatalf("written as %q", w.nb.name)
	}
	there, _ := loadNotebook("plan")
	here, _ := loadNotebook("plan (Kobo)")
	if len(there.pages) != 2 || len(here.pages) != 4 {
		t.Errorf("there %d pages, here %d", len(there.pages), len(here.pages))
	}
	if err := w.nb.save(); err != nil || w.nb.name != "plan (Kobo)" { // its own file now: written over
		t.Errorf("again: %q, %v", w.nb.name, err)
	}
}

// A finger turns a notebook's pages, or writes: as the settings say, or
// once a pen has written.
func TestFingerTurns(t *testing.T) {
	for _, c := range []struct {
		set  string
		pen  bool
		want bool
	}{
		{"auto", false, false}, {"auto", true, true},
		{"writes", false, false}, {"writes", true, false},
		{"turns", false, true}, {"turns", true, true},
	} {
		st := settingList[settingIndex("note-finger")]
		a := &bro{set: settings{"note-finger": slices.Index(st.values, c.set)}}
		a.mem.Pen = c.pen
		if got := a.fingerTurns(); got != c.want {
			t.Errorf("%s, pen %v: turns %v, want %v", c.set, c.pen, got, c.want)
		}
	}
}

// The notebook's strip fits its window, however narrow, with fingers'
// sizes too: along the top, or down the side where the top hasn't room
// for all the tools (then all of them show). Nothing overlaps or goes
// past its end; the pages and the pen's choices are beside it.
func TestNoteBarFits(t *testing.T) {
	a := &bro{App: &kobo.App{F: kobo.LoadFonts()}}
	defer kobo.SetBig(false)
	for _, big := range []bool{false, true} {
		kobo.SetBig(big)
		for _, ht := range []int{500, 1700} {
			for wd := 300; wd <= 1400; wd += 25 {
				for _, ov := range []bool{false, true} {
					w := &noteWin{nb: &notebook{pages: [][]*noteStroke{nil}}, overview: ov, optsOf: "pen"}
					body := image.Rect(0, 0, wd, ht)
					w.place(a, kobo.Panel{Outer: body, Body: body})
					at := fmt.Sprintf("%dx%d (big %v, overview %v, side %v)", wd, ht, big, ov, w.side)
					if want := ht == 1700 && wd <= 600; ht == 1700 && (wd <= 600 || wd == 1400) && w.side != want {
						t.Errorf("%s: down the side %v, want %v", at, w.side, want)
					}
					tools, _ := barTools(ov)
					if w.side && ht > 1000 && len(w.btns) != len(tools)+2 {
						t.Errorf("%s: %d buttons down the side, want all %d", at, len(w.btns), len(tools)+2)
					}
					for i, b := range w.btns {
						if !b.r.In(w.bar) {
							t.Errorf("%s: %s at %v, past the strip %v", at, b.id, b.r, w.bar)
						}
						for _, c := range w.btns[i+1:] {
							if b.r.Overlaps(c.r) {
								t.Errorf("%s: %s and %s overlap", at, b.id, c.id)
							}
						}
					}
					if r := w.counterRect(); len(w.btns) >= 2 && (r.Dx() < 20 || r.Dy() < 20) {
						t.Errorf("%s: the counter is %v", at, r)
					}
					for _, s := range w.slots {
						if !s.In(w.paperArea()) || s.Overlaps(w.bar) {
							t.Errorf("%s: a page at %v, not beside the strip %v", at, s, w.bar)
						}
					}
					for _, o := range w.opts {
						if o.r.Max.X > w.paperArea().Max.X || o.r.Overlaps(w.bar) {
							t.Errorf("%s: the pen's %s at %v, out of %v", at, o.id, o.r, w.paperArea())
						}
					}
				}
			}
		}
	}
}
