package main

// The reader's style panel, in the book's window over its page: the
// fonts, each shown in itself; the size, the lines' spacing, the
// margins, justified or not, hyphens or not; the pages, this book's
// (one, two side by side, or two if the window's wide); the book's own
// text, from its page, set as it will be; the bar, fullscreen. s or Aa on the bar
// opens it, Esc or Done goes back to the page. It's the reader's own:
// the app's Settings don't have it.

import (
	"fmt"
	"image"
	"os"
	"slices"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"

	"hellobro/book"
)

// The controls after the fonts, in order.
const (
	styleSize = iota
	styleLeading
	styleMargin
	styleJustify
	styleHyph
	stylePages
	styleBar
	styleDone
	styleN
)

var styleLabels = [styleN]string{"Size", "Line spacing", "Margins", "Justified", "Hyphenation", "Pages", "Bar, fullscreen", ""}

// bookSample is what the panel sets after the book's text, if there's room.
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

// tileFace is what the font's name is shown in, on its tile: itself.
func (w *bookWin) tileFace(a *bro, f bookFamily) font.Face {
	if f.name != bookOwnFont {
		return a.familyFace(f)
	}
	if w.ownTile == nil {
		if w.own == nil {
			w.own = newOwnFonts(w.b)
		}
		w.ownTile = newReaderFaces(a.bookFaces(), w.own).Face(book.Style(0).InFamily(max(1, mainFamily(w.ch))), 32)
	}
	return w.ownTile
}

// bookSampleMax is how much of a book the panel sets, in bytes: more than
// its room ever holds, as it's set on the main loop.
const bookSampleMax = 8000

// sample is the text to show the style by, and its language: the book's
// from the page it's open at, then Alice, for what room a short page
// leaves (or a page with no text, a cover). She's in what most of the
// chapter would be set in, if that's a font of the book's.
func (w *bookWin) sample() (*book.Chapter, string) {
	var blocks []book.Block
	lang := "en"
	if w.ch != nil && w.pagesFor.spine == w.spine {
		if blocks = w.ch.From(w.at, bookSampleMax); len(blocks) > 0 {
			lang = w.b.Lang
		}
	}
	fam := 0
	if !w.noOwn {
		fam = mainFamily(w.ch)
	}
	for _, b := range bookSample.Blocks {
		b.Spans = slices.Clone(b.Spans)
		for k := range b.Spans {
			b.Spans[k].Style = b.Spans[k].Style.InFamily(fam)
		}
		blocks = append(blocks, b)
	}
	return &book.Chapter{Blocks: blocks}, lang
}

func (w *bookWin) openStyle(a *bro) {
	w.styling, w.list = true, false
	if w.pdf != nil {
		w.sel = 0
		w.draw(a)
		return
	}
	w.sel = 0
	for i, f := range w.fontNames(a) {
		if w.fontPicked(a, f) {
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
	fams := w.fontNames(a)
	st := a.bookStyle()
	if i < len(fams) {
		if d != 0 {
			i = (i + d + len(fams)) % len(fams)
			w.sel = i
		}
		w.pickFont(a, fams[i].name)
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
	case stylePages:
		w.setPageMode(a, nextPages(w.pageMode, d))
		return
	case styleBar:
		w.flipBar(a)
		return
	case styleDone:
		w.closeStyle(a)
		return
	}
	a.setBookStyle(st)
}

func (st bookStyle) values() [styleN]string {
	onOff := map[bool]string{true: "on", false: "off"}
	return [styleN]string{
		fmt.Sprintf("%d px", st.px),
		fmt.Sprintf("%.2f", float64(st.leading)/100),
		fmt.Sprintf("%d%%", bookMargins[st.margin]),
		onOff[!st.ragged],
		onOff[!st.noHyphs],
		"", // the book's own: drawStyle
		"", // the EPUBs': drawStyle
		"",
	}
}

// settings are the layout's, for a page of size in a book in lang.
func (st bookStyle) settings(size image.Point, lang string) book.Settings {
	return book.Settings{W: size.X, H: size.Y, Px: st.px, Leading: float64(st.leading) / 100, Ragged: st.ragged, NoHyph: st.noHyphs, Lang: lang}
}
