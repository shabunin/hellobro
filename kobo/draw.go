package kobo

// Rendering: fonts, drawing primitives, panels, and the app's frame: the
// header, the tabs and the footer.

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"log"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Gray levels the panel shows cleanly (multiples of 0x11).
const (
	Black = 0x00
	Dark  = 0x44
	Mid   = 0x88
	Light = 0xDD
	White = 0xFF
)

type Fonts struct {
	Title, Tab, TabBold, Panel, Body, Small, Mono, Huge font.Face
}

// Face is the font in ttf, px pixels tall.
func Face(ttf []byte, px float64) font.Face {
	f, err := opentype.Parse(ttf)
	if err != nil {
		log.Fatalf("font: %v", err)
	}
	fc, err := opentype.NewFace(f, &opentype.FaceOptions{Size: px, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		log.Fatalf("font: %v", err)
	}
	return fc
}

// LoadFonts makes the UI's fonts.
func LoadFonts() Fonts {
	return Fonts{
		Title: Face(gobold.TTF, 40), Tab: Face(goregular.TTF, 30), TabBold: Face(gobold.TTF, 30),
		Panel: Face(gobold.TTF, 26), Body: Face(goregular.TTF, 26), Small: Face(goregular.TTF, 22),
		Mono: Face(gomono.TTF, 28), Huge: Face(gobold.TTF, 110),
	}
}

// --- primitives ---

type Canvas struct{ *image.Gray }

// Blank is a white image of size.
func Blank(size image.Point) *image.Gray {
	img := image.NewGray(image.Rectangle{Max: size})
	Canvas{img}.Fill(img.Bounds(), White)
	return img
}

// Fill paints r gray g: a row, then copies of it. (draw.Draw has no fast
// way for a Gray image: a pixel at a time, 30× slower on the Kobo.)
func (c Canvas) Fill(r image.Rectangle, g uint8) {
	r = r.Intersect(c.Rect)
	if r.Empty() {
		return
	}
	row := c.Pix[c.PixOffset(r.Min.X, r.Min.Y):][:r.Dx()]
	for i := range row {
		row[i] = g
	}
	for y := r.Min.Y + 1; y < r.Max.Y; y++ {
		copy(c.Pix[c.PixOffset(r.Min.X, y):], row)
	}
}

// CopyGray copies src's pixels from sp on into dst's r, a row at a time
// (draw.Draw does it a pixel at a time for Gray images).
func CopyGray(dst *image.Gray, r image.Rectangle, src *image.Gray, sp image.Point) {
	clipped := r.Intersect(dst.Rect).Intersect(src.Rect.Sub(sp).Add(r.Min))
	if clipped.Empty() {
		return
	}
	sp = sp.Add(clipped.Min.Sub(r.Min))
	for y := 0; y < clipped.Dy(); y++ {
		copy(dst.Pix[dst.PixOffset(clipped.Min.X, clipped.Min.Y+y):][:clipped.Dx()], src.Pix[src.PixOffset(sp.X, sp.Y+y):])
	}
}

// Mask paints gray g into r through mask (at mp), as draw.DrawMask with
// draw.Over does, but fast for the glyphs' *image.Alpha.
func (c Canvas) Mask(r image.Rectangle, g uint8, mask image.Image, mp image.Point) {
	m, ok := mask.(*image.Alpha)
	if !ok {
		draw.DrawMask(c.Gray, r, image.NewUniform(color.Gray{g}), image.Point{}, mask, mp, draw.Over)
		return
	}
	clipped := r.Intersect(c.Rect).Intersect(m.Rect.Sub(mp).Add(r.Min))
	if clipped.Empty() {
		return
	}
	mp = mp.Add(clipped.Min.Sub(r.Min))
	src := int(g)
	for y := 0; y < clipped.Dy(); y++ {
		dst := c.Pix[c.PixOffset(clipped.Min.X, clipped.Min.Y+y):][:clipped.Dx()]
		as := m.Pix[m.PixOffset(mp.X, mp.Y+y):][:len(dst)]
		for i, a := range as {
			switch a {
			case 0:
			case 0xff:
				dst[i] = g
			default:
				d := int(dst[i])
				dst[i] = uint8(d + (src-d)*int(a)/0xff)
			}
		}
	}
}

// Frame draws a t pixels thick border just inside r.
func (c Canvas) Frame(r image.Rectangle, t int, g uint8) {
	c.Fill(image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+t), g)
	c.Fill(image.Rect(r.Min.X, r.Max.Y-t, r.Max.X, r.Max.Y), g)
	c.Fill(image.Rect(r.Min.X, r.Min.Y, r.Min.X+t, r.Max.Y), g)
	c.Fill(image.Rect(r.Max.X-t, r.Min.Y, r.Max.X, r.Max.Y), g)
}

