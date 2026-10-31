package main

// The reader: a book in a window, a page at a time, in the Kobo's own
// fonts (bookfonts.go), laid out by package book. Without a book it's
// the shelf, All Books: the EPUBs and PDFs on the Kobo, in tabs, the
// last read first, all by name, or a folder at a time (library.go). Where each
// book was left is kept in hellobro.json, as a place in a chapter, so a
// new size or font finds the page again.

import (
	"fmt"
	"image"
	"io/fs"
	"log"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"

	"hellobro/book"
	"hellobro/kobo"
	"hellobro/pdf"
)

const (
	bookPxDefault        = 34 // about 11 points on the Elipsa
	bookPxMin, bookPxMax = 20, 64
	bookPxStep           = 2
	bookFlashEvery       = 8                // page turns between flashes, which clear the ghosts
	bookSaveEvery        = 20 * time.Second // the place is kept at most this often while reading
	bookShelfMax         = 500
	bookLeadingDefault   = 135 // a line's height, % of the size
	bookLeadingMin       = 110
	bookLeadingMax       = 200
	bookMarginDefault    = 3 // of 1-5: bookMargins
)

// bookMargins are the sides' margins, % of the window's width, by level.
var bookMargins = [6]int{0, 3, 5, 7, 10, 14}

// bookStyle is how books are set, as chosen in the reader's style panel
// (bookstyle.go): kept in hellobro.json, the same for every book.
type bookStyle struct {
	font            string
	px, leading     int
	margin          int
	ragged, noHyphs bool
	pdfWidth        bool // PDFs (bookpdf.go): fit their pages' width, not the whole page
	pdfNoTrim       bool // and keep their white margins
	pdfZoom         int  // and zoom them, % of the fit: each PDF's own (bookzoom.go)
}

// bookPageModes are the ways a book's pages show (each book its own,
// kept with its place): one, two side by side, or two if the window is
// wider than tall; and their switches' words.
var (
	bookPageModes = []string{"one", "two", "auto"}
	bookPageWords = map[string]string{"one": "one page", "two": "two side by side", "auto": "two if wide"}
)

// nextPages is the mode d (1 or -1; 0: 1) on from pages.
func nextPages(pages string, d int) string {
	if d == 0 {
		d = 1
	}
	n := len(bookPageModes)
	return bookPageModes[(slices.Index(bookPageModes, pages)+d+n)%n]
}

const bookGutter = 48 // px between two pages side by side

func (a *bro) bookStyle() bookStyle {
	m := &a.mem
	st := bookStyle{font: a.bookFaces().fam.name, px: a.bookPx(), leading: m.BookLeading, margin: m.BookMargin, ragged: m.BookRagged, noHyphs: m.BookNoHyph, pdfWidth: m.PDFWidth, pdfNoTrim: m.PDFNoTrim, pdfZoom: pdfZoomMin}
	if st.leading < bookLeadingMin || st.leading > bookLeadingMax {
		st.leading = bookLeadingDefault
	}
	if st.margin < 1 || st.margin >= len(bookMargins) {
		st.margin = bookMarginDefault
	}
	return st
}

// setBookStyle keeps st and sets the books again in it.
func (a *bro) setBookStyle(st bookStyle) {
	m := &a.mem
	if st.font != a.bookFaces().fam.name {
		a.bk.faces = nil
	}
	m.BookFont, m.BookPx, m.BookLeading, m.BookMargin, m.BookRagged, m.BookNoHyph = st.font, st.px, st.leading, st.margin, st.ragged, st.noHyphs
	m.PDFWidth, m.PDFNoTrim = st.pdfWidth, st.pdfNoTrim
	a.mem.save()
	for _, win := range a.windows() {
		if w, ok := win.(*bookWin); ok && w.b != nil {
			w.place(a, w.p)
			if a.shows(w) {
				w.draw(a)
			}
		}
	}
}

// bookPlace is where a book was left.
type bookPlace struct {
	Title string `json:"title,omitempty"`
	Name  string `json:"name,omitempty"` // given on the shelf (bookrename.go)
	Spine int    `json:"spine"`
	Block int    `json:"block"`
	Off   int    `json:"off"`
	Pct   int    `json:"pct"`  // how far in, for the shelf
	Read  int64  `json:"read"` // when, Unix seconds

	Marks []bookMark `json:"marks,omitempty"` // bookmarks.go
	Pages string     `json:"pages,omitempty"` // how its pages show: bookPageModes; "" one
	Zoom  int        `json:"zoom,omitempty"`  // a PDF's, % of the fit (bookzoom.go)
}

