package main

// The Music tab: albums, folders and playlists from the Kobo's storage,
// played over Bluetooth to headphones or a DAC connected in Kobo's
// settings, in order or shuffled. Music goes on while the terminals are
// up; the header says what's playing. Where it was (what was playing,
// the track, position, volume, shuffle) is kept in music-state, so it
// picks up there next time. The library's side is in libview.go.

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"hellobro/audio"
	"hellobro/kobo"
)

const (
	btnPrev = iota
	btnBack
	btnPlay
	btnFwd
	btnNext
	btnVolDown
	btnVolUp
	numButtons
)

var buttonLabels = [numButtons]string{btnBack: "−10 s", btnFwd: "+10 s", btnVolDown: "vol −", btnVolUp: "vol +"}

const (
	rowH          = 52               // a library row
	seekStep      = 10 * time.Second // ←/→, and the buttons
	gainStep      = 5
	gainMax       = 150
	playingIdle   = 6 * time.Hour // while music plays, don't quit for want of input
	musicStateTxt = "music-state"
)

// The chips under the buttons: shuffle, repeat, and adding the track to
// a playlist.
const (
	chipShuffle = iota
	chipRepeat
	chipAdd
	numChips
)

// musicView is where the Music tab's things are.
type musicView struct {
	nowP, libP              kobo.Panel
	cover                   image.Rectangle
	albumR, titleR, trackR  image.Rectangle
	times, bar, statusR     image.Rectangle
	buttons                 [numButtons]image.Rectangle
	chipRow                 image.Rectangle
	chips                   [numChips]image.Rectangle
	segs                    [numViews]image.Rectangle // Albums, Folders, Playlists
	listR, actR             image.Rectangle           // the rows, and the action bar under them
	coverSide, rowsOnScreen int
}

type musicState struct {
	lib               *library
	scanned, scanning bool
	thens             []func() // to run once scanned

	view   int               // the library's view: viewAlbums, viewFolders, viewPlaylists
	places [numViews]place   // where each view is
	list   []libRow          // the rows of the place shown; nil: to be made
	rows   []image.Rectangle // the rows on screen, for taps
	acts   []libAct          // the action bar's buttons, as drawn

	bt                 audio.Report
	btBusy, connecting bool
	player             *audio.Player
	gen                int    // bumps per track started, so a late start is dropped
	q                  *queue // playing, paused, or played last; nil: none
	shuffle            bool
	repeat             repeatMode
	pos, length        time.Duration
	starting           bool
	gain               int
	note               string

	covers   map[string]*image.Gray // by path, coverSide big
	loading  map[string]bool
	shownSec int // the position on screen, in whole seconds
}

func (a *bro) layoutMusic(K kobo.Layout) {
	v := &a.mv
	var nr, lr image.Rectangle
	if K.Landscape {
		nr, lr = K.Split(K.Body, 0.42)
	} else {
		nr, lr = K.Split(K.Body, 0.31)
	}
	v.nowP = kobo.NewPanel(a.F, "Now playing", nr, "")
	v.libP = kobo.NewPanel(a.F, "Library", lr, "")

	b := v.nowP.Body.Inset(kobo.Pad)
	var info image.Rectangle
	if b.Dx() > b.Dy()*3/2 { // cover on the left, the rest beside it
		side := b.Dy()
		v.cover = image.Rect(b.Min.X, b.Min.Y, b.Min.X+side, b.Min.Y+side)
		info = image.Rect(v.cover.Max.X+2*kobo.Pad, b.Min.Y, b.Max.X, b.Max.Y)
	} else { // cover on top
		side := max(100, min(b.Dx(), b.Dy()-410))
		x := b.Min.X + (b.Dx()-side)/2
		v.cover = image.Rect(x, b.Min.Y, x+side, b.Min.Y+side)
		info = image.Rect(b.Min.X, v.cover.Max.Y+kobo.Pad, b.Max.X, b.Max.Y)
	}
	if v.coverSide != v.cover.Dx() {
		v.coverSide = v.cover.Dx()
		a.mus.covers = nil // they're made to measure
	}
	y := info.Min.Y
	next := func(h int) image.Rectangle {
		r := image.Rect(info.Min.X, y, info.Max.X, y+h)
		y += h
		return r
	}
	v.albumR, v.titleR, v.trackR = next(40), next(58), next(36)
	y += 8
	v.times, v.bar = next(34), next(22)
	y += 20
	row := next(76)
	y += 12
	v.chipRow = next(52)
	y += 8
	v.statusR = next(36)
	const gap = 12
	bw := min(130, (row.Dx()-gap*(numButtons-1))/numButtons)
	x := row.Min.X
	for i := range v.buttons {
		v.buttons[i] = image.Rect(x, row.Min.Y, x+bw, row.Max.Y)
		x += bw + gap
	}
	a.layoutLibrary()
}

