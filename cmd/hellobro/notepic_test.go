package main

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"strings"
	"testing"
)

func testPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	m := image.NewGray(image.Rect(0, 0, w, h))
	for i := range m.Pix {
		m.Pix[i] = uint8(i)
	}
	var b bytes.Buffer
	if err := png.Encode(&b, m); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// A picture comes in from a .xopp as one of ours, under the ink, goes
// out once (not twice: not kept as it came as well), and a picture we
// can't read is kept as it came.
func TestNotebookXoppPictures(t *testing.T) {
	pic := base64.StdEncoding.EncodeToString(testPNG(t, 30, 20))
	in := `<xournal creator="Xournal++ 1.2.3" fileversion="4"><page width="445.32" height="593.76"><background type="solid" color="#ffffffff" style="plain"/>
<layer><stroke tool="pen" color="#000000ff" width="1">5 5 6 6</stroke><image left="14.27" top="28.55" right="23.79" bottom="34.89">` + pic + `</image>
<image left="1" top="1" right="2" bottom="2">not a png</image></layer></page></xournal>`
	nb, err := parseXopp("x", strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(nb.pages[0]) != 2 || nb.pages[0][0].pic == nil || len(nb.meta[0].extra) != 1 {
		t.Fatalf("%d strokes, the first a picture: %v, %d kept", len(nb.pages[0]), nb.pages[0][0].pic != nil, len(nb.meta[0].extra))
	}
	if b := nb.pages[0][0].box; b != image.Rect(45, 90, 75, 110) {
		t.Errorf("the picture's box: %v", b)
	}
	out := string(nb.xoppText())
	if n := strings.Count(out, "<image"); n != 2 {
		t.Errorf("%d images out, want 2 (ours, and the one kept)", n)
	}
	if !strings.Contains(out, `<image left="14.27" top="28.55" right="23.79" bottom="34.89">`+pic+`</image>`) {
		t.Errorf("our picture isn't as it came in:\n%s", out)
	}
	if i, j := strings.Index(out, "<image"), strings.Index(out, "<stroke"); i > j {
		t.Error("the picture comes after the ink: it would be over it")
	}
	if nb.pages[0][0].hits(image.Pt(60, 100)) {
		t.Error("the eraser takes a picture")
	}
}

func TestNotePastePlace(t *testing.T) {
	nb := &notebook{pages: [][]*noteStroke{nil}}
	w := &noteWin{nb: nb}
	if page, box := w.pastePlace(image.Pt(600, 300)); page != 0 || box != image.Rect(noteRule, 2*noteRule, noteRule+600, 2*noteRule+300) {
		t.Errorf("on an empty page: %d %v", page, box)
	}
	nb.pages[0] = []*noteStroke{{box: image.Rect(100, 100, 500, 900)}}
	if page, box := w.pastePlace(image.Pt(600, 300)); page != 0 || box.Min.Y != 900+noteRule/2 {
		t.Errorf("under a stroke: %d %v", page, box)
	}
	if page, box := w.pastePlace(image.Pt(600, 1000)); page != 1 || box.Min.Y != 2*noteRule {
		t.Errorf("no room under it: %d %v, want the next page's top", page, box)
	}
	if _, box := w.pastePlace(image.Pt(5000, 500)); box.Dx() != notePageW-2*noteRule || box.Dy() != 500*(notePageW-2*noteRule)/5000 {
		t.Errorf("too wide: %v, not shrunk to the margins", box)
	}
}
