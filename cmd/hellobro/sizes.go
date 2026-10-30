package main

import "hellobro/kobo"

// The sizes of things to tap: for fingers (Settings, Touch, Sizes),
// or compact, more on the screen. HELLOBRO_TOUCH, big or small, in
// launch.sh's environment has the last word (a small sharp screen used
// by touch; the harness, whose feeds tap where things are compact).

// sizesBig is whether they're for fingers.
func (a *bro) sizesBig() bool {
	switch kobo.EnvOr("HELLOBRO_TOUCH", "") {
	case "big":
		return true
	case "small":
		return false
	}
	return a.setting("touch-size") == "fingers"
}

// applySizes sets them: the frame's (kobo's) and the windows' rows and
// buttons. Before anything is laid out, or then Relayout.
func (a *bro) applySizes() {
	big := a.sizesBig()
	kobo.SetBig(big)
	pickRowH, setRowH, setRowMin, radioRowH, libRowMin, footMin, footButton = 58, 50, 40, 56, 64, 64, 100
	if big {
		pickRowH, setRowH, setRowMin, radioRowH, libRowMin, footMin, footButton = 92, 76, 76, 80, 88, 84, 120
	}
}
