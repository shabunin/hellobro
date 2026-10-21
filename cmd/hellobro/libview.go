package main

// The Music tab's library panel, three ways round: Albums (every folder
// with tracks in it), Folders (the folders as they are, each played with
// all its subfolders) and Playlists. A tap (or Enter) opens an album, a
// folder or a playlist, and plays a track: the rest of what it's in
// follows. The bar under the rows plays what's open, in order or
// shuffled, adds it to a playlist, and edits a playlist.

import (
	"fmt"
	"image"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"hellobro/kobo"
)

const (
	viewAlbums = iota
	viewFolders
	viewPlaylists
	numViews
)

var (
	viewNames  = [numViews]string{"albums", "folders", "playlists"}
	viewLabels = [numViews]string{"Albums", "Folders", "Playlists"}
)

const (
	rowAlbum = iota
	rowFolder
	rowPlaylist
	rowTrack
	rowNew // "new playlist"
)

type libRow struct {
	kind          int
	title, detail string
	path          string // the album's or folder's path, the playlist file, the track
	idx           int    // a playlist's entry: its index
	missing       bool   // a playlist's entry that isn't there
}

// place is where a view is: what's open in it ("" for the top), and the
// row selected.
type place struct {
	at       string
	sel, top int
}

// libAct is a button of the action bar.
type libAct struct {
	label string
	do    func()
	r     image.Rectangle
}

func (a *bro) layoutLibrary() {
	v := &a.mv
	if v.libP.Folded { // nothing in it to draw or tap
		v.segs, v.actR, v.listR, v.rowsOnScreen = [numViews]image.Rectangle{}, image.Rectangle{}, image.Rectangle{}, 1
		return
	}
	b := v.libP.Body.Inset(kobo.Pad)
	const segH, gap, barH = 56, 12, 68
	w := min(260, (b.Dx()-2*gap)/numViews)
	for i := range v.segs {
		x := b.Min.X + i*(w+gap)
		v.segs[i] = image.Rect(x, b.Min.Y, x+w, b.Min.Y+segH)
	}
	v.actR = image.Rect(b.Min.X, b.Max.Y-barH, b.Max.X, b.Max.Y)
	v.listR = image.Rect(b.Min.X, b.Min.Y+segH+gap, b.Max.X, v.actR.Min.Y-gap)
	v.rowsOnScreen = max(1, v.listR.Dy()/rowH)
}

func (a *bro) place() *place { return &a.mus.places[a.mus.view] }

// atTop says whether the view is at its top: nothing to go back up from.
func (a *bro) atTop() bool {
	at := a.place().at
	return at == "" || a.mus.view == viewFolders && at == a.mus.lib.top
}

func (a *bro) libPanel() kobo.Panel {
	p := focusMark(a.mv.libP, !a.mv.nowFocus, !a.mv.nowP.Folded)
	if !a.atTop() {
		return kobo.WithButton(a.F, p, "back")
	}
	return p
}

// --- the rows ---

