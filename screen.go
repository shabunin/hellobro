package main

// The e-ink panel, driven through FBInk: linked in (fbink_lib.go, the fast
// path on the Kobo), or by running our fbink CLI build for every update
// (the fallback, also used by the test harness via HELLOBRO_FBINK).
//
// Either way the UI is rendered in Go and each update is one image blit.
// (On sunxi, a process can only refresh what it drew itself, so the CLI
// path can't compose the screen out of several fbink calls anyway.)

import (
	"bytes"
	"fmt"
	"image"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type screenInfo struct {
	viewW, viewH      int // what we draw into
	screenW, screenH  int // full panel in the current rotation
	originX, originY  int // viewport offset inside the panel
	rota              int // current native rotation (what FBINK_FORCE_ROTA takes)
	canonical         int // FB_ROTATE_* as the device is physically held: 0 UR, 1 CW, 2 UD, 3 CCW
	swap, mirX, mirY  bool
	version, codename string
	device            string // name, platform and ID, as FBInk knows the device
}

func (s screenInfo) landscape() bool { return s.viewW > s.viewH }

func (s screenInfo) String() string {
	return fmt.Sprintf("FBInk %s on %s: view %dx%d (origin %d,%d) screen %dx%d rota=%d canonical=%d touch swap=%v mirX=%v mirY=%v",
		s.version, s.codename, s.viewW, s.viewH, s.originX, s.originY, s.screenW, s.screenH,
		s.rota, s.canonical, s.swap, s.mirX, s.mirY)
}

// fbBackend is one way of talking to FBInk.
type fbBackend interface {
	// setRota forces a native rotation (-1 = follow the gyro) and returns the new state.
	setRota(rota int) (screenInfo, error)
	// blit shows r of img. Waveforms: "GC16" (+flash) for full redraws,
	// "GL16" for UI updates, "DU" for fast black & white ink.
	blit(img *image.Gray, r image.Rectangle, waveform string, flash bool) error
	close()
}

type screen struct {
	be   fbBackend
	info screenInfo

	// blit timing, logged now and then
	blits, pixels  int
	total, slowest time.Duration
	statsAt        time.Time
}

// openScreen asks FBInk for the current (gyro) orientation, then locks it,
// so a mid-session tilt can't flip our drawing.
func openScreen() (*screen, error) {
	be, err := openBackend()
	if err != nil {
		return nil, err
	}
	s := &screen{be: be, statsAt: time.Now()}
	if err := s.setRotation(-1); err != nil {
		return nil, err
	}
	if err := s.setRotation(s.info.rota); err != nil {
		return nil, err
	}
	return s, nil
}

// openBackend prefers the linked-in FBInk, unless HELLOBRO_FBINK names a CLI to run.
func openBackend() (fbBackend, error) {
	if os.Getenv("HELLOBRO_FBINK") == "" {
		be, err := openFbinkLib()
		if err == nil {
			return be, nil
		}
		log.Printf("FBInk library: %v; falling back to the fbink CLI", err)
	}
	return &fbinkCLI{path: findFbink(), rota: -1}, nil
}

func (s *screen) setRotation(rota int) error {
	info, err := s.be.setRota(rota)
	if err != nil {
		return err
	}
	if rota >= 0 && info.rota != rota {
		log.Printf("asked FBInk for rota %d, it reports %d", rota, info.rota)
	}
	s.info = info
	log.Print(info)
	return nil
}

func (s *screen) blit(img *image.Gray, r image.Rectangle, waveform string, flash bool) error {
	r = r.Intersect(img.Bounds())
	if r.Empty() {
		return nil
	}
	start := time.Now()
	err := s.be.blit(img, r, waveform, flash)
	d := time.Since(start)
	s.blits++
	s.pixels += r.Dx() * r.Dy()
	s.total += d
	s.slowest = max(s.slowest, d)
	if time.Since(s.statsAt) > 10*time.Second {
		log.Printf("blits: %d in the last %.0fs, avg %v, slowest %v, avg %d px",
			s.blits, time.Since(s.statsAt).Seconds(), (s.total / time.Duration(s.blits)).Round(time.Millisecond),
			s.slowest.Round(time.Millisecond), s.pixels/s.blits)
		s.blits, s.pixels, s.total, s.slowest, s.statsAt = 0, 0, 0, 0, time.Now()
	}
	return err
}

func (s *screen) close() { s.be.close() }

// --- the fbink CLI ---

type fbinkCLI struct {
	path string
	rota int // passed as FBINK_FORCE_ROTA to every call; -1 = gyro
}

func (c *fbinkCLI) setRota(rota int) (screenInfo, error) {
	c.rota = rota
	out, err := c.command("-e").Output()
	if err != nil {
		return screenInfo{}, fmt.Errorf("fbink -e: %w", err)
	}
	info, err := parseFbinkState(string(out))
	info.version += " (CLI)"
	return info, err
}

func (c *fbinkCLI) blit(img *image.Gray, r image.Rectangle, waveform string, flash bool) error {
	var pgm bytes.Buffer
	fmt.Fprintf(&pgm, "P5\n%d %d\n255\n", r.Dx(), r.Dy())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		off := img.PixOffset(r.Min.X, y)
		pgm.Write(img.Pix[off : off+r.Dx()])
	}
	args := []string{"-q", "-g", fmt.Sprintf("file=-,x=%d,y=%d", r.Min.X, r.Min.Y), "-W", waveform}
	if flash {
		args = append(args, "-f")
	}
	cmd := c.command(args...)
	cmd.Stdin = &pgm
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("fbink blit %v: %w: %s", r, err, out)
	}
	return nil
}

