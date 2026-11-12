package theme

import (
	"fmt"
	"image"
	"strconv"

	"hellobro/ink"
	"hellobro/ink/text"
	"hellobro/ink/widget"
)

// stackUnder is the width, in Dp, under which a field's title goes over
// it rather than beside it.
const stackUnder = 480

// fieldRow lays a row out as a dropdown's is: title at its start, a box
// at its end (bw wide at most; 0: half the row, to 300 Dp) with value
// and a ▾, which clk taps; in a narrow row, the title over the box. ""
// title: only the box. It gives the box's place, for a popup to open
// under it.
func fieldRow(gtx ink.Ctx, th *Theme, title string, bw ink.Dp, clk *widget.Clickable, value string, open bool) (image.Rectangle, ink.Dims) {
	w := gtx.Constraints.Max.X
	pad := gtx.Dp(th.Pad * 1.5)
	h := gtx.Dp(th.Touch)
	size := image.Pt(w, h+gtx.Dp(th.Pad))
	box := image.Rect(0, (size.Y-h)/2, w, (size.Y+h)/2)
	if title != "" && w < gtx.Dp(stackUnder) {
		// No room for both side by side: the title, the box under it as
		// wide as the row.
		f := th.Face(gtx, th.TextSize, text.Regular)
		lh := text.Height(f) + gtx.Dp(th.Pad)
		size = image.Pt(w, lh+h+gtx.Dp(th.Pad))
		ink.Fill(gtx.Ops, image.Rectangle{Max: size}, th.Bg)
		cg := gtx
		cg.Constraints = ink.Exact(image.Pt(max(w-2*pad, 0), lh))
		st := ink.Offset(gtx.Ops, image.Pt(pad, 0))
		line(cg, f, title, th.Fg, Left, lh)
		st.Pop()
		box = image.Rect(pad/2, lh, w-pad/2, lh+h)
	} else if title != "" {
		b := min(w/2, gtx.Dp(300))
		if bw > 0 {
			b = min(w/2, gtx.Dp(bw))
		}
		box.Min.X, box.Max.X = w-b-pad/2, w-pad/2
		ink.Fill(gtx.Ops, image.Rectangle{Max: size}, th.Bg)
		f := th.Face(gtx, th.TextSize, text.Regular)
		cg := gtx
		cg.Constraints = ink.Exact(image.Pt(max(box.Min.X-2*pad, 0), size.Y))
		st := ink.Offset(gtx.Ops, image.Pt(pad, 0))
		line(cg, f, title, th.Fg, Left, size.Y)
		st.Pop()
	} else {
		size.Y = h
		box = image.Rectangle{Max: image.Pt(w, h)}
	}
	st := ink.Offset(gtx.Ops, box.Min)
	cg := gtx
	cg.Constraints = ink.Exact(box.Size())
	clk.Layout(cg, func(gtx ink.Ctx) ink.Dims {
		return field(gtx, th, value, clk.Pressed() || open, clk.Focused())
	})
	st.Pop()
	return box, ink.Dims{Size: size, Baseline: size.Y / 3}
}

// popupFrame lays out a popup's inside, framed on white: pad around in.
func popupFrame(gtx ink.Ctx, th *Theme, size image.Point, in func(gtx ink.Ctx)) ink.Dims {
	b := th.border(gtx)
	r := image.Rectangle{Max: size}
	ink.Fill(gtx.Ops, r, th.Bg)
	in(gtx)
	ink.Frame(gtx.Ops, r, 2*b, th.Fg)
	return ink.Dims{Size: size}
}

// cell lays out clk as a square of a grid, s in it: picked filled, hi
// framed (the keys are on it), quiet in gray, now ringed (today, say).
func cell(gtx ink.Ctx, th *Theme, clk *widget.Clickable, r image.Rectangle, s string, picked, hi, quiet, now bool) {
	st := ink.Offset(gtx.Ops, r.Min)
	cg := gtx
	cg.Constraints = ink.Exact(r.Size())
	clk.Layout(cg, func(gtx ink.Ctx) ink.Dims {
		size := r.Size()
		rr := image.Rectangle{Max: size}
		b := th.border(gtx)
		f := th.Face(gtx, th.TextSize*0.9, text.Regular)
		fg, bg := th.Fg, th.Bg
		if quiet {
			fg = th.Quiet
		}
		ink.Fill(gtx.Ops, rr, bg)
		c := image.Pt(size.X/2, size.Y/2)
		rad := min(size.X, size.Y)/2 - b
		if picked {
			f = th.Face(gtx, th.TextSize*0.9, text.Bold)
			ink.Disc(gtx.Ops, c, rad, th.Fg)
			fg = th.Bg
		} else if now {
			ink.Ring(gtx.Ops, c, rad, max(b, 2), th.Fg)
		}
		line(gtx, f, s, fg, Center, size.Y)
		if hi {
			ink.Frame(gtx.Ops, rr, 2*b, th.Fg)
		}
		if clk.Pressed() {
			ink.Invert(gtx.Ops, rr)
		}
		return ink.Dims{Size: size}
	})
	st.Pop()
}