// --- the library ---

func (a *bro) enterMusic() {
	a.scanMusic(nil)
	if a.mus.bt.Lines == nil {
		a.probeAudio()
	}
}

// scanMusic finds the albums and playlists, once; then runs when they're
// known.
func (a *bro) scanMusic(then func()) {
	m := &a.mus
	if m.scanned {
		if then != nil {
			then()
		}
		return
	}
	if then != nil {
		m.thens = append(m.thens, then)
	}
	if m.scanning {
		return
	}
	m.scanning = true
	go func() {
		lib := scanLibrary(musicRoot)
		a.Do(func() {
			m.lib, m.scanned, m.scanning = lib, true, false
			log.Printf("music: %d albums, %d tracks, %d playlists under %s", len(lib.albums), lib.tracks, len(lib.playlists), musicRoot)
			a.loadMusicState()
			a.relist()
			a.redrawMusic()
			for _, f := range m.thens {
				f()
			}
			m.thens = nil
		})
	}()
}

// rescanMusic finds the albums again (a computer sent some), keeping
// what's playing and where the library is. What's playing goes on:
// its queue is a list of files.
func (a *bro) rescanMusic() {
	m := &a.mus
	if !m.scanned || m.scanning {
		return // a scan to come sees them anyway
	}
	m.scanning = true
	go func() {
		lib := scanLibrary(musicRoot)
		a.Do(func() {
			m.scanning = false
			was := len(m.lib.albums)
			m.lib = lib
			log.Printf("music: rescanned: %d albums (%+d), %d playlists", len(lib.albums), len(lib.albums)-was, len(lib.playlists))
			a.relist()
			a.redrawMusic()
		})
	}()
}

// probeAudio looks for the Bluetooth device to play to.
func (a *bro) probeAudio() {
	m := &a.mus
	if m.btBusy || m.player != nil {
		return
	}
	m.btBusy = true
	go func() {
		r := audio.Probe()
		a.Do(func() {
			a.setBluetooth(r)
			m.btBusy = false
			log.Printf("music: bluetooth: %s", a.outputStatus())
			if a.showingMusic() {
				a.drawNow()
			}
		})
	}()
}

func (a *bro) setBluetooth(r audio.Report) {
	if old := a.mus.bt.Bus; old != nil && old != r.Bus {
		old.Close()
	}
	a.mus.bt = r
}

// audioDevice is the paired audio device that isn't connected, if any:
// one to offer connecting to.
func (a *bro) audioDevice() (audio.Device, bool) {
	if _, ok := a.mus.bt.Sink(); ok || a.mus.bt.Bus == nil {
		return audio.Device{}, false
	}
	for _, d := range a.mus.bt.Devices {
		if d.IsAudio {
			return d, true
		}
	}
	return audio.Device{}, false
}

func (a *bro) outputStatus() string {
	m := &a.mus
	switch {
	case m.connecting:
		return "connecting…"
	case m.btBusy:
		return "looking for Bluetooth audio…"
	}
	if p, ok := m.bt.Sink(); ok {
		if d, ok := m.bt.DeviceOf(p); ok {
			return "→ " + d.Name
		}
		return "→ " + p.Codec
	}
	if d, ok := a.audioDevice(); ok {
		return d.Name + " isn't connected"
	}
	return "no Bluetooth audio device"
}

func (a *bro) nowPanel() kobo.Panel {
	if _, ok := a.audioDevice(); ok && !a.mus.connecting {
		return kobo.WithButton(a.F, a.mv.nowP, "connect")
	}
	return a.mv.nowP
}

// --- drawing ---

func (a *bro) showingMusic() bool {
	return a.Tab == tabMusic && !a.Calibrating() && !a.overlay() && !a.Dozing()
}

