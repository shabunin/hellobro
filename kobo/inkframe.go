package kobo

// The frame in ink: the bar, the title strips, the toast are ink's
// widgets (ink/theme's frame.go), painted where the layout here puts
// them. ink's letters are Go's, as kobo's are, at the same sizes: one
// Dp is one pixel here.

import (
	"image"
	"time"

	"hellobro/ink"
	"hellobro/ink/theme"
)

var (
	frameTheme = theme.New()
	frameWins  = map[image.Point]*ink.Window{} // by size: a bar, a strip, a tab
)

// FrameTheme is the theme the frame is painted in.
func FrameTheme() *theme.Theme { return frameTheme }

// FrameLook is the frame's letters and room, as the frame's sizes are
// now (Big's or not).
func FrameLook() theme.FrameLook {
	return theme.FrameLook{Name: 26, Small: 22, TabPad: ink.Dp(barTabPad), Gap: barInfoGap}
}

// PaintInk paints w on dst at r, as big as r, laid out there once.
func PaintInk(dst *image.Gray, r image.Rectangle, w ink.Widget) {
	if r.Empty() {
		return
	}
	win := frameWins[r.Size()]
	if win == nil {
		if len(frameWins) >= 32 { // a strip of each window's width, say: not for good
			clear(frameWins)
		}
		win = ink.NewWindow(r.Size(), ink.Metric{PxPerDp: 1}, w)
		frameWins[r.Size()] = win
	}
	win.UI = w
	win.Invalidate()
	win.Frame(time.Now())
	CopyGray(dst, r, win.Image(), image.Point{})
}
