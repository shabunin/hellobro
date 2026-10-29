package main

// The terminals: SSH, with a real PTY on the remote, a shell on the Kobo
// itself, or Alpine in a jail, a window each. A new one (Meta+Enter) is a
// launcher: it asks what to run, and goes back to asking when that ends.
// Their output goes through an xterm emulator (charmbracelet/x/vt) into
// a grid of cells, which we draw ourselves, redrawing only the cells that
// changed. Keys and touches go back as xterm key sequences and mouse
// events.

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"log"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"

	"hellobro/kobo"
)

const (
	termIdle = iota
	termConnecting
	termWaiting // to connect again, after the connection dropped
	termConnected
)

// A session's kind: what it runs, or ran last.
const (
	kindNone = iota // nothing yet: a launcher
	kindSSH
	kindShell
)

const (
	termFrame     = 60 * time.Millisecond  // at most this often we redraw from the emulator
	termBackFrame = time.Second            // and a pane not in focus, at most this often
	termTyping    = time.Second            // after a key, a pane not in focus waits
	termUnseen    = 250 * time.Millisecond // a session off screen reads a trickle of output this often
	termFullEvery = 3 * time.Second        // and this often we compare all the cells, not only the touched rows
)

// The terminal's font size in pixels: Ctrl+= and Ctrl+- step it, Ctrl+0
// resets it, and it's remembered in hellobro.json.
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
	sel       bool     // selected: shown reversed
	img       uint32   // a placement's cell (kittygfx.go): its key
	place     *kgPlace // and the placement, nil if it's gone
	ix, iy    uint8    // which of its cells
}

type termMods struct{ shift, ctrl, alt, meta, caps bool }

// termBackend is what a session talks to: an SSH session or a local shell.
type termBackend interface {
	io.Reader                              // output
	io.Writer                              // input
	resize(cols, rows int, px image.Point) // px: the cells' size in pixels, all told
	close()
	// ended says, once the output has ended, whether the connection was
	// lost (rather than the remote exiting), and how it ended.
	ended() (lost bool, how string)
}

// termSession is one terminal: an SSH connection or a shell.
type termSession struct {
	kind   int
	alpine bool   // a shell in the Alpine jail (alpine.go)
	exec   string // what it runs there, if not a shell
	input  string // the ssh> line
	notes  []string
	state  int
	target sshHost
	typed  bool     // the target was typed in: saved once it works
	edit   *sshHost // the saved host the line is editing, if it is
	prompt *termPrompt
	gen    int // bumps per connection, so late messages from an old one are ignored

	back     termBackend
	emu      *vt.SafeEmulator
	kitty    kittyKeys  // kitty's keyboard protocol, if the program asked for it (kitty.go)
	gfx      kittyGfx   // and its images (kittygfx.go), under mu
	fontPx   int        // its font's size (face)
	mu       sync.Mutex // held while the emulator changes (write): its touched lines are read and cleared under it
	dirty    atomic.Bool
	seen     atomic.Bool // its cells show: its output is read as it comes, else in batches
	title    string
	keyAt    time.Time // a key last went to it
	notifyAt time.Time // its last notice

	password string    // typed for this connection, for connecting again (only kept in memory)
	retries  int       // connection attempts since it dropped
	retryAt  time.Time // when the next one starts

	pane termPane
	size image.Point // the emulator's and the remote's, in cells
}

// termPane is where a session shows: its panel, and its cells in it.
type termPane struct {
	p            kobo.Panel
	grid         image.Rectangle // the cells' area; empty when they don't show
	cols, rows   int
	shown        [][]cellKey // what's drawn
	shownConnect []string    // the connect screen's rows as drawn, nil: draw it all
	shownStatus  string
	picks        []launchPick    // the launcher's choices, as drawn, for taps
	cursorY      int             // the row the cursor was drawn on
	alt          bool            // the alternate screen was drawn
	selShown     bool            // a selection was drawn
	fullAt       time.Time       // all the cells were last compared
	drawnAt      time.Time       // its output was last drawn
	fastArea     image.Rectangle // drawn with the fast (DU) waveform since the last clean refresh
	fastAt       time.Time       // when fastArea got its first update
}

// write puts the remote's output into the emulator.
func (t *termSession) write(p []byte) {
	t.mu.Lock()
	t.gfx.feed(p, func(b []byte) { t.emu.Write(b) }, t.gfxCommand)
	t.mu.Unlock()
}

// touchedRows are the rows the emulator changed since the last call, and
// whether it's on its alternate screen. vt keeps them but has no way to
// clear them, so we clear its list ourselves. It's called under t.mu.
func (t *termSession) touchedRows(rows int) ([]bool, bool) {
	touched := make([]bool, rows)
	ls := t.emu.Emulator.Touched()
	for y, l := range ls {
		if l != nil && y < rows {
			touched[y] = true
		}
		ls[y] = nil
	}
	return touched, t.emu.Emulator.IsAltScreen()
}

func (t *termSession) connected() bool { return t.state == termConnected }

