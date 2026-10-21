package main

// The Terminal tab: an SSH client with a real PTY. The remote's output
// goes through an xterm emulator (charmbracelet/x/vt) into a grid of
// cells, which we draw ourselves, redrawing only the cells that changed.
// Keys and touches go back as xterm key sequences and mouse events.

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/vt"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

const (
	termIdle = iota
	termConnecting
	termConnected
)

const termFrame = 60 * time.Millisecond // at most this often we redraw from the emulator

// The terminal's font size in pixels: Ctrl+= and Ctrl+- step it, Ctrl+0
// resets it, and it's remembered in term-font next to the binary.
const (
	termFontDefault          = 26
	termFontMin, termFontMax = 14, 56
	termFontStep             = 2
)

type termPrompt struct {
	question string
	echo     bool
	reveal   bool // Tab: show the password being typed
	answer   string
	reply    chan string // closed: canceled
}

type cellKey struct {
	content   string
	fg, bg    uint8
	attrs, ul uint8
	width     int
	cursor    bool
}

type termMods struct{ shift, ctrl, alt, meta, caps bool }

type termState struct {
	hosts        []sshHost
	hostRows     []image.Rectangle // where the saved hosts are listed, for taps
	input        string            // the ssh> line
	notes        []string
	state        int
	shownConnect []string // the connect screen's rows as drawn, nil: draw it all
	shownStatus  string
	target       sshHost
	prompt       *termPrompt
	gen          int // bumps per connection, so late messages from an old one are ignored

	sess  *sshSession
	emu   *vt.SafeEmulator
	dirty atomic.Bool
	title string

	grid       image.Rectangle // the cells' area
	cw, ch     int             // cell size
	cols, rows int
	shown      [][]cellKey // what's drawn
	drawnAt    time.Time
	outAt      time.Time       // last output from the remote
	fastArea   image.Rectangle // drawn with the fast (DU) waveform since the last clean refresh
	fastAt     time.Time       // when fastArea got its first update
	churn      int             // pixels changed since the last flash
	bigAt      time.Time       // the last change of more than a few rows

	mods   termMods
	glyphs map[glyphKey]*image.Alpha
	fontPx int
}

func (t *termState) connected() bool { return t.state == termConnected }

// --- layout & geometry ---

func (a *app) termGeometry() {
	t := &a.trm
	t.grid = a.L.termP.body.Inset(6)
	t.cw = textWidth(a.f.term, "M")
	m := a.f.term.Metrics()
	t.ch = m.Ascent.Ceil() + m.Descent.Ceil() + 2
	t.cols, t.rows = max(20, t.grid.Dx()/t.cw), max(5, t.grid.Dy()/t.ch)
	// Center the grid in the body.
	gw, gh := t.cols*t.cw, t.rows*t.ch
	min := image.Pt(t.grid.Min.X+(t.grid.Dx()-gw)/2, t.grid.Min.Y+(t.grid.Dy()-gh)/2)
	t.grid = image.Rectangle{min, min.Add(image.Pt(gw, gh))}
	t.shown = nil
	if t.connected() {
		t.emu.Resize(t.cols, t.rows)
		t.sess.resize(t.cols, t.rows)
	}
}

// --- drawing ---

func (t *termState) status() string {
	switch t.state {
	case termConnecting:
		return "connecting to " + t.target.String() + "…"
	case termConnected:
		s := fmt.Sprintf("%s · %d×%d · %d px", t.target, t.cols, t.rows, t.fontPx)
		if t.title != "" {
			s = t.title + " · " + s
		}
		return s
	}
	return "not connected"
}

// drawTerm draws the whole Terminal panel.
func (a *app) drawTerm() {
	if a.tab != tabTerm || a.calib != nil {
		return
	}
	t := &a.trm
	t.shownStatus, t.shownConnect = t.status(), nil
	a.drawPanel(a.L.termP, t.shownStatus)
	if t.connected() {
		t.shown = nil
		a.renderTerm()
		return
	}
	a.drawConnect()
}

