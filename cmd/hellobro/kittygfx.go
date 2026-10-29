package main

// Kitty's graphics protocol (sw.kovidgoyal.net/kitty/graphics-protocol):
// a program sends an image as ESC _ G <keys>;<base64> ESC \, and asks for
// it shown at the cursor. vt knows nothing of it, so we read the commands
// out of the output before it gets there (kittyGfx.feed). A placement
// becomes cells of the emulator's own, each with a rune from plane 15's
// private use area that says which row and column of the image it is,
// and a truecolor foreground that says which placement: they scroll,
// are overwritten and cleared like any text, and renderTerm draws the
// image's piece where it finds one.
//
// The subset: images sent directly (t=d; files and shared memory are
// refused: the paths would be a jail's or another host's), as RGB, RGBA
// or PNG, zlib'd or not, in chunks; placed with a size in cells or their
// own, or by Unicode placeholders (kgholder.go); queried; deleted all,
// by id or by number. No animation, no z order.

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
)

const (
	kgMaxData   = 20 << 20 // base64 a command (or its chunks) may bring
	kgMaxPixels = 12 << 20 // an image's, decoded
	kgMaxSide   = 2048     // kept no bigger: the screen is smaller
	kgMaxBytes  = 32 << 20 // the images a terminal keeps, all told
	kgMaxImages = 256      // and how many: a 1×1 one costs more than its byte
	kgMaxPlaces = 512
	kgMaxCells  = 255 // a placement's rows and columns: they're a byte each in the rune
	kgRune0     = 0xF0000
)

// The scanner's states: what of a command's start it has seen.
const (
	scanGround = iota
	scanEsc    // an ESC: held back, in case a _ G follows
	scanApc    // ESC _: held back, in case a G follows
	scanCmd    // in a command
	scanCmdEsc // an ESC in it: the end, if a \ follows
)

// kittyGfx is a terminal's graphics: the commands read out of its output,
// the images and where they're placed. It's the output's: used under
// t.mu, but for the placements renderTerm takes, which don't change.
type kittyGfx struct {
	state int8
	cmd   []byte // the command so far, after the G
	over  bool   // it grew too long: it's dropped
	load  *kgCmd // a transfer in chunks, so far

	cell    image.Point // a cell's size in pixels
	images  map[uint32]*kgImage
	numbers map[uint32]uint32 // the image numbers (I) programs gave, and the ids we gave them
	places  map[uint32]*kgPlace
	bytes   int
	nextID  uint32
	nextKey uint32
	seq     int
	changed bool // a placement went: its cells look different now
}

type kgImage struct {
	id   uint32
	gray *image.Gray // at most kgMaxSide a side
	w, h int         // as sent
	seq  int
}

// kgPlace is an image shown in cells. It doesn't change once made.
type kgPlace struct {
	key        uint32 // its cells' foreground
	img        *kgImage
	pid        uint32
	src        image.Rectangle // the part of the image shown, in its pixels as sent
	cols, rows int
	c, r       int         // the size asked for in cells, or 0
	cell       image.Point // the cells' size when it was placed
	seq        int
	virtual    bool // shown where its placeholders are (kgholder.go), not at the cursor
}

// kgCmd is a command's keys and payload.
type kgCmd struct {
	a, t, o, d byte
	f, s, v    int
	S, O       int
	i, I, p    uint32
	x, y, w, h int
	c, r       int
	C, m, q, U int
	data       []byte
}

