package main

// The style panel in ink (bookstyle.go has what it sets): the lists'
// header (‹ All Books, Style, back to the book); an EPUB's fonts, each a
// tile in itself, the one picked black; the size, the spacing and the
// margins stepped with − and +; justified and hyphens, switches; the
// pages and the bar, dropdowns; and a paragraph set as the books will
// be. A PDF's: the fit, the zoom, the trimming, the pages, the bar.
// What a control picked is done after the frame: it lays the book out
// again, and draws the window.

import (
	"fmt"
	"image"

	"hellobro/book"
	"hellobro/ink"
	"hellobro/ink/inkkobo"
	"hellobro/ink/layout"
	"hellobro/ink/text"
	"hellobro/ink/theme"
	"hellobro/ink/widget"
	"hellobro/kobo"
)

type styleUI struct {
	a     *bro
	w     *bookWin
	pane  *inkkobo.Pane
	head  libHeadUI
	fonts []widget.Clickable
	minus [3]widget.Clickable // size, spacing, margins; a PDF's zoom the first
	plus  [3]widget.Clickable
	just  widget.Bool
	hyph  widget.Bool
	trim  widget.Bool
	fit   widget.Select
	pages widget.Select
	bar   widget.Select
	list  layout.Scroll
	do    func()

	laidFor string // what the sample was set for
	laid    []book.Page
}

// styleOf is the panel's ink, made the first time.
func (w *bookWin) styleOf(a *bro) *styleUI {
	if w.style == nil {
		u := &styleUI{a: a, w: w}
		u.pane = inkkobo.NewPane(u.Layout)
		w.style = u
	}
	return w.style
}

func (u *styleUI) inkPane() *inkkobo.Pane { return u.pane }

func (u *styleUI) take() {
	if do := u.do; do != nil {
		u.do = nil
		do()
	}
}

// drawStyle draws the panel over the window's body, all of it.
func (w *bookWin) drawStyle(a *bro) {
	u := w.styleOf(a)
	u.pane.Place(w.p.Body)
	u.pane.Win.Invalidate()
	u.pane.Draw(a.C.Gray)
}

// stepper is a row: its label, then − value +.
func (u *styleUI) stepper(gtx ink.Ctx, label, value string, minus, plus *widget.Clickable) ink.Dims {
	th := u.a.theme()
	return layout.Inset{Top: th.Pad / 2, Bottom: th.Pad / 2}.Layout(gtx, func(gtx ink.Ctx) ink.Dims {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx ink.Ctx) ink.Dims {
				return layout.Inset{Left: th.Pad * 1.5}.Layout(gtx, theme.Body(th, label).Layout)
			}),
			layout.Rigid(theme.Button(th, minus, "−").Layout),
			layout.Rigid(func(gtx ink.Ctx) ink.Dims {
				gtx.Constraints = ink.Exact(image.Pt(gtx.Dp(100), gtx.Dp(th.Touch)))
				l := theme.Body(th, value)
				l.Align = theme.Center
				return layout.Center.Layout(gtx, l.Layout)
			}),
			layout.Rigid(theme.Button(th, plus, "+").Layout))
	})
}

