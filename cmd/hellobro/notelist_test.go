package main

import (
	"image"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"hellobro/kobo"
	"hellobro/wm"
)

// The list's picking: a row held picks it, rename only for one; copied
// into a folder (a "(copy)" the second time), cut and pasted at the
// top, a folder copied whole and deleted whole, to the trash.
func TestNotesPick(t *testing.T) {
	defer func(r string) { shareRoot = r }(shareRoot)
	shareRoot = t.TempDir()
	nd := filepath.Join(shareRoot, "Notebooks")
	os.MkdirAll(filepath.Join(nd, "Work"), 0o755)
	a := &bro{App: &kobo.App{}}
	a.wm.tree = wm.New(image.Rect(0, 0, 1000, 800), kobo.FoldH)
	nb := &notebook{name: "plan", paper: "plain", pages: [][]*noteStroke{nil}}
	must(t, nb.save())
	w := &notesWin{tab: libFolders}
	w.load()
	item := func(key string) int {
		i := slices.IndexFunc(w.items, func(it libItem) bool { return it.key == key && !it.up })
		if i < 0 {
			t.Fatalf("%s not listed: %+v", key, w.items)
		}
		return i
	}
	tools := func() (ns []string) {
		for _, tl := range w.libSpec(a).tools {
			ns = append(ns, tl.name)
		}
		return
	}
	w.libSpec(a).hold(item("plan"))
	w.libSpec(a).open(item("Work")) // picking: a tap ticks it, doesn't open it
	if !w.picking || len(w.picks) != 2 || w.dir != "" || slices.Contains(tools(), "rename") {
		t.Fatalf("two picked: %v %+v, in %q, %v", w.picking, w.picks, w.dir, tools())
	}
	w.libSpec(a).open(item("Work"))
	if !slices.Contains(tools(), "rename") || !w.libSpec(a).picked(item("plan")) {
		t.Errorf("one picked: %v", tools())
	}
	w.clipPicked(a, false)
	w.show(a, libFolders, "Work")
	w.paste(a)
	w.clip = &noteClip{items: []libItem{{key: "plan", name: "plan"}}}
	w.paste(a)
	w.clip = &noteClip{items: []libItem{{key: "Work/plan (copy)", name: "plan (copy)"}}, cut: true}
	w.show(a, libFolders, "")
	w.paste(a)
	w.clip = &noteClip{items: []libItem{{key: "Work", name: "Work", dir: true}}}
	w.paste(a)
	must(t, removeNote(a, libItem{key: "Work (copy)", name: "Work (copy)", dir: true}))
	for f, want := range map[string]bool{"plan.xopp": true, "Work/plan.xopp": true, "Work/plan (copy).xopp": false, "plan (copy).xopp": true,
		"Work (copy)": false, ".trash/Work (copy)/plan.xopp": true} {
		if fileExists(filepath.Join(nd, filepath.FromSlash(f))) != want {
			t.Errorf("%s there: %v, want %v", f, !want, want)
		}
	}
}
