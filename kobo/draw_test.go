package kobo

import (
	"image"
	"image/color"
	"image/draw"
	"slices"
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// Fill and Text draw what image/draw and font.Drawer would, faster.
func TestFillAndText(t *testing.T) {
	f := LoadFonts()
	size := image.Rect(0, 0, 400, 120)
	for _, g := range []uint8{Black, Dark, White} {
		ours, theirs := image.NewGray(size), image.NewGray(size)
		for _, img := range []*image.Gray{ours, theirs} {
			for i := range img.Pix {
				img.Pix[i] = uint8(i * 7) // a background that isn't flat
			}
		}
		c := Canvas{ours}                     // the second and third time, the glyphs come from the cache
		c.Fill(image.Rect(-10, 5, 50, 30), g) // off the edge too
		draw.Draw(theirs, image.Rect(-10, 5, 50, 30), image.NewUniform(color.Gray{g}), image.Point{}, draw.Src)
		r := image.Rect(60, 10, 300, 100) // a clip: the text doesn't fit
		s := "hello bro · ½ ↑ Wörld, and on and on and on"
		c.Text(f.Panel, r, s, g, AlignLeft)
		fs := fit(f.Panel, s, r.Dx())
		m := f.Panel.Metrics()
		d := font.Drawer{Dst: theirs.SubImage(r).(*image.Gray), Src: image.NewUniform(color.Gray{g}), Face: f.Panel,
			Dot: fixed.P(r.Min.X, r.Min.Y+(r.Dy()+m.Ascent.Ceil()-m.Descent.Ceil())/2)}
		d.DrawString(fs)
		bad := 0
		for i := range ours.Pix {
			if diff := int(ours.Pix[i]) - int(theirs.Pix[i]); diff > 1 || diff < -1 {
				bad++
			}
		}
		if bad > 0 {
			t.Errorf("gray %d: %d pixels differ", g, bad)
		}
	}
	// CopyGray, as draw.Draw copies, at an offset and off the edges.
	src := image.NewGray(size)
	for i := range src.Pix {
		src.Pix[i] = uint8(i * 13)
	}
	ours, theirs := image.NewGray(image.Rect(10, 10, 300, 90)), image.NewGray(image.Rect(10, 10, 300, 90))
	r, sp := image.Rect(0, 20, 250, 200), image.Pt(40, 5)
	CopyGray(ours, r, src, sp)
	draw.Draw(theirs, r, src, sp, draw.Src)
	for i := range ours.Pix {
		if ours.Pix[i] != theirs.Pix[i] {
			t.Fatalf("CopyGray: pixel %d is %d, want %d", i, ours.Pix[i], theirs.Pix[i])
		}
	}
}

func BenchmarkText(b *testing.B) {
	f := LoadFonts()
	c := Canvas{image.NewGray(image.Rect(0, 0, 1404, 200))}
	for _, cached := range []bool{false, true} {
		b.Run(map[bool]string{false: "rasterized", true: "cached"}[cached], func(b *testing.B) {
			for b.Loop() {
				if !cached {
					clear(glyphCache)
				}
				c.Text(f.Panel, image.Rect(0, 0, 1404, 100), "Settings · File sharing (SFTP, port 2022)", Black, AlignLeft)
				c.Text(f.Small, image.Rect(0, 100, 1404, 200), "deep sleep after 10 min · on (Files tab)", Dark, AlignLeft)
			}
		})
	}
}

// The ring goes on the part of it in img, dashed, and nowhere else.
func TestStampRing(t *testing.T) {
	img := image.NewGray(image.Rect(50, 50, 200, 120)) // a blit of part of the screen
	for i := range img.Pix {
		img.Pix[i] = White
	}
	o := image.Rect(20, 60, 180, 100)
	stampRing(img, o)
	for y := 50; y < 120; y++ {
		for x := 50; x < 200; x++ {
			in := image.Pt(x, y).In(o) && !image.Pt(x, y).In(o.Inset(ringW))
			along := x - o.Min.X
			if y >= o.Min.Y+ringW && y < o.Max.Y-ringW {
				along = y - o.Min.Y
			}
			want := uint8(White)
			if in && along%(dash*3/2) < dash {
				want = Black
			}
			if got := img.GrayAt(x, y).Y; got != want {
				t.Fatalf("%d,%d: %d, want %d", x, y, got, want)
			}
		}
	}
}

func TestSqueeze(t *testing.T) {
	for _, c := range []struct {
		want []int
		room int
		got  []int
	}{
		{[]int{100, 200}, 1000, []int{100, 200}},          // they fit
		{[]int{100, 600, 600}, 900, []int{100, 400, 400}}, // the small one keeps its own
		{[]int{500, 500}, 600, []int{300, 300}},
		{[]int{50, 300, 100, 400}, 600, []int{50, 225, 100, 225}},
		{nil, 600, []int{}},
	} {
		if got := squeeze(c.want, c.room); !slices.Equal(got, c.got) && !(len(got) == 0 && len(c.got) == 0) {
			t.Errorf("squeeze(%v, %d) = %v, want %v", c.want, c.room, got, c.got)
		}
	}
}