// makeList makes the rows of the place shown; a place that's gone (a
// rescan, a deleted playlist) goes back to the top.
func (a *bro) makeList() []libRow {
	m, pl := &a.mus, a.place()
	lib := m.lib
	var rows []libRow
	tracks := func(al *album) {
		for i, t := range al.tracks {
			rows = append(rows, libRow{kind: rowTrack, title: trackName(t), path: filepath.Join(al.dir, t), idx: i})
		}
	}
	switch m.view {
	case viewAlbums:
		if pl.at != "" {
			if al := lib.album(pl.at); al != nil {
				tracks(al)
				break
			}
			pl.at = ""
		}
		for _, al := range lib.albums {
			rows = append(rows, libRow{kind: rowAlbum, title: al.name, detail: count(len(al.tracks), "track"), path: al.dir})
		}
	case viewFolders:
		if pl.at == "" || !under(pl.at, lib.top) || lib.folderCount(pl.at) == 0 {
			pl.at = lib.top
		}
		for _, sub := range lib.subfolders(pl.at) {
			rows = append(rows, libRow{kind: rowFolder, title: filepath.Base(sub), detail: count(lib.folderCount(sub), "track"), path: sub})
		}
		if al := lib.album(pl.at); al != nil {
			tracks(al)
		}
	case viewPlaylists:
		if pl.at != "" {
			if p := lib.playlist(pl.at); p != nil {
				for i, e := range p.entries {
					r := libRow{kind: rowTrack, title: trackName(filepath.Base(e)), detail: filepath.Base(filepath.Dir(e)), path: e, idx: i, missing: p.missing[i]}
					if r.missing {
						r.detail = "missing"
					}
					rows = append(rows, r)
				}
				break
			}
			pl.at = ""
		}
		for _, p := range lib.playlists {
			rows = append(rows, libRow{kind: rowPlaylist, title: p.name, detail: count(p.count(), "track"), path: p.path})
		}
		rows = append(rows, libRow{kind: rowNew, title: "+ new playlist"})
	}
	return rows
}

func count(n int, what string) string {
	if n == 1 {
		return "1 " + what
	}
	return fmt.Sprintf("%d %ss", n, what)
}

// relist makes the rows anew, keeping the selection on the same thing.
func (a *bro) relist() {
	m, pl := &a.mus, a.place()
	was := ""
	if pl.sel >= 0 && pl.sel < len(m.list) {
		was = m.list[pl.sel].path
	}
	m.list = a.makeList()
	if i := a.rowOf(was); i >= 0 {
		pl.sel = i
	}
}

// rowOf is the row of path, or -1.
func (a *bro) rowOf(path string) int {
	if path == "" {
		return -1
	}
	return slices.IndexFunc(a.mus.list, func(r libRow) bool { return r.path == path })
}

func (a *bro) selected() (libRow, bool) {
	m, pl := &a.mus, a.place()
	if pl.sel >= 0 && pl.sel < len(m.list) {
		return m.list[pl.sel], true
	}
	return libRow{}, false
}

// followPlaying selects the track playing, if it's in the rows.
func (a *bro) followPlaying() {
	m := &a.mus
	if m.list == nil {
		m.list = a.makeList()
	}
	path := a.playing()
	for i, r := range m.list {
		if r.kind == rowTrack && r.path == path {
			a.place().sel = i
			return
		}
	}
}

func (a *bro) setView(i int) {
	m := &a.mus
	if i == m.view {
		return
	}
	m.view = i
	m.list = a.makeList()
	a.saveMusicState()
	a.drawLibrary()
}

// libEnter opens the album, folder or playlist of row r.
func (a *bro) libEnter(r libRow) {
	m, pl := &a.mus, a.place()
	pl.at, pl.sel, pl.top = r.path, 0, 0
	m.list = a.makeList()
	a.followPlaying()
	a.drawLibrary()
}

// libUp goes back up, to the row it came from.
func (a *bro) libUp() {
	m, pl := &a.mus, a.place()
	if a.atTop() {
		return
	}
	left := pl.at
	if m.view == viewFolders {
		pl.at = filepath.Dir(pl.at)
	} else {
		pl.at = ""
	}
	pl.sel, pl.top = 0, 0
	m.list = a.makeList()
	pl.sel = max(0, a.rowOf(left))
	a.drawLibrary()
}

// showPlaying opens the album playing, on its track.
func (a *bro) showPlaying() {
	m := &a.mus
	al := m.lib.album(filepath.Dir(a.playing()))
	if al == nil {
		return
	}
	m.view = viewAlbums
	m.places[viewAlbums] = place{at: al.dir}
	m.list = a.makeList()
	a.followPlaying()
	a.drawLibrary()
}

