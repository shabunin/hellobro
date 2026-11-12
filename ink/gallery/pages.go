package gallery

import (
	"cmp"
	"fmt"
	"image"
	"slices"
	"strconv"
	"strings"

	"hellobro/ink"
	"hellobro/ink/layout"
	"hellobro/ink/text"
	"hellobro/ink/theme"
	"hellobro/ink/widget"
)

// scrollPage is a page of text in a box that moves under the finger, and
// a strip of covers that moves sideways.
func (g *Gallery) scrollPage(gtx ink.Ctx) ink.Dims {
	th := g.Th
	for i := range g.stripBtn {
		if g.stripBtn[i].Clicked(gtx) {
			g.Said = "the strip: " + books[i].Title
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(theme.Section(th, "A page of text: it moves under the finger")),
		layout.Flexed(1, func(gtx ink.Ctx) ink.Dims {
			return layout.Inset{Left: th.Pad * 1.5, Top: th.Pad}.Layout(gtx, theme.ScrollBox(th, &g.box, func(gtx ink.Ctx) ink.Dims {
				return layout.Inset{Right: th.Pad * 1.5}.Layout(gtx, func(gtx ink.Ctx) ink.Dims {
					ws := []layout.FlexChild{layout.Rigid(theme.Title(th, "Moving under the finger").Layout)}
					for _, p := range longText {
						ws = append(ws, layout.Rigid(layout.Spacer{Height: th.Pad * 1.5}.Layout), layout.Rigid(theme.Body(th, p).Layout))
					}
					ws = append(ws, layout.Rigid(layout.Spacer{Height: th.Pad * 3}.Layout), layout.Rigid(theme.Title(th, "The keys").Layout))
					for _, k := range keyRows {
						r := theme.Row(th, nil, k[0])
						r.Value = k[1]
						ws = append(ws, layout.Rigid(r.Layout))
					}
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, ws...)
				})
			}))
		}),
		layout.Rigid(theme.Section(th, "A strip, sideways")),
		layout.Rigid(func(gtx ink.Ctx) ink.Dims {
			h := gtx.Dp(200)
			gtx.Constraints = ink.Exact(image.Pt(gtx.Constraints.Max.X, h+gtx.Dp(th.Pad*2)))
			return layout.UniformInset(th.Pad).Layout(gtx, func(gtx ink.Ctx) ink.Dims {
				return g.strip.Layout(gtx, len(books), func(gtx ink.Ctx, i int) ink.Dims {
					return layout.Inset{Right: th.Pad}.Layout(gtx, func(gtx ink.Ctx) ink.Dims {
						gtx.Constraints = ink.Exact(image.Pt(gtx.Dp(130), gtx.Constraints.Max.Y))
						return g.cover(gtx, &g.stripBtn[i], i)
					})
				})
			})
		}),
	)
}

