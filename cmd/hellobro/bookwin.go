package main

// The reader: a book in a window, a page at a time, in the Kobo's own
// fonts (bookfonts.go), laid out by package book. Without a book it's
// the shelf: the EPUBs on the Kobo, the last read first. Where each
// book was left is kept in hellobro.json, as a place in a chapter, so a
// new size or font finds the page again.

import (
	"fmt"
	"image"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	xdraw "golang.org/x/image/draw"

	"hellobro/book"
	"hellobro/kobo"
)

const (
	bookPxDefault        = 34 // about 11 points on the Elipsa
	bookPxMin, bookPxMax = 20, 64
	bookPxStep           = 2
	bookFlashEvery       = 8                // page turns between flashes, which clear the ghosts
	bookSaveEvery        = 20 * time.Second // the place is kept at most this often while reading
	bookShelfMax         = 500
)

// bookPlace is where a book was left.
type bookPlace struct {
	Title string `json:"title,omitempty"`
	Spine int    `json:"spine"`
	Block int    `json:"block"`
	Off   int    `json:"off"`
	Pct   int    `json:"pct"`  // how far in, for the shelf
	Read  int64  `json:"read"` // when, Unix seconds
}

// bookState is what the readers share: the fonts, the size.
type bookState struct {
	fams  []bookFamily
	faces *bookFaces
}

type bookRow struct {
	r  image.Rectangle
	do func()
}

type shelfBook struct {
	path, name string
	pct        int
	read       int64
}

type bookWin struct {
	path  string // "": the shelf
	b     *book.Book
	err   string
	spine int
	at    book.Pos // the page shown starts here
	ch    *book.Chapter
	pages []book.Page
	page  int
	laid  [4]int // what the pages are laid out for: width, height, size, font

	p       kobo.Panel
	area    image.Rectangle // the text's, in the body
	turns   int
	savedAt time.Time
	pics    map[string]*image.Gray // the page's pictures, scaled

	list    bool // the contents (or the shelf) show
	sel     int
	top     int
	rows    []bookRow
	shelf   []shelfBook
	looking bool // the shelf is being looked for
}

func (w *bookWin) Title() string {
	switch {
	case w.path == "":
		return "Books"
	case w.b != nil && w.b.Title != "":
		t, _, _ := strings.Cut(w.b.Title, " / ") // Gutenberg's: "Alice… / Illustrated by…"
		return t
	}
	return strings.TrimSuffix(filepath.Base(w.path), ".epub")
}
func (w *bookWin) Class() string    { return "hellobro" }
func (w *bookWin) Instance() string { return "book" }

// openShelf opens the shelf in a window.
func (a *bro) openShelf() { a.openWin(a.newBookWin("")) }

// newBookWin is a window with the book at path in it, where it was
// left; "": the shelf.
func (a *bro) newBookWin(path string) *bookWin {
	w := &bookWin{path: path}
	if path == "" {
		w.list = true
		a.findBooks(w)
		return w
	}
	b, err := book.Open(path)
	if err != nil {
		w.err = err.Error()
		log.Printf("book: %s: %v", path, err)
		return w
	}
	w.b = b
	if pl, ok := a.mem.Books[path]; ok && pl.Spine < len(b.Spine) {
		w.spine, w.at = pl.Spine, book.Pos{Block: pl.Block, Off: pl.Off}
	}
	return w
}

// faces are the reader's fonts, as chosen.
func (a *bro) bookFaces() *bookFaces {
	s := &a.bk
	if s.fams == nil {
		s.fams = bookFamilies()
	}
	if s.faces == nil {
		fam := s.fams[0]
		for _, f := range s.fams {
			if f.name == a.mem.BookFont {
				fam = f
			}
		}
		s.faces = newBookFaces(fam)
	}
	return s.faces
}

func (a *bro) bookPx() int {
	if px := a.mem.BookPx; px >= bookPxMin && px <= bookPxMax {
		return px
	}
	return bookPxDefault
}