// bookState is what the readers share: the fonts, the size.
type bookState struct {
	fams  []bookFamily
	faces *bookFaces
	names map[string]font.Face // each family's regular, to show it by (bookstyle.go)
	cut   *bookCut             // a gesture cutting a part of a PDF's page out (bookcut.go)
}

type bookLaid struct {
	w, h int // a page's: half the window's width, less the gutter, if two show
	st   bookStyle
	two  bool // two pages side by side
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
	path      string // "": the shelf
	b         *book.Book
	pdf       *pdf.Doc // a PDF's: b is made of it, a chapter a page (bookpdf.go)
	shown     []pdfShown
	ahead     *pdfAhead       // a page being drawn ahead
	tiles     []image.Point   // where each screenful starts in the page drawn
	view      pdfView         // the page a cut is on, or would be: the left one of two
	views     []pdfView       // the pages showing
	cut       image.Rectangle // a part cut out, on the screen (bookcut.go)
	cutRows   []bookRow       // its buttons
	cutLabels []string        // and what they say, to show one pressed
	bar       image.Rectangle // the bar along the top of a page (toolbar.go), and its buttons as last drawn
	btns      []barBtn
	selecting bool         // the select tool: taps place the cut's corners
	corner    *image.Point // the first corner, placed
	grab      int          // a corner of the cut picked, to be moved: 1–4; 0 none
	err       string
	opening   bool           // the book's being opened (newBookWin)
	laying    *bookLayingOut // an EPUB's chapter being laid out, aside
	pagesFor  bookLayingOut  // what the pages are laid out for
	pageShown bool           // the page is what the window shows: it stays while the next is laid out
	frag      string         // the anchor the contents went to, found once its chapter's laid out
	gone      bool           // the window's closed
	spine     int
	at        book.Pos // the page shown starts here
	ch        *book.Chapter
	pages     []book.Page
	page      int
	laid      bookLaid // what the pages are laid out for
	pageMode  string   // its pages: one, two, auto (bookPageModes)
	zoom      int      // a PDF's, % of the fit (bookzoom.go)
	panned    bool     // its page moved by a finger: shown from w.at, not a screenful's start

	p       kobo.Panel
	area    image.Rectangle // the text's, in the body
	turns   int
	savedAt time.Time
	pics    map[string]*image.Gray // the page's pictures, scaled

	marks      []bookMark
	list       bool              // the contents (or the shelf) show
	listMks    bool              // the list is the bookmarks, not the contents
	listPages  bool              // a PDF's pages, small (bookpages.go)
	listCols   int               // how many across, then
	thumbs     *pdfThumbs        // the pages drawn small
	thumbTiles []image.Rectangle // where they show
	styling    bool              // the style panel shows (bookstyle.go)
	sel        int
	top        int
	listN      int // the rows the list has room for
	rows       []bookRow
	shelf      []shelfBook
	looking    bool      // the shelf is being looked for
	shelfTab   int       // its tab: libRecent, libAll, libFolders
	shelfDir   string    // the folder it shows, under booksRoot
	back       string    // the book it was opened from, to go back to
	named      string    // the name it was given, if it was (bookrename.go)
	note       *bookNote // a link's target over the page (booknote.go)
	from       *bookFrom // where a link was followed from
	ex         *exLine   // a ":" command being typed (bookex.go)
	found      *bookFind // the word looked for last
	finding    time.Time // the look going on, if it's still wanted
	edit       *noteEdit // a shelf's book's name being typed: key its path
}

func (w *bookWin) Title() string {
	switch {
	case w.path == "":
		return "Books"
	case w.named != "":
		return w.named
	case w.b != nil && w.b.Title != "":
		t, _, _ := strings.Cut(w.b.Title, " / ") // Gutenberg's: "Alice… / Illustrated by…"
		return t
	}
	return strings.TrimSuffix(filepath.Base(w.path), filepath.Ext(w.path))
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
		w.shelfTab, w.shelfDir = max(0, min(libFolders, a.mem.ShelfTab)), a.mem.ShelfDir
		a.findBooks(w)
		return w
	}
	w.opening, w.named = true, a.mem.Books[path].Name
	go func() { // a big book, or a damaged PDF MuPDF reads whole, takes a while: the screen doesn't wait
		t := time.Now()
		o := openBookFile(path)
		log.Printf("book: %s opened in %v", filepath.Base(path), time.Since(t).Round(time.Millisecond))
		a.Do(func() { w.opened(a, o) })
	}()
	return w
}

