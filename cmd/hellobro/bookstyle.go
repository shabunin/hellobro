package main

// The reader's style panel, in the book's window over its page: the
// fonts, each shown in itself; the size, the lines' spacing, the
// margins, justified or not, hyphens or not; and a paragraph set as the
// books will be. s or a tap under the page opens it, Esc or Done goes
// back to the page. It's the reader's own: the app's Settings don't
// have it.

import (
	"fmt"
	"image"
	"os"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"

	"hellobro/book"
	"hellobro/kobo"
)

// The controls after the fonts, in order.
const (
	styleSize = iota
	styleLeading
	styleMargin
	styleJustify
	styleHyph
	styleDone
	styleN
)

var styleLabels = [styleN]string{"Size", "Line spacing", "Margins", "Justified", "Hyphenation", ""}

// bookSample is what the panel sets as the books will be.
var bookSample = &book.Chapter{Blocks: []book.Block{
	{First: 1.5, Spans: []book.Span{
		{Text: "Alice was beginning to get very tired of sitting by her sister on the bank, and of having nothing to do: once or twice she had peeped into the book her sister was reading, but it had no pictures or conversations in it, “and what is the use of a book,” thought Alice, “without pictures or ", Scale: 1},
		{Text: "conversations", Style: book.Italic, Scale: 1},
		{Text: "?”", Scale: 1},
	}},
	{First: 1.5, Spans: []book.Span{
		{Text: "So she was considering in her own mind (as well as she could, for the hot day made her feel very sleepy and stupid) whether the pleasure of making a daisy-chain would be worth the trouble of getting up and picking the daisies, when suddenly a White Rabbit with pink eyes ran close by her.", Scale: 1},
	}},
}}

// familyFace is fam's regular face at the panel's size, to show it by.
func (a *bro) familyFace(fam bookFamily) font.Face {
	s := &a.bk
	if f := s.names[fam.name]; f != nil {
		return f
	}
	if s.names == nil {
		s.names = map[string]font.Face{}
	}
	data := goregular.TTF
	if fam.files[0] != "" {
		if b, err := os.ReadFile(fam.files[0]); err == nil {
			data = b
		}
	}
	o, err := parseFont(data)
	if err != nil {
		o, _ = opentype.Parse(goregular.TTF)
	}
	f, _ := opentype.NewFace(o, &opentype.FaceOptions{Size: 32, DPI: 72, Hinting: font.HintingFull})
	s.names[fam.name] = f
	return f
}

func (w *bookWin) openStyle(a *bro) {
	w.styling, w.list = true, false
	cur := a.bookFaces().fam.name
	w.sel = 0
	for i, f := range a.bk.fams {
		if f.name == cur {
			w.sel = i
		}
	}
	w.draw(a)
}

func (w *bookWin) closeStyle(a *bro) {
	w.styling = false
	w.draw(a)
}

// styleStep changes control i: d -1 or 1 a step down or up, 0 picks it
// (a font, a switch, Done).
func (w *bookWin) styleStep(a *bro, i, d int) {
	fams := a.bk.fams
	st := a.bookStyle()
	if i < len(fams) {
		if d != 0 {
			i = (i + d + len(fams)) % len(fams)
			w.sel = i
		}
		st.font = fams[i].name
		a.setBookStyle(st)
		return
	}
	switch i - len(fams) {
	case styleSize:
		st.px = max(bookPxMin, min(bookPxMax, st.px+d*bookPxStep))
	case styleLeading:
		st.leading = max(bookLeadingMin, min(bookLeadingMax, st.leading+d*10))
	case styleMargin:
		st.margin = max(1, min(len(bookMargins)-1, st.margin+d))
	case styleJustify:
		st.ragged = !st.ragged
	case styleHyph:
		st.noHyphs = !st.noHyphs
	case styleDone:
		w.closeStyle(a)
		return
	}
	a.setBookStyle(st)
}

func (w *bookWin) styleKey(a *bro, code uint16, ch string) bool {
	n := len(a.bk.fams) + styleN
	switch {
	case code == 103 || ch == "k": // up
		w.sel = (w.sel - 1 + n) % n
		w.draw(a)
	case code == 108 || ch == "j" || code == kobo.KeyTab:
		w.sel = (w.sel + 1) % n
		w.draw(a)
	case code == 105 || ch == "h": // left
		w.styleStep(a, w.sel, -1)
	case code == 106 || ch == "l":
		w.styleStep(a, w.sel, 1)
	case code == kobo.KeyEnter || code == kobo.KeySpace:
		w.styleStep(a, w.sel, 0)
	case ch == "+" || ch == "=":
		w.styleStep(a, len(a.bk.fams)+styleSize, 1)
	case ch == "-":
		w.styleStep(a, len(a.bk.fams)+styleSize, -1)
	case code == kobo.KeyEsc || ch == "s" || ch == "q":
		w.closeStyle(a)
	default:
		return false
	}
	return true
}