// feed passes p on to out, but for the graphics commands in it, which go
// to do, the part after G. A command may come in pieces over many feeds;
// so may its start, which is held back till it's known for one.
func (g *kittyGfx) feed(p []byte, out func([]byte), do func([]byte)) {
	from := 0 // the next byte to pass on
	for i := 0; i < len(p); i++ {
		b := p[i]
		switch g.state {
		case scanGround:
			j := bytes.IndexByte(p[i:], 0x1b)
			if j < 0 {
				i = len(p)
				continue
			}
			i += j
			// Most ESCs start something else, seen at once: they stay in
			// the stream, which goes on to vt in one piece.
			if rest := p[i+1:]; len(rest) > 0 && rest[0] != '_' || len(rest) > 1 && rest[1] != 'G' {
				continue
			}
			if i > from {
				out(p[from:i])
			}
			g.state, from = scanEsc, i+1
		case scanEsc:
			switch b {
			case '_':
				g.state, from = scanApc, i+1
			case 0x1b:
				out([]byte{0x1b})
				from = i + 1
			default:
				out([]byte{0x1b})
				g.state, from = scanGround, i
			}
		case scanApc:
			if b == 'G' {
				g.state, from = scanCmd, i+1
				g.cmd, g.over = g.cmd[:0], false
				continue
			}
			out([]byte{0x1b, '_'}) // someone else's
			g.state, from = scanGround, i
			i--
		case scanCmd:
			j := bytes.IndexByte(p[i:], 0x1b)
			if j < 0 {
				j = len(p) - i
			} else {
				g.state = scanCmdEsc
			}
			if !g.over && len(g.cmd)+j > kgMaxData+256 {
				g.over, g.cmd = true, nil
			}
			if !g.over {
				g.cmd = append(g.cmd, p[i:i+j]...)
			}
			i += j
			from = i + 1
		case scanCmdEsc:
			if b != '\\' { // an ESC ends a string, as vt has it: the command with it
				g.state, from = scanEsc, i
				i--
				continue
			}
			g.state, from = scanGround, i+1
			if !g.over {
				do(g.cmd)
			}
			if cap(g.cmd) > 64<<10 {
				g.cmd = nil // not kept for good, an image's worth
			}
		}
	}
	if g.state == scanGround && from < len(p) {
		out(p[from:])
	}
}

// parseKgCmd reads a command: keys=values, a ; then the payload.
func parseKgCmd(b []byte) (kgCmd, error) {
	c := kgCmd{a: 't', t: 'd', f: 32, d: 'a'}
	keys := b
	if i := bytes.IndexByte(b, ';'); i >= 0 {
		keys, c.data = b[:i], b[i+1:]
	}
	for kv := range strings.SplitSeq(string(keys), ",") {
		if kv == "" {
			continue
		}
		k, v, ok := strings.Cut(kv, "=")
		if !ok || len(k) != 1 || v == "" {
			return c, fmt.Errorf("bad key %q", kv)
		}
		if p := c.char(k[0]); p != nil {
			if len(v) != 1 {
				return c, fmt.Errorf("bad %s", k)
			}
			*p = v[0]
			continue
		}
		if c.id(k[0]) == nil && c.num(k[0]) == nil { // the rest (z, X, Y, …) we don't do; z may be below 0
			continue
		}
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return c, fmt.Errorf("bad %s", k)
		}
		if p := c.id(k[0]); p != nil {
			*p = uint32(n)
		} else if p := c.num(k[0]); p != nil {
			if n > 1<<31-1 {
				return c, fmt.Errorf("bad %s", k)
			}
			*p = int(n)
		}
	}
	return c, nil
}

func (c *kgCmd) char(k byte) *byte {
	switch k {
	case 'a':
		return &c.a
	case 't':
		return &c.t
	case 'o':
		return &c.o
	case 'd':
		return &c.d
	}
	return nil
}

func (c *kgCmd) id(k byte) *uint32 {
	switch k {
	case 'i':
		return &c.i
	case 'I':
		return &c.I
	case 'p':
		return &c.p
	}
	return nil
}

func (c *kgCmd) num(k byte) *int {
	switch k {
	case 'f':
		return &c.f
	case 's':
		return &c.s
	case 'v':
		return &c.v
	case 'S':
		return &c.S
	case 'O':
		return &c.O
	case 'x':
		return &c.x
	case 'y':
		return &c.y
	case 'w':
		return &c.w
	case 'h':
		return &c.h
	case 'c':
		return &c.c
	case 'r':
		return &c.r
	case 'C':
		return &c.C
	case 'm':
		return &c.m
	case 'q':
		return &c.q
	case 'U':
		return &c.U
	}
	return nil
}

