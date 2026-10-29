// Package pdf reads PDFs for hellobro's reader, through MuPDF (built by
// build-mupdf.sh, linked in with cgo): their pages drawn gray, at any
// size, a part of one at a time if need be; their outline; their text.
// Without cgo (the PC's tests) Open says so and that's all.
package pdf

import (
	"errors"
	"image"
	"math"
	"strings"
	"unicode"
)

// ErrNoMuPDF is Open's without cgo: no MuPDF to read with.
var ErrNoMuPDF = errors.New("this build reads no PDFs (no MuPDF in it)")

// Entry is a line of the outline: its page, how deep it is.
type Entry struct {
	Title string
	Page  int
	Depth int
}

// Rect is a part of a page, in points from its top left.
type Rect struct{ X0, Y0, X1, Y1 float64 }

// Size is a page's size in points (1/72 inch).
type Size struct{ W, H float64 }

// What's drawn at most: a page's part fit to a window can be any size (a
// thin strip of ink fit to its width; a page a metre long), and the
// Kobo's memory isn't.
const (
	MaxPixels = 24 << 20 // 24 MB of gray
	MaxSide   = 16000    // px, along either side
)

// CapZoom is zoom, made smaller if a part of a page w × h points drawn
// at it would be more than MaxPixels or MaxSide; 0 if it can't be drawn
// at all (no size, a zoom that's no number).
func CapZoom(w, h, zoom float64) float64 {
	if !(w > 0 && h > 0 && zoom > 0) || math.IsInf(w, 0) || math.IsInf(h, 0) || math.IsInf(zoom, 0) {
		return 0
	}
	return min(zoom, MaxSide/w, MaxSide/h, math.Sqrt(MaxPixels/(w*h)))
}

// errSize is Render's for what it won't draw.
var errSize = errors.New("the page can't be drawn that size")

// checkDraw says whether clip, of a page drawn zoom times, can be drawn.
func checkDraw(zoom float64, clip image.Rectangle) error {
	switch {
	case !(zoom > 0) || math.IsInf(zoom, 0), clip.Empty(),
		clip.Dx() > MaxSide, clip.Dy() > MaxSide, clip.Dx()*clip.Dy() > MaxPixels:
		return errSize
	}
	return nil
}

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

// Char is a character on a page: its box, its line (a number, the same
// for a line's characters).
type Char struct {
	R    rune
	Line int
	Box  Rect
}

// TextIn is page i's text in r: in each line r crosses, the words whose
// letters it has the middles of, whole, the lines one under the other.
func (d *Doc) TextIn(i int, r Rect) (string, error) {
	cs, err := d.Chars(i)
	if err != nil {
		return "", err
	}
	return Words(cs, r), nil
}

// Words is the text of the words in r, of cs (TextIn).
func Words(cs []Char, r Rect) string {
	in := func(c Char) bool {
		x, y := (c.Box.X0+c.Box.X1)/2, (c.Box.Y0+c.Box.Y1)/2
		return x >= r.X0 && x <= r.X1 && y >= r.Y0 && y <= r.Y1
	}
	var lines []string
	for start := 0; start < len(cs); {
		end := start
		for end < len(cs) && cs[end].Line == cs[start].Line {
			end++
		}
		line := cs[start:end]
		start = end
		first, last := -1, -1
		for k, c := range line {
			if in(c) {
				if first < 0 {
					first = k
				}
				last = k
			}
		}
		if first < 0 {
			continue
		}
		for first > 0 && !unicode.IsSpace(line[first-1].R) {
			first--
		}
		for last < len(line)-1 && !unicode.IsSpace(line[last+1].R) {
			last++
		}
		var b strings.Builder
		for _, c := range line[first : last+1] {
			b.WriteRune(c.R)
		}
		if s := strings.TrimSpace(b.String()); s != "" {
			lines = append(lines, s)
		}
	}
	return strings.Join(lines, "\n")
}
