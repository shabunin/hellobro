package theme

import (
	"image"
	"strings"

	"hellobro/ink"
	"hellobro/ink/layout"
	"hellobro/ink/text"
	"hellobro/ink/widget"
)

// Column is a table's: its head, its share of the width, where its
// words go.
type Column struct {
	Title  string
	Weight float32 // 0: 1
	Align  Align
}

// TableStyle is a table: heads to sort by over rows that move under the
// finger. Mark is underlined where the cells have it (what was searched
// for); Empty is said when there are no rows.
type TableStyle struct {
	Theme   *Theme
	Table   *widget.Table
	Columns []Column
	Mark    string
	Empty   string
}

// Table is t as a table of columns.
func Table(th *Theme, t *widget.Table, columns ...Column) TableStyle {
	return TableStyle{Theme: th, Table: t, Columns: columns, Empty: "nothing here"}
}

// widths shares w out among the columns.
func (t TableStyle) widths(w int) []int {
	total := float32(0)
	for _, c := range t.Columns {
		total += max(c.Weight, 0) + float32(boolInt(c.Weight <= 0))
	}
	ws := make([]int, len(t.Columns))
	used := 0
	for i, c := range t.Columns {
		wt := c.Weight
		if wt <= 0 {
			wt = 1
		}
		ws[i] = int(float32(w) * wt / total)
		used += ws[i]
	}
	if len(ws) > 0 {
		ws[len(ws)-1] += w - used
	}
	return ws
}

// Layout lays out the heads and the n rows that show, filling what it's
// given; cell is what row i says in column c.
func (t TableStyle) Layout(gtx ink.Ctx, n int, cell func(i, c int) string) ink.Dims {
	th, tb := t.Theme, t.Table
	tb.Update(gtx, len(t.Columns), n)
	size := gtx.Constraints.Max
	pad := gtx.Dp(th.Pad)
	b := th.border(gtx)
	hh := gtx.Dp(th.Touch)
	rh := gtx.Dp(th.Touch * 0.9)
	f := th.Face(gtx, th.TextSize*0.9, text.Regular)
	hf := th.Face(gtx, th.TextSize*0.85, text.Bold)
	// The heads line up with the rows: as wide as they are, the bar aside.
	w := size.X
	if from, to := tb.Body.Span(); n > 0 && (from > 0 || to < 1) {
		w -= th.bar(gtx)
	}
	ws := t.widths(w)
	x := 0
	for c, col := range t.Columns {
		r := image.Rect(x, 0, x+ws[c], hh)
		st := ink.Offset(gtx.Ops, r.Min)
		cg := gtx
		cg.Constraints = ink.Exact(r.Size())
		tb.Heads[c].Layout(cg, func(gtx ink.Ctx) ink.Dims {
			rr := image.Rectangle{Max: r.Size()}
			ink.Fill(gtx.Ops, rr, th.Bg)
			arrow := gtx.Dp(th.TextSize * 0.5)
			room := max(rr.Dx()-2*pad-arrow-pad/2, 0)
			title := text.Fit(hf, col.Title, room)
			tw := text.Width(hf, title)
			// The title where the column's words go, the arrow beside it.
			tx, ax := pad, pad+tw+pad/2
			switch col.Align {
			case Right:
				tx = rr.Dx() - pad - tw
				ax = tx - pad/2 - arrow
			case Center:
				tx = (rr.Dx() - tw - arrow - pad/2) / 2
				ax = tx + tw + pad/2
			}
			ink.TextAt(gtx.Ops, hf, tx, (hh+text.Ascent(hf)-hf.Metrics().Descent.Ceil())/2, title, th.Fg)
			if c == tb.Sort {
				ay := (hh - arrow/2) / 2
				if tb.Desc {
					ink.Polygon(gtx.Ops, th.Fg, image.Pt(ax, ay), image.Pt(ax+arrow, ay), image.Pt(ax+arrow/2, ay+arrow/2))
				} else {
					ink.Polygon(gtx.Ops, th.Fg, image.Pt(ax, ay+arrow/2), image.Pt(ax+arrow, ay+arrow/2), image.Pt(ax+arrow/2, ay))
				}
			}
			if tb.Heads[c].Pressed() {
				ink.Invert(gtx.Ops, rr)
			}
			return ink.Dims{Size: rr.Size()}
		})
		st.Pop()
		x += ws[c]
	}
	ink.Fill(gtx.Ops, image.Rect(0, hh, size.X, hh+2*b), th.Fg)
	top := hh + 2*b
	body := image.Pt(size.X, max(size.Y-top, 0))
	st := ink.Offset(gtx.Ops, image.Pt(0, top))
	if n == 0 {
		cg := gtx
		cg.Constraints = ink.Exact(image.Pt(size.X, rh*2))
		line(cg, f, t.Empty, th.Quiet, Center, rh*2)
	}
	cg := gtx
	cg.Constraints = ink.Exact(body)
	List(th, &tb.Body).Layout(cg, n, func(gtx ink.Ctx, i int) ink.Dims {
		cg := gtx
		cg.Constraints = ink.Exact(image.Pt(gtx.Constraints.Max.X, rh))
		return tb.Rows[i].Layout(cg, func(gtx ink.Ctx) ink.Dims {
			rr := image.Rectangle{Max: gtx.Constraints.Min}
			ink.Fill(gtx.Ops, rr, th.Bg)
			x := 0
			for c, col := range t.Columns {
				cw := ws[c]
				if c == len(t.Columns)-1 {
					cw = rr.Dx() - x
				}
				s := text.Fit(f, cell(i, c), max(cw-2*pad, 0))
				tw := text.Width(f, s)
				tx := x + pad
				switch col.Align {
				case Center:
					tx = x + (cw-tw)/2
				case Right:
					tx = x + cw - pad - tw
				}
				base := (rh + text.Ascent(f) - f.Metrics().Descent.Ceil()) / 2
				ink.TextAt(gtx.Ops, f, tx, base, s, th.Fg)
				if t.Mark != "" {
					if k := strings.Index(strings.ToLower(s), strings.ToLower(t.Mark)); k >= 0 && k+len(t.Mark) <= len(s) {
						x0 := tx + text.Width(f, s[:k])
						x1 := tx + text.Width(f, s[:k+len(t.Mark)])
						ink.Fill(gtx.Ops, image.Rect(x0, base+b+1, x1, base+3*b+1), th.Fg)
					}
				}
				x += ws[c]
			}
			ink.Fill(gtx.Ops, image.Rect(0, rh-1, rr.Dx(), rh), th.Quiet)
			if tb.Rows[i].Pressed() {
				ink.Invert(gtx.Ops, rr)
			}
			return ink.Dims{Size: rr.Size()}
		})
	})
	st.Pop()
	return ink.Dims{Size: size}
}

