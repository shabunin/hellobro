package main

// The Sound tab: Bluetooth status on top, a little player below.

import (
	"fmt"
	"image"
	"log"
	"time"

	"hellobro/kobo"
)

type soundState struct {
	bt     btReport
	btBusy bool
	btNote string // last connect attempt

	tracks   []string
	playing  string
	last     string // the track played last
	pos, len time.Duration
	note     string        // errors, "finished", …
	stop     chan struct{} // closes to stop the current track
	gain     int           // software volume, percent

	btRows, trackRows []image.Rectangle // from the last draw, for taps
	volRow            image.Rectangle
}

const firstTrackLine = 3 // the player's lines before the track list

func (a *tools) enterSound() {
	if a.snd.tracks == nil {
		a.snd.tracks = append([]string{toneTrack}, findTracks()...)
		log.Printf("sound: %d tracks under %s", len(a.snd.tracks)-1, musicDir)
	}
	if a.snd.bt.lines == nil && !a.snd.btBusy {
		a.refreshBluetooth()
	}
}

func (a *tools) refreshBluetooth() {
	if a.snd.btBusy {
		return
	}
	a.snd.btBusy = true
	go func() {
		r := probeBluetooth()
		a.Do(func() {
			if old := a.snd.bt.bus; old != nil && old != r.bus {
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
	lines := append([]string(nil), r.lines...)
	for _, d := range r.devices {
		s := fmt.Sprintf("%s  %s", d.name, d.addr)
		switch {
		case d.connected:
			s += "  · connected"
		case d.paired:
			s += "  · paired (tap to connect)"
		}
		if d.isAudio {
			s += " · audio"
		}
		lines = append(lines, s)
	}
	for _, p := range r.pcms {
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
	} else if len(a.snd.bt.pcms) > 0 {
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
		status = fmt.Sprintf("%s / %s", clock(s.pos), clock(s.len))
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
		a.drawPlayer()
	}
	for i, r := range s.trackRows {
		if p.In(r) {
			a.play(s.tracks[i])
			return
		}
	}
	devStart := len(s.bt.lines)
	for i, d := range s.bt.devices {
		if devStart+i < len(s.btRows) && p.In(s.btRows[devStart+i]) && !d.connected && s.bt.bus != nil {
			a.connect(d)
			return
		}
	}
}

func (a *tools) connect(d btDevice) {
	a.snd.btNote = "connecting to " + d.name + "…"
	a.drawBluetooth()
	bus := a.snd.bt.bus
	go func() {
		err := connectDevice(bus, d)
		a.Do(func() {
			if err != nil {
				a.snd.btNote = fmt.Sprintf("connect %s: %v", d.name, err)
			} else {
				a.snd.btNote = "connected " + d.name
			}
			log.Print(a.snd.btNote)
			a.refreshBluetooth()
		})
	}()
}

// pickPCM prefers a playback PCM of a connected device.
func (a *tools) pickPCM() (bluealsaPCM, bool) {
	var found []bluealsaPCM
	for _, p := range a.snd.bt.pcms {
		if p.mode == "sink" {
			found = append(found, p)
		}
	}
	for _, p := range found {
		for _, d := range a.snd.bt.devices {
			if d.path == p.device && d.connected {
				return p, true
			}
		}
	}
	if len(found) > 0 {
		return found[0], true
	}
	return bluealsaPCM{}, false
}

func (a *tools) play(track string) {
	a.stopPlayback("")
	s := &a.snd
	pcm, ok := a.pickPCM()
	if !ok || s.bt.bus == nil {
		s.note = "no Bluetooth audio: connect the headphones/DAC in Kobo, then refresh"
		a.drawPlayer()
		return
	}
	if pcm.format != 0x8210 && pcm.format != 0 {
		s.note = fmt.Sprintf("PCM format %#04x isn't 16-bit; can't play", pcm.format)
		a.drawPlayer()
		return
	}
	rate, channels := pcm.rate, pcm.channels
	if rate == 0 {
		rate = 44100
	}
	if channels == 0 {
		channels = 2
	}
	src, err := openTrack(track, rate)
	if err != nil {
		s.note = err.Error()
		a.drawPlayer()
		return
	}
	stop := make(chan struct{})
	s.playing, s.last, s.stop, s.pos, s.len, s.note = track, track, stop, 0, src.length(), ""
	log.Printf("sound: playing %q at %d Hz (source %d Hz) to %s", track, rate, src.rate(), pcm.path)
	a.drawPlayer()

	bus := s.bt.bus
	gain := make(chan float64, 1)
	gain <- float64(s.gain) / 100
	curGain := float64(s.gain) / 100
	go func() {
		defer src.close()
		done := func(note string) {
			a.Do(func() {
				log.Printf("sound: %q ended: %s", track, note)
				if a.snd.stop == stop {
					a.snd.playing, a.snd.stop, a.snd.note = "", nil, note
					a.drawPlayer()
				}
			})
		}
		data, ctrl, err := openPCM(bus, pcm)
		if err != nil {
			done(err.Error())
			return
		}
		defer data.Close()
		defer ctrl.Close()
		err = writePCM(data, src, rate, channels, func() float64 {
			select {
			case g := <-gain:
				curGain = g
			default:
			}
			return curGain
		}, stop, func(pos time.Duration) {
			a.Do(func() {
				if a.snd.stop == stop {
					a.snd.pos = pos
					select { // pass on volume changes
					case gain <- float64(a.snd.gain) / 100:
					default:
					}
					a.drawPlayerStatus()
				}
			})
		})
		select {
		case <-stop:
			ctrl.Write([]byte("Drop"))
			done("stopped")
		default:
			if err != nil {
				done(err.Error())
				return
			}
			ctrl.Write([]byte("Drain"))
			buf := make([]byte, 32)
			ctrl.SetReadDeadline(time.Now().Add(5 * time.Second))
			ctrl.Read(buf) // "OK" once drained
			done("finished")
		}
	}()
}

// drawPlayerStatus updates just the position in the title strip.
func (a *tools) drawPlayerStatus() {
	if a.Tab != tabSound || a.Calibrating() || a.Blitting() {
		return
	}
	a.DrawPanelTitle(a.L.playerP, fmt.Sprintf("%s / %s", clock(a.snd.pos), clock(a.snd.len)))
}

func (a *tools) stopPlayback(note string) {
	if a.snd.stop != nil {
		close(a.snd.stop)
		a.snd.stop, a.snd.playing = nil, ""
	}
	a.snd.note = note
}

func clock(d time.Duration) string {
	if d <= 0 {
		return "-:--"
	}
	s := int(d.Seconds())
	return fmt.Sprintf("%d:%02d", s/60, s%60)
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
