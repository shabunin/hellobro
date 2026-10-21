package main

// The Music tab: albums from the Kobo's storage, played over Bluetooth to
// headphones or a DAC connected in Kobo's settings. Music goes on while
// the terminals are up; the header says what's playing. Where it was
// (album, track, position, volume) is kept in music-state, so it picks up
// there next time.

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

// musicView is where the Music tab's things are.
type musicView struct {
	nowP, libP              kobo.Panel
	cover                   image.Rectangle
	albumR, titleR, trackR  image.Rectangle
	times, bar, statusR     image.Rectangle
	buttons                 [numButtons]image.Rectangle
	coverSide, rowsOnScreen int
}

type musicState struct {
	albums            []album
	scanned, scanning bool
	thens             []func()          // to run once scanned
	open              int               // the album listed, or -1 for the list of albums
	sel, top          int               // the selected row, and the first one on screen
	rows              []image.Rectangle // the rows on screen, for taps

	bt                 audio.Report
	btBusy, connecting bool
	player             *audio.Player
	gen                int // bumps per track started, so a late start is dropped
	album, track       int // playing, paused, or played last; -1: none
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
		side := max(100, min(b.Dx(), b.Dy()-360))
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
	v.statusR = next(36)
	const gap = 12
	bw := min(130, (row.Dx()-gap*(numButtons-1))/numButtons)
	x := row.Min.X
	for i := range v.buttons {
		v.buttons[i] = image.Rect(x, row.Min.Y, x+bw, row.Max.Y)
		x += bw + gap
	}
	v.rowsOnScreen = max(1, (v.libP.Body.Dy()-2*kobo.Pad)/rowH)
}

// --- the library ---

func (a *bro) enterMusic() {
	a.scanMusic(nil)
	if a.mus.bt.Lines == nil {
		a.probeAudio()
	}
}

// scanMusic finds the albums, once; then runs when they're known.
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
		albums := scanLibrary(musicRoot)
		a.Do(func() {
			m.albums, m.scanned, m.scanning = albums, true, false
			n := 0
			for _, al := range albums {
				n += len(al.tracks)
			}
			log.Printf("music: %d albums, %d tracks under %s", len(albums), n, musicRoot)
			a.loadMusicState()
			a.redrawMusic()
			for _, f := range m.thens {
				f()
			}
			m.thens = nil
		})
	}()
}

