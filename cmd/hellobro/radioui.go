package main

// The Wi-Fi and Bluetooth windows, in ink (inkwin.go): the radio's power,
// what it's doing, a scan; the networks (devices) as rows, the one tapped
// open with what can be done with it; a question (a password, a pairing
// code) at the top till it's answered. The keys are as they were: o on or
// off, s scan, n another network, i info, f forget.

import (
	"fmt"
	"image"
	"strings"
	"time"

	"hellobro/ink"
	"hellobro/ink/layout"
	"hellobro/ink/text"
	"hellobro/ink/theme"
	"hellobro/ink/widget"
	"hellobro/radio"
)

type radioUI struct {
	a     *bro
	panel int // panelWiFi, panelBT
	list  layout.Scroll

	power                    widget.Bool
	scan, other, info        widget.Clickable
	rows                     []widget.Clickable
	open                     string // the row opened: a network's or a device's key
	act                      [2]widget.Clickable
	answer                   widget.Editor
	asked                    *radioPrompt // the question the editor's for
	before                   ink.Tag      // where the keys were before it
	ok, cancel, show, reveal widget.Clickable
}

// radioRow is a network or a device, as a row shows it.
type radioRow struct {
	key, mark, name, detail string
	bars                    int
	acts                    []string
	do                      []func()
}

func (u *radioUI) Status() string {
	r := &u.a.rs
	if u.panel == panelWiFi {
		ws := r.ws
		switch {
		case !ws.On:
			return "off"
		case ws.State == "COMPLETED":
			return ws.SSID
		}
		return "not connected"
	}
	bs := r.bs
	switch {
	case !bs.Running || !bs.Powered:
		return "off"
	case !r.discoverTo.IsZero():
		return "scanning"
	}
	n := 0
	for _, d := range r.devs {
		if d.Connected {
			n++
		}
	}
	return fmt.Sprintf("%d connected", n)
}

// state is what the radio says now (line1), what's going on or how it
// went (line2), and its rows.
func (u *radioUI) state() (on bool, line1, line2 string, rows []radioRow) {
	a, r := u.a, &u.a.rs
	if u.panel == panelWiFi {
		ws := r.ws
		on = ws.On
		switch {
		case !ws.On:
			line1 = "off"
		case ws.State == "COMPLETED":
			line1 = fmt.Sprintf("%s · %d dBm · %s", ws.SSID, ws.Signal, band(ws.Freq))
			if ws.IP != "" {
				line1 += " · " + ws.IP
			} else {
				line1 += " · no address yet"
			}
		default:
			line1 = "not connected (" + strings.ToLower(ws.State) + ")"
		}
		line2 = r.wNote
		if r.wBusy != "" {
			line2 = r.wBusy
		}
		for i, n := range r.nets {
			rw := radioRow{key: n.SSID, name: n.SSID, bars: n.Bars()}
			switch {
			case n.Current:
				rw.mark = "●"
			case n.Known():
				rw.mark = "○"
			}
			rw.detail = n.Security
			if rw.detail == "" {
				rw.detail = "open"
			}
			if n.Signal == 0 {
				rw.detail = "saved, not around"
			} else if n.Freq > 0 {
				rw.detail += " · " + band(n.Freq)
			}
			go_ := "connect"
			if n.Current {
				go_ = "disconnect"
			}
			rw.acts, rw.do = []string{go_}, []func(){func() { a.wifiGo(i) }}
			if n.Known() {
				rw.acts, rw.do = append(rw.acts, "forget"), append(rw.do, func() { a.wifiForget(i) })
			}
			rows = append(rows, rw)
		}
		return
	}
	bs := r.bs
	on = bs.Powered
	switch {
	case !bs.Running:
		line1 = bs.Err
	case !bs.Powered && bs.Addr == "": // no adapter: the chip itself is off
		line1 = "off: turning it on takes a few seconds"
	case !bs.Powered:
		line1 = fmt.Sprintf("off (%s, %s)", bs.Name, bs.Addr)
	default:
		line1 = fmt.Sprintf("%s (%s)", bs.Name, bs.Addr)
		if !r.discoverTo.IsZero() {
			line1 += fmt.Sprintf(" · scanning, %.0f s more", time.Until(r.discoverTo).Seconds())
		}
	}
	line2 = r.bNote
	if r.bBusy != "" {
		line2 = r.bBusy
	}
	for i, d := range r.devs {
		rw := radioRow{key: d.Addr, name: btName(d)}
		var parts []string
		if k := d.Kind(); k != "" {
			parts = append(parts, k)
		}
		switch {
		case d.Connected:
			rw.mark = "●"
			parts = append(parts, "connected")
			rw.acts = []string{"disconnect"}
		case d.Paired:
			rw.mark = "○"
			parts = append(parts, "paired")
			rw.acts = []string{"connect"}
		default:
			parts = append(parts, "new")
			rw.acts = []string{"pair"}
		}
		rw.do = []func(){func() { a.btGo(i) }}
		if d.Paired {
			rw.acts, rw.do = append(rw.acts, "forget"), append(rw.do, func() { a.btForget(i) })
		}
		rw.detail = strings.Join(parts, " · ")
		if d.RSSI != 0 {
			rw.bars = radio.Network{Signal: int(d.RSSI)}.Bars()
		}
		rows = append(rows, rw)
	}
	return
}