func (a *bro) redrawMusic() {
	if a.showingMusic() {
		a.drawNow()
		a.drawLibrary()
	}
}

func (a *bro) drawMusic() {
	a.drawNow()
	a.drawLibrary()
}

// playing is the file playing, paused, or played last; "" for none.
func (a *bro) playing() string {
	if q := a.mus.q; q != nil {
		return q.path()
	}
	return ""
}

// playingAlbum is the name of the album (the folder) playing.
func (a *bro) playingAlbum() string {
	dir := filepath.Dir(a.playing())
	if al := a.mus.lib.album(dir); al != nil {
		return al.name
	}
	return filepath.Base(dir)
}

// playingCover is the cover of the album playing, or "".
func (a *bro) playingCover() string {
	if al := a.mus.lib.album(filepath.Dir(a.playing())); al != nil {
		return al.cover
	}
	return ""
}

// queueLine says where in the queue it is: "track 3 of 40 · folder Heroes".
func (a *bro) queueLine() string {
	q := a.mus.q
	s := fmt.Sprintf("track %d of %d", q.pos+1, q.len())
	if q.kind != qAlbum {
		s += " · " + q.kind + " " + q.name
	}
	return s
}

// drawNow draws the Now playing panel.
func (a *bro) drawNow() {
	m, v := &a.mus, &a.mv
	a.DrawPanel(a.nowPanel(), a.outputStatus())
	a.drawCover()

	c := a.C
	if m.q == nil {
		c.Text(a.F.Body, v.albumR, "nothing playing", kobo.Mid, kobo.AlignLeft)
		hint := "Pick an album, a folder or a playlist below."
		if !m.scanned {
			hint = "Looking for music…"
		}
		c.Text(a.F.Small, v.trackR, hint, kobo.Dark, kobo.AlignLeft)
	} else {
		c.Text(a.F.Body, v.albumR, a.playingAlbum(), kobo.Dark, kobo.AlignLeft)
		c.Text(a.F.Title, v.titleR, trackName(filepath.Base(a.playing())), kobo.Black, kobo.AlignLeft)
		state := ""
		switch {
		case m.starting:
			state = "  ·  starting…"
		case m.player != nil && m.player.Paused():
			state = "  ·  paused"
		case m.player == nil:
			state = "  ·  stopped"
		}
		c.Text(a.F.Small, v.trackR, a.queueLine()+state, kobo.Dark, kobo.AlignLeft)
	}
	for i, r := range v.buttons {
		a.drawButton(i, r)
	}
	a.drawProgress()
	a.drawChips()
	a.drawMusicStatus()
}

// drawChips draws shuffle, repeat and "+ playlist", under the buttons.
func (a *bro) drawChips() {
	m, v, c := &a.mus, &a.mv, a.C
	c.Fill(v.chipRow, kobo.White)
	labels := [numChips]string{"shuffle", "repeat " + repeatNames[m.repeat], "+ playlist"}
	on := [numChips]bool{m.shuffle, m.repeat != repeatOff, false}
	x := v.chipRow.Min.X
	for i, l := range labels {
		w := kobo.TextWidth(a.F.Small, "repeat all") + 40 // all the same, so they don't jump
		if i == chipAdd {
			w = kobo.TextWidth(a.F.Small, l) + 40
		}
		r := image.Rect(x, v.chipRow.Min.Y, x+w, v.chipRow.Max.Y)
		x += w + 12
		v.chips[i] = r
		fg := uint8(kobo.Black)
		if on[i] {
			c.Fill(r, kobo.Black)
			fg = kobo.White
		} else {
			c.Frame(r, 2, kobo.Black)
		}
		if i == chipAdd && m.q == nil {
			fg = kobo.Mid
		}
		c.Text(a.F.Small, r, l, fg, kobo.AlignCenter)
	}
	a.Mark(v.chipRow, true)
}

func (a *bro) drawCover() {
	r := a.mv.cover
	c := a.C
	c.Fill(r, kobo.White)
	var img *image.Gray
	if path := a.playingCover(); path != "" {
		img = a.mus.covers[path]
		if img == nil {
			a.loadCover(path)
		}
	}
	if img != nil {
		drawImage(c, r, img)
	} else {
		c.Fill(r, kobo.Light)
		c.Text(a.F.Huge, r, "♪", kobo.Mid, kobo.AlignCenter)
	}
	c.Frame(r, 2, kobo.Black)
	a.Mark(r, true)
}

