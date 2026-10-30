package main

// The on-screen keyboard, for when there's no other: a laptop's rows
// under the windows, with the arrows in a cross at the right, and a
// strip either side for what's left (Home, End, the pages, Esc, Del, Tab
// and the modifiers). Its keys go the way a keyboard's do (a.Key), so
// the commands, the terminals and kitty's keys have them alike. A
// modifier holds till the next key is up; tapped again, it lets go. A
// key held goes on, as a keyboard's does. A key at an edge, where the
// swipes start, waits: lifted it's a tap, held it's pressed, and moved
// it's the swipe (the menu's, a workspace's).
//
// It's Settings' "On-screen keyboard": off; fields, only while a text
// field is typed in (a name, the find line, a note's text); auto, which
// shows it a while after the last keyboard went and hides it as one
// comes; or on. Two fingers swiped up from the screen's foot show it
// whatever the setting, and down onto it hide it, as the menu's
// Keyboard does: till a field comes, or a keyboard. Its key left of the
// space bar types in the next of the languages set (Settings, Languages
// to type in), as Ctrl+Alt+Space does, the letters showing it.

import (
	"image"
	"log"
	"strings"
	"time"
	"unicode/utf8"

	"hellobro/kobo"
)

const (
	ownKey     = 6 // a finger on a key
	vkUnits    = 15
	vkPad      = 3 // between keys
	vkDelay    = 500 * time.Millisecond
	vkEvery    = 80 * time.Millisecond
	vkAutoWait = 10 * time.Second // a keyboard's naps and blips are shorter
	vkSlop     = 30               // px a finger on a key at an edge may wander before it's a swipe
	vkFoot     = 100              // px up from the screen's foot where two fingers swiped up show it
	vkLang     = 0xfff0           // the language's key: no key's code
)

type vkey struct {
	r     image.Rectangle
	code  uint16
	label string // a word, else the key's character
}

type vkHeld struct {
	k    vkey
	gen  int // the layout it was pressed in
	at   time.Time
	sent bool // pressed: not waiting at an edge
}

type vkbdState struct {
	on      bool
	keys    []vkey
	top     int // the rows'
	gen     int // the layouts so far
	held    map[*kobo.Gesture]vkHeld
	latched map[uint16]bool // modifiers down till the next key
	next    time.Time       // the held key's next repeat
	since   time.Time       // the keyboard went (or we started): auto waits from it
	ask     bool            // a name's being typed (notelist.go): it shows if there's no keyboard
	said    int             // shown (1) or hidden (-1) by a swipe or the menu, whatever the setting
	lang    int             // the language typed in, of the setting's
}

// vkItem is a key in a row, w units wide; code 0 is a gap.
type vkItem struct {
	w     float64
	code  uint16
	label string
}

func vkChars(first uint16, n int) []vkItem {
	var is []vkItem
	for i := range n {
		is = append(is, vkItem{1, first + uint16(i), ""})
	}
	return is
}

func vkRow(parts ...[]vkItem) []vkItem {
	var r []vkItem
	for _, p := range parts {
		r = append(r, p...)
	}
	return r
}

// The rows: no Tab, Shift, Ctrl or the like, they're in the strips.
var vkRows = [][]vkItem{
	vkRow([]vkItem{{1, 41, ""}}, vkChars(2, 12), []vkItem{{2, kobo.KeyBackspace, "Backspace"}}),
	vkRow([]vkItem{{1.5, 0, ""}}, vkChars(16, 12), []vkItem{{1.5, 43, ""}}),
	vkRow([]vkItem{{1.75, 0, ""}}, vkChars(30, 11), []vkItem{{2.25, kobo.KeyEnter, "Enter"}}),
	vkRow([]vkItem{{2.25, 0, ""}}, vkChars(44, 10), []vkItem{{0.75, 0, ""}, {1, keyUp, "↑"}, {1, 0, ""}}),
	{{0.25, 0, ""}, {2, vkLang, ""}, {0.25, 0, ""}, {9, kobo.KeySpace, " "}, {0.5, 0, ""}, {1, keyLeft, "←"}, {1, keyDown, "↓"}, {1, keyRight, "→"}},
}