func (a *app) drawConnect() {
	t := &a.trm
	lines := []string{"Type user@host[:port] and Enter, or tap / press 1-9 for a saved host.  F11: fullscreen."}
	switch {
	case t.prompt != nil:
		shown, help := t.prompt.answer, "(Enter to send, Esc to cancel)"
		if !t.prompt.echo {
			if !t.prompt.reveal {
				shown = strings.Repeat("•", utf8.RuneCountInString(shown))
			}
			help = "(Enter to send, Tab to show/hide, Esc to cancel)"
		}
		if t.mods.caps {
			help += "   CAPS LOCK is on"
		}
		lines = append(lines, t.prompt.question+shown+"█", help)
	case t.state == termIdle:
		lines = append(lines, "ssh> "+t.input+"█")
	default:
		lines = append(lines, "ssh> "+t.input)
	}
	lines = append(lines, "", fmt.Sprintf("saved hosts (%s):", appFile("ssh.conf")))
	firstHost := len(lines)
	for i, h := range t.hosts {
		lines = append(lines, fmt.Sprintf("   %d   %s   %s", i+1, h.name, h))
	}
	if len(t.hosts) == 0 {
		lines = append(lines, "   none yet: add lines like  myserver  bro@192.168.1.10:2222")
	}
	lines = append(lines, "", "This Kobo's key, for ~/.ssh/authorized_keys (also in id_ed25519.pub next to hellobro):")
	if s, err := clientKey(); err == nil {
		key := authorizedKey(s)
		for len(key) > 0 {
			n := min(len(key), 64)
			lines = append(lines, "   "+key[:n])
			key = key[n:]
		}
	} else {
		lines = append(lines, "   "+err.Error())
	}
	if len(t.notes) > 0 {
		lines = append(lines, "")
		lines = append(lines, t.notes...)
	}
	rows := a.drawTextRows(a.L.termP, a.f.body, lines, &t.shownConnect)
	t.hostRows = nil
	if len(t.hosts) > 0 {
		t.hostRows = rows[firstHost : firstHost+len(t.hosts)]
	}
}

// updateTerm redraws what changed on the connect screen: a key typed into
// a line shouldn't repaint the whole panel (in fullscreen, the whole screen).
func (a *app) updateTerm() {
	t := &a.trm
	if a.tab != tabTerm || a.calib != nil {
		return
	}
	if t.connected() || t.shownConnect == nil {
		a.drawTerm()
		return
	}
	if s := t.status(); s != t.shownStatus {
		t.shownStatus = s
		a.drawPanelTitle(a.L.termP, s)
	}
	a.drawConnect()
}

// renderTerm redraws the cells that changed since the last time.
func (a *app) renderTerm() {
	t := &a.trm
	if t.emu == nil {
		return
	}
	t.drawnAt = time.Now()
	if len(t.shown) != t.rows || (t.rows > 0 && len(t.shown[0]) != t.cols) {
		t.shown = make([][]cellKey, t.rows)
		for y := range t.shown {
			t.shown[y] = make([]cellKey, t.cols)
			for x := range t.shown[y] {
				t.shown[y][x].width = -1 // never matches
			}
		}
	}
	cur := t.emu.CursorPosition()
	showCursor := !t.emu.CursorHidden()
	type changed struct {
		r    image.Rectangle
		gray bool
	}
	var rows []changed
	for y := 0; y < t.rows; y++ {
		from, to, gray := -1, -1, false
		for x := 0; x < t.cols; x++ {
			k := t.cellKeyAt(x, y, showCursor && cur.X == x && cur.Y == y)
			if k == t.shown[y][x] {
				continue
			}
			t.shown[y][x] = k
			a.drawCell(x, y, k)
			if from < 0 {
				from = x
			}
			to = x
			gray = gray || !bw(k.fg) || !bw(k.bg)
		}
		if from >= 0 {
			r := image.Rect(t.grid.Min.X+from*t.cw, t.grid.Min.Y+y*t.ch, t.grid.Min.X+(to+1)*t.cw, t.grid.Min.Y+(y+1)*t.ch)
			rows = append(rows, changed{r.Inset(-1).Intersect(t.grid), gray})
		}
	}
	// Black & white changes to a row or three (typing, a spinner) take the
	// fast waveform. It ghosts, though, so scrolls and redraws, where a
	// moment more doesn't matter, get the clean grayscale one.
	big := len(rows) > 3
	if big {
		t.bigAt = time.Now()
	}
	for _, c := range rows {
		t.churn += area(c.r)
		if !c.gray && !big {
			if t.fastArea.Empty() {
				t.fastAt = time.Now()
			}
			t.fastArea = t.fastArea.Union(c.r)
		}
		a.mark(c.r, c.gray || big)
	}
}

func bw(g uint8) bool { return g == 0 || g == 255 }

