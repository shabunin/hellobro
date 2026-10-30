package main

import (
	"bytes"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
	"time"

	"hellobro/kobo"
	"hellobro/wm"
)

func TestImagesPDF(t *testing.T) {
	ink := &noteStroke{pts: []notePt{{100, 100, 30}, {600, 100, 30}}, gray: 0}
	ink.bound()
	pg := exportPage{size: image.Pt(notePageW, notePageH), pt: [2]float64{445.39, 593.86}, strokes: []*noteStroke{ink}, pdfPage: -1}
	m, err := renderNotePage(pg, "lines", nil, 100)
	if err != nil {
		t.Fatal(err)
	}
	if m.Bounds().Size() != image.Pt(619, 825) || m.GrayAt(150, 44).Y != 0 || m.GrayAt(150, 60).Y != 0xFF || m.GrayAt(150, 39).Y != noteExportRule {
		t.Errorf("drawn %v: ink %d, paper %d, a line %d", m.Bounds(), m.GrayAt(150, 44).Y, m.GrayAt(150, 60).Y, m.GrayAt(150, 39).Y)
	}
	b := imagesPDF([]pdfImagePage{{m, 445.39, 593.86}, {m, 445.39, 593.86}})
	if !bytes.HasPrefix(b, []byte("%PDF-1.4")) || !bytes.HasSuffix(b, []byte("%%EOF\n")) || !bytes.Contains(b, []byte("/Count 2")) {
		t.Fatalf("not a PDF of two pages")
	}
	// each object where the xref says
	xref := regexp.MustCompile(`startxref\n(\d+)`).FindSubmatch(b)
	at, _ := strconv.Atoi(string(xref[1]))
	entries := regexp.MustCompile(`(\d{10}) 00000 n`).FindAllSubmatch(b[at:], -1)
	if len(entries) != 8 {
		t.Fatalf("%d objects", len(entries))
	}
	for i, e := range entries {
		off, _ := strconv.Atoi(string(e[1]))
		if want := fmt.Sprintf("%d 0 obj", i+1); !bytes.HasPrefix(b[off:], []byte(want)) {
			t.Errorf("object %d: at %d is %q", i+1, off, b[off:off+12])
		}
	}
}

func TestExportPath(t *testing.T) {
	defer func(r string) { shareRoot = r }(shareRoot)
	shareRoot = t.TempDir()
	nd := filepath.Join(shareRoot, "Notebooks")
	os.MkdirAll(nd, 0o755)
	if got := exportPath("lecture"); got != filepath.Join(nd, "lecture.pdf") {
		t.Errorf("none there: %s", got)
	}
	// someone else's PDF is left be
	os.WriteFile(filepath.Join(nd, "lecture.pdf"), []byte("%PDF-1.7\nslides"), 0o644)
	if got := exportPath("lecture"); got != filepath.Join(nd, "lecture (notebook).pdf") {
		t.Errorf("beside a PDF: %s", got)
	}
	// ours is written over
	m := image.NewGray(image.Rect(0, 0, 4, 4))
	writeAtomic(filepath.Join(nd, "plan.pdf"), imagesPDF([]pdfImagePage{{m, 10, 10}}))
	if got := exportPath("plan"); got != filepath.Join(nd, "plan.pdf") {
		t.Errorf("an export there: %s", got)
	}
	if _, err := os.Stat(filepath.Join(nd, "plan.pdf.new")); !os.IsNotExist(err) {
		t.Errorf("the new file's left: %v", err)
	}
	// renamed, a notebook takes only its own export along
	a := &bro{}
	a.wm.tree = wm.New(image.Rect(0, 0, 1000, 800), kobo.FoldH)
	for _, n := range []string{"lecture", "plan"} {
		nb := &notebook{name: n, paper: "plain", pages: [][]*noteStroke{nil}}
		if err := nb.save(); err != nil {
			t.Fatal(err)
		}
	}
	if err := renameNotebook(a, "lecture", "talk"); err != nil {
		t.Fatal(err)
	}
	if err := renameNotebook(a, "plan", "plans"); err != nil {
		t.Fatal(err)
	}
	for f, want := range map[string]bool{"lecture.pdf": true, "talk.pdf": false, "talk.xopp": true, "plan.pdf": false, "plans.pdf": true} {
		if fileExists(filepath.Join(nd, f)) != want {
			t.Errorf("%s there: %v, want %v", f, !want, want)
		}
	}
}

