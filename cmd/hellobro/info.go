package main

// A box of facts over the page, to read: the Wi-Fi's, from its panel's
// info button (or i). Esc or Enter, or a tap outside, closes it; c (or
// its copy button) copies them all. The page holds still under it, as
// under a question.

import (
	"fmt"
	"image"
	"net"
	"os"
	"strings"

	"hellobro/kobo"
	"hellobro/radio"
)

type infoBox struct {
	title         string
	rows          [][2]string // a name and its value
	box           image.Rectangle
	copyR, closeR image.Rectangle
}

const infoRowH = 46

func (a *bro) showInfo(title string, rows [][2]string) {
	b := a.screenFree()
	w := min(1100, b.Dx()-80)
	h := min(b.Dy()-80, 32+56+16+len(rows)*infoRowH+24+68+32)
	top := b.Min.Y + (b.Dy()-h)/2
	q := &infoBox{title: title, rows: rows}
	q.box = image.Rect((b.Dx()-w)/2, top, (b.Dx()+w)/2, top+h)
	bw, by := 240, q.box.Max.Y-100
	q.closeR = image.Rect(q.box.Max.X-32-bw, by, q.box.Max.X-32, by+68)
	q.copyR = image.Rect(q.closeR.Min.X-20-bw, by, q.closeR.Min.X-20, by+68)
	a.info = q
	a.drawInfo()
}

func (a *bro) drawInfo() {
	q, c := a.info, a.C
	c.Fill(q.box, kobo.White)
	c.Frame(q.box, 4, kobo.Black)
	in := q.box.Inset(32)
	c.Text(a.F.Panel, image.Rect(in.Min.X, in.Min.Y, in.Max.X, in.Min.Y+56), q.title, kobo.Black, kobo.AlignLeft)
	nameW := 0
	for _, r := range q.rows {
		nameW = max(nameW, kobo.TextWidth(a.F.Small, r[0]))
	}
	y := in.Min.Y + 56 + 16
	for _, r := range q.rows {
		if y+infoRowH > q.closeR.Min.Y-12 {
			break // a short screen: the rest are in what's copied
		}
		c.Text(a.F.Small, image.Rect(in.Min.X, y, in.Min.X+nameW, y+infoRowH), r[0], kobo.Dark, kobo.AlignLeft)
		c.Text(a.F.Small, image.Rect(in.Min.X+nameW+32, y, in.Max.X, y+infoRowH), r[1], kobo.Black, kobo.AlignLeft)
		y += infoRowH
	}
	for _, b := range []struct {
		r     image.Rectangle
		label string
		dark  bool
	}{{q.copyR, "copy  (c)", false}, {q.closeR, "close  (Esc)", true}} {
		fg := uint8(kobo.Black)
		if b.dark {
			c.Fill(b.r, kobo.Black)
			fg = kobo.White
		} else {
			c.Frame(b.r, 2, kobo.Black)
		}
		c.Text(a.F.Small, b.r, b.label, fg, kobo.AlignCenter)
	}
	a.Mark(q.box, true)
}

func (a *bro) closeInfo() {
	a.info = nil
	a.DrawPage()
	a.drawStat(true)
}

// infoText is what's copied: a fact a line.
func infoText(rows [][2]string) string {
	var b strings.Builder
	for _, r := range rows {
		fmt.Fprintf(&b, "%s: %s\n", r[0], r[1])
	}
	return b.String()
}

func (a *bro) infoKey(code uint16, value int32) {
	if a.termMod(code, value) || value != 1 {
		return
	}
	ch, _ := kobo.KeyChar(code, false, false)
	switch {
	case code == kobo.KeyEnter || code == kobo.KeyEsc || ch == "q":
		a.closeInfo()
	case ch == "c":
		a.copyText(infoText(a.info.rows), "the details")
	}
}

func (a *bro) infoTap(p image.Point) {
	q := a.info
	switch {
	case kobo.Hit(p, q.copyR):
		a.copyText(infoText(q.rows), "the details")
	case kobo.Hit(p, q.closeR), !p.In(q.box):
		a.closeInfo()
	}
}

