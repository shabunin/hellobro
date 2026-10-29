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

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
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
		{"a=p,z=-1,X=3,i=2", "a=p f=32 i=2 q=0 data="}, // what we don't do, below 0 or not
		{"x=-1", "error"},
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
	a.tv.fontPx = termFontDefault
	a.C = kobo.Canvas{Gray: image.NewGray(image.Rect(0, 0, 1000, 700))}
	a.C.Fill(a.C.Bounds(), kobo.White)
	tr := testTerm(a)
	tr.pane.p = kobo.NewPanel(a.F, "Terminal", image.Rect(10, 10, 990, 690), "")
	a.paneGeometry(tr)
	v := &tr.pane
	tr.emu = vt.NewSafeEmulator(v.cols, v.rows)
	tr.size = image.Pt(v.cols, v.rows)
	tr.state = termConnected
	tr.gfx.cell = a.face(tr).cell()
	go func() {
		buf := make([]byte, 256)
		for {
			if _, err := tr.emu.Read(buf); err != nil {
				return
			}
		}
	}()
	defer tr.emu.Close()
	cw, ch := a.face(tr).cw, a.face(tr).ch
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

// TestGfxHolder: a virtual placement shows only where its placeholders
// are, the cells without diacritics taking the next column on, and a
// placeholder of no image is blank.
func TestGfxHolder(t *testing.T) {
	a := &bro{App: &kobo.App{F: kobo.LoadFonts()}}
	a.tv.fontPx = termFontDefault
	a.C = kobo.Canvas{Gray: image.NewGray(image.Rect(0, 0, 1000, 700))}
	a.C.Fill(a.C.Bounds(), kobo.White)
	tr := testTerm(a)
	tr.pane.p = kobo.NewPanel(a.F, "Terminal", image.Rect(10, 10, 990, 690), "")
	a.paneGeometry(tr)
	v := &tr.pane
	tr.emu = vt.NewSafeEmulator(v.cols, v.rows)
	tr.size = image.Pt(v.cols, v.rows)
	tr.state = termConnected
	tr.gfx.cell = a.face(tr).cell()
	go func() {
		buf := make([]byte, 256)
		for {
			if _, err := tr.emu.Read(buf); err != nil {
				return
			}
		}
	}()
	defer tr.emu.Close()
	cw, ch := a.face(tr).cw, a.face(tr).ch
	tr.write([]byte("\x1b_Ga=T,U=1,i=300,f=100,c=4,r=2,q=2;" + pngB64(t, 8, 8, color.Black) + "\x1b\\"))
	if c := tr.emu.CursorPosition(); c.X != 0 || c.Y != 0 {
		t.Errorf("a virtual placement moved the cursor to %v", c)
	}
	d := func(i int) string { return string(kgDiacritics[i]) }
	// Image 300 is 0x00012c: rgb 0,1,44. Row 0 on the 2nd line from the
	// image's column 2, the next two cells without diacritics; row 1
	// under it, all four with them; then image 7, which isn't, on the
	// 4th line.
	tr.write([]byte("ab\r\n\x1b[38;2;0;1;44m  \U0010EEEE" + d(0) + d(2) + "\U0010EEEE\U0010EEEE" + "\x1b[m x\r\n" +
		"\x1b[38;2;0;1;44m\U0010EEEE" + d(1) + d(0) + "\U0010EEEE" + d(1) + d(1) + "\U0010EEEE" + d(1) + d(2) + "\U0010EEEE" + d(1) + d(3) + "\x1b[m\r\n" +
		"\x1b[38;5;7m\U0010EEEE" + d(0) + d(0) + "\x1b[m"))
	if c := tr.emu.CursorPosition(); c.X != 1 || c.Y != 3 {
		t.Errorf("placeholders a cell each: cursor at %v", c)
	}
	a.renderTerm(tr)
	g := v.grid.Min
	px := func(col, row int) uint8 { return a.C.GrayAt(g.X+col*cw+cw/2, g.Y+row*ch+ch/2).Y }
	for _, tc := range []struct {
		col, row int
		want     uint8
	}{
		{1, 1, 255},
		{2, 1, 0},   // the image's 3rd column
		{3, 1, 0},   // its 4th, inherited
		{4, 1, 255}, // a 5th: past it, blank
		{0, 2, 0},   // row 1, all of it
		{3, 2, 0},
		{0, 3, 255}, // image 7: none
		{5, 2, 255},
	} {
		if p := px(tc.col, tc.row); p != tc.want {
			t.Errorf("cell %d,%d: %d, want %d", tc.col, tc.row, p, tc.want)
		}
	}
	if got := cellText(tr, v.cols, image.Pt(0, 1), image.Pt(v.cols-1, 1)); got != "      x" {
		t.Errorf("text %q", got)
	}
}

