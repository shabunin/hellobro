package main

// The Sound tab: Bluetooth status on top, a little player below, to try
// the audio path (hellobro's Music tab is the real player).

import (
	"fmt"
	"image"
	"io/fs"
	"log"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"hellobro/audio"
	"hellobro/kobo"
)

const toneTrack = "test tone (a little arpeggio)"

var musicDir = kobo.EnvOr("HELLOBRO_MUSIC_DIR", "/mnt/onboard")

type soundState struct {
	bt     audio.Report
	btBusy bool
	btNote string // last connect attempt

	tracks   []string
	playing  string
	last     string // the track played last
	pos, len time.Duration
	note     string        // errors, "finished", …
	player   *audio.Player // the track playing
	gain     int           // software volume, percent

	btRows, trackRows []image.Rectangle // from the last draw, for taps
	volRow            image.Rectangle
}

const firstTrackLine = 3 // the player's lines before the track list

// findTracks lists MP3 and WAV files a few folders deep, skipping hidden ones.
func findTracks() []string {
	var tracks []string
	filepath.WalkDir(musicDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(musicDir, p)
		if d.IsDir() {
			if p != musicDir && (strings.HasPrefix(d.Name(), ".") || strings.Count(rel, "/") >= 3) {
				return filepath.SkipDir
			}
			return nil
		}
		if audio.Playable(p) {
			tracks = append(tracks, rel)
		}
		if len(tracks) >= 60 {
			return fs.SkipAll
		}
		return nil
	})
	sort.Strings(tracks)
	return tracks
}

func (a *tools) enterSound() {
	if a.snd.tracks == nil {
		a.snd.tracks = append([]string{toneTrack}, findTracks()...)
		log.Printf("sound: %d tracks under %s", len(a.snd.tracks)-1, musicDir)
	}
	if a.snd.bt.Lines == nil && !a.snd.btBusy {
		a.refreshBluetooth()
	}
}

func (a *tools) refreshBluetooth() {
	if a.snd.btBusy {
		return
	}
	a.snd.btBusy = true
	go func() {
		r := audio.Probe()
		a.Do(func() {
			if old := a.snd.bt.Bus; old != nil && old != r.Bus {
				old.Close()
			}
			a.snd.bt, a.snd.btBusy = r, false
			log.Printf("bluetooth:\n\t%s", joinLines(a.btLines()))
			a.redrawSound()
		})
	}()
	a.redrawSound()
}

func (a *tools) redrawSound() {
	if a.Tab == tabSound && !a.Calibrating() {
		a.drawBluetooth()
		a.drawPlayer()
	}
}

// btLines is the Bluetooth panel: general lines, then one per device, then PCMs.
func (a *tools) btLines() []string {
	r := a.snd.bt
	lines := append([]string(nil), r.Lines...)
	for _, d := range r.Devices {
		s := fmt.Sprintf("%s  %s", d.Name, d.Addr)
		switch {
		case d.Connected:
			s += "  · connected"
		case d.Paired:
			s += "  · paired (tap to connect)"
		}
		if d.IsAudio {
			s += " · audio"
		}
		lines = append(lines, s)
	}
	for _, p := range r.PCMs {
		lines = append(lines, "   PCM "+p.String())
	}
	if a.snd.btNote != "" {
		lines = append(lines, a.snd.btNote)
	}
	return lines
}

func (a *tools) drawBluetooth() {
	if a.Tab != tabSound || a.Calibrating() {
		return
	}
	status := ""
	if a.snd.btBusy {
		status = "looking…"
	} else if len(a.snd.bt.PCMs) > 0 {
		status = "audio ready"
	}
	a.DrawPanel(a.L.btP, status)
	a.snd.btRows = a.DrawText(a.L.btP, a.F.Small, a.btLines(), "Tap refresh.", true)
}

func (a *tools) drawPlayer() {
	if a.Tab != tabSound || a.Calibrating() {
		return
	}
	s := &a.snd
	status := "stopped"
	if s.playing != "" {
		status = fmt.Sprintf("%s / %s", audio.Clock(s.pos), audio.Clock(s.len))
	}
	a.DrawPanel(a.L.playerP, status)
	now := "nothing playing"
	if s.playing != "" {
		now = "playing: " + s.playing
	}
	if s.note != "" {
		now += "  (" + s.note + ")"
	}
	lines := []string{
		fmt.Sprintf("volume %d%%        tap here: left half −10, right half +10", s.gain),
		now,
		fmt.Sprintf("tracks (MP3/WAV in %s):", musicDir),
	}
	for _, t := range s.tracks {
		mark := "   "
		if t == s.playing {
			mark = ">  "
		}
		lines = append(lines, mark+t)
	}
	rows := a.DrawText(a.L.playerP, a.F.Body, lines, "", true)
	s.volRow, s.trackRows = rows[0], rows[firstTrackLine:]
}