func (a *bro) libMove(d int) {
	pl := a.place()
	pl.sel = max(0, min(pl.sel+d, len(a.mus.list)-1))
	a.drawLibrary()
}

// libActivate is a tap on row i, or Enter: open it, or play it.
func (a *bro) libActivate(i int) {
	m := &a.mus
	if i < 0 || i >= len(m.list) {
		return
	}
	a.place().sel = i
	r := m.list[i]
	switch r.kind {
	case rowAlbum, rowFolder, rowPlaylist:
		a.libEnter(r)
	case rowNew:
		a.askNewPlaylist()
	case rowTrack:
		if r.missing {
			m.note = "that one isn't there any more"
			a.drawMusicStatus()
			return
		}
		q, start := a.rowQueue(r)
		a.playQueue(q, start)
	}
}

// --- what to play ---

func (a *bro) albumQueue(dir string) *queue {
	al := a.mus.lib.album(dir)
	if al == nil {
		return nil
	}
	return newQueue(qAlbum, dir, al.name, al.paths())
}

func (a *bro) folderQueue(dir string) *queue {
	lib := a.mus.lib
	if lib.folderCount(dir) == 0 {
		return nil
	}
	name := filepath.Base(dir)
	if dir == lib.top {
		name = "all music"
	}
	return newQueue(qFolder, dir, name, lib.folderTracks(dir))
}

func (a *bro) playlistQueue(path string) *queue {
	pl := a.mus.lib.playlist(path)
	if pl == nil {
		return nil
	}
	return newQueue(qPlaylist, path, pl.name, pl.playable())
}

// queueOf is the queue of kind from src, or nil if it's gone.
func (a *bro) queueOf(kind, src string) *queue {
	switch kind {
	case qAlbum:
		return a.albumQueue(src)
	case qFolder:
		return a.folderQueue(src)
	case qPlaylist:
		return a.playlistQueue(src)
	}
	return nil
}

// placeQueue is what's open, to play: an album, a folder (at the top of
// Albums, all of them), a playlist; nil at the top of Playlists.
func (a *bro) placeQueue() *queue {
	m, pl := &a.mus, a.place()
	switch m.view {
	case viewAlbums:
		if pl.at == "" {
			return a.folderQueue(m.lib.top)
		}
		return a.albumQueue(pl.at)
	case viewFolders:
		return a.folderQueue(pl.at)
	case viewPlaylists:
		if pl.at != "" {
			return a.playlistQueue(pl.at)
		}
	}
	return nil
}

// rowQueue is what row r plays: its album, folder or playlist from the
// start (-1); a track, in what's open, from it.
func (a *bro) rowQueue(r libRow) (*queue, int) {
	switch r.kind {
	case rowAlbum:
		return a.albumQueue(r.path), -1
	case rowFolder:
		return a.folderQueue(r.path), -1
	case rowPlaylist:
		return a.playlistQueue(r.path), -1
	case rowTrack:
		q := a.placeQueue()
		if q == nil || r.missing {
			return nil, -1
		}
		if q.kind != qPlaylist {
			return q, q.find(r.path)
		}
		start := 0 // a playlist can have a track twice: by its place, less the missing before it
		for _, row := range a.mus.list[:r.idx] {
			if !row.missing {
				start++
			}
		}
		return q, start
	}
	return nil, -1
}

// playSelected plays the row selected, whole; with none, what's open.
func (a *bro) playSelected() {
	if r, ok := a.selected(); ok && r.kind != rowNew {
		q, start := a.rowQueue(r)
		a.playQueue(q, start)
		return
	}
	a.playQueue(a.placeQueue(), -1)
}

// playPlace plays what's open, from the start: in order, or shuffled.
func (a *bro) playPlace(shuffle bool) {
	a.mus.shuffle = shuffle
	a.playQueue(a.placeQueue(), -1)
	a.saveMusicState()
}

// --- drawing ---

