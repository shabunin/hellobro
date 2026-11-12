package main

// The Sharing window, in ink (inkwin.go): file sharing on or off, how a
// computer reaches the Kobo (share.go), the pairing code, the relay, and
// who's connected, sending what. o turns it on or off, p pairs.

import (
	"fmt"
	"net"
	"time"

	"hellobro/ink"
	"hellobro/ink/layout"
	"hellobro/ink/theme"
	"hellobro/ink/widget"
	"hellobro/share"
)

type shareUI struct {
	a            *bro
	list         layout.Scroll
	on           widget.Bool
	pair, forget widget.Clickable
}

func (u *shareUI) Status() string {
	s := &u.a.share
	switch {
	case s.srv == nil:
		return "off"
	case s.code != "":
		return "pairing"
	}
	return fmt.Sprintf("on · %d connected", len(s.conns))
}

func (u *shareUI) Layout(gtx ink.Ctx) ink.Dims {
	a, th := u.a, u.a.theme()
	s := &a.share
	for _, e := range gtx.Events(ink.Root) {
		if t, ok := e.(ink.Text); ok {
			switch t.Text {
			case "o":
				a.shareToggle()
			case "p":
				a.sharePair()
			}
		}
	}
	if u.on.Update(gtx) {
		a.shareToggle()
	}
	if u.pair.Clicked(gtx) {
		a.sharePair()
	}
	if u.forget.Clicked(gtx) {
		a.shareForgetAll()
	}
	u.on.Value = s.srv != nil
	if s.code != "" {
		gtx.InvalidateAt(gtx.Now.Add(time.Second)) // the time left
	}
	on := theme.Toggle(th, &u.on, "File sharing (o)")
	on.Sub = "SFTP and SSH, port 2022: a computer reaches the Kobo's storage"
	rows := []ink.Widget{on.Layout}
	text := func(w ink.Widget) { rows = append(rows, para(th, w)) }
	if s.srv == nil {
		rows = append(rows, quiet(th, "Then a file manager reaches it (sftp://), or sftp, scp, sshfs, or a shell with ssh."))
		if s.err != "" {
			text(theme.Body(th, "It didn't start: "+s.err).Layout)
		}
	} else {
		ip := share.IPv4(shareIf)
		_, port, _ := net.SplitHostPort(shareAddr)
		host := ip
		if a.setting("mdns") == "on" {
			host = mdnsHost + ".local"
		}
		rows = append(rows, theme.Section(th, "Reach it"))
		if ip == "" {
			text(theme.Body(th, "No Wi-Fi address: turn Wi-Fi on.").Layout)
		} else {
			text(theme.Title(th, fmt.Sprintf("sftp://%s:%s%s", host, port, shareRoot)).Layout)
			for _, c := range []string{
				fmt.Sprintf("ssh -p %s %s", port, host),
				fmt.Sprintf("scp -P %s song.mp3 %s:Music/", port, host),
				fmt.Sprintf("sshfs -p %s %s:%s ~/kobo", port, host, shareRoot),
			} {
				text(theme.Body(th, c).Layout)
			}
			rows = append(rows, quiet(th, fmt.Sprintf("%s:%s · host key %s", ip, port, s.srv.HostKey())))
		}
		rows = append(rows, theme.Section(th, "Computers"))
		if s.code != "" {
			left := max(0, time.Until(s.until)).Round(time.Second)
			text(theme.Title(th, "Pairing code: "+s.code[:3]+" "+s.code[3:]).Layout)
			rows = append(rows, quiet(th, fmt.Sprintf("On the computer: ssh -p %s %s, then type the code (%s left).", port, host, left)))
		} else {
			r := theme.Row(th, nil, fmt.Sprintf("%d paired", len(s.keys)))
			r.Sub = "a computer that's paired gets in with its key"
			rows = append(rows, r.Layout, func(gtx ink.Ctx) ink.Dims {
				return layout.Inset{Left: th.Pad * 1.5, Top: th.Pad, Bottom: th.Pad}.Layout(gtx, func(gtx ink.Ctx) ink.Dims {
					kids := []layout.FlexChild{layout.Rigid(theme.Button(th, &u.pair, "pair one (p)").Layout)}
					if len(s.keys) > 0 {
						kids = append(kids, layout.Rigid(layout.Spacer{Width: th.Pad}.Layout), layout.Rigid(theme.Button(th, &u.forget, "forget them…").Layout))
					}
					return layout.Flex{}.Layout(gtx, kids...)
				})
			})
		}
		switch r := &a.relay; {
		case r.stop != nil && r.up:
			text(theme.Body(th, fmt.Sprintf("Relay %s · from afar: ssh -J you@%s -p %d root@127.0.0.1", r.status, r.conf.server(), r.conf.port)).Layout)
		case r.stop != nil:
			text(theme.Body(th, "Relay "+r.status).Layout)
		case r.err != "":
			text(theme.Body(th, "Relay: "+r.err).Layout)
		}
		if len(s.conns) > 0 {
			rows = append(rows, theme.Section(th, "Connected"))
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
			since := "just now" // by the minute: not a frame every second
			if d := time.Since(cn.Since); d >= time.Minute {
				since = hoursMins(d)
			}
			r := theme.Row(th, nil, cn.User+"@"+cn.Addr)
			r.Sub = fmt.Sprintf("%s · ↓ %s%s ↑ %s", what, humanBytes(cn.In), rate, humanBytes(cn.Out))
			r.Value = since
			rows = append(rows, r.Layout)
		}
	}
	if s.note != "" {
		rows = append(rows, quiet(th, s.note))
	}
	return padList(gtx, th, &u.list, rows)
}