func (u *styleUI) Layout(gtx ink.Ctx) ink.Dims {
	a, w, th := u.a, u.w, u.a.theme()
	st := a.bookStyle()
	pdf := w.pdf != nil
	hs := libHeadSpec{all: "All Books", toAll: func() { w.styling = false; w.toShelf(a) },
		back: "back to the book", toBack: func() { w.closeStyle(a) }, tabs: []libTab{{"Style", true, func() {}}}}
	if do := u.head.taps(gtx, hs); do != nil {
		u.do = do
	}
	fams := w.fontNames(a)
	for len(u.fonts) < len(fams) {
		u.fonts = append(u.fonts, widget.Clickable{})
	}
	step := func(i, d int) func() {
		if pdf {
			return func() { w.pdfStyleStep(a, i, d) }
		}
		return func() { w.styleStep(a, len(fams)+i, d) }
	}
	pageWords := make([]string, len(bookPageModes))
	for k, m := range bookPageModes {
		pageWords[k] = bookPageWords[m]
	}
	bars := []string{fullBarWord(true), fullBarWord(false)}
	barAt := 1
	if a.fullBar(w.barKind()) {
		barAt = 0
	}
	if !pdf {
		for i, f := range fams {
			if u.fonts[i].Clicked(gtx) {
				u.do = func() { w.pickFont(a, f.name) }
			}
		}
		for k, i := range []int{styleSize, styleLeading, styleMargin} {
			if u.minus[k].Clicked(gtx) {
				u.do = step(i, -1)
			}
			if u.plus[k].Clicked(gtx) {
				u.do = step(i, 1)
			}
		}
		if u.just.Update(gtx) {
			u.do = step(styleJustify, 0)
		}
		if u.hyph.Update(gtx) {
			u.do = step(styleHyph, 0)
		}
		u.just.Value, u.hyph.Value = !st.ragged, !st.noHyphs
	} else {
		if u.minus[0].Clicked(gtx) {
			u.do = step(pdfStyleZoom, -1)
		}
		if u.plus[0].Clicked(gtx) {
			u.do = step(pdfStyleZoom, 1)
		}
		if u.fit.Changed(gtx, 2) && (u.fit.Value == 1) != st.pdfWidth {
			u.do = step(pdfStyleFit, 0)
		}
		if u.trim.Update(gtx) {
			u.do = step(pdfStyleTrim, 0)
		}
		if !u.fit.IsOpen() {
			u.fit.Value = map[bool]int{false: 0, true: 1}[st.pdfWidth]
		}
		u.trim.Value = !st.pdfNoTrim
	}
	if u.pages.Changed(gtx, len(bookPageModes)) {
		mode := bookPageModes[u.pages.Value]
		u.do = func() { w.setPageMode(a, mode) }
	}
	if u.bar.Changed(gtx, 2) && u.bar.Value != barAt {
		u.do = func() { w.flipBar(a) }
	}
	if !u.pages.IsOpen() {
		u.pages.Value = max(0, indexOf(bookPageModes, w.pageMode))
	}
	if !u.bar.IsOpen() {
		u.bar.Value = barAt
	}

	var rows []ink.Widget
	if pdf {
		rows = append(rows,
			theme.Select(th, &u.fit, "Fit", "whole page", "page width").Layout,
			func(gtx ink.Ctx) ink.Dims {
				return u.stepper(gtx, "Zoom", fmt.Sprintf("%d%%", w.zoom), &u.minus[0], &u.plus[0])
			},
			theme.Toggle(th, &u.trim, "Trim white margins").Layout)
	} else {
		rows = append(rows, func(gtx ink.Ctx) ink.Dims { return u.fontTiles(gtx, fams) })
		vals := st.values()
		for k, i := range []int{styleSize, styleLeading, styleMargin} {
			rows = append(rows, func(gtx ink.Ctx) ink.Dims {
				return u.stepper(gtx, styleLabels[i], vals[i], &u.minus[k], &u.plus[k])
			})
		}
		rows = append(rows, theme.Toggle(th, &u.just, styleLabels[styleJustify]).Layout,
			theme.Toggle(th, &u.hyph, styleLabels[styleHyph]).Layout)
	}
	rows = append(rows, theme.Select(th, &u.pages, "Pages", pageWords...).Layout,
		theme.Select(th, &u.bar, "Bar, fullscreen", bars...).Layout)
	if pdf {
		l := theme.Small(th, "a page bigger than the screen is read in screenfuls, across then down; + and - zoom too; two side by side are whole")
		rows = append(rows, func(gtx ink.Ctx) ink.Dims {
			return layout.Inset{Top: th.Pad, Left: th.Pad * 1.5}.Layout(gtx, l.Layout)
		})
	}

	size := gtx.Constraints.Max
	ink.Fill(gtx.Ops, image.Rectangle{Max: size}, th.Bg)
	rowH := libRowHeight(gtx, th)
	return layout.UniformInset(th.Pad).Layout(gtx, func(gtx ink.Ctx) ink.Dims {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx ink.Ctx) ink.Dims { return u.head.layout(gtx, th, hs, rowH) }),
			layout.Rigid(layout.Spacer{Height: th.Pad}.Layout),
			layout.Flexed(1, func(gtx ink.Ctx) ink.Dims {
				room := gtx.Constraints.Max.Y
				if !pdf { // the sample last, in what room the controls leave
					used := 0
					mg := gtx.Measuring()
					mg.Constraints = ink.Constraints{Min: image.Pt(gtx.Constraints.Max.X, 0), Max: image.Pt(gtx.Constraints.Max.X, 1<<20)}
					for _, r := range rows {
						m := ink.Record(mg.Ops) // measured, not drawn
						used += r(mg).Size.Y
						m.Stop()
					}
					h := max(room-used, gtx.Dp(120))
					rows = append(rows, func(gtx ink.Ctx) ink.Dims { return u.sample(gtx, st, h) })
				}
				u.list.Axis = layout.Vertical
				return theme.List(th, &u.list).Layout(gtx, len(rows), func(gtx ink.Ctx, i int) ink.Dims { return rows[i](gtx) })
			}))
	})
}

