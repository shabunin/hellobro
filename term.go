package main

// The terminal tabs: SSH, with a real PTY on the remote, and Shell, a
// shell on the Kobo itself. Each tab holds several sessions. Their output
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
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/vt"
)

const (
	termIdle = iota
	termConnecting
	termWaiting // to connect again, after the connection dropped
	termConnected
)

const (
	kindSSH = iota
	kindShell
)

const (
	termFrame   = 60 * time.Millisecond // at most this often we redraw from the emulator
	maxSessions = 9
)

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
	yesno    bool // Enter: yes, Esc: no
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

// termBackend is what a session talks to: an SSH session or a local shell.
type termBackend interface {
	io.Reader // output
	io.Writer // input
	resize(cols, rows int)
	close()
	// ended says, once the output has ended, whether the connection was
	// lost (rather than the remote exiting), and how it ended.
	ended() (lost bool, how string)
}

// termSession is one terminal: an SSH connection or a shell.
type termSession struct {
	kind   int
	input  string // the ssh> line
	notes  []string
	state  int
	target sshHost
	typed  bool // the target was typed in: saved to ssh.conf once it works
	prompt *termPrompt
	gen    int // bumps per connection, so late messages from an old one are ignored

	back  termBackend
	emu   *vt.SafeEmulator
	dirty atomic.Bool
	title string

	password string    // typed for this connection, for connecting again (only kept in memory)
	retries  int       // connection attempts since it dropped
	retryAt  time.Time // when the next one starts
}

func (t *termSession) connected() bool { return t.state == termConnected }

type termGroup struct {
	sessions []*termSession
	cur      int
}

// termView is the terminal on screen. Only one session shows at a time,
// so they share the grid, the font and the glyph cache.
type termView struct {
	groups   [2]termGroup // the SSH tab's sessions, the Shell tab's
	hosts    []sshHost
	keyAsked map[string]bool // hosts we offered to install our key on

	hostRows     []image.Rectangle // where the saved hosts are listed, for taps
	chips        []image.Rectangle // the session buttons in the title strip; the last may be "+"
	shownConnect []string          // the connect screen's rows as drawn, nil: draw it all
	shownStatus  string

	grid       image.Rectangle // the cells' area
	cw, ch     int             // cell size
	cols, rows int
	shown      [][]cellKey // what's drawn
	drawnAt    time.Time
	outAt      time.Time       // last output drawn
	fastArea   image.Rectangle // drawn with the fast (DU) waveform since the last clean refresh
	fastAt     time.Time       // when fastArea got its first update
	churn      int             // pixels changed since the last flash
	bigAt      time.Time       // the last change of more than a few rows

	mods   termMods
	glyphs map[glyphKey]*image.Alpha
	fontPx int
}

func isTermTab(t tabID) bool { return t == tabSSH || t == tabShell }

// termGroup is the current tab's sessions, if it's a terminal tab.
func (a *app) termGroup() *termGroup {
	switch a.tab {
	case tabSSH:
		return &a.tv.groups[kindSSH]
	case tabShell:
		return &a.tv.groups[kindShell]
	}
	return nil
}

// term is the session on screen, if any.
func (a *app) term() *termSession {
	if g := a.termGroup(); g != nil && g.cur < len(g.sessions) {
		return g.sessions[g.cur]
	}
	return nil
}

func (a *app) showing(t *termSession) bool { return a.calib == nil && t != nil && a.term() == t }

// termLive says whether any session is connected, or about to be.
func (a *app) termLive() bool {
	for _, g := range a.tv.groups {
		for _, t := range g.sessions {
			if t.state != termIdle {
				return true
			}
		}
	}
	return false
}

// --- layout & geometry ---