// --- the window ---

func (w *bookWin) place(a *bro, p kobo.Panel) {
	p.Name = w.Title()
	w.p = p
	w.rows = nil
	if p.Outer.Empty() {
		return
	}
	body := p.Body
	mx := max(24, body.Dx()/14)
	foot := kobo.TextWidth(a.F.Small, "M")*2 + 8
	w.area = image.Rect(body.Min.X+mx, body.Min.Y+max(20, body.Dy()/40), body.Max.X-mx, body.Max.Y-foot)
}

func (w *bookWin) panel(a *bro) kobo.Panel {
	p := w.p
	p.Name = w.Title()
	return a.winPanel(p, a.focusedWin() == window(w))
}

func (w *bookWin) drawTitle(a *bro) { a.DrawPanelTitle(w.panel(a), w.status()) }

func (w *bookWin) status() string {
	switch {
	case w.path == "":
		return fmt.Sprintf("%d books", len(w.shelf))
	case w.b == nil || len(w.pages) == 0:
		return ""
	}
	return fmt.Sprintf("%d/%d · %d%%", w.page+1, len(w.pages), w.pct())
}

// pct is how far into the book the page is.
func (w *bookWin) pct() int {
	if w.b == nil || len(w.pages) == 0 {
		return 0
	}
	return int((float64(w.spine) + float64(w.page+1)/float64(len(w.pages))) * 100 / float64(len(w.b.Spine)))
}

// layout lays the chapter out again if it's not laid out for the page
// as it is now, and finds the place in it.
func (w *bookWin) layout(a *bro) {
	if w.b == nil || w.area.Empty() {
		return
	}
	faces := a.bookFaces()
	laid := [4]int{w.area.Dx(), w.area.Dy(), a.bookPx(), slices.IndexFunc(a.bk.fams, func(f bookFamily) bool { return f.name == faces.fam.name })}
	if w.ch != nil && laid == w.laid {
		return
	}
	if w.ch == nil {
		ch, err := w.b.Chapter(w.spine)
		if err != nil {
			w.err = err.Error()
			w.ch = &book.Chapter{}
		} else {
			w.ch, w.err = ch, ""
		}
	}
	t := time.Now()
	w.pages = book.Layout(w.ch, book.Settings{W: w.area.Dx(), H: w.area.Dy(), Px: laid[2], Lang: w.b.Lang}, faces, func(src string) image.Point {
		p, _ := w.b.ImageSize(src)
		return p
	})
	w.laid = laid
	w.page = book.PageAt(w.pages, w.at)
	log.Printf("book: chapter %d: %d pages in %v", w.spine, len(w.pages), time.Since(t).Round(time.Millisecond))
}

func (w *bookWin) draw(a *bro) {
	p := w.panel(a)
	if p.Outer.Empty() {
		return
	}
	a.DrawPanel(p, w.status())
	body := p.Body
	a.C.Fill(body, kobo.White)
	switch {
	case w.list:
		w.drawList(a)
	case w.err != "" && w.b == nil:
		a.C.Text(a.F.Body, image.Rect(w.area.Min.X, w.area.Min.Y, w.area.Max.X, w.area.Min.Y+40), "can't read it: "+w.err, kobo.Dark, kobo.AlignLeft)
	default:
		w.layout(a)
		w.drawPage(a)
	}
	if w.turns >= bookFlashEvery {
		w.turns = 0
		a.MarkFlash(body)
	} else {
		a.Mark(body, true)
	}
}

