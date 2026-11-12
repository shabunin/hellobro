// Package layout places widgets: in a row or a column (Flex), one over
// another (Stack), with room around (Inset), a page of a list at a time
// (List).
package layout

import (
	"image"

	"hellobro/ink"
)

// Axis is which way a Flex or List goes.
type Axis uint8

const (
	Horizontal Axis = iota
	Vertical
)

// main and cross are p along and across the axis.
func (a Axis) main(p image.Point) int {
	if a == Horizontal {
		return p.X
	}
	return p.Y
}

func (a Axis) cross(p image.Point) int {
	if a == Horizontal {
		return p.Y
	}
	return p.X
}

// pt is the point main along the axis, cross across it.
func (a Axis) pt(main, cross int) image.Point {
	if a == Horizontal {
		return image.Pt(main, cross)
	}
	return image.Pt(cross, main)
}

// Alignment is where a child goes across the axis.
type Alignment uint8

const (
	Start Alignment = iota
	End
	Middle
	Baseline
)

// Spacing is where the room left over goes, along the axis.
type Spacing uint8

const (
	SpaceEnd Spacing = iota
	SpaceStart
	SpaceSides
	SpaceAround
	SpaceBetween
	SpaceEvenly
)

// Flex lays its children in a row or a column: the Rigid ones as big as
// they want, then the Flexed share what's left by weight.
type Flex struct {
	Axis      Axis
	Spacing   Spacing
	Alignment Alignment
	WeightSum float32 // 0: the weights' sum
}

// FlexChild is a child of a Flex.
type FlexChild struct {
	flex   bool
	weight float32
	widget ink.Widget

	call ink.CallOp
	dims ink.Dims
}

// Rigid is a child as big as it wants.
func Rigid(w ink.Widget) FlexChild { return FlexChild{widget: w} }

// Flexed is a child taking weight's share of the room the rigid ones
// leave, all of it.
func Flexed(weight float32, w ink.Widget) FlexChild {
	return FlexChild{flex: true, weight: weight, widget: w}
}

