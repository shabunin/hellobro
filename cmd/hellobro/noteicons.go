package main

// The notebook's buttons are pictures: drawn here as lines on a grid of
// 24 units, scaled to the button, so they're as sharp on any screen and
// in the bigger frame for fingers. "clear" alone stays words, as it
// asks first.

import (
	"image"
	"math"

	"hellobro/kobo"
)

// iconPart is a piece of an icon, in grid units.
type iconPart struct {
	kind byte      // 'l' a line through pts; 'o' a ring and 'd' a dot (x, y, r), 'b' one of the paper; 'f' a box (x0, y0, x1, y1) of ink, 'w' of the paper
	pts  []float64 // its points
	w    float64   // a line's half width; 0: thin
}

func iconLine(w float64, pts ...float64) iconPart { return iconPart{'l', pts, w} }

// iconShut is a line round the points, back to the first.
func iconShut(w float64, pts ...float64) iconPart { return iconLine(w, append(pts, pts[0], pts[1])...) }

// iconArc is the points of a circle's arc from a0 to a1 degrees, clockwise
// on the screen (y goes down).
func iconArc(cx, cy, r, a0, a1 float64) []float64 {
	var pts []float64
	for i := range 17 {
		t := (a0 + (a1-a0)*float64(i)/16) * math.Pi / 180
		pts = append(pts, cx+r*math.Cos(t), cy+r*math.Sin(t))
	}
	return pts
}

// iconHead is an arrow's head at (x, y), pointing along (dx, dy).
func iconHead(x, y, dx, dy float64) iconPart {
	n := math.Hypot(dx, dy)
	dx, dy = dx/n*5, dy/n*5
	return iconLine(1, x-dx-dy*0.8, y-dy+dx*0.8, x, y, x-dx+dy*0.8, y-dy-dx*0.8)
}

// iconMirror is the icon flipped left to right.
func iconMirror(ps []iconPart) []iconPart {
	var out []iconPart
	for _, p := range ps {
		q := p
		q.pts = append([]float64(nil), p.pts...)
		for i := 0; i < len(q.pts); i += 2 {
			if p.kind == 'o' || p.kind == 'd' {
				q.pts[i] = 24 - q.pts[i]
				break
			}
			q.pts[i] = 24 - q.pts[i]
		}
		out = append(out, q)
	}
	return out
}

var (
	iconUndo = []iconPart{iconLine(1, append([]float64{5, 9, 15, 9}, append(iconArc(15, 14, 5, -90, 90), 9, 19)...)...), iconHead(5, 9, -1, 0)}
	iconCopy = []iconPart{iconShut(0.8, 4, 8, 14, 8, 14, 21, 4, 21), {kind: 'w', pts: []float64{9, 3, 19, 16}}, iconShut(0.8, 9, 3, 19, 3, 19, 16, 9, 16)}
	iconBin  = []iconPart{iconLine(1, 4, 6, 20, 6), iconLine(0.8, 9, 6, 9, 3, 15, 3, 15, 6), iconLine(0.8, 6, 6, 7, 21, 17, 21, 18, 6), iconLine(0.6, 10, 9, 10, 18), iconLine(0.6, 14, 9, 14, 18)}
)

