package main

// The Battery and What uses it windows, in ink (inkwin.go): what the
// last sample said (power.go), the busiest processes, the battery's last
// hours as a graph. A sample asks for a frame; ink sends the panel only
// the lines that changed.

import (
	"fmt"
	"image"
	"strconv"
	"strings"
	"time"

	"hellobro/ink"
	"hellobro/ink/layout"
	"hellobro/ink/text"
	"hellobro/ink/theme"
)

type batteryUI struct {
	a    *bro
	list layout.Scroll
}

func (u *batteryUI) Status() string {
	if b := u.a.pw.bat; b.ok && !u.a.pw.at.IsZero() {
		return fmt.Sprintf("%d%%", b.pct) // the current is in the window: here it'd change the strip each time
	}
	return ""
}

// quiet is s said aside, in gray.
func quiet(th *theme.Theme, s string) ink.Widget {
	l := theme.Body(th, s)
	l.Gray = th.Quiet
	return para(th, l.Layout)
}

// para is w as words in a list: in from its sides as far as its rows'
// words are.
func para(th *theme.Theme, w ink.Widget) ink.Widget {
	return func(gtx ink.Ctx) ink.Dims {
		return layout.Inset{Left: th.Pad * 1.5, Right: th.Pad * 1.5}.Layout(gtx, w)
	}
}

func (u *batteryUI) Layout(gtx ink.Ctx) ink.Dims {
	a, th := u.a, u.a.theme()
	ps := &a.pw
	b := ps.bat
	var rows []ink.Widget
	gap := layout.Spacer{Height: th.Pad / 2}.Layout
	switch {
	case ps.at.IsZero():
		rows = append(rows, quiet(th, "looking…"))
	case !b.ok:
		rows = append(rows, quiet(th, "no battery found"))
	default:
		word := "on battery"
		if b.charging() {
			word = strings.ToLower(b.status)
		}
		rows = append(rows, para(th, theme.Title(th, fmt.Sprintf("%d%% · %s", b.pct, word)).Layout), gap)
		flow := "drawing"
		if b.charging() {
			flow = "taking in"
		}
		rows = append(rows, theme.Progress(th, float32(b.pct)/100,
			fmt.Sprintf("%s %d mA · %.2f W at %.2f V", flow, b.ma, float64(b.ma)*float64(b.mv)/1e6, float64(b.mv)/1000)).Layout, gap)
		gauge := ""
		switch {
		case b.charging() && b.toFull > 0 && b.toFull < 24*time.Hour:
			gauge = "the gauge: full in " + hoursMins(b.toFull)
		case !b.charging() && b.toEmpty > 0:
			gauge = "the gauge: " + hoursMins(b.toEmpty) + " left"
		}
		rows = append(rows, para(th, theme.Body(th, strings.Join(nonEmpty(gauge, fmt.Sprintf("%.1f °C", float64(b.temp)/10), "health "+strings.ToLower(b.health)), " · ")).Layout))
		if rate, over, ok := drainRate(ps.hist); ok {
			l := fmt.Sprintf("measured: %+.1f %%/h over %s", rate, hoursMins(over))
			if rate < 0 && !b.charging() {
				l += fmt.Sprintf(" · %s left at that", hoursMins(time.Duration(float64(b.pct)/-rate*float64(time.Hour))))
			}
			rows = append(rows, para(th, theme.Body(th, l).Layout))
		} else {
			rows = append(rows, quiet(th, "measured: in a while (a sample a minute)"))
		}
	}
	if us := ps.use; us.ok {
		rows = append(rows, theme.Section(th, fmt.Sprintf("CPU %d%% busy · of one core", int(us.busy*100+0.5))))
		rows = append(rows, procRows(th, us.procs, batProcs)...)
	}
	rows = append(rows, theme.Section(th, fmt.Sprintf("The last %d hours", int(graphSpan.Hours()))), func(gtx ink.Ctx) ink.Dims {
		size := image.Pt(gtx.Constraints.Max.X, max(gtx.Dp(180), gtx.Constraints.Min.Y))
		st := ink.Offset(gtx.Ops, image.Pt(gtx.Dp(th.Pad), 0))
		batGraph(gtx, th, size.Sub(image.Pt(2*gtx.Dp(th.Pad), 0)), ps.hist)
		st.Pop()
		return ink.Dims{Size: size}
	})
	return padList(gtx, th, &u.list, rows)
}

// padList is rows in a list moving under the finger, inset.
func padList(gtx ink.Ctx, th *theme.Theme, l *layout.Scroll, rows []ink.Widget) ink.Dims {
	l.Axis = layout.Vertical
	return layout.UniformInset(th.Pad).Layout(gtx, func(gtx ink.Ctx) ink.Dims {
		return theme.List(th, l).Layout(gtx, len(rows), func(gtx ink.Ctx, i int) ink.Dims {
			return rows[i](gtx)
		})
	})
}

