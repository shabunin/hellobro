package kobo

import (
	"image"
	"image/color"
	"image/draw"
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
		c := Canvas{ours}
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
}