// bookOpened is a book as opened, away from the main loop.
type bookOpened struct {
	b   *book.Book
	pdf *pdf.Doc // a PDF's
	err error
}

// openBookFile opens the book at path: an EPUB, or a PDF as a book.
func openBookFile(path string) bookOpened {
	if isPDF(path) {
		d, b, err := openPDF(path)
		return bookOpened{b: b, pdf: d, err: err}
	}
	b, err := book.Open(path)
	return bookOpened{b: b, err: err}
}

// opened puts the book opened in w, where it was left, and shows it.
func (w *bookWin) opened(a *bro, o bookOpened) {
	w.opening = false
	if w.gone { // closed while it opened
		if o.pdf != nil {
			o.pdf.Close()
		} else if o.b != nil {
			o.b.Close()
		}
		return
	}
	if o.err != nil {
		w.err = o.err.Error()
		log.Printf("book: %s: %v", w.path, o.err)
	} else {
		b := o.b
		w.b, w.pdf = b, o.pdf
		w.loadPageMode(a)
		if w.pdf != nil {
			w.loadZoom(a)
		}
		if pl, ok := a.mem.Books[w.path]; ok {
			if pl.Spine < len(b.Spine) {
				w.spine, w.at = pl.Spine, book.Pos{Block: pl.Block, Off: pl.Off}
				w.panned = w.pdf != nil && w.zoom > pdfZoomMin // zoomed: just where it was, maybe moved by a finger
			}
			w.marks = slices.DeleteFunc(slices.Clone(pl.Marks), func(m bookMark) bool { return m.Spine >= len(b.Spine) })
		}
	}
	w.place(a, w.p) // its title, its margins
	a.wmBar()
	if a.shows(w) {
		w.draw(a)
	}
}

// loadPageMode takes the book's way of showing its pages, as kept.
func (w *bookWin) loadPageMode(a *bro) {
	w.pageMode = "one"
	if pl, ok := a.mem.Books[w.path]; ok && slices.Contains(bookPageModes, pl.Pages) {
		w.pageMode = pl.Pages
	}
}

// setPageMode shows the book's pages as mode says, and keeps it.
func (w *bookWin) setPageMode(a *bro, mode string) {
	w.pageMode = mode
	w.laid = bookLaid{} // laid out again
	w.save(a)
	w.draw(a)
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
	w.rows, w.btns = nil, nil
	w.pageShown = false
	w.dropCut()
	w.note = nil
	if p.Outer.Empty() {
		return
	}
	body := p.Body
	mx, my := max(16, body.Dx()*bookMargins[a.bookStyle().margin]/100), max(20, body.Dy()/40)
	if w.pdf != nil { // its own margins are enough
		mx, my = 8, 8
	}
	in := body.Inset(kobo.Pad / 2)
	w.bar = image.Rect(in.Min.X, in.Min.Y, in.Max.X, in.Min.Y+barHeight())
	w.area = image.Rect(body.Min.X+mx, w.bar.Max.Y+my, body.Max.X-mx, body.Max.Y-my)
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
	return fmt.Sprintf("%d%%", w.pct()) // the page and the chapter are on the bar
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
	laid := bookLaid{w: w.area.Dx(), h: w.area.Dy(), st: a.bookStyle()}
	if mode := w.pageMode; mode == "two" || mode == "auto" && w.area.Dx() > w.area.Dy() {
		laid.two, laid.w = true, (w.area.Dx()-bookGutter)/2
		laid.st.pdfWidth, laid.st.pdfZoom = false, 100 // a PDF's pages whole, each
	} else if w.pdf != nil {
		laid.st.pdfZoom = w.zoom
	}
	if w.ch != nil && laid == w.laid {
		return
	}
	if w.pdf != nil {
		w.layoutPDF(a, laid)
		return
	}
	want := bookLayingOut{spine: w.spine, laid: laid}
	if w.ch != nil && w.pagesFor == want { // laid out: gone to a place in it, its page
		w.laid = laid
		w.findPage()
		return
	}
	if w.laying != nil && *w.laying == want { // on its way
		return
	}
	// An EPUB's chapter is laid out aside: a long one takes the Kobo
	// most of a second, and the keys and the other windows don't wait.
	w.laying = &want
	b, ch := w.b, w.ch
	if w.pagesFor.spine != w.spine {
		ch = nil
	}
	st, faces := laid.st.settings(image.Pt(laid.w, laid.h), b.Lang), faces.clone()
	go func() {
		t := time.Now()
		var err error
		if ch == nil {
			if ch, err = b.Chapter(want.spine); err != nil {
				ch = &book.Chapter{}
			}
		}
		pages := book.Layout(ch, st, faces, func(src string) image.Point {
			p, _ := b.ImageSize(src)
			return p
		})
		log.Printf("book: chapter %d: %d pages in %v", want.spine, len(pages), time.Since(t).Round(time.Millisecond))
		a.Do(func() { w.laidOut(a, want, ch, pages, err) })
	}()
}

