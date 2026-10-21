package main

// Playlists are M3U files: a track per line, its path relative to the
// playlist (or whole), "#" lines aside. hellobro makes its own in
// Music/Playlists (or Playlists, with no Music folder), and finds any
// others in the library too, ones a computer made, say.

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type playlist struct {
	path    string   // the .m3u or .m3u8 file
	name    string   // as shown: the file's name less the extension
	entries []string // the tracks, whole paths, in order
	missing []bool   // by entry: the file isn't there
}

// isPlaylist says whether the file is an M3U playlist.
func isPlaylist(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".m3u" || ext == ".m3u8"
}

// playlistDir is where new playlists go.
func playlistDir(root string) string {
	if st, err := os.Stat(filepath.Join(root, "Music")); err == nil && st.IsDir() {
		return filepath.Join(root, "Music", "Playlists")
	}
	return filepath.Join(root, "Playlists")
}

// readPlaylist reads an M3U file. Windows paths (backslashes) are fine.
func readPlaylist(path string) (*playlist, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	pl := &playlist{path: path, name: trackName(filepath.Base(path))}
	dir := filepath.Dir(path)
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "\ufeff"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.ReplaceAll(line, `\`, "/")
		line = strings.TrimPrefix(line, "file://")
		if !filepath.IsAbs(line) {
			line = filepath.Join(dir, line)
		}
		pl.add(filepath.Clean(line))
	}
	return pl, nil
}

func (pl *playlist) add(path string) {
	_, err := os.Stat(path)
	pl.entries = append(pl.entries, path)
	pl.missing = append(pl.missing, err != nil)
}

// playable is the entries that are there, in order.
func (pl *playlist) playable() []string {
	var out []string
	for i, e := range pl.entries {
		if !pl.missing[i] {
			out = append(out, e)
		}
	}
	return out
}

func (pl *playlist) count() int { return len(pl.playable()) }

// save writes the playlist, its paths relative to it where they can be.
func (pl *playlist) save() error {
	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	dir := filepath.Dir(pl.path)
	for _, e := range pl.entries {
		if rel, err := filepath.Rel(dir, e); err == nil {
			e = rel
		}
		b.WriteString(filepath.ToSlash(e) + "\n")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp := pl.path + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, pl.path)
}

// addTracks adds the tracks not in it yet; how many it added.
func (pl *playlist) addTracks(tracks []string) int {
	n := 0
	for _, t := range tracks {
		if !slices.Contains(pl.entries, t) {
			pl.add(t)
			n++
		}
	}
	return n
}

func (pl *playlist) remove(i int) {
	if i >= 0 && i < len(pl.entries) {
		pl.entries = slices.Delete(pl.entries, i, i+1)
		pl.missing = slices.Delete(pl.missing, i, i+1)
	}
}

// move moves entry i d places; false if it can't.
func (pl *playlist) move(i, d int) bool {
	j := i + d
	if i < 0 || i >= len(pl.entries) || j < 0 || j >= len(pl.entries) {
		return false
	}
	pl.entries[i], pl.entries[j] = pl.entries[j], pl.entries[i]
	pl.missing[i], pl.missing[j] = pl.missing[j], pl.missing[i]
	return true
}

// newPlaylistPath is the file for a new playlist called name in dir.
func newPlaylistPath(dir, name string) string {
	name = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`/\:*?"<>|`, r) || r < ' ' {
			return '_'
		}
		return r
	}, strings.TrimSpace(name))
	name = strings.Trim(name, ". ")
	if name == "" {
		name = "Playlist"
	}
	return filepath.Join(dir, name+".m3u8")
}

// freePlaylistName is "Playlist N", the first N no playlist has.
func freePlaylistName(pls []*playlist) string {
	for n := 1; ; n++ {
		name := fmt.Sprintf("Playlist %d", n)
		if !slices.ContainsFunc(pls, func(pl *playlist) bool { return strings.EqualFold(pl.name, name) }) {
			return name
		}
	}
}
