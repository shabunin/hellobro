package main

// A word held on an EPUB's page, by a finger or the pen: what the
// dictionaries say of it, in a box like a note's, away from the word.
// The dictionaries are StarDict's, in Dictionaries/ on the Kobo's storage,
// looked for at each look up, so one copied there needs no restart. All
// of them are asked (books are in two languages, or say they're in the
// wrong one), the ones for the book's language first; it's done aside, as
// one without its index has to make it, which takes a while.

import (
	"errors"
	"fmt"
	"image"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/image/font"

	"hellobro/book"
	"hellobro/dict"
	"hellobro/kobo"
)

const (
	holdSlop     = 24                     // px a finger holding a word may wander
	dictPerDict  = 4                      // articles from a dictionary
	dictWaitShow = 400 * time.Millisecond // longer than this, the box says it's looking
)

// dictsDir is where the dictionaries are.
func dictsDir() string {
	if d := os.Getenv("HELLOBRO_DICTS"); d != "" {
		return d
	}
	return filepath.Join(shareRoot, "Dictionaries")
}

// dictShelf keeps the dictionaries opened: each reads its index only once.
type dictShelf struct {
	mu    sync.Mutex
	dicts map[string]*dict.Dict
}

var dictsOpen dictShelf

// find is the dictionaries in dir now.
func (s *dictShelf) find(dir string) []*dict.Dict {
	found, err := dict.Find(dir)
	if err != nil {
		log.Printf("dict: %v", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dicts == nil {
		s.dicts = map[string]*dict.Dict{}
	}
	for i, d := range found {
		if old := s.dicts[d.Path()]; old != nil {
			found[i] = old
		} else {
			s.dicts[d.Path()] = d
		}
	}
	return found
}

// dictsForLang orders the dictionaries: the language's first, "grc" for
// "grc-en"; the others as they are. A book's lang may be "en-US".
func dictsForLang(ds []*dict.Dict, lang string) []*dict.Dict {
	lang, _, _ = strings.Cut(strings.ToLower(lang), "-")
	lang, _, _ = strings.Cut(lang, "_")
	first := func(d *dict.Dict) bool {
		from, _, _ := strings.Cut(strings.ToLower(d.Base), "-")
		return lang != "" && from == lang
	}
	out := slices.Clone(ds)
	slices.SortStableFunc(out, func(a, b *dict.Dict) int {
		switch {
		case first(a) && !first(b):
			return -1
		case first(b) && !first(a):
			return 1
		}
		return 0
	})
	return out
}

// lookUp asks each dictionary. What one fails with doesn't stop the others.
func lookUp(ds []*dict.Dict, word string) ([]dict.Entry, error) {
	var out []dict.Entry
	var errs []error
	for _, d := range ds {
		es, err := d.Lookup(word, dictPerDict)
		if err != nil {
			errs = append(errs, err)
		}
		out = append(out, es...)
	}
	return out, errors.Join(errs...)
}

// entryBlocks are an article as the box shows it: its word, whose it is
// (if there's more than one dictionary to tell), then what it says.
func entryBlocks(e dict.Entry, first bool) []book.Block {
	head := book.Block{Align: book.Left, Head: 3, Spans: []book.Span{ // a heading: it stays with its first lines
		{Text: e.Word, Style: book.Bold, Scale: 1.1},
		{Text: "  " + e.Dict, Style: book.Italic, Scale: 0.8},
	}}
	if !first {
		head.Above = 0.7
	}
	out := []book.Block{head}
	for _, f := range e.Fields {
		switch f.Type {
		case 'h', 'g', 'x':
			out = append(out, book.ParseHTML(f.Text)...)
		case 't', 'y': // how it sounds, how it's written
			out = append(out, book.Block{Align: book.Left, Spans: []book.Span{{Text: f.Text, Style: book.Italic, Scale: 1}}})
		case 'm', 'l':
			for _, line := range strings.Split(f.Text, "\n") {
				if line = strings.TrimSpace(line); line != "" {
					out = append(out, book.Block{Align: book.Left, Spans: []book.Span{{Text: line, Scale: 1}}})
				}
			}
		}
	}
	return out
}

// isWordRune says if r can be in a word, wherever in it.
func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Mc, r) || unicode.IsDigit(r) || r == '­'
}

func isHyphen(r rune) bool { return r == '-' || r == '‐' || r == '‑' }