// The strips, top to bottom; they stand on the rows.
var (
	vkLeft  = []vkItem{{1, keyHome, "Home"}, {1, keyPgUp, "PgUp"}, {1, keyPgDn, "PgDn"}, {1, keyEnd, "End"}, {1, kobo.KeyTab, "Tab"}, {1, kobo.KeyLeftShift, "Shift"}}
	vkRight = []vkItem{{1, kobo.KeyEsc, "Esc"}, {1, keyDelete, "Del"}, {1, kobo.KeyTab, "Tab"}, {1, keyLeftMeta, "Mod"}, {1, keyLeftAlt, "Alt"}, {1, kobo.KeyLeftCtrl, "Ctrl"}, {1, kobo.KeyLeftShift, "Shift"}}
)

// vkLayout lays the keyboard out on page: its keys, and what's left for
// the windows.
func vkLayout(page image.Rectangle) ([]vkey, image.Rectangle) {
	u := float64(page.Dx()) / vkUnits
	h := min(max(min(page.Dx(), page.Dy())*4/5/vkUnits, 56), 84) // as tall either way up
	sw := min(int(u), 100)
	top := page.Max.Y - len(vkRows)*h
	var keys []vkey
	for i, row := range vkRows {
		y, x := top+i*h, 0.0
		for _, it := range row {
			x0 := page.Min.X + int(x*u+0.5)
			x += it.w
			if it.code != 0 {
				keys = append(keys, vkey{image.Rect(x0, y, page.Min.X+int(x*u+0.5), y+h), it.code, it.label})
			}
		}
	}
	for side, strip := range [][]vkItem{vkLeft, vkRight} {
		x0 := page.Min.X
		if side == 1 {
			x0 = page.Max.X - sw
		}
		kh := min(h, (top-page.Min.Y)/len(strip))
		for i, it := range strip {
			y := top - (len(strip)-i)*kh
			keys = append(keys, vkey{image.Rect(x0, y, x0+sw, y+kh), it.code, it.label})
		}
	}
	return keys, image.Rect(page.Min.X+sw, page.Min.Y, page.Max.X-sw, top)
}

// vkPlace lays it out on page, if it shows, and says what's left. The
// keys held let go: a rotate forgets the fingers, and their lifts never
// come, so a key would go on repeating.
func (a *bro) vkPlace(page image.Rectangle) image.Rectangle {
	v := &a.vk
	a.vkRelease()
	v.gen++
	v.keys = nil
	if !v.on {
		return page
	}
	keys, rest := vkLayout(page)
	v.keys, v.top = keys, rest.Max.Y
	return rest
}

// screenFree is the screen above the rows, for a question or a menu
// (they're narrower than what's between the strips).
func (a *bro) screenFree() image.Rectangle {
	b := a.C.Bounds()
	if a.vk.on {
		b.Max.Y = a.vk.top
	}
	return b
}

func (a *bro) vkAt(p image.Point) *vkey {
	for i := range a.vk.keys {
		if p.In(a.vk.keys[i].r) {
			return &a.vk.keys[i]
		}
	}
	return nil
}

// vkLit says k shows pressed: held, or a modifier holding.
func (a *bro) vkLit(k vkey) bool {
	for _, h := range a.vk.held {
		if h.gen == a.vk.gen && h.k.r == k.r {
			return true
		}
	}
	return a.vk.latched[k.code]
}

func (a *bro) drawVkey(k vkey) {
	c, r := a.C, k.r.Inset(vkPad)
	c.Fill(k.r, kobo.White)
	fg := uint8(kobo.Black)
	if a.vkLit(k) {
		c.Fill(r, kobo.Black)
		fg = kobo.White
	} else {
		c.Frame(r, 2, kobo.Black)
	}
	f, label := a.F.Panel, k.label
	if k.code == vkLang {
		label = strings.ToUpper(kobo.Typing())
	} else if label == "" {
		label, _ = kobo.KeyChar(k.code, a.vk.latched[kobo.KeyLeftShift], a.tv.mods.caps)
		label = kobo.Retype(label)
	} else if utf8.RuneCountInString(label) > 1 { // a word; an arrow's big
		f = a.F.Small
	}
	c.Text(f, r, label, fg, kobo.AlignCenter)
}

// drawVkbd draws the keys that only shows, or all.
func (a *bro) drawVkbd(only func(vkey) bool) {
	if a.Locked() {
		return
	}
	for _, k := range a.vk.keys {
		if only == nil || only(k) {
			a.drawVkey(k)
			a.Mark(k.r, false) // black and white: the fast update
		}
	}
}