// rescanMusic finds the albums again (a computer sent some), keeping
// what's playing and what's open where they are.
func (a *bro) rescanMusic() {
	m := &a.mus
	if !m.scanned || m.scanning {
		return // a scan to come sees them anyway
	}
	m.scanning = true
	go func() {
		albums := scanLibrary(musicRoot)
		a.Do(func() {
			m.scanning = false
			dirOf := func(i int) string {
				if i >= 0 && i < len(m.albums) {
					return m.albums[i].dir
				}
				return ""
			}
			find := func(dir string) int {
				for i, al := range albums {
					if dir != "" && al.dir == dir {
						return i
					}
				}
				return -1
			}
			playing, open, sel := dirOf(m.album), dirOf(m.open), dirOf(m.sel)
			was := len(m.albums)
			m.albums = albums
			m.album, m.open = find(playing), find(open)
			if m.open < 0 {
				m.sel = max(0, find(sel))
			}
			if m.album < 0 && m.player != nil {
				a.musicStop() // its album went
				a.musicHeader()
			}
			log.Printf("music: rescanned: %d albums (%+d)", len(albums), len(albums)-was)
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

func (a *bro) libPanel() kobo.Panel {
	if a.mus.open >= 0 {
		return kobo.WithButton(a.F, a.mv.libP, "albums")
	}
	return a.mv.libP
}

// libLen is how many rows the library has.
func (a *bro) libLen() int {
	if m := &a.mus; m.open >= 0 {
		return len(m.albums[m.open].tracks)
	}
	return len(a.mus.albums)
}

func (a *bro) openAlbum(i int) {
	m := &a.mus
	m.open, m.sel, m.top = i, 0, 0
	if m.album == i && m.track >= 0 {
		m.sel = m.track
	}
	a.drawLibrary()
}

func (a *bro) closeAlbum() {
	m := &a.mus
	m.sel, m.open = m.open, -1
	a.drawLibrary()
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

// drawNow draws the Now playing panel.
func (a *bro) drawNow() {
	m, v := &a.mus, &a.mv
	a.DrawPanel(a.nowPanel(), a.outputStatus())
	a.drawCover()

	c := a.C
	if m.album < 0 {
		c.Text(a.F.Body, v.albumR, "nothing playing", kobo.Mid, kobo.AlignLeft)
		hint := "Pick an album below."
		if !m.scanned {
			hint = "Looking for music…"
		}
		c.Text(a.F.Small, v.trackR, hint, kobo.Dark, kobo.AlignLeft)
	} else {
		al := m.albums[m.album]
		c.Text(a.F.Body, v.albumR, al.name, kobo.Dark, kobo.AlignLeft)
		c.Text(a.F.Title, v.titleR, trackName(al.tracks[m.track]), kobo.Black, kobo.AlignLeft)
		state := ""
		switch {
		case m.starting:
			state = "  ·  starting…"
		case m.player != nil && m.player.Paused():
			state = "  ·  paused"
		case m.player == nil:
			state = "  ·  stopped"
		}
		c.Text(a.F.Small, v.trackR, fmt.Sprintf("track %d of %d%s", m.track+1, len(al.tracks), state), kobo.Dark, kobo.AlignLeft)
	}
	for i, r := range v.buttons {
		a.drawButton(i, r)
	}
	a.drawProgress()
	a.drawMusicStatus()
}

func (a *bro) drawCover() {
	m, r := &a.mus, a.mv.cover
	c := a.C
	c.Fill(r, kobo.White)
	var img *image.Gray
	if m.album >= 0 {
		if path := m.albums[m.album].cover; path != "" {
			img = m.covers[path]
			if img == nil {
				a.loadCover(path)
			}
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
			if a.showingMusic() && m.album >= 0 && m.albums[m.album].cover == path {
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
	if m.album >= 0 {
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

// drawLibrary draws the list: the albums, or the open album's tracks.
func (a *bro) drawLibrary() {
	if !a.showingMusic() {
		return
	}
	m, v, c := &a.mus, &a.mv, a.C
	p := a.libPanel()
	n := a.libLen()
	m.sel = max(0, min(m.sel, n-1))
	rows := v.rowsOnScreen
	if m.sel < m.top {
		m.top = m.sel
	}
	if m.sel >= m.top+rows {
		m.top = m.sel - rows + 1
	}
	m.top = max(0, min(m.top, n-rows))

	status := ""
	switch {
	case m.scanning || !m.scanned:
		status = "looking…"
	case m.open >= 0:
		status = m.albums[m.open].name
	default:
		tracks := 0
		for _, al := range m.albums {
			tracks += len(al.tracks)
		}
		status = fmt.Sprintf("%d albums · %d tracks", len(m.albums), tracks)
	}
	a.DrawPanel(p, status)
	if m.scanned && len(m.albums) == 0 {
		a.DrawText(p, a.F.Body, []string{
			"No MP3 or WAV files under " + musicRoot + ".",
			"Copy some there, a folder per album, with its cover (cover.jpg),",
			"and come back.",
		}, "", true)
		return
	}

	in := p.Body.Inset(kobo.Pad)
	m.rows = m.rows[:0]
	for i := m.top; i < min(n, m.top+rows); i++ {
		r := image.Rect(in.Min.X, in.Min.Y+(i-m.top)*rowH, in.Max.X-14, in.Min.Y+(i-m.top+1)*rowH)
		m.rows = append(m.rows, r)
		fg, dim := uint8(kobo.Black), uint8(kobo.Dark)
		if i == m.sel {
			c.Fill(r, kobo.Black)
			fg, dim = kobo.White, kobo.Light
		}
		text := r.Inset(4)
		text.Min.X += 44
		if m.open < 0 {
			al := m.albums[i]
			count := fmt.Sprintf("%d tracks", len(al.tracks))
			cw := kobo.TextWidth(a.F.Small, count)
			c.Text(a.F.Small, image.Rect(text.Max.X-cw, text.Min.Y, text.Max.X, text.Max.Y), count, dim, kobo.AlignRight)
			text.Max.X -= cw + 24
			c.Text(a.F.Body, text, al.name, fg, kobo.AlignLeft)
			if i == m.album {
				a.playingMark(r, fg)
			}
			continue
		}
		c.Text(a.F.Body, text, trackName(m.albums[m.open].tracks[i]), fg, kobo.AlignLeft)
		if m.open == m.album && i == m.track {
			a.playingMark(r, fg)
		}
	}
	if n > rows { // where the screenful is in the list
		track := image.Rect(in.Max.X-6, in.Min.Y, in.Max.X, in.Min.Y+rows*rowH)
		c.Fill(track, kobo.Light)
		h := max(24, track.Dy()*rows/n)
		y := track.Min.Y + (track.Dy()-h)*m.top/max(1, n-rows)
		c.Fill(image.Rect(track.Min.X, y, track.Max.X, y+h), kobo.Dark)
	}
	a.Mark(p.Body, true)
}

// playingMark is the ▶ at the start of the row that's playing.
func (a *bro) playingMark(r image.Rectangle, g uint8) {
	cx, cy := r.Min.X+20, r.Min.Y+r.Dy()/2
	triangle(a.C, image.Pt(cx-8, cy-10), image.Pt(cx-8, cy+10), image.Pt(cx+10, cy), g)
}

// nowPlaying is what's playing, for the header and the status bar.
func (a *bro) nowPlaying() string {
	m := &a.mus
	switch {
	case m.album < 0 || m.player == nil && !m.starting:
		return ""
	case m.player != nil && m.player.Paused():
		return "♪ " + trackName(m.albums[m.album].tracks[m.track]) + " (paused)"
	}
	return "♪ " + trackName(m.albums[m.album].tracks[m.track])
}

// musicHeader shows what's playing, on every tab.
func (a *bro) musicHeader() {
	a.SetStatus(a.nowPlaying())
	a.drawStat(false)
	a.redoze()
}

// --- playing ---

// musicPlay plays track tr of album al, from at.
func (a *bro) musicPlay(al, tr int, at time.Duration) {
	m := &a.mus
	if al < 0 || al >= len(m.albums) || tr < 0 || tr >= len(m.albums[al].tracks) {
		return
	}
	a.musicStop()
	m.album, m.track, m.pos, m.length, m.note, m.starting = al, tr, at, 0, "", true
	m.gen++
	gen, bt := m.gen, m.bt
	path := filepath.Join(m.albums[al].dir, m.albums[al].tracks[tr])
	if m.open == al {
		m.sel = tr
	}
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
		if m.track+1 < len(m.albums[m.album].tracks) {
			a.musicPlay(m.album, m.track+1, 0)
			return
		}
		m.pos, m.note = 0, "end of the album"
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
	case m.album >= 0:
		a.musicPlay(m.album, m.track, m.pos)
	case m.open >= 0:
		a.musicPlay(m.open, m.sel, 0)
	case len(m.albums) > 0:
		a.musicPlay(m.sel, 0, 0)
	}
}

// musicSkip goes d tracks on; back from a few seconds in restarts the track.
func (a *bro) musicSkip(d int) {
	m := &a.mus
	if m.album < 0 {
		return
	}
	if d < 0 && m.pos > 3*time.Second {
		d = 0
	}
	a.musicPlay(m.album, max(0, min(m.track+d, len(m.albums[m.album].tracks)-1)), 0)
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
	m := &a.mus
	move := func(d int) {
		m.sel = max(0, min(m.sel+d, a.libLen()-1))
		a.drawLibrary()
	}
	ch, _ := kobo.KeyChar(code, false, false)
	switch {
	case code == kobo.KeySpace || ch == "k":
		if value == 1 {
			a.musicToggle()
		}
	case code == keyUp:
		move(-1)
	case code == keyDown:
		move(1)
	case code == keyPgUp:
		move(-a.mv.rowsOnScreen)
	case code == keyPgDn:
		move(a.mv.rowsOnScreen)
	case code == keyHome:
		move(-a.libLen())
	case code == keyEnd:
		move(a.libLen())
	case code == kobo.KeyEnter && value == 1:
		if m.open < 0 && len(m.albums) > 0 {
			a.openAlbum(m.sel)
		} else if m.open >= 0 {
			a.musicPlay(m.open, m.sel, 0)
		}
	case (code == kobo.KeyBackspace || code == kobo.KeyEsc) && m.open >= 0:
		if value == 1 {
			a.closeAlbum()
		}
	case code == keyLeft || ch == "j":
		a.musicSeek(-seekStep)
	case code == keyRight || ch == "l":
		a.musicSeek(seekStep)
	case ch == "n" && value == 1:
		a.musicSkip(1)
	case ch == "p" && value == 1:
		a.musicSkip(-1)
	case ch == "s" && value == 1:
		a.musicStop()
		a.musicHeader()
		a.redrawMusic()
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
		a.closeAlbum()
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
	for i, r := range m.rows {
		if !p.In(r) {
			continue
		}
		m.sel = m.top + i
		if m.open < 0 {
			a.openAlbum(m.sel)
		} else {
			a.musicPlay(m.open, m.sel, 0)
		}
		return
	}
}

// musicSwipe scrolls the library: a row per row the finger moved.
func (a *bro) musicSwipe(from, to image.Point) {
	m := &a.mus
	dy := to.Y - from.Y
	if !from.In(a.mv.libP.Body) || kobo.Abs(dy) < rowH || kobo.Abs(dy) < 2*kobo.Abs(to.X-from.X) {
		return
	}
	d := -dy / rowH
	m.top = max(0, min(m.top+d, a.libLen()-a.mv.rowsOnScreen))
	m.sel = max(m.top, min(m.sel+d, m.top+a.mv.rowsOnScreen-1))
	a.drawLibrary()
}

// --- remembering ---

func (a *bro) saveMusicState() {
	m := &a.mus
	var b strings.Builder
	fmt.Fprintf(&b, "gain=%d\n", m.gain)
	if m.album >= 0 {
		pos := m.pos
		if m.player != nil {
			pos = m.player.Pos()
		}
		fmt.Fprintf(&b, "dir=%s\nfile=%s\npos=%d\n", m.albums[m.album].dir, m.albums[m.album].tracks[m.track], int(pos.Seconds()))
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

// loadMusicState finds the track played last, once the library is known.
func (a *bro) loadMusicState() {
	m := &a.mus
	kv := readMusicState()
	if m.album >= 0 {
		return // something got played meanwhile
	}
	for i, al := range m.albums {
		if al.dir != kv["dir"] {
			continue
		}
		for j, t := range al.tracks {
			if t == kv["file"] {
				sec, _ := strconv.Atoi(kv["pos"])
				m.album, m.track, m.pos = i, j, time.Duration(sec)*time.Second
				m.open, m.sel = i, j
				m.note = "Space or ▶ goes on from here"
				return
			}
		}
	}
}
