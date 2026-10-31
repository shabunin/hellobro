package main

import (
	"fmt"
	"image"
	"slices"
	"testing"

	"hellobro/kobo"
	"hellobro/pdf"
)

// A book's bar fits its window, however narrow, with fingers' sizes
// too: nothing overlaps or goes past its end, the arrows and the
// counter are always there, and what goes goes in its order: the drops
// first, then the last tools, ‹ All Books last of all.
func TestBookBarFits(t *testing.T) {
	a := &bro{App: &kobo.App{F: kobo.LoadFonts()}}
	defer kobo.SetBig(false)
	for _, big := range []bool{false, true} {
		kobo.SetBig(big)
		for _, isPDF := range []bool{false, true} {
			for _, back := range []bool{false, true} {
				w := &bookWin{}
				if isPDF {
					w.pdf = &pdf.Doc{}
				}
				if back {
					w.from = &bookFrom{}
				}
				tools, drops := w.barTools()
				for wd := 200; wd <= 1400; wd += 25 {
					bar := image.Rect(8, 8, 8+wd, 8+barHeight())
					btns := layTop(a, bar, tools, drops, kobo.TextWidth(a.F.Small, "999 / 999"))
					at := fmt.Sprintf("%d wide (big %v, pdf %v, back %v)", wd, big, isPDF, back)
					n := len(btns)
					if n < 2 || btns[n-2].id != "next" || btns[n-1].id != "prev" {
						t.Fatalf("%s: no arrows: %v", at, btns)
					}
					for i, b := range btns {
						if !b.r.In(bar) {
							t.Errorf("%s: %s at %v, past the bar %v", at, b.id, b.r, bar)
						}
						for _, c := range btns[i+1:] {
							if b.r.Overlaps(c.r) {
								t.Errorf("%s: %s and %s overlap", at, b.id, c.id)
							}
						}
					}
					if r := barCounter(btns, bar, false); wd >= 300 && r.Dx() < kobo.TextWidth(a.F.Small, "999 / 999") {
						t.Errorf("%s: the counter is %v", at, r)
					}
					shown := map[string]bool{}
					for _, b := range btns[:n-2] {
						shown[b.id] = true
					}
					has := func(id string) bool { return slices.ContainsFunc(tools, func(b barBtn) bool { return b.id == id }) }
					for i, d := range drops { // the drops go in their order: one kept, the later ones are too
						for _, e := range drops[i+1:] {
							if shown[d] && has(e) && !shown[e] {
								t.Errorf("%s: %s kept, %s after it gone", at, d, e)
							}
						}
					}
					kept := slices.DeleteFunc(slices.Clone(tools), func(b barBtn) bool { return slices.Contains(drops, b.id) })
					for i := range kept { // the rest go from the end
						if i > 0 && shown[kept[i].id] && !shown[kept[i-1].id] {
							t.Errorf("%s: %s kept, %s before it gone", at, kept[i].id, kept[i-1].id)
						}
					}
					if wd >= 1400 && len(shown) != len(tools) {
						t.Errorf("%s: %d tools of %d", at, len(shown), len(tools))
					}
				}
			}
		}
	}
}
