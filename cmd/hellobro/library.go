package main

// The Music tab's library: albums are folders with MP3 or WAV files in
// them, found under the Kobo's storage, each with its cover if it has one.

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

// scanLibrary finds the albums under root, in path order, skipping hidden
// folders (.kobo, .adds).
func scanLibrary(root string) []album {
	byDir := map[string]*album{}
	images := map[string][]string{}
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
	var albums []album
	for _, al := range byDir {
		slices.SortFunc(al.tracks, naturalCompare)
		al.cover = pickCover(images[al.dir])
		if al.cover != "" {
			al.cover = filepath.Join(al.dir, al.cover)
		}
		albums = append(albums, *al)
	}
	slices.SortFunc(albums, func(a, b album) int { return naturalCompare(a.dir, b.dir) })
	return albums
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