// wordsIn are where the words of a line's text are, in bytes: a hyphen
// between letters is in its word (кто-то), an apostrophe after a letter
// too (l'homme, ἀλλ᾽).
func wordsIn(s string) [][2]int {
	var out [][2]int
	start, end := -1, 0
	for i, r := range s {
		n := i + utf8.RuneLen(r)
		switch {
		case isWordRune(r):
			if start < 0 {
				start = i
			}
			end = n
		case start >= 0 && dict.IsApostrophe(r) && end == i:
			end = n
		case start >= 0 && isHyphen(r) && end == i:
			if next, _ := utf8.DecodeRuneInString(s[n:]); isWordRune(next) {
				end = n
			} else {
				out, start = append(out, [2]int{start, end}), -1
			}
		case start >= 0:
			out, start = append(out, [2]int{start, end}), -1
		}
	}
	if start >= 0 {
		out = append(out, [2]int{start, end})
	}
	return out
}

// lastWord and firstWord are the word text ends with, starts with; "" if
// it ends or starts with something else.
func lastWord(s string) string {
	ws := wordsIn(s)
	if len(ws) == 0 || ws[len(ws)-1][1] != len(s) {
		return ""
	}
	return s[ws[len(ws)-1][0]:]
}

func firstWord(s string) string {
	ws := wordsIn(s)
	if len(ws) == 0 || ws[0][0] != 0 {
		return ""
	}
	return s[:ws[0][1]]
}

// trimHyphen is s without the hyphen it ends with; false if it hasn't one.
func trimHyphen(s string) (string, bool) {
	r, n := utf8.DecodeLastRuneInString(s)
	return s[:len(s)-n], n > 0 && isHyphen(r)
}

// pageWord is the word at bytes [from, to) of item i's text on a page, all
// of it: a word's parts may be in the items next to it (a span's end in
// the middle of it), and it goes on on the next line after a hyphen.
func pageWord(items []book.Item, i, from, to int) string {
	it := items[i]
	word := it.Text[from:to]
	for k := i; k > 0 && from == 0; k-- { // leftwards, while it touches
		p, c := items[k-1], items[k]
		if p.Kind != book.ItemText {
			break
		}
		if p.Y == c.Y && kobo.Abs(p.X+p.W-c.X) <= 2 {
			w := lastWord(p.Text)
			if w == "" {
				break
			}
			word = w + word
			if len(w) != len(p.Text) {
				break
			}
			continue
		}
		if p.Y < c.Y { // the line before, if it ends cut short
			if t, ok := trimHyphen(p.Text); ok {
				word = lastWord(t) + word
			}
		}
		break
	}
	if to < len(it.Text) {
		if rest, ok := trimHyphen(it.Text[to:]); !ok || rest != "" {
			return word
		}
	}
	for k := i; k+1 < len(items); k++ { // rightwards
		c, n := items[k], items[k+1]
		if n.Kind != book.ItemText {
			break
		}
		if n.Y == c.Y && kobo.Abs(c.X+c.W-n.X) <= 2 {
			w := firstWord(n.Text)
			if w == "" {
				break
			}
			word += w
			if len(w) != len(n.Text) {
				break
			}
			continue
		}
		if n.Y > c.Y {
			if _, ok := trimHyphen(c.Text); ok {
				word += firstWord(n.Text)
			}
		}
		break
	}
	return word
}

// wordIn is the word in text under x px from its start, the nearest within
// reach: its bytes, and its px from the start to the end.
func wordIn(text string, f font.Face, x, reach int) (from, to, x0, x1 int, ok bool) {
	best := reach + 1
	for _, w := range wordsIn(text) {
		l, r := font.MeasureString(f, text[:w[0]]).Round(), font.MeasureString(f, text[:w[1]]).Round()
		d := 0
		if x < l {
			d = l - x
		} else if x > r {
			d = x - r
		}
		if d < best {
			from, to, x0, x1, ok, best = w[0], w[1], l, r, true, d
		}
	}
	return
}