// --- the Wi-Fi's ---

// wifiInfo looks at the Wi-Fi off the UI, then shows what it found.
func (a *bro) wifiInfo() {
	w := a.rs.wifi
	if w == nil {
		return
	}
	go func() {
		ws := w.Status()
		rows := wifiFacts(ws, ifaceFacts(ws.Interface), nameservers())
		a.Do(func() {
			if !a.overlay() {
				a.showInfo("Wi-Fi details", rows)
			}
		})
	}()
}

type ifaceInfo struct {
	mac, cidr string
}

func ifaceFacts(name string) ifaceInfo {
	var f ifaceInfo
	ifc, err := net.InterfaceByName(name)
	if err != nil {
		return f
	}
	f.mac = ifc.HardwareAddr.String()
	addrs, _ := ifc.Addrs()
	for _, ad := range addrs {
		if n, ok := ad.(*net.IPNet); ok && n.IP.To4() != nil {
			f.cidr = n.String()
		}
	}
	return f
}

// nameservers are resolv.conf's.
func nameservers() []string {
	b, _ := os.ReadFile("/etc/resolv.conf")
	var ns []string
	for _, line := range strings.Split(string(b), "\n") {
		if f := strings.Fields(line); len(f) >= 2 && f[0] == "nameserver" {
			ns = append(ns, f[1])
		}
	}
	return ns
}

// wifiFacts are the rows of the Wi-Fi's details.
func wifiFacts(ws radio.WiFiStatus, ifc ifaceInfo, dns []string) [][2]string {
	or := func(s, none string) string {
		if s == "" {
			return none
		}
		return s
	}
	rows := [][2]string{{"Wi-Fi", "off"}}
	switch {
	case ws.On && ws.State == "COMPLETED":
		rows[0][1] = "on · connected"
	case ws.On:
		rows[0][1] = "on · " + strings.ToLower(or(ws.State, "?"))
	}
	if ws.On && ws.State == "COMPLETED" {
		rows = append(rows,
			[2]string{"network", ws.SSID},
			[2]string{"access point", or(ws.BSSID, "?")},
			[2]string{"band", channelOf(ws.Freq)},
			[2]string{"signal", signalWords(ws.Signal)})
	}
	addr := or(ifc.cidr, or(ws.IP, "none"))
	rows = append(rows,
		[2]string{"address", addr},
		[2]string{"gateway", or(ws.Gateway, "none")},
		[2]string{"DNS", or(strings.Join(dns, ", "), "none")},
		[2]string{"interface", or(ws.Interface, "?") + " · " + or(ifc.mac, "no MAC")})
	if ws.On {
		saving := "saved (wpa_supplicant's file)"
		if !ws.Saving {
			saving = "only till Wi-Fi goes off"
		}
		rows = append(rows, [2]string{"new networks", saving})
	}
	return rows
}

// channelOf is a frequency's band and channel: "5 GHz · channel 44 (5220 MHz)".
func channelOf(mhz int) string {
	switch {
	case mhz == 0:
		return "?"
	case mhz == 2484:
		return fmt.Sprintf("2.4 GHz · channel 14 (%d MHz)", mhz)
	case mhz >= 2412 && mhz < 2484:
		return fmt.Sprintf("2.4 GHz · channel %d (%d MHz)", (mhz-2407)/5, mhz)
	case mhz >= 5000 && mhz < 5900:
		return fmt.Sprintf("5 GHz · channel %d (%d MHz)", (mhz-5000)/5, mhz)
	case mhz >= 5925 && mhz < 7125:
		return fmt.Sprintf("6 GHz · channel %d (%d MHz)", (mhz-5950)/5, mhz)
	}
	return fmt.Sprintf("%d MHz", mhz)
}

// signalWords is the signal with a word on it: "-48 dBm · good".
func signalWords(dbm int) string {
	if dbm == 0 {
		return "?"
	}
	word := "weak"
	switch {
	case dbm >= -55:
		word = "good"
	case dbm >= -67:
		word = "fair"
	}
	return fmt.Sprintf("%d dBm · %s", dbm, word)
}