func TestKgHolderOf(t *testing.T) {
	d := func(i int) string { return string(kgDiacritics[i]) }
	cell := func(s string, fg color.Color) *uv.Cell {
		return &uv.Cell{Content: "\U0010EEEE" + s, Width: 1, Style: uv.Style{Fg: fg}}
	}
	rgb := color.RGBA{0, 1, 44, 255}
	first, ok := kgHolderOf(cell(d(3)+d(5)+d(1), rgb), nil)
	if !ok || first != (kgHolder{id: 1<<24 | 300, row: 3, col: 5}) {
		t.Fatalf("first %+v", first)
	}
	if h, _ := kgHolderOf(cell("", rgb), &first); h != (kgHolder{id: 1<<24 | 300, row: 3, col: 6}) {
		t.Errorf("inherited %+v", h)
	}
	if h, _ := kgHolderOf(cell(d(4), rgb), &first); h != (kgHolder{id: 300, row: 4, col: 0}) {
		t.Errorf("another row %+v", h)
	}
	if h, _ := kgHolderOf(cell("", ansi.IndexedColor(9)), &first); h != (kgHolder{id: 9}) {
		t.Errorf("another image %+v", h)
	}
	if _, ok := kgHolderOf(&uv.Cell{Content: "x"}, nil); ok {
		t.Error("x is a placeholder")
	}
}

// TestGfxNoInBandResize: mode 2048 isn't taken on (its reports would say
// 0×0 pixels); the modes set with it are.
func TestGfxNoInBandResize(t *testing.T) {
	ts, read := gfxTerm(t, 20, 6)
	ts.write([]byte("\x1b[?2048h\x1b[c"))
	if r := read(); strings.HasPrefix(r, "\x1b[48;") {
		t.Errorf("2048 set: %q", r)
	}
	ts.write([]byte("\x1b[?25l\x1b[?25h\x1b[?1049h"))
	if !ts.emu.IsAltScreen() {
		t.Error("1049 not set")
	}
}

// TestGfxQueries: ratatui-image's questions, answered as it reads them.
func TestGfxQueries(t *testing.T) {
	ts, read := gfxTerm(t, 20, 6)
	ts.write([]byte("\x1b_Gi=31,s=1,v=1,a=q,t=d,f=24;AAAA\x1b\\\x1b[16t\x1b[5n\x1b[6n"))
	for _, want := range []string{"\x1b_Gi=31;OK\x1b\\", "\x1b[6;20;10t", "\x1b[0n", "\x1b[1;1R"} {
		if r := read(); r != want {
			t.Errorf("%q, want %q", r, want)
		}
	}
}

// TestGfxLimits: sizes that wrap a 32-bit int are refused; an image sent
// again takes its placements with it; images are capped in number too.
func TestGfxLimits(t *testing.T) {
	for _, c := range []struct {
		w, h int
		ok   bool
	}{{1, 1, true}, {4096, 3072, true}, {65536, 65536, false}, {kgMaxPixels, 2, false}, {0, 5, false}, {5, -1, false}} {
		if kgSizeOK(c.w, c.h) != c.ok {
			t.Errorf("%d × %d: %v", c.w, c.h, !c.ok)
		}
	}
	ts, read := gfxTerm(t, 20, 6)
	data := pngB64(t, 25, 30, color.Black)
	ts.write([]byte("\x1b_Ga=T,f=100,i=7;" + data + "\x1b\\"))
	read()
	if len(ts.gfx.places) != 1 {
		t.Fatalf("%d placements", len(ts.gfx.places))
	}
	ts.write([]byte("\x1b_Ga=t,f=100,i=7;" + data + "\x1b\\")) // again, not placed
	read()
	if len(ts.gfx.places) != 0 || ts.gfx.bytes != len(ts.gfx.images[7].gray.Pix) {
		t.Errorf("sent again: %d placements, %d bytes", len(ts.gfx.places), ts.gfx.bytes)
	}
	for i := range kgMaxImages + 50 {
		ts.write([]byte(fmt.Sprintf("\x1b_Ga=t,f=24,s=1,v=1,I=%d,q=2;AAAA\x1b\\", i+1)))
	}
	if n := len(ts.gfx.images); n > kgMaxImages {
		t.Errorf("%d images kept", n)
	}
	if n := len(ts.gfx.numbers); n > kgMaxImages {
		t.Errorf("%d numbers kept", n)
	}
}
