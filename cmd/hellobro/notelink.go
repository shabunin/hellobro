package main

// Links between notebooks. A part of a page is marked (the select
// tool's "mark", or :mark): it's kept, with a small picture of it, in
// note-marks.json, the last first. The link tool (on the bar, or :link)
// lists the marks; "link here" on one, then a tap on a page, puts a link
// to it there: a text, "→ what it's to", its target in its attributes
// (Xournal++ shows the text and keeps them). A finger's tap on a link
// opens its notebook in the same window, at the page, the part marked
// framed for a moment; held, in a window of its own. :back goes back.

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"hellobro/kobo"
)

const (
	marksFile    = "note-marks.json"
	linkAttr     = "hellobro-link"      // the notebook a link goes to
	linkPageAttr = "hellobro-link-page" // its page, from 1
	linkBoxAttr  = "hellobro-link-box"  // the part marked, page units: x0 y0 x1 y1
	marksMax     = 40
	linkHold     = 600 * time.Millisecond // held this long, a link opens apart
	linkFlash    = 3 * time.Second        // the part marked, framed, on arriving
	ownLink      = 23                     // a finger on a link: its lift follows it
)

// noteAnchor is a part of a page marked, to link to.
type noteAnchor struct {
	Note  string    `json:"note"`
	Page  int       `json:"page"` // from 0
	Box   [4]int    `json:"box"`
	Label string    `json:"label,omitempty"` // its text, if it has some
	At    time.Time `json:"at"`
	Thumb []byte    `json:"thumb,omitempty"` // a PNG of it, small
}

func (an noteAnchor) rect() image.Rectangle {
	return image.Rect(an.Box[0], an.Box[1], an.Box[2], an.Box[3])
}

// where is its notebook's name (the last of its path) and page.
func (an noteAnchor) where() string {
	return fmt.Sprintf("%s, p. %d", path.Base(an.Note), an.Page+1)
}

// linkText is what a link to it says.
func (an noteAnchor) linkText(from string) string {
	w := fmt.Sprintf("p. %d", an.Page+1)
	if an.Note != from {
		w = an.where()
	}
	if an.Label == "" {
		return "→ " + w
	}
	return "→ " + an.Label + " (" + w + ")"
}

func loadAnchors() []noteAnchor {
	var as []noteAnchor
	if b, err := os.ReadFile(kobo.AppFile(marksFile)); err == nil {
		json.Unmarshal(b, &as)
	}
	return as
}

func saveAnchors(as []noteAnchor) error {
	b, err := json.Marshal(as)
	if err != nil {
		return err
	}
	return writeAtomic(kobo.AppFile(marksFile), b)
}

// keepAnchor puts an first among the marks; the same part marked before
// goes.
func keepAnchor(an noteAnchor) error {
	as := slices.DeleteFunc(loadAnchors(), func(o noteAnchor) bool { return o.Note == an.Note && o.Page == an.Page && o.Box == an.Box })
	as = append([]noteAnchor{an}, as...)
	return saveAnchors(as[:min(len(as), marksMax)])
}

func forgetAnchor(at time.Time) error {
	return saveAnchors(slices.DeleteFunc(loadAnchors(), func(o noteAnchor) bool { return o.At.Equal(at) }))
}

// selLabel is the text among ss, its first line, short; "" if none.
func selLabel(ss []*noteStroke) string {
	var ts []string
	for _, s := range ss {
		if s.txt != nil && attr(s.txt.attrs, linkAttr) == "" {
			ts = append(ts, strings.TrimSpace(s.txt.s))
		}
	}
	t, _, _ := strings.Cut(strings.Join(ts, " "), "\n")
	if r := []rune(t); len(r) > 40 {
		t = string(r[:39]) + "…"
	}
	return t
}

// markSel keeps what's taken as a mark, to link to.
func (w *noteWin) markSel(a *bro) {
	if len(w.sel) == 0 {
		return
	}
	b := boxOf(w.sel)
	an := noteAnchor{Note: w.nb.name, Page: w.selPage, Box: [4]int{b.Min.X, b.Min.Y, b.Max.X, b.Max.Y}, Label: selLabel(w.sel), At: time.Now()}
	an.Thumb = w.anchorThumb(w.selPage, w.sel)
	if err := keepAnchor(an); err != nil {
		a.Notify("not marked: " + err.Error())
		return
	}
	log.Printf("notebook %s: marked, on page %d", w.nb.name, w.selPage+1)
	a.Notify("marked: the link tool puts a link to it, in any notebook")
}

