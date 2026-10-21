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

	"hellobro/kobo"
)

// HELLOBRO_TERM_RAW=in.raw HELLOBRO_TERM_PNG=out.png go test -run TestRenderRecording
// draws a recorded terminal stream (harness/record.py makes one) the way
// the SSH tab would.
func TestRenderRecording(t *testing.T) {
	in, out := os.Getenv("HELLOBRO_TERM_RAW"), os.Getenv("HELLOBRO_TERM_PNG")
	if in == "" || out == "" {
		t.Skip("set HELLOBRO_TERM_RAW and HELLOBRO_TERM_PNG")
	}
	raw, err := os.ReadFile(in)
	if err != nil {
		t.Fatal(err)
	}
	a := &bro{App: &kobo.App{F: kobo.LoadFonts()}}
	a.tf.setSize(termFontDefault)
	a.C = kobo.Canvas{Gray: image.NewGray(image.Rect(0, 0, 1872, 1404))}
	a.C.Fill(a.C.Bounds(), kobo.White)
	a.termP = kobo.NewPanel(a.F, "Terminal", image.Rect(16, 160, 1856, 1342), "") // where landscape puts it
	a.termGeometry()
	tr := &termSession{}
	a.tv.groups[kindSSH].sessions = []*termSession{tr}
	tr.pane.p = a.termP
	a.paneGeometry(tr)
	v := &tr.pane
	tr.emu = vt.NewSafeEmulator(v.cols, v.rows)
	tr.size = image.Pt(v.cols, v.rows)
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
		a.renderTerm(tr)
	}
	// What a full redraw would show, to compare with the incremental one.
	if p := os.Getenv("HELLOBRO_TERM_FULL_PNG"); p != "" {
		inc := image.NewGray(v.grid)
		draw.Draw(inc, v.grid, a.C.Gray, v.grid.Min, draw.Src)
		v.shown = nil
		a.renderTerm(tr)
		diff := 0
		for y := v.grid.Min.Y; y < v.grid.Max.Y; y++ {
			for x := v.grid.Min.X; x < v.grid.Max.X; x++ {
				if inc.GrayAt(x, y) != a.C.GrayAt(x, y) {
					diff++
				}
			}
		}
		t.Logf("incremental vs full redraw: %d pixels differ", diff)
		f, _ := os.Create(p)
		png.Encode(f, a.C.SubImage(v.grid))
		f.Close()
		draw.Draw(a.C.Gray, v.grid, inc, v.grid.Min, draw.Src)
	}
	t.Logf("%d×%d", v.cols, v.rows)
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
	png.Encode(f, a.C.SubImage(v.grid))
}

// TestTermKeys: what a key sends, held down too (a held arrow once went
// out as a private-use rune).
func TestTermKeys(t *testing.T) {
	for _, c := range []struct {
		code uint16
		m    termMods
		want string
	}{
		{keyUp, termMods{}, "\x1b[A"},
		{keyDown, termMods{}, "\x1b[B"},
		{keyLeft, termMods{}, "\x1b[D"},
		{keyRight, termMods{}, "\x1b[C"},
		{keyPgUp, termMods{}, "\x1b[5~"},
		{kobo.KeyA, termMods{}, "a"},
		{kobo.KeyA, termMods{shift: true}, "A"},
		{kobo.KeyA, termMods{ctrl: true}, "\x01"},
		{kobo.KeyBackspace, termMods{}, "\x7f"},
	} {
		k, ok := termKeyEvent(c.code, c.m)
		if !ok {
			t.Errorf("%d: no key", c.code)
			continue
		}
		emu := vt.NewEmulator(80, 24)
		got := make(chan string)
		go func() {
			buf := make([]byte, 64)
			n, _ := emu.Read(buf)
			got <- string(buf[:n])
		}()
		emu.SendKey(k)
		if s := <-got; s != c.want {
			t.Errorf("%d %+v: %q, want %q", c.code, c.m, s, c.want)
		}
		emu.Close()
	}
}
