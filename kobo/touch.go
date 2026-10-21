package kobo

// Mapping raw touch coordinates to what we draw on.
//
// In normalized panel units, every possible mapping is one of 8 axis
// swaps/mirrors. The default comes from FBInk's touch quirks (which
// describe the upright portrait layout) plus the current rotation, the
// same way KOReader does it. Calibration measures it instead, once per
// rotation, and is saved next to the binary.

import (
	"bufio"
	"fmt"
	"image"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type axes struct{ swap, mirX, mirY bool }

func (a axes) String() string {
	return fmt.Sprintf("swap=%d mirX=%d mirY=%d", b2i(a.swap), b2i(a.mirX), b2i(a.mirY))
}

func (a axes) apply(u, v float64) (float64, float64) {
	if a.swap {
		u, v = v, u
	}
	if a.mirX {
		u = 1 - u
	}
	if a.mirY {
		v = 1 - v
	}
	return u, v
}

type touchMap struct {
	axes
	rx, ry absRange
	w, h   int // view size
	ox, oy int // view origin inside the panel
}

func (m touchMap) norm(raw int32, r absRange) float64 {
	if r.max <= r.min {
		return float64(raw) / float64(max(m.w, m.h)) // no range info: assume panel pixels
	}
	return float64(raw-r.min) / float64(r.max-r.min)
}

// toView maps a raw touch position to view pixels.
func (m touchMap) toView(x, y int32) image.Point {
	u, v := m.apply(m.norm(x, m.rx), m.norm(y, m.ry))
	return image.Pt(int(math.Round(u*float64(m.w+m.ox)))-m.ox, int(math.Round(v*float64(m.h+m.oy)))-m.oy)
}

// defaultAxes composes FBInk's portrait touch quirks with the rotation.
func defaultAxes(s ScreenInfo) axes {
	quirks := axes{s.Swap, s.MirX, s.MirY}
	composed := func(u, v float64) (float64, float64) {
		u, v = quirks.apply(u, v)
		switch s.Canonical { // c.f. KOReader's GestureDetector:translateCoordinates
		case 1:
			u, v = 1-v, u
		case 2:
			u, v = 1-u, 1-v
		case 3:
			u, v = v, 1-u
		}
		return u, v
	}
	return bestAxes(func(a axes) float64 {
		var d float64
		for _, p := range [][2]float64{{0.1, 0.2}, {0.9, 0.3}, {0.4, 0.8}} {
			u1, v1 := a.apply(p[0], p[1])
			u2, v2 := composed(p[0], p[1])
			d += math.Hypot(u1-u2, v1-v2)
		}
		return d
	})
}

// calibrationTargets are deliberately asymmetric, so that no two of the 8
// candidate mappings put both taps near the targets.
var calibrationTargets = [][2]float64{{0.15, 0.3}, {0.75, 0.8}}

// calibrate picks the mapping that best explains where the targets were tapped.
// It returns the mapping and the mean miss in view pixels.
func calibrate(m touchMap, raw []image.Point) (axes, float64) {
	var miss float64
	a := bestAxes(func(a axes) float64 {
		t := m
		t.axes = a
		var d float64
		for i, p := range raw {
			got := t.toView(int32(p.X), int32(p.Y))
			want := image.Pt(int(calibrationTargets[i][0]*float64(m.w)), int(calibrationTargets[i][1]*float64(m.h)))
			d += math.Hypot(float64(got.X-want.X), float64(got.Y-want.Y))
		}
		return d
	})
	t := m
	t.axes = a
	for i, p := range raw {
		got := t.toView(int32(p.X), int32(p.Y))
		miss += math.Hypot(float64(got.X)-calibrationTargets[i][0]*float64(m.w), float64(got.Y)-calibrationTargets[i][1]*float64(m.h))
	}
	return a, miss / float64(len(raw))
}

func bestAxes(score func(axes) float64) axes {
	var best axes
	bestScore := math.Inf(1)
	for i := 0; i < 8; i++ {
		a := axes{i&1 != 0, i&2 != 0, i&4 != 0}
		if s := score(a); s < bestScore {
			best, bestScore = a, s
		}
	}
	return best
}

// --- persistence: one "rota=N swap=0 mirX=1 mirY=0" line per rotation ---

func calibrationPath() string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), "touch-calibration.conf")
	}
	return "touch-calibration.conf"
}

func loadCalibrations() map[int]axes {
	cal := map[int]axes{}
	f, err := os.Open(calibrationPath())
	if err != nil {
		return cal
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		rota, a := -1, axes{}
		for _, field := range strings.Fields(sc.Text()) {
			k, v, _ := strings.Cut(field, "=")
			n, _ := strconv.Atoi(v)
			switch k {
			case "rota":
				rota = n
			case "swap":
				a.swap = n == 1
			case "mirX":
				a.mirX = n == 1
			case "mirY":
				a.mirY = n == 1
			}
		}
		if rota >= 0 {
			cal[rota] = a
		}
	}
	return cal
}

func saveCalibrations(cal map[int]axes) {
	var b strings.Builder
	b.WriteString("# hellobro touch calibration, one line per screen rotation\n")
	for rota := 0; rota < 4; rota++ {
		if a, ok := cal[rota]; ok {
			fmt.Fprintf(&b, "rota=%d %s\n", rota, a)
		}
	}
	if err := os.WriteFile(calibrationPath(), []byte(b.String()), 0o644); err != nil {
		log.Printf("save calibration: %v", err)
	}
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}