func (u *radioUI) Layout(gtx ink.Ctx) ink.Dims {
	a, th, r := u.a, u.a.theme(), &u.a.rs
	wifi := u.panel == panelWiFi
	prompt := r.prompt
	if prompt != nil && prompt.panel != u.panel {
		prompt = nil
	}
	on, line1, line2, rows := u.state()
	for len(u.rows) < len(rows) {
		u.rows = append(u.rows, widget.Clickable{})
	}
	// What was done since the last frame.
	if prompt != nil {
		u.promptEvents(gtx, prompt)
	} else {
		if u.asked != nil { // answered: the keys go back where they were
			u.asked = nil
			gtx.Focus(u.before)
		}
		for _, e := range gtx.Events(ink.Root) {
			if t, ok := e.(ink.Text); ok {
				u.letter(t.Text, rows)
			}
		}
	}
	if u.power.Update(gtx) {
		if wifi {
			a.wifiPower()
		} else {
			a.btPower()
		}
	}
	if u.scan.Clicked(gtx) {
		if wifi {
			a.radioPoll(true)
		} else {
			a.btScan()
		}
	}
	if u.other.Clicked(gtx) {
		a.radioAsk(&radioPrompt{panel: panelWiFi, kind: "ssid", text: "The network's name (SSID):"})
	}
	if u.info.Clicked(gtx) {
		a.wifiInfo()
	}
	for i, rw := range rows {
		if u.rows[i].Clicked(gtx) {
			if u.open == rw.key {
				u.open = ""
			} else {
				u.open = rw.key
			}
		}
	}
	opened := -1
	for i, rw := range rows {
		if rw.key == u.open {
			opened = i
		}
	}
	if opened >= 0 {
		for k := range rows[opened].acts {
			if u.act[k].Clicked(gtx) {
				rows[opened].do[k]()
			}
		}
	}
	if !wifi && !r.discoverTo.IsZero() {
		gtx.InvalidateAt(gtx.Now.Add(time.Second)) // the scan's seconds left
	}

	// The window.
	u.power.Value = on
	name := map[bool]string{true: "Wi-Fi (o)", false: "Bluetooth (o)"}[wifi]
	pw := theme.Toggle(th, &u.power, name)
	pw.Sub = line1
	items := []ink.Widget{pw.Layout}
	if line2 != "" {
		items = append(items, quiet(th, line2))
	}
	if prompt != nil {
		items = append(items, u.promptBox(th, prompt))
	}
	items = append(items, func(gtx ink.Ctx) ink.Dims {
		return layout.Inset{Left: th.Pad * 1.5, Top: th.Pad, Bottom: th.Pad}.Layout(gtx, func(gtx ink.Ctx) ink.Dims {
			scan := "scan (s)"
			if !wifi && !r.discoverTo.IsZero() {
				scan = "stop the scan (s)"
			}
			sp := layout.Rigid(layout.Spacer{Width: th.Pad}.Layout)
			kids := []layout.FlexChild{layout.Rigid(theme.Button(th, &u.scan, scan).Layout)}
			if wifi {
				kids = append(kids, sp, layout.Rigid(theme.Button(th, &u.other, "another network… (n)").Layout),
					sp, layout.Rigid(theme.Button(th, &u.info, "details (i)").Layout))
			}
			return layout.Flex{}.Layout(gtx, kids...)
		})
	})
	head := map[bool]string{true: "Networks", false: "Devices"}[wifi]
	items = append(items, theme.Section(th, head))
	if len(rows) == 0 {
		empty := "none: scan, or turn Wi-Fi on"
		if !wifi {
			empty = "none yet: scan, with the device in pairing mode"
		}
		items = append(items, quiet(th, empty))
	}
	for i := range rows {
		rw, clk := rows[i], &u.rows[i]
		items = append(items, func(gtx ink.Ctx) ink.Dims { return u.row(gtx, th, rw, clk, rw.key == u.open) })
		if i == opened {
			items = append(items, func(gtx ink.Ctx) ink.Dims {
				return layout.Inset{Left: th.Pad * 5, Top: th.Pad, Bottom: th.Pad * 1.5}.Layout(gtx, func(gtx ink.Ctx) ink.Dims {
					var kids []layout.FlexChild
					for k, label := range rw.acts {
						if k > 0 {
							kids = append(kids, layout.Rigid(layout.Spacer{Width: th.Pad}.Layout))
						}
						b := theme.Button(th, &u.act[k], label)
						b.Strong = k == 0
						kids = append(kids, layout.Rigid(b.Layout))
					}
					return layout.Flex{}.Layout(gtx, kids...)
				})
			})
		}
	}
	return padList(gtx, th, &u.list, items)
}