func (a *app) termGeometry() {
	v := &a.tv
	v.grid = a.L.termP.body.Inset(6)
	v.cw = textWidth(a.f.term, "M")
	m := a.f.term.Metrics()
	v.ch = m.Ascent.Ceil() + m.Descent.Ceil() + 2
	v.cols, v.rows = max(20, v.grid.Dx()/v.cw), max(5, v.grid.Dy()/v.ch)
	// Center the grid in the body.
	gw, gh := v.cols*v.cw, v.rows*v.ch
	min := image.Pt(v.grid.Min.X+(v.grid.Dx()-gw)/2, v.grid.Min.Y+(v.grid.Dy()-gh)/2)
	v.grid = image.Rectangle{min, min.Add(image.Pt(gw, gh))}
	v.shown = nil
	for _, g := range v.groups {
		for _, t := range g.sessions {
			if t.connected() {
				t.emu.Resize(v.cols, v.rows)
				t.back.resize(v.cols, v.rows)
			}
		}
	}
}

// termPanel is the terminal panel as the current session has it: named
// after the tab, with a button that fits what the session is doing.
func (a *app) termPanel() panel {
	p := a.L.termP
	t := a.term()
	if p.title.Empty() || t == nil {
		return p
	}
	label := ""
	switch {
	case t.state != termIdle && t.kind == kindShell:
		label = "kill"
	case t.state != termIdle:
		label = "disconnect"
	case len(a.termGroup().sessions) > 1:
		label = "close"
	}
	p.name = tabNames[a.tab]
	return withButton(a.f, p, label)
}

// --- drawing ---

func (t *termSession) status(v *termView) string {
	switch t.state {
	case termConnecting:
		return "connecting to " + t.target.String() + "…"
	case termWaiting:
		return fmt.Sprintf("reconnecting to %s in %.0f s", t.target, max(0, time.Until(t.retryAt).Seconds()))
	case termConnected:
		where := t.target.String()
		if t.kind == kindShell {
			where = "sh on this Kobo"
		}
		s := fmt.Sprintf("%s · %d×%d · %d px", where, v.cols, v.rows, v.fontPx)
		if t.title != "" {
			s = t.title + " · " + s
		}
		return s
	}
	if t.kind == kindShell {
		return "not running"
	}
	return "not connected"
}

// drawTerm draws the whole terminal panel.
func (a *app) drawTerm() {
	t := a.term()
	if t == nil || a.calib != nil {
		return
	}
	v := &a.tv
	p := a.termPanel()
	v.shownConnect = nil
	a.drawPanel(p, "")
	a.drawTermTitle()
	if t.connected() {
		v.shown = nil
		a.renderTerm(t)
		return
	}
	a.drawConnect(t)
}

// drawTermTitle draws the title strip: the name, a button per session
// (and "+" for another one), and the session's status.
func (a *app) drawTermTitle() {
	t, g := a.term(), a.termGroup()
	p := a.termPanel()
	v := &a.tv
	v.shownStatus = t.status(v)
	v.chips = nil
	if p.title.Empty() {
		return
	}
	a.drawPanelTitle(p, "")
	x := p.title.Min.X + 16 + textWidth(a.f.panel, p.name) + 24
	n := len(g.sessions)
	if n < maxSessions {
		n++
	}
	for i := range n {
		r := image.Rect(x, p.title.Min.Y+6, x+44, p.title.Max.Y-8)
		label := strconv.Itoa(i + 1)
		if i == len(g.sessions) {
			label = "+"
		}
		if i == g.cur {
			a.c.fill(r, black)
			a.c.text(a.f.small, r, label, white, alignCenter)
		} else {
			a.c.fill(r, white)
			a.c.frame(r, 2, black)
			a.c.text(a.f.small, r, label, black, alignCenter)
		}
		v.chips = append(v.chips, r)
		x = r.Max.X + 8
	}
	end := p.title.Max.X - 16
	if !p.button.Empty() {
		end = p.button.Min.X - 16
	}
	a.c.text(a.f.small, image.Rect(x+16, p.title.Min.Y, end, p.title.Max.Y-2), v.shownStatus, dark, alignRight)
}

// drawTermStatus redraws the title strip if the status changed.
func (a *app) drawTermStatus() {
	if t := a.term(); t != nil && a.calib == nil && t.status(&a.tv) != a.tv.shownStatus {
		a.drawTermTitle()
	}
}