func (a *bro) libStatus() string {
	m, pl := &a.mus, a.place()
	lib := m.lib
	switch {
	case m.scanning || !m.scanned:
		return "looking…"
	case m.view == viewAlbums && pl.at == "":
		return fmt.Sprintf("%s · %s", count(len(lib.albums), "album"), count(lib.tracks, "track"))
	case m.view == viewAlbums:
		if al := lib.album(pl.at); al != nil {
			return al.name
		}
		return ""
	case m.view == viewFolders:
		rel, _ := filepath.Rel(filepath.Dir(lib.top), pl.at)
		return strings.ReplaceAll(rel, "/", " › ") + " · " + count(lib.folderCount(pl.at), "track")
	case pl.at == "":
		return count(len(lib.playlists), "playlist")
	}
	if p := lib.playlist(pl.at); p != nil {
		return p.name + " · " + count(p.count(), "track")
	}
	return ""
}

// rowPlaying says whether row r is, or has in it, what's playing.
func (a *bro) rowPlaying(r libRow) bool {
	cur := a.playing()
	if cur == "" {
		return false
	}
	switch r.kind {
	case rowTrack:
		return r.path == cur
	case rowAlbum:
		return filepath.Dir(cur) == r.path
	case rowFolder:
		return under(cur, r.path)
	case rowPlaylist:
		return a.mus.q.kind == qPlaylist && a.mus.q.src == r.path
	}
	return false
}

// drawLibrary draws the library panel: the views, the rows, the bar.
func (a *bro) drawLibrary() {
	if !a.showingMusic() {
		return
	}
	m, v, c := &a.mus, &a.mv, a.C
	if m.list == nil {
		a.relist()
	}
	p := a.libPanel()
	a.DrawPanel(p, a.libStatus())
	if p.Folded {
		m.rows, m.acts = nil, nil
		return
	}
	for i, r := range v.segs {
		fg := uint8(kobo.Black)
		if i == m.view {
			c.Fill(r, kobo.Black)
			fg = kobo.White
		} else {
			c.Frame(r, 2, kobo.Black)
		}
		c.Text(a.F.Small, r, viewLabels[i], fg, kobo.AlignCenter)
	}
	a.drawActions()

	pl := a.place()
	n, rows := len(m.list), v.rowsOnScreen
	pl.sel = max(0, min(pl.sel, n-1))
	if pl.sel < pl.top {
		pl.top = pl.sel
	}
	if pl.sel >= pl.top+rows {
		pl.top = pl.sel - rows + 1
	}
	pl.top = max(0, min(pl.top, n-rows))

	in := v.listR
	m.rows = m.rows[:0]
	var empty []string
	switch {
	case !m.scanned:
	case len(m.lib.albums) == 0 && m.view != viewPlaylists:
		empty = []string{
			"No MP3 or WAV files under " + musicRoot + ".",
			"Copy some there, a folder per album, with its cover (cover.jpg),",
			"and come back.",
		}
	case n == 0 && m.view == viewPlaylists:
		empty = []string{
			"Nothing in it yet.",
			"“+ playlist” in Albums or Folders adds what's open there;",
			"a adds the row selected, and “+ playlist” up top what's playing.",
		}
	}
	for i, s := range empty {
		c.Text(a.F.Body, image.Rect(in.Min.X+20, in.Min.Y+i*rowH, in.Max.X, in.Min.Y+(i+1)*rowH), s, kobo.Dark, kobo.AlignLeft)
	}
	for i := pl.top; i < min(n, pl.top+rows); i++ {
		row := m.list[i]
		r := image.Rect(in.Min.X, in.Min.Y+(i-pl.top)*rowH, in.Max.X-14, in.Min.Y+(i-pl.top+1)*rowH)
		m.rows = append(m.rows, r)
		fg, dim := uint8(kobo.Black), uint8(kobo.Dark)
		if row.missing {
			fg = kobo.Mid
		}
		if i == pl.sel {
			c.Fill(r, kobo.Black)
			fg, dim = kobo.White, kobo.Light
		}
		text := r.Inset(4)
		text.Min.X += 44
		detail := row.detail
		if row.kind == rowAlbum || row.kind == rowFolder || row.kind == rowPlaylist {
			detail += "  ›"
		}
		if detail != "" {
			dw := kobo.TextWidth(a.F.Small, detail)
			c.Text(a.F.Small, image.Rect(text.Max.X-dw, text.Min.Y, text.Max.X, text.Max.Y), detail, dim, kobo.AlignRight)
			text.Max.X -= dw + 24
		}
		c.Text(a.F.Body, text, row.title, fg, kobo.AlignLeft)
		if a.rowPlaying(row) {
			a.playingMark(r, fg)
		}
	}
	if n > rows { // where the screenful is in the list
		track := image.Rect(in.Max.X-6, in.Min.Y, in.Max.X, in.Min.Y+rows*rowH)
		c.Fill(track, kobo.Light)
		h := max(24, track.Dy()*rows/n)
		y := track.Min.Y + (track.Dy()-h)*pl.top/max(1, n-rows)
		c.Fill(image.Rect(track.Min.X, y, track.Max.X, y+h), kobo.Dark)
	}
	a.Mark(p.Body, true)
}