// procRows are the busiest processes, n at most (0: all): a row each.
func procRows(th *theme.Theme, ps []ranked, n int) []ink.Widget {
	var rows []ink.Widget
	for _, p := range ps {
		if p.v < 0.5 || n > 0 && len(rows) == n {
			break
		}
		r := theme.Row(th, nil, p.name)
		r.Value = fmt.Sprintf("%.1f%%", p.v)
		rows = append(rows, r.Layout)
	}
	if len(rows) == 0 {
		rows = append(rows, quiet(th, "nothing busy"))
	}
	return rows
}

const batProcs = 5 // the Battery window's busiest processes, at most

// batGraph draws the battery's last hours in size: percent up, time along.
func batGraph(gtx ink.Ctx, th *theme.Theme, size image.Point, h []batSample) {
	o := gtx.Ops
	small := th.Face(gtx, th.TextSize*0.8, text.Regular)
	labelW := text.Width(small, "100%") + gtx.Dp(8)
	plot := image.Rect(labelW, gtx.Dp(6), size.X-gtx.Dp(6), size.Y-text.Height(small)-gtx.Dp(8))
	if plot.Dx() < 20 || plot.Dy() < 20 {
		return
	}
	asc := text.Ascent(small)
	for _, pct := range []int{0, 50, 100} {
		y := plot.Max.Y - pct*plot.Dy()/100
		ink.Fill(o, image.Rect(plot.Min.X, y, plot.Max.X, y+1), ink.Light)
		s := strconv.Itoa(pct) + "%"
		ink.TextAt(o, small, plot.Min.X-gtx.Dp(6)-text.Width(small, s), y+asc/2, s, th.Quiet)
	}
	ink.Frame(o, plot, max(gtx.Dp(1.5), 2), ink.Dark)
	for i := 1; i < 4; i++ { // a tick every 3 h
		x := plot.Min.X + i*plot.Dx()/4
		ink.Fill(o, image.Rect(x, plot.Max.Y-gtx.Dp(6), x+2, plot.Max.Y), ink.Dark)
	}
	y := plot.Max.Y + gtx.Dp(4) + asc
	ink.TextAt(o, small, plot.Min.X, y, fmt.Sprintf("%d h ago", int(graphSpan.Hours())), th.Quiet)
	ink.TextAt(o, small, plot.Max.X-text.Width(small, "now"), y, "now", th.Quiet)
	now := gtx.Now
	pt := func(s batSample) image.Point {
		x := plot.Max.X - int(float64(plot.Dx())*now.Sub(s.at).Seconds()/graphSpan.Seconds())
		return image.Pt(x, plot.Max.Y-s.pct*plot.Dy()/100)
	}
	clip := ink.Clip(o, plot)
	var last *batSample
	for i := range h {
		s := &h[i]
		if now.Sub(s.at) > graphSpan {
			continue
		}
		g := uint8(ink.Black)
		if s.charging {
			g = ink.Dark
		}
		if last != nil && s.at.Sub(last.at) < 15*time.Minute { // a gap: hellobro was off
			ink.Line(o, 2, g, pt(*last), pt(*s))
		} else {
			ink.Disc(o, pt(*s), 3, g)
		}
		last = s
	}
	clip.Pop()
	if len(h) == 0 || now.Sub(h[len(h)-1].at) > graphSpan {
		s := "a sample a minute, from now on"
		ink.TextAt(o, small, plot.Min.X+(plot.Dx()-text.Width(small, s))/2, plot.Min.Y+plot.Dy()/2, s, th.Quiet)
	}
}

type usageUI struct {
	a    *bro
	list layout.Scroll
}

func (u *usageUI) Status() string { return fmt.Sprintf("every %d s", int(u.a.powerEvery().Seconds())) }

func (u *usageUI) Layout(gtx ink.Ctx) ink.Dims {
	a, th := u.a, u.a.theme()
	us := a.pw.use
	var rows []ink.Widget
	if !us.ok {
		rows = append(rows, quiet(th, fmt.Sprintf("looking… (a sample every %d s)", int(a.powerEvery().Seconds()))))
		return padList(gtx, th, &u.list, rows)
	}
	cores := us.cores
	if cores == "0" {
		cores = "one core on"
	} else {
		cores = "cores " + cores + " on"
	}
	row := func(title, value string) ink.Widget {
		r := theme.Row(th, nil, title)
		r.Sub = value
		return r.Layout
	}
	rows = append(rows,
		para(th, theme.Title(th, fmt.Sprintf("CPU %d%% busy", int(us.busy*100+0.5))).Layout),
		quiet(th, cores+" · load "+us.load),
		theme.Section(th, "Where the time goes"),
		row("Speed", shares(us.freqs, 3, func(v float64) string { return pct(v) })),
		row("Asleep", shares(us.cstates, 4, func(v float64) string { return pct(v) })),
		row(fmt.Sprintf("Wakeups, %.0f a second", us.wakeups), shares(us.irqs, 4, func(v float64) string { return fmt.Sprintf("%.0f", v) })),
		row("What else draws", a.powerDevices()),
		theme.Section(th, "Busy, of one core"),
	)
	rows = append(rows, procRows(th, us.procs, 0)...)
	return padList(gtx, th, &u.list, rows)
}
