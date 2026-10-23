package main

// A terminal calls out: the bell (BEL), or a message as programs send a
// desktop one (OSC 9, iTerm2's; OSC 777, urxvt's: "notify;title;body").
// The light comes on a moment if it's off; the session's title strip
// blinks if it shows, and if not, a note says so in the corner. What of
// it happens is up to Settings. The notify command (keys.conf, the IPC:
// hellobro-msg notify TEXT) puts up a note too.

import (
	"log"
	"strconv"
	"strings"
	"time"
)

const (
	notifyQuiet = 2 * time.Second // a bell this soon after a key is the shell's answer to it, not news
	notifyEvery = 5 * time.Second // and one session calls out at most this often
)

// oscNotify reads OSC 9 and 777 notifications.
func (a *bro) oscNotify(t *termSession) func([]byte) bool {
	return func(data []byte) bool {
		if msg, ok := parseNotify(string(data)); ok {
			go a.Do(func() { a.termNotify(t, msg) }) // not under the emulator's lock: the loop may want it
		}
		return true
	}
}

// parseNotify is an OSC 9 or 777 sequence's message, from its data (the
// number first): false if it isn't one, as ConEmu's progress (9;4;…).
func parseNotify(data string) (string, bool) {
	cmd, rest, _ := strings.Cut(data, ";")
	switch cmd {
	case "9":
		if strings.HasPrefix(rest, "4;") || rest == "4" {
			return "", false
		}
		return strings.TrimSpace(rest), rest != ""
	case "777":
		parts := strings.SplitN(rest, ";", 3)
		if parts[0] != "notify" || len(parts) < 2 {
			return "", false
		}
		title, body := parts[1], ""
		if len(parts) == 3 {
			body = parts[2]
		}
		switch {
		case title == "":
			return body, body != ""
		case body == "":
			return title, true
		}
		return title + ": " + body, true
	}
	return "", false
}

// termNotify calls out for t: msg, or "" for the bell.
func (a *bro) termNotify(t *termSession, msg string) {
	switch a.setting("notify") {
	case "off":
		return
	case "messages":
		if msg == "" {
			return
		}
	}
	if msg == "" && time.Since(t.keyAt) < notifyQuiet || time.Since(t.notifyAt) < notifyEvery {
		return
	}
	t.notifyAt = time.Now()
	name := t.name()
	if msg == "" {
		msg = "bell"
	}
	log.Printf("terminal: %s: %s", name, msg)
	if s, err := strconv.Atoi(a.setting("notify-light")); err == nil {
		a.FlashLight(max(2*lightStep, a.mem.LightOn), time.Duration(s)*time.Second)
	}
	if n, err := strconv.Atoi(a.setting("notify-blink")); err == nil && a.onScreen(t) && !t.pane.p.Title.Empty() {
		a.Blink(t.pane.p.Title, n)
		return
	}
	if s, err := strconv.Atoi(a.setting("notify-note")); err == nil && !a.onScreen(t) {
		a.Toast(name+": "+msg, time.Duration(s)*time.Second)
	}
}

// notify calls out with msg, from no terminal (the notify command): the
// light as for a terminal's message, and a note in the corner.
func (a *bro) notify(msg string) {
	if a.setting("notify") == "off" {
		return
	}
	if r := []rune(msg); len(r) > notifyMax {
		msg = string(r[:notifyMax-1]) + "…"
	}
	log.Printf("notify: %s", msg)
	if s, err := strconv.Atoi(a.setting("notify-light")); err == nil {
		a.FlashLight(max(2*lightStep, a.mem.LightOn), time.Duration(s)*time.Second)
	}
	if s, err := strconv.Atoi(a.setting("notify-note")); err == nil {
		a.Toast(msg, time.Duration(s)*time.Second)
	}
}

const notifyMax = 200 // runes of a notify command's message kept
