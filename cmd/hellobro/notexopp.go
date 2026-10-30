package main

// Notebooks as Xournal++ keeps them (.xopp): gzipped XML, a page each
// with its background and layers of strokes, in points (1/72 in). A
// stroke's width is its first width, then one for each point after the
// first, as the pen pressed. Xournal++ (Linux, Mac, Windows) opens them,
// to go on writing on a computer, add text, make a PDF.
//
// What we don't draw (a PDF under the page) is kept as it came and
// written back; only all the layers become one. Pictures (images) are
// drawn, under the ink, highlighters' strokes (notepen.go) and texts,
// over it (notetext.go).

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"math"
	"strconv"
	"strings"
)

const notePt72 = 72.0 / 227 // points a page unit: the Elipsa's 227 dpi

// xoppAny is an element we don't draw, kept as it came.
type xoppAny struct {
	XMLName xml.Name
	Attrs   []xml.Attr `xml:",any,attr"`
	Inner   string     `xml:",innerxml"`
}

type xoppStroke struct {
	Attrs []xml.Attr `xml:",any,attr"`
	Data  string     `xml:",chardata"`
}

type xoppLayer struct {
	Images  []xoppImage  `xml:"image"` // first: under the ink (notepic.go)
	Strokes []xoppStroke `xml:"stroke"`
	Texts   []xoppTextEl `xml:"text"` // over it (notetext.go)
	Rest    []xoppAny    `xml:",any"`
}

type xoppTextEl struct {
	Attrs []xml.Attr `xml:",any,attr"`
	Data  string     `xml:",chardata"`
}

type xoppPage struct {
	Width      float64     `xml:"width,attr"`
	Height     float64     `xml:"height,attr"`
	Background *xoppAny    `xml:"background"`
	Layers     []xoppLayer `xml:"layer"`
}

type xoppDoc struct {
	XMLName xml.Name   `xml:"xournal"`
	Attrs   []xml.Attr `xml:",any,attr"`
	Title   string     `xml:"title,omitempty"`
	Pages   []xoppPage `xml:"page"`
}

// notePage is what a page has besides its strokes, from its file.
type notePage struct {
	w, h  float64
	bg    *xoppAny
	extra []xoppAny // what's on it we don't draw
}

