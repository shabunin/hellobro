// A stand-in MPRIS player for the harness (run.sh, MPRIS=1): mpv as
// hellobro sees it on an Alpine session's bus, with three tracks that
// play, pause, skip and set the volume as asked, and say so on stdout.
//
// usage: go run ./harness/mpris BUS_ADDRESS
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"
)

const (
	path   = "/org/mpris/MediaPlayer2"
	iface  = "org.mpris.MediaPlayer2.Player"
	status = "PlaybackStatus"
)

var tracks = []struct{ title, artist, album string }{
	{"Morning Tide", "The Harness Band", "Stand-ins"},
	{"Paper Boats", "The Harness Band", "Stand-ins"},
	{"Night Train", "The Harness Band", "Stand-ins"},
}

type player struct {
	props *prop.Properties
	track int
}

func meta(i int) map[string]dbus.Variant {
	t := tracks[i]
	return map[string]dbus.Variant{
		"mpris:trackid": dbus.MakeVariant(dbus.ObjectPath(fmt.Sprintf("/track/%d", i))),
		"xesam:title":   dbus.MakeVariant(t.title),
		"xesam:artist":  dbus.MakeVariant([]string{t.artist}),
		"xesam:album":   dbus.MakeVariant(t.album),
	}
}

func (p *player) set(s string) {
	p.props.SetMust(iface, status, s)
	log.Printf("mpris: %s, %s", s, tracks[p.track].title)
}

func (p *player) skip(d int) {
	p.track = (p.track + d + len(tracks)) % len(tracks)
	p.props.SetMust(iface, "Metadata", meta(p.track))
	p.set("Playing")
}

func (p *player) PlayPause() *dbus.Error {
	if p.props.GetMust(iface, status) == "Playing" {
		p.set("Paused")
	} else {
		p.set("Playing")
	}
	return nil
}

func (p *player) Play() *dbus.Error     { p.set("Playing"); return nil }
func (p *player) Pause() *dbus.Error    { p.set("Paused"); return nil }
func (p *player) Stop() *dbus.Error     { p.set("Stopped"); return nil }
func (p *player) Next() *dbus.Error     { p.skip(1); return nil }
func (p *player) Previous() *dbus.Error { p.skip(-1); return nil }

func main() {
	log.SetFlags(0)
	if len(os.Args) != 2 {
		log.Fatal("usage: mpris BUS_ADDRESS")
	}
	conn, err := dbus.Connect(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	p := &player{}
	p.props, err = prop.Export(conn, path, prop.Map{
		"org.mpris.MediaPlayer2": {
			"Identity": {Value: "mpv", Emit: prop.EmitFalse},
		},
		iface: {
			status:     {Value: "Stopped", Emit: prop.EmitTrue},
			"Metadata": {Value: meta(0), Emit: prop.EmitTrue},
			"Volume": {Value: 1.0, Writable: true, Emit: prop.EmitTrue, Callback: func(c *prop.Change) *dbus.Error {
				log.Printf("mpris: volume %.2f", c.Value)
				return nil
			}},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	if err := conn.Export(p, path, iface); err != nil {
		log.Fatal(err)
	}
	if r, err := conn.RequestName("org.mpris.MediaPlayer2.mpv", dbus.NameFlagDoNotQueue); err != nil || r != dbus.RequestNameReplyPrimaryOwner {
		log.Fatalf("name: %v %v", r, err)
	}
	log.Print("mpris: up")
	select {}
}
