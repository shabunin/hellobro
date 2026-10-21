//go:build cgo

package main

// FBInk linked in (third_party/fbink, built by build-fbink.sh): the screen
// stays open for the whole session, so an update is just a copy and an
// ioctl, instead of starting a process that sets up the display again.

/*
#cgo CFLAGS: -I${SRCDIR}/third_party/fbink
#cgo LDFLAGS: ${SRCDIR}/third_party/fbink/libfbink.a -lm
#include <errno.h>
#include "fbink.h"
*/
import "C"

import (
	"fmt"
	"image"
	"unsafe"
)

type fbinkLib struct {
	fd       C.int
	cfg      C.FBInkConfig
	inflight []refresh // refreshes that may still be running, oldest first
}

type refresh struct {
	marker C.uint32_t
	r      image.Rectangle
}

// maxInflight bounds the refreshes queued in the panel driver. It takes
// updates much faster than it can show them, so without a limit a long
// stroke builds a backlog and the screen falls further and further behind.
const maxInflight = 2

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

func (l *fbinkLib) setRota(rota int) (screenInfo, error) {
	rc := C.fbink_sunxi_ntx_enforce_rota(l.fd, C.SUNXI_FORCE_ROTA_INDEX_T(rota), &l.cfg)
	if rc == -C.ENOSYS { // not a sunxi device: FBInk follows the gyro on its own
		rc = C.fbink_reinit(l.fd, &l.cfg)
	}
	if rc < 0 {
		return screenInfo{}, fmt.Errorf("fbink rotation %d: %d", rota, rc)
	}
	var st C.FBInkState
	C.fbink_get_state(&l.cfg, &st)
	return screenInfo{
		viewW: int(st.view_width), viewH: int(st.view_height),
		screenW: int(st.screen_width), screenH: int(st.screen_height),
		originX: int(st.view_hori_origin), originY: int(st.view_vert_origin),
		rota:      int(st.current_rota),
		canonical: int(st.rotation_map[st.current_rota&3]),
		swap:      bool(st.touch_swap_axes), mirX: bool(st.touch_mirror_x), mirY: bool(st.touch_mirror_y),
		version:  C.GoString(C.fbink_version()) + " (linked in)",
		codename: C.GoString(&st.device_codename[0]),
	}, nil
}

var waveforms = map[string]C.WFM_MODE_INDEX_T{"DU": C.WFM_DU, "GL16": C.WFM_GL16, "GC16": C.WFM_GC16}

func (l *fbinkLib) blit(img *image.Gray, r image.Rectangle, waveform string, flash bool) error {
	pix := make([]byte, r.Dx()*r.Dy())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		off := img.PixOffset(r.Min.X, y)
		copy(pix[(y-r.Min.Y)*r.Dx():], img.Pix[off:off+r.Dx()])
	}
	if flash { // let everything else finish, so it doesn't spoil the full refresh
		l.waitInflight(0)
	}
	// An update over pixels the panel is still driving collides with that
	// refresh: the two get mixed up, and old and new text show on top of
	// each other. Let the overlapping ones finish first.
	for i := len(l.inflight) - 1; i >= 0; i-- {
		if l.inflight[i].r.Overlaps(r) {
			l.waitInflight(len(l.inflight) - i - 1)
			break
		}
	}
	l.waitInflight(maxInflight - 1)
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
