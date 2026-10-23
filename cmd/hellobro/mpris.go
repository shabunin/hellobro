package main

// The music is a Linux player's: mpv (with mpv-mpris), cmus, mpd (with
// mpd-mpris) or another that has MPRIS, the D-Bus remote control, in an
// Alpine session. Each session is a jail with a session bus of its own
// (alpineinit.go, once dbus is installed), which we reach through its
// first process's root. The media keys go there: a keyboard's, Ctrl+
// Shift+Space, < and >, and the headphones' buttons, which BlueZ sends
// as keys while Nickel is closed. What plays shows in the header and
// the status bar, and keeps us from quitting or sleeping.
//
// The players are looked at when a bus says something changed (their
// properties, a player coming or going), and every few seconds for new
// sessions' buses: nothing polls the players themselves.

import (
	"context"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"

	"hellobro/kobo"
)

const (
	mprisPrefix = "org.mpris.MediaPlayer2."
	mprisPath   = "/org/mpris/MediaPlayer2"
	mprisPlayer = "org.mpris.MediaPlayer2.Player"
	jailBus     = "/run/user/0/bus" // a session's bus, in the jail
	busesEvery  = 5 * time.Second   // new sessions' buses are looked for this often
	mprisWait   = 2 * time.Second   // a player that doesn't answer in this long is skipped
	volumeStep  = 0.05
)

// player is an MPRIS player, as last seen.
type player struct {
	bus      *dbus.Conn
	name     string // its bus name, org.mpris.MediaPlayer2.mpv
	identity string // what it calls itself, "mpv"
	status   string // Playing, Paused or Stopped
	title    string
	artist   string
	album    string
	volume   float64
}

// mediaState is the players, for the loop.
type mediaState struct {
	players []player
	kick    chan struct{} // look again now
}

// jails are the Alpine sessions' first processes, whose buses we watch.
var jails struct {
	sync.Mutex
	pids []int
}

func jailStarted(pid int) {
	jails.Lock()
	jails.pids = append(jails.pids, pid)
	jails.Unlock()
}

// jailCount is how many jails are running: Alpine sessions, here or over
// SSH.
func jailCount() int {
	jails.Lock()
	defer jails.Unlock()
	return len(jails.pids)
}

func jailEnded(pid int) {
	jails.Lock()
	jails.pids = slices.DeleteFunc(jails.pids, func(p int) bool { return p == pid })
	jails.Unlock()
}

// busAddrs are the buses to watch, by where they are: the sessions', and
// HELLOBRO_MPRIS_BUS's (the harness's player).
func busAddrs() []string {
	var addrs []string
	jails.Lock()
	for _, pid := range jails.pids {
		path := fmt.Sprintf("/proc/%d/root%s", pid, jailBus)
		if _, err := os.Stat(path); err == nil {
			addrs = append(addrs, "unix:path="+path)
		}
	}
	jails.Unlock()
	if s := os.Getenv("HELLOBRO_MPRIS_BUS"); s != "" {
		addrs = append(addrs, s)
	}
	return addrs
}

// watchPlayers keeps a.media.players up to date, off the loop. Make
// a.media.kick first.
func (a *bro) watchPlayers() {
	buses := map[string]*dbus.Conn{}
	signals := make(chan struct{}, 1)
	tick := time.NewTicker(busesEvery)
	for {
		changed := false
		addrs := busAddrs()
		for addr, conn := range buses {
			if !slices.Contains(addrs, addr) || !conn.Connected() {
				conn.Close()
				delete(buses, addr)
				changed = true
			}
		}
		for _, addr := range addrs {
			if buses[addr] != nil {
				continue
			}
			conn, err := watchBus(addr, signals)
			if err != nil {
				continue // the bus isn't up yet, or it's going: next time
			}
			log.Printf("media: watching %s", addr)
			buses[addr] = conn
			changed = true
		}
		if changed || len(buses) > 0 {
			var all []player
			for _, conn := range buses {
				all = append(all, players(conn)...)
			}
			a.Do(func() { a.setPlayers(all) })
		}
		select {
		case <-tick.C:
		case <-a.media.kick:
		case <-signals:
			time.Sleep(200 * time.Millisecond) // a change comes as a few signals
			select {
			case <-signals:
			default:
			}
		}
	}
}

// watchBus connects to a bus and has signals told when its players change.
func watchBus(addr string, signals chan struct{}) (*dbus.Conn, error) {
	conn, err := dbus.Connect(addr)
	if err != nil {
		return nil, err
	}
	err = conn.AddMatchSignal(dbus.WithMatchObjectPath(mprisPath), dbus.WithMatchInterface("org.freedesktop.DBus.Properties"))
	if err == nil {
		err = conn.AddMatchSignal(dbus.WithMatchInterface("org.freedesktop.DBus"), dbus.WithMatchMember("NameOwnerChanged"),
			dbus.WithMatchArg0Namespace("org.mpris.MediaPlayer2"))
	}
	if err != nil {
		conn.Close()
		return nil, err
	}
	ch := make(chan *dbus.Signal, 16)
	conn.Signal(ch) // closed with the connection
	go func() {
		for range ch {
			select {
			case signals <- struct{}{}:
			default:
			}
		}
	}()
	return conn, nil
}