// TreeStyle is nodes in rows, a branch's kids under it, indented, lines
// joining them; the rows move under the finger.
type TreeStyle struct {
	Theme *Theme
	Tree  *widget.Tree
	Nodes []widget.Node
}

// Tree is t as a tree of nodes.
func Tree(th *Theme, t *widget.Tree, nodes []widget.Node) TreeStyle { return TreeStyle{th, t, nodes} }

// Layout lays out the rows that show, filling what it's given.
func (t TreeStyle) Layout(gtx ink.Ctx) ink.Dims {
	th, tr := t.Theme, t.Tree
	rows := tr.Visible(t.Nodes)
	tr.Update(gtx)
	rows = tr.Visible(t.Nodes) // as the taps left them
	pad := gtx.Dp(th.Pad * 1.5)
	ind := gtx.Dp(th.Touch * 0.6)
	b := th.border(gtx)
	f := th.Face(gtx, th.TextSize, text.Regular)
	fb := th.Face(gtx, th.TextSize, text.Bold)
	small := th.Face(gtx, th.TextSize*0.8, text.Regular)
	rh := gtx.Dp(th.Touch)
	return List(th, &tr.Body).Layout(gtx, len(rows), func(gtx ink.Ctx, i int) ink.Dims {
		row := rows[i]
		n := row.Node
		clk := tr.Row(n.Key)
		cg := gtx
		cg.Constraints = ink.Exact(image.Pt(gtx.Constraints.Max.X, rh))
		return clk.Layout(cg, func(gtx ink.Ctx) ink.Dims {
			rr := image.Rectangle{Max: gtx.Constraints.Min}
			w := rr.Dx()
			ink.Fill(gtx.Ops, rr, th.Bg)
			col := func(d int) int { return pad + d*ind + ind/2 } // depth d's line
			mid := rh / 2
			// Lines on from the ancestors that have kids after this one.
			for d := 0; d < row.Depth-1; d++ {
				if !row.Last[d+1] {
					ink.Fill(gtx.Ops, image.Rect(col(d), 0, col(d)+1, rh), th.Quiet)
				}
			}
			if d := row.Depth - 1; d >= 0 {
				end := rh
				if row.Last[row.Depth] {
					end = mid
				}
				ink.Fill(gtx.Ops, image.Rect(col(d), 0, col(d)+1, end), th.Quiet)
				ink.Fill(gtx.Ops, image.Rect(col(d), mid, col(d)+ind/2+ind/4, mid+1), th.Quiet)
			}
			x := pad + row.Depth*ind
			face := f
			if n.IsBranch() {
				face = fb
				a := ind / 2
				cx := x + ind/2
				if tr.Open[n.Key] {
					ink.Polygon(gtx.Ops, th.Fg, image.Pt(cx-a/2, mid-a/4), image.Pt(cx+a/2, mid-a/4), image.Pt(cx, mid+a/3))
					ink.Fill(gtx.Ops, image.Rect(col(row.Depth), mid+a/3+b, col(row.Depth)+1, rh), th.Quiet) // down to its kids
				} else {
					ink.Polygon(gtx.Ops, th.Fg, image.Pt(cx-a/4, mid-a/2), image.Pt(cx-a/4, mid+a/2), image.Pt(cx+a/3, mid))
				}
			} else {
				ink.Disc(gtx.Ops, image.Pt(x+ind/2, mid), max(b*2, 3), th.Quiet)
			}
			x += ind + pad/2
			end := w - pad
			if n.Sub != "" {
				sw := min(text.Width(small, n.Sub), (end-x)/2)
				s := text.Fit(small, n.Sub, sw)
				ink.TextAt(gtx.Ops, small, end-text.Width(small, s), (rh+text.Ascent(small)-small.Metrics().Descent.Ceil())/2, s, th.Quiet)
				end -= sw + pad
			}
			ink.TextAt(gtx.Ops, face, x, (rh+text.Ascent(face)-face.Metrics().Descent.Ceil())/2, text.Fit(face, n.Title, max(end-x, 0)), th.Fg)
			if clk.Pressed() {
				ink.Invert(gtx.Ops, rr)
			}
			return ink.Dims{Size: rr.Size()}
		})
	})
}