// anchorThumb is a small picture of ss, on page: 360 by 140 at most.
func (w *noteWin) anchorThumb(page int, ss []*noteStroke) []byte {
	b := boxOf(ss).Inset(-8)
	if b.Empty() {
		return nil
	}
	k := min(1, 360/float64(b.Dx()), 140/float64(b.Dy()))
	m, err := renderNotePage(exportPage{n: page, size: w.pageSize(page), strokes: ss, pdfPage: -1}, "plain", nil, 227*k)
	if err != nil {
		return nil
	}
	crop := image.Rect(int(float64(b.Min.X)*k), int(float64(b.Min.Y)*k), int(float64(b.Max.X)*k)+1, int(float64(b.Max.Y)*k)+1).Intersect(m.Bounds())
	var out bytes.Buffer
	if png.Encode(&out, m.SubImage(crop)) != nil {
		return nil
	}
	return out.Bytes()
}

// linkStroke is a link to an, at (page units), on a page of from.
func linkStroke(an noteAnchor, from string, at image.Point) *noteStroke {
	n := func(k, v string) xml.Attr { return xml.Attr{Name: xml.Name{Local: k}, Value: v} }
	t := &noteText{s: an.linkText(from), size: noteTextSize, attrs: []xml.Attr{n("font", "Sans"), n("color", "#000000ff"),
		n(linkAttr, an.Note), n(linkPageAttr, strconv.Itoa(an.Page+1)), n(linkBoxAttr, fmt.Sprintf("%d %d %d %d", an.Box[0], an.Box[1], an.Box[2], an.Box[3]))}}
	return textStroke(t, at)
}

// linkOf is where s goes, if it's a link.
func linkOf(s *noteStroke) (noteAnchor, bool) {
	if s == nil || s.txt == nil {
		return noteAnchor{}, false
	}
	note := attr(s.txt.attrs, linkAttr)
	if note == "" {
		return noteAnchor{}, false
	}
	an := noteAnchor{Note: note}
	if p, err := strconv.Atoi(attr(s.txt.attrs, linkPageAttr)); err == nil {
		an.Page = max(0, p-1)
	}
	fmt.Sscan(attr(s.txt.attrs, linkBoxAttr), &an.Box[0], &an.Box[1], &an.Box[2], &an.Box[3])
	return an, true
}

// linkAt is the link at p (page units) on page, if there's one.
func (w *noteWin) linkAt(page int, p image.Point) *noteStroke {
	if page < 0 || page >= len(w.nb.pages) {
		return nil
	}
	ss := w.nb.pages[page]
	for i := len(ss) - 1; i >= 0; i-- {
		if _, ok := linkOf(ss[i]); ok && p.In(ss[i].box.Inset(-12)) {
			return ss[i]
		}
	}
	return nil
}

// linkHere puts a link to an where the page is tapped next.
func (w *noteWin) linkHere(a *bro, an noteAnchor) {
	w.setTool(a, "pen")
	w.linking = &an
	a.Notify("tap where the link goes (Esc: none)")
}

// putLink puts the link being placed on page at p (page units).
func (w *noteWin) putLink(a *bro, page int, p image.Point) {
	an := *w.linking
	w.linking = nil
	s := linkStroke(an, w.nb.name, p)
	if d := into(s.box, w.pageBounds(page)); d != (image.Point{}) {
		s = s.moved(d)
	}
	for len(w.nb.pages) <= page {
		w.nb.pages = append(w.nb.pages, nil)
	}
	op := noteOp{page: page, add: []*noteStroke{s}}
	w.nb.apply(op, false)
	w.edited(op)
	log.Printf("notebook %s: a link to %s, on page %d", w.nb.name, an.where(), page+1)
	w.typed(a, s.box)
}

// drawLink frames a link, as a button is: it's tapped.
func (w *noteWin) drawLink(a *bro, s *noteStroke, at, clip image.Rectangle) {
	r := w.toScreenRect(s.box, at).Inset(-6)
	a.C.Frame(r.Intersect(clip.Inset(-4)), 2, kobo.Black)
}

// noteBack is where a link was followed from, for :back.
type noteBack struct {
	name   string
	page   int
	scroll int
}