// playingMark is the ▶ at the start of the row that's playing.
func (a *bro) playingMark(r image.Rectangle, g uint8) {
	cx, cy := r.Min.X+20, r.Min.Y+r.Dy()/2
	triangle(a.C, image.Pt(cx-8, cy-10), image.Pt(cx-8, cy+10), image.Pt(cx+10, cy), g)
}

// libActions are the bar's buttons, for what's open.
func (a *bro) libActions() []libAct {
	m, pl := &a.mus, a.place()
	switch {
	case !m.scanned:
		return nil
	case m.view == viewPlaylists && pl.at == "":
		acts := []libAct{{label: "new playlist", do: a.askNewPlaylist}}
		if len(m.lib.playlists) > 0 {
			acts = append(acts, libAct{label: "delete one…", do: a.pickDeletePlaylist})
		}
		return acts
	case m.view == viewPlaylists:
		return []libAct{
			{label: "play", do: func() { a.playPlace(false) }},
			{label: "shuffle", do: func() { a.playPlace(true) }},
			{label: "remove", do: a.libDelete},
			{label: "↑", do: func() { a.libMoveEntry(-1) }},
			{label: "↓", do: func() { a.libMoveEntry(1) }},
		}
	case m.lib.tracks == 0:
		return nil
	}
	return []libAct{
		{label: "play", do: func() { a.playPlace(false) }},
		{label: "shuffle", do: func() { a.playPlace(true) }},
		{label: "+ playlist", do: a.addPlace},
	}
}

func (a *bro) drawActions() {
	m, v, c := &a.mus, &a.mv, a.C
	c.Fill(v.actR, kobo.White)
	m.acts = a.libActions()
	x := v.actR.Min.X
	for i := range m.acts {
		act := &m.acts[i]
		w := max(150, kobo.TextWidth(a.F.Small, act.label)+48)
		if len([]rune(act.label)) == 1 {
			w = 96
		}
		act.r = image.Rect(x, v.actR.Min.Y, x+w, v.actR.Max.Y)
		x += w + 12
		c.Frame(act.r, 2, kobo.Black)
		c.Text(a.F.Small, act.r, act.label, kobo.Black, kobo.AlignCenter)
	}
	if q := a.placeQueue(); q != nil && len(m.acts) > 0 {
		what := q.kind + " " + q.name
		if q.kind == qFolder && q.src == m.lib.top {
			what = q.name
		}
		c.Text(a.F.Small, image.Rect(x+12, v.actR.Min.Y, v.actR.Max.X, v.actR.Max.Y), what, kobo.Dark, kobo.AlignRight)
	}
}