// gfxRegister has emu answer what the programs ask before they send
// images: the cells' size in pixels (CSI 14, 16, 18 t), the device's
// status as it should, and not report sizes without the pixels.
func (t *termSession) gfxRegister(emu *vt.SafeEmulator) {
	emu.RegisterCsiHandler(ansi.Command(0, 0, 't'), func(p ansi.Params) bool {
		n, _, _ := p.Param(0, 0)
		cell, w, h := t.gfx.cellSize(), emu.Emulator.Width(), emu.Emulator.Height() // under its lock already
		var r string
		switch n {
		case 14:
			r = fmt.Sprintf("\x1b[4;%d;%dt", h*cell.Y, w*cell.X)
		case 16:
			r = fmt.Sprintf("\x1b[6;%d;%dt", cell.Y, cell.X)
		case 18:
			r = fmt.Sprintf("\x1b[8;%d;%dt", h, w)
		default:
			return false
		}
		io.WriteString(emu.InputPipe(), r)
		return true
	})
	// Device status (CSI 5 n): "OK" is CSI 0 n; vt says CSI ? 0 n, the
	// DEC private one's answer, which ratatui-image waits for in vain
	// (and so takes no images, at 10×20 pixels a cell).
	emu.RegisterCsiHandler(ansi.Command(0, 0, 'n'), func(p ansi.Params) bool {
		if n, _, _ := p.Param(0, 0); n != 5 || len(p) != 1 {
			return false
		}
		io.WriteString(emu.InputPipe(), "\x1b[0n")
		return true
	})
	// In-band resize (mode 2048): vt's reports say 0×0 pixels, and a
	// program that asked for them (libvaxis's) believes them over the
	// pty's size, which has the pixels. Not set, it goes by the pty's.
	emu.RegisterCsiHandler(ansi.Command('?', 0, 'h'), func(p ansi.Params) bool {
		for i := range p {
			if n, _, _ := p.Param(i, 0); n != int(ansi.ModeInBandResize.Mode()) {
				return false
			}
		}
		return len(p) > 0
	})
}

func (g *kittyGfx) cellSize() image.Point {
	if g.cell.X <= 0 || g.cell.Y <= 0 {
		return image.Pt(10, 20)
	}
	return g.cell
}

// gfxCommand does a command the output brought, and answers it.
func (t *termSession) gfxCommand(b []byte) {
	g := &t.gfx
	c, err := parseKgCmd(b)
	if g.load != nil { // the next chunk: only m (and q) count
		l := g.load
		if len(l.data)+len(c.data) > kgMaxData {
			g.load = nil
			t.gfxReply(*l, "EFBIG:too much data")
			return
		}
		l.data = append(l.data, c.data...)
		l.q = max(l.q, c.q)
		if err == nil && c.m == 1 {
			return
		}
		g.load = nil
		c, err = *l, nil
	} else if err == nil && c.m == 1 {
		c.data = append([]byte(nil), c.data...)
		g.load = &c
		return
	}
	if err != nil {
		t.gfxReply(c, "EINVAL:"+err.Error())
		return
	}
	t.gfxReply(c, t.gfxRun(&c))
}

// gfxReply answers c with r ("OK" or "CODE:why"), if it has an id to
// answer with and its q doesn't say not to.
func (t *termSession) gfxReply(c kgCmd, r string) {
	if r == "" || c.i == 0 && c.I == 0 || c.q >= 2 || c.q == 1 && r == "OK" {
		return
	}
	var b strings.Builder
	b.WriteString("\x1b_G")
	fmt.Fprintf(&b, "i=%d", c.i)
	if c.I != 0 {
		fmt.Fprintf(&b, ",I=%d", c.I)
	}
	if c.p != 0 {
		fmt.Fprintf(&b, ",p=%d", c.p)
	}
	b.WriteString(";" + r + "\x1b\\")
	io.WriteString(t.emu.InputPipe(), b.String())
}

// gfxRun does c, and says how it went.
func (t *termSession) gfxRun(c *kgCmd) string {
	g := &t.gfx
	if c.i != 0 && c.I != 0 {
		return "EINVAL:both i and I"
	}
	switch c.a {
	case 'q', 't', 'T':
		gray, w, h, err := kgDecode(c)
		if err != nil {
			return err.Error()
		}
		if c.a == 'q' {
			return "OK"
		}
		img := g.add(c, gray, w, h)
		if c.a == 't' {
			return "OK"
		}
		return t.gfxPlace(c, img)
	case 'p':
		img := g.find(c)
		if img == nil {
			return "ENOENT:no such image"
		}
		c.i = img.id // the answer says it, if it came by number
		return t.gfxPlace(c, img)
	case 'd':
		g.delete(c)
		return ""
	}
	return "EINVAL:not supported"
}

