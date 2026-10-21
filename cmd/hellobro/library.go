package main

// The Music tab's library: albums are folders with MP3 or WAV files in
// them, found under the Kobo's storage, each with its cover if it has one;
// and the playlists found there.

import (
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	xdraw "golang.org/x/image/draw"

	"hellobro/audio"
	"hellobro/kobo"
)

var musicRoot = kobo.EnvOr("HELLOBRO_MUSIC_DIR", "/mnt/onboard")

const maxDepth = 5 // folders below musicRoot

type album struct {
	dir    string   // the folder
	name   string   // its name, as shown
	tracks []string // file names, in order
	cover  string   // an image in the folder, or ""
}

// trackName is a track as shown: its file name less the extension.
func trackName(file string) string { return strings.TrimSuffix(file, filepath.Ext(file)) }

// library is what the scan found.
type library struct {
	root      string
	albums    []album
	byDir     map[string]int // albums' indexes, by folder
	playlists []*playlist    // by name
	top       string         // the deepest folder that has all the albums in it
	tracks    int
}

// scanLibrary finds the albums under root, in path order, and the
// playlists, skipping hidden folders (.kobo, .adds).
func scanLibrary(root string) *library {
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	byDir := map[string]*album{}
	images := map[string][]string{}
	var lists []string
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			rel, _ := filepath.Rel(root, p)
			if p != root && (strings.HasPrefix(d.Name(), ".") || strings.Count(rel, string(filepath.Separator)) >= maxDepth) {
				return filepath.SkipDir
			}
			return nil
		}
		dir := filepath.Dir(p)
		switch strings.ToLower(filepath.Ext(p)) {
		case ".jpg", ".jpeg", ".png":
			images[dir] = append(images[dir], d.Name())
		case ".m3u", ".m3u8":
			if !strings.HasPrefix(d.Name(), ".") {
				lists = append(lists, p)
			}
		default:
			if !audio.Playable(p) {
				return nil
			}
			al := byDir[dir]
			if al == nil {
				al = &album{dir: dir, name: filepath.Base(dir)}
				if dir == root {
					al.name = "(loose tracks)"
				}
				byDir[dir] = al
			}
			al.tracks = append(al.tracks, d.Name())
		}
		return nil
	})
	lib := &library{root: root, byDir: map[string]int{}}
	for _, al := range byDir {
		slices.SortFunc(al.tracks, naturalCompare)
		al.cover = pickCover(images[al.dir])
		if al.cover != "" {
			al.cover = filepath.Join(al.dir, al.cover)
		}
		lib.albums = append(lib.albums, *al)
		lib.tracks += len(al.tracks)
	}
	slices.SortFunc(lib.albums, func(a, b album) int { return pathCompare(a.dir, b.dir) })
	for i, al := range lib.albums {
		lib.byDir[al.dir] = i
		if i == 0 {
			lib.top = al.dir
		}
		for !under(al.dir, lib.top) && lib.top != filepath.Dir(lib.top) {
			lib.top = filepath.Dir(lib.top)
		}
	}
	if lib.top == "" {
		lib.top = root
	}
	for _, p := range lists {
		if pl, err := readPlaylist(p); err == nil {
			lib.playlists = append(lib.playlists, pl)
		}
	}
	lib.sortPlaylists()
	return lib
}

func (lib *library) sortPlaylists() {
	slices.SortFunc(lib.playlists, func(a, b *playlist) int {
		if c := naturalCompare(a.name, b.name); c != 0 {
			return c
		}
		return pathCompare(a.path, b.path)
	})
}

// album is the album in dir, or nil.
func (lib *library) album(dir string) *album {
	if i, ok := lib.byDir[dir]; ok {
		return &lib.albums[i]
	}
	return nil
}

// paths is the album's tracks, whole paths.
func (al *album) paths() []string {
	out := make([]string, len(al.tracks))
	for i, t := range al.tracks {
		out[i] = filepath.Join(al.dir, t)
	}
	return out
}

// under says whether path is dir or in it, however deep.
func under(path, dir string) bool {
	return path == dir || strings.HasPrefix(path, strings.TrimSuffix(dir, "/")+"/")
}

// subfolders is the folders right in dir with music in them, somewhere.
func (lib *library) subfolders(dir string) []string {
	var out []string
	for _, al := range lib.albums {
		if al.dir == dir || !under(al.dir, dir) {
			continue
		}
		rel, _ := filepath.Rel(dir, al.dir)
		sub := filepath.Join(dir, strings.Split(filepath.ToSlash(rel), "/")[0])
		if len(out) == 0 || out[len(out)-1] != sub { // the albums are in path order
			out = append(out, sub)
		}
	}
	return out
}