// --- input ---

func (a *bro) libTap(p image.Point) {
	m, v := &a.mus, &a.mv
	for i, r := range v.segs {
		if kobo.Hit(p, r) {
			a.setView(i)
			return
		}
	}
	for _, act := range m.acts {
		if kobo.Hit(p, act.r) {
			act.do()
			return
		}
	}
	for i, r := range m.rows {
		if p.In(r) {
			a.libActivate(a.place().top + i)
			return
		}
	}
}

// musicSwipe scrolls the library: a row per row the finger moved.
func (a *bro) musicSwipe(from, to image.Point) {
	pl, rows := a.place(), a.mv.rowsOnScreen
	dy := to.Y - from.Y
	if !from.In(a.mv.listR) || kobo.Abs(dy) < rowH || kobo.Abs(dy) < 2*kobo.Abs(to.X-from.X) {
		return
	}
	d := -dy / rowH
	pl.top = max(0, min(pl.top+d, len(a.mus.list)-rows))
	pl.sel = max(pl.top, min(pl.sel+d, pl.top+rows-1))
	a.drawLibrary()
}

// --- playlists ---

// openPlaylist is the playlist open in the Playlists view, or nil.
func (a *bro) openPlaylist() *playlist {
	if a.mus.view != viewPlaylists || a.place().at == "" {
		return nil
	}
	return a.mus.lib.playlist(a.place().at)
}

// addSelected offers to add the row selected to a playlist.
func (a *bro) addSelected() {
	r, ok := a.selected()
	switch {
	case !ok || r.kind == rowNew || r.missing:
	case r.kind == rowTrack:
		a.pickPlaylist(r.title, []string{r.path})
	default:
		if q, _ := a.rowQueue(r); q != nil {
			a.pickPlaylist(r.title, q.tracks)
		}
	}
}

// addPlace offers to add what's open to a playlist.
func (a *bro) addPlace() {
	if q := a.placeQueue(); q != nil {
		a.pickPlaylist(q.name, q.tracks)
	}
}

// addPlaying offers to add the track playing to a playlist.
func (a *bro) addPlaying() {
	if path := a.playing(); path != "" {
		a.pickPlaylist(trackName(filepath.Base(path)), []string{path})
	}
}

// pickPlaylist asks which playlist to add tracks (what, as shown) to: one
// there is, or a new one, named as typed.
func (a *bro) pickPlaylist(what string, tracks []string) {
	lib := a.mus.lib
	var es []pickEntry
	for _, pl := range lib.playlists {
		es = append(es, pickEntry{kind: "playlist", title: pl.name, detail: count(len(pl.entries), "track"),
			do: func() { a.addTo(pl, what, tracks) }})
	}
	a.openMenu("Add “"+what+"” to", "a playlist, or a new one's name…", es, func(q string) *pickEntry {
		name := strings.TrimSpace(q)
		if name == "" {
			name = freePlaylistName(lib.playlists)
		}
		return &pickEntry{kind: "new", title: name, detail: "a new playlist", do: func() { a.newPlaylist(name, what, tracks) }}
	})
}

// askNewPlaylist makes an empty playlist, named as typed.
func (a *bro) askNewPlaylist() {
	lib := a.mus.lib
	a.openMenu("New playlist", "its name… (Enter)", nil, func(q string) *pickEntry {
		name := strings.TrimSpace(q)
		if name == "" {
			name = freePlaylistName(lib.playlists)
		}
		return &pickEntry{kind: "new", title: name, detail: "a new playlist", do: func() { a.newPlaylist(name, "", nil) }}
	})
}