// termView is the terminals on screen. Each session has a panel of its
// own; they share the font and the glyph cache.
type termView struct {
	hosts    []sshHost
	keyAsked map[string]bool // hosts we offered to install our key on

	nearby  nearbyState // the other hellobros heard (typein.go)
	sel     *termSel    // the selection, or a contact that may become one (clipboard.go): the focused session's
	drawnAt time.Time
	churn   int       // pixels changed since the last flash
	bigAt   time.Time // the last change of more than a few rows

	mods   termMods
	faces  map[int]*termFace // the font, by size: the terminals' sizes
	fontPx int               // the size last set: a new terminal's

	tiles     map[*kgPlace]*image.Gray // placements drawn for the cells' size (kittygfx.go)
	tileBytes int
}

// term is the terminal in focus, if it's one.
func (a *bro) term() *termSession {
	t, _ := a.focusedWin().(*termSession)
	return t
}

// onScreen says whether t's panel is up.
func (a *bro) onScreen(t *termSession) bool {
	return !a.Calibrating() && !a.overlay() && !a.sketching() && t != nil && !t.pane.p.Outer.Empty()
}

// showing says whether t's cells or launcher are up.
func (a *bro) showing(t *termSession) bool { return a.onScreen(t) }

// termLive says whether any session is connected, or about to be.
func (a *bro) termLive() bool {
	for _, t := range a.terms() {
		if t.state != termIdle {
			return true
		}
	}
	return false
}

// --- the window ---

func (t *termSession) Class() string { return "terminal" }

func (t *termSession) Instance() string {
	switch {
	case t.launcher():
		return "launcher"
	case t.kind == kindShell && t.alpine:
		return "alpine"
	case t.kind == kindShell:
		return "sh"
	}
	return "ssh"
}

// Title is its name, or the title its program gave it.
func (t *termSession) Title() string {
	if t.title != "" && t.connected() {
		return t.title
	}
	return t.name()
}

// name is what runs in it: "Alpine", a host's name.
func (t *termSession) name() string {
	switch {
	case t.launcher():
		return "new window"
	case t.kind == kindShell && t.alpine:
		return "Alpine"
	case t.kind == kindShell:
		return "Kobo sh"
	case t.target.name != "":
		return t.target.name
	}
	return t.target.String()
}

// launcher says whether it's asking what to run.
func (t *termSession) launcher() bool { return t.state == termIdle }

func (t *termSession) place(a *bro, p kobo.Panel) {
	t.pane.p = p
	a.paneGeometry(t)
}

func (t *termSession) panel(a *bro) kobo.Panel   { return a.termPanel(t) }
func (t *termSession) draw(a *bro)               { a.drawPane(t) }
func (t *termSession) drawTitle(a *bro)          { a.drawTermTitle(t) }
func (t *termSession) tap(a *bro, p image.Point) { a.termTap(p) }
func (t *termSession) closing(a *bro)            { a.termClose(t) }

func (t *termSession) press(a *bro, code uint16, value int32) bool { return a.termKey(code, value) }

// --- layout & geometry ---

// termGeometry lays the sessions out again.
func (a *bro) termGeometry() {
	v := &a.tv
	v.sel = nil // its cells are other ones now
	for _, t := range a.terms() {
		a.paneGeometry(t)
	}
}

// paneGeometry fits t's cells to its panel. A session that doesn't show
// keeps its size: the remote isn't told of every fold.
func (a *bro) paneGeometry(t *termSession) {
	f, pn := a.face(t), &t.pane
	pn.shown, pn.shownConnect, pn.picks, pn.grid = nil, nil, nil, image.Rectangle{}
	if pn.p.Body.Empty() {
		return
	}
	grid := pn.p.Body.Inset(6)
	pn.cols, pn.rows = max(20, grid.Dx()/f.cw), max(5, grid.Dy()/f.ch)
	// Centered in the body.
	gw, gh := pn.cols*f.cw, pn.rows*f.ch
	at := image.Pt(grid.Min.X+(grid.Dx()-gw)/2, grid.Min.Y+(grid.Dy()-gh)/2)
	pn.grid = image.Rectangle{at, at.Add(image.Pt(gw, gh))}
	if !t.connected() {
		return
	}
	cell := f.cell()
	t.mu.Lock()
	moved := cell != t.gfx.cell
	t.gfx.cell = cell
	if size := image.Pt(pn.cols, pn.rows); size != t.size {
		t.size, moved = size, true
		t.emu.Resize(pn.cols, pn.rows)
	}
	t.mu.Unlock()
	if moved {
		t.back.resize(pn.cols, pn.rows, image.Pt(gw, gh))
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

// termPanel is t's panel as it is now: its name, in focus or not, with a
// button that fits what the session is doing.
func (a *bro) termPanel(t *termSession) kobo.Panel {
	p := t.pane.p
	if p.Title.Empty() {
		return p
	}
	p.Name = t.name()
	switch {
	case t.state != termIdle && t.kind == kindShell:
		p.ButtonLabel = "kill"
	case t.state != termIdle:
		p.ButtonLabel = "disconnect"
	default:
		p.ButtonLabel = ""
	}
	focused := a.term() == t
	return kobo.WithWindow(a.F, p, focused, focused)
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
			where = map[bool]string{false: "sh on this Kobo", true: "Alpine, in a jail"}[t.alpine]
		}
		s := fmt.Sprintf("%s · %d×%d · %d px", where, t.size.X, t.size.Y, t.fontPx)
		if t.title != "" {
			s = t.title + " · " + s
		}
		return s
	}
	return "1 Alpine · 2 the Kobo's sh · 3… a host"
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
	case t.connected():
		pn.shown = nil
		a.renderTerm(t)
	default:
		a.drawConnect(t)
	}
}