func drawImage(c kobo.Canvas, r image.Rectangle, img *image.Gray) {
	for y := 0; y < r.Dy() && y < img.Bounds().Dy(); y++ {
		copy(c.Pix[c.PixOffset(r.Min.X, r.Min.Y+y):], img.Pix[img.PixOffset(0, y):img.PixOffset(min(r.Dx(), img.Bounds().Dx()), y)])
	}
}

// loadCover decodes a cover in the background (a big JPEG takes a moment).
func (a *bro) loadCover(path string) {
	m := &a.mus
	if m.loading[path] {
		return
	}
	if m.loading == nil {
		m.loading = map[string]bool{}
	}
	m.loading[path] = true
	side := a.mv.coverSide
	go func() {
		start := time.Now()
		img, err := loadCover(path, side)
		a.Do(func() {
			delete(m.loading, path)
			if err != nil {
				log.Printf("music: cover %s: %v", path, err)
				img = kobo.Blank(image.Pt(side, side)) // don't try again
			} else {
				log.Printf("music: cover %s in %v", filepath.Base(path), time.Since(start).Round(time.Millisecond))
			}
			if side != a.mv.coverSide {
				return
			}
			if m.covers == nil {
				m.covers = map[string]*image.Gray{}
			}
			m.covers[path] = img
			if a.showingMusic() && a.playingCover() == path {
				a.drawCover()
			}
			a.redoze()
		})
	}()
}

func (a *bro) drawButton(i int, r image.Rectangle) {
	c := a.C
	fg, bg := uint8(kobo.Black), uint8(kobo.White)
	if i == btnPlay {
		fg, bg = kobo.White, kobo.Black
	}
	c.Fill(r, bg)
	c.Frame(r, 2, kobo.Black)
	s := min(r.Dx(), r.Dy()) * 2 / 5
	cx, cy := r.Min.X+r.Dx()/2, r.Min.Y+r.Dy()/2
	h := s / 2
	switch i {
	case btnPlay:
		if p := a.mus.player; p != nil && !p.Paused() || a.mus.starting {
			bar := max(4, s/3)
			c.Fill(image.Rect(cx-h, cy-h, cx-h+bar, cy+h), fg)
			c.Fill(image.Rect(cx+h-bar, cy-h, cx+h, cy+h), fg)
		} else {
			triangle(c, image.Pt(cx-h+3, cy-h), image.Pt(cx-h+3, cy+h), image.Pt(cx+h+3, cy), fg)
		}
	case btnNext:
		triangle(c, image.Pt(cx-h, cy-h), image.Pt(cx-h, cy+h), image.Pt(cx+h/2, cy), fg)
		c.Fill(image.Rect(cx+h/2, cy-h, cx+h/2+max(3, s/8), cy+h), fg)
	case btnPrev:
		triangle(c, image.Pt(cx+h, cy-h), image.Pt(cx+h, cy+h), image.Pt(cx-h/2, cy), fg)
		c.Fill(image.Rect(cx-h/2-max(3, s/8), cy-h, cx-h/2, cy+h), fg)
	default:
		c.Text(a.F.Small, r, buttonLabels[i], fg, kobo.AlignCenter)
	}
	a.Mark(r, true)
}

// triangle fills the triangle p, q, s.
func triangle(c kobo.Canvas, p, q, s image.Point, g uint8) {
	b := image.Rectangle{p, p.Add(image.Pt(1, 1))}.Union(image.Rectangle{q, q.Add(image.Pt(1, 1))}).Union(image.Rectangle{s, s.Add(image.Pt(1, 1))})
	edge := func(a, b, x image.Point) int { return (b.X-a.X)*(x.Y-a.Y) - (b.Y-a.Y)*(x.X-a.X) }
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			pt := image.Pt(x, y)
			e1, e2, e3 := edge(p, q, pt), edge(q, s, pt), edge(s, p, pt)
			if (e1 >= 0 && e2 >= 0 && e3 >= 0) || (e1 <= 0 && e2 <= 0 && e3 <= 0) {
				c.SetGray(x, y, color.Gray{g})
			}
		}
	}
}

