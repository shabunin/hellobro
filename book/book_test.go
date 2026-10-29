package book

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testEPUB writes an EPUB of files (and the container and package
// files for them) to a temporary file.
func testEPUB(t *testing.T, files map[string]string) string {
	t.Helper()
	name := filepath.Join(t.TempDir(), "t.epub")
	f, err := os.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	all := map[string]string{
		"META-INF/container.xml": `<?xml version="1.0"?><container><rootfiles><rootfile full-path="OPS/book.opf"/></rootfiles></container>`,
		"OPS/book.opf": `<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" xmlns:dc="http://purl.org/dc/elements/1.1/">
<metadata><dc:title>A Test</dc:title><dc:creator>Some One</dc:creator></metadata>
<manifest><item id="nav" href="nav.xhtml" properties="nav"/><item id="c1" href="text/one.xhtml"/><item id="c2" href="text/two%20b.xhtml"/><item id="css" href="s.css"/></manifest>
<spine><itemref idref="c1"/><itemref idref="c2"/></spine></package>`,
	}
	for k, v := range files {
		all[k] = v
	}
	for k, v := range all {
		w, _ := z.Create(k)
		w.Write([]byte(v))
	}
	z.Close()
	f.Close()
	return name
}

func TestOpenAndTOC(t *testing.T) {
	b, err := Open(testEPUB(t, map[string]string{
		"OPS/nav.xhtml": `<html xmlns:epub="http://www.idpf.org/2007/ops"><body><nav epub:type="toc"><ol>
<li><a href="text/one.xhtml">One</a><ol><li><a href="text/one.xhtml#x">  Inner
 part</a></li></ol></li><li><a href="text/two%20b.xhtml">Two</a></li></ol></nav></body></html>`,
		"OPS/text/one.xhtml":   `<html><body><p>a</p></body></html>`,
		"OPS/text/two b.xhtml": `<html><body><p>b</p></body></html>`,
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if b.Title != "A Test" || b.Author != "Some One" || len(b.Spine) != 2 || b.Spine[1] != "OPS/text/two b.xhtml" {
		t.Errorf("book: %q %q %q", b.Title, b.Author, b.Spine)
	}
	want := []TOCEntry{{"One", 0, "", 0}, {"Inner part", 0, "x", 1}, {"Two", 1, "", 0}}
	if fmt.Sprint(b.TOC) != fmt.Sprint(want) {
		t.Errorf("toc: %v, want %v", b.TOC, want)
	}
}

// show is a chapter's blocks, short: kind, head, align, indent, then
// the spans as text with their style.
func show(ch *Chapter) []string {
	var out []string
	for _, b := range ch.Blocks {
		var s strings.Builder
		switch b.Kind {
		case Picture:
			s.WriteString("pic " + b.Src)
			if b.Float != NoFloat {
				fmt.Fprintf(&s, " float %d", b.Float)
			}
		case Rule:
			s.WriteString("rule")
		default:
			if b.Head > 0 {
				fmt.Fprintf(&s, "h%d ", b.Head)
			}
			if b.Bullet != "" {
				s.WriteString(b.Bullet + " ")
			}
			fmt.Fprintf(&s, "a%d i%g f%g ", b.Align, b.Indent, b.First)
			for _, sp := range b.Spans {
				if sp.Style != 0 || sp.Scale != 1 {
					fmt.Fprintf(&s, "[%d %g]%q", sp.Style, sp.Scale, sp.Text)
				} else {
					fmt.Fprintf(&s, "%q", sp.Text)
				}
			}
		}
		out = append(out, s.String())
	}
	return out
}

func TestChapter(t *testing.T) {
	b, err := Open(testEPUB(t, map[string]string{
		"OPS/s.css": `/* x */ p { text-indent: 1.5em; margin: 0 }
.c, h2.mid { text-align: center } .it { font-style: italic } div p.x { color: red }
@media print { p { text-indent: 9em } } .gone { display: none } span.sc { font-size: 80% }`,
		"OPS/text/one.xhtml": `<?xml version="1.0"?><html><head><title>t</title><link rel="stylesheet" href="../s.css"/>
<style>.big { font-size: 1.5em }</style></head><body>
<h2 class="mid" id="top">Down the <i>Rabbit</i> Hole</h2>
<p>It was <b>so   very
 <i>odd</i></b>,&nbsp;she&#8217;s <span class="sc">here</span>.</p>
<p class="c gone">not shown</p>
<div class="c" style="float: left"><img src="../img/a%20b.png" alt="x"/></div>
<p style="text-indent:0">One<br/>two</p>
<ul><li>first</li><li>second</li></ul><ol><li>uno</li></ol>
<pre>  a := 1
  b</pre>
<hr/><p class="big it">Big</p></body></html>`,
		"OPS/text/two b.xhtml": `<html><body></body></html>`,
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	ch, err := b.Chapter(0)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		`h2 a2 i0 f0 [1 1.4]"Down the "[3 1.4]"Rabbit"[1 1.4]" Hole"`,
		"a0 i0 f1.5 \"It was \"[1 1]\"so very \"[3 1]\"odd\"\",\\u00a0she’s \"[0 0.8]\"here\"\".\"",
		`pic OPS/img/a b.png float 1`,
		`a0 i0 f0 "One""\n""two"`,
		`• a0 i1.5 f0 "first"`,
		`• a0 i1.5 f0 "second"`,
		`1. a0 i1.5 f0 "uno"`,
		`a1 i0 f0 [4 1]"  a := 1\n  b"`,
		`rule`,
		`a0 i0 f1.5 [2 1.5]"Big"`,
	}
	got := show(ch)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("blocks:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if ch.IDs["top"] != 0 {
		t.Errorf("ids: %v", ch.IDs)
	}
}

// TestDump shows a real book's chapters, as parsed: BOOK=file.epub.
func TestDump(t *testing.T) {
	name := os.Getenv("BOOK")
	if name == "" {
		t.Skip("BOOK not set")
	}
	b, err := Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	t.Logf("%q by %q, %d chapters, toc %v", b.Title, b.Author, len(b.Spine), b.TOC)
	for i := range b.Spine {
		ch, err := b.Chapter(i)
		if err != nil {
			t.Errorf("%d: %v", i, err)
			continue
		}
		for j, s := range show(ch) {
			if j < 12 {
				t.Logf("%d.%d %.160s", i, j, s)
			}
		}
	}
}

// TestImageTooBig: a picture whose header says it's huge isn't decoded;
// a small one is.
func TestImageTooBig(t *testing.T) {
	ihdr := func(w, h uint32) string { // a PNG's start: enough for its size
		var b bytes.Buffer
		b.WriteString("\x89PNG\r\n\x1a\n")
		data := binary.BigEndian.AppendUint32(binary.BigEndian.AppendUint32([]byte("IHDR"), w), h)
		data = append(data, 8, 0, 0, 0, 0)
		binary.Write(&b, binary.BigEndian, uint32(len(data)-4))
		b.Write(data)
		binary.Write(&b, binary.BigEndian, crc32.ChecksumIEEE(data))
		return b.String()
	}
	var small bytes.Buffer
	png.Encode(&small, image.NewGray(image.Rect(0, 0, 2, 2)))
	b, err := Open(testEPUB(t, map[string]string{"OPS/huge.png": ihdr(20000, 20000), "OPS/small.png": small.String()}))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if _, err := b.Image("OPS/huge.png"); err == nil || !strings.Contains(err.Error(), "too big") {
		t.Errorf("huge: %v", err)
	}
	if m, err := b.Image("OPS/small.png"); err != nil || m.Bounds().Dx() != 2 {
		t.Errorf("small: %v", err)
	}
}

func TestStripComments(t *testing.T) {
	for in, want := range map[string]string{
		"p{a:b}":                "p{a:b}",
		"/*x*/p{/* y */a:b}/*z": " p{ a:b}",
		"a/**/b/*/c*/d":         "a b d",
	} {
		if got := stripComments(in); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
	many := strings.Repeat("/* a comment */p{a:b}", 200000) // 4 MB: was minutes
	start := time.Now()
	stripComments(many)
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("%v for 200000 comments", d)
	}
}

// TestNote: a link's spans say where to, and whether it's a note's; the
// note is its id's blocks, up to the next id.
func TestNote(t *testing.T) {
	b, err := Open(testEPUB(t, map[string]string{
		"OPS/text/one.xhtml": `<html xmlns:epub="http://www.idpf.org/2007/ops"><body>
<p>See<a epub:type="noteref" href="../notes.xhtml#n1">1</a> and <a href="two%20b.xhtml#t">there</a>, <a href="http://x.org/">not</a>.</p></body></html>`,
		"OPS/text/two b.xhtml": `<html><body><p id="t">b</p></body></html>`,
		"OPS/notes.xhtml": `<html><body><aside id="n1"><p>First note.</p><p>More of it.</p></aside>
<aside id="n2"><p>Second.</p></aside></body></html>`,
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	ch, err := b.Chapter(0)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, sp := range ch.Blocks[0].Spans {
		got = append(got, fmt.Sprintf("%s|%s|%v", sp.Text, sp.Link, sp.Note))
	}
	want := []string{"See||false", "1|OPS/notes.xhtml#n1|true", " and ||false", "there|OPS/text/two b.xhtml#t|false", ", not.||false"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("spans:\n%q\nwant\n%q", got, want)
	}
	links := map[string]string{}
	for _, it := range Layout(ch, Settings{W: 600, H: 800, Px: 20}, goFaces{}, nil)[0].Items {
		if it.Link != "" {
			links[it.Text] = fmt.Sprint(it.Link, " ", it.Note)
		}
	}
	if len(links) != 2 || links["1"] != "OPS/notes.xhtml#n1 true" || links["there"] != "OPS/text/two b.xhtml#t false" {
		t.Errorf("laid out: %q", links)
	}
	bs, err := b.Note("OPS/notes.xhtml#n1")
	if err != nil || len(bs) != 2 || bs[1].Spans[0].Text != "More of it." {
		t.Errorf("note: %v %v", bs, err)
	}
	if _, err := b.Note("OPS/notes.xhtml#n9"); err == nil {
		t.Errorf("no such id")
	}
	if b.SpineOf("OPS/text/two b.xhtml") != 1 || b.SpineOf("OPS/notes.xhtml") != -1 {
		t.Errorf("spine")
	}
}

// TestFind: on from the place, into the next chapters, round to the
// start; any case.
func TestFind(t *testing.T) {
	b, err := Open(testEPUB(t, map[string]string{
		"OPS/text/one.xhtml":   `<html><body><p>The Rabbit ran.</p><p>A rabbit again.</p></body></html>`,
		"OPS/text/two b.xhtml": `<html><body><p>No hare here.</p><p>The last rabbit.</p></body></html>`,
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	never := func() bool { return false }
	for _, c := range []struct {
		spine int
		at    Pos
		q     string
		want  string
	}{
		{0, Pos{}, "rabbit", "0 {0 4} true"},
		{0, Pos{0, 4}, "rabbit", "0 {1 2} true"},
		{0, Pos{1, 2}, "RABBIT", "1 {1 9} true"},
		{1, Pos{1, 9}, "rabbit", "0 {0 4} true"},
		{0, Pos{}, "fox", "0 {0 0} false"},
	} {
		s, at, ok := b.Find(c.spine, c.at, c.q, never)
		if got := fmt.Sprint(s, " ", at, " ", ok); got != c.want {
			t.Errorf("%d %v %q: %s, want %s", c.spine, c.at, c.q, got, c.want)
		}
	}
}
