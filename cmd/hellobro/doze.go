package main

// Light sleep (kobo's doze) for listening: the light off and the screen
// still on what's playing, while the music plays on and the headphones'
// buttons (or a keyboard's media keys) play, pause, skip and set the
// volume. It comes after a while without input while music plays, or
// from Sleep with music on (the settings say); a tap or a key wakes it.
// Wi-Fi goes off meanwhile, unless the settings keep it; once the music
// has stopped a while, the Kobo goes to sleep properly.

import (
	"fmt"
	"image"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/image/font"

	"hellobro/kobo"
)

type dozeState struct {
	since    time.Time
	quietAt  time.Time // when the music stopped, while dozing
	shown    string    // what the doze screen shows
	wifiOff  bool      // Wi-Fi is off for the doze, to go back on after
	wifiBusy bool      // the Wi-Fi panel shows it coming back
	jobs     chan string
}

// musicPlaying says whether music plays (or is about to).
func (a *bro) musicPlaying() bool {
	m := &a.mus
	return m.starting || m.player != nil && !m.player.Paused()
}

func (a *bro) DozeInstead() bool {
	return a.musicPlaying() && a.setting("sleep-music") == "light"
}

// dozeWhenIdle dozes once nothing's been touched a while with music on.
func (a *bro) dozeWhenIdle() {
	d := a.settingMinutes("doze-after")
	if d == 0 || !a.musicPlaying() || a.IdleFor() < d || a.overlay() || a.rs.prompt != nil || a.Calibrating() || a.shareBusy() {
		return
	}
	log.Printf("doze: %v without input, the music playing", d)
	a.Doze()
}

// dozeTick, instead of Tick while dozing: once the music has stopped
// a while, sleep.
func (a *bro) dozeTick() {
	z := &a.doze
	if a.musicPlaying() {
		z.quietAt = time.Time{}
		return
	}
	if z.quietAt.IsZero() {
		z.quietAt = time.Now()
	}
	if d := a.settingMinutes("doze-deep"); d > 0 && time.Since(z.quietAt) >= d {
		log.Printf("doze: no music for %v: sleeping", d)
		a.App.Sleep()
	}
}

func (a *bro) Dozed(on bool) {
	z, r := &a.doze, &a.rs
	z.quietAt, z.shown = time.Time{}, ""
	if !on {
		a.dozeWiFi("on")
		return
	}
	z.since = time.Now()
	if r.started && !r.discoverTo.IsZero() { // a scan would only stutter the music
		r.discoverTo = time.Time{}
		go r.bt.Discover(false)
	}
	if a.setting("doze-wifi") == "off" {
		a.dozeWiFi("off")
	}
}

// dozeWiFi turns Wi-Fi off for a doze ("off"), back on after it ("on"),
// or forgets it was on ("forget": sleep takes it from there), one after
// the other, off the loop.
func (a *bro) dozeWiFi(job string) {
	z := &a.doze
	if z.jobs == nil {
		a.radioStart()
		z.jobs = make(chan string, 8)
		wifi, bt := a.rs.wifi, a.rs.bt
		go func() {
			off := false // we turned it off
			progress := func(s string) { log.Printf("doze: wifi: %s", s) }
			for job := range z.jobs {
				switch {
				case job == "off" && !off && wifi.Status().On:
					if err := wifiOff(wifi, bt, progress); err != nil { // Bluetooth plays on
						log.Printf("doze: wifi off: %v", err)
					} else {
						off = true
					}
				case job == "on" && off:
					off = false
					a.Do(func() { a.dozeWiFiBusy("turning Wi-Fi back on…") })
					if err := wifi.SetPower(true, progress); err != nil {
						log.Printf("doze: wifi on: %v", err)
					}
					a.Do(func() {
						a.dozeWiFiBusy("")
						a.termKick()
						if a.showingRadio() {
							a.radioPoll(false)
						}
					})
				case job == "forget":
					off = false
				}
				a.Do(func() { z.wifiOff = off })
			}
		}()
	}
	z.jobs <- job
}

