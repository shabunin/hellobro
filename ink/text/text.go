// Package text has ink's fonts and what it does with words: faces by
// style and size, their widths, lines wrapped to a width, a line cut
// short with an ellipsis.
package text

import (
	"strings"
	"sync"
	"unicode"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goitalic"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
)

// Style is a face of the family: regular, bold, italic, mono.
type Style uint8

const (
	Regular Style = iota
	Bold
	Italic
	Mono
)

// Shaper makes faces, and keeps them: a face is costly to make, and
// keeps a cache of its own.
type Shaper struct {
	TTF [4][]byte // a font for each Style; nil: the Go fonts'

	mu    sync.Mutex
	fonts [4]*opentype.Font
	faces map[faceKey]font.Face
}

type faceKey struct {
	s  Style
	px int
}

// NewShaper is one with the Go fonts.
func NewShaper() *Shaper { return &Shaper{} }

var goTTF = [4][]byte{goregular.TTF, gobold.TTF, goitalic.TTF, gomono.TTF}

// Face is style's face, px pixels tall.
func (s *Shaper) Face(st Style, px int) font.Face {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := faceKey{st, max(px, 4)}
	if f, ok := s.faces[k]; ok {
		return f
	}
	if s.fonts[st] == nil {
		ttf := s.TTF[st]
		if ttf == nil {
			ttf = goTTF[st]
		}
		f, err := opentype.Parse(ttf)
		if err != nil {
			f, _ = opentype.Parse(goTTF[st])
		}
		s.fonts[st] = f
	}
	f, err := opentype.NewFace(s.fonts[st], &opentype.FaceOptions{Size: float64(k.px), DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		panic("ink/text: " + err.Error())
	}
	if s.faces == nil {
		s.faces = map[faceKey]font.Face{}
	}
	s.faces[k] = f
	return f
}

// Width is s's width in f, in pixels.
func Width(f font.Face, s string) int { return font.MeasureString(f, s).Ceil() }

// Height is a line's height in f: ascent, descent and the gap between.
func Height(f font.Face) int { return f.Metrics().Height.Ceil() }

// Ascent is how far f goes above the baseline.
func Ascent(f font.Face) int { return f.Metrics().Ascent.Ceil() }

// Fit is s as it fits in w pixels: cut, with an ellipsis, if it's wider.
func Fit(f font.Face, s string, w int) string {
	if Width(f, s) <= w {
		return s
	}
	r := []rune(s)
	// Halve the way to the longest prefix that fits with its "…".
	lo, hi := 0, len(r)
	for lo < hi {
		m := (lo + hi + 1) / 2
		if Width(f, string(r[:m])+"…") <= w {
			lo = m
		} else {
			hi = m - 1
		}
	}
	return strings.TrimRightFunc(string(r[:lo]), unicode.IsSpace) + "…"
}

// Wrap breaks s into lines of at most w pixels: at spaces where it can,
// inside a word too long for a line, and at each newline.
func Wrap(f font.Face, s string, w int) []string {
	var lines []string
	for _, para := range strings.Split(s, "\n") {
		lines = append(lines, wrapPara(f, para, w)...)
	}
	return lines
}

func wrapPara(f font.Face, s string, w int) []string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	line := ""
	for _, word := range words {
		try := word
		if line != "" {
			try = line + " " + word
		}
		if Width(f, try) <= w {
			line = try
			continue
		}
		if line != "" {
			lines = append(lines, line)
		}
		// A word wider than a line goes on in pieces.
		for Width(f, word) > w {
			r := []rune(word)
			n := 1
			for n < len(r) && Width(f, string(r[:n+1])) <= w {
				n++
			}
			lines = append(lines, string(r[:n]))
			word = string(r[n:])
		}
		line = word
	}
	return append(lines, line)
}