// kgDecode makes c's payload an image, gray on white.
func kgDecode(c *kgCmd) (gray *image.Gray, w, h int, err error) {
	if c.t != 'd' {
		return nil, 0, 0, fmt.Errorf("EINVAL:only direct transmission (t=d)")
	}
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimRight(string(c.data), "="))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("EINVAL:bad base64")
	}
	if len(raw) == 0 {
		return nil, 0, 0, fmt.Errorf("ENODATA:no data")
	}
	bpp := map[int]int{24: 3, 32: 4}[c.f]
	if c.f != 100 && bpp == 0 {
		return nil, 0, 0, fmt.Errorf("EINVAL:format %d not supported", c.f)
	}
	if bpp != 0 && !kgSizeOK(c.s, c.v) {
		return nil, 0, 0, fmt.Errorf("EINVAL:bad size")
	}
	if c.o == 'z' {
		limit := kgMaxPixels * 4
		if bpp != 0 {
			limit = c.s * c.v * bpp
		}
		zr, err := zlib.NewReader(bytes.NewReader(raw))
		if err != nil {
			return nil, 0, 0, fmt.Errorf("EINVAL:bad zlib data")
		}
		raw, err = io.ReadAll(io.LimitReader(zr, int64(limit)+1))
		if err != nil || len(raw) > limit {
			return nil, 0, 0, fmt.Errorf("EINVAL:bad zlib data")
		}
	} else if c.o != 0 {
		return nil, 0, 0, fmt.Errorf("EINVAL:compression %c not supported", c.o)
	}
	var m image.Image
	if bpp == 0 {
		cfg, err := png.DecodeConfig(bytes.NewReader(raw))
		if err != nil {
			return nil, 0, 0, fmt.Errorf("EBADPNG:%v", err)
		}
		if !kgSizeOK(cfg.Width, cfg.Height) {
			return nil, 0, 0, fmt.Errorf("EFBIG:image too big")
		}
		if m, err = png.Decode(bytes.NewReader(raw)); err != nil {
			return nil, 0, 0, fmt.Errorf("EBADPNG:%v", err)
		}
	} else {
		if len(raw) < c.s*c.v*bpp {
			return nil, 0, 0, fmt.Errorf("ENODATA:short data")
		}
		m = kgRaw(raw, c.s, c.v, bpp)
	}
	gray = toGray(m)
	b := gray.Rect
	if f := float64(kgMaxSide) / float64(max(b.Dx(), b.Dy())); f < 1 {
		small := image.NewGray(image.Rect(0, 0, max(1, int(float64(b.Dx())*f)), max(1, int(float64(b.Dy())*f))))
		scaleGray(small, small.Rect, gray)
		gray = small
	}
	return gray, b.Dx(), b.Dy(), nil
}

// kgSizeOK says whether a w × h image may be decoded: divided, not
// multiplied, as an int is 32 bits on the Kobo (65536 × 65536 is 0).
func kgSizeOK(w, h int) bool { return w > 0 && h > 0 && w <= kgMaxPixels/h }

// kgRaw is RGB or RGBA pixels as an image.
func kgRaw(raw []byte, w, h, bpp int) image.Image {
	if bpp == 4 {
		return &image.NRGBA{Pix: raw[:w*h*4], Stride: w * 4, Rect: image.Rect(0, 0, w, h)}
	}
	m := image.NewGray(image.Rect(0, 0, w, h))
	for i := range m.Pix {
		p := raw[i*3:]
		m.Pix[i] = luma(uint32(p[0]), uint32(p[1]), uint32(p[2]))
	}
	return m
}

func luma(r, g, b uint32) uint8 { return uint8((19595*r + 38470*g + 7471*b + 1<<15) >> 16) }