func attr(as []xml.Attr, name string) string {
	for _, a := range as {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

func setAttr(as []xml.Attr, name, v string) []xml.Attr {
	for i, a := range as {
		if a.Name.Local == name {
			as[i].Value = v
			return as
		}
	}
	return append(as, xml.Attr{Name: xml.Name{Local: name}, Value: v})
}

var xoppPapers = map[string]string{"graph": "squares", "ruled": "lines", "lined": "lines", "plain": "plain"}

// parseXopp reads a .xopp, gzipped or not.
func parseXopp(name string, r io.Reader) (*notebook, error) {
	br := bufio.NewReader(r)
	if magic, _ := br.Peek(2); len(magic) == 2 && magic[0] == 0x1f && magic[1] == 0x8b {
		z, err := gzip.NewReader(br)
		if err != nil {
			return nil, err
		}
		defer z.Close()
		r = z
	} else {
		r = br
	}
	var doc xoppDoc
	if err := xml.NewDecoder(r).Decode(&doc); err != nil {
		return nil, fmt.Errorf("not a notebook: %w", err)
	}
	nb := &notebook{name: name, paper: "squares", attrs: doc.Attrs}
	for i, xp := range doc.Pages {
		pg := notePage{w: xp.Width, h: xp.Height, bg: xp.Background}
		if bg := xp.Background; i == 0 && bg != nil && xoppPapers[attr(bg.Attrs, "style")] != "" {
			nb.paper = xoppPapers[attr(bg.Attrs, "style")]
		}
		var strokes []*noteStroke
		for _, l := range xp.Layers {
			for _, xi := range l.Images {
				s, err := xoppToPic(xi)
				if err != nil { // not one we read: kept as it came
					log.Printf("notebook %s: an image: %v", name, err)
					pg.extra = append(pg.extra, xoppAny{XMLName: xml.Name{Local: "image"}, Attrs: xi.Attrs, Inner: xi.Data})
					continue
				}
				strokes = append(strokes, s)
			}
			for _, xs := range l.Strokes {
				s, err := xoppToStroke(xs)
				if err != nil {
					return nil, err
				}
				if s == nil { // not the pen's: kept, not drawn
					pg.extra = append(pg.extra, xoppAny{XMLName: xml.Name{Local: "stroke"}, Attrs: xs.Attrs, Inner: xs.Data})
					continue
				}
				strokes = append(strokes, s)
			}
			for _, xt := range l.Texts {
				strokes = append(strokes, xoppToText(xt))
			}
			pg.extra = append(pg.extra, l.Rest...)
		}
		nb.pages = append(nb.pages, strokes)
		nb.meta = append(nb.meta, pg)
	}
	if len(nb.pages) == 0 {
		nb.pages = [][]*noteStroke{nil}
	}
	return nb, nil
}

// xoppToStroke makes a stroke of the pen's or the highlighter's ours;
// nil for another tool's.
func xoppToStroke(xs xoppStroke) (*noteStroke, error) {
	t := attr(xs.Attrs, "tool")
	if t != "" && t != "pen" && t != "highlighter" {
		return nil, nil
	}
	nums := func(s string) ([]float64, error) {
		var vs []float64
		for _, f := range strings.Fields(s) {
			v, err := strconv.ParseFloat(f, 64)
			if err != nil {
				return nil, fmt.Errorf("a stroke that isn't: %q", f)
			}
			vs = append(vs, v)
		}
		return vs, nil
	}
	xy, err := nums(xs.Data)
	if err != nil {
		return nil, err
	}
	ws, err := nums(attr(xs.Attrs, "width"))
	if err != nil {
		return nil, err
	}
	if len(xy) < 2 || len(ws) == 0 {
		return nil, fmt.Errorf("a stroke of nothing")
	}
	unit := func(v float64) int16 { return int16(max(-1<<15, min(1<<15-1, math.Round(v/notePt72)))) }
	radius := func(w float64) int16 { return int16(max(1, min(1<<15-1, math.Round(w*5/notePt72)))) } // tenths of a unit, half the width
	s := &noteStroke{marker: t == "highlighter"}
	s.gray = colorGray(attr(xs.Attrs, "color"), s.marker)
	for _, a := range xs.Attrs {
		if a.Name.Local != "width" {
			s.attrs = append(s.attrs, a)
		}
	}
	n := len(xy) / 2
	for i := 0; i < n; i++ {
		w := ws[0]
		if i > 0 && i < len(ws) {
			w = ws[i]
		}
		s.pts = append(s.pts, notePt{unit(xy[2*i]), unit(xy[2*i+1]), radius(w)})
	}
	s.bound()
	return s, nil
}

func fmtPt(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }

// xoppText is nb as a .xopp, before it's gzipped. Empty pages at the
// end aren't kept.
func (nb *notebook) xoppText() []byte {
	doc := xoppDoc{Attrs: nb.attrs, Title: "Xournal++ document - see https://xournalpp.github.io/"}
	if attr(doc.Attrs, "creator") == "" {
		doc.Attrs = []xml.Attr{{Name: xml.Name{Local: "creator"}, Value: "hellobro"}, {Name: xml.Name{Local: "fileversion"}, Value: "4"}}
	}
	blank := func(i int) bool { // nothing on it, a PDF's page or a picture under it neither
		if len(nb.pages[i]) > 0 || i >= len(nb.meta) {
			return len(nb.pages[i]) == 0
		}
		m := nb.meta[i]
		return len(m.extra) == 0 && (m.bg == nil || attr(m.bg.Attrs, "type") == "" || attr(m.bg.Attrs, "type") == "solid")
	}
	n := len(nb.pages)
	for n > 1 && blank(n-1) {
		n--
	}
	style := map[string]string{"squares": "graph", "lines": "ruled", "plain": "plain"}[nb.paper]
	for i, strokes := range nb.pages[:n] {
		pg := defaultMeta()
		if i < len(nb.meta) {
			pg = nb.meta[i]
		}
		bg := xoppAny{XMLName: xml.Name{Local: "background"}, Attrs: []xml.Attr{
			{Name: xml.Name{Local: "type"}, Value: "solid"}, {Name: xml.Name{Local: "color"}, Value: "#ffffffff"}}}
		if pg.bg != nil {
			bg = *pg.bg
			bg.Attrs = append([]xml.Attr(nil), bg.Attrs...)
		}
		if t := attr(bg.Attrs, "type"); t == "" || t == "solid" { // a PDF's or a picture's stays as it is
			bg.Attrs = setAttr(bg.Attrs, "style", style)
		}
		layer := xoppLayer{Rest: pg.extra}
		for _, s := range strokes {
			if s.pic != nil {
				layer.Images = append(layer.Images, s.xoppImage())
				continue
			}
			if s.txt != nil {
				layer.Texts = append(layer.Texts, s.xoppText())
				continue
			}
			layer.Strokes = append(layer.Strokes, s.xopp())
		}
		doc.Pages = append(doc.Pages, xoppPage{Width: pg.w, Height: pg.h, Background: &bg, Layers: []xoppLayer{layer}})
	}
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" standalone="no"?>` + "\n")
	xml.NewEncoder(&b).Encode(doc) // what's in it came from xml, or is ours: it can't fail
	b.WriteString("\n")
	return b.Bytes()
}

// xopp is s as a .xopp's stroke: one point is two, for Xournal++.
func (s *noteStroke) xopp() xoppStroke {
	pts := s.pts
	if len(pts) == 1 {
		pts = []notePt{pts[0], pts[0]}
	}
	width := func(p notePt) string { return fmtPt(float64(p.r) / 5 * notePt72) }
	var xy, ws strings.Builder
	ws.WriteString(width(pts[0]))
	for i, p := range pts {
		if i > 0 {
			xy.WriteByte(' ')
			ws.WriteString(" " + width(p))
		}
		xy.WriteString(fmtPt(float64(p.x)*notePt72) + " " + fmtPt(float64(p.y)*notePt72))
	}
	as := s.attrs
	if as == nil {
		as = []xml.Attr{{Name: xml.Name{Local: "tool"}, Value: "pen"}, {Name: xml.Name{Local: "color"}, Value: "#000000ff"},
			{Name: xml.Name{Local: "capStyle"}, Value: "round"}}
	}
	as = append(append([]xml.Attr(nil), as...), xml.Attr{Name: xml.Name{Local: "width"}, Value: ws.String()})
	return xoppStroke{Attrs: as, Data: xy.String()}
}

// xopp is nb's file: gzipped.
func (nb *notebook) xopp() ([]byte, error) {
	var b bytes.Buffer
	z := gzip.NewWriter(&b)
	if _, err := z.Write(nb.xoppText()); err != nil {
		return nil, err
	}
	if err := z.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