// --- a date ---

// DateStyle is a row with a day in a box: a tap opens a month's calendar
// under it.
type DateStyle struct {
	Theme  *Theme
	Date   *widget.Date
	Title  string
	Format string // the day as it's said, in time's way; "": "Mon 2 Jan 2006"
	Hint   string // what the box says with no day picked
}

// Date is d as a row saying title.
func Date(th *Theme, d *widget.Date, title string) DateStyle {
	return DateStyle{Theme: th, Date: d, Title: title, Hint: "pick a day"}
}

var weekdays = []string{"Mo", "Tu", "We", "Th", "Fr", "Sa", "Su"}

// Layout lays it out.
func (s DateStyle) Layout(gtx ink.Ctx) ink.Dims {
	th, d := s.Theme, s.Date
	d.Update(gtx)
	value := s.Hint
	if !d.Value.IsZero() {
		format := s.Format
		if format == "" {
			format = "Mon 2 Jan 2006"
		}
		value = d.Value.Format(format)
	}
	box, dims := fieldRow(gtx, th, s.Title, 0, &d.Field, value, d.IsOpen())
	if !d.IsOpen() {
		return dims
	}
	cs := gtx.Dp(th.Touch)
	pad := gtx.Dp(th.Pad)
	small := th.Face(gtx, th.TextSize*0.8, text.Regular)
	wh := text.Height(small) + pad
	size := image.Pt(7*cs+2*pad, pad+cs+wh+6*cs+pad)
	today := gtx.Now
	ty, tm, td := today.Date()
	d.Popup(gtx, box, size, func(gtx ink.Ctx) ink.Dims {
		return popupFrame(gtx, th, size, func(gtx ink.Ctx) {
			// ‹ November 2026 ›
			bold := th.Face(gtx, th.TextSize, text.Bold)
			cg := gtx
			cg.Constraints = ink.Exact(image.Pt(7*cs, cs))
			st := ink.Offset(gtx.Ops, image.Pt(pad, pad))
			line(cg, bold, d.Month().Format("January 2006"), th.Fg, Center, cs)
			st.Pop()
			cell(gtx, th, &d.Prev, image.Rect(pad, pad, pad+cs, pad+cs), "‹", false, false, false, false)
			cell(gtx, th, &d.Next, image.Rect(pad+6*cs, pad, pad+7*cs, pad+cs), "›", false, false, false, false)
			y := pad + cs
			for i, w := range weekdays {
				cg.Constraints = ink.Exact(image.Pt(cs, wh))
				st := ink.Offset(gtx.Ops, image.Pt(pad+i*cs, y))
				line(cg, small, w, th.Quiet, Center, wh)
				st.Pop()
			}
			ink.Fill(gtx.Ops, image.Rect(pad, y+wh-1, pad+7*cs, y+wh), th.Quiet)
			y += wh
			_, mon, _ := d.Month().Date()
			for i := range d.Days {
				t := d.Day(i)
				yy, mm, dd := t.Date()
				r := image.Rect(pad+i%7*cs, y+i/7*cs, pad+(i%7+1)*cs, y+(i/7+1)*cs)
				cell(gtx, th, &d.Days[i], r, strconv.Itoa(dd),
					t.Equal(d.Value), t.Equal(d.Highlit()), mm != mon,
					yy == ty && mm == tm && dd == td)
			}
		})
	})
	return dims
}

// --- a time of day ---

// ClockStyle is a row with a time in a box: a tap opens the hours and
// the minutes under it.
type ClockStyle struct {
	Theme *Theme
	Clock *widget.Clock
	Title string
}

// Clock is c as a row saying title.
func Clock(th *Theme, c *widget.Clock, title string) ClockStyle { return ClockStyle{th, c, title} }