// folderTracks is every track in dir and its subfolders, in path order.
func (lib *library) folderTracks(dir string) []string {
	var out []string
	for i := range lib.albums {
		if under(lib.albums[i].dir, dir) {
			out = append(out, lib.albums[i].paths()...)
		}
	}
	return out
}

// folderCount is how many tracks folderTracks has.
func (lib *library) folderCount(dir string) int {
	n := 0
	for _, al := range lib.albums {
		if under(al.dir, dir) {
			n += len(al.tracks)
		}
	}
	return n
}

// playlist is the one in the file path, or nil.
func (lib *library) playlist(path string) *playlist {
	for _, pl := range lib.playlists {
		if pl.path == path {
			return pl
		}
	}
	return nil
}

// pathCompare orders paths a folder at a time, naturally, so a folder's
// subfolders come right after it.
func pathCompare(a, b string) int {
	as, bs := strings.Split(a, "/"), strings.Split(b, "/")
	for i := 0; i < len(as) && i < len(bs); i++ {
		if c := naturalCompare(as[i], bs[i]); c != 0 {
			return c
		}
	}
	return len(as) - len(bs)
}

// pickCover prefers cover.jpg, folder.jpg and the like, then a front.
func pickCover(names []string) string {
	slices.SortFunc(names, naturalCompare)
	for _, want := range []func(string) bool{
		func(n string) bool {
			b := strings.ToLower(trackName(n))
			return b == "cover" || b == "folder" || b == "front" || b == "albumart"
		},
		func(n string) bool { return strings.Contains(strings.ToLower(n), "front") },
		func(n string) bool { return strings.Contains(strings.ToLower(n), "cover") },
		func(n string) bool { return !strings.Contains(strings.ToLower(n), "back") },
	} {
		for _, n := range names {
			if want(n) {
				return n
			}
		}
	}
	return ""
}

// naturalCompare orders "2. x" before "10. x": digit runs by their value.
func naturalCompare(a, b string) int {
	for a != "" && b != "" {
		if isDigit(a[0]) && isDigit(b[0]) {
			i, j := digits(a), digits(b)
			na, nb := strings.TrimLeft(a[:i], "0"), strings.TrimLeft(b[:j], "0")
			if c := len(na) - len(nb); c != 0 {
				return c
			}
			if c := strings.Compare(na, nb); c != 0 {
				return c
			}
			a, b = a[i:], b[j:]
			continue
		}
		ca, cb := lower(a[0]), lower(b[0])
		if ca != cb {
			return int(ca) - int(cb)
		}
		a, b = a[1:], b[1:]
	}
	return len(a) - len(b)
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func digits(s string) int {
	i := 0
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	return i
}

func lower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 'a' - 'A'
	}
	return c
}

// loadCover reads an image and makes it side×side pixels (fitted, centered
// on white), in the 16 grays the panel shows, dithered.
func loadCover(path string, side int) (*image.Gray, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	src, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	b := src.Bounds()
	w, h := side, side
	if b.Dx() > b.Dy() {
		h = side * b.Dy() / max(1, b.Dx())
	} else {
		w = side * b.Dx() / max(1, b.Dy())
	}
	out := kobo.Blank(image.Pt(side, side))
	at := image.Rect((side-w)/2, (side-h)/2, (side-w)/2+w, (side-h)/2+h)
	xdraw.CatmullRom.Scale(out, at, src, b, xdraw.Src, nil)
	dither(out, at)
	return out, nil
}

// dither turns r of img into multiples of 0x11, spreading the error
// (Floyd–Steinberg), so gradients don't band.
func dither(img *image.Gray, r image.Rectangle) {
	w := r.Dx()
	cur, next := make([]int, w+2), make([]int, w+2)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			i := x - r.Min.X + 1
			v := int(img.GrayAt(x, y).Y) + cur[i]/16
			q := max(0, min(15, (v+8)/17)) * 17
			img.SetGray(x, y, color.Gray{uint8(q)})
			e := v - q
			cur[i+1] += e * 7
			next[i-1] += e * 3
			next[i] += e * 5
			next[i+1] += e
		}
		cur, next = next, cur
		clear(next)
	}
}
