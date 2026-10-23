package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
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

func TestAlpineSize(t *testing.T) {
	for _, c := range []struct {
		in   string
		most int
		mb   int // 0 for an error
	}{
		{"", 4095, 1024},
		{"\n", 4095, 1024},
		{"", 600, 600}, // the default won't fit
		{"2048", 4095, 2048},
		{" 2048 MB\n", 4095, 2048},
		{"3000mb", 4095, 3000},
		{"256", 4095, 256},
		{"4095", 4095, 4095},
		{"4096", 4095, 0}, // FAT's limit
		{"2048", 1500, 0}, // no room
		{"255", 4095, 0},
		{"-1", 4095, 0},
		{"2 GB", 4095, 0},
		{"lots", 4095, 0},
	} {
		mb, err := alpineSize(c.in, c.most)
		if c.mb == 0 && err == nil || c.mb != 0 && (err != nil || mb != c.mb) {
			t.Errorf("%q (most %d): %d, %v; want %d", c.in, c.most, mb, err, c.mb)
		}
	}
}

// tty is a terminal's two ends, as alpineAskSize sees them.
type tty struct {
	io.Reader
	out strings.Builder
}

func (t *tty) Write(p []byte) (int, error) { return t.out.Write(p) }

func TestAlpineAskSize(t *testing.T) {
	term := &tty{Reader: strings.NewReader("lots\n100\n2048\n")}
	mb, err := alpineAskSize(term, 4095)
	if err != nil || mb != 2048 {
		t.Fatalf("%d, %v; want 2048", mb, err)
	}
	if n := strings.Count(term.out.String(), "how big"); n != 3 {
		t.Errorf("asked %d times, want 3:\n%s", n, term.out.String())
	}
	if _, err := alpineAskSize(&tty{Reader: strings.NewReader("lots\n")}, 4095); err == nil {
		t.Error("the session closed, and it still has a size")
	}
}

func TestJailReadOnly(t *testing.T) {
	kobo, bq := t.TempDir(), t.TempDir()
	for _, d := range []string{".adds/hellobro", ".kobo", "Books"} {
		os.MkdirAll(filepath.Join(kobo, d), 0o755)
	}
	os.MkdirAll(filepath.Join(bq, ".hellobro"), 0o755)
	for _, c := range []struct {
		storage, img string
		want         []string
	}{
		{kobo, kobo + "/.adds/hellobro/alpine.img", []string{kobo + "/.adds", kobo + "/.kobo"}},
		{bq, bq + "/.hellobro/alpine.img", []string{bq + "/.hellobro"}},
		{bq, "/elsewhere/alpine.img", nil},
	} {
		if got := jailReadOnly(c.storage, c.img); fmt.Sprint(got) != fmt.Sprint(c.want) {
			t.Errorf("%s, %s: %v, want %v", c.storage, c.img, got, c.want)
		}
	}
}

func TestProcSession(t *testing.T) {
	sid, _, errno := syscall.RawSyscall(syscall.SYS_GETSID, 0, 0, 0)
	if errno != 0 {
		t.Fatal(errno)
	}
	if got, ok := procSession(os.Getpid()); !ok || got != int(sid) {
		t.Errorf("our session: %d %v, want %d", got, ok, sid)
	}
}