// tapSound handles taps on the Sound tab.
func (a *tools) tapSound(p image.Point) {
	s := &a.snd
	switch {
	case kobo.Hit(p, a.L.btP.Button):
		a.refreshBluetooth()
	case kobo.Hit(p, a.L.playerP.Button):
		a.stopPlayback("stopped")
		a.drawPlayer()
	case p.In(s.volRow):
		if p.X < s.volRow.Min.X+s.volRow.Dx()/2 {
			s.gain = max(0, s.gain-10)
		} else {
			s.gain = min(150, s.gain+10)
		}
		if s.player != nil {
			s.player.SetGain(s.gain)
		}
		a.drawPlayer()
	}
	for i, r := range s.trackRows {
		if p.In(r) {
			a.play(s.tracks[i])
			return
		}
	}
	devStart := len(s.bt.Lines)
	for i, d := range s.bt.Devices {
		if devStart+i < len(s.btRows) && p.In(s.btRows[devStart+i]) && !d.Connected && s.bt.Bus != nil {
			a.connect(d)
			return
		}
	}
}

func (a *tools) connect(d audio.Device) {
	a.snd.btNote = "connecting to " + d.Name + "…"
	a.drawBluetooth()
	bus := a.snd.bt.Bus
	go func() {
		err := audio.Connect(bus, d)
		a.Do(func() {
			if err != nil {
				a.snd.btNote = fmt.Sprintf("connect %s: %v", d.Name, err)
			} else {
				a.snd.btNote = "connected " + d.Name
			}
			log.Print(a.snd.btNote)
			a.refreshBluetooth()
		})
	}()
}

func (a *tools) play(track string) {
	a.stopPlayback("")
	s := &a.snd
	pcm, ok := s.bt.Sink()
	if !ok {
		s.note = "no Bluetooth audio: connect the headphones/DAC in Kobo, then refresh"
		a.drawPlayer()
		return
	}
	var src audio.Source
	if track == toneTrack {
		src = audio.Tone(max(pcm.Rate, 8000))
	} else {
		var err error
		if src, err = audio.Open(filepath.Join(musicDir, track)); err != nil {
			s.note = err.Error()
			a.drawPlayer()
			return
		}
	}
	log.Printf("sound: playing %q (%d Hz) to %s", track, src.Rate(), pcm.Path)
	var p *audio.Player
	p = audio.Play(s.bt.Bus, pcm, src, s.gain, func(pos time.Duration) {
		a.Do(func() {
			if a.snd.player == p {
				a.snd.pos = pos
				a.drawPlayerStatus()
			}
		})
	}, func(how string) {
		a.Do(func() {
			log.Printf("sound: %q ended: %s", track, how)
			if a.snd.player == p {
				a.snd.playing, a.snd.player, a.snd.note = "", nil, how
				a.drawPlayer()
			}
		})
	})
	s.playing, s.last, s.player, s.pos, s.len, s.note = track, track, p, 0, p.Length, ""
	a.drawPlayer()
}

// drawPlayerStatus updates just the position in the title strip.
func (a *tools) drawPlayerStatus() {
	if a.Tab != tabSound || a.Calibrating() || a.Blitting() {
		return
	}
	a.DrawPanelTitle(a.L.playerP, fmt.Sprintf("%s / %s", audio.Clock(a.snd.pos), audio.Clock(a.snd.len)))
}

func (a *tools) stopPlayback(note string) {
	if a.snd.player != nil {
		a.snd.player.Stop()
		a.snd.player, a.snd.playing = nil, ""
	}
	a.snd.note = note
}

// mediaKey handles headphone/keyboard media buttons. There's no pause:
// "pause" stops, "play" starts the last track again.
func (a *tools) mediaKey(code uint16) {
	s := &a.snd
	log.Printf("media key %d", code)
	if s.tracks == nil {
		s.tracks = append([]string{toneTrack}, findTracks()...)
	}
	cur := 0
	for i, t := range s.tracks {
		if t == s.playing || (s.playing == "" && t == s.last) {
			cur = i
		}
	}
	switch code {
	case kobo.KeyStopCD, kobo.KeyPauseCD:
		a.stopPlayback("stopped")
		a.drawPlayer()
	case kobo.KeyPlayPause, kobo.KeyPlayCD:
		if s.playing != "" && code == kobo.KeyPlayPause {
			a.stopPlayback("stopped")
			a.drawPlayer()
		} else if s.playing == "" {
			a.play(s.tracks[cur])
		}
	case kobo.KeyNextSong:
		a.play(s.tracks[(cur+1)%len(s.tracks)])
	case kobo.KeyPreviousSong:
		a.play(s.tracks[(cur+len(s.tracks)-1)%len(s.tracks)])
	}
}
