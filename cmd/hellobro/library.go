package main

// What the books' and the notebooks' lists share: their header, and
// their tabs and folders. The header is the same everywhere (the shelf,
// a book's contents and style, the notebooks), so a finger finds it
// without looking: at the left the way to all of them ("All Books"),
// lit where that's where we are; at the right the way back to the one
// open ("back to the book"), if there's one; tabs between. The tabs are
// Recent (the last read or written first, then the rest by name), All
// (by name) and Folders (a folder at a time, those in it first).

import (
	"cmp"
	"fmt"
	"image"
	"path"
	"slices"
	"strings"

	"hellobro/kobo"
)

// The tabs of a list of them all.
const (
	libRecent = iota
	libAll
	libFolders
)

var libTabNames = []string{"Recent", "All", "Folders"}

// libTab is a tab in the header.
type libTab struct {
	name string
	on   bool
	do   func()
}

// libHead is where the header put its buttons.
type libHead struct {
	all, back image.Rectangle
	rows      []bookRow
}

// libRowH is a list's row: room for a finger.
func (a *bro) libRowH() int { return max(libRowMin, kobo.TextWidth(a.F.Body, "M")*2+10) }

var libRowMin = 64 // sizes.go: taller for fingers

// drawLibHeader draws the header in r: all ("All Books"), lit if here;
// back ("back to the book"), if there's somewhere to go back to; the
// tabs between.
func (a *bro) drawLibHeader(r image.Rectangle, all string, here bool, toAll func(), back string, toBack func(), tabs []libTab) libHead {
	c := a.C
	var h libHead
	h.all = image.Rect(r.Min.X, r.Min.Y, r.Min.X+min(230, r.Dx()/5), r.Max.Y-6)
	if here {
		c.Fill(h.all, kobo.Black)
		c.Text(a.F.Panel, h.all, all, kobo.White, kobo.AlignCenter)
	} else {
		c.Frame(h.all, 2, kobo.Black)
		c.Text(a.F.Panel, h.all, "‹ "+all, kobo.Black, kobo.AlignCenter)
	}
	h.rows = append(h.rows, bookRow{h.all, toAll})
	h.back = image.Rect(r.Max.X-min(300, r.Dx()/4), r.Min.Y, r.Max.X, r.Max.Y-6)
	if back != "" {
		if kobo.TextWidth(a.F.Panel, back)+24 > h.back.Dx() { // a narrow window
			back = "back"
		}
		c.Frame(h.back, 2, kobo.Black)
		c.Text(a.F.Panel, h.back, back, kobo.Black, kobo.AlignCenter)
		h.rows = append(h.rows, bookRow{h.back, toBack})
	}
	r.Min.X, r.Max.X = h.all.Max.X+16, h.back.Min.X-16
	c.Fill(image.Rect(r.Min.X, r.Max.Y-2, r.Max.X, r.Max.Y), kobo.Light)
	if len(tabs) == 0 {
		return h
	}
	each := r.Dx() / len(tabs)
	for k, t := range tabs {
		tr := image.Rect(r.Min.X+k*each, r.Min.Y, r.Min.X+(k+1)*each, r.Max.Y)
		ink := uint8(kobo.Mid)
		if t.on {
			ink = kobo.Black
			c.Fill(image.Rect(tr.Min.X, tr.Max.Y-5, tr.Max.X, tr.Max.Y), kobo.Black)
		}
		c.Text(a.F.Body, tr, t.name, ink, kobo.AlignCenter)
		h.rows = append(h.rows, bookRow{tr, t.do})
	}
	return h
}

// libTabs are the tabs of a list of them all, tab on; pick takes one.
func libTabs(tab int, pick func(int)) []libTab {
	var ts []libTab
	for k, n := range libTabNames {
		ts = append(ts, libTab{n, k == tab, func() { pick(k) }})
	}
	return ts
}

// libItem is a row in a list of them all: a book or a notebook, a
// folder, or the way up out of one.
type libItem struct {
	name string // as shown
	note string // gray, at the right: how far in, how many in a folder
	key  string // the book's path, the notebook's name; a folder's path
	dir  bool
	up   bool
}

// libEntry is a book or a notebook, for libView.
type libEntry struct {
	key  string // its path, or name: what opens it
	rel  string // where it is under the root, its folders and its file's name: "Fiction/alice.epub"
	name string
	note string
	at   int64 // when last read or written; 0 never
}

// libView is what a tab shows of es: Recent, the last first then the
// rest by name; All, by name; Folders, those in dir ("" the root), its
// folders first, each with how many there are in it, all told. dirs are
// folders there are besides those es are in: empty ones.
func libView(es []libEntry, dirs []string, tab int, dir string) []libItem {
	byName := func(x, y libEntry) int {
		return cmp.Or(strings.Compare(strings.ToLower(x.name), strings.ToLower(y.name)), strings.Compare(x.rel, y.rel))
	}
	es = slices.Clone(es)
	var items []libItem
	switch tab {
	case libRecent, libAll:
		slices.SortFunc(es, func(x, y libEntry) int {
			if tab == libRecent && x.at != y.at {
				return cmp.Compare(y.at, x.at)
			}
			return byName(x, y)
		})
		for _, e := range es {
			items = append(items, libItem{name: e.name, note: e.note, key: e.key})
		}
		return items
	}
	if dir != "" { // the way up: to the folder it's in
		up, name := path.Dir(dir), "‹ all folders"
		if up == "." {
			up = ""
		} else {
			name = "‹ " + path.Base(up)
		}
		items = append(items, libItem{name: name, note: dir, key: up, dir: true, up: true})
	}
	pre := ""
	if dir != "" {
		pre = dir + "/"
	}
	count := map[string]int{}
	for _, d := range dirs {
		if rest, ok := strings.CutPrefix(d, pre); ok && rest != "" {
			sub, _, _ := strings.Cut(rest, "/")
			count[sub] += 0
		}
	}
	var here []libEntry
	for _, e := range es {
		rest, ok := strings.CutPrefix(e.rel, pre)
		if !ok {
			continue
		}
		if sub, _, ok := strings.Cut(rest, "/"); ok {
			count[sub]++
		} else {
			here = append(here, e)
		}
	}
	subs := make([]string, 0, len(count))
	for s := range count {
		subs = append(subs, s)
	}
	slices.SortFunc(subs, func(x, y string) int { return strings.Compare(strings.ToLower(x), strings.ToLower(y)) })
	for _, s := range subs {
		note := fmt.Sprint(count[s])
		if count[s] == 0 {
			note = "empty"
		}
		items = append(items, libItem{name: s, note: note, key: pre + s, dir: true})
	}
	slices.SortFunc(here, byName)
	for _, e := range here {
		items = append(items, libItem{name: e.name, note: e.note, key: e.key})
	}
	return items
}