const (
	AlignLeft = iota
	AlignCenter
	AlignRight
)

// Text draws s vertically centered in r, clipped to r and shortened with
// an ellipsis if it doesn't fit.
func (c Canvas) Text(f font.Face, r image.Rectangle, s string, g uint8, align int) {
	if r.Empty() {
		return
	}
	s = fit(f, s, r.Dx())
	w := font.MeasureString(f, s).Ceil()
	x := r.Min.X
	switch align {
	case AlignCenter:
		x += (r.Dx() - w) / 2
	case AlignRight:
		x = r.Max.X - w
	}
	m := f.Metrics()
	baseline := r.Min.Y + (r.Dy()+m.Ascent.Ceil()-m.Descent.Ceil())/2
	// font.Drawer's DrawString, with our Mask for its draw.DrawMask.
	dot, prev := fixed.P(x, baseline), rune(-1)
	for _, ch := range s {
		if prev >= 0 {
			dot.X += f.Kern(prev, ch)
		}
		dr, mask, mp, advance, ok := glyph(f, dot, ch)
		if ok {
			if clip := dr.Intersect(r); !clip.Empty() {
				c.Mask(clip, g, mask, mp.Add(clip.Min.Sub(dr.Min)))
			}
		}
		dot.X += advance
		prev = ch
	}
}

// A glyph's mask, as the face gave it at a dot's fraction of a pixel.
type glyphKey struct {
	f    font.Face
	ch   rune
	x, y fixed.Int26_6 // the dot's fraction
}

type cachedGlyph struct {
	dr      image.Rectangle // from the dot, floored
	mask    *image.Alpha    // dr's size, at 0, 0
	advance fixed.Int26_6
	ok      bool
}

// The glyphs drawn so far: opentype rasterizes each anew, a third of
// drawing a page. It's emptied when it gets big (a few MB).
var (
	glyphMu    sync.Mutex
	glyphCache = map[glyphKey]cachedGlyph{}
)

const glyphCacheMax = 8192

// glyph is f.Glyph, from the cache when it can.
func glyph(f font.Face, dot fixed.Point26_6, ch rune) (image.Rectangle, image.Image, image.Point, fixed.Int26_6, bool) {
	k := glyphKey{f, ch, dot.X & 63, dot.Y & 63}
	at := image.Pt(dot.X.Floor(), dot.Y.Floor())
	glyphMu.Lock()
	defer glyphMu.Unlock()
	if g, ok := glyphCache[k]; ok {
		return g.dr.Add(at), g.mask, image.Point{}, g.advance, g.ok
	}
	dr, mask, mp, advance, ok := f.Glyph(dot, ch)
	a, isAlpha := mask.(*image.Alpha)
	if !isAlpha && ok {
		return dr, mask, mp, advance, ok // not ours to keep
	}
	g := cachedGlyph{dr: dr.Sub(at), advance: advance, ok: ok}
	if ok {
		// The face draws its next glyph into the same mask: keep a copy.
		g.mask = image.NewAlpha(image.Rect(0, 0, dr.Dx(), dr.Dy()))
		for y := 0; y < dr.Dy(); y++ {
			copy(g.mask.Pix[y*g.mask.Stride:][:dr.Dx()], a.Pix[a.PixOffset(mp.X, mp.Y+y):])
		}
	}
	if len(glyphCache) >= glyphCacheMax {
		clear(glyphCache)
	}
	glyphCache[k] = g
	return g.dr.Add(at), g.mask, image.Point{}, advance, ok
}