// toGray is m in grays, what's transparent white.
func toGray(m image.Image) *image.Gray {
	b := m.Bounds()
	out := image.NewGray(image.Rect(0, 0, b.Dx(), b.Dy()))
	switch m := m.(type) {
	case *image.Gray:
		for y := 0; y < b.Dy(); y++ {
			copy(out.Pix[y*out.Stride:][:b.Dx()], m.Pix[m.PixOffset(b.Min.X, b.Min.Y+y):])
		}
	case *image.NRGBA:
		for y := 0; y < b.Dy(); y++ {
			src, dst := m.Pix[m.PixOffset(b.Min.X, b.Min.Y+y):], out.Pix[y*out.Stride:][:b.Dx()]
			for x := range dst {
				p := src[x*4:]
				a := uint32(p[3])
				dst[x] = uint8((uint32(luma(uint32(p[0]), uint32(p[1]), uint32(p[2])))*a + 255*(255-a)) / 255)
			}
		}
	case *image.RGBA: // premultiplied: white shows through as what alpha leaves
		for y := 0; y < b.Dy(); y++ {
			src, dst := m.Pix[m.PixOffset(b.Min.X, b.Min.Y+y):], out.Pix[y*out.Stride:][:b.Dx()]
			for x := range dst {
				p := src[x*4:]
				dst[x] = uint8(min(255, uint32(luma(uint32(p[0]), uint32(p[1]), uint32(p[2])))+255-uint32(p[3])))
			}
		}
	default:
		for y := 0; y < b.Dy(); y++ {
			for x := 0; x < b.Dx(); x++ {
				r, g, bb, a := m.At(b.Min.X+x, b.Min.Y+y).RGBA()
				l := (19595*r + 38470*g + 7471*bb + 1<<15) >> 16
				out.Pix[y*out.Stride+x] = uint8(min(0xffff, l+0xffff-a) >> 8)
			}
		}
	}
	return out
}

// add keeps an image sent with c, and makes room for it.
func (g *kittyGfx) add(c *kgCmd, gray *image.Gray, w, h int) *kgImage {
	if g.images == nil {
		g.images, g.numbers, g.places = map[uint32]*kgImage{}, map[uint32]uint32{}, map[uint32]*kgPlace{}
	}
	id := c.i
	if id == 0 { // ours to give: from the top, out of the programs' way
		for g.nextID++; g.images[^g.nextID] != nil; g.nextID++ {
		}
		id = ^g.nextID
		if c.I != 0 {
			g.numbers[c.I] = id
			c.i = id // the answer says it
		}
	}
	if old := g.images[id]; old != nil { // sent again: the old one goes, and where it was placed
		g.free(old)
	}
	g.seq++
	img := &kgImage{id: id, gray: gray, w: w, h: h, seq: g.seq}
	g.images[id] = img
	g.bytes += len(gray.Pix)
	for (g.bytes > kgMaxBytes || len(g.images) > kgMaxImages) && len(g.images) > 1 {
		var oldest *kgImage
		for _, m := range g.images {
			if m != img && (oldest == nil || m.seq < oldest.seq) {
				oldest = m
			}
		}
		g.free(oldest)
	}
	return img
}

func (g *kittyGfx) free(img *kgImage) {
	g.bytes -= len(img.gray.Pix)
	delete(g.images, img.id)
	for n, id := range g.numbers {
		if id == img.id {
			delete(g.numbers, n)
		}
	}
	for k, pl := range g.places {
		if pl.img == img {
			delete(g.places, k)
			g.changed = true
		}
	}
}

// find is the image c means, by id or by number.
func (g *kittyGfx) find(c *kgCmd) *kgImage {
	id := c.i
	if c.I != 0 {
		id = g.numbers[c.I]
	}
	return g.images[id]
}

// delete does a d command: the placements it means go, and with an
// uppercase one the images too.
func (g *kittyGfx) delete(c *kgCmd) {
	free := c.d >= 'A' && c.d <= 'Z'
	switch c.d | 0x20 {
	case 'a':
		if len(g.places) > 0 {
			clear(g.places)
			g.changed = true
		}
		if free {
			for _, img := range g.images {
				g.free(img)
			}
		}
	case 'i', 'n':
		img := g.find(c)
		if img == nil {
			return
		}
		for k, pl := range g.places {
			if pl.img.id == img.id && (c.p == 0 || pl.pid == c.p) {
				delete(g.places, k)
				g.changed = true
			}
		}
		if free && c.p == 0 {
			g.free(img)
		}
	}
}

