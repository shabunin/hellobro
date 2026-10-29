package main

// Pictures in notebooks: a part of a PDF's page cut out and copied
// (bookcut.go) is pasted on the notebook's page, under what's on it
// already, as notes are taken; there's no room left, on the next page.
// The paste button does it, or Ctrl+V. A picture is a stroke of a kind
// (noteStroke.pic): written over, erased, undone and cleared as one.
// In the .xopp it's an image, as Xournal++ has them: a PNG.

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"log"
	"os"
	"strconv"

	xdraw "golang.org/x/image/draw"

	"hellobro/kobo"
)

const clipPic = "clipboard.png" // the clipboard's picture, next to its text

// notePic is a picture on a page: its PNG as it came (and is kept), and
// gray, drawn at the size it was last shown.
type notePic struct {
	png    []byte
	gray   *image.Gray
	scaled *image.Gray
}

func newNotePic(data []byte) (*notePic, error) {
	m, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	g, ok := m.(*image.Gray)
	if !ok {
		g = image.NewGray(m.Bounds())
		draw.Draw(g, g.Bounds(), m, m.Bounds().Min, draw.Src)
	}
	return &notePic{png: data, gray: g}, nil
}

// picStroke is a picture put in box (page units) as a stroke.
func picStroke(p *notePic, box image.Rectangle) *noteStroke {
	return &noteStroke{pic: p, box: box}
}

// drawPic draws s's picture where its box is on the page at.
func (w *noteWin) drawPic(a *bro, s *noteStroke, at, clip image.Rectangle) {
	r := image.Rect(at.Min.X+int(float64(s.box.Min.X)*w.scale+0.5), at.Min.Y+int(float64(s.box.Min.Y)*w.scale+0.5),
		at.Min.X+int(float64(s.box.Max.X)*w.scale+0.5), at.Min.Y+int(float64(s.box.Max.Y)*w.scale+0.5))
	p := s.pic
	if p.scaled == nil || p.scaled.Bounds().Size() != r.Size() {
		p.scaled = image.NewGray(image.Rectangle{Max: r.Size()})
		xdraw.ApproxBiLinear.Scale(p.scaled, p.scaled.Bounds(), p.gray, p.gray.Bounds(), xdraw.Src, nil)
	}
	cl := r.Intersect(clip)
	kobo.CopyGray(a.C.Gray, cl, p.scaled, cl.Min.Sub(r.Min))
}

// copyPicture puts a picture in the clipboard (a PNG), for a notebook to
// paste.
func copyPicture(m image.Image) error {
	var b bytes.Buffer
	if err := png.Encode(&b, m); err != nil {
		return err
	}
	return os.WriteFile(kobo.AppFile(clipPic), b.Bytes(), 0o600)
}

// paste puts the clipboard's picture on the page shown (the left one of
// two), under what's on it, or on the next page if there's no room; or
// at the select tool's mark. It comes in taken by the select tool.
func (w *noteWin) paste(a *bro) {
	data, err := os.ReadFile(kobo.AppFile(clipPic))
	if err != nil {
		a.Notify("nothing to paste: copy a part of a PDF's page first")
		return
	}
	p, err := newNotePic(data)
	if err != nil {
		log.Printf("notebook: the clipboard's picture: %v", err)
		a.Notify("can't paste it: " + err.Error())
		return
	}
	page, box := w.pasteWhere(p.gray.Bounds().Size()) // notesel.go
	for len(w.nb.pages) <= page {
		w.nb.pages = append(w.nb.pages, nil)
	}
	op := noteOp{page: page, add: []*noteStroke{picStroke(p, box)}}
	w.nb.apply(op, false)
	w.edited(op)
	w.pasted(page, op.add[0])
	if page >= w.cur+w.shown() {
		w.cur = page
	}
	log.Printf("notebook %s: a picture pasted on page %d", w.nb.name, page+1)
	w.show(a)
}

// pastePlace is where a picture of size (page units, one a pixel) goes:
// shrunk to fit inside the margins, under the lowest thing on the page
// shown, or at the top of the one after it.
func (w *noteWin) pastePlace(size image.Point) (int, image.Rectangle) {
	const margin = noteRule
	room := image.Pt(notePageW-2*margin, notePageH-3*margin)
	if size.X > room.X || size.Y > room.Y {
		k := min(float64(room.X)/float64(size.X), float64(room.Y)/float64(size.Y))
		size = image.Pt(max(1, int(float64(size.X)*k)), max(1, int(float64(size.Y)*k)))
	}
	page := w.cur
	top := 2 * margin
	for _, s := range w.strokes(page) {
		top = max(top, s.box.Max.Y+margin/2)
	}
	if top+size.Y > notePageH-margin && len(w.strokes(page)) > 0 {
		page, top = page+1, 2*margin
		for _, s := range w.strokes(page) { // a page after it with something on it already: under that
			top = max(top, s.box.Max.Y+margin/2)
		}
	}
	return page, image.Rectangle{Max: size}.Add(image.Pt(margin, top))
}

// --- in the file ---

// xoppImage is a picture on a .xopp's page: where (points), and a PNG,
// base64.
type xoppImage struct {
	Attrs []xml.Attr `xml:",any,attr"`
	Data  string     `xml:",chardata"`
}

// xoppToPic makes a .xopp's image ours; an error if it isn't one we can.
func xoppToPic(xi xoppImage) (*noteStroke, error) {
	var at [4]int
	for i, name := range []string{"left", "top", "right", "bottom"} {
		v, err := strconv.ParseFloat(attr(xi.Attrs, name), 64)
		if err != nil {
			return nil, fmt.Errorf("an image's %s: %v", name, err)
		}
		at[i] = int(v/notePt72 + 0.5)
	}
	data, err := base64.StdEncoding.DecodeString(xi.Data)
	if err != nil {
		return nil, err
	}
	p, err := newNotePic(data)
	if err != nil {
		return nil, err
	}
	return picStroke(p, image.Rect(at[0], at[1], at[2], at[3])), nil
}

// xoppImage is s's picture as a .xopp's image.
func (s *noteStroke) xoppImage() xoppImage {
	b := s.box
	return xoppImage{Attrs: []xml.Attr{
		{Name: xml.Name{Local: "left"}, Value: fmtPt(float64(b.Min.X) * notePt72)},
		{Name: xml.Name{Local: "top"}, Value: fmtPt(float64(b.Min.Y) * notePt72)},
		{Name: xml.Name{Local: "right"}, Value: fmtPt(float64(b.Max.X) * notePt72)},
		{Name: xml.Name{Local: "bottom"}, Value: fmtPt(float64(b.Max.Y) * notePt72)},
	}, Data: base64.StdEncoding.EncodeToString(s.pic.png)}
}