func (st bookStyle) values() [styleN]string {
	onOff := map[bool]string{true: "on", false: "off"}
	return [styleN]string{
		fmt.Sprintf("%d px", st.px),
		fmt.Sprintf("%.2f", float64(st.leading)/100),
		fmt.Sprintf("%d%%", bookMargins[st.margin]),
		onOff[!st.ragged],
		onOff[!st.noHyphs],
		"",
	}
}

// settings are the layout's, for a page of size in a book in lang.
func (st bookStyle) settings(size image.Point, lang string) book.Settings {
	return book.Settings{W: size.X, H: size.Y, Px: st.px, Leading: float64(st.leading) / 100, Ragged: st.ragged, NoHyph: st.noHyphs, Lang: lang}
}

func (w *bookWin) drawStyle(a *bro) {
	faces := a.bookFaces()
	fams := a.bk.fams
	st := a.bookStyle()
	area := w.p.Body.Inset(kobo.Pad)
	c := a.C
	w.rows = nil
	focus := func(i int, r image.Rectangle) {
		if i == w.sel {
			c.Frame(r.Inset(-4), 3, kobo.Dark)
		}
	}
	// the fonts, each in itself, two columns if there's room
	cols := 1
	if area.Dx() >= 760 {
		cols = 2
	}
	nameH := 56
	colW := area.Dx() / cols
	y := area.Min.Y + 4
	for i, f := range fams {
		col, row := i%cols, i/cols
		r := image.Rect(area.Min.X+col*colW+6, y+row*nameH, area.Min.X+(col+1)*colW-6, y+(row+1)*nameH-6)
		ink := uint8(kobo.Black)
		if f.name == st.font {
			c.Fill(r, kobo.Black)
			ink = kobo.White
		}
		focus(i, r)
		c.Text(a.familyFace(f), r.Inset(12), f.name, ink, kobo.AlignLeft)
		w.rows = append(w.rows, bookRow{r, func() { w.sel = i; w.styleStep(a, i, 0) }})
	}
	y += (len(fams)+cols-1)/cols*nameH + 14
	// the rest: a label, then − value + or a switch
	rowH, boxW := 64, 72
	vals := st.values()
	for k := range styleN {
		i := len(fams) + k
		r := image.Rect(area.Min.X+6, y, area.Max.X-6, y+rowH-8)
		y += rowH
		focus(i, r)
		box := func(b image.Rectangle, label string, do func()) {
			c.Fill(b, kobo.White)
			c.Frame(b, 2, kobo.Black)
			c.Text(a.F.Panel, b, label, kobo.Black, kobo.AlignCenter)
			w.rows = append(w.rows, bookRow{b, func() { w.sel = i; do() }})
		}
		if k == styleDone {
			box(image.Rect(r.Max.X-260, r.Min.Y, r.Max.X, r.Max.Y), "back to the book", func() { w.styleStep(a, i, 0) })
			break
		}
		c.Text(a.F.Body, image.Rect(r.Min.X+12, r.Min.Y, r.Max.X, r.Max.Y), styleLabels[k], kobo.Black, kobo.AlignLeft)
		right := r.Max.X
		if k == styleJustify || k == styleHyph {
			box(image.Rect(right-2*boxW, r.Min.Y, right, r.Max.Y), vals[k], func() { w.styleStep(a, i, 0) })
			continue
		}
		box(image.Rect(right-boxW, r.Min.Y, right, r.Max.Y), "+", func() { w.styleStep(a, i, 1) })
		vr := image.Rect(right-boxW-160, r.Min.Y, right-boxW, r.Max.Y)
		c.Text(a.F.Body, vr, vals[k], kobo.Black, kobo.AlignCenter)
		box(image.Rect(vr.Min.X-boxW, r.Min.Y, vr.Min.X, r.Max.Y), "−", func() { w.styleStep(a, i, -1) })
	}
	// a paragraph as the books will be, in the page's own margins
	pv := image.Rect(w.area.Min.X, y+8, w.area.Max.X, area.Max.Y)
	if pv.Dy() < 2*st.px {
		return
	}
	c.Fill(image.Rect(area.Min.X, y, area.Max.X, y+2), kobo.Light)
	pages := book.Layout(bookSample, st.settings(pv.Size(), "en"), faces, nil)
	for _, it := range pages[0].Items {
		if it.Kind == book.ItemText {
			c.TextAt(faces.Face(it.Style, it.Px), pv.Min.X+it.X, pv.Min.Y+it.Y, it.Text, kobo.Black, pv)
		}
	}
}
