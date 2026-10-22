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

// Saved hosts: an ssh.conf is taken in once (new names only) and kept as
// ssh.conf.old; jump hosts and commands come back as they were.
func TestRememberedHosts(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "ssh.conf"), []byte(
		"# a comment\npc  me@192.168.1.10  tmux new -A -s kobo\nhome  me@127.0.0.1:2222  via kobo@203.0.113.7\n"), 0o644)
	r := loadRemembered(dir)
	hs := r.hosts()
	if len(hs) != 2 || hs[0].line() != "pc  me@192.168.1.10:22  tmux new -A -s kobo" ||
		hs[1].line() != "home  me@127.0.0.1:2222 via kobo@203.0.113.7:22" {
		t.Fatalf("hosts: %q", r.Hosts)
	}
	if _, err := os.Stat(filepath.Join(dir, "ssh.conf.old")); err != nil {
		t.Error("no ssh.conf.old")
	}
	os.WriteFile(filepath.Join(dir, "ssh.conf"), []byte("pc  other@host\nnew  bro@10.0.0.2:2222\n"), 0o644)
	r = loadRemembered(dir)
	if hs := r.hosts(); len(hs) != 3 || hs[0].user != "me" || hs[2].name != "new" {
		t.Errorf("again: %q", r.Hosts)
	}
}

func TestHostDirName(t *testing.T) {
	for in, want := range map[string]string{"pc": "pc", "a/b": "a_b", "..": "_..", ".x": "_.x", "": "_"} {
		if got := hostDirName(in); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
}
