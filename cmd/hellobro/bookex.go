package main

// A book's ":" commands (excmd.go): pages, chapters, the lists and the
// panels, the size and the font, a PDF's zoom, looking for a word.

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"hellobro/book"
)

// bookFind is a word looked for: where it was found last, for :/ again.
type bookFind struct {
	text  string
	spine int
	at    book.Pos
}

func (w *bookWin) exCmds(a *bro) []exCmd {
	var cmds []exCmd
	cmds = []exCmd{
		{"page p", "N or N%", "go to page N (an EPUB's: of this chapter); N% that far into the book", func(arg string) error { return w.exPage(a, arg) }},
		{"chapter ch", "N", "go to chapter N", func(arg string) error {
			n, _, err := exNumber(arg)
			if err != nil || n < 1 || n > len(w.b.Spine) {
				return fmt.Errorf("a chapter from 1 to %d", len(w.b.Spine))
			}
			w.goTo(n-1, book.Pos{})
			w.list = false
			w.draw(a)
			return nil
		}},
		{"find / f", "word", "the next page with the word on it; :/ alone, the next again", func(arg string) error { return w.exFind(a, arg) }},
		{"contents toc", "", "the contents", func(string) error { w.openContents(a); return nil }},
		{"bookmarks marks", "", "the bookmarks", func(string) error { w.showList(a, true); return nil }},
		{"mark m", "", "a bookmark on this page, or not", func(string) error { w.toggleMark(a); return nil }},
		{"back", "", "back from a link followed", func(string) error {
			if w.from == nil {
				return errors.New("no link followed to go back from")
			}
			w.goBack(a)
			return nil
		}},
		{"style s", "", "the style panel", func(string) error { w.openStyle(a); return nil }},
		{"size", "px", "the text's size", func(arg string) error {
			n, _, err := exNumber(arg)
			if err != nil || n < bookPxMin || n > bookPxMax {
				return fmt.Errorf("a size from %d to %d", bookPxMin, bookPxMax)
			}
			a.restyle(n, a.bookFaces().fam.name)
			return nil
		}},
		{"font", "name", "the font, the one that matches best", func(arg string) error {
			best, score := "", -1
			for _, f := range a.bk.fams {
				if s := fuzzy(arg, f.name); s > score {
					best, score = f.name, s
				}
			}
			if best == "" {
				return fmt.Errorf("no font like %q", arg)
			}
			a.restyle(a.bookPx(), best)
			return nil
		}},
		{"zoom z", "%", "a PDF's zoom", func(arg string) error {
			n, _, err := exNumber(strings.TrimSuffix(arg, "%"))
			if w.pdf == nil || err != nil || n < pdfZoomMin || n > pdfZoomMax {
				return fmt.Errorf("a PDF's zoom, from %d to %d", pdfZoomMin, pdfZoomMax)
			}
			w.setZoom(a, n, w.area.Min.Add(w.area.Max).Div(2))
			return nil
		}},
		{"pages", "", "a PDF's pages, small", func(string) error {
			if w.pdf == nil {
				return errors.New(":pages is a PDF's")
			}
			w.showPages(a)
			return nil
		}},
		{"rename", "name", "the book's name, its file's too", func(arg string) error { return renameBook(a, w.path, arg) }},
		{"shelf books", "", "All Books", func(string) error { w.toShelf(a); return nil }},
		{"quit q close", "", "close the window", func(string) error { a.closeWin(w); return nil }},
	}
	return append(cmds, exCmd{"help h", "", "this list", nil})
}

// exOpen opens the ":" line, text typed on it already.
func (w *bookWin) exOpen(a *bro, text string) {
	if w.gone {
		return
	}
	w.ex, w.note = &exLine{text: text}, nil
	a.vkAsk(true)
	w.draw(a)
}