// drawProgress draws the times and the bar.
func (a *bro) drawProgress() {
	m, v, c := &a.mus, &a.mv, a.C
	m.shownSec = int(m.pos.Seconds())
	c.Fill(v.times, kobo.White)
	c.Fill(v.bar, kobo.White)
	if m.q != nil {
		c.Text(a.F.Small, v.times, audio.Clock(m.pos), kobo.Black, kobo.AlignLeft)
		c.Text(a.F.Small, v.times, audio.Clock(m.length), kobo.Dark, kobo.AlignRight)
	}
	c.Frame(v.bar, 2, kobo.Black)
	if m.length > 0 {
		in := v.bar.Inset(5)
		w := int(float64(in.Dx()) * min(1, float64(m.pos)/float64(m.length)))
		c.Fill(image.Rect(in.Min.X, in.Min.Y, in.Min.X+w, in.Max.Y), kobo.Black)
	}
	a.Mark(v.times.Union(v.bar), true)
}

func (a *bro) drawMusicStatus() {
	m, v := &a.mus, &a.mv
	a.C.Fill(v.statusR, kobo.White)
	s := fmt.Sprintf("volume %d%%", m.gain)
	if m.note != "" {
		s += "   ·   " + m.note
	}
	a.C.Text(a.F.Small, v.statusR, s, kobo.Dark, kobo.AlignLeft)
	a.Mark(v.statusR, true)
}

// nowPlaying is what's playing, for the header and the status bar.
func (a *bro) nowPlaying() string {
	m := &a.mus
	name := trackName(filepath.Base(a.playing()))
	switch {
	case m.q == nil || m.player == nil && !m.starting:
		return ""
	case m.player != nil && m.player.Paused():
		return "♪ " + name + " (paused)"
	}
	return "♪ " + name
}

// musicHeader shows what's playing, on every tab.
func (a *bro) musicHeader() {
	a.SetStatus(a.nowPlaying())
	a.drawStat(false)
	a.redoze()
}

// --- playing ---

// playQueue plays q from its track start (an index in its tracks), or
// with start < 0 from its first (shuffled: any).
func (a *bro) playQueue(q *queue, start int) {
	m := &a.mus
	if q == nil {
		return
	}
	if q.len() == 0 {
		m.note = "nothing to play in " + q.name
		a.redrawMusic()
		return
	}
	if start >= 0 {
		q.jump(start)
	}
	if m.shuffle {
		q.shuffle(start >= 0)
	}
	m.q = q
	log.Printf("music: playing %s %s (%d tracks, shuffle %v, repeat %s)", q.kind, q.src, q.len(), m.shuffle, repeatNames[m.repeat])
	a.musicPlay(0)
}

// musicPlay plays the queue's track on, from at.
func (a *bro) musicPlay(at time.Duration) {
	m := &a.mus
	path := a.playing()
	if path == "" {
		return
	}
	a.musicStop()
	m.pos, m.length, m.note, m.starting = at, 0, "", true
	m.gen++
	gen, bt := m.gen, m.bt
	a.followPlaying()
	go func() {
		if _, ok := bt.Sink(); !ok {
			bt = audio.Probe() // a device may have come since
		}
		src, err := audio.Open(path)
		a.Do(func() {
			if bt.Bus != m.bt.Bus {
				a.setBluetooth(bt)
			}
			if gen != m.gen {
				if src != nil {
					src.Close()
				}
				return
			}
			m.starting = false
			pcm, ok := bt.Sink()
			switch {
			case err != nil:
				m.note = err.Error()
			case !ok:
				src.Close()
				m.note = "no Bluetooth audio: connect headphones in Kobo's Bluetooth settings"
			default:
				a.musicStart(pcm, src, at, path)
			}
			if m.note != "" {
				log.Printf("music: %s: %s", path, m.note)
			}
			a.musicHeader()
			a.redrawMusic()
		})
	}()
	a.musicHeader()
	a.redrawMusic()
}

func (a *bro) musicStart(pcm audio.PCM, src audio.Source, at time.Duration, path string) {
	m := &a.mus
	var p *audio.Player
	p = audio.Play(m.bt.Bus, pcm, src, m.gain, func(pos time.Duration) {
		a.Do(func() {
			if m.player == p {
				m.pos = pos
				if a.showingMusic() && int(pos.Seconds()) != m.shownSec && !a.Blitting() {
					a.drawProgress()
				}
			}
		})
	}, func(how string) {
		a.Do(func() { a.musicEnded(p, how) })
	})
	m.player, m.length = p, p.Length
	if at > 0 {
		p.SeekTo(at)
	}
	log.Printf("music: playing %s (%s) from %s", path, audio.Clock(p.Length), audio.Clock(at))
	a.saveMusicState()
}