func TestKeepUnsaved(t *testing.T) {
	defer func(r string) { shareRoot = r }(shareRoot)
	shareRoot = t.TempDir()
	nd := filepath.Join(shareRoot, "Notebooks")
	os.WriteFile(nd, []byte("in the way"), 0o644) // can't be written
	a := &bro{}
	a.wm.tree = wm.New(image.Rect(0, 0, 1000, 800), kobo.FoldH)
	a.nt.unsaved = []*notebook{{name: "plan", paper: "plain", pages: [][]*noteStroke{nil}}}
	a.keepUnsaved()
	if len(a.nt.unsaved) != 1 {
		t.Fatalf("let go of, unwritten")
	}
	if renameNotebook(a, "plan", "plans"); a.nt.unsaved[0].name != "plans" {
		t.Errorf("renamed: %s", a.nt.unsaved[0].name)
	}
	os.Remove(nd)
	a.keepUnsaved()
	if len(a.nt.unsaved) != 0 || !fileExists(filepath.Join(nd, "plans.xopp")) {
		t.Errorf("not kept: %d left", len(a.nt.unsaved))
	}
}

// TestNoteMove: a notebook goes into a folder with its export; a folder
// out of the one it's in, with what's in it; not into itself.
func TestNoteMove(t *testing.T) {
	defer func(r string) { shareRoot = r }(shareRoot)
	shareRoot = t.TempDir()
	nd := filepath.Join(shareRoot, "Notebooks")
	os.MkdirAll(filepath.Join(nd, "Work", "Old"), 0o755)
	a := &bro{}
	a.wm.tree = wm.New(image.Rect(0, 0, 1000, 800), kobo.FoldH)
	for _, n := range []string{"plan", "Work/Old/notes"} {
		nb := &notebook{name: n, paper: "plain", pages: [][]*noteStroke{nil}}
		if err := nb.save(); err != nil {
			t.Fatal(err)
		}
	}
	writeAtomic(filepath.Join(nd, "plan.pdf"), imagesPDF([]pdfImagePage{{image.NewGray(image.Rect(0, 0, 4, 4)), 10, 10}}))
	w := &notesWin{back: "plan"}
	if err := w.moveTo(a, "plan", false, "Work/plan"); err != nil {
		t.Fatal(err)
	}
	if err := w.moveTo(a, "Work", true, "Work/Old/Work"); err == nil {
		t.Errorf("a folder into itself")
	}
	if err := w.moveTo(a, "Work/Old", true, "Old"); err != nil {
		t.Fatal(err)
	}
	for f, want := range map[string]bool{"plan.xopp": false, "plan.pdf": false, "Work/plan.xopp": true, "Work/plan.pdf": true,
		"Work/Old": false, "Old/notes.xopp": true} {
		if fileExists(filepath.Join(nd, filepath.FromSlash(f))) != want {
			t.Errorf("%s there: %v, want %v", f, !want, want)
		}
	}
	if w.back != "Work/plan" {
		t.Errorf("back to %q", w.back)
	}
}

// An exported page's PNG is named for its notebook (in a folder too),
// the page and when: each export its own, no colons for the card.
func TestExportPNGName(t *testing.T) {
	at := time.Date(2026, 10, 30, 14, 5, 7, 0, time.Local)
	if got, want := exportPNGName("Work/plan", 2, at), "plan p3 2026-10-30 14.05.07.png"; got != want {
		t.Errorf("%q, want %q", got, want)
	}
}