// fontTiles are the fonts, each its name in itself, in two columns if
// there's room; the one picked black.
func (u *styleUI) fontTiles(gtx ink.Ctx, fams []bookFamily) ink.Dims {
	a, w, th := u.a, u.w, u.a.theme()
	width := gtx.Constraints.Max.X
	cols := 1
	if width >= gtx.Dp(535) {
		cols = 2
	}
	tileH := gtx.Dp(40)
	gap := gtx.Dp(th.Pad / 2)
	colW := width / cols
	for i, f := range fams {
		r := image.Rect(i%cols*colW+gap, i/cols*tileH+gap, (i%cols+1)*colW-gap, (i/cols+1)*tileH)
		st := ink.Offset(gtx.Ops, r.Min)
		cg := gtx
		cg.Constraints = ink.Exact(r.Size())
		clk, picked, face := &u.fonts[i], w.fontPicked(a, f), w.tileFace(a, f)
		clk.Layout(cg, func(gtx ink.Ctx) ink.Dims {
			b := image.Rectangle{Max: r.Size()}
			fg := th.Fg
			if picked {
				ink.Fill(gtx.Ops, b, th.Fg)
				fg = th.Bg
			} else {
				ink.Fill(gtx.Ops, b, th.Bg)
				ink.Frame(gtx.Ops, b, max(gtx.Dp(th.Border), 2)+boolInt(clk.Focused()), th.Fg)
			}
			pad := gtx.Dp(th.Pad * 1.5)
			ink.TextAt(gtx.Ops, face, pad, (b.Dy()-text.Height(face))/2+text.Ascent(face), text.Fit(face, f.name, b.Dx()-2*pad), fg)
			if clk.Pressed() {
				ink.Invert(gtx.Ops, b)
			}
			return ink.Dims{Size: b.Size()}
		})
		st.Pop()
	}
	rows := (len(fams) + cols - 1) / cols
	return ink.Dims{Size: image.Pt(width, rows*tileH+gap*2)}
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// sample is a paragraph as the books will be set, h high, as wide as
// the page's text: the book's own from its page, then Alice.
func (u *styleUI) sample(gtx ink.Ctx, st bookStyle, h int) ink.Dims {
	a, w, th := u.a, u.w, u.a.theme()
	width := gtx.Constraints.Max.X
	ink.Fill(gtx.Ops, image.Rect(0, gtx.Dp(th.Pad), width, gtx.Dp(th.Pad)+2), ink.Light)
	pw := min(width, w.area.Dx())
	top := gtx.Dp(th.Pad * 2)
	size := image.Pt(pw, h-top)
	if size.Y < 2*st.px || gtx.IsMeasuring() {
		return ink.Dims{Size: image.Pt(width, h)}
	}
	faces := w.bookFaces(a)
	key := fmt.Sprint(size, st, w.spine, w.at, w.noOwn, faces)
	if key != u.laidFor {
		sample, lang := w.sample()
		u.laidFor, u.laid = key, book.Layout(sample, st.settings(size, lang), faces, nil)
	}
	pages := u.laid
	x := (width - pw) / 2
	ink.Draw(gtx.Ops, image.Rect(x, top, x+size.X, top+size.Y), func(dst *image.Gray, r image.Rectangle) {
		if len(pages) == 0 {
			return
		}
		c := kobo.Canvas{Gray: dst}
		for _, it := range pages[0].Items {
			if it.Kind == book.ItemText {
				c.TextAt(faces.Face(it.Style, it.Px), r.Min.X+it.X, r.Min.Y+it.Y, it.Text, kobo.Black, r)
			}
		}
	})
	return ink.Dims{Size: image.Pt(width, h)}
}

// styleKey: + and - the size (a PDF's zoom); j k as ↓ ↑; the rest
// ink's: Tab and the arrows go through the controls, Enter or Space
// presses one; Esc, s or q back to the book.
func (w *bookWin) styleKey(a *bro, code uint16, ch string, value int32) bool {
	if value == 1 {
		switch {
		case (ch == "+" || ch == "=") && w.pdf != nil:
			w.zoomStep(a, 1)
			return true
		case ch == "-" && w.pdf != nil:
			w.zoomStep(a, -1)
			return true
		case ch == "+" || ch == "=":
			w.styleStep(a, len(w.fontNames(a))+styleSize, 1)
			return true
		case ch == "-":
			w.styleStep(a, len(w.fontNames(a))+styleSize, -1)
			return true
		}
	}
	switch ch {
	case "j":
		code = keyDown
	case "k":
		code = keyUp
	}
	u := w.styleOf(a)
	if u.pane.Key(code, value, inkMods(a.tv.mods), a.tv.mods.caps) {
		if u.pane.At == w.p.Body && a.shows(w) && !a.overlay() {
			u.pane.Win.Invalidate()
			a.flushPane(u.pane)
			u.take()
		}
		return true
	}
	if value == 1 && (code == kobo.KeyEsc || ch == "s" || ch == "q") {
		w.closeStyle(a)
		return true
	}
	return false
}