// letter is a letter typed with no question asked: the window's keys.
func (u *radioUI) letter(ch string, rows []radioRow) {
	a, wifi := u.a, u.panel == panelWiFi
	switch ch {
	case "o":
		if wifi {
			a.wifiPower()
		} else {
			a.btPower()
		}
	case "s":
		if wifi {
			a.radioPoll(true)
		} else {
			a.btScan()
		}
	case "n":
		if wifi {
			a.radioAsk(&radioPrompt{panel: panelWiFi, kind: "ssid", text: "The network's name (SSID):"})
		}
	case "i":
		if wifi {
			a.wifiInfo()
		}
	case "f":
		for _, rw := range rows {
			if rw.key == u.open && len(rw.acts) > 1 {
				rw.do[1]()
			}
		}
	}
}

// row is a network or a device: whether it's joined (●) or known (○), its
// name, what it is, its signal.
func (u *radioUI) row(gtx ink.Ctx, th *theme.Theme, rw radioRow, clk *widget.Clickable, open bool) ink.Dims {
	return clk.Layout(gtx, func(gtx ink.Ctx) ink.Dims {
		w := gtx.Constraints.Max.X
		body := th.Face(gtx, th.TextSize, text.Regular)
		bold := th.Face(gtx, th.TextSize, text.Bold)
		small := th.Face(gtx, th.TextSize*0.85, text.Regular)
		pad := gtx.Dp(th.Pad * 1.5)
		h := max(gtx.Dp(th.Touch*1.2), text.Height(body)+text.Height(small)+gtx.Dp(th.Pad))
		r := image.Rect(0, 0, w, h)
		ink.Fill(gtx.Ops, r, th.Bg)
		markW := gtx.Dp(28)
		y := (h-text.Height(body)-text.Height(small))/2 + text.Ascent(body)
		switch rw.mark {
		case "●":
			ink.Disc(gtx.Ops, image.Pt(pad+gtx.Dp(6), h/2), gtx.Dp(6), th.Fg)
		case "○":
			ink.Ring(gtx.Ops, image.Pt(pad+gtx.Dp(6), h/2), gtx.Dp(6), max(gtx.Dp(1.5), 2), th.Fg)
		}
		barsW := gtx.Dp(30)
		f := body
		if open {
			f = bold
		}
		x := pad + markW
		room := w - x - pad - barsW - pad
		ink.TextAt(gtx.Ops, f, x, y, text.Fit(f, rw.name, room), th.Fg)
		ink.TextAt(gtx.Ops, small, x, y+text.Height(small), text.Fit(small, rw.detail, room), th.Quiet)
		if rw.bars > 0 {
			br := image.Rect(w-pad-barsW, h/2-gtx.Dp(10), w-pad, h/2+gtx.Dp(10))
			bw := br.Dx() / 4
			for k := range 4 {
				bh := br.Dy() * (k + 1) / 4
				b := image.Rect(br.Min.X+k*bw+1, br.Max.Y-bh, br.Min.X+(k+1)*bw-1, br.Max.Y)
				if k < rw.bars {
					ink.Fill(gtx.Ops, b, th.Fg)
				} else {
					ink.Frame(gtx.Ops, b, 1, th.Quiet)
				}
			}
		}
		ink.Fill(gtx.Ops, image.Rect(pad, h-1, w, h), ink.Light)
		if clk.Pressed() {
			ink.Invert(gtx.Ops, r)
		}
		return ink.Dims{Size: r.Size()}
	})
}

