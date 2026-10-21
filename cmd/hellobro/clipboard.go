package main

// The clipboard, for the terminals, the switcher and the prompts. It's a
// file, clipboard, next to hellobro, so a computer gets at it too (SFTP,
// or ssh … cat). In a terminal the pen drawn across the text selects it
// (or a finger held still a moment, then moved), and it's copied as it
// lifts; Ctrl+Shift+V pastes. A remote program copying (OSC 52: tmux,
// helix, Claude Code) copies into it as well, but can't read it back.
// The Kobo's SSH key can be copied, or typed in as it is.

import (
	"encoding/base64"
	"fmt"
	"image"
	"log"
	"os"
	"strings"
	"time"

	"hellobro/kobo"
)

const (
	clipTxt   = "clipboard"
	ownSelect = 2                      // a gesture that is a selection
	selHold   = 600 * time.Millisecond // a finger held this long, then moved, selects
	selStill  = 12                     // px it may wander meanwhile
)

func readClip() string {
	b, _ := os.ReadFile(kobo.AppFile(clipTxt))
	return string(b)
}

// copyText puts text in the clipboard; what says what it was, for the
// header ("" for a bit of it).
func (a *bro) copyText(text, what string) {
	if strings.TrimSpace(text) == "" {
		a.Notify("nothing to copy")
		return
	}
	if err := os.WriteFile(kobo.AppFile(clipTxt), []byte(text), 0o600); err != nil {
		log.Printf("clipboard: %v", err)
		a.Notify("couldn't copy: " + err.Error())
		return
	}
	if what == "" {
		what = clipSummary(text)
	}
	log.Printf("clipboard: copied %d bytes", len(text))
	a.Notify("copied " + what)
}

// clipSummary is a short word on text: "3 lines", `"ssh-ed25519 AAAA…"`.
func clipSummary(text string) string {
	if n := strings.Count(strings.TrimRight(text, "\n"), "\n") + 1; n > 1 {
		return fmt.Sprintf("%d lines", n)
	}
	if r := []rune(strings.TrimSpace(text)); len(r) > 24 {
		return "“" + string(r[:20]) + "…”"
	}
	return "“" + strings.TrimSpace(text) + "”"
}

// paste puts text where the keys go now: all of it into a terminal, its
// first line into a prompt, the switcher's query, the ssh> line.
func (a *bro) paste(text, what string) {
	if text == "" {
		a.Notify("the clipboard is empty")
		return
	}
	line, _, _ := strings.Cut(strings.TrimLeft(text, "\r\n"), "\n")
	line = strings.TrimRight(line, "\r")
	if what == "" {
		what = clipSummary(text)
	}
	switch t := a.term(); {
	case a.picker != nil:
		a.picker.query += line
		a.filterPicker()
		a.drawPicker()
	case a.overlay():
		return
	case a.Tab == tabRadio && a.rs.prompt != nil:
		a.rs.prompt.answer += line
		a.drawRadioPanel(a.rs.prompt.panel)
	case !isTermTab(a.Tab) || t == nil:
		a.Notify("nothing to paste into here")
		return
	case t.prompt != nil:
		t.prompt.answer += line
		a.updateTerm(t)
	case t.connected():
		a.clearSel()
		// As xterm does: lines end in CR, as typed. Off the loop, as a
		// big paste waits on the connection.
		emu, s := t.emu, strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\r"), "\n", "\r")
		go emu.Paste(s)
	case t.state == termIdle && t.kind == kindSSH:
		t.input += line
		a.updateTerm(t)
	default:
		return
	}
	a.Notify("pasted " + what)
}

// publicKey is the Kobo's public SSH key, made if there's none yet.
func publicKey() (string, error) {
	s, err := clientKey()
	if err != nil {
		return "", err
	}
	return authorizedKey(s), nil
}

func (a *bro) copyKoboKey() {
	k, err := publicKey()
	if err != nil {
		a.Notify("no key: " + err.Error())
		return
	}
	a.copyText(k+"\n", "the Kobo's SSH key")
}

func (a *bro) pasteKoboKey() {
	k, err := publicKey()
	if err != nil {
		a.Notify("no key: " + err.Error())
		return
	}
	a.paste(k, "the Kobo's SSH key")
}

// osc52 is a remote program copying: OSC 52 ; c ; base64. Asking what's
// in the clipboard ("?") gets nothing: it's not the remote's to read.
func (a *bro) osc52(t *termSession) func([]byte) bool {
	return func(data []byte) bool {
		parts := strings.SplitN(string(data), ";", 3)
		if len(parts) != 3 || parts[2] == "?" {
			return true
		}
		b, err := base64.StdEncoding.DecodeString(parts[2])
		if err != nil {
			return true
		}
		go a.Do(func() { a.copyText(string(b), "") }) // not under the emulator's lock: the loop may want it
		return true
	}
}

