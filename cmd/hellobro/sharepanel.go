package main

// The Settings tab's Sharing panel: how a computer reaches the Kobo
// (share.go), the pairing code, and who's connected, sending what.

import (
	"fmt"
	"image"
	"log"
	"net"
	"time"

	"hellobro/kobo"
	"hellobro/share"
)

const panelShare = 2 // after Wi-Fi and Bluetooth, before the settings

type sharePanel struct {
	p     kobo.Panel
	text  image.Rectangle // its lines
	btns  []radioButton
	shown string
}

// placeShare puts the Sharing window at p.
func (a *bro) placeShare(p kobo.Panel) {
	v := &a.shp
	v.p = p
	v.shown, v.text, v.btns = "", image.Rectangle{}, v.btns[:0]
	if p.Outer.Empty() || p.Body.Dy() < 160 {
		return
	}
	b := v.p.Body.Inset(kobo.Pad)
	v.text = image.Rect(b.Min.X, b.Min.Y, b.Max.X, b.Max.Y-76)
	bw := min(250, (b.Dx()-24)/3)
	for i, label := range []string{"on/off", "pair", "forget paired"} {
		x := b.Min.X + i*(bw+12)
		v.btns = append(v.btns, radioButton{r: image.Rect(x, b.Max.Y-64, x+bw, b.Max.Y), label: label})
	}
}

func (a *bro) drawShare() {
	if !a.guiShows("sharing") || a.shp.text.Empty() {
		return
	}
	v, s, c := &a.shp, &a.share, a.C
	type line struct {
		text string
		big  bool
		g    uint8
	}
	var lines []line
	add := func(g uint8, f string, args ...any) { lines = append(lines, line{fmt.Sprintf(f, args...), false, g}) }
	ip := share.IPv4(shareIf)
	_, port, _ := net.SplitHostPort(shareAddr)
	status := "off"
	switch {
	case s.srv == nil:
		add(kobo.Black, "File sharing is off: o turns it on.")
		add(kobo.Dark, "Then a computer reaches the Kobo's storage with a file manager (sftp://), sftp, scp or sshfs, or a shell with ssh.")
		if s.err != "" {
			add(kobo.Black, "It didn't start: %s", s.err)
		}
	default:
		status = fmt.Sprintf("on · %d connected", len(s.conns))
		host := ip
		if a.setting("mdns") == "on" {
			host = mdnsHost + ".local"
		}
		if ip == "" {
			add(kobo.Black, "No Wi-Fi address: turn Wi-Fi on.")
		} else {
			add(kobo.Black, "sftp://%s:%s%s", host, port, shareRoot)
			add(kobo.Dark, "ssh -p %s %s  ·  scp -P %s song.mp3 %s:Music/  ·  sshfs -p %s %s:%s ~/kobo", port, host, port, host, port, host, shareRoot)
			add(kobo.Mid, "%s:%s  ·  host key %s", ip, port, s.srv.HostKey())
		}
		if s.code != "" {
			status = "pairing"
			left := max(0, time.Until(s.until)).Round(time.Second)
			lines = append(lines, line{"Pairing code: " + s.code[:3] + " " + s.code[3:], true, kobo.Black})
			add(kobo.Dark, "On the computer: ssh -p %s %s, then type the code (%s left).", port, host, left)
		} else {
			add(kobo.Dark, "Paired computers: %d  (p: pair one more)", len(s.keys))
		}
		switch r := &a.relay; {
		case r.stop != nil && r.up:
			add(kobo.Black, "Relay %s · from afar: ssh -J you@%s -p %d root@127.0.0.1", r.status, r.conf.server(), r.conf.port)
		case r.stop != nil:
			add(kobo.Black, "Relay %s", r.status)
		case r.err != "":
			add(kobo.Black, "Relay: %s", r.err)
		}
		for _, cn := range s.conns {
			what := cn.What
			if what == "" {
				what = "connecting"
			}
			rate := ""
			if r := s.rate[cn.ID]; r > 1024 {
				rate = fmt.Sprintf(" (%s/s)", humanBytes(int64(r)))
			}
			since := "just now" // by the minute: the panel isn't drawn again every second
			if d := time.Since(cn.Since); d >= time.Minute {
				since = hoursMins(d)
			}
			add(kobo.Black, "● %s@%s · %s · ↓ %s%s ↑ %s · %s", cn.User, cn.Addr, what, humanBytes(cn.In), rate, humanBytes(cn.Out), since)
		}
	}
	if s.note != "" {
		add(kobo.Mid, "%s", s.note)
	}
	focused := a.guiFocused("sharing")
	key := fmt.Sprint(status, lines, focused)
	if key == v.shown {
		return
	}
	v.shown = key
	a.DrawPanel(a.winPanel(v.p, focused), status)
	y := v.text.Min.Y
	for _, l := range lines {
		f, h := a.F.Small, 40
		if l.big {
			f, h = a.F.Panel, 64
		}
		if y+h > v.text.Max.Y {
			break
		}
		c.Text(f, image.Rect(v.text.Min.X, y, v.text.Max.X, y+h), l.text, l.g, kobo.AlignLeft)
		y += h
	}
	for _, b := range v.btns {
		label := b.label
		if label == "on/off" {
			label = map[bool]string{true: "turn off", false: "turn on"}[s.srv != nil]
		}
		c.Fill(b.r, kobo.White)
		c.Frame(b.r, 2, kobo.Black)
		c.Text(a.F.Small, b.r, label, kobo.Black, kobo.AlignCenter)
	}
	a.Mark(v.p.Outer, true)
}

func (a *bro) shareToggle() {
	i := settingIndex("share")
	if a.setting("share") == "on" {
		a.set["share"] = indexOf(settingList[i].values, "off")
	} else {
		a.set["share"] = indexOf(settingList[i].values, "on")
	}
	a.set.save()
	a.shareApply()
}

func (a *bro) shareForgetAll() {
	s := &a.share
	if s.srv == nil || len(s.keys) == 0 {
		return
	}
	a.ask("Forget the paired computers?", fmt.Sprintf("%d of them will have to pair again to get in.", len(s.keys)), "forget", func() {
		for _, k := range s.keys {
			if err := s.srv.Forget(k.Fingerprint); err != nil {
				log.Printf("share: forget: %v", err)
			}
		}
		s.keys, _ = s.srv.Keys()
		s.note = "forgot the paired computers"
		a.drawShare()
	})
}

// shareTap is a tap on the panel's buttons.
func (a *bro) shareTap(p image.Point) {
	for _, b := range a.shp.btns {
		if kobo.Hit(p, b.r) {
			switch b.label {
			case "on/off":
				a.shareToggle()
			case "pair":
				a.sharePair()
			case "forget paired":
				a.shareForgetAll()
			}
			return
		}
	}
}

// shareKey is a key with the panel in focus: o on/off, p pair.
func (a *bro) shareKey(code uint16) bool {
	ch, _ := kobo.KeyChar(code, false, false)
	switch {
	case code == kobo.KeyEsc:
		return false
	case ch == "o":
		a.shareToggle()
	case ch == "p":
		a.sharePair()
	}
	return true
}