// newPlaylist makes a playlist (or finds the one by that name) with the
// tracks in it.
func (a *bro) newPlaylist(name, what string, tracks []string) {
	m := &a.mus
	lib := m.lib
	path := newPlaylistPath(playlistDir(lib.root), name)
	pl := lib.playlist(path)
	if pl == nil {
		if old, err := readPlaylist(path); err == nil { // there, but not found by the scan
			pl = old
		} else {
			pl = &playlist{path: path, name: trackName(filepath.Base(path))}
		}
		lib.playlists = append(lib.playlists, pl)
		lib.sortPlaylists()
	}
	a.addTo(pl, what, tracks)
	if what == "" && m.view == viewPlaylists { // made in Playlists: open it
		a.place().at, a.place().sel = pl.path, 0
		m.list = nil
		a.drawLibrary()
	}
}

// addTo adds tracks (what, as shown) to pl, and saves it.
func (a *bro) addTo(pl *playlist, what string, tracks []string) {
	m := &a.mus
	n := pl.addTracks(tracks)
	if err := pl.save(); err != nil {
		m.note = "saving " + pl.name + ": " + err.Error()
	} else {
		switch {
		case what == "":
			m.note = "made " + pl.name
		case n == 0:
			m.note = "“" + what + "” is in " + pl.name + " already"
		case len(tracks) == 1:
			m.note = "added “" + what + "” to " + pl.name
		default:
			m.note = fmt.Sprintf("added %s to %s", count(n, "track"), pl.name)
		}
	}
	log.Printf("music: %s (%s)", m.note, pl.path)
	a.relist()
	a.redrawMusic()
}

// libDelete is Delete (or "remove"): a track off the playlist open, or
// (asked first) the playlist selected.
func (a *bro) libDelete() {
	m, place := &a.mus, a.place()
	r, ok := a.selected()
	switch {
	case !ok:
	case r.kind == rowPlaylist:
		if pl := m.lib.playlist(r.path); pl != nil {
			a.askDeletePlaylist(pl)
		}
	case r.kind == rowTrack:
		pl := a.openPlaylist()
		if pl == nil {
			return // tracks are deleted in the Files tab
		}
		pl.remove(r.idx)
		m.note = "took “" + r.title + "” off " + pl.name
		if err := pl.save(); err != nil {
			m.note = "saving " + pl.name + ": " + err.Error()
		}
		log.Printf("music: %s", m.note)
		m.list = a.makeList()
		place.sel = min(place.sel, len(m.list)-1)
		a.redrawMusic()
	}
}

// libMoveEntry moves the entry selected in the playlist open d places.
func (a *bro) libMoveEntry(d int) {
	m, place := &a.mus, a.place()
	pl := a.openPlaylist()
	r, ok := a.selected()
	if pl == nil || !ok || !pl.move(r.idx, d) {
		return
	}
	if err := pl.save(); err != nil {
		m.note = "saving " + pl.name + ": " + err.Error()
		a.drawMusicStatus()
	}
	m.list = a.makeList()
	place.sel = r.idx + d
	a.drawLibrary()
}

func (a *bro) pickDeletePlaylist() {
	var es []pickEntry
	for _, pl := range a.mus.lib.playlists {
		es = append(es, pickEntry{kind: "playlist", title: pl.name, detail: count(len(pl.entries), "track"), do: func() { a.askDeletePlaylist(pl) }})
	}
	a.openMenu("Delete a playlist", "which one?", es, nil)
}

func (a *bro) askDeletePlaylist(pl *playlist) {
	a.ask("Delete the playlist "+pl.name+"?", "The playlist goes; the tracks in it stay where they are.", "delete", func() {
		m := &a.mus
		if err := os.Remove(pl.path); err != nil && !os.IsNotExist(err) {
			m.note = "deleting " + pl.name + ": " + err.Error()
		} else {
			m.note = "deleted the playlist " + pl.name
			m.lib.playlists = slices.DeleteFunc(m.lib.playlists, func(p *playlist) bool { return p == pl })
		}
		log.Printf("music: %s", m.note)
		m.list = nil
		a.redrawMusic()
	})
}
