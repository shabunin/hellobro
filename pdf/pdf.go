// Package pdf reads PDFs for hellobro's reader, through MuPDF (built by
// build-mupdf.sh, linked in with cgo): their pages drawn gray, at any
// size, a part of one at a time if need be; their outline; their text.
// Without cgo (the PC's tests) Open says so and that's all.
package pdf

import (
	"errors"
	"image"
)

// ErrNoMuPDF is Open's without cgo: no MuPDF to read with.
var ErrNoMuPDF = errors.New("this build reads no PDFs (no MuPDF in it)")

// Entry is a line of the outline: its page, how deep it is.
type Entry struct {
	Title string
	Page  int
	Depth int
}

// Size is a page's size in points (1/72 inch).
type Size struct{ W, H float64 }

// Scaled is the size of the page drawn zoom times its size in points.
func (s Size) Scaled(zoom float64) image.Point {
	return image.Pt(int(s.W*zoom+0.5), int(s.H*zoom+0.5))
}

// Ink is where the page has ink in m, drawn of it: the bounds of what
// isn't (nearly) white, empty if nothing is.
func Ink(m *image.Gray) image.Rectangle {
	const white = 0xf0
	b := m.Bounds()
	ink := image.Rectangle{}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		row := m.Pix[(y-b.Min.Y)*m.Stride:][:b.Dx()]
		for x, g := range row {
			if g < white {
				ink = ink.Union(image.Rect(b.Min.X+x, y, b.Min.X+x+1, y+1))
				break
			}
		}
		for x := len(row) - 1; x >= 0; x-- {
			if row[x] < white {
				ink = ink.Union(image.Rect(b.Min.X+x, y, b.Min.X+x+1, y+1))
				break
			}
		}
	}
	return ink
}
