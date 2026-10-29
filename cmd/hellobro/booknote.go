package main

// A link tapped on an EPUB's page: what it points at shows in a box over
// the page, a note most often (its number, raised), so it's read without
// leaving the page. "go there" goes to it; the strip under the page then
// has "‹ back" where the chapter's name was. A link to a whole chapter
// goes there at once. The notes may be in a file not read in order (the
// end notes), and a long one: it's read aside.

import (
	"image"
	"log"
	"strings"

	"hellobro/book"
	"hellobro/kobo"
)

// bookNote is a link's target, shown in a box over the page.
type bookNote struct {
	link  string
	page  *book.Page // laid out for the box; nil while it's read
	box   image.Rectangle
	there image.Rectangle // "go there"
}

// bookFrom is where a link was followed from, to go back to.
type bookFrom struct {
	spine int
	at    book.Pos
}

const linkReach = 56 // a link's number grown to a finger's size

// linkAt is the link under p on the page shown, the nearest; "" none.
func (w *bookWin) linkAt(p image.Point) string {
	if w.b == nil || w.pdf != nil || w.page >= len(w.pages) {
		return ""
	}
	type shown struct {
		n  int
		at image.Point
	}
	pages := []shown{{w.page, w.area.Min}}
	if w.laid.two && w.page+1 < len(w.pages) {
		pages = append(pages, shown{w.page + 1, w.area.Min.Add(image.Pt(w.laid.w+bookGutter, 0))})
	}
	best, far := "", 1<<30
	for _, s := range pages {
		for _, it := range w.pages[s.n].Items {
			if it.Kind != book.ItemText || it.Link == "" {
				continue
			}
			r := image.Rect(it.X, it.Y-it.Px, it.X+it.W, it.Y+it.Px/3).Add(s.at)
			if d := linkReach - r.Dx(); d > 0 {
				r.Min.X, r.Max.X = r.Min.X-d/2, r.Max.X+d-d/2
			}
			if d := linkReach - r.Dy(); d > 0 {
				r.Min.Y, r.Max.Y = r.Min.Y-d/2, r.Max.Y+d-d/2
			}
			if !p.In(r) {
				continue
			}
			m := r.Min.Add(r.Max).Div(2).Sub(p)
			if d := m.X*m.X + m.Y*m.Y; d < far {
				best, far = it.Link, d
			}
		}
	}
	return best
}

// followLink shows what link points at, or goes to a chapter.
func (w *bookWin) followLink(a *bro, link string) {
	if _, id, _ := strings.Cut(link, "#"); id == "" {
		w.goThere(a, link)
		return
	}
	w.note = &bookNote{link: link}
	b := w.b
	go func() {
		bs, err := b.Note(link)
		a.Do(func() { w.noteRead(a, link, bs, err) })
	}()
}

// noteRead lays out the note read aside, in a box at the page's foot as
// high as it needs (three fifths of the page at most), and shows it.
func (w *bookWin) noteRead(a *bro, link string, bs []book.Block, err error) {
	if w.gone || w.note == nil || w.note.link != link {
		return
	}
	if err != nil {
		log.Printf("book: link %s: %v", link, err)
		w.note = nil
		w.goThere(a, link) // an id it can't find: the chapter, at least
		return
	}
	var text []book.Block
	for _, b := range bs { // no pictures in the box
		if b.Kind != book.Picture {
			text = append(text, b)
		}
	}
	st := a.bookStyle()
	st.px = max(bookPxMin, st.px*85/100)
	pad, rowH := 24, a.libRowH()
	room := image.Pt(w.area.Dx()-2*pad, w.area.Dy()*3/5-2*pad-rowH)
	if len(text) == 0 || room.X < 100 || room.Y < 2*st.px {
		w.note = nil
		w.goThere(a, link)
		return
	}
	pages := book.Layout(&book.Chapter{Blocks: text}, st.settings(room, w.b.Lang), a.bookFaces(), nil)
	pg := &pages[0]
	h := 0
	for _, it := range pg.Items {
		switch it.Kind {
		case book.ItemText:
			h = max(h, it.Y+it.Px/3)
		default:
			h = max(h, it.Y+it.H)
		}
	}
	box := image.Rect(w.area.Min.X, w.area.Max.Y-min(room.Y, h)-2*pad-rowH, w.area.Max.X, w.area.Max.Y)
	there := image.Rect(box.Max.X-pad-220, box.Max.Y-pad/2-rowH, box.Max.X-pad, box.Max.Y-pad/2).Inset(4)
	w.note.page, w.note.box, w.note.there = pg, box, there
	if a.shows(w) {
		w.draw(a)
	}
}

// drawNote draws the note's box over the page.
func (w *bookWin) drawNote(a *bro) {
	n := w.note
	if n == nil || n.page == nil {
		return
	}
	c := a.C
	c.Fill(n.box, kobo.White)
	c.Fill(image.Rect(n.box.Min.X, n.box.Min.Y, n.box.Max.X, n.box.Min.Y+3), kobo.Black)
	c.Frame(n.box, 1, kobo.Dark)
	in := n.box.Inset(24)
	faces := a.bookFaces()
	for _, it := range n.page.Items {
		switch it.Kind {
		case book.ItemText:
			c.TextAt(faces.Face(it.Style, it.Px), in.Min.X+it.X, in.Min.Y+it.Y, it.Text, kobo.Black, in)
		case book.ItemRule:
			c.Fill(image.Rect(it.X, it.Y, it.X+it.W, it.Y+it.H).Add(in.Min).Intersect(in), kobo.Dark)
		}
	}
	c.Frame(n.there, 2, kobo.Black)
	c.Text(a.F.Panel, n.there, "go there", kobo.Black, kobo.AlignCenter)
	c.Text(a.F.Small, image.Rect(in.Min.X, n.there.Min.Y, n.there.Min.X-16, n.there.Max.Y), "a tap elsewhere closes it", kobo.Mid, kobo.AlignLeft)
}

// noteTap takes a tap while a note shows: go there, or it goes.
func (w *bookWin) noteTap(a *bro, p image.Point) {
	n := w.note
	w.note = nil
	if n.page != nil && p.In(n.there) {
		w.goThere(a, n.link)
		return
	}
	w.draw(a)
}

// goThere goes where link points, if it's in the book's reading order;
// "‹ back" comes back.
func (w *bookWin) goThere(a *bro, link string) {
	file, id, _ := strings.Cut(link, "#")
	spine := w.b.SpineOf(file)
	if spine < 0 {
		a.Notify("that link goes out of the book's pages")
		w.draw(a)
		return
	}
	from := &bookFrom{w.spine, w.at}
	w.goTo(spine, book.Pos{})
	w.frag, w.from = id, from
	w.draw(a)
}

// goBack goes back where the link was followed from.
func (w *bookWin) goBack(a *bro) {
	f := w.from
	w.goTo(f.spine, f.at)
	w.from = nil
	w.draw(a)
}
