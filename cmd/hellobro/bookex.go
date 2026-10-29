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
		{"page p", "N or N%", func(arg string) error { return w.exPage(a, arg) }},
		{"chapter ch", "N", func(arg string) error {
			n, _, err := exNumber(arg)
			if err != nil || n < 1 || n > len(w.b.Spine) {
				return fmt.Errorf("a chapter from 1 to %d", len(w.b.Spine))
			}
			w.goTo(n-1, book.Pos{})
			w.list = false
			w.draw(a)
			return nil
		}},
		{"find / f", "word", func(arg string) error { return w.exFind(a, arg) }},
		{"contents toc", "", func(string) error { w.openContents(a); return nil }},
		{"bookmarks marks", "", func(string) error { w.showList(a, true); return nil }},
		{"mark m", "", func(string) error { w.toggleMark(a); return nil }},
		{"back", "", func(string) error {
			if w.from == nil {
				return errors.New("no link followed to go back from")
			}
			w.goBack(a)
			return nil
		}},
		{"style s", "", func(string) error { w.openStyle(a); return nil }},
		{"size", "px", func(arg string) error {
			n, _, err := exNumber(arg)
			if err != nil || n < bookPxMin || n > bookPxMax {
				return fmt.Errorf("a size from %d to %d", bookPxMin, bookPxMax)
			}
			a.restyle(n, a.bookFaces().fam.name)
			return nil
		}},
		{"font", "name", func(arg string) error {
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
		{"zoom z", "%", func(arg string) error {
			n, _, err := exNumber(strings.TrimSuffix(arg, "%"))
			if w.pdf == nil || err != nil || n < pdfZoomMin || n > pdfZoomMax {
				return fmt.Errorf("a PDF's zoom, from %d to %d", pdfZoomMin, pdfZoomMax)
			}
			w.setZoom(a, n, w.area.Min.Add(w.area.Max).Div(2))
			return nil
		}},
		{"pages", "", func(string) error {
			if w.pdf == nil {
				return errors.New(":pages is a PDF's")
			}
			w.showPages(a)
			return nil
		}},
		{"rename", "name", func(arg string) error { return renameBook(a, w.path, arg) }},
		{"shelf books", "", func(string) error { w.toShelf(a); return nil }},
		{"quit q close", "", func(string) error { a.closeWin(w); return nil }},
	}
	return append(cmds, exCmd{"help h", "", exHelp(a, cmds)})
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
		return errors.New("looking for a word: in EPUBs only, for now")
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
