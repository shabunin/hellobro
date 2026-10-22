package main

// The terminal: SSH, with a real PTY on the remote, and Shell, a shell
// on the Kobo itself. Each tab holds several sessions. Their output
// goes through an xterm emulator (charmbracelet/x/vt) into a grid of
// cells, which we draw ourselves, redrawing only the cells that changed.
// Keys and touches go back as xterm key sequences and mouse events.

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"log"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/vt"

	"hellobro/kobo"
)

const (
	termIdle = iota
	termConnecting
	termWaiting // to connect again, after the connection dropped
	termConnected
)

// A session's kind is the tab it's on.
const (
	kindSSH   = tabSSH
	kindShell = tabShell
)

const (
	termFrame     = 60 * time.Millisecond // at most this often we redraw from the emulator
	termFullEvery = 3 * time.Second       // and this often we compare all the cells, not only the touched rows
	maxSessions   = 9
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
	sel       bool // selected: shown reversed
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

	back     termBackend
	emu      *vt.SafeEmulator
	mu       sync.Mutex // held while the emulator changes (write): its touched lines are read and cleared under it
	dirty    atomic.Bool
	title    string
	keyAt    time.Time // a key last went to it
	notifyAt time.Time // its last notice

	password string    // typed for this connection, for connecting again (only kept in memory)
	retries  int       // connection attempts since it dropped
	retryAt  time.Time // when the next one starts

	pane   termPane
	folded bool        // its panel is only its title strip
	size   image.Point // the emulator's and the remote's, in cells
}

// termPane is where a session shows: its panel, and its cells in it.
type termPane struct {
	p            kobo.Panel
	grid         image.Rectangle // the cells' area; empty when they don't show
	cols, rows   int
	shown        [][]cellKey // what's drawn
	shownConnect []string    // the connect screen's rows as drawn, nil: draw it all
	shownStatus  string
	hostRows     []image.Rectangle // where the saved hosts are listed, for taps
	newR         image.Rectangle   // "+", a new session: in the focused one's title strip
	cursorY      int               // the row the cursor was drawn on
	alt          bool              // the alternate screen was drawn
	selShown     bool              // a selection was drawn
	fullAt       time.Time         // all the cells were last compared
}

// write puts the remote's output into the emulator.
func (t *termSession) write(p []byte) {
	t.mu.Lock()
	t.emu.Write(p)
	t.mu.Unlock()
}

// touchedRows are the rows the emulator changed since the last call, and
// whether it's on its alternate screen. vt keeps them but has no way to
// clear them, so we clear its list ourselves, under t.mu.
func (t *termSession) touchedRows(rows int) ([]bool, bool) {
	touched := make([]bool, rows)
	t.mu.Lock()
	defer t.mu.Unlock()
	ls := t.emu.Touched()
	for y, l := range ls {
		if l != nil && y < rows {
			touched[y] = true
		}
		ls[y] = nil
	}
	return touched, t.emu.IsAltScreen()
}

func (t *termSession) connected() bool { return t.state == termConnected }

// termGroup is a tab's sessions; cur is the one in focus, which the keys
// go to.
type termGroup struct {
	sessions []*termSession
	cur      int
}

// termView is the terminals on screen. Each session has a panel of its
// own (the open ones share the page); they share the font and the glyph
// cache.
type termView struct {
	groups   [2]termGroup // the SSH tab's sessions, the Shell tab's
	hosts    []sshHost
	keyAsked map[string]bool // hosts we offered to install our key on

	sel      *termSel // the selection, or a contact that may become one (clipboard.go): the focused session's
	cw, ch   int      // cell size
	drawnAt  time.Time
	outAt    time.Time       // last output drawn
	fastArea image.Rectangle // drawn with the fast (DU) waveform since the last clean refresh
	fastAt   time.Time       // when fastArea got its first update
	churn    int             // pixels changed since the last flash
	bigAt    time.Time       // the last change of more than a few rows

	mods   termMods
	glyphs map[glyphKey]*image.Alpha
	fontPx int
}

// termGroup is the current tab's sessions, if it's a terminal tab.
func (a *bro) termGroup() *termGroup {
	if !isTermTab(a.Tab) {
		return nil
	}
	return &a.tv.groups[a.Tab]
}

// term is the session on screen, if any.
func (a *bro) term() *termSession {
	if g := a.termGroup(); g != nil && g.cur < len(g.sessions) {
		return g.sessions[g.cur]
	}
	return nil
}

// onScreen says whether t's panel is up, even folded to its strip.
func (a *bro) onScreen(t *termSession) bool {
	return !a.Calibrating() && !a.overlay() && t != nil && isTermTab(a.Tab) && t.kind == a.Tab && !t.pane.p.Outer.Empty()
}

