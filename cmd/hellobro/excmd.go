package main

// ":" commands, vim's way: in a book or a notebook, ":" opens a line at
// the window's foot. A command and Enter runs it; Esc, or Backspace on
// nothing, leaves it. A number alone goes to that page ("40%" that far
// in), "/word" looks for it. A name can be cut short while it's the
// only one so begun ("ex" is export), and the line shows the commands
// what's typed could be. The keyboard on the screen shows for it, if
// there's no other.

import (
	"fmt"
	"image"
	"slices"
	"strings"
	"unicode"

	"hellobro/ink"
	"hellobro/ink/inkkobo"
	"hellobro/ink/text"
	"hellobro/kobo"
)

// exLine is a ":" command being typed.
type exLine struct{ text string }

// exCmd is a ":" command: its names (the first the one shown, then the
// short ones), what it takes, as the line hints it, what it's for (in
// :help), and what it does (nil: it's :help).
type exCmd struct {
	names string
	arg   string
	help  string
	run   func(arg string) error
}

// exFind is the command line asks for, and what's after its name.
func exFind(cmds []exCmd, line string) (*exCmd, string, error) {
	name, arg, _ := exSplit(line)
	var begun []*exCmd
	for i := range cmds {
		ns := strings.Fields(cmds[i].names)
		if slices.Contains(ns, name) {
			return &cmds[i], arg, nil
		}
		if strings.HasPrefix(ns[0], name) {
			begun = append(begun, &cmds[i])
		}
	}
	switch {
	case name == "":
		return nil, "", nil
	case len(begun) == 1:
		return begun[0], arg, nil
	case len(begun) > 1:
		var ns []string
		for _, c := range begun {
			ns = append(ns, strings.Fields(c.names)[0])
		}
		return nil, "", fmt.Errorf(":%s is %s?", name, strings.Join(ns, ", "))
	}
	return nil, "", fmt.Errorf("no :%s (:help says what there is)", name)
}

// exSplit is a line's command and what's after it, and whether there's
// more than the name: "/word" is "/", a number "page".
func exSplit(line string) (name, arg string, more bool) {
	line = strings.TrimLeft(line, " ")
	name, arg, more = strings.Cut(line, " ")
	switch {
	case strings.HasPrefix(line, "/"):
		name, arg, more = "/", line[1:], len(line) > 1
	case line != "" && strings.IndexFunc(strings.TrimSuffix(line, "%"), func(r rune) bool { return !unicode.IsDigit(r) }) < 0:
		name, arg, more = "page", line, true
	}
	return name, strings.TrimSpace(arg), more
}

// exHint is the commands what's typed could be, with what they take:
// the one named, once there's more than its name.
func exHint(cmds []exCmd, text string) string {
	name, _, typed := exSplit(text)
	var hs []string
	for _, c := range cmds {
		ns := strings.Fields(c.names)
		if exact := slices.Contains(ns, name); !exact && (typed || !strings.HasPrefix(ns[0], name)) {
			continue
		}
		h := ns[0]
		if c.arg != "" {
			h += " " + c.arg
		}
		hs = append(hs, h)
	}
	return strings.Join(hs, " · ")
}

// exRun runs the line typed; what went wrong is noted. again opens the
// line again with text in it, for :help's commands that take something.
func (a *bro) exRun(cmds []exCmd, line string, again func(text string)) {
	c, arg, err := exFind(cmds, line)
	switch {
	case err == nil && c != nil && c.run == nil:
		a.exHelp(cmds, again)
	case err == nil && c != nil:
		err = c.run(arg)
	}
	if err != nil {
		a.Notify(err.Error())
	}
}

// exHelp shows the commands in a menu: typing narrows it, and the one
// picked runs, or, if it takes something, is typed on the line to go on.
func (a *bro) exHelp(cmds []exCmd, again func(text string)) {
	es := []pickEntry{{title: ":N   :N%", detail: "a page; that far in"}}
	for _, c := range cmds {
		if c.run == nil {
			continue
		}
		ns := strings.Fields(c.names)
		title, detail := ":"+ns[0], c.help
		if c.arg != "" {
			title += " " + c.arg
		}
		if len(ns) > 1 {
			detail += "   (:" + strings.Join(ns[1:], " :") + ")"
		}
		do := func() { again(ns[0] + " ") }
		if c.arg == "" {
			do = func() { a.exRun(cmds, ns[0], again) }
		}
		es = append(es, pickEntry{title: title, detail: detail, do: do})
	}
	es[0].do = func() { again("") }
	a.openMenu(":help", "type to narrow · Enter: it, or it on the line · Esc closes", es, nil)
}

// exStrip is where the line goes in body: along its foot.
func (a *bro) exStrip(body image.Rectangle) image.Rectangle {
	return image.Rect(body.Min.X, body.Max.Y-a.libRowH(), body.Max.X, body.Max.Y)
}

// drawEx draws the line in r, in ink: what's typed, a caret, and what
// it could be. It's painted whole each time: a strip, and the page
// under it may have been drawn since.
func (a *bro) drawEx(r image.Rectangle, e *exLine, cmds []exCmd) {
	if a.exUI == nil {
		a.exUI = &exUI{a: a}
	}
	u := a.exUI
	u.text, u.hint = e.text, exHint(cmds, e.text)
	a.inkPaint(&u.pane, r, u.Layout)
	a.Mark(r, true)
}

type exUI struct {
	a          *bro
	pane       *inkkobo.Pane
	text, hint string
}

func (u *exUI) Layout(gtx ink.Ctx) ink.Dims {
	th := u.a.theme()
	size := gtx.Constraints.Max
	ink.Fill(gtx.Ops, image.Rectangle{Max: size}, th.Bg)
	ink.Fill(gtx.Ops, image.Rect(0, 0, size.X, max(gtx.Dp(1.5), 2)), th.Fg)
	f := th.Face(gtx, th.TextSize, text.Regular)
	small := th.Face(gtx, th.TextSize*0.85, text.Regular)
	pad := gtx.Dp(th.Pad * 1.5)
	base := (size.Y-text.Height(f))/2 + text.Ascent(f)
	s := text.Fit(f, ":"+u.text, size.X*3/5-pad)
	ink.TextAt(gtx.Ops, f, pad, base, s, th.Fg)
	cx := pad + text.Width(f, s) + gtx.Dp(1)
	ink.Fill(gtx.Ops, image.Rect(cx, size.Y/4, cx+max(gtx.Dp(2), 3), size.Y-size.Y/4), th.Fg) // the caret
	x := max(cx+pad*2, min(size.X*3/5, cx+pad*4))
	ink.TextAt(gtx.Ops, small, x, base, text.Fit(small, u.hint, size.X-pad-x), th.Quiet)
	return ink.Dims{Size: size}
}

// exKey types a key into the line: done when it's to run (ok) or be
// left (Esc, Backspace on nothing).
func exKey(e *exLine, code uint16, ch string) (done, ok bool) {
	if code == kobo.KeyBackspace && e.text == "" {
		return true, false
	}
	return nameKey(&e.text, code, ch)
}

// exNumber is arg as a page's number (from 1), or how far in ("40%").
func exNumber(arg string) (n int, pct bool, err error) {
	s, pct := strings.CutSuffix(arg, "%")
	if _, err := fmt.Sscan(s, &n); err != nil || n < 0 || pct && n > 100 {
		return 0, false, fmt.Errorf("a page, please: :12, or :40%%")
	}
	return n, pct, nil
}
