package kobo

import "image"

// The focus ring: a dashed frame around the control the keys go to, as a
// desktop draws one when Tab moves through a window. It's never drawn
// into C: flush puts it on what goes to the panel. So what's drawn under
// it gets it back by itself, and moving it is redrawing two frames.

const (
	RingOut = 7 // a ring around a control: its outer edge, this far out
	ringW   = 3
	dash    = 12 // black, then a gap half as long
)

// SetRing puts the focus ring at r (its outer edge), or takes it off (an
// empty r).
func (a *App) SetRing(r image.Rectangle) {
	if r == a.ring {
		return
	}
	for _, s := range ringStrips(a.ring) {
		a.Mark(s, true)
	}
	a.ring = r
	for _, s := range ringStrips(r) {
		a.Mark(s, true)
	}
}

// TabRect is where tab's place in the tab strip is; empty if it doesn't
// show.
func (a *App) TabRect(tab int) image.Rectangle {
	for i, t := range a.Tabs() {
		if t == tab && i < len(a.L.tabs) && !a.Full {
			return a.L.tabs[i]
		}
	}
	return image.Rectangle{}
}

func ringStrips(o image.Rectangle) []image.Rectangle {
	if o.Dx() < 2*ringW || o.Dy() < 2*ringW {
		return nil
	}
	return []image.Rectangle{
		image.Rect(o.Min.X, o.Min.Y, o.Max.X, o.Min.Y+ringW),
		image.Rect(o.Min.X, o.Max.Y-ringW, o.Max.X, o.Max.Y),
		image.Rect(o.Min.X, o.Min.Y+ringW, o.Min.X+ringW, o.Max.Y-ringW),
		image.Rect(o.Max.X-ringW, o.Min.Y+ringW, o.Max.X, o.Max.Y-ringW),
	}
}

// stampRing draws the ring o into img, as far as img goes.
func stampRing(img *image.Gray, o image.Rectangle) {
	for i, s := range ringStrips(o) {
		s = s.Intersect(img.Rect)
		for y := s.Min.Y; y < s.Max.Y; y++ {
			row := img.Pix[img.PixOffset(s.Min.X, y):][:s.Dx()]
			for x := s.Min.X; x < s.Max.X; x++ {
				along := x - o.Min.X // the top and bottom dash along x, the sides along y
				if i >= 2 {
					along = y - o.Min.Y
				}
				if along%(dash*3/2) < dash {
					row[x-s.Min.X] = Black
				}
			}
		}
	}
}