// showing says whether t's cells or connect screen are up.
func (a *bro) showing(t *termSession) bool {
	return a.onScreen(t) && !t.pane.p.Folded
}

// termLive says whether any session is connected, or about to be.
func (a *bro) termLive() bool {
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

// termGeometry measures the cells and lays the sessions out again.
func (a *bro) termGeometry() {
	v := &a.tv
	v.sel = nil // its cells are other ones now
	v.cw = kobo.TextWidth(a.tf.term, "M")
	m := a.tf.term.Metrics()
	v.ch = m.Ascent.Ceil() + m.Descent.Ceil() + 2
	a.layoutSessions(tabSSH)
	a.layoutSessions(tabShell)
}

// layoutSessions places a terminal tab's sessions, a panel each: the
// open ones share the page (portrait: one above the other; landscape:
// two abreast), a folded one is its strip. Fullscreen, the one in focus
// has it all and the others don't show.
func (a *bro) layoutSessions(tab int) {
	g := &a.tv.groups[tab]
	if a.tv.cw == 0 || len(g.sessions) == 0 {
		return
	}
	area := a.termP.Body
	if a.lay.Full {
		for i, t := range g.sessions {
			t.pane.p = kobo.Panel{}
			if i == g.cur {
				t.pane.p = kobo.Panel{Name: sessionName(tab, i), Outer: area, Body: area}
			}
			a.paneGeometry(t)
		}
		return
	}
	if !a.stat.r.Empty() {
		area.Max.Y -= kobo.Pad / 2 // a gap above the status bar
	}
	per := 1
	if a.lay.Landscape {
		per = 2
	}
	var rows []kobo.Row
	for i := 0; i < len(g.sessions); i += per {
		row := kobo.Row{Height: 1}
		for _, t := range g.sessions[i:min(i+per, len(g.sessions))] {
			row.Widths, row.Folded = append(row.Widths, 1), append(row.Folded, t.folded)
		}
		rows = append(rows, row)
	}
	at := a.lay.Arrange(area, rows...)
	for i, t := range g.sessions {
		t.pane.p = kobo.WithFold(a.F, kobo.NewPanel(a.F, sessionName(tab, i), at[i], ""), t.folded)
		a.paneGeometry(t)
	}
}

func sessionName(tab, i int) string { return fmt.Sprintf("%s %d", tabNames[tab], i+1) }

// paneGeometry fits t's cells to its panel. A session that doesn't show
// keeps its size: the remote isn't told of every fold.
func (a *bro) paneGeometry(t *termSession) {
	v, pn := &a.tv, &t.pane
	pn.shown, pn.shownConnect, pn.hostRows, pn.grid = nil, nil, nil, image.Rectangle{}
	if pn.p.Folded || pn.p.Body.Empty() {
		return
	}
	grid := pn.p.Body.Inset(6)
	pn.cols, pn.rows = max(20, grid.Dx()/v.cw), max(5, grid.Dy()/v.ch)
	// Centered in the body.
	gw, gh := pn.cols*v.cw, pn.rows*v.ch
	at := image.Pt(grid.Min.X+(grid.Dx()-gw)/2, grid.Min.Y+(grid.Dy()-gh)/2)
	pn.grid = image.Rectangle{at, at.Add(image.Pt(gw, gh))}
	if size := image.Pt(pn.cols, pn.rows); t.connected() && size != t.size {
		t.size = size
		t.mu.Lock()
		t.emu.Resize(pn.cols, pn.rows)
		t.mu.Unlock()
		t.back.resize(pn.cols, pn.rows)
	}
}

// startSize is the size a new connection starts with: its panel's, or a
// classic one while it doesn't show.
func (t *termSession) startSize() image.Point {
	if t.pane.cols == 0 {
		return image.Pt(80, 24)
	}
	return image.Pt(t.pane.cols, t.pane.rows)
}

// termPanel is t's panel as it is now: in focus or not, with a button
// that fits what the session is doing.
func (a *bro) termPanel(t *termSession) kobo.Panel {
	p := t.pane.p
	if p.Title.Empty() {
		return p
	}
	g := &a.tv.groups[t.kind]
	if len(g.sessions) > 1 && g.cur < len(g.sessions) && g.sessions[g.cur] == t {
		p.Name = "► " + p.Name
	}
	label := ""
	switch {
	case t.state != termIdle && t.kind == kindShell:
		label = "kill"
	case t.state != termIdle:
		label = "disconnect"
	case len(g.sessions) > 1:
		label = "close"
	}
	return kobo.WithButton(a.F, p, label)
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
		s := fmt.Sprintf("%s · %d×%d · %d px", where, t.size.X, t.size.Y, v.fontPx)
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
// drawTerm draws the tab's sessions, each in its panel, and the status bar.
func (a *bro) drawTerm() {
	g := a.termGroup()
	if g == nil || len(g.sessions) == 0 || a.Calibrating() {
		return
	}
	a.C.Fill(a.termP.Outer, kobo.White) // the gaps between the panels
	a.Mark(a.termP.Outer, true)
	for _, t := range g.sessions {
		a.drawPane(t)
	}
	a.drawStat(true)
}

// drawPane draws one session's panel: its frame and title strip, then
// its cells or its connect screen.
func (a *bro) drawPane(t *termSession) {
	pn := &t.pane
	if pn.p.Outer.Empty() {
		return
	}
	pn.shownConnect = nil
	a.DrawPanel(a.termPanel(t), "")
	a.drawTermTitle(t)
	switch {
	case pn.p.Folded:
	case t.connected():
		pn.shown = nil
		a.renderTerm(t)
	default:
		a.drawConnect(t)
	}
}

// drawTermTitle draws the title strip: the name, a button per session
// (and "+" for another one), and the session's status.
// drawTermTitle draws t's title strip: the name, "+" for another session
// (in the focused one's), and the session's status.
func (a *bro) drawTermTitle(t *termSession) {
	p := a.termPanel(t)
	pn, g := &t.pane, &a.tv.groups[t.kind]
	pn.shownStatus = t.status(&a.tv)
	pn.newR = image.Rectangle{}
	if p.Title.Empty() {
		return
	}
	a.DrawPanelTitle(p, "")
	x := p.Title.Min.X + 16 + kobo.TextWidth(a.F.Panel, p.Name) + 24
	if g.cur < len(g.sessions) && g.sessions[g.cur] == t && len(g.sessions) < maxSessions {
		pn.newR = image.Rect(x, p.Title.Min.Y+6, x+44, p.Title.Max.Y-8)
		a.C.Fill(pn.newR, kobo.White)
		a.C.Frame(pn.newR, 2, kobo.Black)
		a.C.Text(a.F.Small, pn.newR, "+", kobo.Black, kobo.AlignCenter)
		x = pn.newR.Max.X + 8
	}
	end := p.Title.Max.X - 16
	switch {
	case !p.Button.Empty():
		end = p.Button.Min.X - 16
	case !p.Fold.Empty():
		end = p.Fold.Min.X - 16
	}
	a.C.Text(a.F.Small, image.Rect(x+16, p.Title.Min.Y, end, p.Title.Max.Y-2), pn.shownStatus, kobo.Dark, kobo.AlignRight)
}

// drawTermStatus redraws the title strip if the status changed.
// drawTermStatus redraws the title strips whose status changed.
func (a *bro) drawTermStatus() {
	g := a.termGroup()
	if g == nil {
		return
	}
	for _, t := range g.sessions {
		if a.onScreen(t) && t.status(&a.tv) != t.pane.shownStatus {
			a.drawTermTitle(t)
		}
	}
}

func (a *bro) drawConnect(t *termSession) {
	v, pn := &a.tv, &t.pane
	keys := a.keyHints("session.new", "new session", "panel.next", "next panel", "palette", "go to…",
		"fullscreen", "fullscreen (or swipe in from the top or bottom edge to leave it)")
	music := a.keyHints("music.play", "play/pause", "music.prev", "previous track", "music.next", "next track", "statusbar", "status bar")
	var lines []string
	if t.kind == kindShell {
		lines = []string{"A shell on this Kobo (/bin/sh, as " + whoami() + "): Enter or tap here to start it.", keys, music}
		if t.state == termIdle && len(a.tv.groups[t.kind].sessions) > 1 {
			lines = append(lines, "Esc: close this session.")
		}
		if len(t.notes) > 0 {
			lines = append(lines, "")
			lines = append(lines, t.notes...)
		}
		a.DrawTextRows(a.termPanel(t), a.F.Body, lines, &pn.shownConnect)
		pn.hostRows = nil
		return
	}
	lines = []string{"Type user@host[:port] [command] and Enter, or tap / press 1-9 for a saved host.", keys, music}
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
	lines = append(lines, "", fmt.Sprintf("saved hosts (%s):", kobo.AppFile("ssh.conf")))
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
	rows := a.DrawTextRows(a.termPanel(t), a.F.Body, lines, &pn.shownConnect)
	pn.hostRows = nil
	if len(v.hosts) > 0 {
		pn.hostRows = rows[firstHost : firstHost+len(v.hosts)]
	}
}

// updateTerm redraws what changed on t's connect screen, if it's on
// screen: a key typed into a line shouldn't repaint the whole panel (in
// fullscreen, the whole screen).
func (a *bro) updateTerm(t *termSession) {
	switch {
	case !a.onScreen(t):
	case t.pane.p.Folded:
		a.drawTermTitle(t)
	case t.connected() || t.pane.shownConnect == nil:
		a.drawPane(t)
	default:
		a.drawTermStatus()
		a.drawConnect(t)
	}
}

// renderTerm redraws the cells that changed since the last time.
func (a *bro) renderTerm(t *termSession) {
	v, pn := &a.tv, &t.pane
	if t.emu == nil || pn.grid.Empty() {
		return
	}
	t.dirty.Store(false)
	v.drawnAt = time.Now()
	// Only the rows that may have changed are compared: the emulator's
	// touched ones, where the cursor was and is. All of them after a
	// redraw, a switch of screens, with a selection, and every few
	// seconds, in case something changed that vt didn't mark.
	touched, alt := t.touchedRows(pn.rows)
	sel := t == a.term() && v.sel != nil
	full := alt != pn.alt || sel || pn.selShown || time.Since(pn.fullAt) > termFullEvery
	pn.alt, pn.selShown = alt, sel
	if len(pn.shown) != pn.rows || (pn.rows > 0 && len(pn.shown[0]) != pn.cols) {
		full = true
		pn.shown = make([][]cellKey, pn.rows)
		for y := range pn.shown {
			pn.shown[y] = make([]cellKey, pn.cols)
			for x := range pn.shown[y] {
				pn.shown[y][x].width = -1 // never matches
			}
		}
	}
	cur := t.emu.CursorPosition()
	showCursor := !t.emu.CursorHidden()
	type changed struct {
		r    image.Rectangle
		gray bool
	}
	if full {
		pn.fullAt = time.Now()
	}
	var rows []changed
	for y := 0; y < pn.rows; y++ {
		if !full && !touched[y] && y != pn.cursorY && y != cur.Y {
			continue
		}
		from, to, gray := -1, -1, false
		for x := 0; x < pn.cols; x++ {
			k := t.cellKeyAt(x, y, showCursor && cur.X == x && cur.Y == y)
			k.sel = t == a.term() && v.sel.has(x, y)
			if k == pn.shown[y][x] {
				continue
			}
			pn.shown[y][x] = k
			a.drawCell(pn, x, y, k)
			if from < 0 {
				from = x
			}
			to = x
			gray = gray || !bw(k.fg) || !bw(k.bg)
		}
		if from >= 0 {
			r := image.Rect(pn.grid.Min.X+from*v.cw, pn.grid.Min.Y+y*v.ch, pn.grid.Min.X+(to+1)*v.cw, pn.grid.Min.Y+(y+1)*v.ch)
			rows = append(rows, changed{r.Inset(-1).Intersect(pn.grid), gray})
		}
	}
	pn.cursorY = cur.Y
	// Black & white changes to a row or three (typing, a spinner) take the
	// fast waveform. It ghosts, though, so scrolls and redraws, where a
	// moment more doesn't matter, get the clean grayscale one.
	big := len(rows) > 3
	if big {
		v.bigAt = time.Now()
	}
	for _, c := range rows {
		v.churn += kobo.Area(c.r)
		if !c.gray && !big {
			if v.fastArea.Empty() {
				v.fastAt = time.Now()
			}
			v.fastArea = v.fastArea.Union(c.r)
		}
		a.Mark(c.r, c.gray || big)
	}
}

func bw(g uint8) bool { return g == 0 || g == 255 }

func (t *termSession) cellKeyAt(x, y int, cursor bool) cellKey {
	c := t.emu.CellAt(x, y)
	k := cellKey{fg: kobo.Black, bg: kobo.White, width: 1, cursor: cursor}
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
			k.fg = kobo.Black
		} else {
			k.fg = kobo.White
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

func (a *bro) drawCell(pn *termPane, x, y int, k cellKey) {
	v := &a.tv
	if k.width == 0 {
		return // the right half of a wide character, drawn with it
	}
	w := max(1, k.width)
	r := image.Rect(pn.grid.Min.X+x*v.cw, pn.grid.Min.Y+y*v.ch, pn.grid.Min.X+(x+w)*v.cw, pn.grid.Min.Y+(y+1)*v.ch).Intersect(pn.grid)
	fg, bg := k.fg, k.bg
	if k.cursor != k.sel {
		fg, bg = bg, fg
	}
	a.C.Fill(r, bg)
	if k.content != "" && k.content != " " {
		mask := a.glyph(k.content, k.attrs&uv.AttrBold != 0, w)
		a.C.Mask(r, fg, mask, image.Point{})
	}
	if k.ul != 0 {
		a.C.Fill(image.Rect(r.Min.X, r.Max.Y-3, r.Max.X, r.Max.Y-1), fg)
	}
	if k.attrs&uv.AttrStrikethrough != 0 {
		a.C.Fill(image.Rect(r.Min.X, r.Min.Y+v.ch/2, r.Max.X, r.Min.Y+v.ch/2+2), fg)
	}
}

// --- sessions ---

// enterTerm gets a terminal tab ready: the font, the geometry, and a
// first session (the Shell tab's starts its shell right away).
func (a *bro) enterTerm() {
	v := &a.tv
	v.sel = nil // the other tab's
	if v.hosts == nil {
		v.hosts = loadSSHHosts()
	}
	if v.cw == 0 {
		v.fontPx = termFontDefault
		if b, err := os.ReadFile(kobo.AppFile("term-font")); err == nil {
			if px, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil && px != v.fontPx {
				v.fontPx = max(termFontMin, min(termFontMax, px))
				a.tf.setSize(v.fontPx)
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
func (a *bro) newSession(start bool) {
	g := a.termGroup()
	if g == nil || len(g.sessions) >= maxSessions {
		return
	}
	t := &termSession{kind: a.Tab}
	g.sessions = append(g.sessions, t)
	g.cur = len(g.sessions) - 1
	a.tv.sel = nil
	a.layoutSessions(a.Tab)
	log.Printf("terminal: session %d on the %s tab", g.cur+1, a.TabName())
	if t.kind == kindShell && start {
		a.termStartShell(t)
	}
}

// switchSession shows session i of the current tab, opened.
func (a *bro) switchSession(i int) { a.focusSession(i, true) }

// focusSession moves the focus to session i of the current tab; open, it
// opens it too if it's folded.
func (a *bro) focusSession(i int, open bool) {
	g := a.termGroup()
	if g == nil || i < 0 || i >= len(g.sessions) || i == g.cur && !(open && g.sessions[i].folded) {
		return
	}
	was := a.term()
	g.cur = i
	a.tv.sel = nil
	if t := g.sessions[i]; t.folded && open || a.lay.Full { // its panel changes: lay them out again
		t.folded = t.folded && !open
		a.layoutSessions(a.Tab)
		a.drawTerm()
		a.MarkFlash(a.termP.Outer) // a whole new screen: clean, not ghosted
		return
	}
	// Only the focus moves: the two title strips.
	if was != nil {
		a.drawTermTitle(was)
		if was.connected() {
			a.renderTerm(was) // its selection goes
		}
	}
	a.drawTermTitle(g.sessions[i])
}

// setSessionFold folds t's panel or opens it; the focus leaves a folded
// one for an open one.
func (a *bro) setSessionFold(t *termSession, on bool) {
	g := &a.tv.groups[t.kind]
	if t.folded == on {
		return
	}
	t.folded = on
	log.Printf("folds: %s %s", sessionName(t.kind, slices.Index(g.sessions, t)), map[bool]string{true: "folded", false: "open"}[on])
	if on && g.sessions[g.cur] == t {
		for d := 1; d < len(g.sessions); d++ {
			if j := (g.cur + d) % len(g.sessions); !g.sessions[j].folded {
				g.cur = j
				break
			}
		}
		a.tv.sel = nil
	}
	if !on {
		g.cur = slices.Index(g.sessions, t)
	}
	a.layoutSessions(t.kind)
	if a.Tab == t.kind {
		a.drawTerm()
		a.MarkFlash(a.termP.Outer)
	}
}

// closeSession ends the session on screen and drops it, unless it's the
// tab's only one.
func (a *bro) closeSession() {
	g, t := a.termGroup(), a.term()
	if t == nil || len(g.sessions) < 2 {
		return
	}
	a.termClose(t)
	g.sessions = slices.Delete(g.sessions, g.cur, g.cur+1)
	g.cur = min(g.cur, len(g.sessions)-1)
	a.tv.sel = nil
	a.layoutSessions(a.Tab)
	a.drawTerm()
	a.MarkFlash(a.termP.Outer)
}

// termFontSwipe follows a swipe along the right edge, dy pixels up so
// far: half the screen height covers the whole range. The gauge shows the
// size; the terminal gets it when the finger lifts, since every change
// means a reflow on the remote and a full redraw here.
func (a *bro) termFontSwipe(f *fontSwipe, dy int) {
	steps := (termFontMax - termFontMin) / termFontStep
	px := f.from + dy*steps/(a.C.Bounds().Dy()/2)*termFontStep
	px = max(termFontMin, min(termFontMax, px))
	if px == f.to && a.GaugeUp() {
		return
	}
	f.to = px
	b := a.C.Bounds()
	cy := b.Dy() / 2
	r := image.Rect(b.Max.X-kobo.EdgeWidth-130, cy-260, b.Max.X-kobo.EdgeWidth-20, cy+260)
	a.DrawGauge(r, px-termFontMin, termFontMax-termFontMin, fmt.Sprintf("%d px", px))
}

// setTermFont changes the terminal's font size; the remotes get the new
// cell count, as when a terminal window is resized.
func (a *bro) setTermFont(px int) {
	v := &a.tv
	px = max(termFontMin, min(termFontMax, px))
	if px == v.fontPx {
		return
	}
	log.Printf("terminal font: %d px", px)
	v.fontPx = px
	a.tf.setSize(px)
	v.glyphs = nil
	a.termGeometry()
	os.WriteFile(kobo.AppFile("term-font"), []byte(strconv.Itoa(px)+"\n"), 0o644)
	if a.term() != nil {
		a.drawTerm()
		a.MarkFlash(a.termP.Outer) // every glyph changed: clean, not ghosted
	}
}

func (a *bro) termNote(t *termSession, s string) {
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
func (a *bro) waitToReconnect(t *termSession) {
	t.retries++
	t.state, t.retryAt = termWaiting, time.Now().Add(reconnectDelay(t.retries))
	a.termNote(t, fmt.Sprintf("reconnecting in %.0f s (Esc: stop)", reconnectDelay(t.retries).Seconds()))
}

func (a *bro) termConnect(t *termSession, h sshHost) {
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
		a.Do(func() {
			if t.gen == gen {
				t.prompt = &termPrompt{question: q, echo: echo, yesno: yesno, reply: reply}
				a.updateTerm(t)
			} else {
				close(reply)
			}
		})
		ans, ok := <-reply
		if !ok {
			return "", errors.New("canceled")
		}
		return ans, nil
	}
	size := t.startSize()
	o := dialOpts{
		cols: size.X, rows: size.Y, password: t.password,
		ask:  func(q string, echo bool) (string, error) { return ask(q, echo, false) },
		note: func(s string) { a.Do(func() { a.termNote(t, s) }) },
	}
	if a.tv.keyAsked == nil {
		a.tv.keyAsked = map[string]bool{}
	}
	if !a.tv.keyAsked[h.String()] {
		o.confirm = func(q string) bool {
			a.Do(func() { a.tv.keyAsked[h.String()] = true })
			ans, err := ask(q, true, true)
			return err == nil && ans == "y"
		}
	}
	go func() {
		s, typed, err := dialSSH(h, o)
		a.Do(func() {
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
		})
	}()
}

func (a *bro) termStartShell(t *termSession) {
	if t.state != termIdle {
		return
	}
	t.notes, t.title = nil, ""
	size := t.startSize()
	s, err := startShell(size.X, size.Y)
	if err != nil {
		a.termNote(t, "couldn't start a shell: "+err.Error())
		return
	}
	t.gen++
	a.termStart(t, s, t.gen)
}

func (a *bro) termStart(t *termSession, back termBackend, gen int) {
	t.size = t.startSize()
	cols, rows := t.size.X, t.size.Y
	t.back, t.state = back, termConnected
	t.emu = vt.NewSafeEmulator(cols, rows)
	t.emu.SetDefaultForegroundColor(color.Black) // what apps get when they ask, so they pick light-theme colors
	t.emu.SetDefaultBackgroundColor(color.White)
	t.emu.SetScrollbackSize(1000)    // not shown; the default 10000 costs a big copy per line once full
	t.emu.SetCallbacks(vt.Callbacks{ // they're called under t.mu, which the loop may want: not a.Do there
		Title: func(s string) {
			go a.Do(func() {
				t.title = s
				if a.onScreen(t) {
					a.drawTermStatus()
				}
			})
		},
		Bell: func() { go a.Do(func() { a.termNotify(t, "") }) },
	})
	t.emu.RegisterOscHandler(52, a.osc52(t))
	t.emu.RegisterOscHandler(9, a.oscNotify(t))
	t.emu.RegisterOscHandler(777, a.oscNotify(t))
	back.resize(cols, rows)
	if t.kind == kindShell {
		log.Printf("shell: started, %d×%d", cols, rows)
	} else {
		log.Printf("ssh: connected to %s, %d×%d", t.target, cols, rows)
	}
	emu := t.emu
	go func() { // remote → emulator
		buf := make([]byte, 32<<10)
		var err error
		for {
			var n int
			n, err = back.Read(buf)
			if n > 0 {
				t.write(buf[:n])
				t.dirty.Store(true)
			}
			if err != nil {
				break
			}
		}
		lost, how := back.ended()
		a.Do(func() { a.termEnded(t, gen, err, lost, how) })
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
	if a.onScreen(t) {
		a.drawPane(t)
	}
}

func (a *bro) termEnded(t *termSession, gen int, err error, lost bool, how string) {
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
func (a *bro) termClose(t *termSession) {
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
	t.pane.shown = nil
}

// termCloseAll ends every session, when we quit.
func (a *bro) termCloseAll() {
	for _, g := range a.tv.groups {
		for _, t := range g.sessions {
			a.termClose(t)
		}
	}
}

// termTick starts the reconnections that are due, redraws from the
// emulator now and then, and gives the screen a clean grayscale refresh
// once output settles after fast updates.
func (a *bro) termTick() {
	for _, g := range a.tv.groups {
		for _, t := range g.sessions {
			if t.state == termWaiting && time.Now().After(t.retryAt) {
				a.termConnect(t, t.target)
			}
		}
	}
	g, v := a.termGroup(), &a.tv
	if g == nil || a.Calibrating() {
		return
	}
	if slices.ContainsFunc(g.sessions, func(t *termSession) bool { return t.state == termWaiting }) {
		a.drawTermStatus() // the countdown
	}
	if a.GaugeUp() || a.overlay() {
		return // something is up over the terminal: drawing under it would be undone when it goes
	}
	if time.Since(v.drawnAt) >= termFrame {
		for _, t := range g.sessions {
			if t.connected() && t.dirty.Load() && a.showing(t) {
				v.outAt = time.Now()
				a.renderTerm(t)
			}
		}
	}
	if a.Blitting() {
		return
	}
	// Clean up after the fast waveform once output pauses, or every few
	// seconds if it doesn't (a spinner never stops while its app works).
	if !v.fastArea.Empty() && (time.Since(v.outAt) > 1200*time.Millisecond || time.Since(v.fastAt) > 4*time.Second) {
		a.Mark(v.fastArea, true)
		v.fastArea = image.Rectangle{}
	}
	// Grayscale updates leave ghosts too, slowly: after a few screenfuls
	// of change, flash the terminal clean once it settles (a clock or a
	// spinner may still tick), as e-readers do every few pages.
	if v.churn > 4*kobo.Area(a.termP.Body) && time.Since(v.bigAt) > 2500*time.Millisecond {
		v.churn, v.fastArea = 0, image.Rectangle{}
		a.MarkFlash(a.termP.Body)
	}
}

// --- input ---

// termKey takes a key event while a terminal tab is up. It returns false
// for keys the rest of the app should handle (tab switching, quitting)
// when there's no session to send them to.
func (a *bro) termKey(code uint16, value int32) bool {
	t, g, v := a.term(), a.termGroup(), &a.tv
	m := &v.mods
	if a.termMod(code, value) {
		return true
	}
	if value == 0 {
		return true
	}
	a.clearSel() // a key: the selection's done with
	ch, printable := kobo.KeyChar(code, m.shift, m.caps)

	if t.prompt != nil {
		p := t.prompt
		switch {
		case p.yesno && (code == kobo.KeyEnter || ch == "y" || ch == "Y"):
			p.reply <- "y"
			t.prompt = nil
		case p.yesno && (code == kobo.KeyEsc || ch == "n" || ch == "N"):
			p.reply <- "n"
			t.prompt = nil
		case p.yesno:
		case code == kobo.KeyEnter:
			p.reply <- p.answer
			t.prompt = nil
		case code == kobo.KeyEsc:
			a.termClose(t)
			t.retries = 0
			a.termNote(t, "canceled")
		case code == kobo.KeyBackspace:
			if r := []rune(p.answer); len(r) > 0 {
				p.answer = string(r[:len(r)-1])
			}
		case code == kobo.KeyTab:
			p.reveal = !p.reveal
		case printable:
			p.answer += ch
		}
		a.updateTerm(t)
		return true
	}

	if !t.connected() {
		switch {
		case (t.state == termConnecting || t.state == termWaiting) && code == kobo.KeyEsc:
			a.termClose(t)
			t.retries = 0
			a.termNote(t, "canceled")
		case t.state != termIdle:
		case code == kobo.KeyEsc && len(g.sessions) > 1:
			a.closeSession()
			return true
		case t.kind == kindShell && code == kobo.KeyEnter:
			a.termStartShell(t)
			return true
		case t.kind == kindShell:
			return code != kobo.KeyEsc
		case code == kobo.KeyEnter:
			in := strings.TrimSpace(t.input)
			if in == "" && len(v.hosts) > 0 {
				a.termConnect(t, v.hosts[0])
			} else if h, err := parseSSHTarget(in); err == nil {
				t.typed = true
				a.termConnect(t, h)
			} else {
				a.termNote(t, err.Error())
			}
		case code == kobo.KeyBackspace:
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

	if k, ok := termKeyEvent(code, *m); ok {
		t.emu.SendKey(k)
		t.keyAt = time.Now()
	}
	return true
}

// termKeyEvent is the key for the emulator to send on, if it's one it
// knows. Never IsRepeat: vt matches keys whole, so a held arrow would
// match none and go out as its private-use rune.
func termKeyEvent(code uint16, m termMods) (uv.KeyPressEvent, bool) {
	var k uv.KeyPressEvent
	if m.ctrl {
		k.Mod |= uv.ModCtrl
	}
	if m.alt {
		k.Mod |= uv.ModAlt
	}
	if m.meta {
		k.Mod |= uv.ModMeta
	}
	if special, ok := termSpecialKeys[code]; ok {
		k.Code = special
		if m.shift {
			k.Mod |= uv.ModShift
		}
		return k, true
	}
	ch, printable := kobo.KeyChar(code, m.shift, m.caps)
	if !printable {
		return k, false
	}
	plain, _ := kobo.KeyChar(code, false, false)
	k.Code = []rune(plain)[0]
	if !m.ctrl {
		k.Text = ch
	}
	return k, true
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
	kobo.KeyEsc: uv.KeyEscape, kobo.KeyBackspace: uv.KeyBackspace, kobo.KeyTab: uv.KeyTab, kobo.KeyEnter: uv.KeyEnter, 96: uv.KeyEnter,
	102: uv.KeyHome, 103: uv.KeyUp, keyPgUp: uv.KeyPgUp, keyLeft: uv.KeyLeft, keyRight: uv.KeyRight, 107: uv.KeyEnd,
	108: uv.KeyDown, keyPgDn: uv.KeyPgDown, 110: uv.KeyInsert, 111: uv.KeyDelete,
	59: uv.KeyF1, 60: uv.KeyF2, 61: uv.KeyF3, 62: uv.KeyF4, 63: uv.KeyF5, 64: uv.KeyF6,
	65: uv.KeyF7, 66: uv.KeyF8, 67: uv.KeyF9, 68: uv.KeyF10, keyF11: uv.KeyF11, keyF12: uv.KeyF12,
}

// termMod keeps track of the modifier keys; true if code is one.
func (a *bro) termMod(code uint16, value int32) bool {
	m := &a.tv.mods
	down := value != 0
	switch code {
	case kobo.KeyLeftShift, kobo.KeyRightShift:
		m.shift = down
	case kobo.KeyLeftCtrl, kobo.KeyRightCtrl:
		m.ctrl = down
	case keyLeftAlt, keyRightAlt:
		m.alt = down
	case keyLeftMeta, keyRightMeta:
		m.meta = down
	case kobo.KeyCapsLock:
		if value == 1 {
			m.caps = !m.caps
			if t := a.term(); t != nil && !t.connected() {
				a.updateTerm(t) // the prompt says so
			}
		}
	default:
		return false
	}
	return true
}

// termCell maps a screen point to a cell, if it's on the grid.
func (a *bro) termCell(p image.Point) (int, int, bool) {
	t, v := a.term(), &a.tv
	if t == nil || !p.In(t.pane.grid) || v.cw == 0 {
		return 0, 0, false
	}
	return (p.X - t.pane.grid.Min.X) / v.cw, (p.Y - t.pane.grid.Min.Y) / v.ch, true
}

// sessionAt is the session whose panel p is on, and its place in the tab.
func (a *bro) sessionAt(p image.Point) (*termSession, int) {
	if g := a.termGroup(); g != nil {
		for i, t := range g.sessions {
			if p.In(t.pane.p.Outer) {
				return t, i
			}
		}
	}
	return nil, -1
}

// termTap handles a tap on a terminal tab: the panel's button, the
// session buttons, a saved host, or a click for the remote app.
func (a *bro) termTap(p image.Point) {
	t, v := a.term(), &a.tv
	if t == nil {
		return
	}
	if on, i := a.sessionAt(p); on != nil && on != t { // another panel: it takes the focus, and that's all
		a.switchSession(i)
		return
	}
	if kobo.Hit(p, a.termPanel(t).Button) {
		switch {
		case t.state != termIdle:
			a.termClose(t)
			t.retries = 0
			a.termNote(t, "disconnected")
			a.drawPane(t)
		default:
			a.closeSession()
		}
		return
	}
	if kobo.Hit(p, t.pane.newR) {
		a.newSession(true)
		a.drawTerm()
		a.MarkFlash(a.termP.Outer)
		return
	}
	if !t.connected() {
		if t.kind == kindShell && p.In(t.pane.p.Body) && t.state == termIdle {
			a.termStartShell(t)
			return
		}
		for i, r := range t.pane.hostRows {
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
	if a.tv.sel != nil && a.tv.sel.on { // a tap takes the selection away, and only that
		a.clearSel()
		return
	}
	m := uv.Mouse{X: x, Y: y, Button: uv.MouseLeft}
	t.emu.SendMouse(uv.MouseClickEvent(m))
	t.emu.SendMouse(uv.MouseReleaseEvent(m))
}

// termSwipe turns a vertical swipe into mouse wheel steps, one per row moved.
func (a *bro) termSwipe(from, to image.Point) bool {
	t, v := a.term(), &a.tv
	x, y, ok := a.termCell(from)
	if t == nil || !t.connected() || !ok {
		return false
	}
	dy := to.Y - from.Y
	steps := min(20, kobo.Abs(dy)/v.ch)
	if steps == 0 || kobo.Abs(dy) < 2*kobo.Abs(to.X-from.X) {
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