func (c *fbinkCLI) close() {}

func (c *fbinkCLI) command(args ...string) *exec.Cmd {
	cmd := exec.Command(c.path, args...)
	cmd.Env = os.Environ()
	if c.rota >= 0 {
		cmd.Env = append(cmd.Env, "FBINK_FORCE_ROTA="+strconv.Itoa(c.rota))
	}
	return cmd
}

// parseFbinkState reads the "key=value;" dump of fbink -e.
func parseFbinkState(dump string) (screenInfo, error) {
	kv := map[string]string{}
	for _, field := range strings.Split(strings.TrimSpace(dump), ";") {
		if k, v, ok := strings.Cut(field, "="); ok {
			kv[strings.TrimSpace(k)] = strings.Trim(v, "'")
		}
	}
	num := func(k string) int { n, _ := strconv.Atoi(kv[k]); return n }
	s := screenInfo{
		viewW: num("viewWidth"), viewH: num("viewHeight"),
		screenW: num("screenWidth"), screenH: num("screenHeight"),
		originX: num("viewHoriOrigin"), originY: num("viewVertOrigin"),
		rota:    num("currentRota"),
		swap:    kv["touchSwapAxes"] == "1",
		mirX:    kv["touchMirrorX"] == "1",
		mirY:    kv["touchMirrorY"] == "1",
		version: kv["FBINK_VERSION"], codename: kv["deviceCodename"],
		device: fmt.Sprintf("Kobo %s (%s, ID %s)", kv["deviceName"], kv["devicePlatform"], kv["deviceId"]),
	}
	if s.viewW == 0 || s.viewH == 0 {
		return s, fmt.Errorf("unexpected fbink -e output: %q", dump)
	}
	// rotationMap='{ a, b, c, d }': index is native, value is canonical.
	s.canonical = s.rota
	m := strings.Fields(strings.NewReplacer("{", "", "}", "", ",", " ").Replace(kv["rotationMap"]))
	if len(m) == 4 && s.rota >= 0 && s.rota < 4 {
		s.canonical, _ = strconv.Atoi(m[s.rota])
	}
	return s, nil
}

// findFbink returns our image-capable build, shipped next to the binary.
// (KOReader's and KFMon's fbink can't show images.)
func findFbink() string {
	if p := os.Getenv("HELLOBRO_FBINK"); p != "" {
		return p
	}
	exe, err := os.Executable()
	if err != nil {
		return "fbink"
	}
	return filepath.Join(filepath.Dir(exe), "fbink")
}
