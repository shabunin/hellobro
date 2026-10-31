package main

// A cheat sheet over a book or a notebook: ? puts up its keys and its
// ":" commands, in columns as many as the screen's width holds; any key,
// or a tap, takes it down.

import (
	"image"
	"strings"

	"hellobro/kobo"
)

const (
	sheetHead = "\x00" // a row's name that makes it a heading
	sheetRowH = 38
	sheetColW = 560 // a column at least this wide
)

// showSheet puts up title's sheet: keys, then cmds as ":" would take them.
func (a *bro) showSheet(title string, keys [][2]string, cmds []exCmd) {
	b := a.screenFree()
	box := b.Inset(40)
	box.Max.X = box.Min.X + min(box.Dx(), 1800)
	box = box.Add(image.Pt((b.Dx()-box.Dx())/2-box.Min.X+b.Min.X, 0))
	q := &infoBox{title: title, rows: sheetRows(keys, cmds), box: box, sheet: true}
	bw, by := 240, box.Max.Y-100
	q.closeR = image.Rect(box.Max.X-32-bw, by, box.Max.X-32, by+68)
	a.info = q
	a.drawInfo()
}

// sheetRows are keys under a heading, then the commands under theirs.
func sheetRows(keys [][2]string, cmds []exCmd) [][2]string {
	rows := append([][2]string{{sheetHead, "Keys"}}, keys...)
	rows = append(rows, [2]string{sheetHead, "After \":\" (Enter runs it)"}, [2]string{":N  :N%", "a page; that far in"})
	for _, c := range cmds {
		if c.run == nil {
			continue
		}
		ns := strings.Fields(c.names)
		name := ":" + ns[0]
		if c.arg != "" {
			name += " " + c.arg
		}
		help := c.help
		if len(ns) > 1 {
			help += "  (:" + strings.Join(ns[1:], " :") + ")"
		}
		rows = append(rows, [2]string{name, help})
	}
	return rows
}

// sheetLayout deals rows into as few columns as hold them, of rowsMax
// rows at most, colsMax columns at most, as even as they come.
func sheetLayout(rows [][2]string, rowsMax, colsMax int) [][][2]string {
	rowsMax, colsMax = max(1, rowsMax), max(1, colsMax)
	n := min(colsMax, (len(rows)+rowsMax-1)/rowsMax)
	for per := min(rowsMax, (len(rows)+n-1)/max(1, n)); ; per++ {
		if cols := sheetCols(rows, per); len(cols) <= n || per >= rowsMax {
			return cols
		}
	}
}

// sheetCols deals rows into columns of at most per, a heading never
// last in one: it goes on with what it heads.
func sheetCols(rows [][2]string, per int) [][][2]string {
	var cols [][][2]string
	var col [][2]string
	for i, r := range rows {
		full := len(col) >= per || r[0] == sheetHead && len(col) >= per-1 && i+1 < len(rows)
		if full && len(col) > 0 {
			cols, col = append(cols, col), nil
		}
		col = append(col, r)
	}
	if len(col) > 0 {
		cols = append(cols, col)
	}
	return cols
}

// drawSheet draws the sheet's rows in in, under its title.
func (a *bro) drawSheet(in image.Rectangle) {
	q, c := a.info, a.C
	area := image.Rect(in.Min.X, in.Min.Y+72, in.Max.X, q.closeR.Min.Y-12)
	colsMax, gap := max(1, area.Dx()/sheetColW), 40
	cols := sheetLayout(q.rows, area.Dy()/sheetRowH, colsMax)
	n := min(len(cols), colsMax)
	cw := (area.Dx() - (n-1)*gap) / n
	for i, col := range cols {
		if i >= n {
			break // a short screen: the rest are left out
		}
		x := area.Min.X + i*(cw+gap)
		nameW := 0
		for _, r := range col {
			if r[0] != sheetHead {
				nameW = max(nameW, kobo.TextWidth(a.F.Small, r[0]))
			}
		}
		nameW = min(nameW, cw*2/5)
		y := area.Min.Y
		for _, r := range col {
			if r[0] == sheetHead {
				c.Text(a.F.Body, image.Rect(x, y, x+cw, y+sheetRowH), r[1], kobo.Black, kobo.AlignLeft)
				c.Fill(image.Rect(x, y+sheetRowH-4, x+cw, y+sheetRowH-2), kobo.Mid)
			} else {
				c.Text(a.F.Small, image.Rect(x, y, x+nameW, y+sheetRowH), r[0], kobo.Black, kobo.AlignLeft)
				c.Text(a.F.Small, image.Rect(x+nameW+20, y, x+cw, y+sheetRowH), r[1], kobo.Dark, kobo.AlignLeft)
			}
			y += sheetRowH
		}
	}
	c.Fill(q.closeR, kobo.Black)
	c.Text(a.F.Small, q.closeR, "close  (any key)", kobo.White, kobo.AlignCenter)
	a.Mark(q.box, true)
}

// bookKeys are a book's keys, for its sheet: an EPUB's, or a PDF's.
func bookKeys(pdf bool) [][2]string {
	turn := [][2]string{
		{"Space → ↓ PgDn  j l n", "the next page"},
		{"← ↑ PgUp ⌫  h k p", "the page before"},
	}
	ks := append(turn, [][2]string{
		{"Home  End", "the chapter's first page, its last"},
		{"+  −", "the text bigger, smaller"},
		{"f  F", "the next font, the one before"},
		{"s", "the style: size, font, margins…"},
	}...)
	if pdf {
		ks = append(turn, [][2]string{
			{"+  −", "zoom in, out"},
			{"o", "the pages, small"},
			{"s", "the style: fit, margins, pages, the bar…"},
		}...)
	}
	return append(ks, [][2]string{
		{"t  Enter", "the contents"},
		{"Tab", "the contents, the bookmarks, round"},
		{"b", "a bookmark here, or not"},
		{":", "a command (below)"},
		{"?", "this sheet"},
		{"q", "the book put away: the shelf"},
		{"Esc", "a cut, the tool put down; else the shelf"},
	}...)
}

// noteKeys are a notebook's keys, for its sheet.
func noteKeys() [][2]string {
	return [][2]string{
		{"→ ↓ PgDn", "the next page"},
		{"← ↑ PgUp", "the page before"},
		{"p  e  s  t", "pen, eraser, select, text"},
		{"o", "the pages, small (again: back)"},
		{"Ctrl+Z", "undo; Ctrl+Shift+Z, Ctrl+Y: redo"},
		{"Ctrl+C  X  D", "the selection copied, cut, doubled"},
		{"Del  ⌫", "the selection gone"},
		{"Ctrl+V", "paste"},
		{"Ctrl+E", "export: a PDF or a PNG"},
		{":", "a command (below)"},
		{"?", "this sheet"},
		{"q", "the notebook put away: All Notebooks"},
	}
}