func (t *termState) cellKeyAt(x, y int, cursor bool) cellKey {
	c := t.emu.CellAt(x, y)
	k := cellKey{fg: black, bg: white, width: 1, cursor: cursor}
	if c == nil {
		return k
	}
	k.content, k.width, k.attrs, k.ul = c.Content, c.Width, c.Style.Attrs, uint8(c.Style.Underline)
	if c.Style.Fg != nil {
		k.fg = gray(c.Style.Fg)
	}
	if c.Style.Bg != nil {
		k.bg = gray(c.Style.Bg)
	}
	if k.attrs&uv.AttrReverse != 0 {
		k.fg, k.bg = k.bg, k.fg
	}
	if k.attrs&uv.AttrFaint != 0 {
		k.fg = uint8((int(k.fg) + int(k.bg)) / 2)
	}
	if k.attrs&uv.AttrConceal != 0 {
		k.fg = k.bg
	} else if d := int(k.fg) - int(k.bg); d > -96 && d < 96 {
		// Colors that differ but turn into similar grays: keep text readable.
		if k.bg > 127 {
			k.fg = black
		} else {
			k.fg = white
		}
	}
	return k
}

// gray turns a color into one of the 16 levels the panel shows cleanly.
func gray(c color.Color) uint8 {
	r, g, b, _ := c.RGBA()
	y := (299*r + 587*g + 114*b) / 1000 >> 8
	return uint8((y + 8) / 17 * 17)
}

func (a *app) drawCell(x, y int, k cellKey) {
	t := &a.trm
	if k.width == 0 {
		return // the right half of a wide character, drawn with it
	}
	w := max(1, k.width)
	r := image.Rect(t.grid.Min.X+x*t.cw, t.grid.Min.Y+y*t.ch, t.grid.Min.X+(x+w)*t.cw, t.grid.Min.Y+(y+1)*t.ch).Intersect(t.grid)
	fg, bg := k.fg, k.bg
	if k.cursor {
		fg, bg = bg, fg
	}
	a.c.fill(r, bg)
	if k.content != "" && k.content != " " {
		mask := a.glyph(k.content, k.attrs&uv.AttrBold != 0, w)
		draw.DrawMask(a.c.Gray, r, image.NewUniform(color.Gray{fg}), image.Point{}, mask, image.Point{}, draw.Over)
	}
	if k.ul != 0 {
		a.c.fill(image.Rect(r.Min.X, r.Max.Y-3, r.Max.X, r.Max.Y-1), fg)
	}
	if k.attrs&uv.AttrStrikethrough != 0 {
		a.c.fill(image.Rect(r.Min.X, r.Min.Y+t.ch/2, r.Max.X, r.Min.Y+t.ch/2+2), fg)
	}
}

// --- glyphs ---

type glyphKey struct {
	s     string
	bold  bool
	width int
}

func (a *app) glyph(s string, bold bool, width int) *image.Alpha {
	t := &a.trm
	k := glyphKey{s, bold, width}
	if g, ok := t.glyphs[k]; ok {
		return g
	}
	if t.glyphs == nil || len(t.glyphs) > 4000 {
		t.glyphs = map[glyphKey]*image.Alpha{}
	}
	m := image.NewAlpha(image.Rect(0, 0, t.cw*width, t.ch))
	r, _ := utf8.DecodeRuneInString(s)
	if !drawSpecial(m, r) {
		f := a.f.term
		if bold {
			f = a.f.termBold
		}
		if _, ok := f.GlyphAdvance(r); !ok {
			f = a.f.termFallback // Go Mono lacks it: try the proportional Go font, centered
		}
		x := 0
		if f == a.f.termFallback {
			if _, ok := f.GlyphAdvance(r); !ok {
				drawMissing(m)
				t.glyphs[k] = m
				return m
			}
			x = (t.cw*width - textWidth(f, s)) / 2
		}
		d := font.Drawer{Dst: m, Src: image.Opaque, Face: f, Dot: fixed.P(x, 1+a.f.term.Metrics().Ascent.Ceil())}
		d.DrawString(s)
	}
	t.glyphs[k] = m
	return m
}