func (a *bro) musicEnded(p *audio.Player, how string) {
	m := &a.mus
	if m.player != p {
		return // stopped by us, and something else is on by now
	}
	m.player = nil
	log.Printf("music: track ended: %s", how)
	switch how {
	case "finished":
		if m.q.step(1, m.repeat, m.shuffle) {
			a.musicPlay(0)
			return
		}
		m.pos, m.note = 0, "end of the "+m.q.kind
		if m.q.kind != qAlbum {
			m.note += " " + m.q.name
		}
	default:
		m.note = "stopped: " + how
		a.setBluetooth(audio.Report{}) // look again next time: the device may have gone
	}
	a.saveMusicState()
	a.musicHeader()
	a.redrawMusic()
}

// musicStop stops what's playing, keeping where it was.
func (a *bro) musicStop() {
	m := &a.mus
	m.gen++
	m.starting = false
	if m.player != nil {
		m.pos = m.player.Pos()
		m.player.Stop()
		m.player = nil
	}
}

// musicToggle pauses, resumes, or starts: where it was, or what's selected.
func (a *bro) musicToggle() {
	m := &a.mus
	switch {
	case m.starting:
	case m.player != nil:
		m.player.SetPaused(!m.player.Paused())
		m.pos = m.player.Pos()
		log.Printf("music: paused %v at %s", m.player.Paused(), audio.Clock(m.pos))
		a.saveMusicState()
		if a.showingMusic() {
			a.drawNow()
		}
		a.musicHeader()
	case m.q != nil:
		a.musicPlay(m.pos)
	default: // the row selected, or the place it's in
		a.playSelected()
	}
}

// musicSkip goes d tracks on; back from a few seconds in restarts the
// track. By hand, "repeat one" goes on to the next all the same.
func (a *bro) musicSkip(d int) {
	m := &a.mus
	if m.q == nil {
		return
	}
	if d < 0 && m.pos > 3*time.Second {
		a.musicPlay(0)
		return
	}
	rep := m.repeat
	if rep == repeatOne {
		rep = repeatAll
	}
	if !m.q.step(d, rep, m.shuffle) && d > 0 {
		m.note = "that's the last one"
		if a.showingMusic() {
			a.drawMusicStatus()
		}
		return
	}
	a.musicPlay(0)
}

// setShuffle turns shuffle on or off; what's playing plays on.
func (a *bro) setShuffle(on bool) {
	m := &a.mus
	m.shuffle = on
	if m.q != nil {
		if on {
			m.q.shuffle(true)
		} else {
			m.q.unshuffle()
		}
	}
	m.note = map[bool]string{false: "in order", true: "shuffled"}[on]
	log.Printf("music: shuffle %v", on)
	a.saveMusicState()
	if a.showingMusic() {
		a.drawNow()
	}
	a.redoze()
}

// cycleRepeat goes off, all, one, and round.
func (a *bro) cycleRepeat() {
	m := &a.mus
	m.repeat = (m.repeat + 1) % 3
	m.note = map[repeatMode]string{repeatOff: "stops at the end", repeatAll: "goes round again", repeatOne: "plays the track over"}[m.repeat]
	log.Printf("music: repeat %s", repeatNames[m.repeat])
	a.saveMusicState()
	if a.showingMusic() {
		a.drawChips()
		a.drawMusicStatus()
	}
}

func (a *bro) musicSeek(d time.Duration) {
	m := &a.mus
	if m.player == nil {
		return
	}
	m.pos = max(0, min(m.pos+d, m.length))
	m.player.SeekTo(m.pos)
	if a.showingMusic() {
		a.drawProgress()
	}
}

func (a *bro) musicGain(d int) {
	m := &a.mus
	m.gain = max(0, min(gainMax, m.gain+d))
	if m.player != nil {
		m.player.SetGain(m.gain)
	}
	a.saveMusicState()
	if a.showingMusic() {
		a.drawMusicStatus()
	}
}

