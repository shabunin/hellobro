//go:build cgo

package kobo

// FBInk linked in (third_party/fbink, built by build-fbink.sh): the screen
// stays open for the whole session, so an update is just a copy and an
// ioctl, instead of starting a process that sets up the display again.

/*
#cgo CFLAGS: -I${SRCDIR}/../third_party/fbink
#cgo LDFLAGS: ${SRCDIR}/../third_party/fbink/libfbink.a -lm
#include <errno.h>
#include "fbink.h"
*/
import "C"

import (
	"fmt"
	"image"
	"log"
	"unsafe"
)

type fbinkLib struct {
	fd       C.int
	cfg      C.FBInkConfig
	inflight []refresh // refreshes that may still be running, oldest first
	pen      bool      // sunxi's pen mode is on ("PEN" blits)
}

type refresh struct {
	marker C.uint32_t
	r      image.Rectangle
}

// maxInflight bounds the refreshes queued in the panel driver. It takes
// updates much faster than it can show them, so without a limit a long
// stroke builds a backlog and the screen falls further and further behind.
const maxInflight = 2

// penInflight is the same for ink in pen mode: its pieces are small, and
// a few may be in the panel at once.
const penInflight = 4

func openFbinkLib() (fbBackend, error) {
	l := &fbinkLib{}
	l.cfg.is_quiet = true
	l.cfg.no_viewport = true // we place everything ourselves, from the panel's top-left corner
	if l.fd = C.fbink_open(); l.fd < 0 {
		return nil, fmt.Errorf("fbink_open failed")
	}
	if rc := C.fbink_init(l.fd, &l.cfg); rc < 0 {
		C.fbink_close(l.fd)
		return nil, fmt.Errorf("fbink_init: %d", rc)
	}
	return l, nil
}

func (l *fbinkLib) setRota(rota int) (ScreenInfo, error) {
	rc := C.fbink_sunxi_ntx_enforce_rota(l.fd, C.SUNXI_FORCE_ROTA_INDEX_T(rota), &l.cfg)
	if rc == -C.ENOSYS { // not a sunxi Device: FBInk follows the gyro on its own
		rc = C.fbink_reinit(l.fd, &l.cfg)
	}
	if rc < 0 {
		return ScreenInfo{}, fmt.Errorf("fbink rotation %d: %d", rota, rc)
	}
	var st C.FBInkState
	C.fbink_get_state(&l.cfg, &st)
	return ScreenInfo{
		ViewW: int(st.view_width), ViewH: int(st.view_height),
		ScreenW: int(st.screen_width), ScreenH: int(st.screen_height),
		OriginX: int(st.view_hori_origin), OriginY: int(st.view_vert_origin),
		Rota:      int(st.current_rota),
		Canonical: int(st.rotation_map[st.current_rota&3]),
		Swap:      bool(st.touch_swap_axes), MirX: bool(st.touch_mirror_x), MirY: bool(st.touch_mirror_y),
		Version:  C.GoString(C.fbink_version()) + " (linked in)",
		Codename: C.GoString(&st.device_codename[0]),
		Device:   fmt.Sprintf("Kobo %s (%s, ID %d)", C.GoString(&st.device_name[0]), C.GoString(&st.device_platform[0]), int(st.device_id)),
	}, nil
}

var waveforms = map[string]C.WFM_MODE_INDEX_T{"DU": C.WFM_DU, "GL16": C.WFM_GL16, "GC16": C.WFM_GC16}

func (l *fbinkLib) blit(img *image.Gray, r image.Rectangle, waveform string, flash bool) error {
	pix := img.Pix // flush makes img just r's size: no copy needed then
	if r != img.Rect || img.Stride != r.Dx() {
		pix = make([]byte, r.Dx()*r.Dy())
		for y := r.Min.Y; y < r.Max.Y; y++ {
			off := img.PixOffset(r.Min.X, y)
			copy(pix[(y-r.Min.Y)*r.Dx():], img.Pix[off:off+r.Dx()])
		}
	}
	// Ink as it's written goes in sunxi's pen mode (Nickel's, for the
	// stylus): DU, low latency, its pieces sent over the ones before
	// without waiting for them; ink only adds black, so they can't mix
	// up. The next other update ends it: left on, an eink kernel thread
	// spins.
	pen := waveform == "PEN"
	if pen != l.pen {
		if rc := C.fbink_sunxi_toggle_ntx_pen_mode(l.fd, C.bool(pen)); rc < 0 {
			log.Printf("fbink pen mode %v: %d", pen, rc)
		}
		l.pen = pen
	}
	if pen {
		waveform = "DU"
		l.waitInflight(penInflight - 1)
	}
	if flash { // let everything else finish, so it doesn't spoil the full refresh
		l.waitInflight(0)
	}
	// An update over pixels the panel is still driving collides with that
	// refresh: the two get mixed up, and old and new text show on top of
	// each other. Let the overlapping ones finish first.
	for i := len(l.inflight) - 1; i >= 0 && !pen; i-- {
		if l.inflight[i].r.Overlaps(r) {
			l.waitInflight(len(l.inflight) - i - 1)
			break
		}
	}
	if !pen {
		l.waitInflight(maxInflight - 1)
	}
	cfg := l.cfg
	cfg.wfm_mode = waveforms[waveform]
	cfg.is_flashing = C.bool(flash)
	rc := C.fbink_print_raw_data(l.fd, (*C.uchar)(unsafe.Pointer(&pix[0])), C.int(r.Dx()), C.int(r.Dy()),
		C.size_t(len(pix)), C.short(r.Min.X), C.short(r.Min.Y), &cfg)
	if rc < 0 {
		return fmt.Errorf("fbink blit %v: %d", r, rc)
	}
	l.inflight = append(l.inflight, refresh{C.fbink_get_last_marker(), r})
	return nil
}

// waitInflight waits for the oldest refreshes until at most n are left.
func (l *fbinkLib) waitInflight(n int) {
	for len(l.inflight) > n {
		C.fbink_wait_for_complete(l.fd, l.inflight[0].marker)
		l.inflight = l.inflight[1:]
	}
}

func (l *fbinkLib) close() {
	l.waitInflight(0)
	C.fbink_close(l.fd)
}
