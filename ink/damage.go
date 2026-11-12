package ink

import (
	"bytes"
	"image"
)

// What changed between two frames, as a few rectangles: e-ink pays by
// the area it updates, and a pixel's change may be a cheap one (black
// and white: DU) or not (grays: GL16).

const tile = 32 // the diff's grain, in pixels

// diff is what changed from a to b (the same size), tile by tile, joined
// into rows of tiles and the rows into blocks.
func diff(a, b *image.Gray) []Update {
	r := b.Rect
	cols := (r.Dx() + tile - 1) / tile
	var out []Update
	for y0 := r.Min.Y; y0 < r.Max.Y; y0 += tile {
		y1 := min(y0+tile, r.Max.Y)
		run, runGray := -1, false
		flush := func(tx int) {
			if run >= 0 {
				x0, x1 := r.Min.X+run*tile, min(r.Min.X+tx*tile, r.Max.X)
				out = append(out, Update{R: image.Rect(x0, y0, x1, y1), Gray: runGray})
			}
			run, runGray = -1, false
		}
		for tx := range cols {
			x0, x1 := r.Min.X+tx*tile, min(r.Min.X+(tx+1)*tile, r.Max.X)
			changed, gray := false, false
			for y := y0; y < y1; y++ {
				ra := a.Pix[a.PixOffset(x0, y):][:x1-x0]
				rb := b.Pix[b.PixOffset(x0, y):][:x1-x0]
				if !changed && !bytes.Equal(ra, rb) {
					changed = true
				}
				if !gray {
					gray = hasGray(rb)
				}
				if changed && gray {
					break
				}
			}
			if !changed {
				flush(tx)
				continue
			}
			if run < 0 {
				run = tx
			}
			runGray = runGray || gray
		}
		flush(cols)
	}
	return out
}

func hasGray(p []byte) bool {
	for _, v := range p {
		if v != 0 && v != 0xff {
			return true
		}
	}
	return false
}

// mergeUpdates joins updates that overlap or nearly touch, when the
// union isn't much bigger than they are: small changes stay small, a
// column of rows becomes one block.
func mergeUpdates(us []Update) []Update {
	for merged := true; merged; {
		merged = false
		for i := 0; i < len(us); i++ {
			for j := i + 1; j < len(us); j++ {
				a, b := us[i].R, us[j].R
				u := a.Union(b)
				if area(u) <= (area(a)+area(b))*5/4 || a.Inset(-1).Overlaps(b) && area(u) <= (area(a)+area(b))*3/2 {
					us[i] = Update{R: u, Gray: us[i].Gray || us[j].Gray, Full: us[i].Full || us[j].Full, Pen: us[i].Pen || us[j].Pen}
					us = append(us[:j], us[j+1:]...)
					merged = true
					j--
				}
			}
		}
	}
	return us
}

func area(r image.Rectangle) int { return r.Dx() * r.Dy() }