func (w *bookWin) drawPage(a *bro) {
	if w.page >= len(w.pages) {
		return
	}
	faces := a.bookFaces()
	at := w.area.Min
	pics := map[string]*image.Gray{}
	for _, it := range w.pages[w.page].Items {
		switch it.Kind {
		case book.ItemText:
			a.C.TextAt(faces.Face(it.Style, it.Px), at.X+it.X, at.Y+it.Y, it.Text, kobo.Black, w.p.Body)
		case book.ItemRule:
			a.C.Fill(image.Rect(it.X, it.Y, it.X+it.W, it.Y+it.H).Add(at), kobo.Dark)
		case book.ItemPicture:
			r := image.Rect(it.X, it.Y, it.X+it.W, it.Y+it.H).Add(at)
			key := fmt.Sprintf("%s@%dx%d", it.Text, it.W, it.H)
			g := w.pics[key]
			if g == nil {
				g = w.picture(it.Text, r.Size())
			}
			if g != nil {
				pics[key] = g
				kobo.CopyGray(a.C.Gray, r.Intersect(w.p.Body), g, image.Point{})
			}
		}
	}
	w.pics = pics // only this page's are kept
	// the chapter, under the page
	foot := image.Rect(w.area.Min.X, w.area.Max.Y, w.area.Max.X, w.p.Body.Max.Y-4)
	a.C.Text(a.F.Small, foot, w.chapterName(), kobo.Mid, kobo.AlignLeft)
	a.C.Text(a.F.Small, foot, fmt.Sprintf("%d%%", w.pct()), kobo.Mid, kobo.AlignRight)
}

// picture is the book's picture src, gray, scaled to size.
func (w *bookWin) picture(src string, size image.Point) *image.Gray {
	m, err := w.b.Image(src)
	if err != nil {
		log.Printf("book: %s: %v", src, err)
		return nil
	}
	g := image.NewGray(image.Rectangle{Max: size})
	xdraw.ApproxBiLinear.Scale(g, g.Bounds(), m, m.Bounds(), xdraw.Src, nil)
	return g
}

// chapterName is the contents' line for the chapter shown.
func (w *bookWin) chapterName() string {
	name := ""
	for _, e := range w.b.TOC {
		if e.Spine > w.spine {
			break
		}
		if e.Frag == "" || w.ch == nil || w.ch.IDs[e.Frag] <= w.pages[w.page].Start.Block {
			name = e.Title
		}
	}
	return name
}

// --- lists: the contents, the shelf ---

func (w *bookWin) listLines() []string {
	var ls []string
	if w.path == "" {
		for _, b := range w.shelf {
			s := b.name
			if b.read > 0 {
				s += fmt.Sprintf("   · %d%%", b.pct)
			}
			ls = append(ls, s)
		}
		return ls
	}
	for _, e := range w.b.TOC {
		ls = append(ls, strings.Repeat("    ", min(e.Depth, 4))+e.Title)
	}
	return ls
}

func (w *bookWin) listOpen(a *bro, i int) {
	if w.path == "" {
		if i < len(w.shelf) {
			a.openBook(w, w.shelf[i].path)
		}
		return
	}
	e := w.b.TOC[i]
	w.list = false
	w.goTo(e.Spine, book.Pos{})
	if e.Frag != "" {
		w.layout(a)
		if bi, ok := w.ch.IDs[e.Frag]; ok {
			w.at = book.Pos{Block: bi}
			w.page = book.PageAt(w.pages, w.at)
		}
	}
	w.draw(a)
}

func (w *bookWin) drawList(a *bro) {
	lines := w.listLines()
	f := a.F.Body
	rowH := kobo.TextWidth(f, "M")*2 + 10
	area := w.p.Body.Inset(kobo.Pad)
	n := max(1, area.Dy()/rowH)
	if len(lines) == 0 {
		msg := "no books here: put EPUBs on the Kobo (anywhere but its dot folders)"
		if w.looking {
			msg = "looking for books…"
		}
		a.C.Text(f, image.Rect(area.Min.X, area.Min.Y, area.Max.X, area.Min.Y+rowH), msg, kobo.Dark, kobo.AlignLeft)
		return
	}
	w.sel = max(0, min(w.sel, len(lines)-1))
	if w.sel < w.top {
		w.top = w.sel
	} else if w.sel >= w.top+n {
		w.top = w.sel - n + 1
	}
	w.top = max(0, min(w.top, len(lines)-n))
	w.rows = nil
	for k := 0; k < n && w.top+k < len(lines); k++ {
		i := w.top + k
		r := image.Rect(area.Min.X, area.Min.Y+k*rowH, area.Max.X, area.Min.Y+(k+1)*rowH)
		ink := uint8(kobo.Black)
		if i == w.sel {
			a.C.Fill(r, kobo.Black)
			ink = kobo.White
		}
		a.C.Text(f, r.Inset(8), lines[i], ink, kobo.AlignLeft)
		w.rows = append(w.rows, bookRow{r, func() { w.listOpen(a, i) }})
	}
}

