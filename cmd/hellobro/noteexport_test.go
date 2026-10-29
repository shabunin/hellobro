package main

import (
	"bytes"
	"fmt"
	"image"
	"regexp"
	"strconv"
	"testing"
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