func (a *bro) musicConnect() {
	m := &a.mus
	d, ok := a.audioDevice()
	if !ok {
		return
	}
	m.connecting = true
	a.drawNow()
	bus := m.bt.Bus
	go func() {
		err := audio.Connect(bus, d)
		if err == nil {
			time.Sleep(2 * time.Second) // bluealsa's PCM comes a moment after
		}
		a.Do(func() {
			m.connecting = false
			m.note = "connected " + d.Name
			if err != nil {
				m.note = fmt.Sprintf("connecting %s: %v", d.Name, err)
			}
			log.Printf("music: %s", m.note)
			a.probeAudio()
			if a.showingMusic() {
				a.drawNow()
			}
		})
	}()
}

func (a *bro) musicIdleTimeout() (time.Duration, bool) {
	if p := a.mus.player; p != nil && !p.Paused() {
		return playingIdle, true
	}
	return 0, false
}

// --- input ---

// Media keys, whichever tab is up: headphones' and keyboards' buttons.
func (a *bro) musicMediaKey(code uint16) {
	log.Printf("music: media key %d", code)
	switch code {
	case kobo.KeyPlayPause, kobo.KeyPlayCD, kobo.KeyPauseCD:
		if a.mus.player != nil && code == kobo.KeyPlayCD && !a.mus.player.Paused() {
			return
		}
		a.musicToggle()
	case kobo.KeyStopCD:
		a.musicStop()
		a.musicHeader()
		a.redrawMusic()
	case kobo.KeyNextSong:
		a.musicSkip(1)
	case kobo.KeyPreviousSong:
		a.musicSkip(-1)
	case kobo.KeyVolumeUp:
		a.musicGain(gainStep)
	case kobo.KeyVolumeDown:
		a.musicGain(-gainStep)
	}
}

const (
	keyHome = 102
	keyUp   = 103
	keyEnd  = 107
	keyDown = 108
)

// musicKey takes a key on the Music tab; false leaves it to the app
// (Esc quits).
func (a *bro) musicKey(code uint16, value int32) bool {
	if value == 0 {
		return true
	}
	shift := a.tv.mods.shift
	once := value == 1 // not held down
	ch, _ := kobo.KeyChar(code, false, false)
	switch {
	case code == kobo.KeySpace || ch == "k":
		if once {
			a.musicToggle()
		}
	case code == keyUp && shift:
		a.libMoveEntry(-1)
	case code == keyDown && shift:
		a.libMoveEntry(1)
	case code == keyUp:
		a.libMove(-1)
	case code == keyDown:
		a.libMove(1)
	case code == keyPgUp:
		a.libMove(-a.mv.rowsOnScreen)
	case code == keyPgDn:
		a.libMove(a.mv.rowsOnScreen)
	case code == keyHome:
		a.libMove(-len(a.mus.list))
	case code == keyEnd:
		a.libMove(len(a.mus.list))
	case code == kobo.KeyEnter && once:
		if shift {
			a.playSelected()
		} else {
			a.libActivate(a.place().sel)
		}
	case (code == kobo.KeyBackspace || code == kobo.KeyEsc) && !a.atTop():
		if once {
			a.libUp()
		}
	case code == kobo.KeyTab || ch == "v":
		if once {
			a.setView((a.mus.view + map[bool]int{false: 1, true: numViews - 1}[shift]) % numViews)
		}
	case ch >= "1" && ch <= "3":
		if once {
			a.setView(int(ch[0] - '1'))
		}
	case code == keyLeft || ch == "j":
		a.musicSeek(-seekStep)
	case code == keyRight || ch == "l":
		a.musicSeek(seekStep)
	case ch == "n" && once:
		a.musicSkip(1)
	case ch == "p" && once:
		a.musicSkip(-1)
	case ch == "s" && once:
		a.musicStop()
		a.musicHeader()
		a.redrawMusic()
	case ch == "r" && once:
		a.setShuffle(!a.mus.shuffle)
	case ch == "t" && once:
		a.cycleRepeat()
	case ch == "a" && once:
		if shift {
			a.addPlace()
		} else {
			a.addSelected()
		}
	case (code == keyDelete || ch == "x") && once:
		a.libDelete()
	case ch == "f" && once:
		a.showPlaying()
	case code == keyEqual:
		a.musicGain(gainStep)
	case code == keyMinus:
		a.musicGain(-gainStep)
	default:
		return false
	}
	return true
}

