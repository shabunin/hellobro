package widget

import (
	"image"

	"hellobro/ink"
)

// Popover is what opens over everything from a field: a dropdown's
// options, a menu, a calendar. A tap on the field opens it (or Enter,
// Space, ↓ in focus), a tap outside or Esc closes it; while it's open
// the keys are its own.
type Popover struct {
	Field Clickable // what opens it

	open     bool
	backdrop backdrop
}

type backdrop struct{ _ byte }

var popoverKeys = []string{"Down"}

// IsOpen says it shows.
func (p *Popover) IsOpen() bool { return p.open }

// Open opens it, the keys in it.
func (p *Popover) Open(gtx ink.Ctx) {
	p.open = true
	gtx.Focus(p)
	gtx.Invalidate()
}

// Close closes it, the keys back on its field.
func (p *Popover) Close(gtx ink.Ctx) {
	if p.open {
		p.open = false
		gtx.Focus(&p.Field)
		gtx.Invalidate()
	}
}

// toggled takes the field's taps and the backdrop's: true if it's to
// open (the caller opens it, set up as it wants).
func (p *Popover) toggled(gtx ink.Ctx) bool {
	p.Field.Keys = popoverKeys
	open := false
	for p.Field.Clicked(gtx) {
		if p.open {
			p.Close(gtx)
			open = false
		} else {
			open = true
		}
	}
	for _, k := range p.Field.TakeKeys(gtx) {
		if k.Name == "Down" && !p.open {
			open = true
		}
	}
	for _, e := range gtx.Events(&p.backdrop) {
		if e, ok := e.(ink.Pointer); ok && e.Kind == ink.Press {
			p.Close(gtx)
			open = false
		}
	}
	return open
}

// keys are the keys it took while open, Esc (which closes it) aside.
func (p *Popover) keys(gtx ink.Ctx) []ink.Key {
	var ks []ink.Key
	for _, e := range gtx.Events(p) {
		k, ok := e.(ink.Key)
		if !ok {
			continue
		}
		if k.Name == "Esc" || k.Name == "Back" {
			p.Close(gtx)
			continue
		}
		ks = append(ks, k)
	}
	return ks
}

// Popup lays it out by anchor (its field, where it is), if it's open: w
// lays out what's in it, size big at most.
func (p *Popover) Popup(gtx ink.Ctx, anchor image.Rectangle, size image.Point, w ink.Widget) {
	if !p.open {
		return
	}
	cg := gtx
	cg.Constraints = ink.Constraints{Min: image.Pt(size.X, 0), Max: size}
	m := ink.Record(gtx.Ops)
	d := w(cg)
	in := m.Stop()
	// Under what's in it, its own area: it has the keys.
	m = ink.Record(gtx.Ops)
	ink.Input(gtx.Ops, ink.Area{Rect: image.Rectangle{Max: d.Size}, Tag: p, Focus: true, Keys: []string{"*"}})
	in.Add(gtx.Ops)
	ink.Popup(gtx.Ops, anchor, d.Size, &p.backdrop, m.Stop())
}