// box-drawing segments: up, right, down, left; 1 light, 2 heavy, 3 double.
var boxChars = map[rune][4]uint8{
	'─': {0, 1, 0, 1}, '│': {1, 0, 1, 0}, '┌': {0, 1, 1, 0}, '┐': {0, 0, 1, 1}, '└': {1, 1, 0, 0}, '┘': {1, 0, 0, 1},
	'├': {1, 1, 1, 0}, '┤': {1, 0, 1, 1}, '┬': {0, 1, 1, 1}, '┴': {1, 1, 0, 1}, '┼': {1, 1, 1, 1},
	'━': {0, 2, 0, 2}, '┃': {2, 0, 2, 0}, '┏': {0, 2, 2, 0}, '┓': {0, 0, 2, 2}, '┗': {2, 2, 0, 0}, '┛': {2, 0, 0, 2},
	'┣': {2, 2, 2, 0}, '┫': {2, 0, 2, 2}, '┳': {0, 2, 2, 2}, '┻': {2, 2, 0, 2}, '╋': {2, 2, 2, 2},
	'═': {0, 3, 0, 3}, '║': {3, 0, 3, 0}, '╔': {0, 3, 3, 0}, '╗': {0, 0, 3, 3}, '╚': {3, 3, 0, 0}, '╝': {3, 0, 0, 3},
	'╠': {3, 3, 3, 0}, '╣': {3, 0, 3, 3}, '╦': {0, 3, 3, 3}, '╩': {3, 3, 0, 3}, '╬': {3, 3, 3, 3},
	'╭': {0, 1, 1, 0}, '╮': {0, 0, 1, 1}, '╯': {1, 0, 0, 1}, '╰': {1, 1, 0, 0},
	'╴': {0, 0, 0, 1}, '╵': {1, 0, 0, 0}, '╶': {0, 1, 0, 0}, '╷': {0, 0, 1, 0},
	'┄': {0, 1, 0, 1}, '┈': {0, 1, 0, 1}, '╌': {0, 1, 0, 1}, '┅': {0, 2, 0, 2}, '┉': {0, 2, 0, 2}, '╍': {0, 2, 0, 2},
	'┆': {1, 0, 1, 0}, '┊': {1, 0, 1, 0}, '╎': {1, 0, 1, 0}, '┇': {2, 0, 2, 0}, '┋': {2, 0, 2, 0}, '╏': {2, 0, 2, 0},
}