// followLink opens where s goes: in w's place, or apart (a window of
// its own); there, if it's open already.
func (a *bro) followLink(w *noteWin, an noteAnchor, apart bool) {
	log.Printf("notebook %s: a link to %s followed", w.nb.name, an.where())
	back := &noteBack{name: w.nb.name, page: w.cur, scroll: w.scroll}
	if an.Note == w.nb.name {
		w.back = back
		w.goTo(a, an)
		return
	}
	if o := a.noteWindow(an.Note); o != nil {
		o.back = back
		a.wm.tree.Focus(a.nodeOf(o))
		a.wmApply()
		o.goTo(a, an)
		return
	}
	if _, err := os.Stat(notePath(an.Note)); err != nil && a.unsavedNote(an.Note) == nil {
		a.Notify("it's gone: notebook " + an.Note)
		return
	}
	nw := a.noteOpen(an.Note)
	if nw == nil {
		return
	}
	nw.back = back
	nw.cur = min(an.Page, max(len(nw.nb.pages)-1, 0))
	if nw.nb.rollOn() {
		nw.scroll = max(0, an.Box[1]-200)
	}
	if apart {
		a.openWin(nw)
	} else {
		w.keep(a)
		a.replaceWin(w, nw)
	}
	nw.goTo(a, an)
	a.Notify("back where you were: :back")
}

// goTo shows an's page, its part framed for a moment.
func (w *noteWin) goTo(a *bro, an noteAnchor) {
	page := min(an.Page, max(len(w.nb.pages)-1, 0))
	box := an.rect()
	w.overview, w.settings = false, false
	switch {
	case w.roll():
		w.rollShow(a, page, box.Min.Y, box.Max.Y)
	case page < w.cur || page >= w.cur+w.shown():
		w.cur = page
		w.show(a)
		if a.shows(w) {
			a.MarkFlash(w.pages().Intersect(w.paperArea()))
		}
	}
	w.flash = &noteFlash{page: page, box: box}
	f := w.flash
	w.showFlash(a, f)
	time.AfterFunc(linkFlash, func() {
		a.Do(func() {
			if w.flash == f {
				w.flash = nil
				w.showFlash(a, f)
			}
		})
	})
}

// noteFlash is a part of a page framed for a moment: a link's target.
type noteFlash struct {
	page int
	box  image.Rectangle
}

func (w *noteWin) flashScreen(f *noteFlash) image.Rectangle {
	if f == nil || f.box.Empty() {
		return image.Rectangle{}
	}
	slot, ok := w.slotOf(f.page)
	if !ok {
		return image.Rectangle{}
	}
	return w.toScreenRect(f.box, slot).Inset(-12)
}

// showFlash draws the pages again where f is (or was).
func (w *noteWin) showFlash(a *bro, f *noteFlash) {
	r := w.flashScreen(f).Inset(-2).Intersect(w.paperArea())
	if r.Empty() || !a.shows(w) || a.overlay() {
		return
	}
	w.drawPages(a, r)
	a.Mark(r, false)
}

// drawFlash frames the part a link went to, thick.
func (w *noteWin) drawFlash(a *bro, clip image.Rectangle) {
	r := w.flashScreen(w.flash)
	if r.Empty() {
		return
	}
	for _, s := range frameStrips(r, 5) {
		a.C.Fill(s.Intersect(clip).Intersect(w.paperArea()), kobo.Black)
	}
}

// goBack goes where the last link was followed from.
func (a *bro) goBack(w *noteWin) error {
	b := w.back
	if b == nil {
		return fmt.Errorf("no link followed here")
	}
	w.back = nil
	an := noteAnchor{Note: b.name, Page: b.page}
	if b.name == w.nb.name {
		w.cur, w.scroll = b.page, b.scroll
		w.place(a, w.p)
		w.show(a)
		if a.shows(w) {
			a.MarkFlash(w.paperArea())
		}
		return nil
	}
	if o := a.noteWindow(b.name); o != nil {
		a.wm.tree.Focus(a.nodeOf(o))
		a.wmApply()
		return nil
	}
	nw := a.noteOpen(b.name)
	if nw == nil {
		return fmt.Errorf("can't open %s", b.name)
	}
	nw.cur, nw.scroll = min(an.Page, max(len(nw.nb.pages)-1, 0)), b.scroll
	w.keep(a)
	a.replaceWin(w, nw)
	return nil
}