// drawTermTitle draws t's title strip: its name, + and ×, its button,
// and the session's status.
func (a *bro) drawTermTitle(t *termSession) {
	pn := &t.pane
	pn.shownStatus = t.status(&a.tv)
	a.DrawPanelTitle(a.termPanel(t), pn.shownStatus)
}

// drawTermStatus redraws the title strips whose status changed.
func (a *bro) drawTermStatus() {
	for _, t := range a.terms() {
		if a.onScreen(t) && t.status(&a.tv) != t.pane.shownStatus {
			a.drawTermTitle(t)
		}
	}
}

// launchPick is a choice on the launcher, where it's drawn.
type launchPick struct {
	r  image.Rectangle
	do func()
}

// guiKeys are the settings' windows, as the launcher offers them.
var guiKeys = []string{"wifi", "bluetooth", "sharing", "settings", "battery", "usage"}

// drawConnect draws the launcher: what can run here (Alpine, the Kobo's
// sh, the saved hosts, by number), the ssh> line for a host not saved,
// the settings' windows, and how the last thing ended. While SSH logs
// in, its questions are asked here.
func (a *bro) drawConnect(t *termSession) {
	v, pn := &a.tv, &t.pane
	keys := a.keyHints("launcher", "another window", "palette", "the switcher", "kill", "close it")
	var lines []string
	type pick struct {
		line int
		do   func()
	}
	var picks []pick
	add := func(do func(), l ...string) {
		if do != nil {
			picks = append(picks, pick{len(lines), do})
		}
		lines = append(lines, l...)
	}
	idle := t.state == termIdle && t.edit == nil && t.prompt == nil
	run := func(do func()) func() {
		if !idle {
			return nil
		}
		return do
	}
	add(nil, "What runs here? Press its number, or tap it.")
	alpine := "   1   Alpine Linux " + alpineVersion + ", in a jail on this Kobo"
	if _, err := os.Stat(alpineImg); err != nil {
		alpine += " (set up the first time: a disk image, 3 MB from the net)"
	}
	add(run(func() { a.termStartShell(t, true) }), alpine)
	add(run(func() { a.termStartShell(t, false) }), "   2   the Kobo's own /bin/sh, as "+whoami())
	for i, h := range v.hosts {
		if i+3 > 9 {
			break
		}
		line := fmt.Sprintf("   %d   %s   %s", i+3, h.name, h)
		if h.cmd != "" {
			line += "   ▸ " + h.cmd
		}
		if m := a.mounted(h); m != nil && m.cmd != nil {
			line += "   · mounted, " + mountMode(m.rw)
		}
		add(run(func() { a.hostMenu(t, i) }), line)
	}
	a.nearbyLook()
	for i, p := range v.nearby.peers {
		if n := len(v.hosts) + i + 3; n <= 9 {
			add(run(func() { a.typeInto(t, p) }), fmt.Sprintf("   %d   %s   nearby: type into it", n, p.Name))
		}
	}
	add(nil, "", "Or a host not saved: user@host[:port] [command] and Enter (saved once it connects).")
	switch {
	case t.prompt != nil && t.prompt.yesno:
		add(nil, t.prompt.question, "(Enter: yes, Esc: no)")
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
		add(nil, t.prompt.question+shown+"█", help)
	case t.state == termIdle && t.edit != nil:
		add(nil, "edit> "+t.input+"█", "(name  user@host[:port]  [via user@jump]  [command]; Enter to save, Esc to leave it)")
	case t.state == termIdle:
		add(nil, "ssh> "+t.input+"█")
	case t.state == termWaiting:
		add(nil, "ssh> "+t.input, "(Esc: stop reconnecting)")
	default:
		add(nil, "ssh> "+t.input)
	}
	add(nil, "")
	add(run(func() { a.replaceWin(t, a.newBookWin("")) }), "Books: the EPUBs and PDFs on this Kobo, to read (tap here)")
	guiLine := len(lines)
	add(nil, "", "Settings, in a window:   "+strings.Join(guiNames(), "   "))
	notes := append([]string{"new"}, noteNames()...)
	notes = notes[:min(len(notes), 4)] // the last three written
	noteLine := len(lines)
	add(nil, "Notebooks, to write in with the pen:   "+strings.Join(notes, "   "))
	add(nil, "", keys, "Esc: close this window.")
	add(nil, "", "This Kobo's key, for ~/.ssh/authorized_keys (also in id_ed25519.pub next to hellobro):")
	if s, err := clientKey(); err == nil {
		key := authorizedKey(s)
		for len(key) > 0 {
			n := min(len(key), 64)
			add(nil, "   "+key[:n])
			key = key[n:]
		}
	} else {
		add(nil, "   "+err.Error())
	}
	if len(t.notes) > 0 {
		add(nil, "")
		add(nil, t.notes...)
	}
	rows := a.DrawTextRows(a.termPanel(t), a.F.Body, lines, &pn.shownConnect)
	pn.picks = nil
	for _, p := range picks {
		if r := rows[p.line]; !r.Empty() {
			pn.picks = append(pn.picks, launchPick{r, p.do})
		}
	}
	// the settings' windows, a word each
	if r := rows[guiLine+1]; !r.Empty() && idle {
		x := r.Min.X + kobo.TextWidth(a.F.Body, "Settings, in a window:   ")
		for i, key := range guiKeys {
			w := kobo.TextWidth(a.F.Body, guiName(key))
			pn.picks = append(pn.picks, launchPick{image.Rect(x-8, r.Min.Y, x+w+8, r.Max.Y), func() { a.replaceWin(t, &guiWin{key: guiKeys[i]}) }})
			x += w + kobo.TextWidth(a.F.Body, "   ")
		}
	}
	if r := rows[noteLine]; !r.Empty() && idle {
		x := r.Min.X + kobo.TextWidth(a.F.Body, "Notebooks, to write in with the pen:   ")
		for i, n := range notes {
			w := kobo.TextWidth(a.F.Body, n)
			if i == 0 {
				n = ""
			}
			pn.picks = append(pn.picks, launchPick{image.Rect(x-8, r.Min.Y, x+w+8, r.Max.Y), func() { a.launchNotebook(t, n) }})
			x += w + kobo.TextWidth(a.F.Body, "   ")
		}
	}
}