// drawSpecial draws box-drawing, block and braille characters to fill the
// cell exactly, so lines and blocks join up across cells (font glyphs don't).
func drawSpecial(m *image.Alpha, r rune) bool {
	b := m.Bounds()
	w, h := b.Dx(), b.Dy()
	fill := func(x0, y0, x1, y1 int, a uint8) {
		draw.Draw(m, image.Rect(x0, y0, x1, y1).Intersect(b), image.NewUniform(color.Alpha{a}), image.Point{}, draw.Src)
	}
	if seg, ok := boxChars[r]; ok {
		t := max(2, w/8)
		cx, cy := w/2, h/2
		line := func(dir int, weight uint8) {
			th := t
			if weight == 2 {
				th = 2 * t
			}
			offs := []int{0}
			if weight == 3 {
				offs = []int{-t, t}
			}
			for _, o := range offs {
				switch dir {
				case 0: // up
					fill(cx-th/2+o, 0, cx-th/2+o+th, cy+th/2, 255)
				case 1: // right
					fill(cx-th/2, cy-th/2+o, w, cy-th/2+o+th, 255)
				case 2: // down
					fill(cx-th/2+o, cy-th/2, cx-th/2+o+th, h, 255)
				case 3: // left
					fill(0, cy-th/2+o, cx+th/2, cy-th/2+o+th, 255)
				}
			}
		}
		for dir, weight := range seg {
			if weight > 0 {
				line(dir, weight)
			}
		}
		return true
	}
	tri := func(dir int) { // a filled triangle pointing right, left, up or down
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				u, v := float64(x)/float64(w), float64(y)/float64(h) // 0..1
				var in bool
				switch dir {
				case 0:
					in = u >= 0.2 && u <= 0.85 && abs(int((v-0.5)*1000)) <= int((0.85-u)/0.65*0.3*1000)
				case 1:
					in = u >= 0.15 && u <= 0.8 && abs(int((v-0.5)*1000)) <= int((u-0.15)/0.65*0.3*1000)
				case 2:
					in = v >= 0.3 && v <= 0.7 && abs(int((u-0.5)*1000)) <= int((v-0.3)/0.4*0.4*1000)
				case 3:
					in = v >= 0.3 && v <= 0.7 && abs(int((u-0.5)*1000)) <= int((0.7-v)/0.4*0.4*1000)
				}
				if in {
					m.SetAlpha(x, y, color.Alpha{255})
				}
			}
		}
	}
	stroke := func(x0, y0, x1, y1 float64) { // a thick line, in cell fractions
		steps := 2 * max(w, h)
		th := max(1, w/7)
		for i := 0; i <= steps; i++ {
			f := float64(i) / float64(steps)
			cx, cy := int((x0+(x1-x0)*f)*float64(w)), int((y0+(y1-y0)*f)*float64(h))
			fill(cx-th/2, cy-th/2, cx-th/2+th, cy-th/2+th, 255)
		}
	}
	switch {
	case r == '▶' || r == '►' || r == '▸':
		tri(0)
	case r == '◀' || r == '◄' || r == '◂':
		tri(1)
	case r == '▲' || r == '▴':
		tri(2)
	case r == '▼' || r == '▾':
		tri(3)
	case r == '✓' || r == '✔':
		stroke(0.15, 0.55, 0.4, 0.75)
		stroke(0.4, 0.75, 0.88, 0.25)
	case r == '✗' || r == '✘':
		stroke(0.2, 0.3, 0.8, 0.75)
		stroke(0.8, 0.3, 0.2, 0.75)
	case r == '█':
		fill(0, 0, w, h, 255)
	case r == '▀':
		fill(0, 0, w, h/2, 255)
	case r == '▔':
		fill(0, 0, w, h/8, 255)
	case r >= '▁' && r <= '▇': // lower eighths
		fill(0, h-h*int(r-'▁'+1)/8, w, h, 255)
	case r >= '▉' && r <= '▏': // left eighths, 7/8 down to 1/8
		fill(0, 0, w*int('▏'-r+1)/8, h, 255)
	case r == '▐':
		fill(w/2, 0, w, h, 255)
	case r == '▕':
		fill(w-w/8, 0, w, h, 255)
	case r >= '░' && r <= '▓':
		fill(0, 0, w, h, uint8(64*int(r-'░'+1)))
	case r >= '▖' && r <= '▟':
		// quadrants: bits UL, UR, LL, LR
		q := map[rune]int{'▖': 4, '▗': 8, '▘': 1, '▙': 1 | 4 | 8, '▚': 1 | 8, '▛': 1 | 2 | 4, '▜': 1 | 2 | 8, '▝': 2, '▞': 2 | 4, '▟': 2 | 4 | 8}[r]
		for i, rr := range []image.Rectangle{image.Rect(0, 0, w/2, h/2), image.Rect(w/2, 0, w, h/2), image.Rect(0, h/2, w/2, h), image.Rect(w/2, h/2, w, h)} {
			if q&(1<<i) != 0 {
				fill(rr.Min.X, rr.Min.Y, rr.Max.X, rr.Max.Y, 255)
			}
		}
	case r >= 0x2800 && r <= 0x28FF: // braille: 2×4 dots
		bits := int(r - 0x2800)
		dots := [8][2]int{{0, 0}, {0, 1}, {0, 2}, {1, 0}, {1, 1}, {1, 2}, {0, 3}, {1, 3}}
		dw, dh := w/2, h/4
		s := max(2, min(dw, dh)*2/3)
		for i, d := range dots {
			if bits&(1<<i) != 0 {
				x0, y0 := d[0]*dw+(dw-s)/2, d[1]*dh+(dh-s)/2
				fill(x0, y0, x0+s, y0+s, 255)
			}
		}
	default:
		return false
	}
	return true
}

// --- connecting ---

func (a *app) enterTerm() {
	t := &a.trm
	if t.hosts == nil {
		t.hosts = loadSSHHosts()
	}
	if t.cw == 0 {
		t.fontPx = termFontDefault
		if b, err := os.ReadFile(appFile("term-font")); err == nil {
			if px, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil && px != t.fontPx {
				t.fontPx = max(termFontMin, min(termFontMax, px))
				a.f.setTermSize(t.fontPx)
			}
		}
		a.termGeometry()
	}
}

// termFontSwipe follows a swipe along the right edge, dy pixels up so
// far: half the screen height covers the whole range. The gauge shows the
// size; the terminal gets it when the finger lifts, since every change
// means a reflow on the remote and a full redraw here.
func (a *app) termFontSwipe(g *gesture, dy int) {
	steps := (termFontMax - termFontMin) / termFontStep
	px := g.fontFrom + dy*steps/(a.c.Bounds().Dy()/2)*termFontStep
	px = max(termFontMin, min(termFontMax, px))
	if px == g.fontTo && a.fontOverlay != nil {
		return
	}
	g.fontTo = px
	b := a.c.Bounds()
	cy := b.Dy() / 2
	r := image.Rect(b.Max.X-lightEdge-130, cy-260, b.Max.X-lightEdge-20, cy+260)
	a.drawGauge(&a.fontOverlay, r, px-termFontMin, termFontMax-termFontMin, fmt.Sprintf("%d px", px))
}