// bookLayingOut is what a chapter's pages are laid out for.
type bookLayingOut struct {
	spine int
	laid  bookLaid
}

// laidOut takes the pages laid out aside, if they're still the ones
// wanted, and shows them.
func (w *bookWin) laidOut(a *bro, want bookLayingOut, ch *book.Chapter, pages []book.Page, err error) {
	if w.gone || w.laying == nil || *w.laying != want { // closed, or other pages are wanted now
		return
	}
	w.laying = nil
	w.err = ""
	if err != nil {
		w.err = err.Error()
	}
	w.ch, w.pages, w.pagesFor, w.laid = ch, pages, want, want.laid
	w.findPage()
	if w.page < len(w.pages) && w.at.Block >= len(ch.Blocks) { // its last page: from where it starts
		w.at = w.pages[w.page].Start
	}
	if a.shows(w) {
		w.draw(a)
	}
}

// findPage finds the page w.at is on, in the pages laid out (or the
// anchor the contents went to).
func (w *bookWin) findPage() {
	if w.frag != "" {
		if bi, ok := w.ch.IDs[w.frag]; ok {
			w.at = book.Pos{Block: bi}
		}
		w.frag = ""
	}
	w.page = book.PageAt(w.pages, w.at)
	if w.laid.two { // a spread starts on an even page
		w.page -= w.page % 2
	}
}