// openBook puts the book at path in the shelf's window.
func (a *bro) openBook(w *bookWin, path string) {
	nw := a.newBookWin(path)
	a.replaceWin(w, nw)
}

// findBooks looks for the EPUBs on the Kobo, away from the main loop.
func (a *bro) findBooks(w *bookWin) {
	root := os.Getenv("HELLOBRO_BOOKS")
	if root == "" {
		root = "/mnt/onboard"
	}
	w.looking = true
	known := a.mem.Books
	go func() {
		var found []shelfBook
		filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if p != root && (strings.HasPrefix(d.Name(), ".") || strings.Count(strings.TrimPrefix(p, root), "/") > 5) {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(strings.ToLower(d.Name()), ".epub") && len(found) < bookShelfMax {
				b := shelfBook{path: p, name: strings.TrimSuffix(strings.TrimSuffix(d.Name(), ".epub"), ".kepub")}
				if pl, ok := known[p]; ok {
					b.pct, b.read = pl.Pct, pl.Read
					if pl.Title != "" {
						b.name = pl.Title
					}
				}
				found = append(found, b)
			}
			return nil
		})
		slices.SortFunc(found, func(x, y shelfBook) int {
			if x.read != y.read {
				return int(y.read - x.read)
			}
			return strings.Compare(strings.ToLower(x.name), strings.ToLower(y.name))
		})
		a.Do(func() {
			w.shelf, w.looking = found, false
			if a.shows(w) {
				w.draw(a)
			}
		})
	}()
}

// --- turning pages ---

// goTo shows chapter spine at pos.
func (w *bookWin) goTo(spine int, pos book.Pos) {
	if spine != w.spine {
		w.ch, w.pics = nil, nil
	}
	w.spine, w.at = spine, pos
	w.laid = [4]int{}
}

// turn goes d pages on (back if negative), across chapters.
func (w *bookWin) turn(a *bro, d int) {
	if w.b == nil || len(w.pages) == 0 {
		return
	}
	switch n := w.page + d; {
	case n >= len(w.pages):
		if w.spine+1 >= len(w.b.Spine) {
			return
		}
		w.goTo(w.spine+1, book.Pos{})
	case n < 0:
		if w.spine == 0 {
			return
		}
		w.goTo(w.spine-1, book.Pos{Block: 1 << 30}) // its last page
	default:
		w.page = n
		w.at = w.pages[n].Start
	}
	w.turns++
	w.draw(a)
	if w.ch != nil && w.page < len(w.pages) {
		w.at = w.pages[w.page].Start
	}
	if time.Since(w.savedAt) > bookSaveEvery {
		w.save(a)
	}
}

// save keeps the place.
func (w *bookWin) save(a *bro) {
	if w.b == nil {
		return
	}
	if a.mem.Books == nil {
		a.mem.Books = map[string]bookPlace{}
	}
	a.mem.Books[w.path] = bookPlace{Title: w.b.Title, Spine: w.spine, Block: w.at.Block, Off: w.at.Off, Pct: w.pct(), Read: time.Now().Unix()}
	a.mem.save()
	w.savedAt = time.Now()
}