// noteIcon is the picture of the button id; nil if it's words. paper
// is the notebook's, which its button shows.
func noteIcon(id, paper string) []iconPart {
	switch id {
	case "pen":
		return []iconPart{iconShut(0.7, 7, 13.5, 16.5, 4, 20, 7.5, 10.5, 17), iconShut(0.7, 7, 13.5, 4, 20, 10.5, 17), iconLine(0.7, 14, 6.5, 17.5, 10)}
	case "eraser":
		return []iconPart{iconShut(0.9, 3, 16, 11, 8, 18, 15, 10, 23), iconLine(0.9, 6.5, 19.5, 14.5, 11.5), iconLine(0.6, 10, 23, 21, 23)}
	case "select":
		return []iconPart{iconShut(0.5, 5, 6, 19, 6, 19, 18, 5, 18), {kind: 'd', pts: []float64{5, 6, 2}}, {kind: 'd', pts: []float64{19, 6, 2}},
			{kind: 'd', pts: []float64{19, 18, 2}}, {kind: 'd', pts: []float64{5, 18, 2}}}
	case "text": // a T
		return []iconPart{iconLine(1, 5, 5, 19, 5), iconLine(1, 12, 5, 12, 20), iconLine(0.7, 9, 20, 15, 20)}
	case "undo":
		return iconUndo
	case "redo":
		return iconMirror(iconUndo)
	case "paste":
		return []iconPart{iconShut(0.9, 6, 5, 18, 5, 18, 21, 6, 21), {kind: 'f', pts: []float64{9, 3, 15, 7.5}}, iconLine(0.5, 9, 12, 15, 12), iconLine(0.5, 9, 16, 15, 16)}
	case "paper":
		ps := []iconPart{iconShut(0.8, 6, 3, 18, 3, 18, 21, 6, 21)}
		if paper == "plain" {
			return append(ps, iconLine(0.5, 14, 3, 14, 7, 18, 7)) // a page, its corner turned
		}
		for y := 7.5; y < 21; y += 4.5 {
			ps = append(ps, iconLine(0, 6, y, 18, y))
		}
		if paper == "squares" {
			for x := 10.0; x < 18; x += 4 {
				ps = append(ps, iconLine(0, x, 3, x, 21))
			}
		}
		return ps
	case "export":
		return []iconPart{iconLine(0.9, 4, 13, 4, 21, 20, 21, 20, 13), iconLine(1, 12, 17, 12, 3), iconHead(12, 3, 0, -1)}
	case "open":
		return []iconPart{iconLine(1, 4, 9, 4, 4, 9, 4), iconLine(1, 15, 4, 20, 4, 20, 9), iconLine(1, 20, 15, 20, 20, 15, 20), iconLine(1, 9, 20, 4, 20, 4, 15)}
	case "new before":
		return []iconPart{iconShut(0.8, 11, 4, 20, 4, 20, 20, 11, 20), iconLine(1, 2, 12, 8, 12), iconLine(1, 5, 9, 5, 15)}
	case "new after":
		return iconMirror(noteIcon("new before", paper))
	case "copy", "copy page":
		return iconCopy
	case "duplicate":
		return append(append([]iconPart(nil), iconCopy...), iconLine(0.8, 11, 9.5, 17, 9.5), iconLine(0.8, 14, 6.5, 14, 12.5))
	case "cut":
		return []iconPart{{kind: 'o', pts: []float64{7, 18, 3}, w: 0.8}, {kind: 'o', pts: []float64{17, 18, 3}, w: 0.8}, iconLine(0.9, 9, 15.5, 17, 3), iconLine(0.9, 15, 15.5, 7, 3)}
	case "turn":
		end := 60.0 * math.Pi / 180
		return []iconPart{iconLine(1, iconArc(12, 12, 7, 150, 420)...), iconHead(12+7*math.Cos(end), 12+7*math.Sin(end), -math.Sin(end), math.Cos(end))}
	case "delete":
		return iconBin
	case "earlier":
		return []iconPart{iconLine(1, 20, 12, 4, 12), iconHead(4, 12, -1, 0)}
	case "later":
		return iconMirror(noteIcon("earlier", paper))
	}
	return nil
}

// drawIcon draws parts in the middle of r, in ink on bg.
func drawIcon(c kobo.Canvas, parts []iconPart, r image.Rectangle, ink, bg uint8) {
	s := float64(min(r.Dx(), r.Dy())) * 0.66 / 24
	m := r.Min.Add(r.Max).Div(2)
	at := func(x, y float64) image.Point {
		return image.Pt(m.X+int(math.Round((x-12)*s)), m.Y+int(math.Round((y-12)*s)))
	}
	px := func(u float64) int { return int(math.Round(u * s)) }
	for _, p := range parts {
		switch p.kind {
		case 'l':
			for i := 2; i+1 < len(p.pts); i += 2 {
				c.Line(at(p.pts[i-2], p.pts[i-1]), at(p.pts[i], p.pts[i+1]), px(p.w), ink, r)
			}
		case 'o':
			o, rad, w := at(p.pts[0], p.pts[1]), px(p.pts[2]), max(1, px(p.w))
			c.Disc(o, rad+w, ink, r)
			c.Disc(o, max(0, rad-w), bg, r)
		case 'd', 'b':
			g := ink
			if p.kind == 'b' {
				g = bg
			}
			c.Disc(at(p.pts[0], p.pts[1]), px(p.pts[2]), g, r)
		case 'f', 'w':
			g := ink
			if p.kind == 'w' {
				g = bg
			}
			c.Fill(image.Rectangle{at(p.pts[0], p.pts[1]), at(p.pts[2], p.pts[3])}.Canon().Intersect(r), g)
		}
	}
}