// dozeWiFiBusy shows in the Wi-Fi panel what the doze does with Wi-Fi,
// and keeps the panel's own power button off it meanwhile.
func (a *bro) dozeWiFiBusy(what string) {
	r, z := &a.rs, &a.doze
	if what != "" && r.wBusy != "" || what == "" && !z.wifiBusy {
		return // the panel is busy with its own, or it's not ours
	}
	r.wBusy, z.wifiBusy = what, what != ""
	a.drawRadioPanel(panelWiFi)
}

// dozeKey is what the doze screen shows, to redraw it only for a change.
func (a *bro) dozeKey() string {
	m := &a.mus
	key := fmt.Sprint(a.playing(), a.musicPlaying(), m.player != nil, m.shuffle)
	if m.q != nil {
		key += fmt.Sprint(m.q.pos, m.covers[a.playingCover()] != nil)
	}
	return key
}

// redoze redraws the doze screen if what it shows changed.
func (a *bro) redoze() {
	if a.Dozing() && a.dozeKey() != a.doze.shown {
		a.Redoze()
	}
}

func (a *bro) DrawDoze() {
	m, z, c := &a.mus, &a.doze, a.C
	z.shown = a.dozeKey()
	b := c.Bounds()
	c.Fill(b, kobo.White)
	line := func(y, h int, f font.Face, s string, g uint8) {
		c.Text(f, image.Rect(b.Min.X+60, y, b.Max.X-60, y+h), s, g, kobo.AlignCenter)
	}
	y := b.Min.Y + b.Dy()/10
	line(y, 60, a.F.Title, "hello bro · light sleep", kobo.Black)
	y += 100

	side := min(a.mv.coverSide, b.Dx()-160, b.Dy()*2/5)
	if m.q != nil && side > 0 {
		r := image.Rect(b.Min.X+(b.Dx()-side)/2, y, b.Min.X+(b.Dx()+side)/2, y+side)
		var img *image.Gray
		if path := a.playingCover(); path != "" {
			if img = m.covers[path]; img == nil {
				a.loadCover(path) // redozes once it's in
			}
		}
		if img != nil && img.Bounds().Dx() >= side {
			drawImage(c, r, img)
		} else {
			c.Fill(r, kobo.Light)
			c.Text(a.F.Huge, r, "♪", kobo.Mid, kobo.AlignCenter)
		}
		c.Frame(r, 2, kobo.Black)
		y = r.Max.Y + 50
	}

	if m.q == nil {
		line(y, 60, a.F.Panel, "Nothing playing.", kobo.Dark)
		y += 80
	} else {
		line(y, 60, a.F.Panel, trackName(filepath.Base(a.playing())), kobo.Black)
		y += 64
		line(y, 50, a.F.Body, a.playingAlbum(), kobo.Dark)
		y += 56
		state := "playing"
		switch {
		case m.player != nil && m.player.Paused():
			state = "paused"
		case !a.musicPlaying():
			state = "stopped"
		}
		if m.shuffle {
			state += " · shuffled"
		}
		line(y, 44, a.F.Small, a.queueLine()+" · "+state, kobo.Dark)
		y += 70
	}

	y = max(y, b.Max.Y-b.Dy()/6)
	line(y, 44, a.F.Small, "Headphone buttons: play/pause, next, previous, volume.", kobo.Dark)
	wake := "Press the power button or a key to wake up."
	switch {
	case !a.PowerButton() && a.Keyboard == "":
		wake = "Tap the screen to wake up."
	case !a.PowerButton():
		wake = "Press a key to wake up."
	}
	line(y+48, 44, a.F.Small, wake, kobo.Dark)
	note := "since " + z.since.Format("15:04")
	if bat, err := os.ReadFile("/sys/class/power_supply/battery/capacity"); err == nil {
		note += fmt.Sprintf(" · battery %s%%", strings.TrimSpace(string(bat)))
	}
	line(y+100, 40, a.F.Small, note, kobo.Mid)
}