// restyle lays the books out again in another size or font.
func (a *bro) restyle(px int, font string) {
	a.mem.BookPx, a.mem.BookFont = px, font
	a.bk.faces = nil
	a.mem.save()
	a.Notify(fmt.Sprintf("%s · %d px", a.bookFaces().fam.name, a.bookPx()))
	for _, win := range a.windows() {
		if w, ok := win.(*bookWin); ok && w.b != nil && a.shows(w) {
			w.draw(a)
		}
	}
}

func (w *bookWin) press(a *bro, code uint16, value int32) bool {
	if value == 0 {
		return false
	}
	m := &a.tv.mods
	if m.ctrl || m.alt || m.meta {
		return false
	}
	ch, _ := kobo.KeyChar(code, m.shift, m.caps)
	if w.list {
		switch {
		case code == 103 || ch == "k": // up
			w.sel--
		case code == 108 || ch == "j":
			w.sel++
		case code == 104: // PgUp
			w.sel -= max(1, len(w.rows)-1)
		case code == 109:
			w.sel += max(1, len(w.rows)-1)
		case code == kobo.KeyEnter:
			w.listOpen(a, w.sel)
			return true
		case code == kobo.KeyEsc && w.b != nil, ch == "t" && w.b != nil:
			w.list = false
		default:
			return false
		}
		w.draw(a)
		return true
	}
	switch {
	case code == kobo.KeySpace || code == 106 || code == 109 || code == 108 || ch == "j" || ch == "l" || ch == "n":
		w.turn(a, 1)
	case code == 105 || code == 104 || code == 103 || code == kobo.KeyBackspace || ch == "k" || ch == "h" || ch == "p":
		w.turn(a, -1)
	case code == 102: // Home: the chapter's start
		w.turn(a, -w.page)
	case code == 107:
		w.turn(a, len(w.pages)-1-w.page)
	case ch == "+" || ch == "=":
		a.restyle(min(bookPxMax, a.bookPx()+bookPxStep), a.bookFaces().fam.name)
	case ch == "-":
		a.restyle(max(bookPxMin, a.bookPx()-bookPxStep), a.bookFaces().fam.name)
	case ch == "f" || ch == "F":
		fams, cur := a.bk.fams, a.bookFaces().fam.name
		i := slices.IndexFunc(fams, func(f bookFamily) bool { return f.name == cur })
		d := 1
		if ch == "F" {
			d = len(fams) - 1
		}
		a.restyle(a.bookPx(), fams[(i+d)%len(fams)].name)
	case ch == "t" || code == kobo.KeyEnter:
		w.openContents(a)
	case code == kobo.KeyEsc:
		w.save(a)
		a.replaceWin(w, a.newBookWin(""))
	default:
		return false
	}
	return true
}

// openContents shows the book's contents, the chapter shown picked.
func (w *bookWin) openContents(a *bro) {
	if w.b == nil || len(w.b.TOC) == 0 {
		a.Notify("no contents in this book")
		return
	}
	w.list, w.sel = true, 0
	for i, e := range w.b.TOC {
		if e.Spine <= w.spine {
			w.sel = i
		}
	}
	w.top = w.sel - 3
	w.draw(a)
}

// tap: on the list, a row; on a page, the left third back, the rest
// on, the top the contents.
func (w *bookWin) tap(a *bro, p image.Point) {
	body := w.p.Body
	if !p.In(body) {
		return
	}
	if w.list {
		for _, r := range w.rows {
			if p.In(r.r) {
				r.do()
				return
			}
		}
		return
	}
	switch {
	case p.Y < body.Min.Y+body.Dy()/8:
		w.openContents(a)
	case p.X < body.Min.X+body.Dx()/3:
		w.turn(a, -1)
	default:
		w.turn(a, 1)
	}
}

func (w *bookWin) closing(a *bro) {
	w.save(a)
	if w.b != nil {
		w.b.Close()
	}
	w.place(a, kobo.Panel{})
}
