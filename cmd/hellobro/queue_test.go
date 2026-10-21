package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func tracks(n int) []string {
	var out []string
	for i := range n {
		out = append(out, string(rune('a'+i)))
	}
	return out
}

func TestQueueInOrder(t *testing.T) {
	q := newQueue(qAlbum, "x", "x", tracks(3))
	var got []string
	for {
		got = append(got, q.path())
		if !q.step(1, repeatOff, false) {
			break
		}
	}
	if strings.Join(got, "") != "abc" {
		t.Fatalf("played %v", got)
	}
	if q.path() != "c" || q.step(-1, repeatOff, false) && q.path() != "b" {
		t.Fatalf("back: %s", q.path())
	}
	q.jump(0)
	if q.step(-1, repeatOff, false) {
		t.Fatal("went back past the first")
	}
	if !q.step(-1, repeatAll, false) || q.path() != "c" {
		t.Fatalf("round, backwards: %s", q.path())
	}
	if !q.step(1, repeatAll, false) || q.path() != "a" {
		t.Fatalf("round: %s", q.path())
	}
	if !q.step(1, repeatOne, false) || q.path() != "a" {
		t.Fatalf("repeat one: %s", q.path())
	}
}

func TestQueueShuffle(t *testing.T) {
	q := newQueue(qFolder, "x", "x", tracks(20))
	q.jump(7)
	q.shuffle(true)
	if q.path() != "h" || q.pos != 0 {
		t.Fatalf("the one on isn't first: %s at %d", q.path(), q.pos)
	}
	seen := map[string]bool{}
	for {
		seen[q.path()] = true
		if !q.step(1, repeatOff, true) {
			break
		}
	}
	if len(seen) != 20 {
		t.Fatalf("played %d of 20", len(seen))
	}
	last := q.path()
	if !q.step(1, repeatAll, true) || q.pos != 0 || q.path() == last {
		t.Fatalf("round again: %s at %d, after %s", q.path(), q.pos, last)
	}
	if !q.shuffled() {
		t.Fatal("not shuffled")
	}
	cur := q.path()
	q.unshuffle()
	if q.path() != cur || q.shuffled() || q.order[q.pos] != q.find(cur) {
		t.Fatalf("unshuffled: %s, was %s", q.path(), cur)
	}
}

func TestPlaylistFile(t *testing.T) {
	root := t.TempDir()
	for _, f := range []string{"Music/A/1.mp3", "Music/A/2.mp3", "Music/B/x y.mp3"} {
		os.MkdirAll(filepath.Join(root, filepath.Dir(f)), 0o755)
		os.WriteFile(filepath.Join(root, f), nil, 0o644)
	}
	dir := playlistDir(root)
	if dir != filepath.Join(root, "Music", "Playlists") {
		t.Fatalf("dir %s", dir)
	}
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "pc.m3u"), []byte("\ufeff#EXTM3U\r\n#EXTINF:1,x\r\n..\\B\\x y.mp3\r\n../A/gone.mp3\r\n"+root+"/Music/A/1.mp3\n"), 0o644)
	pl, err := readPlaylist(filepath.Join(dir, "pc.m3u"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{root + "/Music/B/x y.mp3", root + "/Music/A/gone.mp3", root + "/Music/A/1.mp3"}
	if !slices.Equal(pl.entries, want) || !slices.Equal(pl.missing, []bool{false, true, false}) || pl.name != "pc" {
		t.Fatalf("read %q %v", pl.entries, pl.missing)
	}
	if pl.count() != 2 {
		t.Fatalf("count %d", pl.count())
	}
	if n := pl.addTracks([]string{root + "/Music/A/1.mp3", root + "/Music/A/2.mp3"}); n != 1 {
		t.Fatalf("added %d", n)
	}
	pl.remove(1)
	if !pl.move(2, -1) || pl.move(0, -1) {
		t.Fatal("move")
	}
	pl.path = newPlaylistPath(dir, " Road: trip/2 ")
	if filepath.Base(pl.path) != "Road_ trip_2.m3u8" {
		t.Fatalf("named %s", pl.path)
	}
	if err := pl.save(); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(pl.path)
	if string(b) != "#EXTM3U\n../B/x y.mp3\n../A/2.mp3\n../A/1.mp3\n" {
		t.Fatalf("saved %q", b)
	}
	back, _ := readPlaylist(pl.path)
	if !slices.Equal(back.entries, pl.entries) {
		t.Fatalf("read back %q", back.entries)
	}
	if n := freePlaylistName([]*playlist{{name: "Playlist 1"}, {name: "playlist 2"}}); n != "Playlist 3" {
		t.Fatalf("free name %s", n)
	}
}

func TestLibraryFolders(t *testing.T) {
	root := t.TempDir()
	for _, f := range []string{
		"Music/Heroes/HoMM3/01 a.mp3", "Music/Heroes/HoMM2/02 b.mp3", "Music/Heroes/HoMM2/10 c.mp3",
		"Music/Heroes/HoMM2/Extra/x.mp3", "Music/Heroes/HoMM2 Live/y.mp3", "Music/Jazz/z.mp3",
		"Music/Playlists/mine.m3u8", "Books/b.epub", ".kobo/k.mp3",
	} {
		os.MkdirAll(filepath.Join(root, filepath.Dir(f)), 0o755)
		os.WriteFile(filepath.Join(root, f), nil, 0o644)
	}
	lib := scanLibrary(root)
	m := root + "/Music"
	if lib.top != m || len(lib.albums) != 5 || lib.tracks != 6 || len(lib.playlists) != 1 {
		t.Fatalf("top %s, %d albums, %d tracks, %d playlists", lib.top, len(lib.albums), lib.tracks, len(lib.playlists))
	}
	var dirs []string
	for _, al := range lib.albums {
		dirs = append(dirs, strings.TrimPrefix(al.dir, m+"/"))
	}
	if strings.Join(dirs, "|") != "Heroes/HoMM2|Heroes/HoMM2/Extra|Heroes/HoMM2 Live|Heroes/HoMM3|Jazz" {
		t.Fatalf("albums in order %v", dirs)
	}
	if subs := lib.subfolders(m); !slices.Equal(subs, []string{m + "/Heroes", m + "/Jazz"}) {
		t.Fatalf("subfolders %v", subs)
	}
	if subs := lib.subfolders(m + "/Heroes/HoMM2"); !slices.Equal(subs, []string{m + "/Heroes/HoMM2/Extra"}) {
		t.Fatalf("subfolders of an album %v", subs)
	}
	var names []string
	for _, p := range lib.folderTracks(m + "/Heroes") {
		names = append(names, filepath.Base(p))
	}
	if strings.Join(names, " ") != "02 b.mp3 10 c.mp3 x.mp3 y.mp3 01 a.mp3" || lib.folderCount(m+"/Heroes/HoMM2") != 3 {
		t.Fatalf("folder tracks %v", names)
	}
	if !under(m+"/Heroes/HoMM2/Extra", m+"/Heroes") || under(m+"/Heroes/HoMM2 Live", m+"/Heroes/HoMM2") {
		t.Fatal("under")
	}
}
