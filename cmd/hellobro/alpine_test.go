package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type tarEntry struct {
	name, link, body string
	kind             byte
	mode             int64
}

func tgz(t *testing.T, es ...tarEntry) []byte {
	var b bytes.Buffer
	zw := gzip.NewWriter(&b)
	tw := tar.NewWriter(zw)
	for _, e := range es {
		h := &tar.Header{Name: e.name, Linkname: e.link, Typeflag: e.kind, Mode: e.mode, Size: int64(len(e.body))}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		tw.Write([]byte(e.body))
	}
	tw.Close()
	zw.Close()
	return b.Bytes()
}

// The minirootfs as it is: directories, files with their modes, symlinks
// (left as they are, absolute ones too), hard links.
func TestAlpineExtract(t *testing.T) {
	dir := t.TempDir()
	err := alpineExtract(bytes.NewReader(tgz(t,
		tarEntry{name: "./", kind: tar.TypeDir, mode: 0o755},
		tarEntry{name: "./bin/", kind: tar.TypeDir, mode: 0o755},
		tarEntry{name: "./bin/busybox", body: "#!", kind: tar.TypeReg, mode: 0o755},
		tarEntry{name: "./bin/sh", link: "/bin/busybox", kind: tar.TypeSymlink, mode: 0o777},
		tarEntry{name: "./bin/ash", link: "./bin/busybox", kind: tar.TypeLink, mode: 0o755},
		tarEntry{name: "./etc/", kind: tar.TypeDir, mode: 0o755},
		tarEntry{name: "./etc/shadow", body: "root:*", kind: tar.TypeReg, mode: 0o640},
	)), dir)
	if err != nil {
		t.Fatal(err)
	}
	if l, _ := os.Readlink(filepath.Join(dir, "bin/sh")); l != "/bin/busybox" {
		t.Errorf("bin/sh -> %q", l)
	}
	if st, _ := os.Stat(filepath.Join(dir, "etc/shadow")); st == nil || st.Mode().Perm() != 0o640 {
		t.Errorf("etc/shadow: %v", st)
	}
	a, _ := os.Stat(filepath.Join(dir, "bin/ash"))
	b, _ := os.Stat(filepath.Join(dir, "bin/busybox"))
	if a == nil || !os.SameFile(a, b) {
		t.Error("bin/ash isn't a hard link to bin/busybox")
	}
}

// Nothing lands outside the directory: not by "..", nor through a
// symlink (an absolute one points at the Kobo's own files, as we're not
// in the jail).
func TestAlpineExtractStaysIn(t *testing.T) {
	for name, es := range map[string][]tarEntry{
		"dotdot": {{name: "../../escaped", body: "x", kind: tar.TypeReg, mode: 0o644}},
		"through a symlink": {
			{name: "etc", link: "OUTSIDE", kind: tar.TypeSymlink, mode: 0o777},
			{name: "etc/escaped", body: "x", kind: tar.TypeReg, mode: 0o644},
		},
		"a hard link through a symlink": {
			{name: "etc", link: "OUTSIDE", kind: tar.TypeSymlink, mode: 0o777},
			{name: "stolen", link: "etc/secret", kind: tar.TypeLink, mode: 0o644},
		},
	} {
		top := t.TempDir()
		dir, outside := filepath.Join(top, "root"), filepath.Join(top, "outside")
		os.Mkdir(dir, 0o755)
		os.Mkdir(outside, 0o755)
		os.WriteFile(filepath.Join(outside, "secret"), []byte("s"), 0o600)
		for i := range es {
			es[i].link = strings.ReplaceAll(es[i].link, "OUTSIDE", outside)
		}
		err := alpineExtract(bytes.NewReader(tgz(t, es...)), dir)
		if _, serr := os.Stat(filepath.Join(top, "escaped")); serr == nil {
			t.Errorf("%s: a file outside", name)
		}
		if _, serr := os.Stat(filepath.Join(outside, "escaped")); serr == nil {
			t.Errorf("%s: a file through the symlink", name)
		}
		if _, serr := os.Stat(filepath.Join(dir, "stolen")); serr == nil {
			t.Errorf("%s: a link to a file outside", name)
		}
		if name != "dotdot" && err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

// The real minirootfs, if there's one at ALPINE_TGZ: it unpacks whole.
func TestAlpineExtractMinirootfs(t *testing.T) {
	path := os.Getenv("ALPINE_TGZ")
	if path == "" {
		t.Skip("ALPINE_TGZ: the minirootfs")
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	dir := t.TempDir()
	if err := alpineExtract(f, dir); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "etc/alpine-release")); strings.TrimSpace(string(b)) != alpineVersion {
		t.Errorf("etc/alpine-release: %q", b)
	}
}
