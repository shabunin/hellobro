package main

import (
	"os"
	"path/filepath"
	"testing"
)

// hellobro.json round-trips, and takes the font size from the old
// term-font file once, which then goes.
func TestRemembered(t *testing.T) {
	dir := t.TempDir()
	if r := loadRemembered(dir); r.Light != nil || r.TermFont != 0 {
		t.Fatalf("nothing yet: %+v", r)
	}
	os.WriteFile(filepath.Join(dir, "term-font"), []byte("32\n"), 0o644)
	r := loadRemembered(dir)
	if r.TermFont != 32 {
		t.Fatalf("term-font: %d", r.TermFont)
	}
	if _, err := os.Stat(filepath.Join(dir, "term-font")); err == nil {
		t.Error("term-font left behind")
	}
	off := 0
	r.Light = &off
	r.save()
	r = loadRemembered(dir)
	if r.Light == nil || *r.Light != 0 || r.TermFont != 32 {
		t.Errorf("again: %+v", r)
	}
}
