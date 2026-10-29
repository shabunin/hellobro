package book

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gobolditalic"
	"golang.org/x/image/font/gofont/goitalic"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// goFaces are the Go fonts, for the tests.
type goFaces map[[2]int]font.Face

func (g goFaces) Face(s Style, px int) font.Face {
	k := [2]int{int(s & (Bold | Italic | Mono)), px}
	if f := g[k]; f != nil {
		return f
	}
	ttf := [4][]byte{goregular.TTF, gobold.TTF, goitalic.TTF, gobolditalic.TTF}[s&(Bold|Italic)]
	if s&Mono != 0 {
		ttf = gomono.TTF
	}
	o, _ := opentype.Parse(ttf)
	f, _ := opentype.NewFace(o, &opentype.FaceOptions{Size: float64(px), DPI: 72, Hinting: font.HintingFull})
	g[k] = f
	return f
}

func para(text string) Block {
	return Block{Spans: []Span{{Text: text, Scale: 1}}, First: 1.5}
}

func lorem(n int) string {
	words := strings.Fields("so she was considering in her own mind as well as she could for the hot day made her feel very sleepy and stupid whether the pleasure of making a daisy-chain would be worth the trouble of getting up and picking the daisies")
	var s []string
	for i := range n {
		s = append(s, words[i%len(words)])
	}
	return strings.Join(s, " ")
}

func TestLayoutLines(t *testing.T) {
	ch := &Chapter{}
	for i := range 12 {
		ch.Blocks = append(ch.Blocks, para(fmt.Sprint(i)+" "+lorem(80+i*7)))
	}
	ch.Blocks = append(ch.Blocks, Block{Spans: []Span{{Text: "averyveryveryveryveryveryveryveryverylongwordthatwontfitonanyline", Scale: 1}}})
	s := Settings{W: 600, H: 800, Px: 30}
	pages := Layout(ch, s, goFaces{}, nil)
	if len(pages) < 4 {
		t.Fatalf("%d pages", len(pages))
	}
	words := 0
	for pi, p := range pages {
		ends := map[int]int{} // a line's baseline: where it ends
		for _, it := range p.Items {
			if it.X < 0 || it.X+it.W > s.W+1 || it.Y > s.H {
				t.Errorf("page %d: %q at %d,%d w %d: off the page", pi, it.Text, it.X, it.Y, it.W)
			}
			ends[it.Y] = max(ends[it.Y], it.X+it.W)
			words += len(strings.Fields(it.Text))
		}
		full := 0
		for _, e := range ends {
			if e >= s.W-2 {
				full++
			}
		}
		if full < len(ends)/2 {
			t.Errorf("page %d: only %d of %d lines justified", pi, full, len(ends))
		}
	}
	want := 0
	for _, b := range ch.Blocks {
		want += len(strings.Fields(b.Spans[0].Text))
	}
	if words < want { // split words count twice
		t.Errorf("%d words set, of %d", words, want)
	}
	// the place, found again in another size
	at := pages[2].Start
	big := Layout(ch, Settings{W: 600, H: 800, Px: 40}, goFaces{}, nil)
	n := PageAt(big, at)
	if n == 0 || at.Before(big[n].Start) {
		t.Errorf("page %d at %v starts %v", n, at, big[n].Start)
	}
	if n+1 < len(big) && !at.Before(big[n+1].Start) {
		t.Errorf("%v is past page %d", at, n)
	}
}

