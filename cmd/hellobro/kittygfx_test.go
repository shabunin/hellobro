package main

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/vt"

	"hellobro/kobo"
)

// TestGfxFeed: the commands come out of the stream, whole however it's
// cut up; the rest goes on as it came, other APCs too.
func TestGfxFeed(t *testing.T) {
	stream := "ab\x1b[1mc\x1b_Ga=T,f=100;QUJD\x1b\\d\x1b_Xother\x1b\\e\x1b\x1b_Gi=2;\x1b\\f\x1b_Gcut\x1bz"
	wantOut := "ab\x1b[1mcd\x1b_Xother\x1b\\e\x1bf\x1bz"
	wantCmds := []string{"a=T,f=100;QUJD", "i=2;"}
	for _, sizes := range [][]int{{len(stream)}, {1}, {2}, {3}, {5}, {7}} {
		var g kittyGfx
		var out bytes.Buffer
		var cmds []string
		s := stream
		for i := 0; len(s) > 0; i++ {
			n := min(len(s), sizes[i%len(sizes)])
			g.feed([]byte(s[:n]), func(b []byte) { out.Write(b) }, func(b []byte) { cmds = append(cmds, string(b)) })
			s = s[n:]
		}
		if out.String() != wantOut {
			t.Errorf("by %v: passed on %q, want %q", sizes, out.String(), wantOut)
		}
		if fmt.Sprint(cmds) != fmt.Sprint(wantCmds) {
			t.Errorf("by %v: commands %q, want %q", sizes, cmds, wantCmds)
		}
	}
	// Without a command, a read goes on whole.
	var g kittyGfx
	n := 0
	g.feed([]byte("\x1b[1mbold\x1b[0m \x1b_Xapc\x1b\\ \x1b]0;title\a"), func([]byte) { n++ }, nil)
	if n != 1 {
		t.Errorf("passed on in %d pieces", n)
	}
}

func TestParseKgCmd(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
	}{
		{"a=T,f=100,i=7,q=2;AAAA", "a=T f=100 i=7 q=2 data=AAAA"},
		{"a=p,i=4294967295,p=3,c=10,r=5,C=1", "a=p f=32 i=4294967295 q=0 data="},
		{"m=1;xyz", "a=t f=32 i=0 q=0 data=xyz"},
		{"", "a=t f=32 i=0 q=0 data="},
		{"z=-1,X=3", "error"},
		{"a=TT", "error"},
		{"i=x", "error"},
		{"i=4294967296", "error"},
		{"f", "error"},
	} {
		c, err := parseKgCmd([]byte(tc.in))
		got := fmt.Sprintf("a=%c f=%d i=%d q=%d data=%s", c.a, c.f, c.i, c.q, c.data)
		if err != nil {
			got = "error"
		}
		if got != tc.want {
			t.Errorf("%q: %s, want %s", tc.in, got, tc.want)
		}
	}
}

// gfxTerm is a terminal with graphics, 10×20 cells, its answers read.
func gfxTerm(t *testing.T, cols, rows int) (*termSession, func() string) {
	ts := &termSession{state: termConnected}
	ts.emu = vt.NewSafeEmulator(cols, rows)
	ts.gfx.cell = image.Pt(10, 20)
	ts.gfxRegister(ts.emu)
	out := make(chan string, 16)
	go func() {
		buf := make([]byte, 256)
		for {
			n, err := ts.emu.Read(buf)
			if err != nil {
				return
			}
			out <- string(buf[:n])
		}
	}()
	t.Cleanup(func() { ts.emu.Close() })
	return ts, func() string {
		select {
		case b := <-out:
			return b
		case <-time.After(time.Second):
			return ""
		}
	}
}

func pngB64(t *testing.T, w, h int, c color.Color) string {
	m := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			m.Set(x, y, c)
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, m); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(b.Bytes())
}

// TestGfxPlace: an image sent and shown takes the cells it covers, the
// cursor goes after it, and the answers are kitty's.
func TestGfxPlace(t *testing.T) {
	ts, read := gfxTerm(t, 20, 6)
	ts.write([]byte("hi\r\n"))
	// 25×30 pixels: 3 columns, 2 rows of 10×20 cells.
	data := pngB64(t, 25, 30, color.Black)
	ts.write([]byte("\x1b_Ga=T,f=100,i=7,m=1;" + data[:8] + "\x1b\\"))
	ts.write([]byte("\x1b_Gm=0;" + data[8:] + "\x1b\\"))
	if r := read(); r != "\x1b_Gi=7;OK\x1b\\" {
		t.Fatalf("answer %q", r)
	}
	for y := range 3 {
		for x := range 4 {
			key, row, col, ok := kgCell(ts.emu.CellAt(x, 1+y))
			want := x < 3 && y < 2
			if ok != want || ok && (int(row) != y || int(col) != x || ts.gfx.places[key] == nil) {
				t.Errorf("cell %d,%d: %v %d %d %d", x, 1+y, ok, key, row, col)
			}
		}
	}
	if c := ts.emu.CursorPosition(); c.X != 3 || c.Y != 2 {
		t.Errorf("cursor at %v, want 3,2", c)
	}
	if got := cellText(ts, 20, image.Pt(0, 0), image.Pt(19, 2)); got != "hi\n\n" { // the image's rows blank
		t.Errorf("text %q", got)
	}
	// Placed again, asked 2×1 cells, the cursor staying.
	ts.write([]byte("\r\n\x1b_Ga=p,i=7,p=1,c=2,r=1,C=1\x1b\\"))
	if r := read(); r != "\x1b_Gi=7,p=1;OK\x1b\\" {
		t.Fatalf("answer %q", r)
	}
	if _, _, _, ok := kgCell(ts.emu.CellAt(1, 3)); !ok {
		t.Error("not placed")
	}
	if c := ts.emu.CursorPosition(); c.X != 0 || c.Y != 3 {
		t.Errorf("cursor at %v, want 0,3", c)
	}
	// Errors, and quiet.
	ts.write([]byte("\x1b_Ga=p,i=9\x1b\\"))
	if r := read(); !strings.HasPrefix(r, "\x1b_Gi=9;ENOENT:") {
		t.Errorf("no image: %q", r)
	}
	ts.write([]byte("\x1b_Ga=t,t=f,i=10;L2V0Yy9wYXNzd2Q=\x1b\\"))
	if r := read(); !strings.HasPrefix(r, "\x1b_Gi=10;EINVAL:") {
		t.Errorf("a file: %q", r)
	}
	ts.write([]byte("\x1b_Ga=q,i=31,s=1,v=1,f=24,q=1;AAAA\x1b\\\x1b[16t"))
	if r := read(); r != "\x1b[6;20;10t" {
		t.Errorf("quiet query, then the cell size: %q", r)
	}
	// Deleted: the cells stay, but show nothing.
	ts.gfx.takeChanged()
	ts.write([]byte("\x1b_Ga=d,d=i,i=7\x1b\\"))
	if len(ts.gfx.places) != 0 || !ts.gfx.takeChanged() || ts.gfx.images[7] == nil {
		t.Errorf("deleted: %d placements, image %v", len(ts.gfx.places), ts.gfx.images[7] != nil)
	}
	ts.write([]byte("\x1b_Ga=d,d=I,i=7\x1b\\"))
	if ts.gfx.images[7] != nil || ts.gfx.bytes != 0 {
		t.Errorf("freed: %d bytes left", ts.gfx.bytes)
	}
}