func (a *bro) musicTap(p image.Point) {
	m, v := &a.mus, &a.mv
	switch {
	case kobo.Hit(p, a.nowPanel().Button):
		a.musicConnect()
		return
	case kobo.Hit(p, a.libPanel().Button):
		a.libUp()
		return
	case p.In(v.cover):
		a.musicToggle()
		return
	case p.In(v.bar.Inset(-12)) && m.player != nil && m.length > 0:
		in := v.bar.Inset(5)
		a.musicSeek(time.Duration(float64(m.length)*float64(p.X-in.Min.X)/float64(max(1, in.Dx()))) - m.pos)
		return
	}
	for i, r := range v.buttons {
		if !kobo.Hit(p, r) {
			continue
		}
		switch i {
		case btnPrev:
			a.musicSkip(-1)
		case btnBack:
			a.musicSeek(-seekStep)
		case btnPlay:
			a.musicToggle()
		case btnFwd:
			a.musicSeek(seekStep)
		case btnNext:
			a.musicSkip(1)
		case btnVolDown:
			a.musicGain(-gainStep)
		case btnVolUp:
			a.musicGain(gainStep)
		}
		return
	}
	for i, r := range v.chips {
		if !kobo.Hit(p, r) {
			continue
		}
		switch i {
		case chipShuffle:
			a.setShuffle(!m.shuffle)
		case chipRepeat:
			a.cycleRepeat()
		case chipAdd:
			a.addPlaying()
		}
		return
	}
	a.libTap(p)
}

// --- remembering ---

// saveMusicState writes music-state. dir and file are the track, as
// hellobro has always written them; queue and src are what it's playing
// from.
func (a *bro) saveMusicState() {
	m := &a.mus
	if !m.scanned {
		return // what's in it now is all there is to say
	}
	var b strings.Builder
	fmt.Fprintf(&b, "gain=%d\nshuffle=%v\nrepeat=%s\nview=%s\n", m.gain, m.shuffle, repeatNames[m.repeat], viewNames[m.view])
	if path := a.playing(); path != "" {
		pos := m.pos
		if m.player != nil {
			pos = m.player.Pos()
		}
		fmt.Fprintf(&b, "queue=%s\nsrc=%s\ndir=%s\nfile=%s\npos=%d\n", m.q.kind, m.q.src, filepath.Dir(path), filepath.Base(path), int(pos.Seconds()))
	}
	if err := os.WriteFile(kobo.AppFile(musicStateTxt), []byte(b.String()), 0o644); err != nil {
		log.Printf("music: %v", err)
	}
}

// readMusicState is what saveMusicState wrote.
func readMusicState() map[string]string {
	kv := map[string]string{}
	b, _ := os.ReadFile(kobo.AppFile(musicStateTxt))
	for _, line := range strings.Split(string(b), "\n") {
		if k, v, ok := strings.Cut(line, "="); ok {
			kv[k] = v
		}
	}
	return kv
}

// loadMusicState finds what played last, once the library is known: the
// track, in what it was playing from (or else its album).
func (a *bro) loadMusicState() {
	m := &a.mus
	kv := readMusicState()
	m.shuffle = kv["shuffle"] == "true"
	m.repeat = repeatMode(indexOf(repeatNames, kv["repeat"]))
	m.view = indexOf(viewNames[:], kv["view"])
	if m.q != nil || kv["dir"] == "" {
		return // something got played meanwhile, or nothing ever was
	}
	path := filepath.Join(kv["dir"], kv["file"])
	kind, src := kv["queue"], kv["src"]
	if kind == "" {
		kind, src = qAlbum, kv["dir"]
	}
	q := a.queueOf(kind, src)
	if q == nil || q.find(path) < 0 {
		q = a.queueOf(qAlbum, kv["dir"])
	}
	if q == nil || q.find(path) < 0 {
		return
	}
	q.jump(q.find(path))
	if m.shuffle {
		q.shuffle(true)
	}
	sec, _ := strconv.Atoi(kv["pos"])
	m.q, m.pos = q, time.Duration(sec)*time.Second
	if al := m.lib.album(kv["dir"]); al != nil { // the Albums view opens on it
		m.places[viewAlbums] = place{at: al.dir}
	}
	m.list = nil
	a.followPlaying()
	m.note = "Space or ▶ goes on from here"
}