// vkTouch takes a finger down on a key.
func (a *bro) vkTouch(p image.Point, g *kobo.Gesture) bool {
	v := &a.vk
	k := a.vkAt(p)
	if k == nil || a.Locked() {
		return false
	}
	g.Owner = ownKey
	v.held[g] = vkHeld{k: *k, gen: v.gen, at: time.Now()}
	a.drawVkbd(func(o vkey) bool { return o.r == k.r })
	if edge := p.X < kobo.EdgeWidth || p.X >= a.C.Bounds().Dx()-kobo.EdgeWidth; !edge {
		a.vkPress(g)
	}
	return true
}

// vkPress presses g's key: a modifier holds (or lets go), another key
// goes down.
func (a *bro) vkPress(g *kobo.Gesture) {
	v := &a.vk
	h := v.held[g]
	if h.k.code == vkLang {
		delete(v.held, g)
		a.typeIn(v.lang + 1)
		return
	}
	if kobo.IsModifier(h.k.code) {
		delete(v.held, g)
		on := !v.latched[h.k.code]
		v.latched[h.k.code] = on
		a.Key(kobo.KeyMsg{Code: h.k.code, Value: map[bool]int32{false: 0, true: 1}[on]})
		if h.gen == v.gen {
			a.vkRedrawMods(h.k.code == kobo.KeyLeftShift)
		}
		return
	}
	h.sent = true
	v.held[g] = h
	v.next = time.Now().Add(vkDelay)
	a.Key(kobo.KeyMsg{Code: h.k.code, Value: 1})
}

// vkMove makes a finger that waited on a key at an edge and moved the
// edge's swipe.
func (a *bro) vkMove(p image.Point, g *kobo.Gesture) bool {
	v := &a.vk
	h, ok := v.held[g]
	switch {
	case !ok:
		return false
	case h.sent || g.Moved < vkSlop:
		return true
	}
	delete(v.held, g)
	if h.gen == v.gen {
		a.drawVkbd(func(o vkey) bool { return o.r == h.k.r })
	}
	g.Owner = 0
	if !a.overlay() {
		a.edge = g
		a.edgeSwipe(p, g)
	}
	return true
}

// vkLift lets the key go, and the modifiers that held for it.
func (a *bro) vkLift(g *kobo.Gesture) bool {
	v := &a.vk
	h, ok := v.held[g]
	if !ok {
		return false
	}
	if !h.sent { // a tap at an edge
		a.vkPress(g)
		if _, ok := v.held[g]; !ok { // a modifier: done
			return true
		}
	}
	delete(v.held, g)
	a.Key(kobo.KeyMsg{Code: h.k.code, Value: 0})
	shift := v.latched[kobo.KeyLeftShift]
	if len(v.held) == 0 {
		a.vkLetGo()
	}
	if h.gen == v.gen {
		a.drawVkbd(func(o vkey) bool { return o.r == h.k.r })
		a.vkRedrawMods(shift)
	}
	return true
}

// vkRelease lets go the keys held and the modifiers.
func (a *bro) vkRelease() {
	for g, h := range a.vk.held {
		if h.sent {
			a.Key(kobo.KeyMsg{Code: h.k.code, Value: 0})
		}
		delete(a.vk.held, g)
	}
	a.vkLetGo()
}

// vkLetGo lets the modifiers go.
func (a *bro) vkLetGo() {
	for code, on := range a.vk.latched {
		if on {
			a.Key(kobo.KeyMsg{Code: code, Value: 0})
		}
	}
	clear(a.vk.latched)
}

// vkRedrawMods draws the modifiers again, and the characters if Shift
// changed them.
func (a *bro) vkRedrawMods(chars bool) {
	a.drawVkbd(func(k vkey) bool { return kobo.IsModifier(k.code) || chars && k.label == "" })
}

// vkTick repeats a key held, and shows or hides the keyboard on auto.
func (a *bro) vkTick() {
	v := &a.vk
	for g, h := range v.held {
		if !h.sent && time.Since(h.at) >= vkDelay { // held at an edge: pressed
			a.vkPress(g)
			v.next = time.Now().Add(vkEvery)
		}
	}
	if len(v.held) == 1 && time.Now().After(v.next) {
		for _, h := range v.held {
			if h.sent {
				a.Key(kobo.KeyMsg{Code: h.k.code, Value: 2})
			}
		}
		v.next = time.Now().Add(vkEvery)
	}
	if !a.overlay() && !a.Calibrating() && len(v.held) == 0 && a.vkWanted() != v.on {
		a.vkApply()
	}
}