// players reads the players on a bus.
func players(conn *dbus.Conn) []player {
	ctx, cancel := context.WithTimeout(context.Background(), mprisWait)
	defer cancel()
	var names []string
	if conn.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.ListNames", 0).Store(&names) != nil {
		return nil
	}
	var ps []player
	for _, n := range names {
		if !strings.HasPrefix(n, mprisPrefix) {
			continue
		}
		obj := conn.Object(n, mprisPath)
		var props map[string]dbus.Variant
		if obj.CallWithContext(ctx, "org.freedesktop.DBus.Properties.GetAll", 0, mprisPlayer).Store(&props) != nil {
			continue
		}
		p := player{bus: conn, name: n, identity: strings.TrimPrefix(n, mprisPrefix),
			status: prop[string](props, "PlaybackStatus"), volume: prop[float64](props, "Volume")}
		var id dbus.Variant
		if obj.CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0, "org.mpris.MediaPlayer2", "Identity").Store(&id) == nil {
			if s, ok := id.Value().(string); ok && s != "" {
				p.identity = s
			}
		}
		meta := prop[map[string]dbus.Variant](props, "Metadata")
		p.title = prop[string](meta, "xesam:title")
		p.album = prop[string](meta, "xesam:album")
		p.artist = strings.Join(prop[[]string](meta, "xesam:artist"), ", ")
		if p.title == "" { // a file without tags: its name
			if u := prop[string](meta, "xesam:url"); u != "" {
				p.title = u[strings.LastIndex(u, "/")+1:]
			}
		}
		ps = append(ps, p)
	}
	return ps
}

// prop reads a property, or T's zero value.
func prop[T any](props map[string]dbus.Variant, name string) T {
	var zero T
	if v, ok := props[name]; ok {
		if t, ok := v.Value().(T); ok {
			return t
		}
	}
	return zero
}

func (a *bro) setPlayers(ps []player) {
	was := a.nowPlaying()
	a.media.players = ps
	if now := a.nowPlaying(); now != was {
		a.SetStatus(now)
		a.drawStat(false)
	}
	a.SetLockLines(a.lockLines())
}

// lookAgain has the players looked at now, after a key changed them.
func (a *bro) lookAgain() {
	select {
	case a.media.kick <- struct{}{}:
	default:
	}
}

// target is the player the keys go to: the one playing, else a paused
// one, else any.
func (a *bro) target() (player, bool) {
	ps := a.media.players
	for _, status := range []string{"Playing", "Paused"} {
		if i := slices.IndexFunc(ps, func(p player) bool { return p.status == status }); i >= 0 {
			return ps[i], true
		}
	}
	if len(ps) > 0 {
		return ps[0], true
	}
	return player{}, false
}

// musicPlaying says whether a player plays.
func (a *bro) musicPlaying() bool {
	return slices.ContainsFunc(a.media.players, func(p player) bool { return p.status == "Playing" })
}

// nowPlaying is the header's line: what the keys' player plays.
func (a *bro) nowPlaying() string {
	p, ok := a.target()
	if !ok || p.status == "Stopped" || p.title == "" {
		return ""
	}
	s := "♪ " + p.title
	if p.artist != "" {
		s += " · " + p.artist
	}
	if p.status == "Paused" {
		s += " (paused)"
	}
	return s
}

// mediaKey sends a media key to the player.
func (a *bro) mediaKey(code uint16) {
	p, ok := a.target()
	if !ok {
		a.Toast("No player. In an Alpine shell: apk add dbus mpv mpv-mpris (or cmus), open a new one, play there.", 6*time.Second)
		return
	}
	var method string
	var volume float64
	switch code {
	case kobo.KeyPlayPause, kobo.KeyPlayCD, kobo.KeyPauseCD:
		method = "PlayPause" // headphones send play or pause as they think it's playing: either toggles
	case kobo.KeyStopCD:
		method = "Stop"
	case kobo.KeyNextSong:
		method = "Next"
	case kobo.KeyPreviousSong:
		method = "Previous"
	case kobo.KeyVolumeUp:
		volume = min(1, p.volume+volumeStep)
	case kobo.KeyVolumeDown:
		volume = max(0, p.volume-volumeStep)
	default:
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), mprisWait)
		defer cancel()
		obj := p.bus.Object(p.name, mprisPath)
		var err error
		if method != "" {
			err = obj.CallWithContext(ctx, mprisPlayer+"."+method, 0).Err
		} else {
			err = obj.CallWithContext(ctx, "org.freedesktop.DBus.Properties.Set", 0, mprisPlayer, "Volume", dbus.MakeVariant(volume)).Err
		}
		if err != nil {
			log.Printf("media: %s %s: %v", p.identity, method, err)
		}
		a.lookAgain()
	}()
}

// pausePlayers pauses what plays, before deep sleep takes the sound away.
func (a *bro) pausePlayers() {
	for _, p := range a.media.players {
		if p.status != "Playing" {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), mprisWait)
		if err := p.bus.Object(p.name, mprisPath).CallWithContext(ctx, mprisPlayer+".Pause", 0).Err; err != nil {
			log.Printf("media: %s: pause: %v", p.identity, err)
		}
		cancel()
	}
}