// Layout lays it out.
func (s ClockStyle) Layout(gtx ink.Ctx) ink.Dims {
	th, c := s.Theme, s.Clock
	c.Update(gtx)
	value := fmt.Sprintf("%02d:%02d", c.Hour, c.Minute)
	box, dims := fieldRow(gtx, th, s.Title, 160, &c.Field, value, c.IsOpen())
	if !c.IsOpen() {
		return dims
	}
	cs := gtx.Dp(th.Touch)
	pad := gtx.Dp(th.Pad)
	gap := gtx.Dp(th.Pad * 3)
	small := th.Face(gtx, th.TextSize*0.8, text.Regular)
	smallB := th.Face(gtx, th.TextSize*0.8, text.Bold)
	lh := text.Height(small) + pad
	st := c.StepOf()
	nm := 60 / st
	mrows := (nm + 3) / 4
	size := image.Pt(pad+6*cs+gap+4*cs+pad, pad+lh+max(4, mrows)*cs+pad)
	c.Popup(gtx, box, size, func(gtx ink.Ctx) ink.Dims {
		return popupFrame(gtx, th, size, func(gtx ink.Ctx) {
			head := func(x, w int, s string, on bool) {
				f, g := small, th.Quiet
				if on {
					f, g = smallB, th.Fg
				}
				cg := gtx
				cg.Constraints = ink.Exact(image.Pt(w, lh))
				o := ink.Offset(gtx.Ops, image.Pt(x, pad))
				line(cg, f, s, g, Left, lh)
				o.Pop()
				if on {
					ink.Fill(gtx.Ops, image.Rect(x, pad+lh-2, x+text.Width(f, s), pad+lh), th.Fg)
				}
			}
			head(pad, 6*cs, "hour", c.Column() == 0)
			mx := pad + 6*cs + gap
			head(mx, 4*cs, "minutes", c.Column() == 1)
			y := pad + lh
			for h := range c.Hours {
				r := image.Rect(pad+h%6*cs, y+h/6*cs, pad+(h%6+1)*cs, y+(h/6+1)*cs)
				cell(gtx, th, &c.Hours[h], r, fmt.Sprintf("%02d", h), h == c.Hour, false, false, false)
			}
			for i := 0; i < nm; i++ {
				r := image.Rect(mx+i%4*cs, y+i/4*cs, mx+(i%4+1)*cs, y+(i/4+1)*cs)
				cell(gtx, th, &c.Minutes[i], r, fmt.Sprintf(":%02d", i*st), i*st == c.Minute, false, false, false)
			}
			ink.Fill(gtx.Ops, image.Rect(mx-gap/2, y, mx-gap/2+1, y+4*cs), th.Quiet)
		})
	})
	return dims
}

// --- a range ---

// RangeStyle is a row saying what it is and the span, a track under them
// with two ends to move.
type RangeStyle struct {
	Theme *Theme
	Range *widget.Range
	Title string
	Value string
}

// RangeSlider is r as a slider saying title, its span value.
func RangeSlider(th *Theme, r *widget.Range, title, value string) RangeStyle {
	return RangeStyle{th, r, title, value}
}

// Layout lays it out.
func (s RangeStyle) Layout(gtx ink.Ctx) ink.Dims {
	th, rg := s.Theme, s.Range
	return sliderRow(gtx, th, s.Title, s.Value, func(gtx ink.Ctx) ink.Dims {
		return rg.Layout(gtx, func(gtx ink.Ctx) ink.Dims {
			size := gtx.Constraints.Min
			ink.Fill(gtx.Ops, image.Rectangle{Max: size}, th.Bg)
			b := th.border(gtx)
			mid := size.Y / 2
			k := gtx.Dp(th.Touch * 0.3)
			at := func(v float32) int { return min(max(int(float32(size.X)*rg.Frac(v)), k), size.X-k) }
			x0, x1 := at(rg.Low), at(rg.High)
			ink.Fill(gtx.Ops, image.Rect(0, mid-b, size.X, mid+b), th.Quiet)
			ink.Fill(gtx.Ops, image.Rect(x0, mid-2*b, x1, mid+2*b), th.Fg)
			for i, x := range []int{x0, x1} {
				ink.Disc(gtx.Ops, image.Pt(x, mid), k, th.Fg)
				if rg.Focused() && i == rg.Active() {
					ink.Disc(gtx.Ops, image.Pt(x, mid), k/2, th.Bg)
				}
			}
			return ink.Dims{Size: size}
		})
	})
}

// sliderRow lays a slider's row out: title and value over track.
func sliderRow(gtx ink.Ctx, th *Theme, title, value string, track ink.Widget) ink.Dims {
	w := gtx.Constraints.Max.X
	pad := gtx.Dp(th.Pad * 1.5)
	f := th.Face(gtx, th.TextSize, text.Regular)
	small := th.Face(gtx, th.TextSize*0.9, text.Regular)
	lh := text.Height(f) + gtx.Dp(th.Pad/2)
	cg := gtx
	cg.Constraints = ink.Exact(image.Pt(max(w-2*pad, 0), lh))
	st := ink.Offset(gtx.Ops, image.Pt(pad, 0))
	line(cg, f, title, th.Fg, Left, lh)
	line(cg, small, value, th.Quiet, Right, lh)
	st.Pop()
	h := gtx.Dp(th.Touch)
	st = ink.Offset(gtx.Ops, image.Pt(pad, lh))
	cg.Constraints = ink.Exact(image.Pt(max(w-2*pad, 0), h))
	track(cg)
	st.Pop()
	return ink.Dims{Size: image.Pt(w, lh+h)}
}