// cover draws book i as a cover to tap: a band across its top, its title,
// its author at its foot.
func (g *Gallery) cover(gtx ink.Ctx, clk *widget.Clickable, i int) ink.Dims {
	th := g.Th
	return clk.Layout(gtx, func(gtx ink.Ctx) ink.Dims {
		w := gtx.Constraints.Max.X
		h := gtx.Constraints.Min.Y
		if h == 0 {
			h = w * 4 / 3
		}
		size := image.Pt(w, h)
		r := image.Rectangle{Max: size}
		b := max(gtx.Dp(th.Border), 1)
		pad := gtx.Dp(th.Pad)
		ink.Fill(gtx.Ops, r, th.Bg)
		band := image.Rect(0, 0, w, h/5)
		switch i % 4 {
		case 0:
			ink.Fill(gtx.Ops, band, th.Fg)
		case 1:
			for y := 0; y < band.Max.Y; y += 3 * b {
				ink.Fill(gtx.Ops, image.Rect(0, y, w, y+b), th.Fg)
			}
		case 2:
			ink.Disc(gtx.Ops, image.Pt(w/2, band.Max.Y/2), band.Dy()/3, th.Fg)
		case 3:
			ink.Polygon(gtx.Ops, th.Fg, image.Pt(0, band.Max.Y), image.Pt(w/2, 0), image.Pt(w, band.Max.Y))
		}
		cg := gtx
		cg.Constraints = ink.Constraints{Max: image.Pt(max(w-2*pad, 0), h/2)}
		st := ink.Offset(gtx.Ops, image.Pt(pad, band.Max.Y+pad))
		t := theme.Label(th, th.TextSize*0.8, books[i].Title)
		t.Style, t.MaxLines = text.Bold, 3
		t.Layout(cg)
		st.Pop()
		small := th.Face(gtx, th.TextSize*0.65, text.Regular)
		ink.TextAt(gtx.Ops, small, pad, h-pad-small.Metrics().Descent.Ceil(), text.Fit(small, books[i].Author, max(w-2*pad, 0)), th.Quiet)
		ink.Frame(gtx.Ops, r, b+boolInt(clk.Focused()), th.Fg)
		if clk.Pressed() {
			ink.Invert(gtx.Ops, r)
		}
		return ink.Dims{Size: size}
	})
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// sortBooks puts the books that have what's searched for in the table's
// order.
func (g *Gallery) sortBooks() {
	q := strings.ToLower(strings.TrimSpace(g.find.Text()))
	g.shown = g.shown[:0]
	for i, b := range books {
		if q == "" || strings.Contains(strings.ToLower(b.Title), q) || strings.Contains(strings.ToLower(b.Author), q) {
			g.shown = append(g.shown, i)
		}
	}
	col, desc := g.table.Sort, g.table.Desc
	slices.SortStableFunc(g.shown, func(a, b int) int {
		x, y := books[a], books[b]
		var c int
		switch col {
		case 0:
			c = strings.Compare(x.Title, y.Title)
		case 1:
			c = strings.Compare(x.Author, y.Author)
		case 2:
			c = cmp.Compare(x.Year, y.Year)
		case 3:
			c = cmp.Compare(x.Pages, y.Pages)
		}
		if desc {
			c = -c
		}
		return c
	})
}

func (g *Gallery) tablePage(gtx ink.Ctx) ink.Dims {
	th := g.Th
	if g.find.Changed(gtx) || g.findClr.Clicked(gtx) {
		g.sortBooks()
		g.table.Body.ScrollTo(0)
		g.Said = fmt.Sprintf("%d of %d books", len(g.shown), len(books))
	}
	if g.table.Sorted(gtx) {
		g.sortBooks()
		g.Said = "sorted by " + strings.ToLower(bookCols[g.table.Sort]) + map[bool]string{true: ", the other way", false: ""}[g.table.Desc]
	}
	if i, ok := g.table.Clicked(gtx); ok && i < len(g.shown) {
		g.Said = "opened " + books[g.shown[i]].Title
	}
	search := theme.Editor(th, &g.find, "search the titles and authors")
	search.Clear = &g.findClr
	t := theme.Table(th, &g.table,
		theme.Column{Title: bookCols[0], Weight: 3}, theme.Column{Title: bookCols[1], Weight: 2},
		theme.Column{Title: bookCols[2], Weight: 0.8, Align: theme.Right}, theme.Column{Title: bookCols[3], Weight: 0.8, Align: theme.Right})
	t.Mark = strings.TrimSpace(g.find.Text())
	t.Empty = "no book has “" + t.Mark + "”"
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx ink.Ctx) ink.Dims {
			return layout.Inset{Left: th.Pad * 1.5, Right: th.Pad * 1.5, Top: th.Pad, Bottom: th.Pad}.Layout(gtx, search.Layout)
		}),
		layout.Rigid(func(gtx ink.Ctx) ink.Dims {
			return layout.Inset{Left: th.Pad * 1.5, Bottom: th.Pad}.Layout(gtx, theme.Small(th, fmt.Sprintf("%d of %d books; a head sorts by it, again the other way", len(g.shown), len(books))).Layout)
		}),
		layout.Flexed(1, func(gtx ink.Ctx) ink.Dims {
			return t.Layout(gtx, len(g.shown), func(i, c int) string {
				b := books[g.shown[i]]
				switch c {
				case 0:
					return b.Title
				case 1:
					return b.Author
				case 2:
					return strconv.Itoa(b.Year)
				}
				return strconv.Itoa(b.Pages)
			})
		}),
	)
}

func (g *Gallery) treePage(gtx ink.Ctx) ink.Dims {
	th := g.Th
	if k, ok := g.tree.Picked(gtx); ok {
		g.Said = "picked " + k
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx ink.Ctx) ink.Dims {
			return layout.Inset{Left: th.Pad * 1.5, Top: th.Pad, Bottom: th.Pad}.Layout(gtx, theme.Small(th, "a folder opens and closes; → and ← too, in focus").Layout)
		}),
		layout.Flexed(1, theme.Tree(th, &g.tree, tree).Layout),
	)
}
