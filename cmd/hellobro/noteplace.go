package main

// Where each notebook was left, its page and how far down its roll, so
// it opens there again: after closing it, after a restart, and after the
// Kobo went off (written a few seconds after it moves, not only at quit).
// Kept in note-places.json in the app's folder, by the notebook's name.

import (
	"encoding/json"
	"log"
	"os"
	"time"

	"hellobro/kobo"
)

const (
	notePlacesFile  = "note-places.json"
	notePlacesAfter = 3 * time.Second // still this long: written
	notePlacesMost  = 300             // notebooks remembered; the ones gone go first
)

// notePlace is where a notebook was left.
type notePlace struct {
	Page   int       `json:"page"`
	Scroll int       `json:"scroll,omitempty"` // a roll's: how far down the page, units
	At     time.Time `json:"at"`
}

type notePlaces struct {
	m       map[string]notePlace
	loaded  bool
	dirty   bool
	movedAt time.Time // the last place that changed
}

// places are the places, read the first time.
func (a *bro) places() *notePlaces {
	ps := &a.nt.places
	if !ps.loaded {
		ps.loaded, ps.m = true, map[string]notePlace{}
		if b, err := os.ReadFile(kobo.AppFile(notePlacesFile)); err == nil {
			if err := json.Unmarshal(b, &ps.m); err != nil {
				log.Printf("%s: %v", notePlacesFile, err)
			}
		}
	}
	return ps
}

// putPlace opens w where its notebook was left.
func (a *bro) putPlace(w *noteWin) {
	p, ok := a.places().m[w.nb.name]
	if !ok {
		return
	}
	w.cur = max(0, min(p.Page, len(w.nb.pages)-1))
	w.scroll = max(p.Scroll, 0) // place() keeps it in the page
}

// notePlaceOf is where w is now.
func notePlaceOf(w *noteWin) notePlace {
	p := notePlace{Page: w.cur}
	if w.nb.rollOn() {
		p.Scroll = w.scroll
	}
	return p
}

// notePlacesTick notes where the notebooks are, and writes it once they
// have stayed put a moment (now: at once).
func (a *bro) notePlacesTick(now bool) {
	ps := a.places()
	for _, w := range a.noteWins() {
		a.notePlaceSeen(w)
	}
	if ps.dirty && (now || time.Since(ps.movedAt) > notePlacesAfter) {
		a.notePlacesWrite()
	}
}

// notePlaceSeen notes where w is, if it moved.
func (a *bro) notePlaceSeen(w *noteWin) {
	if w.nb == nil || w.nb.name == "" {
		return
	}
	ps := a.places()
	p := notePlaceOf(w)
	if old, ok := ps.m[w.nb.name]; ok && old.Page == p.Page && old.Scroll == p.Scroll {
		return
	}
	p.At = time.Now()
	ps.m[w.nb.name], ps.dirty, ps.movedAt = p, true, time.Now()
}

// notePlacesWrite writes them, the notebooks gone left out, and the
// oldest past notePlacesMost.
func (a *bro) notePlacesWrite() {
	ps := a.places()
	for name := range ps.m {
		if _, err := os.Stat(notePath(name)); err != nil && a.noteWindow(name) == nil {
			delete(ps.m, name)
		}
	}
	for len(ps.m) > notePlacesMost {
		oldest := ""
		for name, p := range ps.m {
			if oldest == "" || p.At.Before(ps.m[oldest].At) {
				oldest = name
			}
		}
		delete(ps.m, oldest)
	}
	b, err := json.MarshalIndent(ps.m, "", " ")
	if err == nil {
		err = writeAtomic(kobo.AppFile(notePlacesFile), b)
	}
	if err != nil {
		log.Printf("%s: %v", notePlacesFile, err)
		return
	}
	ps.dirty = false
}