// setTermFont changes the terminal's font size; the remote gets the new
// cell count, as when a terminal window is resized.
func (a *app) setTermFont(px int) {
	t := &a.trm
	px = max(termFontMin, min(termFontMax, px))
	if px == t.fontPx {
		return
	}
	log.Printf("terminal font: %d px", px)
	t.fontPx = px
	a.f.setTermSize(px)
	t.glyphs = nil
	a.termGeometry()
	os.WriteFile(appFile("term-font"), []byte(strconv.Itoa(px)+"\n"), 0o644)
	a.drawTerm()
	a.markFlash(a.L.termP.body) // every glyph changed: clean, not ghosted
}

func (a *app) termNote(s string) {
	t := &a.trm
	log.Printf("ssh: %s", s)
	t.notes = append(t.notes, s)
	if len(t.notes) > 6 {
		t.notes = t.notes[len(t.notes)-6:]
	}
	if !t.connected() {
		a.updateTerm()
	}
}

func (a *app) termConnect(h sshHost) {
	t := &a.trm
	if t.state != termIdle {
		return
	}
	t.state, t.target, t.notes, t.title = termConnecting, h, nil, ""
	t.gen++
	gen := t.gen
	a.termNote("connecting to " + h.String() + "…")
	cols, rows := t.cols, t.rows
	ask := func(q string, echo bool) (string, error) {
		reply := make(chan string, 1)
		a.call <- func() {
			if t.gen == gen {
				t.prompt = &termPrompt{question: q, echo: echo, reply: reply}
				a.updateTerm()
			} else {
				close(reply)
			}
		}
		ans, ok := <-reply
		if !ok {
			return "", errors.New("canceled")
		}
		return ans, nil
	}
	note := func(s string) { a.call <- func() { a.termNote(s) } }
	go func() {
		s, err := dialSSH(h, cols, rows, ask, note)
		a.call <- func() {
			if t.gen != gen { // canceled meanwhile
				if s != nil {
					s.close()
				}
				return
			}
			t.prompt = nil
			if err != nil {
				t.state = termIdle
				a.termNote("failed: " + err.Error())
				if strings.Contains(err.Error(), "unable to authenticate") {
					a.termNote(fmt.Sprintf("the server refused user %q: check the user name and password", h.user))
				}
				return
			}
			a.termStart(s, gen)
		}
	}()
}