func (a *app) drawConnect(t *termSession) {
	v := &a.tv
	keys := "Ctrl+Shift+T: new session  ·  Ctrl+PgUp/PgDn: switch  ·  Alt+F1-F9: tabs  ·  F11: fullscreen"
	var lines []string
	if t.kind == kindShell {
		lines = []string{"A shell on this Kobo (/bin/sh, as " + whoami() + "): Enter or tap here to start it.", keys}
		if t.state == termIdle && len(a.termGroup().sessions) > 1 {
			lines = append(lines, "Esc: close this session.")
		}
		if len(t.notes) > 0 {
			lines = append(lines, "")
			lines = append(lines, t.notes...)
		}
		a.drawTextRows(a.termPanel(), a.f.body, lines, &v.shownConnect)
		v.hostRows = nil
		return
	}
	lines = []string{"Type user@host[:port] [command] and Enter, or tap / press 1-9 for a saved host.", keys}
	switch {
	case t.prompt != nil && t.prompt.yesno:
		lines = append(lines, t.prompt.question, "(Enter: yes, Esc: no)")
	case t.prompt != nil:
		shown, help := t.prompt.answer, "(Enter to send, Esc to cancel)"
		if !t.prompt.echo {
			if !t.prompt.reveal {
				shown = strings.Repeat("•", utf8.RuneCountInString(shown))
			}
			help = "(Enter to send, Tab to show/hide, Esc to cancel)"
		}
		if v.mods.caps {
			help += "   CAPS LOCK is on"
		}
		lines = append(lines, t.prompt.question+shown+"█", help)
	case t.state == termIdle:
		lines = append(lines, "ssh> "+t.input+"█")
	case t.state == termWaiting:
		lines = append(lines, "ssh> "+t.input, "(Esc: stop reconnecting)")
	default:
		lines = append(lines, "ssh> "+t.input)
	}
	lines = append(lines, "", fmt.Sprintf("saved hosts (%s):", appFile("ssh.conf")))
	firstHost := len(lines)
	for i, h := range v.hosts {
		line := fmt.Sprintf("   %d   %s   %s", i+1, h.name, h)
		if h.cmd != "" {
			line += "   ▸ " + h.cmd
		}
		lines = append(lines, line)
	}
	if len(v.hosts) == 0 {
		lines = append(lines, "   none yet: add lines like  myserver  bro@192.168.1.10:2222  [command]")
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
	rows := a.drawTextRows(a.termPanel(), a.f.body, lines, &v.shownConnect)
	v.hostRows = nil
	if len(v.hosts) > 0 {
		v.hostRows = rows[firstHost : firstHost+len(v.hosts)]
	}
}

// updateTerm redraws what changed on t's connect screen, if it's on
// screen: a key typed into a line shouldn't repaint the whole panel (in
// fullscreen, the whole screen).
func (a *app) updateTerm(t *termSession) {
	if !a.showing(t) {
		return
	}
	if t.connected() || a.tv.shownConnect == nil {
		a.drawTerm()
		return
	}
	a.drawTermStatus()
	a.drawConnect(t)
}

// renderTerm redraws the cells that changed since the last time.
func (a *app) renderTerm(t *termSession) {
	v := &a.tv
	if t.emu == nil {
		return
	}
	t.dirty.Store(false)
	v.drawnAt = time.Now()
	if len(v.shown) != v.rows || (v.rows > 0 && len(v.shown[0]) != v.cols) {
		v.shown = make([][]cellKey, v.rows)
		for y := range v.shown {
			v.shown[y] = make([]cellKey, v.cols)
			for x := range v.shown[y] {
				v.shown[y][x].width = -1 // never matches
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
	for y := 0; y < v.rows; y++ {
		from, to, gray := -1, -1, false
		for x := 0; x < v.cols; x++ {
			k := t.cellKeyAt(x, y, showCursor && cur.X == x && cur.Y == y)
			if k == v.shown[y][x] {
				continue
			}
			v.shown[y][x] = k
			a.drawCell(x, y, k)
			if from < 0 {
				from = x
			}
			to = x
			gray = gray || !bw(k.fg) || !bw(k.bg)
		}
		if from >= 0 {
			r := image.Rect(v.grid.Min.X+from*v.cw, v.grid.Min.Y+y*v.ch, v.grid.Min.X+(to+1)*v.cw, v.grid.Min.Y+(y+1)*v.ch)
			rows = append(rows, changed{r.Inset(-1).Intersect(v.grid), gray})
		}
	}
	// Black & white changes to a row or three (typing, a spinner) take the
	// fast waveform. It ghosts, though, so scrolls and redraws, where a
	// moment more doesn't matter, get the clean grayscale one.
	big := len(rows) > 3
	if big {
		v.bigAt = time.Now()
	}
	for _, c := range rows {
		v.churn += area(c.r)
		if !c.gray && !big {
			if v.fastArea.Empty() {
				v.fastAt = time.Now()
			}
			v.fastArea = v.fastArea.Union(c.r)
		}
		a.mark(c.r, c.gray || big)
	}
}

func bw(g uint8) bool { return g == 0 || g == 255 }

func (t *termSession) cellKeyAt(x, y int, cursor bool) cellKey {
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
	v := &a.tv
	if k.width == 0 {
		return // the right half of a wide character, drawn with it
	}
	w := max(1, k.width)
	r := image.Rect(v.grid.Min.X+x*v.cw, v.grid.Min.Y+y*v.ch, v.grid.Min.X+(x+w)*v.cw, v.grid.Min.Y+(y+1)*v.ch).Intersect(v.grid)
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
		a.c.fill(image.Rect(r.Min.X, r.Min.Y+v.ch/2, r.Max.X, r.Min.Y+v.ch/2+2), fg)
	}
}

// --- sessions ---

// enterTerm gets a terminal tab ready: the font, the geometry, and a
// first session (the Shell tab's starts its shell right away).
func (a *app) enterTerm() {
	v := &a.tv
	if v.hosts == nil {
		v.hosts = loadSSHHosts()
	}
	if v.cw == 0 {
		v.fontPx = termFontDefault
		if b, err := os.ReadFile(appFile("term-font")); err == nil {
			if px, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil && px != v.fontPx {
				v.fontPx = max(termFontMin, min(termFontMax, px))
				a.f.setTermSize(v.fontPx)
			}
		}
		a.termGeometry()
	}
	if g := a.termGroup(); g != nil && len(g.sessions) == 0 {
		a.newSession(true)
	}
}

// newSession adds a session to the current tab and shows it; start runs
// the Shell tab's shell.
func (a *app) newSession(start bool) {
	g := a.termGroup()
	if g == nil || len(g.sessions) >= maxSessions {
		return
	}
	t := &termSession{kind: kindSSH}
	if a.tab == tabShell {
		t.kind = kindShell
	}
	g.sessions = append(g.sessions, t)
	g.cur = len(g.sessions) - 1
	log.Printf("terminal: session %d on the %s tab", g.cur+1, tabNames[a.tab])
	if t.kind == kindShell && start {
		a.termStartShell(t)
	}
}

// switchSession shows session i of the current tab.
func (a *app) switchSession(i int) {
	g := a.termGroup()
	if g == nil || i < 0 || i >= len(g.sessions) || i == g.cur {
		return
	}
	g.cur = i
	a.drawTerm()
	a.markFlash(a.L.termP.outer) // a whole new screen: clean, not ghosted
}

// closeSession ends the session on screen and drops it, unless it's the
// tab's only one.
func (a *app) closeSession() {
	g, t := a.termGroup(), a.term()
	if t == nil || len(g.sessions) < 2 {
		return
	}
	a.termClose(t)
	g.sessions = slices.Delete(g.sessions, g.cur, g.cur+1)
	g.cur = min(g.cur, len(g.sessions)-1)
	a.drawTerm()
	a.markFlash(a.L.termP.outer)
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

// setTermFont changes the terminal's font size; the remotes get the new
// cell count, as when a terminal window is resized.
func (a *app) setTermFont(px int) {
	v := &a.tv
	px = max(termFontMin, min(termFontMax, px))
	if px == v.fontPx {
		return
	}
	log.Printf("terminal font: %d px", px)
	v.fontPx = px
	a.f.setTermSize(px)
	v.glyphs = nil
	a.termGeometry()
	os.WriteFile(appFile("term-font"), []byte(strconv.Itoa(px)+"\n"), 0o644)
	if a.term() != nil {
		a.drawTerm()
		a.markFlash(a.L.termP.body) // every glyph changed: clean, not ghosted
	}
}

func (a *app) termNote(t *termSession, s string) {
	log.Printf("terminal: %s", s)
	t.notes = append(t.notes, s)
	if len(t.notes) > 6 {
		t.notes = t.notes[len(t.notes)-6:]
	}
	if !t.connected() {
		a.updateTerm(t)
	}
}

// reconnectDelay is how long to wait before attempt n to connect again.
func reconnectDelay(n int) time.Duration {
	return min(30*time.Second, 2*time.Second<<min(n-1, 5))
}

// waitToReconnect schedules the next attempt to connect again.
func (a *app) waitToReconnect(t *termSession) {
	t.retries++
	t.state, t.retryAt = termWaiting, time.Now().Add(reconnectDelay(t.retries))
	a.termNote(t, fmt.Sprintf("reconnecting in %.0f s (Esc: stop)", reconnectDelay(t.retries).Seconds()))
}

func (a *app) termConnect(t *termSession, h sshHost) {
	if t.state != termIdle && t.state != termWaiting {
		return
	}
	retry := t.state == termWaiting
	if !retry {
		t.notes, t.retries = nil, 0
	}
	t.state, t.target, t.title = termConnecting, h, ""
	t.gen++
	gen := t.gen
	a.termNote(t, "connecting to "+h.String()+"…")
	ask := func(q string, echo, yesno bool) (string, error) {
		reply := make(chan string, 1)
		a.call <- func() {
			if t.gen == gen {
				t.prompt = &termPrompt{question: q, echo: echo, yesno: yesno, reply: reply}
				a.updateTerm(t)
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
	o := dialOpts{
		cols: a.tv.cols, rows: a.tv.rows, password: t.password,
		ask:  func(q string, echo bool) (string, error) { return ask(q, echo, false) },
		note: func(s string) { a.call <- func() { a.termNote(t, s) } },
	}
	if a.tv.keyAsked == nil {
		a.tv.keyAsked = map[string]bool{}
	}
	if !a.tv.keyAsked[h.String()] {
		o.confirm = func(q string) bool {
			a.call <- func() { a.tv.keyAsked[h.String()] = true }
			ans, err := ask(q, true, true)
			return err == nil && ans == "y"
		}
	}
	go func() {
		s, typed, err := dialSSH(h, o)
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
				a.termNote(t, "failed: "+err.Error())
				refused := strings.Contains(err.Error(), "unable to authenticate")
				if refused {
					a.termNote(t, fmt.Sprintf("the server refused user %q: check the user name and password", h.user))
					t.password = ""
				}
				if retry && !refused && !strings.Contains(err.Error(), "host key") {
					a.waitToReconnect(t)
				}
				return
			}
			if typed != "" {
				t.password = typed
			}
			t.retries = 0
			if t.typed && !slices.ContainsFunc(a.tv.hosts, h.same) {
				if name, err := saveSSHHost(a.tv.hosts, h); err != nil {
					a.termNote(t, "couldn't save the host: "+err.Error())
				} else {
					h.name = name
					a.tv.hosts = append(a.tv.hosts, h)
					log.Printf("ssh: saved %s as %s", h, name)
				}
			}
			t.typed, t.input = false, ""
			a.termStart(t, s, gen)
		}
	}()
}

func (a *app) termStartShell(t *termSession) {
	if t.state != termIdle {
		return
	}
	t.notes, t.title = nil, ""
	s, err := startShell(a.tv.cols, a.tv.rows)
	if err != nil {
		a.termNote(t, "couldn't start a shell: "+err.Error())
		return
	}
	t.gen++
	a.termStart(t, s, t.gen)
}

func (a *app) termStart(t *termSession, back termBackend, gen int) {
	v := &a.tv
	t.back, t.state = back, termConnected
	t.emu = vt.NewSafeEmulator(v.cols, v.rows)
	t.emu.SetDefaultForegroundColor(color.Black) // what apps get when they ask, so they pick light-theme colors
	t.emu.SetDefaultBackgroundColor(color.White)
	t.emu.SetScrollbackSize(1000) // not shown; the default 10000 costs a big copy per line once full
	t.emu.SetCallbacks(vt.Callbacks{
		Title: func(s string) {
			a.call <- func() {
				t.title = s
				if a.showing(t) {
					a.drawTermStatus()
				}
			}
		},
	})
	back.resize(v.cols, v.rows)
	if t.kind == kindShell {
		log.Printf("shell: started, %d×%d", v.cols, v.rows)
	} else {
		log.Printf("ssh: connected to %s, %d×%d", t.target, v.cols, v.rows)
	}
	emu := t.emu
	go func() { // remote → emulator
		buf := make([]byte, 32<<10)
		var err error
		for {
			var n int
			n, err = back.Read(buf)
			if n > 0 {
				emu.Write(buf[:n])
				t.dirty.Store(true)
			}
			if err != nil {
				break
			}
		}
		lost, how := back.ended()
		a.call <- func() { a.termEnded(t, gen, err, lost, how) }
	}()
	go func() { // emulator (keys, mouse, replies to queries) → remote
		buf := make([]byte, 4096)
		for {
			n, err := emu.Read(buf)
			if n > 0 {
				if _, werr := back.Write(buf[:n]); werr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()
	if a.showing(t) {
		a.drawTerm()
	}
}

func (a *app) termEnded(t *termSession, gen int, err error, lost bool, how string) {
	if t.gen != gen || t.state != termConnected {
		return
	}
	a.termClose(t)
	msg := "connection closed"
	if t.kind == kindShell {
		msg = "the shell exited"
	}
	if how == "" && err != nil && !errors.Is(err, io.EOF) {
		how = err.Error()
	}
	if how != "" {
		msg += ": " + how
	}
	if t.kind == kindShell {
		msg += "  (Enter: start another)"
	}
	a.termNote(t, msg)
	if lost && t.kind == kindSSH {
		a.waitToReconnect(t)
	}
}

// termClose drops the session's connection, or cancels one in progress.
func (a *app) termClose(t *termSession) {
	if t.prompt != nil {
		close(t.prompt.reply)
		t.prompt = nil
	}
	if t.back != nil {
		t.back.close()
		t.emu.Close()
		t.back, t.emu = nil, nil
	}
	t.gen++
	t.state = termIdle
	if a.showing(t) {
		a.tv.shown = nil
	}
}

// termCloseAll ends every session, when we quit.
func (a *app) termCloseAll() {
	for _, g := range a.tv.groups {
		for _, t := range g.sessions {
			a.termClose(t)
		}
	}
}

// termTick starts the reconnections that are due, redraws from the
// emulator now and then, and gives the screen a clean grayscale refresh
// once output settles after fast updates.
func (a *app) termTick() {
	for _, g := range a.tv.groups {
		for _, t := range g.sessions {
			if t.state == termWaiting && time.Now().After(t.retryAt) {
				a.termConnect(t, t.target)
			}
		}
	}
	t, v := a.term(), &a.tv
	if t == nil || a.calib != nil {
		return
	}
	if t.state == termWaiting {
		a.drawTermStatus() // the countdown
	}
	if !t.connected() {
		return
	}
	if a.lightOverlay != nil || a.fontOverlay != nil {
		return // a gauge is up: drawing under it would be undone when it goes
	}
	if t.dirty.Load() && time.Since(v.drawnAt) >= termFrame {
		v.outAt = time.Now()
		a.renderTerm(t)
	}
	if a.blitting {
		return
	}
	// Clean up after the fast waveform once output pauses, or every few
	// seconds if it doesn't (a spinner never stops while its app works).
	if !v.fastArea.Empty() && (time.Since(v.outAt) > 1200*time.Millisecond || time.Since(v.fastAt) > 4*time.Second) {
		a.mark(v.fastArea, true)
		v.fastArea = image.Rectangle{}
	}
	// Grayscale updates leave ghosts too, slowly: after a few screenfuls
	// of change, flash the terminal clean once it settles (a clock or a
	// spinner may still tick), as e-readers do every few pages.
	if v.churn > 4*area(v.grid) && time.Since(v.bigAt) > 2500*time.Millisecond {
		v.churn, v.fastArea = 0, image.Rectangle{}
		a.markFlash(v.grid)
	}
}

// --- input ---

// termKey takes a key event while a terminal tab is up. It returns false
// for keys the rest of the app should handle (tab switching, quitting)
// when there's no session to send them to.
func (a *app) termKey(code uint16, value int32) bool {
	t, g, v := a.term(), a.termGroup(), &a.tv
	m := &v.mods
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
	case keyCapsLock:
		if value == 1 {
			m.caps = !m.caps
			if !t.connected() {
				a.updateTerm(t) // the prompt says so
			}
		}
		return true
	}
	// Ours, not the remote's: fullscreen, the font size, sessions and tabs.
	if value != 0 {
		switch {
		case code == keyF11:
			if value == 1 {
				a.setFull(!a.full)
			}
			return true
		case m.ctrl && !m.shift && (code == keyEqual || code == keyMinus || code == key0):
			switch code {
			case keyEqual:
				a.setTermFont(v.fontPx + termFontStep)
			case keyMinus:
				a.setTermFont(v.fontPx - termFontStep)
			default:
				a.setTermFont(termFontDefault)
			}
			return true
		case m.ctrl && m.shift && code == keyT:
			if value == 1 {
				a.newSession(true)
				a.drawTerm()
				a.markFlash(a.L.termP.outer)
			}
			return true
		case m.ctrl && (code == keyPgUp || m.shift && code == keyLeft):
			a.switchSession((g.cur + len(g.sessions) - 1) % len(g.sessions))
			return true
		case m.ctrl && (code == keyPgDn || m.shift && code == keyRight):
			a.switchSession((g.cur + 1) % len(g.sessions))
			return true
		case m.alt && code >= keyF1 && code < keyF1+uint16(numTabs):
			if value == 1 {
				a.switchTab(tabID(code - keyF1))
			}
			return true
		}
	}
	if value == 0 {
		return true
	}
	ch, printable := keyChar(code, m.shift, m.caps)

	if t.prompt != nil {
		p := t.prompt
		switch {
		case p.yesno && (code == keyEnter || ch == "y" || ch == "Y"):
			p.reply <- "y"
			t.prompt = nil
		case p.yesno && (code == keyEsc || ch == "n" || ch == "N"):
			p.reply <- "n"
			t.prompt = nil
		case p.yesno:
		case code == keyEnter:
			p.reply <- p.answer
			t.prompt = nil
		case code == keyEsc:
			a.termClose(t)
			t.retries = 0
			a.termNote(t, "canceled")
		case code == keyBackspace:
			if r := []rune(p.answer); len(r) > 0 {
				p.answer = string(r[:len(r)-1])
			}
		case code == keyTab:
			p.reveal = !p.reveal
		case printable:
			p.answer += ch
		}
		a.updateTerm(t)
		return true
	}

	if !t.connected() {
		switch {
		case (t.state == termConnecting || t.state == termWaiting) && code == keyEsc:
			a.termClose(t)
			t.retries = 0
			a.termNote(t, "canceled")
		case t.state != termIdle:
		case code == keyEsc && len(g.sessions) > 1:
			a.closeSession()
			return true
		case t.kind == kindShell && code == keyEnter:
			a.termStartShell(t)
			return true
		case t.kind == kindShell:
			return code != keyEsc && !(code >= keyF1 && code < keyF1+uint16(numTabs))
		case code == keyEnter:
			in := strings.TrimSpace(t.input)
			if in == "" && len(v.hosts) > 0 {
				a.termConnect(t, v.hosts[0])
			} else if h, err := parseSSHTarget(in); err == nil {
				t.typed = true
				a.termConnect(t, h)
			} else {
				a.termNote(t, err.Error())
			}
		case code == keyBackspace:
			if r := []rune(t.input); len(r) > 0 {
				t.input = string(r[:len(r)-1])
			}
		case printable && t.input == "" && ch >= "1" && ch <= "9":
			if i := int(ch[0] - '1'); i < len(v.hosts) {
				a.termConnect(t, v.hosts[i])
			}
		case printable && !m.ctrl:
			t.input += ch
		default:
			return false // Esc, F-keys: the app's
		}
		a.updateTerm(t)
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
	keyT         = 20
	keyF11       = 87
	keyF12       = 88
	keyPgUp      = 104
	keyLeft      = 105
	keyRight     = 106
	keyPgDn      = 109
)

var termSpecialKeys = map[uint16]rune{
	keyEsc: uv.KeyEscape, keyBackspace: uv.KeyBackspace, keyTab: uv.KeyTab, keyEnter: uv.KeyEnter, 96: uv.KeyEnter,
	102: uv.KeyHome, 103: uv.KeyUp, keyPgUp: uv.KeyPgUp, keyLeft: uv.KeyLeft, keyRight: uv.KeyRight, 107: uv.KeyEnd,
	108: uv.KeyDown, keyPgDn: uv.KeyPgDown, 110: uv.KeyInsert, 111: uv.KeyDelete,
	59: uv.KeyF1, 60: uv.KeyF2, 61: uv.KeyF3, 62: uv.KeyF4, 63: uv.KeyF5, 64: uv.KeyF6,
	65: uv.KeyF7, 66: uv.KeyF8, 67: uv.KeyF9, 68: uv.KeyF10, keyF11: uv.KeyF11, keyF12: uv.KeyF12,
}

// termCell maps a screen point to a cell, if it's on the grid.
func (a *app) termCell(p image.Point) (int, int, bool) {
	v := &a.tv
	if !p.In(v.grid) || v.cw == 0 {
		return 0, 0, false
	}
	return (p.X - v.grid.Min.X) / v.cw, (p.Y - v.grid.Min.Y) / v.ch, true
}

// termTap handles a tap on a terminal tab: the panel's button, the
// session buttons, a saved host, or a click for the remote app.
func (a *app) termTap(p image.Point) {
	t, v := a.term(), &a.tv
	if t == nil {
		return
	}
	if hit(p, a.termPanel().button) {
		switch {
		case t.state != termIdle:
			a.termClose(t)
			t.retries = 0
			a.termNote(t, "disconnected")
			a.drawTerm()
		default:
			a.closeSession()
		}
		return
	}
	for i, r := range v.chips {
		if p.In(r.Inset(-4)) {
			if g := a.termGroup(); i == len(g.sessions) {
				a.newSession(true)
				a.drawTerm()
				a.markFlash(a.L.termP.outer)
			} else {
				a.switchSession(i)
			}
			return
		}
	}
	if !t.connected() {
		if t.kind == kindShell && p.In(a.L.termP.body) && t.state == termIdle {
			a.termStartShell(t)
			return
		}
		for i, r := range v.hostRows {
			if p.In(r) && t.state == termIdle {
				a.termConnect(t, v.hosts[i])
				return
			}
		}
		return
	}
	x, y, ok := a.termCell(p)
	if !ok {
		return
	}
	m := uv.Mouse{X: x, Y: y, Button: uv.MouseLeft}
	t.emu.SendMouse(uv.MouseClickEvent(m))
	t.emu.SendMouse(uv.MouseReleaseEvent(m))
}

// termSwipe turns a vertical swipe into mouse wheel steps, one per row moved.
func (a *app) termSwipe(from, to image.Point) bool {
	t, v := a.term(), &a.tv
	x, y, ok := a.termCell(from)
	if t == nil || !t.connected() || !ok {
		return false
	}
	dy := to.Y - from.Y
	steps := min(20, abs(dy)/v.ch)
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