// promptEvents: the question's answer typed, its buttons.
func (u *radioUI) promptEvents(gtx ink.Ctx, p *radioPrompt) {
	a := u.a
	if u.asked != p { // a new question: an empty box, the keys in it
		if u.asked == nil {
			u.before = gtx.FocusedTag()
		}
		u.asked = p
		u.answer.SetText("")
		if p.kind != "confirm" && p.kind != "authorize" && p.kind != "display" {
			gtx.Focus(&u.answer)
		}
	}
	for _, e := range gtx.Events(ink.Root) {
		switch e := e.(type) {
		case ink.Key:
			switch e.Name {
			case "Esc":
				a.radioAnswer(false)
				return
			case "Enter":
				a.radioAnswer(p.kind != "display")
				return
			}
		case ink.Text:
			if p.kind == "confirm" || p.kind == "authorize" {
				switch e.Text {
				case "y":
					a.radioAnswer(true)
					return
				case "n":
					a.radioAnswer(false)
					return
				}
			}
		}
	}
	if u.answer.Changed(gtx) {
		p.answer = u.answer.Text()
	}
	if u.answer.Submitted(gtx) || u.ok.Clicked(gtx) {
		p.answer = u.answer.Text()
		a.radioAnswer(true)
		return
	}
	if u.cancel.Clicked(gtx) {
		a.radioAnswer(false)
		return
	}
	if u.reveal.Clicked(gtx) {
		p.reveal = !p.reveal
	}
	if p.kind == "password" && !p.reveal {
		u.answer.Mask = '•'
	} else {
		u.answer.Mask = 0
	}
}

// promptBox is the question: its words, a box for the answer, buttons.
func (u *radioUI) promptBox(th *theme.Theme, p *radioPrompt) ink.Widget {
	return func(gtx ink.Ctx) ink.Dims {
		return layout.Inset{Left: th.Pad * 1.5, Right: th.Pad * 1.5, Top: th.Pad, Bottom: th.Pad}.Layout(gtx, func(gtx ink.Ctx) ink.Dims {
			var oks, nos string
			typed := true
			switch p.kind {
			case "confirm", "authorize":
				oks, nos, typed = "pair (y)", "no (n)", false
			case "display":
				nos, typed = "cancel", false
			default:
				oks, nos = "ok", "cancel"
			}
			sp := layout.Rigid(layout.Spacer{Width: th.Pad}.Layout)
			var btns []layout.FlexChild
			if typed {
				btns = append(btns, layout.Flexed(1, theme.Editor(th, &u.answer, "").Layout), sp)
			} else {
				btns = append(btns, layout.Flexed(1, layout.Spacer{}.Layout))
			}
			if p.kind == "password" {
				btns = append(btns, layout.Rigid(theme.Button(th, &u.reveal, map[bool]string{true: "hide", false: "show"}[p.reveal]).Layout), sp)
			}
			if oks != "" {
				b := theme.Button(th, &u.ok, oks)
				b.Strong = true
				btns = append(btns, layout.Rigid(b.Layout), sp)
			}
			btns = append(btns, layout.Rigid(theme.Button(th, &u.cancel, nos).Layout))
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(theme.Label(th, th.TextSize, p.text).Layout),
				layout.Rigid(layout.Spacer{Height: th.Pad}.Layout),
				layout.Rigid(func(gtx ink.Ctx) ink.Dims {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx, btns...)
				}))
		})
	}
}