// exPage goes to page N: a PDF's, or of the chapter shown in an EPUB;
// N% goes that far into the book.
func (w *bookWin) exPage(a *bro, arg string) error {
	n, pct, err := exNumber(arg)
	if err != nil {
		return err
	}
	switch {
	case pct:
		w.goTo(min(len(w.b.Spine)-1, n*len(w.b.Spine)/100), book.Pos{})
	case w.pdf != nil:
		if n < 1 || n > len(w.b.Spine) {
			return fmt.Errorf("a page from 1 to %d", len(w.b.Spine))
		}
		w.goTo(n-1, book.Pos{})
	default:
		if n < 1 || n > len(w.pages) {
			return fmt.Errorf("this chapter's pages: 1 to %d (:ch N another chapter)", len(w.pages))
		}
		w.page, w.at = n-1, w.pages[n-1].Start
	}
	w.list = false
	w.draw(a)
	return nil
}

// exFind looks for word from the page on (:/ alone: the next one),
// aside: a long book's chapters take a while to read.
func (w *bookWin) exFind(a *bro, word string) error {
	if w.pdf != nil {
		return w.exFindPDF(a, word)
	}
	spine, at := w.spine, w.at
	if word == "" {
		if w.found == nil {
			return errors.New(":/ what?")
		}
		word, spine, at = w.found.text, w.found.spine, w.found.at
	} else if at.Off > 0 || at.Block > 0 {
		at.Off-- // the page's first word too
	}
	a.Notify("looking for " + word + "…")
	b, gen := w.b, time.Now()
	w.finding = gen
	go func() {
		t := time.Now()
		s, pos, ok := b.Find(spine, at, word, func() bool { return w.gone || w.finding != gen })
		log.Printf("book: %q found %v in %v", word, ok, time.Since(t).Round(time.Millisecond))
		a.Do(func() {
			if w.gone || w.finding != gen {
				return
			}
			if !ok {
				a.Notify(word + ": not in the book")
				return
			}
			w.found = &bookFind{word, s, pos}
			w.goTo(s, pos)
			w.list = false
			w.draw(a)
			a.Notify(word + " is on this page (:/ the next)")
		})
	}()
	return nil
}

// exFindPDF is exFind's for a PDF: the next page with word in its text,
// this one first (:/ alone: the one after the last found).
func (w *bookWin) exFindPDF(a *bro, word string) error {
	from := w.spine
	if word == "" {
		if w.found == nil {
			return errors.New(":/ what?")
		}
		word, from = w.found.text, w.found.spine+1
	}
	a.Notify("looking for " + word + "…")
	d, n, gen := w.pdf, len(w.b.Spine), time.Now()
	w.finding = gen
	go func() {
		t := time.Now()
		page, ok := pdfFind(n, from, word, d.Text, func() bool { return w.gone || w.finding != gen })
		log.Printf("book: %q found %v in %v", word, ok, time.Since(t).Round(time.Millisecond))
		a.Do(func() {
			if w.gone || w.finding != gen {
				return
			}
			if !ok {
				a.Notify(word + ": not in the book")
				return
			}
			w.found = &bookFind{text: word, spine: page}
			if page != w.spine {
				w.goTo(page, book.Pos{})
			}
			w.list = false
			w.draw(a)
			a.Notify(fmt.Sprintf("%s is on page %d (:/ the next)", word, page+1))
		})
	}()
	return nil
}

// pdfFind is the first of n pages from from on, round, whose text has
// q in it, case and line breaks aside (a word cut by a hyphen at a
// line's end too).
func pdfFind(n, from int, q string, text func(int) (string, error), stop func() bool) (int, bool) {
	q = squash(q)
	if q == "" || n == 0 {
		return 0, false
	}
	for k := range n {
		if stop() {
			return 0, false
		}
		i := (from + k) % n
		if s, err := text(i); err == nil && strings.Contains(squash(s), q) {
			return i, true
		}
	}
	return 0, false
}

// squash is s in lower case, its runs of space one space, a hyphen at a
// line's end gone with the break.
func squash(s string) string {
	s = strings.ReplaceAll(s, "-\n", "")
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}