// GridListStyle is a list whose items are cells in rows, as many across
// as fit Grid's MinWidth: covers, say. By the page or under the finger,
// as List is.
type GridListStyle struct {
	Theme *Theme
	List  layout.Scroller
	Grid  layout.Grid
}

// GridList is l in rows of g's cells.
func GridList(th *Theme, l layout.Scroller, g layout.Grid) GridListStyle {
	return GridListStyle{th, l, g}
}

// Layout lays out the rows that show of n cells, el laying out cell i.
func (s GridListStyle) Layout(gtx ink.Ctx, n int, el func(gtx ink.Ctx, i int) ink.Dims) ink.Dims {
	gap := gtx.Dp(s.Grid.Gap)
	cols := s.Grid.Columns
	if cols <= 0 {
		w := gtx.Constraints.Max.X - s.Theme.bar(gtx)
		cols = max(1, (w+gap)/max(gtx.Dp(s.Grid.MinWidth)+gap, 1))
	}
	g := s.Grid
	g.Columns = cols
	rows := (n + cols - 1) / cols
	return List(s.Theme, s.List).Layout(gtx, rows, func(gtx ink.Ctx, r int) ink.Dims {
		return layout.Inset{Bottom: s.Grid.Gap}.Layout(gtx, func(gtx ink.Ctx) ink.Dims {
			return g.Layout(gtx, min(cols, n-r*cols), func(gtx ink.Ctx, c int) ink.Dims { return el(gtx, r*cols+c) })
		})
	})
}