// Layout lays the children out.
func (f Flex) Layout(gtx ink.Ctx, children ...FlexChild) ink.Dims {
	ax := f.Axis
	mainMax, mainMin := ax.main(gtx.Constraints.Max), ax.main(gtx.Constraints.Min)
	crossMax, crossMin := ax.cross(gtx.Constraints.Max), ax.cross(gtx.Constraints.Min)
	size, total := 0, float32(0)
	for i := range children {
		c := &children[i]
		if c.flex {
			total += c.weight
			continue
		}
		cgtx := gtx
		cgtx.Constraints = ink.Constraints{Min: ax.pt(0, crossMin), Max: ax.pt(max(mainMax-size, 0), crossMax)}
		m := ink.Record(gtx.Ops)
		c.dims = c.widget(cgtx)
		c.call = m.Stop()
		size += ax.main(c.dims.Size)
	}
	if f.WeightSum > 0 {
		total = f.WeightSum
	}
	room := max(mainMax-size, 0)
	left := room
	for i := range children {
		c := &children[i]
		if !c.flex {
			continue
		}
		share := room
		if total > 0 {
			share = int(float32(room) * c.weight / total)
		}
		share = min(share, left)
		cgtx := gtx
		cgtx.Constraints = ink.Constraints{Min: ax.pt(share, crossMin), Max: ax.pt(share, crossMax)}
		m := ink.Record(gtx.Ops)
		c.dims = c.widget(cgtx)
		c.call = m.Stop()
		left -= ax.main(c.dims.Size)
		size += ax.main(c.dims.Size)
	}
	cross, baseline := ax.cross(gtx.Constraints.Min), 0
	for _, c := range children {
		cross = max(cross, ax.cross(c.dims.Size))
		if f.Alignment == Baseline {
			baseline = max(baseline, c.dims.Size.Y-c.dims.Baseline)
		}
	}
	cross = min(cross, crossMax)
	if f.Alignment == Baseline && ax == Horizontal {
		// Lined up by their baselines, the row may be taller.
		for _, c := range children {
			cross = max(cross, baseline+c.dims.Baseline)
		}
	}
	space := 0
	if mainMin > size {
		space = mainMin - size
	}
	n := len(children)
	var before, between, after int
	switch f.Spacing {
	case SpaceEnd:
		after = space
	case SpaceStart:
		before = space
	case SpaceSides:
		before, after = space/2, space-space/2
	case SpaceAround:
		if n > 0 {
			between = space / n
			before = between / 2
			after = space - before - between*(n-1)
		}
	case SpaceBetween:
		if n > 1 {
			between = space / (n - 1)
			after = space - between*(n-1)
		} else {
			after = space
		}
	case SpaceEvenly:
		between = space / (n + 1)
		before = between
		after = space - between*n
	}
	at, lastEnd := before, 0
	for i, c := range children {
		cd := ax.cross(c.dims.Size)
		off := 0
		switch f.Alignment {
		case End:
			off = cross - cd
		case Middle:
			off = (cross - cd) / 2
		case Baseline:
			if ax == Horizontal {
				off = baseline - (c.dims.Size.Y - c.dims.Baseline)
			}
		}
		st := ink.Offset(gtx.Ops, ax.pt(at, off))
		c.call.Add(gtx.Ops)
		st.Pop()
		at += ax.main(c.dims.Size)
		lastEnd = at
		if i < n-1 {
			at += between
		}
	}
	at += after
	sz := gtx.Constraints.Constrain(ax.pt(at, cross))
	d := ink.Dims{Size: sz}
	if ax == Horizontal && f.Alignment == Baseline {
		d.Baseline = sz.Y - baseline
	} else if len(children) > 0 && ax == Horizontal {
		d.Baseline = children[0].dims.Baseline
	} else if len(children) > 0 {
		last := children[len(children)-1]
		d.Baseline = last.dims.Baseline + max(sz.Y-lastEnd, 0)
	}
	return d
}

// Inset is room around a widget.
type Inset struct {
	Top, Right, Bottom, Left ink.Dp
}

// UniformInset is v on every side.
func UniformInset(v ink.Dp) Inset { return Inset{v, v, v, v} }

// Layout lays w out inside it.
func (in Inset) Layout(gtx ink.Ctx, w ink.Widget) ink.Dims {
	t, r, b, l := gtx.Dp(in.Top), gtx.Dp(in.Right), gtx.Dp(in.Bottom), gtx.Dp(in.Left)
	c := gtx.Constraints
	shrink := image.Pt(l+r, t+b)
	c.Max = c.Max.Sub(shrink)
	c.Max.X, c.Max.Y = max(c.Max.X, 0), max(c.Max.Y, 0)
	c.Min = c.Min.Sub(shrink)
	c.Min.X, c.Min.Y = max(min(c.Min.X, c.Max.X), 0), max(min(c.Min.Y, c.Max.Y), 0)
	cgtx := gtx
	cgtx.Constraints = c
	st := ink.Offset(gtx.Ops, image.Pt(l, t))
	d := w(cgtx)
	st.Pop()
	return ink.Dims{Size: gtx.Constraints.Constrain(d.Size.Add(shrink)), Baseline: d.Baseline + b}
}

// Spacer is empty room, so wide and so high.
type Spacer struct {
	Width, Height ink.Dp
}

// Layout takes the room.
func (s Spacer) Layout(gtx ink.Ctx) ink.Dims {
	return ink.Dims{Size: gtx.Constraints.Constrain(image.Pt(gtx.Dp(s.Width), gtx.Dp(s.Height)))}
}

// Direction is where in a box a smaller thing goes.
type Direction uint8

const (
	NW Direction = iota
	N
	NE
	E
	SE
	S
	SW
	W
	Center
)