func fit(f font.Face, s string, w int) string {
	if font.MeasureString(f, s).Ceil() <= w {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && font.MeasureString(f, string(r)+"…").Ceil() > w {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}

func TextWidth(f font.Face, s string) int { return font.MeasureString(f, s).Ceil() }

func (c Canvas) Disc(p image.Point, radius int, g uint8, clip image.Rectangle) {
	r2 := radius*radius + radius // a bit rounder for small radii
	for y := -radius; y <= radius; y++ {
		for x := -radius; x <= radius; x++ {
			if q := p.Add(image.Pt(x, y)); x*x+y*y <= r2 && q.In(clip) {
				c.SetGray(q.X, q.Y, color.Gray{g})
			}
		}
	}
}

func (c Canvas) Line(a, b image.Point, radius int, g uint8, clip image.Rectangle) {
	d := b.Sub(a)
	steps := max(Abs(d.X), Abs(d.Y), 1)
	for i := 0; i <= steps; i++ {
		c.Disc(image.Pt(a.X+d.X*i/steps, a.Y+d.Y*i/steps), radius, g, clip)
	}
}

// target draws a calibration crosshair.
func (c Canvas) target(p image.Point) {
	for y := -46; y <= 46; y++ {
		for x := -46; x <= 46; x++ {
			if d := x*x + y*y; d <= 46*46 && d >= 40*40 {
				c.SetGray(p.X+x, p.Y+y, color.Gray{Black})
			}
		}
	}
	c.Fill(image.Rect(p.X-70, p.Y-1, p.X+71, p.Y+2), Black)
	c.Fill(image.Rect(p.X-1, p.Y-70, p.X+2, p.Y+71), Black)
	c.Disc(p, 6, Black, c.Bounds())
}

// --- layout ---

// Panel is a framed box with a title strip, and maybe a button in it.
type Panel struct {
	Name               string
	Outer, Title, Body image.Rectangle
	Button             image.Rectangle // optional, inside the title strip
	ButtonLabel        string
	Fold               image.Rectangle // the − / + box at the strip's end, if it folds
	Folded             bool            // only the title strip shows
	Close              image.Rectangle // a window's ×, at the strip's end
	New                image.Rectangle // a window's +, after its name: a window next to it
	Focused            bool            // the keys go to it: its strip is black
}

// Layout is where things go on the screen.
type Layout struct {
	W, H      int
	Landscape bool
	Full      bool            // the page has the whole screen
	Page      image.Rectangle // between the tab bar and the footer
	Body      image.Rectangle // the page less padding: where panels go

	header, tabBar, footer, status, titleText image.Rectangle
	tabs                                      []image.Rectangle
	btnRotate, btnCalib, btnExit, btnSleep    image.Rectangle
}

const (
	Pad       = 16
	headerH   = 76
	tabH      = 68
	footerH   = 46
	titleH    = 48
	border    = 3
	FoldH     = titleH + 2*border // a folded panel: its frame and title strip
	tapMargin = 12                // buttons accept taps a little outside their frame
)

// makeLayout places the frame; full gives the whole screen to the page.
func makeLayout(w, h int, f Fonts, title string, tabs int, full bool) Layout {
	L := Layout{W: w, H: h, Landscape: w > h, Full: full}
	if full { // no header, tabs or footer: nothing else to draw or tap
		L.Page = image.Rect(0, 0, w, h)
		L.Body = L.Page
		return L
	}
	L.header = image.Rect(0, 0, w, headerH)
	L.tabBar = image.Rect(0, headerH, w, headerH+tabH)
	L.footer = image.Rect(0, h-footerH, w, h)
	L.Page = image.Rect(0, L.tabBar.Max.Y, w, L.footer.Min.Y)
	L.Body = L.Page.Inset(Pad)

	n := max(1, tabs)
	tw := min(w/n, w/4) // a few tabs needn't stretch across
	for i := range n {
		L.tabs = append(L.tabs, image.Rect(i*tw, L.tabBar.Min.Y, (i+1)*tw, L.tabBar.Max.Y))
	}

	// Header buttons, right to left.
	x := w - Pad
	button := func(label string) image.Rectangle {
		bw := TextWidth(f.Small, label) + 40
		r := image.Rect(x-bw, (headerH-48)/2, x, (headerH+48)/2)
		x = r.Min.X - 12
		return r
	}
	L.btnExit = button("exit")
	L.btnSleep = button("sleep")
	L.btnCalib = button("touch cal")
	L.btnRotate = button("rotate")
	titleEnd := 24 + TextWidth(f.Title, title) + 24
	L.titleText = image.Rect(24, 0, titleEnd, headerH)
	L.status = image.Rect(titleEnd, 0, x-12, headerH)
	return L
}

// Split cuts r in two at frac of the way across (landscape) or down (portrait).
func (L Layout) Split(r image.Rectangle, frac float64) (image.Rectangle, image.Rectangle) {
	if L.Landscape {
		at := r.Min.X + int(float64(r.Dx())*frac)
		return image.Rect(r.Min.X, r.Min.Y, at-Pad/2, r.Max.Y), image.Rect(at+Pad/2, r.Min.Y, r.Max.X, r.Max.Y)
	}
	at := r.Min.Y + int(float64(r.Dy())*frac)
	return image.Rect(r.Min.X, r.Min.Y, r.Max.X, at-Pad/2), image.Rect(r.Min.X, at+Pad/2, r.Max.X, r.Max.Y)
}

// Stack splits r top to bottom, at percentages, with gaps between.
func (L Layout) Stack(r image.Rectangle, fracs ...int) []image.Rectangle {
	var out []image.Rectangle
	top := r.Min.Y
	for _, f := range append(fracs, 100) {
		bot := r.Min.Y + r.Dy()*f/100
		out = append(out, image.Rect(r.Min.X, top, r.Max.X, bot))
		top = bot
	}
	for i := range out {
		if i > 0 {
			out[i].Min.Y += Pad / 2
		}
		if i < len(out)-1 {
			out[i].Max.Y -= Pad / 2
		}
	}
	return out
}

func NewPanel(f Fonts, name string, outer image.Rectangle, buttonLabel string) Panel {
	p := Panel{Name: name, Outer: outer}
	p.Title = image.Rect(outer.Min.X+border, outer.Min.Y+border, outer.Max.X-border, outer.Min.Y+border+titleH)
	p.Body = image.Rect(outer.Min.X+border, p.Title.Max.Y, outer.Max.X-border, outer.Max.Y-border)
	return WithButton(f, p, buttonLabel)
}

// WithButton is p with a button labeled label in its title strip, or none.
func WithButton(f Fonts, p Panel, label string) Panel {
	p.ButtonLabel, p.Button = label, image.Rectangle{}
	if label != "" {
		end := p.Title.Max.X - 10
		if !p.Fold.Empty() {
			end = p.Fold.Min.X - 12
		}
		if !p.Close.Empty() {
			end = p.Close.Min.X - 12
		}
		bw := TextWidth(f.Small, label) + 36
		p.Button = image.Rect(end-bw, p.Title.Min.Y+6, end, p.Title.Max.Y-8)
	}
	return p
}

// WithFold is p with a box at the end of its title strip: − folds it to
// the strip, + (folded) opens it again.
func WithFold(f Fonts, p Panel, folded bool) Panel {
	s := p.Title.Dy() - 14
	p.Fold = image.Rect(p.Title.Max.X-10-s, p.Title.Min.Y+6, p.Title.Max.X-10, p.Title.Max.Y-8)
	p.Folded = folded
	return WithButton(f, p, p.ButtonLabel)
}

// WithWindow is p as a window's: a × at the end of its strip that
// closes it, and with newBox a + after its name, for a window next to it.
func WithWindow(f Fonts, p Panel, focused, newBox bool) Panel {
	if p.Title.Empty() {
		return p
	}
	s := p.Title.Dy() - 14
	p.Close = image.Rect(p.Title.Max.X-10-s, p.Title.Min.Y+6, p.Title.Max.X-10, p.Title.Max.Y-8)
	p.New, p.Focused = image.Rectangle{}, focused
	if newBox {
		x := p.Title.Min.X + 16 + TextWidth(f.Panel, p.Name) + 24
		p.New = image.Rect(x, p.Title.Min.Y+6, x+44, p.Title.Max.Y-8)
	}
	return WithButton(f, p, p.ButtonLabel)
}

// FoldHit says whether a tap at pt folds or opens p: on its box, or
// anywhere on the strip of a folded one (but its button).
func FoldHit(pt image.Point, p Panel) bool {
	switch {
	case p.Fold.Empty() || Hit(pt, p.Button):
		return false
	case p.Folded:
		return pt.In(p.Outer)
	}
	return Hit(pt, p.Fold)
}

// Row is a row of panels for Arrange: its share of the height (percent,
// as Stack takes them), its panels' shares of its width (nil: one panel),
// and which of them are folded.
type Row struct {
	Height int
	Widths []int
	Folded []bool
}

// Arrange places a page's panels, row by row, as Stack and Split would.
// A folded panel is only its strip, FoldH high, across its row's place:
// above the open ones if it comes before them, below if after. The rest
// share what it leaves. The places come back in order, row by row.
func (L Layout) Arrange(r image.Rectangle, rows ...Row) []image.Rectangle {
	type item struct {
		row, panel int // panel < 0: the row's open panels
	}
	var items []item
	var weights []int
	flex := r
	n := 0
	for i, row := range rows {
		if row.Widths == nil {
			rows[i].Widths = []int{100}
		}
		open := false
		for j := range rows[i].Widths {
			n++
			switch {
			case j < len(row.Folded) && row.Folded[j]:
				items = append(items, item{i, j})
				flex.Max.Y -= FoldH + Pad
			case !open: // the first open one: the open ones go here
				open = true
				items = append(items, item{i, -1})
				weights = append(weights, row.Height)
			}
		}
	}
	// The open rows' heights, cut as Stack cuts.
	total := 0
	for _, w := range weights {
		total += w
	}
	var heights []int
	cum, top := 0, flex.Min.Y
	for k, w := range weights {
		cum += w
		bot := flex.Min.Y + flex.Dy()*cum/max(1, total)
		lo, hi := top, bot
		if k > 0 {
			lo += Pad / 2
		}
		if k < len(weights)-1 {
			hi -= Pad / 2
		}
		heights = append(heights, hi-lo)
		top = bot
	}

	out := make([]image.Rectangle, n)
	first := make([]int, len(rows)) // where each row's panels start in out
	for i := 1; i < len(rows); i++ {
		first[i] = first[i-1] + len(rows[i-1].Widths)
	}
	y, k := r.Min.Y, 0
	for _, it := range items {
		if it.panel >= 0 {
			out[first[it.row]+it.panel] = image.Rect(r.Min.X, y, r.Max.X, y+FoldH)
			y += FoldH + Pad
			continue
		}
		row := rows[it.row]
		h := heights[k]
		k++
		var open []int
		wsum := 0
		for j, w := range row.Widths {
			if j >= len(row.Folded) || !row.Folded[j] {
				open = append(open, j)
				wsum += w
			}
		}
		// Side by side, cut as Split cuts.
		left, c := r.Min.X, 0
		for m, j := range open {
			c += row.Widths[j]
			right := r.Min.X + r.Dx()*c/max(1, wsum)
			lo, hi := left, right
			if m > 0 {
				lo += Pad / 2
			}
			if m < len(open)-1 {
				hi -= Pad / 2
			}
			out[first[it.row]+j] = image.Rect(lo, y, hi, y+h)
			left = right
		}
		y += h + Pad
	}
	return out
}

// Hit says whether p is on the button r, give or take a little.
func Hit(p image.Point, r image.Rectangle) bool {
	return !r.Empty() && p.In(r.Inset(-tapMargin))
}

// --- drawing the app ---

func (a *App) drawAll(flash bool) {
	a.lightOverlay, a.gaugeOverlay = nil, nil
	a.C.Fill(a.C.Bounds(), White)
	a.drawHeader()
	a.drawTabs()
	a.DrawPage()
	a.drawFooter()
	a.markAll(flash)
}

func (a *App) drawHeader() {
	if a.Full {
		return
	}
	c, L := a.C, a.L
	c.Fill(L.header, Black)
	c.Text(a.F.Title, L.titleText, a.cfg.Title, White, AlignLeft)
	for _, b := range []struct {
		r     image.Rectangle
		label string
	}{{L.btnRotate, "rotate"}, {L.btnCalib, "touch cal"}, {L.btnSleep, "sleep"}, {L.btnExit, "exit"}} {
		c.Frame(b.r, 2, White)
		c.Text(a.F.Small, b.r, b.label, White, AlignCenter)
	}
	a.status = ""
	a.drawStatus()
	a.Mark(L.header, true)
}

// SetStatus puts a note in the header, left of the battery and clock
// ("" for none): what's playing, say.
func (a *App) SetStatus(note string) {
	if note != a.note {
		a.note = note
		a.drawStatus()
	}
}

// SetNetwork puts the network first in the header's status, where the
// orientation goes otherwise: "home-wifi · 192.168.1.20".
func (a *App) SetNetwork(s string) {
	if s != a.network {
		a.network = s
		a.drawStatus()
	}
}

// Notify puts text in the header for a few seconds, over the note:
// "copied 3 lines".
func (a *App) Notify(text string) {
	log.Printf("notice: %s", text)
	a.notice, a.noticeUntil = text, time.Now().Add(3*time.Second)
	a.drawStatus()
}

// drawStatus refreshes the note, battery & clock text, if they changed.
func (a *App) drawStatus() {
	s := statusText(a.scr.info, a.network)
	note := a.note
	if a.notice != "" {
		note = a.notice
	}
	if s+"\x00"+note == a.status || a.calib != nil || a.Full {
		return
	}
	a.status = s + "\x00" + note
	r := a.L.status
	a.C.Fill(r, Black)
	a.C.Text(a.F.Small, r, s, Light, AlignRight)
	if note != "" {
		r.Max.X -= TextWidth(a.F.Small, s) + 28
		a.C.Text(a.F.Small, r, note, White, AlignRight)
	}
	a.Mark(a.L.status, true)
}

func statusText(s ScreenInfo, network string) string {
	first := network
	switch {
	case first != "":
	case s.Landscape():
		first = "landscape"
	default:
		first = "portrait"
	}
	parts := []string{first}
	if b, err := os.ReadFile("/sys/class/power_supply/battery/capacity"); err == nil {
		bat := strings.TrimSpace(string(b)) + "%"
		if st, err := os.ReadFile("/sys/class/power_supply/battery/status"); err == nil && strings.TrimSpace(string(st)) == "Charging" {
			bat += " charging"
		}
		parts = append(parts, bat)
	}
	parts = append(parts, time.Now().Format("15:04"))
	return strings.Join(parts, "  ·  ")
}

func (a *App) drawTabs() {
	if a.Full {
		return
	}
	c, L := a.C, a.L
	c.Fill(L.tabBar, Light)
	order := a.Tabs()
	for i, r := range L.tabs {
		label := a.cfg.Tabs[order[i]]
		if order[i] == a.Tab {
			c.Fill(r, White)
			c.Frame(r, border, Black)
			c.Fill(image.Rect(r.Min.X+border, r.Max.Y-border, r.Max.X-border, r.Max.Y), White) // open into the page
			c.Text(a.F.TabBold, r, label, Black, AlignCenter)
		} else {
			c.Fill(image.Rect(r.Min.X, r.Max.Y-border, r.Max.X, r.Max.Y), Black)
			if i > 0 {
				c.Fill(image.Rect(r.Min.X, r.Min.Y+14, r.Min.X+1, r.Max.Y-14), Mid)
			}
			c.Text(a.F.Tab, r, label, Dark, AlignCenter)
		}
		hint := strconv.Itoa(i + 1) // Ctrl+Alt+N
		if a.hints != nil {
			hint = ""
			if i < len(a.hints) {
				hint = a.hints[i]
			}
		}
		c.Text(a.F.Small, image.Rect(r.Max.X-90, r.Min.Y+4, r.Max.X-10, r.Min.Y+30), hint, Mid, AlignRight)
	}
	if end := L.tabs[len(L.tabs)-1].Max.X; end < L.W { // the bar's line goes on past the tabs
		c.Fill(image.Rect(end, L.tabBar.Max.Y-border, L.W, L.tabBar.Max.Y), Black)
	}
	a.Mark(L.tabBar, true)
}

// DrawFooter redraws the footer, for an app whose idle timeout changed.
func (a *App) DrawFooter() {
	if a.calib == nil && a.C.Gray != nil && !a.L.footer.Empty() { // not before the first layout
		a.drawFooter()
	}
}

func (a *App) drawFooter() {
	if a.Full {
		return
	}
	c, L := a.C, a.L
	c.Fill(L.footer, White)
	c.Fill(image.Rect(0, L.footer.Min.Y, L.W, L.footer.Min.Y+2), Black)
	var ways []string
	if !a.noPower {
		ways = append(ways, "power button")
	}
	switch a.footerSleep {
	case "":
		ways = append(ways, "Ctrl+Alt+S")
	case "-": // no key for it
	default:
		ways = append(ways, a.footerSleep)
	}
	sleep := "the header's button"
	if len(ways) > 0 {
		sleep = strings.Join(ways, " or ")
	}
	tabs := fmt.Sprintf("tabs: tap or Ctrl+Alt+1–%d", len(a.Tabs()))
	if a.footerTabs != "" {
		tabs = a.footerTabs
	}
	hints := []string{tabs}
	if a.light != nil {
		hints = append(hints, "light: swipe the left edge")
	}
	exit := fmt.Sprintf("auto-exit after %.0f min idle", IdleTimeout.Minutes())
	if a.prog.IdleTimeout() >= 24*time.Hour { // the app wants to stay: say so
		exit = "no auto-exit"
	}
	quit := "Esc or exit"
	switch a.footerQuit {
	case "":
	case "-":
		quit = "the exit button"
	default:
		quit = a.footerQuit + " or exit"
	}
	hints = append(hints, "sleep: "+sleep, "quit: "+quit, exit)
	c.Text(a.F.Small, image.Rect(Pad, L.footer.Min.Y+2, L.W-Pad, L.footer.Max.Y), strings.Join(hints, "   ·   "), Dark, AlignCenter)
	a.Mark(L.footer, true)
}

// DrawPage redraws the current tab's page.
func (a *App) DrawPage() {
	a.C.Fill(a.L.Page, White)
	a.prog.Draw()
	a.Mark(a.L.Page, true)
}

// DrawPanel draws the frame and title strip, and clears the body.
func (a *App) DrawPanel(p Panel, status string) {
	c := a.C
	c.Fill(p.Outer, White)
	if p.Title.Empty() { // a fullscreen page: no frame
		return
	}
	c.Frame(p.Outer, border, Black)
	a.DrawPanelTitle(p, status)
}

// DrawPanelTitle draws the title strip: the name, the button, and status
// on the right.
func (a *App) DrawPanelTitle(p Panel, status string) {
	if p.Title.Empty() {
		return
	}
	c := a.C
	bg, ink, dim := uint8(Light), uint8(Black), uint8(Dark)
	if p.Focused {
		bg, ink, dim = Black, White, Light
	}
	c.Fill(p.Title, bg)
	c.Fill(image.Rect(p.Title.Min.X, p.Title.Max.Y-2, p.Title.Max.X, p.Title.Max.Y), Black)
	t := image.Rect(p.Title.Min.X+16, p.Title.Min.Y, p.Title.Max.X-16, p.Title.Max.Y-2)
	box := func(r image.Rectangle) {
		c.Fill(r, White)
		c.Frame(r, 2, Black)
	}
	if f := p.Close; !f.Empty() {
		box(f)
		m := f.Min.Add(f.Max).Div(2)
		for d := -8; d <= 8; d++ { // ×
			c.Fill(image.Rect(m.X+d-1, m.Y+d-1, m.X+d+2, m.Y+d+2), Black)
			c.Fill(image.Rect(m.X+d-1, m.Y-d-1, m.X+d+2, m.Y-d+2), Black)
		}
		t.Max.X = f.Min.X - 16
	}
	if f := p.Fold; !f.Empty() {
		c.Fill(f, White)
		c.Frame(f, 2, Black)
		m := f.Min.Add(f.Max).Div(2)
		c.Fill(image.Rect(m.X-9, m.Y-1, m.X+10, m.Y+2), Black) // −
		if p.Folded {
			c.Fill(image.Rect(m.X-1, m.Y-9, m.X+2, m.Y+10), Black) // +
		}
		t.Max.X = f.Min.X - 16
	}
	if p.ButtonLabel != "" {
		c.Fill(p.Button, White)
		c.Frame(p.Button, 2, Black)
		c.Text(a.F.Small, p.Button, p.ButtonLabel, Black, AlignCenter)
		t.Max.X = p.Button.Min.X - 16
	}
	nameW := TextWidth(a.F.Panel, p.Name)
	c.Text(a.F.Panel, image.Rect(t.Min.X, t.Min.Y, t.Min.X+nameW+2, t.Max.Y), p.Name, ink, AlignLeft)
	if n := p.New; !n.Empty() {
		box(n)
		m := n.Min.Add(n.Max).Div(2)
		c.Fill(image.Rect(m.X-9, m.Y-1, m.X+10, m.Y+2), Black) // +
		c.Fill(image.Rect(m.X-1, m.Y-9, m.X+2, m.Y+10), Black)
		nameW = n.Max.X - t.Min.X - 12
	}
	c.Text(a.F.Small, image.Rect(t.Min.X+nameW+28, t.Min.Y, t.Max.X, t.Max.Y), status, dim, AlignRight)
	a.Mark(p.Title, true)
}

// DrawLines fills a panel body with the newest lines that fit.
func (a *App) DrawLines(p Panel, lines []string, empty string) {
	a.DrawText(p, a.F.Body, lines, empty, false)
}

// DrawText fills a panel body with lines: the newest that fit (logs), or
// the first ones (reports).
// It returns where each line went (empty for lines that didn't fit), for taps.
func (a *App) DrawText(p Panel, f font.Face, lines []string, empty string, fromTop bool) []image.Rectangle {
	b := p.Body
	a.C.Fill(b, White)
	in := b.Inset(Pad)
	lh := f.Metrics().Height.Ceil() + 8
	n := max(0, in.Dy()/lh)
	col := uint8(Black)
	rows := make([]image.Rectangle, len(lines))
	first := 0
	if len(lines) == 0 {
		lines, col = []string{empty}, Mid
	}
	if len(lines) > n {
		if fromTop {
			lines = append(slices.Clone(lines[:max(0, n-1)]), fmt.Sprintf("… %d more lines", len(lines)-n+1))
		} else {
			first = len(lines) - n
			lines = lines[first:]
		}
	}
	for i, s := range lines {
		r := image.Rect(in.Min.X, in.Min.Y+i*lh, in.Max.X, in.Min.Y+(i+1)*lh)
		a.C.Text(f, r, s, col, AlignLeft)
		if first+i < len(rows) && !(fromTop && len(lines) < len(rows) && i == len(lines)-1 && len(rows) > n) {
			rows[first+i] = r
		}
	}
	a.Mark(b, true)
	return rows
}

// DrawTextRows is DrawText from the top, repainting only the rows that
// differ from *shown (all of them when it's nil): for text being typed.
func (a *App) DrawTextRows(p Panel, f font.Face, lines []string, shown *[]string) []image.Rectangle {
	in := p.Body.Inset(Pad)
	lh := f.Metrics().Height.Ceil() + 8
	n := max(1, in.Dy()/lh)
	rows := make([]image.Rectangle, len(lines))
	if len(lines) > n {
		lines = append(slices.Clone(lines[:n-1]), fmt.Sprintf("… %d more lines", len(lines)-n+1))
	}
	full := *shown == nil
	if full {
		a.C.Fill(p.Body, White)
		a.Mark(p.Body, true)
	}
	for i := range max(len(lines), len(*shown)) {
		r := image.Rect(in.Min.X, in.Min.Y+i*lh, in.Max.X, in.Min.Y+(i+1)*lh)
		s := ""
		if i < len(lines) {
			s = lines[i]
			if i < len(rows) && !(len(rows) > n && i == n-1) {
				rows[i] = r
			}
		}
		if !full && i < len(*shown) && (*shown)[i] == s {
			continue
		}
		if !full {
			a.C.Fill(r, White)
			a.Mark(r, true)
		}
		a.C.Text(f, r, s, Black, AlignLeft)
	}
	*shown = slices.Clone(lines)
	return rows
}

// InfoLines describes the input devices, the screen and the touch panel.
func (a *App) InfoLines() (devices, screen, touch []string) {
	s := a.scr.info
	for _, d := range a.devs {
		line := d.handler + "  " + d.name
		if r := d.role(); r != "" {
			line += "  (" + r + ")"
		}
		devices = append(devices, line)
	}

	orient := "portrait"
	if s.Landscape() {
		orient = "landscape"
	}
	screen = []string{
		fmt.Sprintf("%s, %d x %d px", orient, s.ViewW, s.ViewH),
		fmt.Sprintf("rotation: native %d, canonical %d", s.Rota, s.Canonical),
		fmt.Sprintf("device: %s", s.Codename),
		fmt.Sprintf("FBInk %s", s.Version),
		a.lightInfo(),
	}

	if a.touch == nil {
		return devices, screen, []string{"no touch panel found"}
	}
	t := a.touch
	src := "FBInk default"
	if _, ok := a.cal[s.Rota]; ok {
		src = "calibrated"
	}
	touch = []string{
		t.name,
		fmt.Sprintf("raw x %d..%d, y %d..%d", a.tmap.rx.min, a.tmap.rx.max, a.tmap.ry.min, a.tmap.ry.max),
		fmt.Sprintf("pen pressure up to %d", a.PressureMax()),
		fmt.Sprintf("mapping: %s (%s)", a.tmap.axes, src),
		fmt.Sprintf("FBInk default would be: %s", defaultAxes(s)),
	}
	if a.calibNote != "" {
		touch = append(touch, a.calibNote)
	}
	return devices, screen, touch
}

func (a *App) drawCalib() {
	c := a.C
	b := c.Bounds()
	c.Fill(b, White)
	c.Fill(image.Rect(0, 0, b.Dx(), headerH), Black)
	c.Text(a.F.Title, image.Rect(24, 0, b.Dx()-24, headerH), "touch calibration", White, AlignLeft)
	step := len(a.calib.raw)
	lines := []string{
		fmt.Sprintf("Tap the center of the target with your finger  (%d of %d)", step+1, len(calibrationTargets)),
		"This happens once for each screen orientation.",
	}
	if a.calib.note != "" {
		lines = append(lines, a.calib.note)
	}
	y := b.Dy() * 52 / 100
	for _, l := range lines {
		c.Text(a.F.Body, image.Rect(Pad, y, b.Dx()-Pad, y+40), l, Black, AlignCenter)
		y += 48
	}
	t := calibrationTargets[step]
	c.target(image.Pt(int(t[0]*float64(b.Dx())), int(t[1]*float64(b.Dy()))))
	a.markAll(true)
}

// drawCard fills the screen with a word and a line under it: hello as
// we start, bye as we go; the About lines at its foot.
func (a *App) drawCard(big, small string) {
	c := a.C
	b := c.Bounds()
	c.Fill(b, White)
	c.Text(a.F.Huge, image.Rect(0, b.Dy()/2-130, b.Dx(), b.Dy()/2+30), big, Black, AlignCenter)
	c.Text(a.F.Body, image.Rect(0, b.Dy()/2+60, b.Dx(), b.Dy()/2+110), small, Dark, AlignCenter)
	y := b.Dy() - 90 - 48*len(a.cfg.About)
	for _, l := range a.cfg.About {
		c.Text(a.F.Small, image.Rect(0, y, b.Dx(), y+44), l, Dark, AlignCenter)
		y += 48
	}
	a.markAll(true)
}

func (a *App) drawRestart() { a.drawCard(a.cfg.Title, "starting the new "+a.cfg.Name+"…") }
func (a *App) drawBye()     { a.drawCard("bye bro!", "back to Kobo…") }