// updateTerm redraws what changed on t's connect screen, if it's on
// screen: a key typed into a line shouldn't repaint the whole panel (in
// fullscreen, the whole screen).
func (a *bro) updateTerm(t *termSession) {
	switch {
	case !a.onScreen(t):
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
	sel := t == a.term() && v.sel != nil
	full := sel || pn.selShown || time.Since(pn.fullAt) > termFullEvery
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
	// The cells are read under t.mu, which every write and resize holds:
	// then they're vt's own, without its lock taken for each (a few
	// thousand times a frame, that was a good part of the drawing). What
	// changed is drawn after, so the output isn't kept waiting on glyphs.
	type changed struct {
		y, from, to int
		gray        bool
	}
	var rows []changed
	var cells []image.Point
	t.mu.Lock()
	emu := t.emu.Emulator
	// Only the rows that may have changed are compared: the emulator's
	// touched ones, where the cursor was and is. All of them after a
	// redraw, a switch of screens, with a selection, and every few
	// seconds, in case something changed that vt didn't mark.
	touched, alt := t.touchedRows(pn.rows)
	full = full || alt != pn.alt || t.gfx.takeChanged()
	cur := emu.CursorPosition()
	showCursor := !emu.CursorHidden()
	for y := 0; y < pn.rows; y++ {
		if !full && !touched[y] && y != pn.cursorY && y != cur.Y {
			continue
		}
		c := changed{y, -1, -1, false}
		shown := pn.shown[y]
		var k cellKey
		var left *kgHolder
		for x := range shown {
			cl := emu.CellAt(x, y)
			k.set(cl, showCursor && cur.X == x && cur.Y == y)
			if k.img != 0 {
				k.place = t.gfx.places[k.img]
			}
			if h, ok := kgHolderOf(cl, left); ok { // a placeholder: the image's, or blank
				k.content, k.width, k.attrs, k.ul = "", 1, 0, 0
				if pl := t.gfx.virtualFor(h.id, h.pid); pl != nil && h.row < pl.rows && h.col < pl.cols {
					k.img, k.place, k.iy, k.ix = pl.key, pl, uint8(h.row), uint8(h.col)
				}
				left = &h
			} else {
				left = nil
			}
			k.sel = sel && v.sel.has(x, y) // sel, not a.term() again: a cell at a time, that's a map lookup each
			if k.same(&shown[x]) {
				continue
			}
			shown[x] = k
			cells = append(cells, image.Pt(x, y))
			if c.from < 0 {
				c.from = x
			}
			c.to = x
			c.gray = c.gray || !bw(k.fg) || !bw(k.bg) || k.place != nil
		}
		if c.from >= 0 {
			rows = append(rows, c)
		}
	}
	t.mu.Unlock()
	pn.alt, pn.selShown, pn.cursorY = alt, sel, cur.Y
	if full {
		pn.fullAt = time.Now()
	}
	// Black & white changes to a row or three (typing, a spinner) take the
	// fast waveform. It ghosts, though, so scrolls and redraws, where a
	// moment more doesn't matter, get the clean grayscale one.
	big := len(rows) > 3
	if big {
		v.bigAt = time.Now()
	}
	f := a.face(t)
	for _, p := range cells {
		a.drawCell(pn, f, p.X, p.Y, pn.shown[p.Y][p.X])
	}
	for _, c := range rows {
		r := image.Rect(pn.grid.Min.X+c.from*f.cw, pn.grid.Min.Y+c.y*f.ch, pn.grid.Min.X+(c.to+1)*f.cw, pn.grid.Min.Y+(c.y+1)*f.ch)
		r = r.Inset(-1).Intersect(pn.grid)
		v.churn += kobo.Area(r)
		if !c.gray && !big {
			if pn.fastArea.Empty() {
				pn.fastAt = time.Now()
			}
			pn.fastArea = pn.fastArea.Union(r)
		}
		a.Mark(r, c.gray || big)
	}
}

func bw(g uint8) bool { return g == 0 || g == 255 }

// same is k == o, with the cheap fields first: most cells haven't changed,
// and those that have mostly differ in their content.
func (k *cellKey) same(o *cellKey) bool {
	return k.content == o.content && k.fg == o.fg && k.bg == o.bg && k.attrs == o.attrs &&
		k.ul == o.ul && k.width == o.width && k.cursor == o.cursor && k.sel == o.sel &&
		k.place == o.place && k.ix == o.ix && k.iy == o.iy
}

// set makes k what a cell of the emulator looks like on the panel, all
// but sel. Field by field: a new cellKey for each cell, zeroed and
// copied, cost as much as the rest.
func (k *cellKey) set(c *uv.Cell, cursor bool) {
	k.fg, k.bg, k.cursor = kobo.Black, kobo.White, cursor
	k.img, k.place, k.ix, k.iy = 0, nil, 0, 0
	if c == nil {
		k.content, k.width, k.attrs, k.ul = "", 1, 0, 0
		return
	}
	if key, row, col, ok := kgCell(c); ok { // drawn from the image, not the font
		k.content, k.width, k.attrs, k.ul = "", 1, 0, 0
		k.img, k.iy, k.ix = key, row, col
		return
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
}

// gray turns a color into one of the 16 levels the panel shows cleanly.
// The 256 colors' are worked out once (grays): most cells' colors are of
// them.
func gray(c color.Color) uint8 {
	switch c := c.(type) {
	case ansi.IndexedColor:
		return grays[c]
	case ansi.BasicColor:
		if c < 16 {
			return grays[c]
		}
	}
	return grayOf(c)
}

var grays = func() (g [256]uint8) {
	for i := range g {
		g[i] = grayOf(ansi.IndexedColor(i))
	}
	return g
}()

func grayOf(c color.Color) uint8 {
	r, g, b, _ := c.RGBA()
	y := (299*r + 587*g + 114*b) / 1000 >> 8
	return uint8((y + 8) / 17 * 17)
}

func (a *bro) drawCell(pn *termPane, f *termFace, x, y int, k cellKey) {
	if k.width == 0 {
		return // the right half of a wide character, drawn with it
	}
	w := max(1, k.width)
	r := image.Rect(pn.grid.Min.X+x*f.cw, pn.grid.Min.Y+y*f.ch, pn.grid.Min.X+(x+w)*f.cw, pn.grid.Min.Y+(y+1)*f.ch).Intersect(pn.grid)
	fg, bg := k.fg, k.bg
	if k.cursor != k.sel {
		fg, bg = bg, fg
	}
	if k.place != nil {
		kobo.CopyGray(a.C.Gray, r, a.gfxTile(k.place, f.cell()), image.Pt(int(k.ix)*f.cw, int(k.iy)*f.ch))
		return
	}
	a.C.Fill(r, bg)
	if k.content != "" && k.content != " " {
		var style uint8
		if k.attrs&uv.AttrBold != 0 {
			style |= glyphBold
		}
		if k.attrs&uv.AttrItalic != 0 {
			style |= glyphItalic
		}
		mask := f.glyph(k.content, style, w)
		a.C.Mask(r, fg, mask, image.Point{})
	}
	if k.ul != 0 {
		a.C.Fill(image.Rect(r.Min.X, r.Max.Y-3, r.Max.X, r.Max.Y-1), fg)
	}
	if k.attrs&uv.AttrStrikethrough != 0 {
		a.C.Fill(image.Rect(r.Min.X, r.Min.Y+f.ch/2, r.Max.X, r.Min.Y+f.ch/2+2), fg)
	}
}

// --- sessions ---

// termInit gets the terminals ready: the font, and the cells' size.
func (a *bro) termInit() {
	v := &a.tv
	v.fontPx = termFontDefault
	if px := a.mem.TermFont; px != 0 {
		v.fontPx = max(termFontMin, min(termFontMax, px))
	}
	a.termGeometry()
}

// endSession is the session title's button: it disconnects the session
// in focus (or kills its shell): its window is a launcher again.
func (a *bro) endSession() {
	t := a.term()
	if t == nil || t.state == termIdle {
		return
	}
	a.termClose(t)
	t.retries = 0
	a.termNote(t, "disconnected")
	a.drawPane(t)
}

// termFontBy changes the font size of the terminal in focus by d px;
// d 0: back to the default.
func (a *bro) termFontBy(d int) {
	if t := a.term(); t != nil {
		px := termFontDefault
		if d != 0 {
			px = a.face(t).px + d
		}
		a.setTermFont(t, px)
	}
}

// setTermFont changes t's font size, the other terminals' staying as
// they are; the remote gets the new cell count, as when its window is
// resized. New terminals take the size too.
func (a *bro) setTermFont(t *termSession, px int) {
	v := &a.tv
	px = max(termFontMin, min(termFontMax, px))
	if px != v.fontPx {
		v.fontPx, a.mem.TermFont = px, px
		a.mem.save()
	}
	if px == a.face(t).px {
		return
	}
	log.Printf("terminal font: %d px", px)
	t.fontPx = px
	if t == a.term() {
		v.sel = nil // its cells are other ones now
	}
	for size := range v.faces { // the sizes no terminal has any more
		if !slices.ContainsFunc(a.terms(), func(o *termSession) bool { return o.fontPx == size }) {
			delete(v.faces, size)
		}
	}
	a.paneGeometry(t)
	a.wm.changed = time.Now() // kept with the layout
	if !a.overlay() && !t.pane.p.Outer.Empty() {
		a.drawPane(t)
		a.MarkFlash(t.pane.p.Outer) // every glyph changed: clean, not ghosted
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
	t.state, t.target, t.title, t.kind = termConnecting, h, "", kindSSH
	t.gen++
	gen := t.gen
	a.termNote(t, "connecting to "+h.String()+"…")
	size := t.startSize()
	o := a.dialPrompts(t, gen, h)
	o.cols, o.rows, o.password = size.X, size.Y, t.password
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
				h.name = newHostName(a.tv.hosts, h)
				a.tv.hosts = append(a.tv.hosts, h)
				if err := a.saveHosts(); err != nil {
					a.termNote(t, "couldn't save the host: "+err.Error())
				} else {
					log.Printf("ssh: saved %s as %s", h, h.name)
				}
			}
			t.typed, t.input = false, ""
			a.termStart(t, s, gen)
		})
	}()
}