// --- selecting in a terminal ---

// termSel is a selection on the grid, or a gesture that may become one.
type termSel struct {
	g        *kobo.Gesture
	early    float64 // how far the finger got in its first moment
	on       bool    // selecting: the gesture is ours
	from, to image.Point
}

// span is the selection's ends in reading order.
func (s *termSel) span() (image.Point, image.Point) {
	a, b := s.from, s.to
	if b.Y < a.Y || b.Y == a.Y && b.X < a.X {
		a, b = b, a
	}
	return a, b
}

func (s *termSel) has(x, y int) bool {
	if s == nil || !s.on {
		return false
	}
	a, b := s.span()
	return (y > a.Y || y == a.Y && x >= a.X) && (y < b.Y || y == b.Y && x <= b.X)
}

// selStart notes a contact on a connected terminal's cells: it may become
// a selection.
func (a *bro) selStart(p image.Point, g *kobo.Gesture) {
	t := a.term()
	x, y, ok := a.termCell(p)
	if t == nil || !t.connected() || !ok || a.overlay() {
		return
	}
	a.clearSel()
	a.tv.sel = &termSel{g: g, from: image.Pt(x, y), to: image.Pt(x, y)}
}

// selMove follows it: the pen selects as soon as it moves, a finger once
// it has been held still a moment.
func (a *bro) selMove(p image.Point, g *kobo.Gesture) {
	s := a.tv.sel
	if !s.on {
		switch {
		case g.Owner != 0:
			a.tv.sel = nil
			return
		case g.Pen && g.Moved > selStill:
		case g.Pen:
			return
		case time.Since(g.At) < selHold:
			s.early = g.Moved
			return
		case s.early > selStill: // it was moving from the start: a scroll
			a.tv.sel = nil
			return
		}
		s.on, g.Owner = true, ownSelect
		log.Print("terminal: selecting")
	}
	v := &a.tv
	q := p.Sub(v.grid.Min)
	s.to = image.Pt(max(0, min(v.cols-1, q.X/max(1, v.cw))), max(0, min(v.rows-1, q.Y/max(1, v.ch))))
	if t := a.term(); t != nil && t.connected() {
		a.renderTerm(t)
	}
}

// selLift copies the selection, which stays shown till a key or a tap.
func (a *bro) selLift(g *kobo.Gesture) bool {
	s := a.tv.sel
	if s == nil || s.g != g {
		return false
	}
	if !s.on {
		a.tv.sel = nil
		return false
	}
	if t := a.term(); t != nil && t.connected() {
		from, to := s.span()
		a.copyText(cellText(t, a.tv.cols, from, to), "")
	}
	return true
}

func (a *bro) clearSel() {
	s := a.tv.sel
	a.tv.sel = nil
	if s != nil && s.on {
		if t := a.term(); t != nil && t.connected() {
			a.renderTerm(t)
		}
	}
}

// cellText is the text from cell from to cell to, in reading order. A
// row filled to its end runs on into the next (a long line, wrapped)
// rather than ending there.
func cellText(t *termSession, cols int, from, to image.Point) string {
	var b strings.Builder
	for y := from.Y; y <= to.Y; y++ {
		x0, x1 := 0, cols-1
		if y == from.Y {
			x0 = from.X
		}
		if y == to.Y {
			x1 = to.X
		}
		var row strings.Builder
		for x := x0; x <= x1; x++ {
			c := t.emu.CellAt(x, y)
			switch {
			case c == nil || c.Content == "":
				row.WriteByte(' ')
			case c.Width == 0: // a wide character's right half
			default:
				row.WriteString(c.Content)
			}
		}
		line := row.String()
		b.WriteString(strings.TrimRight(line, " "))
		full := x1 == cols-1 && !strings.HasSuffix(line, " ")
		if y < to.Y && !full {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// screenText is all the terminal shows, the blank rows at its end left off.
func (a *bro) screenText(t *termSession) string {
	v := &a.tv
	return strings.TrimRight(cellText(t, v.cols, image.Pt(0, 0), image.Pt(v.cols-1, v.rows-1)), "\n ")
}

// copySel copies the selection again, or says how to make one.
func (a *bro) copySel() {
	s, t := a.tv.sel, a.term()
	if s == nil || !s.on || t == nil || !t.connected() {
		a.Notify("select with the pen first (or a finger held still, then moved)")
		return
	}
	from, to := s.span()
	a.copyText(cellText(t, a.tv.cols, from, to), "")
}
