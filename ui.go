package main

// Rendering: fonts, drawing primitives, layout, and the tab pages.

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"log"
	"os"
	"slices"
	"strings"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Gray levels the panel shows cleanly (multiples of 0x11).
const (
	black = 0x00
	dark  = 0x44
	mid   = 0x88
	light = 0xDD
	white = 0xFF
)

type fonts struct {
	title, tab, tabBold, panel, body, small, mono, huge font.Face
	term, termBold, termFallback                        font.Face
}

func face(ttf []byte, px float64) font.Face {
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

func loadFonts() fonts {
	f := fonts{
		title: face(gobold.TTF, 40), tab: face(goregular.TTF, 30), tabBold: face(gobold.TTF, 30),
		panel: face(gobold.TTF, 26), body: face(goregular.TTF, 26), small: face(goregular.TTF, 22),
		mono: face(gomono.TTF, 28), huge: face(gobold.TTF, 110),
	}
	f.setTermSize(termFontDefault)
	return f
}

// setTermSize makes the terminal's faces px pixels tall.
func (f *fonts) setTermSize(px int) {
	f.term, f.termBold, f.termFallback = face(gomono.TTF, float64(px)), face(gomonobold.TTF, float64(px)), face(goregular.TTF, float64(px))
}

// --- primitives ---

type canvas struct{ *image.Gray }

func blank(size image.Point) *image.Gray {
	img := image.NewGray(image.Rectangle{Max: size})
	canvas{img}.fill(img.Bounds(), white)
	return img
}

func (c canvas) fill(r image.Rectangle, g uint8) {
	draw.Draw(c.Gray, r, image.NewUniform(color.Gray{g}), image.Point{}, draw.Src)
}

// frame draws a t pixels thick border just inside r.
func (c canvas) frame(r image.Rectangle, t int, g uint8) {
	c.fill(image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+t), g)
	c.fill(image.Rect(r.Min.X, r.Max.Y-t, r.Max.X, r.Max.Y), g)
	c.fill(image.Rect(r.Min.X, r.Min.Y, r.Min.X+t, r.Max.Y), g)
	c.fill(image.Rect(r.Max.X-t, r.Min.Y, r.Max.X, r.Max.Y), g)
}

const (
	alignLeft = iota
	alignCenter
	alignRight
)