// dialPrompts are dialOpts for logging in to h whose questions go to
// t's page, while its connection is gen.
func (a *bro) dialPrompts(t *termSession, gen int, h sshHost) dialOpts {
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
	o := dialOpts{
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
	return o
}

// hostMenu is what can be done with saved host i: connect, mount its
// files (or unmount them), edit its line, forget it.
func (a *bro) hostMenu(t *termSession, i int) {
	h := a.tv.hosts[i]
	opts := []askOpt{{"connect", "c", func() { a.termConnect(t, h) }}}
	if m := a.mounted(h); m != nil {
		opts = append(opts, askOpt{"unmount its files", "u", func() { a.unmountHost(h) }})
	} else {
		opts = append(opts, askOpt{"mount its files…", "m", func() { a.mountAsk(t, h) }})
	}
	opts = append(opts,
		askOpt{"edit", "e", func() { a.hostEdit(t, h) }},
		askOpt{"forget", "f", func() {
			a.ask("Forget "+h.name+"?", h.line(), "forget", func() { a.hostSave(t, h, "") })
		}},
		askOpt{"cancel", "", nil})
	a.choose(h.name, h.String(), opts...)
}

// mountAsk asks how to mount h: reading only, or writing too.
func (a *bro) mountAsk(t *termSession, h sshHost) {
	a.choose("Mount "+h.name, "its home, at /mnt/hosts/"+hostDirName(h.name)+" in Alpine",
		askOpt{"read-only", "r", func() { a.mountHost(t, h, false, false) }},
		askOpt{"read-write", "w", func() { a.mountHost(t, h, true, false) }},
		askOpt{"cancel", "", nil})
}

// hostEdit puts h's line in the line, to change and save with Enter.
func (a *bro) hostEdit(t *termSession, h sshHost) {
	if t.state != termIdle {
		return
	}
	t.edit, t.input = &h, h.line()
	a.updateTerm(t)
}

// hostSave puts line in place of h's line, or takes h out if it's
// empty.
func (a *bro) hostSave(t *termSession, h sshHost, line string) {
	if line != "" {
		n, ok := parseSSHConfLine(line)
		switch {
		case !ok:
			a.termNote(t, "that's not a host: name  user@host[:port]  [via user@jump]  [command]")
			return
		case n.name != h.name && slices.ContainsFunc(a.tv.hosts, func(o sshHost) bool { return o.name == n.name }):
			a.termNote(t, "there's a host called "+n.name+" already")
			return
		}
		line = n.line()
	}
	if m := a.mounted(h); m != nil && line == "" {
		a.unmountHost(h)
	}
	i := slices.IndexFunc(a.tv.hosts, func(o sshHost) bool { return o.name == h.name })
	if i < 0 {
		a.termNote(t, h.name+" isn't saved any more")
		return
	}
	if line == "" {
		a.tv.hosts = slices.Delete(a.tv.hosts, i, i+1)
	} else {
		a.tv.hosts[i], _ = parseSSHConfLine(line)
	}
	if err := a.saveHosts(); err != nil {
		a.termNote(t, "couldn't save: "+err.Error())
		return
	}
	if line == "" {
		a.termNote(t, h.name+" forgotten")
	} else {
		a.termNote(t, "saved: "+line)
	}
}

// redrawHosts redraws the saved hosts where they show: a mount came or
// went.
func (a *bro) redrawHosts() {
	for _, t := range a.terms() {
		if !t.connected() && a.onScreen(t) {
			t.pane.shownConnect = nil
			a.updateTerm(t)
		}
	}
}

// termStartShell runs a shell in t: Alpine's, in a jail, or the Kobo's
// own /bin/sh.
func (a *bro) termStartShell(t *termSession, alpine bool) {
	if alpine {
		a.termStartAlpine(t, "")
		return
	}
	a.termRun(t, false, func(size image.Point) (*localShell, error) { return startShell(size.X, size.Y) })
}

// termStartAlpine runs command (a login shell, for "") in Alpine, in t.
func (a *bro) termStartAlpine(t *termSession, command string) {
	t.exec = command
	a.termRun(t, true, func(size image.Point) (*localShell, error) { return startAlpine(size.X, size.Y, command) })
}

func (a *bro) termRun(t *termSession, alpine bool, start func(image.Point) (*localShell, error)) {
	if t.state != termIdle {
		return
	}
	t.notes, t.title, t.kind, t.alpine, t.input, t.edit = nil, "", kindShell, alpine, "", nil
	s, err := start(t.startSize())
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
	defer a.grabTyping() // typing into another device, maybe
	t.emu = vt.NewSafeEmulator(cols, rows)
	t.mu.Lock()
	t.gfx = kittyGfx{cell: a.face(t).cell()}
	t.mu.Unlock()
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
	t.kittyRegister(t.emu)
	t.gfxRegister(t.emu)
	t.emu.RegisterOscHandler(52, a.osc52(t))
	t.emu.RegisterOscHandler(9, a.oscNotify(t))
	t.emu.RegisterOscHandler(777, a.oscNotify(t))
	f := a.face(t)
	back.resize(cols, rows, image.Pt(cols*f.cw, rows*f.ch))
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
			// Off screen, a trickle (a player's position, ten times a
			// second) waits and comes in batches: each read wakes us,
			// and the wakeups cost more than the parsing. A flood is
			// read at once, so whatever writes it never waits on us.
			if n < len(buf)/8 && !t.seen.Load() {
				time.Sleep(termUnseen)
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
	defer a.grabTyping() // done typing into another device, maybe
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
	for _, t := range a.terms() {
		a.termClose(t)
	}
}

// termTick starts the reconnections that are due, redraws from the
// emulator now and then, and gives the screen a clean grayscale refresh
// once output settles after fast updates.
func (a *bro) termTick() {
	ts, v := a.terms(), &a.tv
	for _, t := range ts {
		t.seen.Store(a.showing(t))
		if t.state == termWaiting && time.Now().After(t.retryAt) {
			a.termConnect(t, t.target)
		}
	}
	if a.Calibrating() {
		return
	}
	if slices.ContainsFunc(ts, func(t *termSession) bool { return t.state == termWaiting }) {
		a.drawTermStatus() // the countdown
	}
	if a.GaugeUp() || a.overlay() {
		return // something is up over the terminal: drawing under it would be undone when it goes
	}
	// The panel does one update at a time, so a pane whose output never
	// stops (a player's position, a clock) would keep it busy, and the
	// keys typed into another behind it: the panes not in focus show
	// their output once a second, and not while the one in focus is
	// being typed into.
	focus := a.term()
	typing := focus != nil && time.Since(focus.keyAt) < termTyping
	if time.Since(v.drawnAt) >= termFrame {
		for _, t := range ts {
			if !t.connected() || !t.dirty.Load() || !a.showing(t) {
				continue
			}
			if t != focus && (typing || time.Since(t.pane.drawnAt) < termBackFrame) {
				continue
			}
			t.pane.drawnAt = time.Now()
			a.renderTerm(t)
		}
	}
	if a.Blitting() {
		return
	}
	// Clean up after the fast waveform once a pane's output pauses, or
	// every few seconds if it doesn't (a spinner never stops while its app
	// works): each pane its own, as one area for two would cover what's
	// between them.
	for _, t := range ts {
		pn := &t.pane
		if !pn.fastArea.Empty() && (time.Since(pn.drawnAt) > 1200*time.Millisecond || time.Since(pn.fastAt) > 4*time.Second) {
			a.Mark(pn.fastArea, true)
			pn.fastArea = image.Rectangle{}
			break // one a tick: each is an update of its own
		}
	}
	// Grayscale updates leave ghosts too, slowly: after a few screenfuls
	// of change, flash the terminal clean once it settles (a clock or a
	// spinner may still tick), as e-readers do every few pages.
	if v.churn > 4*kobo.Area(a.wm.area) && time.Since(v.bigAt) > 2500*time.Millisecond {
		v.churn = 0
		for _, t := range ts {
			t.pane.fastArea = image.Rectangle{}
		}
		a.MarkFlash(a.wm.area)
	}
}

// --- input ---

// termKey takes a key event while a terminal tab is up. It returns false
// for keys the rest of the app should handle (tab switching, quitting)
// when there's no session to send them to.
func (a *bro) termKey(code uint16, value int32) bool {
	t, v := a.term(), &a.tv
	m := &v.mods
	if a.termMod(code, value) {
		return true
	}
	if value == 0 {
		t.kittySend(code, value, *m) // released: only said the kitty way
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
		case t.edit != nil && code == kobo.KeyEsc:
			t.edit, t.input = nil, ""
		case t.edit != nil && code == kobo.KeyEnter:
			h, line := *t.edit, strings.TrimSpace(t.input)
			t.edit, t.input = nil, ""
			if line == "" {
				break
			}
			a.hostSave(t, h, line)
		case code == kobo.KeyEsc:
			a.closeWin(t)
			return true
		case code == kobo.KeyEnter:
			in := strings.TrimSpace(t.input)
			if in == "" {
				a.termStartShell(t, true)
				return true
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
		case printable && t.input == "" && t.edit == nil && ch >= "1" && ch <= "9":
			switch i := int(ch[0] - '1'); {
			case i < 2:
				a.termStartShell(t, i == 0)
				return true
			case i-2 < len(v.hosts):
				a.hostMenu(t, i-2)
			case i-2-len(v.hosts) < len(v.nearby.peers):
				a.typeInto(t, v.nearby.peers[i-2-len(v.hosts)])
				return true
			}
		case printable && !m.ctrl:
			t.input += ch
		default:
			return false // Esc, F-keys: the app's
		}
		a.updateTerm(t)
		return true
	}

	if t.kittySend(code, value, *m) {
		t.keyAt = time.Now()
	} else if s, ok := xtermKey(code, *m); ok {
		io.WriteString(t.emu.InputPipe(), s)
		t.keyAt = time.Now()
	} else if k, ok := termKeyEvent(code, *m); ok {
		t.emu.SendKey(k)
		t.keyAt = time.Now()
	}
	return true
}

// xtermKey is an arrow, Home, End, PgUp, PgDn, Ins, Del or an F-key
// with modifiers held, as xterm says it (CSI 1;5A: Ctrl+Up), if it's
// one: vt sends nothing for those, and tmux's prefix, Ctrl+arrows
// (resizing panes) never came.
func xtermKey(code uint16, m termMods) (string, bool) {
	mods := 1
	for _, b := range []struct {
		on  bool
		bit int
	}{{m.shift, 1}, {m.alt, 2}, {m.ctrl, 4}, {m.meta, 8}} {
		if b.on {
			mods += b.bit
		}
	}
	fn, ok := kittyFuncs[code]
	if !ok || mods == 1 {
		return "", false
	}
	if code == 61 { // F3: kitty's CSI 13 ~ is xterm's CSI 1;m R
		fn = kittyFunc{1, 'R'}
	}
	if fn.n == 1 {
		return fmt.Sprintf("\x1b[1;%d%c", mods, fn.final), true
	}
	return fmt.Sprintf("\x1b[%d;%d~", fn.n, mods), true
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
	keyUp        = 103
	keyPgUp      = 104
	keyLeft      = 105
	keyRight     = 106
	keyHome      = 102
	keyEnd       = 107
	keyDown      = 108
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
	t := a.term()
	if t == nil || !p.In(t.pane.grid) {
		return 0, 0, false
	}
	f := a.face(t)
	return (p.X - t.pane.grid.Min.X) / f.cw, (p.Y - t.pane.grid.Min.Y) / f.ch, true
}

// termTap handles a tap on a terminal: its button, a choice on the
// launcher, or a click for the remote app.
func (a *bro) termTap(p image.Point) {
	t := a.term()
	if t == nil {
		return
	}
	if kobo.Hit(p, a.termPanel(t).Button) {
		a.endSession()
		return
	}
	if !t.connected() {
		for _, k := range t.pane.picks {
			if p.In(k.r) {
				k.do()
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
	t := a.term()
	x, y, ok := a.termCell(from)
	if t == nil || !t.connected() || !ok {
		return false
	}
	dy := to.Y - from.Y
	steps := min(20, kobo.Abs(dy)/a.face(t).ch)
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