func (a *app) termStart(s *sshSession, gen int) {
	t := &a.trm
	t.sess, t.state = s, termConnected
	t.emu = vt.NewSafeEmulator(t.cols, t.rows)
	t.emu.SetDefaultForegroundColor(color.Black) // what apps get when they ask, so they pick light-theme colors
	t.emu.SetDefaultBackgroundColor(color.White)
	t.emu.SetScrollbackSize(1000) // not shown; the default 10000 costs a big copy per line once full
	t.emu.SetCallbacks(vt.Callbacks{
		Title: func(s string) { a.call <- func() { t.title = s; a.drawTermStatus() } },
	})
	t.sess.resize(t.cols, t.rows)
	log.Printf("ssh: connected to %s, %d×%d", t.target, t.cols, t.rows)
	emu := t.emu
	go func() { // remote → emulator
		buf := make([]byte, 32<<10)
		var err error
		for {
			var n int
			n, err = s.stdout.Read(buf)
			if n > 0 {
				emu.Write(buf[:n])
				t.dirty.Store(true)
			}
			if err != nil {
				break
			}
		}
		a.call <- func() { a.termEnded(gen, err) }
	}()
	go func() { // emulator (keys, mouse, replies to queries) → remote
		buf := make([]byte, 4096)
		for {
			n, err := emu.Read(buf)
			if n > 0 {
				if _, werr := s.stdin.Write(buf[:n]); werr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()
	a.drawTerm()
}

func (a *app) termEnded(gen int, err error) {
	t := &a.trm
	if t.gen != gen || t.state != termConnected {
		return
	}
	a.termClose()
	msg := "connection closed"
	if err != nil && !errors.Is(err, io.EOF) {
		msg += ": " + err.Error()
	}
	a.termNote(msg)
	a.drawTerm()
}

// termClose drops the session, or cancels a connection in progress.
func (a *app) termClose() {
	t := &a.trm
	if t.prompt != nil {
		close(t.prompt.reply)
		t.prompt = nil
	}
	if t.sess != nil {
		t.sess.close()
		t.emu.Close()
		t.sess, t.emu = nil, nil
	}
	t.gen++
	t.state = termIdle
	t.shown = nil
}

func (a *app) drawTermStatus() {
	if t := &a.trm; a.tab == tabTerm && a.calib == nil && t.connected() && t.status() != t.shownStatus {
		t.shownStatus = t.status()
		a.drawPanelTitle(a.L.termP, t.shownStatus)
	}
}

// termTick redraws from the emulator now and then, and gives the screen a
// clean grayscale refresh once output settles after fast updates.
func (a *app) termTick() {
	t := &a.trm
	if !t.connected() || a.tab != tabTerm || a.calib != nil {
		return
	}
	if a.lightOverlay != nil || a.fontOverlay != nil {
		return // a gauge is up: drawing under it would be undone when it goes
	}
	if t.dirty.Load() && time.Since(t.drawnAt) >= termFrame {
		t.dirty.Store(false)
		t.outAt = time.Now()
		a.renderTerm()
	}
	if a.blitting {
		return
	}
	// Clean up after the fast waveform once output pauses, or every few
	// seconds if it doesn't (a spinner never stops while its app works).
	if !t.fastArea.Empty() && (time.Since(t.outAt) > 1200*time.Millisecond || time.Since(t.fastAt) > 4*time.Second) {
		a.mark(t.fastArea, true)
		t.fastArea = image.Rectangle{}
	}
	// Grayscale updates leave ghosts too, slowly: after a few screenfuls
	// of change, flash the terminal clean once it settles (a clock or a
	// spinner may still tick), as e-readers do every few pages.
	if t.churn > 4*area(t.grid) && time.Since(t.bigAt) > 2500*time.Millisecond {
		t.churn, t.fastArea = 0, image.Rectangle{}
		a.markFlash(t.grid)
	}
}

// --- input ---

// termKey takes a key event while the Terminal tab is up. It returns false
// for keys the rest of the app should handle (tab switching, quitting)
// when there's no session to send them to.
func (a *app) termKey(code uint16, value int32) bool {
	t := &a.trm
	m := &t.mods
	down := value != 0
	switch code {
	case keyLeftShift, keyRightShift:
		m.shift = down
		return true
	case keyLeftCtrl, keyRightCtrl:
		m.ctrl = down
		return true
	case keyLeftAlt, keyRightAlt:
		m.alt = down
		return true
	case keyLeftMeta, keyRightMeta:
		m.meta = down
		return true
	case keyF11: // ours, not the remote's: fullscreen on and off
		if value == 1 {
			a.setFull(!a.full)
		}
		return true
	case keyEqual, keyMinus, key0: // with Ctrl, ours too: the font size
		if !m.ctrl {
			break
		}
		if value != 0 {
			switch code {
			case keyEqual:
				a.setTermFont(t.fontPx + termFontStep)
			case keyMinus:
				a.setTermFont(t.fontPx - termFontStep)
			default:
				a.setTermFont(termFontDefault)
			}
		}
		return true
	case keyCapsLock:
		if value == 1 {
			m.caps = !m.caps
			if !t.connected() {
				a.updateTerm() // the prompt says so
			}
		}
		return true
	}
	if value == 0 {
		return true
	}
	ch, printable := keyChar(code, m.shift, m.caps)

	if t.prompt != nil {
		p := t.prompt
		switch {
		case code == keyEnter:
			p.reply <- p.answer
			t.prompt = nil
		case code == keyEsc:
			a.termClose()
			a.termNote("canceled")
		case code == keyBackspace:
			if r := []rune(p.answer); len(r) > 0 {
				p.answer = string(r[:len(r)-1])
			}
		case code == keyTab:
			p.reveal = !p.reveal
		case printable:
			p.answer += ch
		}
		a.updateTerm()
		return true
	}

	if !t.connected() {
		switch {
		case t.state == termConnecting && code == keyEsc:
			a.termClose()
			a.termNote("canceled")
		case t.state != termIdle:
		case code == keyEnter:
			in := strings.TrimSpace(t.input)
			if in == "" && len(t.hosts) > 0 {
				a.termConnect(t.hosts[0])
			} else if h, err := parseSSHTarget(in); err == nil {
				a.termConnect(h)
			} else {
				a.termNote(err.Error())
			}
		case code == keyBackspace:
			if r := []rune(t.input); len(r) > 0 {
				t.input = string(r[:len(r)-1])
			}
		case printable && t.input == "" && ch >= "1" && ch <= "9":
			if i := int(ch[0] - '1'); i < len(t.hosts) {
				a.termConnect(t.hosts[i])
			}
		case printable && !m.ctrl:
			t.input += ch
		default:
			return false // Esc, F-keys: the app's
		}
		a.updateTerm()
		return true
	}

	var mod uv.KeyMod
	if m.ctrl {
		mod |= uv.ModCtrl
	}
	if m.alt {
		mod |= uv.ModAlt
	}
	if m.meta {
		mod |= uv.ModMeta
	}
	k := uv.KeyPressEvent{Mod: mod, IsRepeat: value == 2}
	if special, ok := termSpecialKeys[code]; ok {
		k.Code = special
		if m.shift {
			k.Mod |= uv.ModShift
		}
	} else if printable {
		k.Code = []rune(plainKeys[code])[0]
		if !m.ctrl {
			k.Text = ch
		}
	} else {
		return true
	}
	t.emu.SendKey(k)
	return true
}

const (
	keyLeftAlt   = 56
	keyRightAlt  = 100
	keyLeftMeta  = 125
	keyRightMeta = 126
	key0         = 11
	keyMinus     = 12
	keyEqual     = 13
	keyF11       = 87
	keyF12       = 88
)

var termSpecialKeys = map[uint16]rune{
	keyEsc: uv.KeyEscape, keyBackspace: uv.KeyBackspace, keyTab: uv.KeyTab, keyEnter: uv.KeyEnter, 96: uv.KeyEnter,
	102: uv.KeyHome, 103: uv.KeyUp, 104: uv.KeyPgUp, 105: uv.KeyLeft, 106: uv.KeyRight, 107: uv.KeyEnd,
	108: uv.KeyDown, 109: uv.KeyPgDown, 110: uv.KeyInsert, 111: uv.KeyDelete,
	59: uv.KeyF1, 60: uv.KeyF2, 61: uv.KeyF3, 62: uv.KeyF4, 63: uv.KeyF5, 64: uv.KeyF6,
	65: uv.KeyF7, 66: uv.KeyF8, 67: uv.KeyF9, 68: uv.KeyF10, keyF11: uv.KeyF11, keyF12: uv.KeyF12,
}

// termCell maps a screen point to a cell, if it's on the grid.
func (a *app) termCell(p image.Point) (int, int, bool) {
	t := &a.trm
	if !p.In(t.grid) || t.cw == 0 {
		return 0, 0, false
	}
	return (p.X - t.grid.Min.X) / t.cw, (p.Y - t.grid.Min.Y) / t.ch, true
}

// termTap clicks, if the remote app listens to the mouse.
func (a *app) termTap(p image.Point) bool {
	t := &a.trm
	if !t.connected() {
		for i, r := range t.hostRows {
			if p.In(r) {
				a.termConnect(t.hosts[i])
				return true
			}
		}
		return false
	}
	x, y, ok := a.termCell(p)
	if !ok {
		return false
	}
	m := uv.Mouse{X: x, Y: y, Button: uv.MouseLeft}
	t.emu.SendMouse(uv.MouseClickEvent(m))
	t.emu.SendMouse(uv.MouseReleaseEvent(m))
	return true
}

// termSwipe turns a vertical swipe into mouse wheel steps, one per row moved.
func (a *app) termSwipe(from, to image.Point) bool {
	t := &a.trm
	x, y, ok := a.termCell(from)
	if !t.connected() || !ok {
		return false
	}
	dy := to.Y - from.Y
	steps := min(20, abs(dy)/t.ch)
	if steps == 0 || abs(dy) < 2*abs(to.X-from.X) {
		return false
	}
	button := uv.MouseWheelUp // finger moves down: content scrolls back up
	if dy < 0 {
		button = uv.MouseWheelDown
	}
	for range steps {
		t.emu.SendMouse(uv.MouseWheelEvent(uv.Mouse{X: x, Y: y, Button: button}))
	}
	return true
}

// drawMissing marks a character we have no glyph for: a small hollow box.
func drawMissing(m *image.Alpha) {
	b := m.Bounds()
	r := image.Rect(b.Dx()/4, b.Dy()/4, b.Dx()*3/4, b.Dy()*3/4)
	for x := r.Min.X; x < r.Max.X; x++ {
		m.SetAlpha(x, r.Min.Y, color.Alpha{160})
		m.SetAlpha(x, r.Max.Y-1, color.Alpha{160})
	}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		m.SetAlpha(r.Min.X, y, color.Alpha{160})
		m.SetAlpha(r.Max.X-1, y, color.Alpha{160})
	}
}