func TestLayoutStyles(t *testing.T) {
	ch := &Chapter{Blocks: []Block{
		{Head: 2, Align: Center, Spans: []Span{{Text: "Down the ", Style: Bold, Scale: 1.4}, {Text: "Rabbit", Style: Bold | Italic, Scale: 1.4}}},
		{Spans: []Span{{Text: "one", Scale: 1}, {Text: "\n", Scale: 1}, {Text: "two x", Scale: 1}, {Text: "2", Style: Sup, Scale: 1}}},
		{Kind: Rule},
		{Pre: true, Align: Left, Spans: []Span{{Text: "a := 1\n\tb", Style: Mono, Scale: 1}}},
	}}
	s := Settings{W: 600, H: 800, Px: 30}
	p := Layout(ch, s, goFaces{}, nil)[0]
	var got []string
	for _, it := range p.Items {
		got = append(got, fmt.Sprintf("%d %q %d %d", it.Kind, it.Text, it.Style, it.Px))
	}
	want := []string{
		`0 "Down the " 1 42`, `0 "Rabbit" 3 42`,
		`0 "one" 0 30`, `0 "two x" 0 30`, `0 "2" 0 21`,
		`2 "" 0 0`,
		`0 "a := 1" 4 30`, `0 "\tb" 4 30`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("items:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if c := p.Items[0].X + (p.Items[1].X+p.Items[1].W-p.Items[0].X)/2; c < s.W/2-2 || c > s.W/2+2 {
		t.Errorf("heading not centered: %d", c)
	}
	if p.Items[4].Y >= p.Items[3].Y {
		t.Error("superscript not raised")
	}
}

func TestLayoutFloat(t *testing.T) {
	ch := &Chapter{Blocks: []Block{
		{Kind: Picture, Src: "cap", Float: FloatLeft},
		para(lorem(60)),
		{Kind: Picture, Src: "big"},
	}}
	sizes := map[string]image.Point{"cap": {60, 60}, "big": {2000, 1000}}
	s := Settings{W: 600, H: 800, Px: 30}
	pages := Layout(ch, s, goFaces{}, func(src string) image.Point { return sizes[src] })
	cap := pages[0].Items[0]
	if cap.Kind != ItemPicture || cap.X != 0 || cap.W != 100 {
		t.Fatalf("drop cap: %+v", cap)
	}
	beside := 0
	for _, it := range pages[0].Items[1:] {
		if it.Kind == ItemText && it.Y < cap.Y+cap.H && it.X < cap.W {
			t.Errorf("%q at %d, under the picture", it.Text, it.X)
		}
		if it.Kind == ItemText && it.Y < cap.Y+cap.H {
			beside++
		}
	}
	if beside == 0 {
		t.Error("no text beside the picture")
	}
	last := pages[len(pages)-1].Items
	big := last[len(last)-1]
	if big.Kind != ItemPicture || big.W > s.W || big.H > s.H || big.W != s.W && big.H != s.H-big.Y {
		t.Errorf("big picture: %+v", big)
	}
}

// TestRender draws a real book's pages as PNGs, to look at:
// BOOK=file.epub RENDER=dir [CHAPTER=n].
func TestRender(t *testing.T) {
	name, dir := os.Getenv("BOOK"), os.Getenv("RENDER")
	if name == "" || dir == "" {
		t.Skip("BOOK, RENDER not set")
	}
	b, err := Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	n, _ := strconv.Atoi(os.Getenv("CHAPTER"))
	ch, err := b.Chapter(n)
	if err != nil {
		t.Fatal(err)
	}
	faces := goFaces{}
	s := Settings{W: 1264 - 160, H: 1680 - 200, Px: 34, Lang: b.Lang}
	pages := Layout(ch, s, faces, func(src string) image.Point { p, _ := b.ImageSize(src); return p })
	t.Logf("%d pages", len(pages))
	for i, p := range pages {
		if i >= 6 {
			break
		}
		m := image.NewGray(image.Rect(0, 0, 1264, 1680))
		draw.Draw(m, m.Bounds(), image.White, image.Point{}, draw.Src)
		at := image.Pt(80, 100)
		for _, it := range p.Items {
			switch it.Kind {
			case ItemText:
				d := font.Drawer{Dst: m, Src: image.Black, Face: faces.Face(it.Style, it.Px), Dot: fixed.P(at.X+it.X, at.Y+it.Y)}
				d.DrawString(it.Text)
			case ItemRule:
				draw.Draw(m, image.Rect(it.X, it.Y, it.X+it.W, it.Y+it.H).Add(at), image.Black, image.Point{}, draw.Src)
			case ItemPicture:
				r := image.Rect(it.X, it.Y, it.X+it.W, it.Y+it.H).Add(at)
				if pic, err := b.Image(it.Text); err == nil {
					scale(m, r, pic)
				} else {
					draw.Draw(m, r, &image.Uniform{color.Gray{0xcc}}, image.Point{}, draw.Src)
				}
			}
		}
		f, _ := os.Create(filepath.Join(dir, fmt.Sprintf("page-%02d.png", i)))
		png.Encode(f, m)
		f.Close()
	}
}

// scale draws src into r, nearest pixel: enough to look at.
func scale(dst *image.Gray, r image.Rectangle, src image.Image) {
	sb := src.Bounds()
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			c := src.At(sb.Min.X+(x-r.Min.X)*sb.Dx()/r.Dx(), sb.Min.Y+(y-r.Min.Y)*sb.Dy()/r.Dy())
			dst.Set(x, y, c)
		}
	}
}

// TestLayoutHyphens: English breaks words at the end of a line; a book
// in another language doesn't, but where it has soft hyphens.
func TestLayoutHyphens(t *testing.T) {
	broken := func(lang string, text string) (n int, all string) {
		ch := &Chapter{Blocks: []Block{para(text)}}
		for _, p := range Layout(ch, Settings{W: 320, H: 4000, Px: 30, Lang: lang}, goFaces{}, nil) {
			for _, it := range p.Items {
				if strings.HasSuffix(it.Text, "-") && !strings.HasSuffix(it.Text, "daisy-") {
					n++
				}
				all += it.Text
			}
		}
		return n, all
	}
	if n, _ := broken("en", lorem(200)); n == 0 {
		t.Error("English: no word hyphenated")
	}
	if n, _ := broken("ru", lorem(200)); n != 0 {
		t.Errorf("Russian: %d words hyphenated by English patterns", n)
	}
	soft := strings.Repeat("con\u00adsid\u00ader\u00ading ", 30)
	n, all := broken("ru", soft)
	if n == 0 || strings.Contains(all, "\u00ad") {
		t.Errorf("soft hyphens: %d breaks, shown: %v", n, strings.Contains(all, "\u00ad"))
	}
}