// TestGfxNumber: an image sent by number gets an id of ours, said in the
// answer, and is placed by its number.
func TestGfxNumber(t *testing.T) {
	ts, read := gfxTerm(t, 20, 6)
	var z bytes.Buffer
	zw := zlib.NewWriter(&z)
	zw.Write(bytes.Repeat([]byte{255, 0, 0, 128}, 4))
	zw.Close()
	ts.write([]byte("\x1b_Ga=t,I=5,s=2,v=2,o=z;" + base64.StdEncoding.EncodeToString(z.Bytes()) + "\x1b\\"))
	r := read()
	var id uint32
	if _, err := fmt.Sscanf(r, "\x1b_Gi=%d,I=5;OK", &id); err != nil || id == 0 {
		t.Fatalf("answer %q", r)
	}
	img := ts.gfx.images[id]
	if img == nil || img.w != 2 || img.gray.Pix[0] != (76*128+255*127)/255 {
		t.Fatalf("image %+v", img)
	}
	ts.write([]byte("\x1b_Ga=p,I=5\x1b\\"))
	if r := read(); r != fmt.Sprintf("\x1b_Gi=%d,I=5;OK\x1b\\", id) {
		t.Errorf("placed: %q", r)
	}
}

// TestGfxRender: the placement's cells are drawn from the image.
func TestGfxRender(t *testing.T) {
	a := &bro{App: &kobo.App{F: kobo.LoadFonts()}}
	a.tf.setSize(termFontDefault)
	a.C = kobo.Canvas{Gray: image.NewGray(image.Rect(0, 0, 1000, 700))}
	a.C.Fill(a.C.Bounds(), kobo.White)
	tr := testTerm(a)
	tr.pane.p = kobo.NewPanel(a.F, "Terminal", image.Rect(10, 10, 990, 690), "")
	a.paneGeometry(tr)
	v := &tr.pane
	tr.emu = vt.NewSafeEmulator(v.cols, v.rows)
	tr.size = image.Pt(v.cols, v.rows)
	tr.state = termConnected
	tr.gfx.cell = image.Pt(a.tv.cw, a.tv.ch)
	go func() {
		buf := make([]byte, 256)
		for {
			if _, err := tr.emu.Read(buf); err != nil {
				return
			}
		}
	}()
	defer tr.emu.Close()
	cw, ch := a.tv.cw, a.tv.ch
	tr.write([]byte("\x1b_Ga=T,f=100,c=4,r=2,q=2;" + pngB64(t, 8, 8, color.Black) + "\x1b\\"))
	a.renderTerm(tr)
	g := v.grid.Min
	if p := a.C.GrayAt(g.X+2*cw, g.Y+ch); p.Y != 0 {
		t.Errorf("inside the image: %d, want black", p.Y)
	}
	if p := a.C.GrayAt(g.X+4*cw+cw/2, g.Y+ch/2); p.Y != 255 {
		t.Errorf("after it: %d, want white", p.Y)
	}
	tr.write([]byte("\x1b_Ga=d\x1b\\"))
	a.renderTerm(tr)
	if p := a.C.GrayAt(g.X+2*cw, g.Y+ch); p.Y != 255 {
		t.Errorf("deleted: %d, want white", p.Y)
	}
}

func TestDither16(t *testing.T) {
	m := image.NewGray(image.Rect(0, 0, 64, 4))
	for x := range 64 {
		for y := range 4 {
			m.Pix[y*m.Stride+x] = uint8(x * 4)
		}
	}
	dither16(m, m.Rect)
	sum := 0
	for _, p := range m.Pix {
		if p%17 != 0 {
			t.Fatalf("level %d isn't one of the 16", p)
		}
		sum += int(p)
	}
	if avg := sum / len(m.Pix); avg < 120 || avg > 132 {
		t.Errorf("average %d, want about 126", avg)
	}
}
