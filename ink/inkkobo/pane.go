package inkkobo

import (
	"image"
	"time"

	"hellobro/ink"
	"hellobro/kobo"
)

// Pane is an ink Window at a place on a kobo.App's canvas: a whole app's
// page (Host), or one window among others (hellobro's). It takes the
// contacts that come down on it, and keys handed to it, and puts what
// changed on the canvas, marked for the panel.
type Pane struct {
	Win *ink.Window
	At  image.Rectangle // on the canvas; empty: it doesn't show

	contacts map[*kobo.Gesture]int
	next     int
}

// NewPane is a pane showing ui, not placed yet.
func NewPane(ui ink.Widget) *Pane {
	return &Pane{Win: ink.NewWindow(image.Pt(1, 1), Metric(), ui), contacts: map[*kobo.Gesture]int{}}
}

// Place puts it at r, laid out for it at the next frame.
func (p *Pane) Place(r image.Rectangle) {
	p.At = r
	if r.Empty() {
		for g := range p.contacts { // gone with it: no lift will come here
			delete(p.contacts, g)
		}
		return
	}
	p.Win.Metric = Metric()
	p.Win.Resize(r.Size())
	p.Win.Invalidate()
}

// Draw paints it all on c, as it is now (the caller marks it).
func (p *Pane) Draw(c *image.Gray) {
	if p.At.Empty() {
		return
	}
	p.Win.Invalidate()
	p.Win.Frame(time.Now())
	kobo.CopyGray(c, p.At, p.Win.Image(), image.Point{})
}

// Flush lays it out if it's due and puts what changed on a's canvas,
// marked: grays as the App's UI, black and white in the fast waveform.
// It says if any of it was ink written (the pen's waveform wanted).
func (p *Pane) Flush(a *kobo.App) (pen bool) { return p.FlushTo(a.C.Gray, a.Mark) }

// FlushTo is Flush onto c, each change marked by mark: a host whose
// marks do more (draw what floats over the pane again).
func (p *Pane) FlushTo(c *image.Gray, mark func(r image.Rectangle, ui bool)) (pen bool) {
	if p.At.Empty() {
		return false
	}
	for _, u := range together(p.Win.Frame(time.Now())) {
		r := u.R.Add(p.At.Min)
		kobo.CopyGray(c, r, p.Win.Image(), u.R.Min)
		mark(r, u.Gray || u.Full)
		pen = pen || u.Pen
	}
	return pen
}

// together makes a frame's updates one, if they're most of what they
// span: a list scrolled changes its labels and its values, two columns,
// and the panel showing one, then the other, has them out of step for a
// moment.
func together(us []ink.Update) []ink.Update {
	if len(us) < 2 {
		return us
	}
	all, sum := ink.Update{}, 0
	for _, u := range us {
		all.R = all.R.Union(u.R)
		all.Gray, all.Full, all.Pen = all.Gray || u.Gray, all.Full || u.Full, all.Pen || u.Pen
		sum += u.R.Dx() * u.R.Dy()
	}
	if sum*2 < all.R.Dx()*all.R.Dy() {
		return us
	}
	return []ink.Update{all}
}

// Ring is the focus ring, on the canvas; empty: none.
func (p *Pane) Ring() image.Rectangle {
	r := p.Win.Ring()
	if r.Empty() || p.At.Empty() {
		return image.Rectangle{}
	}
	return r.Add(p.At.Min)
}

// Down takes g, coming down at pt, if it's on the pane: its moves and
// lift are the pane's then (owner marks it so).
func (p *Pane) Down(ev kobo.TouchEv, pt image.Point, g *kobo.Gesture, owner int) bool {
	if g == nil || !pt.In(p.At) {
		return false
	}
	g.Owner = owner
	g.NoLight()
	p.next++
	p.contacts[g] = p.next
	p.Win.Pointer(pointer(ink.Press, p.next, pt.Sub(p.At.Min), ev))
	return true
}

// Move: one of the pane's contacts moved. Laid out at the next flush: a
// frame a move would be too many.
func (p *Pane) Move(ev kobo.TouchEv, pt image.Point, g *kobo.Gesture) bool {
	id, ok := p.contacts[g]
	if ok {
		p.Win.Pointer(pointer(ink.Move, id, pt.Sub(p.At.Min), ev))
	}
	return ok
}

// Lift: one of the pane's contacts lifted; false if it wasn't one.
func (p *Pane) Lift(g *kobo.Gesture) bool {
	id, ok := p.contacts[g]
	if !ok {
		return false
	}
	delete(p.contacts, g)
	p.Win.Pointer(ink.Pointer{Kind: ink.Release, ID: id, Pos: g.Last.Sub(p.At.Min), Time: time.Now()})
	return true
}

// Has says g is one of its contacts.
func (p *Pane) Has(g *kobo.Gesture) bool { _, ok := p.contacts[g]; return ok }

// Keys ink knows by name.
var keyNames = map[uint16]string{
	kobo.KeyEsc: "Esc", kobo.KeyBackspace: "Back", kobo.KeyTab: "Tab", kobo.KeyEnter: "Enter", kobo.KeySpace: "Space",
	102: "Home", 103: "Up", 104: "PageUp", 105: "Left", 106: "Right", 107: "End", 108: "Down", 109: "PageDown", 111: "Delete",
}

// Mods by their keys' codes.
var modKeys = map[uint16]ink.Mods{
	kobo.KeyLeftShift: ink.Shift, kobo.KeyRightShift: ink.Shift, kobo.KeyLeftCtrl: ink.Ctrl, kobo.KeyRightCtrl: ink.Ctrl,
	56: ink.Alt, 100: ink.Alt, 125: ink.Super, 126: ink.Super,
}

// ModOf is the modifier code is, if it's one.
func ModOf(code uint16) (ink.Mods, bool) { m, ok := modKeys[code]; return m, ok }

// Key hands a key (not a modifier: mods are those held) to ink; true if
// it took it. A key coming up is taken, quietly.
func (p *Pane) Key(code uint16, value int32, mods ink.Mods, caps bool) bool {
	if value == 0 {
		return true
	}
	if name, ok := keyNames[code]; ok {
		return p.Win.Key(ink.Key{Name: name, Mods: mods, Repeat: value == 2})
	}
	if ch, ok := kobo.KeyChar(code, mods&ink.Shift != 0, caps); ok {
		if mods&^ink.Shift == 0 {
			return p.Win.Text(ch)
		}
		return p.Win.Key(ink.Key{Name: lower(ch), Mods: mods})
	}
	return false
}

func lower(s string) string {
	if len(s) == 1 && s[0] >= 'A' && s[0] <= 'Z' {
		return string(s[0] + 'a' - 'A')
	}
	return s
}

// pointer is ev as ink has it: the pen's pressure (the Elipsa's goes to
// 4095) and its button.
func pointer(k ink.PointerKind, id int, p image.Point, ev kobo.TouchEv) ink.Pointer {
	e := ink.Pointer{Kind: k, ID: id, Pos: p, Source: source(ev), Time: ev.At}
	if ev.Pen {
		e.Pressure = min(float32(ev.Pressure)/4095, 1)
		e.Eraser = ev.Button != kobo.PenNoButton
	}
	return e
}

func source(ev kobo.TouchEv) ink.Source {
	if ev.Pen {
		return ink.Pen
	}
	return ink.Finger
}
