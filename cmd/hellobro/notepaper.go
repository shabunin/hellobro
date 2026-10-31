package main

// A notebook's paper, a tool of its own on the bar: plain, lines,
// squares or dots, each at 5 mm or narrow (3.3 mm). The screen, the PDF
// and the PNG draw it the same (drawPaper). In the .xopp it's Xournal++'s
// style (ruled, graph, dotted), the narrow ones marked hellobro-paper as
// well, which Xournal++ leaves be.

import (
	"encoding/xml"
	"image"
	"strings"

	"hellobro/kobo"
)

const (
	notePapers   = "plain lines lines-narrow squares squares-narrow dots dots-narrow"
	noteNarrow   = noteRule * 2 / 3 // a narrow paper's spacing
	notePaperTop = 2 * noteRule     // the first line down, a margin over it; a multiple of both spacings
)

// paperWords are the papers as the tool says them.
var paperWords = map[string]string{
	"plain": "plain", "lines": "lines", "lines-narrow": "narrow lines", "squares": "squares",
	"squares-narrow": "narrow squares", "dots": "dots", "dots-narrow": "narrow dots",
}

// isPaper says p is one of notePapers.
func isPaper(p string) bool { return p != "" && strings.Contains(" "+notePapers+" ", " "+p+" ") }

// paperOf is p's kind (plain, lines, squares, dots) and spacing, in
// page units.
func paperOf(p string) (kind string, rule int) {
	kind, narrow := strings.CutSuffix(p, "-narrow")
	if narrow {
		return kind, noteNarrow
	}
	return kind, noteRule
}

// drawPaper draws paper on a page of size (page units) at, k px a unit,
// where it's in cl, in ink: the lines, the squares or the dots.
func drawPaper(c kobo.Canvas, paper string, size, at image.Point, k float64, cl image.Rectangle, ink uint8) {
	kind, rule := paperOf(paper)
	px := func(u, from int) int { return from + int(float64(u)*k) }
	switch kind {
	case "squares", "lines":
		if kind == "squares" {
			for u := rule; u < size.X; u += rule {
				x := px(u, at.X)
				c.Fill(image.Rect(x, at.Y, x+1, px(size.Y, at.Y)).Intersect(cl), ink)
			}
		}
		for u := notePaperTop; u < size.Y; u += rule {
			y := px(u, at.Y)
			c.Fill(image.Rect(at.X, y, px(size.X, at.X), y+1).Intersect(cl), ink)
		}
	case "dots":
		d, ink := max(2, int(2.5*k+0.5)), uint8(int(ink)*3/4) // a dot's side, and darker than a line: seen, not ink
		for v := notePaperTop; v < size.Y; v += rule {
			y := px(v, at.Y) - d/2
			if y+d < cl.Min.Y || y > cl.Max.Y {
				continue
			}
			for u := rule; u < size.X; u += rule {
				x := px(u, at.X) - d/2
				c.Fill(image.Rect(x, y, x+d, y+d).Intersect(cl), ink)
			}
		}
	}
}

// xoppStyles are the papers' kinds as Xournal++'s backgrounds' styles.
var xoppStyles = map[string]string{"squares": "graph", "lines": "ruled", "plain": "plain", "dots": "dotted"}

// xoppPaper is the paper a .xopp's background says, "" if none of ours.
// The narrow mark counts while the style is still its kind's: changed
// in Xournal++, the style's what it is now.
func xoppPaper(as []xml.Attr) string {
	kind := map[string]string{"graph": "squares", "ruled": "lines", "lined": "lines", "plain": "plain", "dotted": "dots"}[attr(as, "style")]
	if p := attr(as, "hellobro-paper"); isPaper(p) {
		if k, _ := paperOf(p); k == kind {
			return p
		}
	}
	return kind
}

// paperAttrs sets paper on a .xopp's background.
func paperAttrs(as []xml.Attr, paper string) []xml.Attr {
	kind, _ := paperOf(paper)
	as = setAttr(as, "style", xoppStyles[kind])
	if kind != paper {
		return setAttr(as, "hellobro-paper", paper)
	}
	for i, a := range as { // a narrow one's mark, gone
		if a.Name.Local == "hellobro-paper" {
			return append(as[:i:i], as[i+1:]...)
		}
	}
	return as
}

// paperIcon is the paper tool's picture: a page as the paper is.
func paperIcon(paper string) []iconPart {
	ps := []iconPart{iconShut(0.8, 6, 3, 18, 3, 18, 21, 6, 21)}
	kind, rule := paperOf(paper)
	step := 6.0 // fewer than on the page: a picture at a button's size
	if rule == noteNarrow {
		step = 4
	}
	switch kind {
	case "plain":
		return append(ps, iconLine(0.5, 14, 3, 14, 7, 18, 7)) // its corner turned
	case "dots":
		for y := 3 + step; y < 21; y += step {
			for x := 6 + step; x < 18; x += step {
				ps = append(ps, iconPart{kind: 'd', pts: []float64{x, y, 0.7}})
			}
		}
		return ps
	}
	for y := 3 + step; y < 21; y += step {
		ps = append(ps, iconLine(0, 6, y, 18, y))
	}
	if kind == "squares" {
		for x := 6 + step; x < 18; x += step {
			ps = append(ps, iconLine(0, x, 3, x, 21))
		}
	}
	return ps
}
