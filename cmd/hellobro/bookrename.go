package main

// A book renamed from the shelf: a row's rename (or F2) types its name
// in the row, as the notebooks' list does. Its file is renamed, its
// kind kept (".kepub.epub" stays one), and the name is shown from then
// on, over the title the book gives itself. Its place and bookmarks go
// along, and the windows it's open in.

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// placeName is the name a book's shown by: the one it was given, else
// its own title, else its file's.
func placeName(path string, pl bookPlace) string {
	switch {
	case pl.Name != "":
		return pl.Name
	case pl.Title != "":
		t, _, _ := strings.Cut(pl.Title, " / ") // Gutenberg's: "Alice… / Illustrated by…"
		return t
	}
	return shelfName(filepath.Base(path))
}

// startRename types a new name for the book at path, in its row.
func (w *bookWin) startRename(a *bro, path, name string) {
	w.edit = &noteEdit{key: path, text: name}
	a.vk.ask = true
	a.vkApply()
	w.draw(a)
}

// endRename takes the name typed (ok) or leaves it.
func (w *bookWin) endRename(a *bro, ok bool) {
	e := w.edit
	w.edit = nil
	a.vk.ask = false
	defer a.vkApply()
	if ok && e != nil {
		if err := renameBook(a, e.key, strings.TrimSpace(e.text)); err != nil {
			a.Notify(err.Error())
		}
	}
	w.draw(a)
}

// renameBook gives the book at from the name: its file's too, beside it.
func renameBook(a *bro, from, name string) error {
	if err := noteNameOK(name); err != nil {
		return err
	}
	file := filepath.Base(from)
	to := filepath.Join(filepath.Dir(from), name+file[len(shelfName(file)):])
	if to != from {
		if fileExists(to) {
			return fmt.Errorf("there's a %s already", filepath.Base(to))
		}
		for _, w := range a.windows() { // where it's read: kept first
			if b, ok := w.(*bookWin); ok && b.path == from {
				b.save(a)
			}
		}
		if err := os.Rename(from, to); err != nil {
			return err
		}
	}
	if a.mem.Books == nil {
		a.mem.Books = map[string]bookPlace{}
	}
	pl := a.mem.Books[from]
	delete(a.mem.Books, from)
	pl.Name = name
	a.mem.Books[to] = pl
	a.mem.save()
	open := false
	for _, w := range a.windows() {
		b, ok := w.(*bookWin)
		if !ok {
			continue
		}
		if b.back == from {
			b.back = to
		}
		for i := range b.shelf {
			if b.shelf[i].path == from {
				b.shelf[i].path, b.shelf[i].name = to, name
			}
		}
		if b.path == from {
			b.path, b.named = to, name
			b.place(a, b.p) // its title
			open = true
		}
	}
	if open {
		a.wm.changed = time.Now() // the layout names them
		a.wmBar()
	}
	log.Printf("book: %s is %s now", from, filepath.Base(to))
	return nil
}
