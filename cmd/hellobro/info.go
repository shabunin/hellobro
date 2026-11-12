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

	"hellobro/radio"
)

type infoBox struct {
	title string
	rows  [][2]string // a name and its value; a sheet's heading: {sheetHead, the heading}
	sheet bool        // a cheat sheet (sheet.go): rows in columns, any key closes it
	ui    *infoUI
	over  *inkOver
}

func (a *bro) showInfo(title string, rows [][2]string) {
	a.putInfo(&infoBox{title: title, rows: rows}, 1100)
}

// putInfo puts q up, w wide (at the Elipsa's density) as the screen lets it.
func (a *bro) putInfo(q *infoBox, w int) {
	q.ui = &infoUI{a: a, q: q}
	q.over = newOver(q.ui.Layout, w)
	q.over.tall = q.sheet
	q.over.take = func() {
		act := q.ui.act
		q.ui.act = ""
		switch act {
		case "close":
			a.closeInfo()
		case "copy":
			a.copyText(infoText(q.rows), "the details")
		}
	}
	a.info = q
	a.drawOver(q.over)
}

func (a *bro) closeInfo() {
	if a.info == nil {
		return
	}
	a.info.over.gone()
	a.info = nil
	a.DrawPage()
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
	if a.termMod(code, value) {
		return
	}
	if a.info.sheet && value == 1 { // any key: read, and back to the page
		a.closeInfo()
		return
	}
	a.overKey(a.info.over, code, value)
}

// infoTap: a tap outside the box closes it; anywhere, a sheet. One on
// the box was its pane's.
func (a *bro) infoTap(p image.Point) {
	if q := a.info; q.sheet || !p.In(q.over.box) {
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