func (w *bookWin) draw(a *bro) {
	p := w.panel(a)
	if p.Outer.Empty() {
		return
	}
	reading := !w.styling && !w.list && w.b != nil
	if reading {
		w.layout(a)
		if w.laying != nil && w.pageShown { // the page shown stays till the next is laid out
			return
		}
	}
	w.pageShown = false
	a.DrawPanel(p, w.status())
	body := p.Body
	a.C.Fill(body, kobo.White)
	note := func(s string, ink uint8) {
		a.C.Text(a.F.Body, image.Rect(w.area.Min.X, w.area.Min.Y, w.area.Max.X, w.area.Min.Y+40), s, ink, kobo.AlignLeft)
	}
	switch {
	case w.styling:
		w.drawStyle(a)
	case w.list:
		w.drawList(a)
	case w.opening, reading && w.laying != nil && w.pagesFor == bookLayingOut{}:
		note("opening "+filepath.Base(w.path)+"…", kobo.Mid)
	case reading && w.laying != nil:
		note("laying out…", kobo.Mid)
	case w.err != "" && w.b == nil:
		note("can't read it: "+w.err, kobo.Dark)
	default:
		w.drawPage(a)
		w.pageShown = true
	}
	if w.ex != nil {
		a.drawEx(a.exStrip(body), w.ex, w.exCmds(a))
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
	if w.pdf != nil {
		w.drawPDF(a)
		return
	}
	pics := map[string]*image.Gray{}
	w.drawLaid(a, w.page, w.area.Min, pics)
	if w.laid.two && w.page+1 < len(w.pages) {
		w.drawLaid(a, w.page+1, w.area.Min.Add(image.Pt(w.laid.w+bookGutter, 0)), pics)
	}
	w.pics = pics // only these pages' are kept
	w.drawPageMark(a)
	w.drawBar(a)
	w.drawNote(a)
}

// drawLaid draws page n of the chapter with its top left at at; the
// pictures drawn go in pics.
func (w *bookWin) drawLaid(a *bro, n int, at image.Point, pics map[string]*image.Gray) {
	faces := a.bookFaces()
	for _, it := range w.pages[n].Items {
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

// shelfItems are the shelf's rows, as its tab and folder have them.
func (w *bookWin) shelfItems() []libItem {
	root := booksRoot()
	var es []libEntry
	for _, b := range w.shelf {
		rel, err := filepath.Rel(root, b.path)
		if err != nil {
			rel = filepath.Base(b.path)
		}
		e := libEntry{key: b.path, rel: filepath.ToSlash(rel), name: b.name, at: b.read}
		if b.read > 0 {
			e.note = fmt.Sprintf("%d%%", b.pct)
		}
		es = append(es, e)
	}
	return libView(es, nil, w.shelfTab, w.shelfDir)
}

// shelfGo shows the shelf's tab, in folder dir; the choice at the top.
func (w *bookWin) shelfGo(a *bro, tab int, dir string) {
	w.shelfTab, w.shelfDir, w.sel, w.top = tab, dir, 0, 0
	a.mem.ShelfTab, a.mem.ShelfDir = tab, dir
	a.mem.save()
	w.draw(a)
}

// toShelf puts the shelf where the book is, to come back to it.
func (w *bookWin) toShelf(a *bro) {
	w.save(a)
	s := a.newBookWin("")
	s.back = w.path
	a.replaceWin(w, s)
}

// fromShelf goes back to the book the shelf was opened from.
func (w *bookWin) fromShelf(a *bro) {
	if w.back != "" {
		a.replaceWin(w, a.newBookWin(w.back))
	}
}

func (w *bookWin) listLines() []string {
	var ls []string
	if w.path == "" {
		for _, it := range w.shelfItems() {
			ls = append(ls, it.name)
		}
		return ls
	}
	if w.listMks {
		return w.markLines()
	}
	for _, e := range w.b.TOC {
		ls = append(ls, strings.Repeat("    ", min(e.Depth, 4))+e.Title)
	}
	return ls
}

func (w *bookWin) listOpen(a *bro, i int) {
	if w.path == "" {
		items := w.shelfItems()
		switch {
		case i >= len(items):
		case items[i].dir:
			w.shelfGo(a, libFolders, items[i].key)
		default:
			a.openBook(w, items[i].key)
		}
		return
	}
	if w.listPages {
		w.openThumb(a, i)
		return
	}
	if w.listMks {
		if i < len(w.marks) {
			w.goToMark(a, i)
		}
		return
	}
	if i >= len(w.b.TOC) {
		return
	}
	e := w.b.TOC[i]
	w.list = false
	w.goTo(e.Spine, book.Pos{})
	w.frag = e.Frag
	w.draw(a)
}

func (w *bookWin) drawList(a *bro) {
	if w.path == "" {
		w.drawShelf(a)
		return
	}
	lines := w.listLines()
	f := a.F.Body
	rowH := max(64, kobo.TextWidth(f, "M")*2+10) // room for a finger
	area := w.p.Body.Inset(kobo.Pad)
	w.rows = nil
	if w.b != nil {
		w.drawTabs(a, image.Rect(area.Min.X, area.Min.Y, area.Max.X, area.Min.Y+rowH))
		area.Min.Y += rowH + 12
	}
	if w.listPages {
		w.drawThumbs(a, area)
		return
	}
	n := max(1, area.Dy()/rowH)
	w.listN = n
	if len(lines) == 0 {
		msg := "no contents in this book"
		if w.listMks {
			msg = "no bookmarks yet: b, or the ribbon over the page"
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
	for k := 0; k < n && w.top+k < len(lines); k++ {
		i := w.top + k
		r := image.Rect(area.Min.X, area.Min.Y+k*rowH, area.Max.X, area.Min.Y+(k+1)*rowH)
		ink := uint8(kobo.Black)
		if i == w.sel {
			a.C.Fill(r, kobo.Black)
			ink = kobo.White
		}
		text := r.Inset(8)
		if w.listMks { // a box to take it away, at the right
			x := image.Rect(r.Max.X-rowH, r.Min.Y, r.Max.X, r.Max.Y).Inset(6)
			a.C.Frame(x, 2, ink)
			a.C.Text(f, x, "×", ink, kobo.AlignCenter)
			text.Max.X = x.Min.X - 12
			w.rows = append(w.rows, bookRow{x, func() { w.deleteMark(a, i) }})
		}
		a.C.Text(f, text, lines[i], ink, kobo.AlignLeft)
		w.rows = append(w.rows, bookRow{r, func() { w.listOpen(a, i) }})
	}
}

// drawShelf draws the shelf: the header, its tab's rows.
func (w *bookWin) drawShelf(a *bro) {
	rowH := a.libRowH()
	area := w.p.Body.Inset(kobo.Pad)
	back := ""
	if w.back != "" {
		back = "back to the book"
	}
	h := a.drawLibHeader(image.Rect(area.Min.X, area.Min.Y, area.Max.X, area.Min.Y+rowH), "All Books", true,
		func() { w.shelfGo(a, w.shelfTab, "") }, back, func() { w.fromShelf(a) },
		libTabs(w.shelfTab, func(k int) { w.shelfGo(a, k, w.shelfDir) }))
	w.rows = h.rows
	area.Min.Y += rowH + 12
	items := w.shelfItems()
	n := max(1, area.Dy()/rowH)
	w.listN = n
	if len(items) == 0 {
		msg := "no books here: put EPUBs or PDFs on the Kobo (anywhere but its dot folders)"
		if w.looking {
			msg = "looking for books…"
		}
		a.C.Text(a.F.Body, image.Rect(area.Min.X, area.Min.Y, area.Max.X, area.Min.Y+rowH), msg, kobo.Dark, kobo.AlignLeft)
		return
	}
	w.sel = max(0, min(w.sel, len(items)-1))
	if w.sel < w.top {
		w.top = w.sel
	} else if w.sel >= w.top+n {
		w.top = w.sel - n + 1
	}
	w.top = max(0, min(w.top, len(items)-n))
	for k := 0; k < n && w.top+k < len(items); k++ {
		i := w.top + k
		r := image.Rect(area.Min.X, area.Min.Y+k*rowH, area.Max.X, area.Min.Y+(k+1)*rowH)
		it := items[i]
		if e := w.edit; e != nil && e.key == it.key && !it.dir {
			w.rows = append(w.rows, a.drawNameEdit(r, "name: ", e.text, func() { w.endRename(a, true) }, func() { w.endRename(a, false) })...)
			continue
		}
		var btns []libTab
		if !it.dir {
			btns = []libTab{{"rename", false, func() { w.startRename(a, it.key, it.name) }}}
		}
		w.rows = append(w.rows, a.drawLibRow(r, it, i == w.sel, func() { w.listOpen(a, i) }, btns...)...)
	}
}

// openBook puts the book at path in the shelf's window.
func (a *bro) openBook(w *bookWin, path string) {
	nw := a.newBookWin(path)
	a.replaceWin(w, nw)
}

// booksRoot is where the books are looked for: the Kobo's storage.
func booksRoot() string {
	if root := os.Getenv("HELLOBRO_BOOKS"); root != "" {
		return root
	}
	return "/mnt/onboard"
}

// findBooks looks for the EPUBs on the Kobo, away from the main loop.
func (a *bro) findBooks(w *bookWin) {
	root := booksRoot()
	w.looking = true
	known := maps.Clone(a.mem.Books) // the walk's own: a book's place is written meanwhile
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
			if (strings.HasSuffix(strings.ToLower(d.Name()), ".epub") || isPDF(d.Name())) && len(found) < bookShelfMax {
				b := shelfBook{path: p, name: shelfName(d.Name())}
				if pl, ok := known[p]; ok {
					b.pct, b.read, b.name = pl.Pct, pl.Read, placeName(p, pl)
				}
				found = append(found, b)
			}
			return nil
		})
		a.Do(func() {
			w.shelf, w.looking = found, false
			if a.shows(w) {
				w.draw(a)
			}
		})
	}()
}

// shelfName is a book's name from its file's: ".epub", ".kepub.epub"
// and ".pdf" off, however they're written.
func shelfName(file string) string {
	for _, ext := range []string{".epub", ".kepub", ".pdf"} {
		if strings.HasSuffix(strings.ToLower(file), ext) {
			file = file[:len(file)-len(ext)]
		}
	}
	return file
}

// bookExit keeps where each book open is, for quitting and sleep: while
// reading, a place is only kept every 20 s.
func (a *bro) bookExit() {
	for _, w := range a.windows() {
		if b, ok := w.(*bookWin); ok {
			b.save(a)
		}
	}
}

// --- turning pages ---

// goTo shows chapter spine at pos.
func (w *bookWin) goTo(spine int, pos book.Pos) {
	if spine != w.spine {
		w.ch, w.pics, w.pages = nil, nil, nil
	}
	w.dropCut()
	w.note = nil
	w.spine, w.at, w.panned = spine, pos, false
	w.laid = bookLaid{}
}

// turn goes d pages on (back if negative), across chapters.
func (w *bookWin) turn(a *bro, d int) {
	if w.b == nil || len(w.pages) == 0 || w.laying != nil { // the next pages aren't laid out yet
		return
	}
	w.note = nil
	two, step := w.laid.two, 1 // two side by side: a PDF's chapters, its pages, go two at a time too
	if two && (d == 1 || d == -1) {
		d *= 2
	}
	if two && w.pdf != nil {
		step = 2
	}
	switch n := w.page + d; {
	case w.panned && (d == 1 || d == -1) && w.panStep(d): // moved by a finger: a screenful on from there
	case n >= len(w.pages):
		if w.spine+step >= len(w.b.Spine) {
			return
		}
		w.goTo(w.spine+step, book.Pos{})
	case n < 0:
		if w.spine == 0 {
			return
		}
		w.goTo(max(0, w.spine-step), book.Pos{Block: 1 << 30}) // its last page
	default:
		if two {
			n -= n % 2
		}
		w.page, w.panned = n, false
		w.at = w.pages[n].Start
		w.dropCut()
	}
	w.turns++
	w.draw(a)
	if w.ch != nil && w.page < len(w.pages) && !w.panned {
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
	pct := w.pct()
	if len(w.pages) == 0 { // its chapter's being laid out: as far as it was
		pct = a.mem.Books[w.path].Pct
	}
	a.mem.Books[w.path] = bookPlace{Title: w.b.Title, Name: w.named, Spine: w.spine, Block: w.at.Block, Off: w.at.Off, Pct: pct, Read: time.Now().Unix(), Marks: w.marks, Pages: w.pageMode, Zoom: w.zoom}
	a.mem.save()
	w.savedAt = time.Now()
}

// restyle sets the books in another size or font, from a key: the
// note says which.
func (a *bro) restyle(px int, font string) {
	st := a.bookStyle()
	st.px, st.font = px, font
	a.setBookStyle(st)
	a.Notify(fmt.Sprintf("%s · %d px", a.bookFaces().fam.name, a.bookPx()))
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
	if w.styling {
		return w.styleKey(a, code, ch)
	}
	if e := w.edit; e != nil {
		if done, ok := nameKey(&e.text, code, ch); done {
			w.endRename(a, ok)
		} else {
			w.draw(a)
		}
		return true
	}
	if e := w.ex; e != nil {
		done, ok := exKey(e, code, ch)
		if !done {
			a.drawEx(a.exStrip(w.p.Body), e, w.exCmds(a))
			return true
		}
		w.ex = nil
		a.vkAsk(false)
		if ok {
			a.exRun(w.exCmds(a), e.text, func(text string) { w.exOpen(a, text) })
		}
		if !w.gone && !a.overlay() { // :help's menu stays
			w.draw(a)
		}
		return true
	}
	if ch == ":" && w.b != nil && !w.styling {
		w.exOpen(a, "")
		return true
	}
	if ch == "q" && w.b != nil { // the book put away: the shelf
		w.toShelf(a)
		return true
	}
	if ch == "?" && w.b != nil {
		a.showSheet(w.Title()+": keys and commands", bookKeys(w.pdf != nil), w.exCmds(a))
		return true
	}
	if w.list && w.listPages {
		by := map[uint16]int{103: -w.listCols, 108: w.listCols, 105: -1, 106: 1, 104: -w.listN, 109: w.listN}
		switch {
		case by[code] != 0:
			w.sel += by[code]
		case code == kobo.KeyEnter:
			w.listOpen(a, w.sel)
			return true
		case code == kobo.KeyEsc, ch == "t", ch == "o":
			w.list = false
		case code == kobo.KeyTab:
			w.showList(a, false)
			return true
		case ch == "b":
			w.showList(a, true)
			return true
		default:
			return false
		}
		w.draw(a)
		return true
	}
	if w.list {
		switch {
		case code == 103 || ch == "k": // up
			w.sel--
		case code == 108 || ch == "j":
			w.sel++
		case code == 104: // PgUp
			w.sel -= max(1, w.listN)
		case code == 109:
			w.sel += max(1, w.listN)
		case code == kobo.KeyEnter:
			w.listOpen(a, w.sel)
			return true
		case code == kobo.KeyTab && w.path == "": // the shelf's next tab
			w.shelfGo(a, (w.shelfTab+1)%len(libTabNames), w.shelfDir)
			return true
		case code == kobo.KeyBackspace && w.path == "" && w.shelfTab == libFolders && w.shelfDir != "": // up a folder
			w.listOpen(a, 0)
			return true
		case code == kobo.KeyEsc && w.path == "" && w.back != "":
			w.fromShelf(a)
			return true
		case code == 60 && w.path == "": // F2: rename
			if items := w.shelfItems(); w.sel < len(items) && !items[w.sel].dir {
				w.startRename(a, items[w.sel].key, items[w.sel].name)
			}
			return true
		case code == kobo.KeyEsc && w.b != nil, ch == "t" && w.b != nil:
			w.list = false
		case code == kobo.KeyTab && w.b != nil && w.listMks && w.pdf != nil, ch == "o" && w.pdf != nil:
			w.showPages(a)
			return true
		case code == kobo.KeyTab && w.b != nil:
			w.showList(a, !w.listMks)
			return true
		case ch == "b" && w.b != nil:
			w.showList(a, true)
			return true
		case (ch == "d" || code == 111) && w.listMks: // Delete
			w.deleteMark(a, w.sel)
			return true
		default:
			return false
		}
		w.draw(a)
		return true
	}
	if n := w.note; n != nil { // Enter goes there, Esc closes it, another key too and goes on
		w.note = nil
		switch {
		case code == kobo.KeyEnter && n.page != nil:
			w.goThere(a, n.link)
			return true
		case code == kobo.KeyEsc:
			w.draw(a)
			return true
		}
		w.draw(a)
	}
	switch {
	case code == kobo.KeyEsc && (!w.cut.Empty() || w.corner != nil):
		w.dropCut()
		w.draw(a)
	case code == kobo.KeyEsc && w.selecting: // nothing cut: the tool put down
		w.selecting = false
		w.draw(a)
	case (ch == "+" || ch == "=") && w.pdf != nil:
		w.zoomStep(a, 1)
	case ch == "-" && w.pdf != nil:
		w.zoomStep(a, -1)
	case code == kobo.KeySpace || code == 106 || code == 109 || code == 108 || ch == "j" || ch == "l" || ch == "n":
		w.turn(a, 1)
	case code == 105 || code == 104 || code == 103 || code == kobo.KeyBackspace || ch == "k" || ch == "h" || ch == "p":
		w.turn(a, -1)
	case code == 102: // Home: the chapter's start
		w.turn(a, -w.page)
	case code == 107: // End: the chapter's last page, of two side by side the last two
		last := len(w.pages) - 1
		if w.laid.two {
			last -= last % 2
		}
		if last != w.page {
			w.turn(a, last-w.page) // of two, an even step: turn doubles only one
		}
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
	case ch == "s" && w.b != nil:
		w.openStyle(a)
	case ch == "b" && w.b != nil:
		w.toggleMark(a)
	case ch == "o" && w.pdf != nil:
		w.showPages(a)
	case code == kobo.KeyEsc:
		w.toShelf(a)
	default:
		return false
	}
	return true
}

// openContents shows the book's contents, the chapter shown picked;
// the bookmarks if it has no contents.
func (w *bookWin) openContents(a *bro) {
	if w.b != nil {
		w.showList(a, len(w.b.TOC) == 0)
	}
}

// tap: on the list or the style panel, a row; on a page, the left
// third back, the rest on, the top the contents but for its right
// corner, a bookmark; the bar over it, its buttons (bookmarks.go).
func (w *bookWin) tap(a *bro, p image.Point) {
	body := w.p.Body
	if !p.In(body) {
		return
	}
	if w.ex != nil { // a tap elsewhere leaves the line
		w.ex = nil
		a.vkAsk(false)
		w.draw(a)
		return
	}
	if w.list || w.styling {
		for _, r := range w.rows {
			if p.In(r.r) {
				r.do()
				return
			}
		}
		return
	}
	if w.selecting && w.pdf != nil {
		if !(p.In(w.bar) && w.barTap(a, p)) {
			w.selectTap(a, p)
		}
		return
	}
	if w.note != nil {
		w.noteTap(a, p)
		return
	}
	if !w.cut.Empty() { // a part cut out: its buttons, or away with it
		for i, r := range w.cutRows {
			if p.In(r.r) {
				w.pressCut(a, i)
				return
			}
		}
		w.dropCut()
		w.draw(a)
		return
	}
	link := w.linkAt(p)
	switch {
	case p.In(w.bar) && w.b != nil:
		w.barTap(a, p)
	case p.In(w.markCorner()) && w.b != nil:
		w.toggleMark(a)
	case link != "":
		w.followLink(a, link)
	case p.Y < w.area.Min.Y+w.area.Dy()/8:
		w.openContents(a)
	case p.X < body.Min.X+body.Dx()/3:
		w.turn(a, -1)
	default:
		w.turn(a, 1)
	}
}

func (w *bookWin) closing(a *bro) {
	if w.ex != nil {
		w.ex = nil
		a.vkAsk(false)
	}
	if w.edit != nil {
		w.edit = nil
		a.vkAsk(false)
	}
	w.gone = true
	w.save(a)
	if w.pdf != nil {
		w.pdf.Close()
	} else if w.b != nil {
		w.b.Close()
	}
	w.place(a, kobo.Panel{})
}