// text draws s vertically centered in r, clipped to r and shortened with
// an ellipsis if it doesn't fit.
func (c canvas) text(f font.Face, r image.Rectangle, s string, g uint8, align int) {
	if r.Empty() {
		return
	}
	s = fit(f, s, r.Dx())
	w := font.MeasureString(f, s).Ceil()
	x := r.Min.X
	switch align {
	case alignCenter:
		x += (r.Dx() - w) / 2
	case alignRight:
		x = r.Max.X - w
	}
	m := f.Metrics()
	baseline := r.Min.Y + (r.Dy()+m.Ascent.Ceil()-m.Descent.Ceil())/2
	d := font.Drawer{Dst: c.SubImage(r).(*image.Gray), Src: image.NewUniform(color.Gray{g}), Face: f, Dot: fixed.P(x, baseline)}
	d.DrawString(s)
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

func textWidth(f font.Face, s string) int { return font.MeasureString(f, s).Ceil() }

func (c canvas) disc(p image.Point, radius int, g uint8, clip image.Rectangle) {
	r2 := radius*radius + radius // a bit rounder for small radii
	for y := -radius; y <= radius; y++ {
		for x := -radius; x <= radius; x++ {
			if q := p.Add(image.Pt(x, y)); x*x+y*y <= r2 && q.In(clip) {
				c.SetGray(q.X, q.Y, color.Gray{g})
			}
		}
	}
}

func (c canvas) line(a, b image.Point, radius int, g uint8, clip image.Rectangle) {
	d := b.Sub(a)
	steps := max(abs(d.X), abs(d.Y), 1)
	for i := 0; i <= steps; i++ {
		c.disc(image.Pt(a.X+d.X*i/steps, a.Y+d.Y*i/steps), radius, g, clip)
	}
}

// target draws a calibration crosshair.
func (c canvas) target(p image.Point) {
	for y := -46; y <= 46; y++ {
		for x := -46; x <= 46; x++ {
			if d := x*x + y*y; d <= 46*46 && d >= 40*40 {
				c.SetGray(p.X+x, p.Y+y, color.Gray{black})
			}
		}
	}
	c.fill(image.Rect(p.X-70, p.Y-1, p.X+71, p.Y+2), black)
	c.fill(image.Rect(p.X-1, p.Y-70, p.X+2, p.Y+71), black)
	c.disc(p, 6, black, c.Bounds())
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// --- layout ---

type tabID int

const (
	tabKeys tabID = iota
	tabFinger
	tabStylus
	tabNet
	tabSound
	tabTerm
	tabInfo
	numTabs
)

var tabNames = [numTabs]string{"Keys", "Finger", "Stylus", "Net", "Sound", "SSH", "Info"}

type panel struct {
	name               string
	outer, title, body image.Rectangle
	button             image.Rectangle // optional, inside the title strip
	buttonLabel        string
}

type layout struct {
	w, h                         int
	header, tabBar, page         image.Rectangle
	footer, status, titleText    image.Rectangle
	tabs                         [numTabs]image.Rectangle
	btnRotate, btnCalib, btnExit image.Rectangle

	typing, lastKey          panel
	fingerCanvas, fingerLog  panel
	stylusCanvas, stylusLog  panel
	devices, screenP, touchP panel
	net                      [numNetPanels]panel
	btP, playerP             panel
	termP                    panel
}

const (
	pad       = 16
	headerH   = 76
	tabH      = 68
	footerH   = 46
	titleH    = 48
	border    = 3
	tapMargin = 12 // buttons accept taps a little outside their frame
)

// makeLayout places everything; full gives the whole screen to the terminal.
func makeLayout(w, h int, f fonts, full bool) layout {
	L := layout{w: w, h: h}
	L.header = image.Rect(0, 0, w, headerH)
	L.tabBar = image.Rect(0, headerH, w, headerH+tabH)
	L.footer = image.Rect(0, h-footerH, w, h)
	L.page = image.Rect(0, L.tabBar.Max.Y, w, L.footer.Min.Y)
	body := L.page.Inset(pad)

	for i := range L.tabs {
		L.tabs[i] = image.Rect(i*w/int(numTabs), L.tabBar.Min.Y, (i+1)*w/int(numTabs), L.tabBar.Max.Y)
	}

	// Header buttons, right to left.
	x := w - pad
	button := func(label string) image.Rectangle {
		bw := textWidth(f.small, label) + 40
		r := image.Rect(x-bw, (headerH-48)/2, x, (headerH+48)/2)
		x = r.Min.X - 12
		return r
	}
	L.btnExit = button("exit")
	L.btnCalib = button("touch cal")
	L.btnRotate = button("rotate")
	titleEnd := 24 + textWidth(f.title, "hello bro") + 24
	L.titleText = image.Rect(24, 0, titleEnd, headerH)
	L.status = image.Rect(titleEnd, 0, x-12, headerH)

	landscape := w > h
	split := func(r image.Rectangle, frac float64) (image.Rectangle, image.Rectangle) {
		if landscape {
			at := r.Min.X + int(float64(r.Dx())*frac)
			return image.Rect(r.Min.X, r.Min.Y, at-pad/2, r.Max.Y), image.Rect(at+pad/2, r.Min.Y, r.Max.X, r.Max.Y)
		}
		at := r.Min.Y + int(float64(r.Dy())*frac)
		return image.Rect(r.Min.X, r.Min.Y, r.Max.X, at-pad/2), image.Rect(r.Min.X, at+pad/2, r.Max.X, r.Max.Y)
	}

	// Keyboard: a big text box and a "last key" strip.
	L.typing = newPanel(f, "Typing", image.Rect(body.Min.X, body.Min.Y, body.Max.X, body.Max.Y-140-pad), "")
	L.lastKey = newPanel(f, "Last key", image.Rect(body.Min.X, body.Max.Y-140, body.Max.X, body.Max.Y), "")

	cr, lr := split(body, 0.66)
	L.fingerCanvas = newPanel(f, "Finger canvas", cr, "clear")
	L.fingerLog = newPanel(f, "Finger log", lr, "")
	L.stylusCanvas = newPanel(f, "Stylus canvas", cr, "clear")
	L.stylusLog = newPanel(f, "Stylus log", lr, "")

	dr, rest := split(body, 0.45)
	if landscape { // devices on the left, screen above touch on the right
		at := rest.Min.Y + rest.Dy()*45/100
		L.screenP = newPanel(f, "Screen", image.Rect(rest.Min.X, rest.Min.Y, rest.Max.X, at-pad/2), "")
		L.touchP = newPanel(f, "Touch", image.Rect(rest.Min.X, at+pad/2, rest.Max.X, rest.Max.Y), "")
	} else {
		at := rest.Min.Y + rest.Dy()*40/100
		L.screenP = newPanel(f, "Screen", image.Rect(rest.Min.X, rest.Min.Y, rest.Max.X, at-pad/2), "")
		L.touchP = newPanel(f, "Touch", image.Rect(rest.Min.X, at+pad/2, rest.Max.X, rest.Max.Y), "")
	}
	L.devices = newPanel(f, "Input devices", dr, "")

	// Network: Wi-Fi status, scan, mDNS, HTTP test.
	stack := func(r image.Rectangle, fracs ...int) []image.Rectangle { // splits r top to bottom, at percentages
		var out []image.Rectangle
		top := r.Min.Y
		for _, f := range append(fracs, 100) {
			bot := r.Min.Y + r.Dy()*f/100
			out = append(out, image.Rect(r.Min.X, top, r.Max.X, bot))
			top = bot
		}
		for i := range out { // gaps between panels
			if i > 0 {
				out[i].Min.Y += pad / 2
			}
			if i < len(out)-1 {
				out[i].Max.Y -= pad / 2
			}
		}
		return out
	}
	var nr []image.Rectangle // wifi, scan, mdns, http
	if landscape {           // Wi-Fi above HTTPS on the left, scan above mDNS on the right
		left, right := split(body, 0.5)
		l, r := stack(left, 45), stack(right, 50)
		nr = []image.Rectangle{l[0], r[0], r[1], l[1]}
	} else {
		nr = stack(body, 24, 50, 76)
	}
	L.net[netWifi] = newPanel(f, "Wi-Fi", nr[0], "refresh")
	L.net[netScan] = newPanel(f, "Scan", nr[1], "scan")
	L.net[netMDNS] = newPanel(f, "mDNS", nr[2], "browse")
	L.net[netHTTP] = newPanel(f, "HTTPS", nr[3], "GET")

	// Sound: Bluetooth devices & PCMs, and the player.
	br, pr := split(body, 0.4)
	if landscape {
		br, pr = split(body, 0.5)
	}
	L.btP = newPanel(f, "Bluetooth", br, "refresh")
	L.playerP = newPanel(f, "Player", pr, "stop")

	// SSH: one big terminal.
	L.termP = newPanel(f, "Terminal", body, "disconnect")
	if full { // no header, tabs, footer or frame: nothing else to draw or tap
		whole := image.Rect(0, 0, w, h)
		L = layout{w: w, h: h, page: whole, termP: panel{name: "Terminal", outer: whole, body: whole}}
	}
	return L
}

func newPanel(f fonts, name string, outer image.Rectangle, buttonLabel string) panel {
	p := panel{name: name, outer: outer, buttonLabel: buttonLabel}
	p.title = image.Rect(outer.Min.X+border, outer.Min.Y+border, outer.Max.X-border, outer.Min.Y+border+titleH)
	p.body = image.Rect(outer.Min.X+border, p.title.Max.Y, outer.Max.X-border, outer.Max.Y-border)
	if buttonLabel != "" {
		bw := textWidth(f.small, buttonLabel) + 36
		p.button = image.Rect(p.title.Max.X-10-bw, p.title.Min.Y+6, p.title.Max.X-10, p.title.Max.Y-8)
	}
	return p
}

func hit(p image.Point, r image.Rectangle) bool {
	return !r.Empty() && p.In(r.Inset(-tapMargin))
}

// --- drawing the app ---

func (a *app) drawAll(flash bool) {
	a.lightOverlay, a.fontOverlay = nil, nil
	a.c.fill(a.c.Bounds(), white)
	a.drawHeader()
	a.drawTabs()
	a.drawPage()
	a.drawFooter()
	a.markAll(flash)
}

func (a *app) drawHeader() {
	if a.full {
		return
	}
	c, L := a.c, a.L
	c.fill(L.header, black)
	c.text(a.f.title, L.titleText, "hello bro", white, alignLeft)
	for _, b := range []struct {
		r     image.Rectangle
		label string
	}{{L.btnRotate, "rotate"}, {L.btnCalib, "touch cal"}, {L.btnExit, "exit"}} {
		c.frame(b.r, 2, white)
		c.text(a.f.small, b.r, b.label, white, alignCenter)
	}
	a.status = ""
	a.drawStatus()
	a.mark(L.header, true)
}

// drawStatus refreshes the battery & clock text, if it changed.
func (a *app) drawStatus() {
	s := statusText(a.scr.info)
	if s == a.status || a.calib != nil || a.full {
		return
	}
	a.status = s
	a.c.fill(a.L.status, black)
	a.c.text(a.f.small, a.L.status, s, light, alignRight)
	a.mark(a.L.status, true)
}

func statusText(s screenInfo) string {
	orient := "portrait"
	if s.landscape() {
		orient = "landscape"
	}
	parts := []string{orient}
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

func (a *app) drawTabs() {
	if a.full {
		return
	}
	c, L := a.c, a.L
	c.fill(L.tabBar, light)
	for i, r := range L.tabs {
		label := tabNames[i]
		if tabID(i) == a.tab {
			c.fill(r, white)
			c.frame(r, border, black)
			c.fill(image.Rect(r.Min.X+border, r.Max.Y-border, r.Max.X-border, r.Max.Y), white) // open into the page
			c.text(a.f.tabBold, r, label, black, alignCenter)
		} else {
			c.fill(image.Rect(r.Min.X, r.Max.Y-border, r.Max.X, r.Max.Y), black)
			if i > 0 {
				c.fill(image.Rect(r.Min.X, r.Min.Y+14, r.Min.X+1, r.Max.Y-14), mid)
			}
			c.text(a.f.tab, r, label, dark, alignCenter)
		}
		c.text(a.f.small, image.Rect(r.Max.X-60, r.Min.Y+4, r.Max.X-10, r.Min.Y+30), fmt.Sprintf("F%d", i+1), mid, alignRight)
	}
	a.mark(L.tabBar, true)
}

func (a *app) drawFooter() {
	if a.full {
		return
	}
	c, L := a.c, a.L
	c.fill(L.footer, white)
	c.fill(image.Rect(0, L.footer.Min.Y, L.w, L.footer.Min.Y+2), black)
	quit := "Esc, exit or power button"
	if a.noPower {
		quit = "Esc or exit"
	}
	hints := []string{"tabs: tap or F1-F7"}
	if a.light != nil {
		hints = append(hints, "light: swipe the left edge")
	}
	hints = append(hints, "quit: "+quit, "auto-exit after 3 min idle")
	c.text(a.f.small, image.Rect(pad, L.footer.Min.Y+2, L.w-pad, L.footer.Max.Y), strings.Join(hints, "   ·   "), dark, alignCenter)
	a.mark(L.footer, true)
}

func (a *app) drawPage() {
	c, L := a.c, a.L
	c.fill(L.page, white)
	switch a.tab {
	case tabKeys:
		a.drawPanel(L.typing, a.keyboardStatus())
		a.shown = nil
		a.drawTyping()
		a.drawPanel(L.lastKey, "")
		a.drawLastKey()
	case tabFinger:
		a.drawPanel(L.fingerCanvas, "pen is ignored here")
		draw.Draw(c.Gray, L.fingerCanvas.body, a.fingerImg, image.Point{}, draw.Src)
		a.drawPanel(L.fingerLog, fmt.Sprintf("%d", len(a.fingerLog)))
		a.drawLines(L.fingerLog, a.fingerLog, "Draw or tap with a finger on the canvas.")
	case tabStylus:
		a.drawPanel(L.stylusCanvas, a.penStatus)
		a.penShown = a.penStatus
		draw.Draw(c.Gray, L.stylusCanvas.body, a.stylusImg, image.Point{}, draw.Src)
		a.drawPanel(L.stylusLog, "fingers are ignored on the canvas")
		a.drawLines(L.stylusLog, a.stylusLog, "Draw with the stylus. Pressure sets the line width.")
	case tabNet:
		for i := range a.L.net {
			a.drawNetPanel(i)
		}
	case tabSound:
		a.drawBluetooth()
		a.drawPlayer()
	case tabTerm:
		a.drawTerm()
	case tabInfo:
		a.drawInfo()
	}
	a.mark(L.page, true)
}

// drawPanel draws the frame and title strip, and clears the body.
func (a *app) drawPanel(p panel, status string) {
	c := a.c
	c.fill(p.outer, white)
	if p.title.Empty() { // the fullscreen terminal: no frame
		return
	}
	c.frame(p.outer, border, black)
	a.drawPanelTitle(p, status)
}

func (a *app) drawPanelTitle(p panel, status string) {
	if p.title.Empty() {
		return
	}
	c := a.c
	c.fill(p.title, light)
	c.fill(image.Rect(p.title.Min.X, p.title.Max.Y-2, p.title.Max.X, p.title.Max.Y), black)
	t := image.Rect(p.title.Min.X+16, p.title.Min.Y, p.title.Max.X-16, p.title.Max.Y-2)
	if p.buttonLabel != "" {
		c.fill(p.button, white)
		c.frame(p.button, 2, black)
		c.text(a.f.small, p.button, p.buttonLabel, black, alignCenter)
		t.Max.X = p.button.Min.X - 16
	}
	nameW := textWidth(a.f.panel, p.name)
	c.text(a.f.panel, image.Rect(t.Min.X, t.Min.Y, t.Min.X+nameW+2, t.Max.Y), p.name, black, alignLeft)
	c.text(a.f.small, image.Rect(t.Min.X+nameW+28, t.Min.Y, t.Max.X, t.Max.Y), status, dark, alignRight)
	a.mark(p.title, true)
}

// drawTyping repaints only the text rows that changed.
func (a *app) drawTyping() {
	b := a.L.typing.body.Inset(pad)
	lineH := a.f.mono.Metrics().Height.Ceil() + 6
	adv := textWidth(a.f.mono, "M")
	cols, rows := max(1, b.Dx()/adv-1), max(1, b.Dy()/lineH)
	lines := a.ed.wrapped(cols)
	if len(lines) > rows {
		lines = lines[len(lines)-rows:]
	}
	if len(a.shown) != rows {
		a.shown = make([]string, rows)
		for i := range a.shown {
			a.shown[i] = "\x00" // never matches, so everything is drawn once
		}
	}
	for i := 0; i < rows; i++ {
		s, cursor := "", i == len(lines)-1
		if i < len(lines) {
			s = lines[i]
		}
		key := s
		if cursor {
			key += "\x01"
		}
		if key == a.shown[i] {
			continue
		}
		a.shown[i] = key
		lr := image.Rect(b.Min.X, b.Min.Y+i*lineH, b.Max.X, b.Min.Y+(i+1)*lineH)
		a.c.fill(lr, white)
		if i == 0 && len(a.ed.lines) == 0 {
			a.c.text(a.f.body, image.Rect(lr.Min.X+adv+12, lr.Min.Y, lr.Max.X, lr.Max.Y), "type something, bro", mid, alignLeft)
		} else {
			a.c.text(a.f.mono, lr, s, black, alignLeft)
		}
		if cursor {
			x := lr.Min.X + textWidth(a.f.mono, s)
			a.c.fill(image.Rect(x+2, lr.Min.Y+4, x+2+adv, lr.Max.Y-4), black)
		}
		a.mark(lr, true)
	}
}

func (a *app) drawLastKey() {
	b := a.L.lastKey.body
	a.c.fill(b, white)
	s := a.lastKey
	if s == "" {
		s = "-"
	}
	a.c.text(a.f.mono, b.Inset(pad), s, black, alignLeft)
	a.mark(b, true)
}

func (a *app) keyboardStatus() string {
	if a.keyboard == "" {
		return "no keyboard: pair one in Kobo Settings > Bluetooth"
	}
	return "connected: " + a.keyboard
}

// drawLines fills a panel body with the newest lines that fit.
func (a *app) drawLines(p panel, lines []string, empty string) {
	a.drawText(p, a.f.body, lines, empty, false)
}

// drawText fills a panel body with lines: the newest that fit (logs), or
// the first ones (reports).
// It returns where each line went (empty for lines that didn't fit), for taps.
func (a *app) drawText(p panel, f font.Face, lines []string, empty string, fromTop bool) []image.Rectangle {
	b := p.body
	a.c.fill(b, white)
	in := b.Inset(pad)
	lh := f.Metrics().Height.Ceil() + 8
	n := max(0, in.Dy()/lh)
	col := uint8(black)
	rows := make([]image.Rectangle, len(lines))
	first := 0
	if len(lines) == 0 {
		lines, col = []string{empty}, mid
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
		a.c.text(f, r, s, col, alignLeft)
		if first+i < len(rows) && !(fromTop && len(lines) < len(rows) && i == len(lines)-1 && len(rows) > n) {
			rows[first+i] = r
		}
	}
	a.mark(b, true)
	return rows
}

// drawTextRows is drawText from the top, repainting only the rows that
// differ from *shown (all of them when it's nil): for text being typed.
func (a *app) drawTextRows(p panel, f font.Face, lines []string, shown *[]string) []image.Rectangle {
	in := p.body.Inset(pad)
	lh := f.Metrics().Height.Ceil() + 8
	n := max(1, in.Dy()/lh)
	rows := make([]image.Rectangle, len(lines))
	if len(lines) > n {
		lines = append(slices.Clone(lines[:n-1]), fmt.Sprintf("… %d more lines", len(lines)-n+1))
	}
	full := *shown == nil
	if full {
		a.c.fill(p.body, white)
		a.mark(p.body, true)
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
			a.c.fill(r, white)
			a.mark(r, true)
		}
		a.c.text(f, r, s, black, alignLeft)
	}
	*shown = slices.Clone(lines)
	return rows
}

var netHints = [numNetPanels]string{
	netWifi: "Tap refresh.",
	netScan: "Tap scan: nearby Wi-Fi networks, then hosts on this network.",
	netMDNS: "Tap browse: services announced over multicast DNS (printers, TVs, AirPlay, …).",
	netHTTP: "Tap GET to fetch " + httpTestURL + ".",
}

func (a *app) drawNetPanel(i int) {
	p, st := a.L.net[i], a.netState[i]
	status := st.status
	if st.busy && status == "" {
		status = "working…"
	}
	a.drawPanel(p, status)
	f := a.f.small
	if i == netWifi && len(st.lines) <= 7 {
		f = a.f.body
	}
	a.drawText(p, f, st.lines, netHints[i], true)
}

func (a *app) drawInfo() {
	L, s := a.L, a.scr.info
	var devs []string
	for _, d := range a.devs {
		line := d.handler + "  " + d.name
		if r := d.role(); r != "" {
			line += "  (" + r + ")"
		}
		devs = append(devs, line)
	}
	a.drawPanel(L.devices, fmt.Sprintf("%d", len(a.devs)))
	a.drawLines(L.devices, devs, "none found")

	orient := "portrait"
	if s.landscape() {
		orient = "landscape"
	}
	a.drawPanel(L.screenP, "")
	a.drawLines(L.screenP, []string{
		fmt.Sprintf("%s, %d x %d px", orient, s.viewW, s.viewH),
		fmt.Sprintf("rotation: native %d, canonical %d", s.rota, s.canonical),
		fmt.Sprintf("device: %s", s.codename),
		fmt.Sprintf("FBInk %s", s.version),
		a.lightInfo(),
	}, "")

	var touch []string
	if a.touch == nil {
		touch = []string{"no touch panel found"}
	} else {
		t := a.touch
		src := "FBInk default"
		if _, ok := a.cal[s.rota]; ok {
			src = "calibrated"
		}
		touch = []string{
			t.name,
			fmt.Sprintf("raw x %d..%d, y %d..%d", a.tmap.rx.min, a.tmap.rx.max, a.tmap.ry.min, a.tmap.ry.max),
			fmt.Sprintf("pen pressure up to %d", a.pressureMax()),
			fmt.Sprintf("mapping: %s (%s)", a.tmap.axes, src),
			fmt.Sprintf("FBInk default would be: %s", defaultAxes(s)),
		}
		if a.calibNote != "" {
			touch = append(touch, a.calibNote)
		}
	}
	a.drawPanel(L.touchP, "")
	a.drawLines(L.touchP, touch, "")
}

func (a *app) drawCalib() {
	c := a.c
	b := c.Bounds()
	c.fill(b, white)
	c.fill(a.L.header, black)
	c.text(a.f.title, image.Rect(24, 0, b.Dx()-24, headerH), "touch calibration", white, alignLeft)
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
		c.text(a.f.body, image.Rect(pad, y, b.Dx()-pad, y+40), l, black, alignCenter)
		y += 48
	}
	t := calibrationTargets[step]
	c.target(image.Pt(int(t[0]*float64(b.Dx())), int(t[1]*float64(b.Dy()))))
	a.markAll(true)
}

func (a *app) drawBye() {
	c := a.c
	b := c.Bounds()
	c.fill(b, white)
	c.text(a.f.huge, image.Rect(0, b.Dy()/2-130, b.Dx(), b.Dy()/2+30), "bye bro!", black, alignCenter)
	c.text(a.f.body, image.Rect(0, b.Dy()/2+60, b.Dx(), b.Dy()/2+110), "back to Kobo…", dark, alignCenter)
	a.markAll(true)
}