// Position is where something sz big goes in a box size big.
func (d Direction) Position(sz, size image.Point) image.Point {
	var p image.Point
	switch d {
	case N, S, Center:
		p.X = (size.X - sz.X) / 2
	case NE, E, SE:
		p.X = size.X - sz.X
	}
	switch d {
	case W, E, Center:
		p.Y = (size.Y - sz.Y) / 2
	case SW, S, SE:
		p.Y = size.Y - sz.Y
	}
	return p
}

// Layout lays w out as small as it likes, placed in the room it must
// take (its Min).
func (d Direction) Layout(gtx ink.Ctx, w ink.Widget) ink.Dims {
	cgtx := gtx
	cgtx.Constraints.Min = image.Point{}
	m := ink.Record(gtx.Ops)
	dims := w(cgtx)
	call := m.Stop()
	size := gtx.Constraints.Constrain(dims.Size) // at least Min: the room to place it in
	p := d.Position(dims.Size, size)
	st := ink.Offset(gtx.Ops, p)
	call.Add(gtx.Ops)
	st.Pop()
	return ink.Dims{Size: size, Baseline: dims.Baseline + size.Y - dims.Size.Y - p.Y}
}

// Stack lays children over one another: the Stacked ones as big as
// they like, then the Expanded ones the size of all of those.
type Stack struct {
	Alignment Direction
}

// StackChild is a child of a Stack.
type StackChild struct {
	expanded bool
	widget   ink.Widget
	call     ink.CallOp
	dims     ink.Dims
}

// Stacked is a child as big as it likes.
func Stacked(w ink.Widget) StackChild { return StackChild{widget: w} }

// Expanded is a child the size of the Stacked ones together (a
// background, a frame).
func Expanded(w ink.Widget) StackChild { return StackChild{expanded: true, widget: w} }

// Layout lays them out, the first at the bottom.
func (s Stack) Layout(gtx ink.Ctx, children ...StackChild) ink.Dims {
	var size image.Point
	base := 0
	for i := range children {
		c := &children[i]
		if c.expanded {
			continue
		}
		cgtx := gtx
		cgtx.Constraints.Min = image.Point{}
		m := ink.Record(gtx.Ops)
		c.dims = c.widget(cgtx)
		c.call = m.Stop()
		size.X, size.Y = max(size.X, c.dims.Size.X), max(size.Y, c.dims.Size.Y)
	}
	size = gtx.Constraints.Constrain(size)
	for i := range children {
		c := &children[i]
		if !c.expanded {
			continue
		}
		cgtx := gtx
		cgtx.Constraints = ink.Exact(size)
		m := ink.Record(gtx.Ops)
		c.dims = c.widget(cgtx)
		c.call = m.Stop()
	}
	for i, c := range children {
		p := s.Alignment.Position(c.dims.Size, size)
		st := ink.Offset(gtx.Ops, p)
		c.call.Add(gtx.Ops)
		st.Pop()
		if i == 0 || !c.expanded {
			base = c.dims.Baseline + size.Y - c.dims.Size.Y - p.Y
		}
	}
	return ink.Dims{Size: size, Baseline: base}
}

// Background lays w out, and bg behind it at w's size.
func Background(gtx ink.Ctx, bg, w ink.Widget) ink.Dims {
	return Stack{}.Layout(gtx, Expanded(bg), Stacked(w))
}

// Fill is a widget filling what it's given with gray g.
func Fill(g uint8) ink.Widget {
	return func(gtx ink.Ctx) ink.Dims {
		ink.Fill(gtx.Ops, image.Rectangle{Max: gtx.Constraints.Min}, g)
		return ink.Dims{Size: gtx.Constraints.Min}
	}
}

// Exact lays w out at exactly size (within what's allowed).
func Exact(gtx ink.Ctx, size image.Point, w ink.Widget) ink.Dims {
	gtx.Constraints = ink.Exact(gtx.Constraints.Constrain(size))
	return w(gtx)
}