// wordAt is the word under p on the page shown, and a line under it.
func (w *bookWin) wordAt(a *bro, p image.Point) (string, image.Rectangle, bool) {
	if w.b == nil || w.pdf != nil || w.page >= len(w.pages) {
		return "", image.Rectangle{}, false
	}
	faces := w.bookFaces(a)
	type shown struct {
		n  int
		at image.Point
	}
	pages := []shown{{w.page, w.area.Min}}
	if w.laid.two && w.page+1 < len(w.pages) {
		pages = append(pages, shown{w.page + 1, w.area.Min.Add(image.Pt(w.laid.w+bookGutter, 0))})
	}
	for _, s := range pages {
		items := w.pages[s.n].Items
		for i, it := range items {
			if it.Kind != book.ItemText || it.Text == "" {
				continue
			}
			const reach = 28 // a finger's not sure
			r := image.Rect(it.X-reach, it.Y-it.Px, it.X+it.W+reach, it.Y+it.Px/3).Add(s.at)
			if !p.In(r) {
				continue
			}
			from, to, x0, x1, ok := wordIn(it.Text, faces.Face(it.Style, it.Px), p.X-s.at.X-it.X, reach)
			if !ok {
				continue
			}
			under := image.Rect(it.X+x0, it.Y+it.Px/8, it.X+x1, it.Y+it.Px/8+3).Add(s.at)
			return pageWord(items, i, from, to), under, true
		}
	}
	return "", image.Rectangle{}, false
}

// holdWord looks up the word held at p, if there's one: true if so.
func (w *bookWin) holdWord(a *bro, p image.Point) bool {
	if w.list || w.styling || w.ex != nil || w.edit != nil || w.selecting || !w.cut.Empty() || !p.In(w.area) {
		return false
	}
	if w.note != nil && p.In(w.note.box) {
		return false
	}
	word, under, ok := w.wordAt(a, p)
	if !ok {
		return false
	}
	log.Printf("dict: looking up %q", word)
	w.note = &bookNote{word: word, under: under, top: p.Y > w.area.Min.Y+w.area.Dy()/2}
	n, dir, lang := w.note, dictsDir(), w.b.Lang
	go func() {
		ds := dictsForLang(dictsOpen.find(dir), lang)
		if len(ds) == 0 {
			a.Do(func() { w.dictRead(a, n, nil, nil, 0) })
			return
		}
		show := time.AfterFunc(dictWaitShow, func() {
			a.Do(func() { w.dictLooking(a, n) })
		})
		t := time.Now()
		es, err := lookUp(ds, word)
		show.Stop()
		log.Printf("dict: %q: %d found in %v", word, len(es), time.Since(t).Round(time.Millisecond))
		a.Do(func() { w.dictRead(a, n, es, err, len(ds)) })
	}()
	return true
}

// dictLooking says it's looking, if it's still not there.
func (w *bookWin) dictLooking(a *bro, n *bookNote) {
	if w.gone || w.note != n || n.page != nil {
		return
	}
	w.dictBox(a, n, dictMessage(n.word, "looking it up… a new dictionary takes a while, once"))
}

func dictMessage(word, msg string) []book.Block {
	return []book.Block{
		{Align: book.Left, Spans: []book.Span{{Text: word, Style: book.Bold, Scale: 1.1}}},
		{Align: book.Left, Spans: []book.Span{{Text: msg, Scale: 1}}},
	}
}

// dictRead shows what was found.
func (w *bookWin) dictRead(a *bro, n *bookNote, es []dict.Entry, err error, dicts int) {
	if w.gone || w.note != n {
		return
	}
	if err != nil {
		log.Printf("dict: %v", err)
	}
	var text []book.Block
	switch {
	case dicts == 0:
		text = dictMessage(n.word, fmt.Sprintf("no dictionaries yet: StarDict ones go in %s", filepath.Base(dictsDir())))
	case len(es) == 0 && err != nil:
		text = dictMessage(n.word, "can't read the dictionaries: the log says why")
	case len(es) == 0:
		text = dictMessage(n.word, "not in the dictionaries")
	}
	for i, e := range es {
		text = append(text, entryBlocks(e, i == 0)...)
	}
	w.dictBox(a, n, text)
}

// dictBox lays text out in the box and shows it.
func (w *bookWin) dictBox(a *bro, n *bookNote, text []book.Block) {
	st := a.bookStyle()
	st.px = max(bookPxMin, st.px*85/100)
	st.ragged, st.noHyphs = true, true
	if !w.noteBox(a, n, text, st) {
		a.Notify(n.word + ": no room for it here")
		w.note = nil
		return
	}
	if a.shows(w) {
		w.draw(a)
	}
}