// gfxPlace shows img at the cursor as c says.
func (t *termSession) gfxPlace(c *kgCmd, img *kgImage) string {
	g := &t.gfx
	src := image.Rect(c.x, c.y, img.w, img.h)
	if c.w > 0 {
		src.Max.X = min(src.Max.X, c.x+c.w)
	}
	if c.h > 0 {
		src.Max.Y = min(src.Max.Y, c.y+c.h)
	}
	if src.Empty() {
		return "EINVAL:empty source rectangle"
	}
	cell := g.cellSize()
	pl := &kgPlace{img: img, pid: c.p, src: src, c: c.c, r: c.r, cell: cell, virtual: c.U == 1}
	sw, sh := src.Dx(), src.Dy()
	switch {
	case c.c > 0 && c.r > 0:
		pl.cols, pl.rows = c.c, c.r
	case c.c > 0:
		pl.cols, pl.rows = c.c, ceilDiv(sh*c.c*cell.X/sw, cell.Y)
	case c.r > 0:
		pl.cols, pl.rows = ceilDiv(sw*c.r*cell.Y/sh, cell.X), c.r
	default:
		pl.cols, pl.rows = ceilDiv(sw, cell.X), ceilDiv(sh, cell.Y)
	}
	pl.cols, pl.rows = max(1, min(kgMaxCells, pl.cols)), max(1, min(kgMaxCells, pl.rows))
	if c.p != 0 || pl.virtual { // a placement of the same id goes (a virtual one, of no id, the last such)
		for k, o := range g.places {
			if o.img.id == img.id && o.pid == c.p && o.virtual == pl.virtual {
				delete(g.places, k)
				g.changed = true
			}
		}
	}
	for len(g.places) >= kgMaxPlaces {
		var oldest *kgPlace
		for _, o := range g.places {
			if oldest == nil || o.seq < oldest.seq {
				oldest = o
			}
		}
		delete(g.places, oldest.key)
		g.changed = true
	}
	for g.nextKey = g.nextKey%0xffffff + 1; g.places[g.nextKey] != nil; g.nextKey = g.nextKey%0xffffff + 1 {
	}
	g.seq++
	pl.key, pl.seq = g.nextKey, g.seq
	g.places[pl.key] = pl
	if pl.virtual {
		g.changed = true // its placeholders may be there already
	} else {
		t.gfxShow(pl, c.C == 1)
	}
	return "OK"
}

func ceilDiv(a, b int) int { return (a + b - 1) / b }

// gfxShow puts pl's cells in at the cursor, the screen scrolled up if
// they need the room, and the cursor after them unless stay.
func (t *termSession) gfxShow(pl *kgPlace, stay bool) {
	e := t.emu
	w, h := e.Width(), e.Height()
	rows := min(pl.rows, h)
	if rows > 1 {
		fmt.Fprintf(e, "%s\x1b[%dA", strings.Repeat("\n", rows-1), rows-1)
	}
	cur := e.CursorPosition()
	cols := min(pl.cols, w-cur.X)
	fg := color.RGBA{uint8(pl.key >> 16), uint8(pl.key >> 8), uint8(pl.key), 255}
	for y := range rows {
		for x := range cols {
			e.SetCell(cur.X+x, cur.Y+y, &uv.Cell{Content: string(rune(kgRune0 + y<<8 + x)), Width: 1, Style: uv.Style{Fg: fg}})
		}
	}
	if !stay {
		fmt.Fprintf(e, "\x1b[%d;%dH", cur.Y+rows, cur.X+cols+1)
	}
}

// kgCell says whether a cell is a placement's, and which: its key, and
// the row and column of it.
func kgCell(c *uv.Cell) (key uint32, row, col uint8, ok bool) {
	if len(c.Content) != 4 || c.Content[0] != 0xf3 || c.Style.Fg == nil {
		return 0, 0, 0, false
	}
	r, _ := utf8.DecodeRuneInString(c.Content)
	if r < kgRune0 || r > kgRune0+0xffff {
		return 0, 0, 0, false
	}
	rr, gg, bb, _ := c.Style.Fg.RGBA()
	return rr>>8<<16 | gg>>8<<8 | bb>>8, uint8((r - kgRune0) >> 8), uint8(r - kgRune0), true
}

// takeChanged says whether placements went since it last said.
func (g *kittyGfx) takeChanged() bool {
	ch := g.changed
	g.changed = false
	return ch
}

// --- drawing ---

// size is how big pl's image is drawn in cells of cell's size, within
// its cells.
func (pl *kgPlace) size(cell image.Point) image.Point {
	sw, sh := pl.src.Dx(), pl.src.Dy()
	box := image.Pt(pl.cols*cell.X, pl.rows*cell.Y)
	var s image.Point
	switch {
	case pl.c > 0 && pl.r > 0:
		s = box
	case pl.c > 0:
		s = image.Pt(box.X, sh*box.X/sw)
	case pl.r > 0:
		s = image.Pt(sw*box.Y/sh, box.Y)
	default: // its own size, grown or shrunk with the font
		s = image.Pt(sw*cell.X/pl.cell.X, sh*cell.Y/pl.cell.Y)
	}
	return image.Pt(max(1, min(box.X, s.X)), max(1, min(box.Y, s.Y)))
}

