package main

// A notebook's pages' size ("Page size" in its settings, :pagesize):
// the screen upright, as it's always been; the screen across, a page
// that fills a landscape screen; A4, A5, Letter, a square; or a size
// of one's own, in mm. New pages are made so, and the page shown too if
// nothing's on it yet. Each page keeps its size in the file, so a
// notebook can have pages of several.

import (
	"errors"
	"fmt"
	"image"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const pageSizeAttr = "hellobro-page" // the notebook's: its new pages' size

// pageSizes are the sizes it has names for, in page units (the Elipsa's
// pixels, 227 dpi).
var pageSizes = []struct {
	id, label string
	w, h      int
}{
	{"screen", "the screen, upright", notePageW, notePageH},
	{"landscape", "the screen, across", notePageH, notePageW},
	{"a4", "A4", mmUnits(210), mmUnits(297)},
	{"a5", "A5", mmUnits(148), mmUnits(210)},
	{"letter", "Letter", mmUnits(215.9), mmUnits(279.4)},
	{"square", "a square", notePageW, notePageW},
}

func mmUnits(mm float64) int { return int(math.Round(mm / 25.4 * 227)) }

func unitsMM(u int) float64 { return math.Round(float64(u)*25.4/227*10) / 10 }

var customSizeRE = regexp.MustCompile(`^\s*(\d+(?:\.\d+)?)\s*(?:x|×|\*|by)\s*(\d+(?:\.\d+)?)\s*(?:mm)?\s*$`)

// parsePageSize is a size as :pagesize says it: a name, or W x H in mm
// (40 to 1000 mm a side).
func parsePageSize(s string) (image.Point, string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	for _, ps := range pageSizes {
		if s == ps.id || s == strings.ToLower(ps.label) || ps.id == "screen" && (s == "portrait" || s == "upright") {
			return image.Pt(ps.w, ps.h), ps.id, nil
		}
	}
	if m := customSizeRE.FindStringSubmatch(s); m != nil {
		w, _ := strconv.ParseFloat(m[1], 64)
		h, _ := strconv.ParseFloat(m[2], 64)
		if w >= 40 && h >= 40 && w <= 1000 && h <= 1000 {
			return image.Pt(mmUnits(w), mmUnits(h)), fmt.Sprintf("%gx%g", w, h), nil
		}
	}
	var ids []string
	for _, ps := range pageSizes {
		ids = append(ids, ps.id)
	}
	return image.Point{}, "", errors.New("a page size: " + strings.Join(ids, ", ") + ", or W x H in mm (as 200x120)")
}

// newSize is the size the notebook's new pages are made.
func (nb *notebook) newSize() image.Point {
	if v := attr(nb.attrs, pageSizeAttr); v != "" {
		if p, _, err := parsePageSize(v); err == nil {
			return p
		}
	}
	return image.Pt(notePageW, notePageH)
}

// newMeta is a new page of the notebook, in the file: its size, in
// points.
func (nb *notebook) newMeta() notePage {
	p := nb.newSize()
	return notePage{w: math.Round(float64(p.X)*notePt72*100) / 100, h: math.Round(float64(p.Y)*notePt72*100) / 100}
}

// sizeShown is how the settings say the new pages' size.
func (nb *notebook) sizeShown() string {
	v := attr(nb.attrs, pageSizeAttr)
	for _, ps := range pageSizes {
		if v == ps.id || v == "" && ps.id == "screen" {
			return fmt.Sprintf("%s, %g × %g mm", ps.label, unitsMM(ps.w), unitsMM(ps.h))
		}
	}
	p := nb.newSize()
	return fmt.Sprintf("%g × %g mm (:pagesize)", unitsMM(p.X), unitsMM(p.Y))
}

// setPageSize makes v (a name, or W x H in mm) the new pages' size, and
// the page shown's if nothing's written on it.
func (w *noteWin) setPageSize(a *bro, v string) error {
	_, id, err := parsePageSize(v)
	if err != nil {
		return err
	}
	w.nb.attrs = setAttr(w.nb.attrs, pageSizeAttr, id)
	if len(w.strokes(w.cur)) == 0 && w.pdfPage(w.cur) < 0 {
		w.nb.padMeta()
		for len(w.nb.meta) <= w.cur {
			w.nb.meta = append(w.nb.meta, w.nb.newMeta())
		}
		m := w.nb.newMeta()
		w.nb.meta[w.cur].w, w.nb.meta[w.cur].h = m.w, m.h
	}
	w.changed = time.Now()
	w.scroll = 0
	return nil
}

// stepPageSize steps the new pages' size through the named ones, d on.
func (w *noteWin) stepPageSize(a *bro, d int) {
	v := attr(w.nb.attrs, pageSizeAttr)
	i := 0
	for k, ps := range pageSizes {
		if ps.id == v {
			i = k
		}
	}
	w.setPageSize(a, pageSizes[(i+d+len(pageSizes))%len(pageSizes)].id)
}
