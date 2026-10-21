package main

// The computer's way in (the share package): the SSH and SFTP server on
// port 2022 while Settings has file sharing on, the Kobo as kobo.local on
// the network (mDNS), pairing a computer with a code shown here, and our
// own commands: hellobro-version, and hellobro-update (make push).

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	"hellobro/kobo"
	"hellobro/share"
)

const (
	pairTime = 5 * time.Minute
	mdnsHost = "kobo" // kobo.local
)

var (
	shareRoot = kobo.EnvOr("HELLOBRO_SHARE_ROOT", "/mnt/onboard")
	shareAddr = kobo.EnvOr("HELLOBRO_SHARE_ADDR", fmt.Sprintf(":%d", share.Port))
	shareIf   = kobo.EnvOr("INTERFACE", "wlan0")
)

type shareState struct {
	srv      *share.Server
	stopMDNS func()
	err      string // why it isn't up
	note     string // what happened last
	conns    []share.ConnInfo
	rate     map[int]float64 // bytes a second in, by connection
	keys     []share.AuthorizedKey
	code     string // the pairing code, while pairing
	until    time.Time
	polledAt time.Time
	lastIn   map[int]int64
}

// shareApply starts or stops the server and mDNS as the settings say.
func (a *bro) shareApply() {
	s := &a.share
	on := a.setting("share") == "on"
	switch {
	case on && s.srv == nil:
		srv, err := share.Start(share.Config{
			Addr: shareAddr, StateDir: kobo.AppFile(""), Root: shareRoot,
			StartShell: func(cmd string, cols, rows int) (share.Shell, error) {
				sh, err := startShell(cols, rows, cmd)
				if err != nil {
					return nil, err
				}
				return sharedShell{sh}, nil
			},
			Builtin: a.builtin,
			Notify:  func(e share.Event) { a.Do(func() { a.shareEvent(e) }) },
		})
		if err != nil {
			s.err = err.Error()
			log.Printf("share: %v", err)
			break
		}
		s.srv, s.err, s.note = srv, "", ""
		s.keys, _ = srv.Keys()
	case !on && s.srv != nil:
		s.srv.Close()
		s.srv, s.code, s.conns = nil, "", nil
		log.Print("share: off")
	}
	a.relayApply()
	mdns := s.srv != nil && a.setting("mdns") == "on" && os.Getenv("HELLOBRO_NO_MDNS") == ""
	switch {
	case mdns && s.stopMDNS == nil:
		_, port, _ := net.SplitHostPort(shareAddr)
		p, _ := strconv.Atoi(port)
		s.stopMDNS = share.Advertise(share.Advert{Host: mdnsHost, Instance: "Kobo (hellobro)", Port: p, Iface: shareIf,
			TXT: []string{"u=root", "path=" + shareRoot}})
	case !mdns && s.stopMDNS != nil:
		s.stopMDNS()
		s.stopMDNS = nil
	}
	a.drawShare()
}

func (a *bro) shareExit() {
	s := &a.share
	if a.relay.stop != nil {
		close(a.relay.stop)
		a.relay.stop = nil
	}
	if s.srv != nil {
		s.srv.Close()
		s.srv = nil
	}
	if s.stopMDNS != nil {
		s.stopMDNS()
		s.stopMDNS = nil
	}
}

// sharePair opens pairing, sharing on first if it's off.
func (a *bro) sharePair() {
	s := &a.share
	if s.srv == nil {
		a.set["share"] = indexOf(settingList[settingIndex("share")].values, "on")
		a.set.save()
		a.shareApply()
	}
	if s.srv == nil {
		return
	}
	if c := os.Getenv("HELLOBRO_FAKE_PAIR_CODE"); c != "" { // the harness's computer knows it
		s.code = s.srv.PairWith(c, pairTime)
	} else {
		s.code = s.srv.Pair(pairTime)
	}
	s.until = time.Now().Add(pairTime)
	s.note = ""
	a.drawShare()
	a.drawSettings()
}

// shareBusy says whether a computer is connected.
func (a *bro) shareBusy() bool { return len(a.share.conns) > 0 }

func (a *bro) shareEvent(e share.Event) {
	s := &a.share
	c := e.Conn
	switch e.Kind {
	case "connected":
		s.note = fmt.Sprintf("%s@%s connected", c.User, c.Addr)
	case "closed":
		s.note = fmt.Sprintf("%s@%s left (%s in, %s out)", c.User, c.Addr, humanBytes(c.In), humanBytes(c.Out))
		if c.WroteSFTP {
			a.rescanMusic()
			a.fv.listAt = time.Time{} // look again
		}
	case "paired":
		s.note, s.code = "paired: "+e.Note, ""
		if s.srv != nil {
			s.keys, _ = s.srv.Keys()
		}
	case "pairing over":
		s.note, s.code = "pairing over: "+e.Note, ""
	case "refused":
		s.note = "refused " + e.Note
	}
	a.sharePoll()
	a.drawShare()
	a.drawSettings()
}

// sharePoll looks at the connections again.
func (a *bro) sharePoll() {
	s := &a.share
	if s.srv == nil {
		return
	}
	now := time.Now()
	conns := s.srv.Conns()
	rate := map[int]float64{}
	in := map[int]int64{}
	for _, c := range conns {
		in[c.ID] = c.In
		if prev, ok := s.lastIn[c.ID]; ok && now.After(s.polledAt) {
			rate[c.ID] = float64(c.In-prev) / now.Sub(s.polledAt).Seconds()
		}
	}
	s.conns, s.rate, s.lastIn, s.polledAt = conns, rate, in, now
	if s.code != "" && s.srv.Pairing() == "" {
		s.code = ""
	}
}

// shareTick polls now and then while it's on.
func (a *bro) shareTick() {
	s := &a.share
	if s.srv == nil || time.Since(s.polledAt) < time.Second {
		return
	}
	a.sharePoll()
	a.drawShare()
	if s.code != "" || a.Tab == tabRadio {
		a.drawSettings() // the countdown
	}
}

// sharedShell is a localShell, as the share package wants it.
type sharedShell struct{ *localShell }

func (s sharedShell) Resize(cols, rows int) { s.resize(cols, rows) }

func (s sharedShell) Wait() error {
	<-s.exit
	return s.err
}

func (s sharedShell) Close() error {
	s.close()
	return nil
}

// builtin runs our own commands, for "ssh kobo hellobro-…".
func (a *bro) builtin(cmd string, out io.Writer) (int, bool) {
	switch cmd {
	case "hellobro-version":
		fmt.Fprintln(out, "hellobro", version)
		return 0, true
	case "hellobro-update": // make push: hellobro.new is there; restart into it
		path := kobo.AppFile(newBinary)
		ver, err := checkBinary(path)
		if err != nil {
			fmt.Fprintf(out, "hellobro-update: %v\n", err)
			return 1, true
		}
		fmt.Fprintf(out, "hellobro-update: %s → %s, restarting\n", version, ver)
		time.AfterFunc(700*time.Millisecond, func() { a.Do(func() { a.installUpdate(path, ver) }) })
		return 0, true
	}
	return 0, false
}

func humanBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f kB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}