// drawFolder draws a folder's sign at the left of r, in ink.
func drawFolder(c kobo.Canvas, r image.Rectangle, ink uint8) {
	m := r.Min.Y + r.Dy()/2
	x := r.Min.X + 8
	c.Fill(image.Rect(x, m-13, x+14, m-9), ink) // its tab
	box := image.Rect(x, m-9, x+34, m+13)
	c.Frame(box, 3, ink)
}

// drawLibRow draws item it in row r, lit if chosen; at its right,
// buttons (each a label and what it does), if any. It gives back the
// rows to tap.
func (a *bro) drawLibRow(r image.Rectangle, it libItem, lit bool, open func(), buttons ...libTab) []bookRow {
	c, f := a.C, a.F.Body
	ink, gray := uint8(kobo.Black), uint8(kobo.Mid)
	if lit {
		c.Fill(r, kobo.Black)
		ink, gray = kobo.White, kobo.Light
	}
	var rows []bookRow
	right := r.Max.X - 8
	for i := len(buttons) - 1; i >= 0; i-- {
		b := buttons[i]
		bw := max(r.Dy(), kobo.TextWidth(a.F.Panel, b.name)+32)
		br := image.Rect(right-bw, r.Min.Y, right, r.Max.Y).Inset(6)
		c.Frame(br, 2, ink)
		c.Text(a.F.Panel, br, b.name, ink, kobo.AlignCenter)
		rows = append(rows, bookRow{br, b.do})
		right = br.Min.X - 6
	}
	text := image.Rect(r.Min.X+8, r.Min.Y, right-8, r.Max.Y)
	if it.dir && !it.up {
		drawFolder(c, text, ink)
		text.Min.X += 56
	}
	if it.note != "" {
		nw := kobo.TextWidth(a.F.Small, it.note)
		c.Text(a.F.Small, image.Rect(text.Max.X-nw, text.Min.Y, text.Max.X, text.Max.Y), it.note, gray, kobo.AlignRight)
		text.Max.X -= nw + 24
	}
	c.Text(f, text, it.name, ink, kobo.AlignLeft)
	return append(rows, bookRow{r, open})
}

// drawNameEdit draws a name being typed in r, after prompt, with ok and
// cancel: their rows, and the box's (a tap on it does nothing).
func (a *bro) drawNameEdit(r image.Rectangle, prompt, text string, ok, cancel func()) []bookRow {
	c := a.C
	bw := r.Dy() + 64
	okR := image.Rect(r.Max.X-2*bw-8, r.Min.Y, r.Max.X-bw-8, r.Max.Y).Inset(6)
	no := image.Rect(r.Max.X-bw, r.Min.Y, r.Max.X, r.Max.Y).Inset(6)
	box := image.Rect(r.Min.X, r.Min.Y+4, okR.Min.X-12, r.Max.Y-4)
	c.Fill(box, kobo.White)
	c.Frame(box, 3, kobo.Black)
	t := box.Inset(12)
	pw := kobo.TextWidth(a.F.Small, prompt)
	c.Text(a.F.Small, image.Rect(t.Min.X, t.Min.Y, t.Min.X+pw, t.Max.Y), prompt, kobo.Mid, kobo.AlignLeft)
	t.Min.X += pw
	c.Text(a.F.Body, t, text, kobo.Black, kobo.AlignLeft)
	cx := min(t.Max.X-3, t.Min.X+kobo.TextWidth(a.F.Body, text)+2) // the caret
	c.Fill(image.Rect(cx, t.Min.Y+4, cx+3, t.Max.Y-4), kobo.Black)
	var rows []bookRow
	for _, b := range []struct {
		r     image.Rectangle
		label string
		do    func()
	}{{okR, "ok", ok}, {no, "cancel", cancel}} {
		c.Frame(b.r, 2, kobo.Black)
		c.Text(a.F.Panel, b.r, b.label, kobo.Black, kobo.AlignCenter)
		rows = append(rows, bookRow{b.r, b.do})
	}
	return append(rows, bookRow{box, func() {}})
}

// nameKey types a key into a name: Enter is done with it, Esc done
// without it (ok false).
func nameKey(text *string, code uint16, ch string) (done, ok bool) {
	switch {
	case code == kobo.KeyEnter:
		return true, true
	case code == kobo.KeyEsc:
		return true, false
	case code == kobo.KeyBackspace:
		if r := []rune(*text); len(r) > 0 {
			*text = string(r[:len(r)-1])
		}
	case ch != "" && ch != "\t" && ch != "\n":
		*text += ch
	}
	return false, false
}
