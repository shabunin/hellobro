package main

import (
	"image"
	"image/draw"
	"image/png"
	"math/rand/v2"
	"os"
	"strconv"
	"testing"

	"github.com/charmbracelet/x/vt"
)

// HELLOBRO_TERM_RAW=in.raw HELLOBRO_TERM_PNG=out.png go test -run TestRenderRecording
// draws a recorded terminal stream (harness/record.py makes one) the way
// the Terminal tab would.
func TestRenderRecording(t *testing.T) {
	in, out := os.Getenv("HELLOBRO_TERM_RAW"), os.Getenv("HELLOBRO_TERM_PNG")
	if in == "" || out == "" {
		t.Skip("set HELLOBRO_TERM_RAW and HELLOBRO_TERM_PNG")
	}
	raw, err := os.ReadFile(in)
	if err != nil {
		t.Fatal(err)
	}
	a := &app{f: loadFonts(), tab: tabTerm}
	a.c = canvas{image.NewGray(image.Rect(0, 0, 1872, 1404))}
	a.c.fill(a.c.Bounds(), white)
	a.L = makeLayout(1872, 1404, a.f, false)
	a.termGeometry()
	tr := &a.trm
	tr.emu = vt.NewSafeEmulator(tr.cols, tr.rows)
	tr.emu.SetDefaultForegroundColor(image.Black.C)
	tr.emu.SetDefaultBackgroundColor(image.White.C)
	tr.state = termConnected
	go func() { // answers to the stream's queries
		buf := make([]byte, 4096)
		for {
			if _, err := tr.emu.Read(buf); err != nil {
				return
			}
		}
	}()
	// Like the network: random chunks, a redraw after each (HELLOBRO_TERM_CHUNK=max size).
	chunk, _ := strconv.Atoi(os.Getenv("HELLOBRO_TERM_CHUNK"))
	rng := rand.New(rand.NewPCG(1, 2))
	for len(raw) > 0 {
		n := len(raw)
		if chunk > 0 {
			n = min(n, 1+rng.IntN(chunk))
		}
		tr.emu.Write(raw[:n])
		raw = raw[n:]
		a.renderTerm()
	}
	// What a full redraw would show, to compare with the incremental one.
	if p := os.Getenv("HELLOBRO_TERM_FULL_PNG"); p != "" {
		inc := image.NewGray(tr.grid)
		draw.Draw(inc, tr.grid, a.c.Gray, tr.grid.Min, draw.Src)
		tr.shown = nil
		a.renderTerm()
		diff := 0
		for y := tr.grid.Min.Y; y < tr.grid.Max.Y; y++ {
			for x := tr.grid.Min.X; x < tr.grid.Max.X; x++ {
				if inc.GrayAt(x, y) != a.c.GrayAt(x, y) {
					diff++
				}
			}
		}
		t.Logf("incremental vs full redraw: %d pixels differ", diff)
		f, _ := os.Create(p)
		png.Encode(f, a.c.SubImage(tr.grid))
		f.Close()
		draw.Draw(a.c.Gray, tr.grid, inc, tr.grid.Min, draw.Src)
	}
	t.Logf("%d×%d", tr.cols, tr.rows)
	for _, p := range []image.Point{{10, 5}, {20, 5}, {30, 6}} {
		if c := tr.emu.CellAt(p.X, p.Y); c != nil {
			t.Logf("cell %v %q fg %v bg %v → %+v", p, c.Content, c.Style.Fg, c.Style.Bg, tr.cellKeyAt(p.X, p.Y, false))
		}
	}
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	png.Encode(f, a.c.SubImage(tr.grid))
}