// tile is pl drawn into its cells: white where the image doesn't reach,
// dithered to the panel's 16 grays.
func (pl *kgPlace) tile(cell image.Point) *image.Gray {
	m := image.NewGray(image.Rect(0, 0, pl.cols*cell.X, pl.rows*cell.Y))
	for i := range m.Pix {
		m.Pix[i] = 255
	}
	g := pl.img.gray
	// The source rectangle in the kept image's pixels, which may be fewer.
	fx, fy := float64(g.Rect.Dx())/float64(pl.img.w), float64(g.Rect.Dy())/float64(pl.img.h)
	src := image.Rect(int(float64(pl.src.Min.X)*fx), int(float64(pl.src.Min.Y)*fy),
		int(float64(pl.src.Max.X)*fx+0.5), int(float64(pl.src.Max.Y)*fy+0.5)).Intersect(g.Rect)
	if src.Empty() {
		return m
	}
	r := image.Rectangle{Max: pl.size(cell)}
	scaleGray(m, r, g.SubImage(src).(*image.Gray))
	if r.Size() == src.Size() {
		// Pixel for pixel, it was made for the screen: a page's text,
		// say, which dots would only fray. Each pixel to its nearest gray.
		for y := range r.Dy() {
			for i, p := range m.Pix[y*m.Stride:][:r.Dx()] {
				m.Pix[y*m.Stride+i] = uint8((int(p) + 8) / 17 * 17)
			}
		}
	} else {
		dither16(m, r)
	}
	return m
}

// scaleGray draws src into dst's r: each pixel the average of the source
// pixels under it (the nearest one, growing).
func scaleGray(dst *image.Gray, r image.Rectangle, src *image.Gray) {
	sw, sh, dw, dh := src.Rect.Dx(), src.Rect.Dy(), r.Dx(), r.Dy()
	if sw == 0 || sh == 0 || dw == 0 || dh == 0 {
		return
	}
	xs := make([]int, dw+1)
	for x := range xs {
		xs[x] = x * sw / dw
	}
	for y := range dh {
		y0, y1 := y*sh/dh, (y+1)*sh/dh
		y1 = max(y1, y0+1)
		row := dst.Pix[dst.PixOffset(r.Min.X, r.Min.Y+y):][:dw]
		for x := range row {
			x0, x1 := xs[x], max(xs[x+1], xs[x]+1)
			sum := 0
			for sy := y0; sy < y1; sy++ {
				for _, p := range src.Pix[src.PixOffset(src.Rect.Min.X+x0, src.Rect.Min.Y+sy):][:x1-x0] {
					sum += int(p)
				}
			}
			row[x] = uint8(sum / ((x1 - x0) * (y1 - y0)))
		}
	}
}

// dither16 brings m's r to the 16 grays, the error spread Floyd and
// Steinberg's way: smooth shades come out as dots, not bands.
func dither16(m *image.Gray, r image.Rectangle) {
	w := r.Dx()
	cur, next := make([]int, w+2), make([]int, w+2) // errors, in 16ths
	for y := r.Min.Y; y < r.Max.Y; y++ {
		row := m.Pix[m.PixOffset(r.Min.X, y):][:w]
		for x := range row {
			v := max(0, min(255, int(row[x])+cur[x+1]/16))
			q := (v + 8) / 17 * 17
			row[x] = uint8(q)
			e := v - q
			cur[x+2] += e * 7
			next[x] += e * 3
			next[x+1] += e * 5
			next[x+2] += e
		}
		cur, next = next, cur
		clear(next)
	}
}

// gfxTile is pl drawn for the cells as they are now: kept, as long as
// they don't come to too much.
func (a *bro) gfxTile(pl *kgPlace, cell image.Point) *image.Gray {
	v := &a.tv
	if m := v.tiles[pl]; m != nil && m.Rect.Dx() == pl.cols*cell.X && m.Rect.Dy() == pl.rows*cell.Y {
		return m
	}
	m := pl.tile(cell)
	if v.tiles == nil || v.tileBytes+len(m.Pix) > kgMaxBytes {
		v.tiles, v.tileBytes = map[*kgPlace]*image.Gray{}, 0
	}
	v.tiles[pl] = m
	v.tileBytes += len(m.Pix)
	return m
}
