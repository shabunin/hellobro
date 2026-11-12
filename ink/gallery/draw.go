package gallery

import (
	"fmt"
	"image"

	"hellobro/ink"
	"hellobro/ink/layout"
	"hellobro/ink/text"
	"hellobro/ink/theme"
)

// The map's places, in Dp on a town 1600 by 1200: tapping a pin says it.
var places = []struct {
	Name string
	At   image.Point
}{
	{"the library", image.Pt(260, 220)}, {"the station", image.Pt(820, 160)}, {"the market", image.Pt(560, 520)},
	{"the old bridge", image.Pt(1010, 640)}, {"a café", image.Pt(340, 860)}, {"the school", image.Pt(1320, 380)},
	{"the park's pond", image.Pt(1240, 960)}, {"the bookshop", image.Pt(720, 1040)},
}

// town is a map to drag about: streets on a grid, a river, parks, pins.
func (g *Gallery) town(gtx ink.Ctx) ink.Dims {
	th := g.Th
	for i := range g.places {
		if g.places[i].Clicked(gtx) {
			g.Said = "the map: " + places[i].Name
		}
	}
	dp := func(x, y int) image.Point { return image.Pt(gtx.Dp(ink.Dp(x)), gtx.Dp(ink.Dp(y))) }
	size := dp(1600, 1200)
	all := image.Rectangle{Max: size}
	ink.Fill(gtx.Ops, all, th.Bg)
	// Parks first, then the river, the streets over them.
	for _, p := range [][4]int{{80, 600, 420, 760}, {1100, 820, 1480, 1120}, {980, 60, 1180, 260}} {
		ink.Fill(gtx.Ops, image.Rectangle{Min: dp(p[0], p[1]), Max: dp(p[2], p[3])}, ink.Light)
	}
	ink.Polygon(gtx.Ops, ink.Mid, dp(900, 0), dp(960, 0), dp(1080, 600), dp(1060, 900), dp(1120, 1200),
		dp(1060, 1200), dp(1000, 900), dp(1020, 600))
	w, thin := gtx.Dp(6), gtx.Dp(2)
	for x := 100; x < 1600; x += 200 {
		ink.Line(gtx.Ops, thin, th.Fg, dp(x, 0), dp(x, 1200))
	}
	for y := 80; y < 1200; y += 160 {
		ink.Line(gtx.Ops, thin, th.Fg, dp(0, y), dp(1600, y))
	}
	ink.Line(gtx.Ops, w, th.Fg, dp(0, 1150), dp(600, 600), dp(1600, 300)) // the high street
	ink.Line(gtx.Ops, w, th.Fg, dp(40, 400), dp(1600, 700))
	f := th.Face(gtx, th.TextSize*0.85, text.Regular)
	bold := th.Face(gtx, th.TextSize, text.Bold)
	ink.TextAt(gtx.Ops, bold, gtx.Dp(110), gtx.Dp(640), "west park", th.Fg)
	ink.TextAt(gtx.Ops, bold, gtx.Dp(1140), gtx.Dp(860), "river park", th.Fg)
	r := gtx.Dp(th.Touch / 2)
	for i, p := range places {
		at := dp(p.At.X, p.At.Y)
		st := ink.Offset(gtx.Ops, at.Sub(image.Pt(r, r)))
		cg := gtx
		cg.Constraints = ink.Exact(image.Pt(2*r, 2*r))
		clk := &g.places[i]
		clk.Layout(cg, func(gtx ink.Ctx) ink.Dims {
			c := image.Pt(r, r)
			ink.Disc(gtx.Ops, c, r*2/3, th.Bg)
			ink.Ring(gtx.Ops, c, r*2/3, thin, th.Fg)
			ink.Disc(gtx.Ops, c, r/3, map[bool]uint8{true: th.Bg, false: th.Fg}[clk.Pressed()])
			return ink.Dims{Size: image.Pt(2*r, 2*r)}
		})
		st.Pop()
		tw := text.Width(f, p.Name)
		lb := image.Rect(at.X+r, at.Y-text.Height(f)/2-2, at.X+r+tw+gtx.Dp(8), at.Y+text.Height(f)/2+2)
		ink.Fill(gtx.Ops, lb, th.Bg)
		ink.TextAt(gtx.Ops, f, lb.Min.X+gtx.Dp(4), lb.Min.Y+2+text.Ascent(f), p.Name, th.Fg)
	}
	return ink.Dims{Size: size}
}

// mapBox is the map in a frame, a line under it saying where it is.
func (g *Gallery) mapBox(gtx ink.Ctx) ink.Dims {
	th := g.Th
	size := gtx.Constraints.Max
	b := max(gtx.Dp(th.Border), 1)
	in := image.Rectangle{Max: size}.Inset(b)
	st := ink.Offset(gtx.Ops, in.Min)
	cg := gtx
	cg.Constraints = ink.Exact(in.Size())
	g.mapView.Layout(cg, g.town)
	st.Pop()
	ink.Frame(gtx.Ops, image.Rectangle{Max: size}, b, th.Fg)
	return ink.Dims{Size: size}
}

func (g *Gallery) drawPage(gtx ink.Ctx) ink.Dims {
	th := g.Th
	if g.fingerDraws.Update(gtx) {
		g.Said = "a finger draws: " + onOff(g.fingerDraws.Value)
	}
	if g.rubbing.Update(gtx) {
		g.Said = "the pen rubs out: " + onOff(g.rubbing.Value)
	}
	if g.undo.Clicked(gtx) {
		g.canvas.Undo()
	}
	if g.clear.Clicked(gtx) {
		g.canvas.Clear()
	}
	g.canvas.FingerDraws, g.canvas.Erasing = g.fingerDraws.Value, g.rubbing.Value
	if g.canvas.Changed() {
		g.Said = fmt.Sprintf("%d strokes", g.canvas.Strokes())
	}
	sp := layout.Rigid(layout.Spacer{Width: th.Pad * 2}.Layout)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(theme.Section(th, "A sheet: the pen draws, harder thicker; its button rubs out")),
		layout.Rigid(func(gtx ink.Ctx) ink.Dims {
			return layout.Inset{Left: th.Pad * 1.5, Right: th.Pad * 1.5, Top: th.Pad}.Layout(gtx, func(gtx ink.Ctx) ink.Dims {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1.3, theme.CheckBox(th, &g.fingerDraws, "a finger draws").Layout),
					layout.Flexed(1, theme.CheckBox(th, &g.rubbing, "rub out").Layout), sp,
					layout.Rigid(theme.Button(th, &g.undo, "undo").Layout),
					layout.Rigid(layout.Spacer{Width: th.Pad}.Layout),
					layout.Rigid(theme.Button(th, &g.clear, "clear").Layout))
			})
		}),
		layout.Flexed(1, func(gtx ink.Ctx) ink.Dims {
			return layout.UniformInset(th.Pad*1.5).Layout(gtx, func(gtx ink.Ctx) ink.Dims {
				size := gtx.Constraints.Max
				b := max(gtx.Dp(th.Border), 1)
				in := image.Rectangle{Max: size}.Inset(b)
				st := ink.Offset(gtx.Ops, in.Min)
				cg := gtx
				cg.Constraints = ink.Exact(in.Size())
				g.canvas.Layout(cg)
				st.Pop()
				ink.Frame(gtx.Ops, image.Rectangle{Max: size}, b, th.Fg)
				return ink.Dims{Size: size}
			})
		}),
	)
}