// vkWanted says whether it should show now.
func (a *bro) vkWanted() bool {
	if a.vk.said != 0 {
		return a.vk.said > 0
	}
	set := a.setting("vkbd")
	if a.vk.ask && a.Keyboard == "" && set != "none" {
		return true
	}
	switch set {
	case "on":
		return true
	case "auto":
		since := a.vk.since
		if a.slp.wokeAt.After(since) {
			since = a.slp.wokeAt // the keyboard wakes up too
		}
		return a.Keyboard == "" && time.Since(since) >= vkAutoWait
	}
	return false
}

// vkApply shows it or hides it, as the setting has it now.
func (a *bro) vkApply() {
	v := &a.vk
	if on := a.vkWanted(); on != v.on {
		log.Printf("on-screen keyboard: %v", on)
		v.on = on
		a.Layout(a.lay)
		if !a.overlay() && !a.Calibrating() {
			a.DrawPage()
			a.MarkFlash(a.lay.Page)
		}
	}
}

// vkToggle shows it if it's hidden, else hides it, whatever the
// setting: till a text field wants it, or a keyboard comes.
func (a *bro) vkToggle() {
	a.vk.said = map[bool]int{false: 1, true: -1}[a.vk.on]
	a.vkApply()
}

// typeIn types in the i'th of the languages set, round; the letters on
// the screen show it.
func (a *bro) typeIn(i int) {
	langs := a.langs()
	a.vk.lang = i % len(langs)
	kobo.SetTyping(langs[a.vk.lang])
	log.Printf("typing in %s", kobo.Typing())
	a.showMode()
	if len(langs) == 1 && i > 0 {
		a.Toast("one language: Settings, Languages to type in", 3*time.Second)
	} else if i > 0 {
		a.Toast("typing in "+langNames[kobo.Typing()], 2*time.Second)
	}
	a.drawVkbd(func(k vkey) bool { return k.label == "" || k.code == vkLang })
}

// langs are the languages to type in, set in Settings.
func (a *bro) langs() []string { return strings.Fields(a.setting("languages")) }

var langNames = map[string]string{"en": "English", "ru": "Russian", "uk": "Ukrainian", "de": "German"}

// vkAsk shows it for a text field being typed in, if there's no other
// keyboard (and even if it was hidden), or lets it go.
func (a *bro) vkAsk(on bool) {
	if on && a.vk.said < 0 {
		a.vk.said = 0
	}
	a.vk.ask = on
	a.vkApply()
}

// vkSwipe takes two fingers going up from the screen's foot (shown), or
// down onto it while it shows (hidden): true once it's done.
func (a *bro) vkSwipe(f, g *kobo.Gesture) bool {
	if a.Locked() {
		return false
	}
	up := func(g *kobo.Gesture) int { // how far up it went, if it's mostly up
		d := g.Last.Sub(g.Start)
		if kobo.Abs(d.Y) < 2*kobo.Abs(d.X) {
			return 0
		}
		return -d.Y
	}
	foot := a.C.Bounds().Max.Y - vkFoot
	switch {
	case !a.vk.on && f.Start.Y >= foot && g.Start.Y >= foot:
		if up(f) < pairSwipe || up(g) < pairSwipe {
			return false
		}
		a.vk.said = 1
	case a.vk.on && f.Start.Y < a.vk.top && g.Start.Y < a.vk.top:
		if up(f) > -pairSwipe || up(g) > -pairSwipe || max(f.Last.Y, g.Last.Y) < a.vk.top-pairSwipe {
			return false
		}
		a.vk.said = -1
	default:
		return false
	}
	log.Printf("two-finger swipe: on-screen keyboard %v", a.vk.said > 0)
	a.vkApply()
	return true
}

// vkFromFoot says f and g both came down at the screen's foot: two
// fingers for the keyboard, not a pinch.
func (a *bro) vkFromFoot(f, g *kobo.Gesture) bool {
	foot := a.C.Bounds().Max.Y - vkFoot
	return !a.vk.on && f.Start.Y >= foot && g.Start.Y >= foot
}
