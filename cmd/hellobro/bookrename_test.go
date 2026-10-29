package main

import (
	"image"
	"os"
	"path/filepath"
	"testing"

	"hellobro/kobo"
	"hellobro/wm"
)

// TestRenameBook: the file takes the name, its kind kept; the place and
// the bookmarks go to it, with the name to show; a name taken is left.
func TestRenameBook(t *testing.T) {
	d := t.TempDir()
	alice, bob := filepath.Join(d, "alice.kepub.epub"), filepath.Join(d, "bob.pdf")
	os.WriteFile(alice, []byte("epub"), 0o644)
	os.WriteFile(bob, []byte("pdf"), 0o644)
	a := &bro{}
	a.wm.tree = wm.New(image.Rect(0, 0, 1000, 800), kobo.FoldH)
	a.mem.dir = t.TempDir()
	a.mem.Books = map[string]bookPlace{alice: {Title: "Alice's Adventures / Illustrated", Spine: 3, Marks: []bookMark{{Spine: 2}}}}
	if got := placeName(alice, a.mem.Books[alice]); got != "Alice's Adventures" {
		t.Errorf("by its title: %q", got)
	}
	if err := renameBook(a, alice, "Alice"); err != nil {
		t.Fatal(err)
	}
	to := filepath.Join(d, "Alice.kepub.epub")
	pl, ok := a.mem.Books[to]
	if !fileExists(to) || fileExists(alice) || !ok || pl.Spine != 3 || len(pl.Marks) != 1 || placeName(to, pl) != "Alice" {
		t.Errorf("renamed: %v %v %+v", fileExists(to), ok, pl)
	}
	if _, ok := a.mem.Books[alice]; ok {
		t.Errorf("the old place's left")
	}
	if err := renameBook(a, bob, "Alice"); err != nil || !fileExists(filepath.Join(d, "Alice.pdf")) {
		t.Errorf("another kind, the same name: %v", err)
	}
	if err := renameBook(a, filepath.Join(d, "Alice.pdf"), "Alice"); err != nil {
		t.Errorf("its own name again: %v", err)
	}
	os.WriteFile(bob, []byte("pdf"), 0o644)
	if err := renameBook(a, bob, "Alice"); err == nil {
		t.Errorf("a name taken")
	}
	if err := renameBook(a, bob, "a/b"); err == nil {
		t.Errorf("a slash")
	}
}
