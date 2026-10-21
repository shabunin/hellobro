package main

// The frontlight, the way KOReader drives the Elipsa's: an ioctl on
// /dev/ntx_io taking 0-100. It can't be read back, so we start from the
// level Nickel saved, and put that back when we leave, since Nickel would
// otherwise believe its own level is still on.

import (
	"bufio"
	"fmt"
	"image"
	"image/draw"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const ntxSetLight = 241 // ioctl on /dev/ntx_io: set the frontlight, 0-100

var nickelConf = envOr("HELLOBRO_NICKEL_CONF", "/mnt/onboard/.kobo/Kobo/Kobo eReader.conf")

type frontlight struct {
	f       *os.File
	level   int // what we set last
	initial int // Nickel's, restored on close
	from    string
}

func openFrontlight() *frontlight {
	f, err := os.OpenFile(envOr("HELLOBRO_NTX_IO", "/dev/ntx_io"), os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		log.Printf("frontlight: %v", err)
		return nil
	}
	l := &frontlight{f: f, level: 50, from: "a guess"}
	if lvl, ok := nickelLightLevel(); ok {
		l.level, l.from = lvl, "Nickel's settings"
	} else if lvl, ok := sysfsBrightness(); ok {
		l.level, l.from = lvl, "sysfs"
	}
	l.initial = l.level
	log.Printf("frontlight: %d%% (from %s)", l.level, l.from)
	return l
}

// set changes the light, 0 (off) to 100.
func (l *frontlight) set(level int) error {
	level = max(0, min(100, level))
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, l.f.Fd(), ntxSetLight, uintptr(level))
	if errno == syscall.ENOTTY && os.Getenv("HELLOBRO_NTX_IO") != "" {
		errno = 0 // a stand-in file, for testing
	}
	if errno != 0 {
		return fmt.Errorf("frontlight ioctl: %w", errno)
	}
	l.level = level
	return nil
}

func (l *frontlight) close() {
	if l.level != l.initial {
		log.Printf("frontlight: back to Nickel's %d%%", l.initial)
		l.set(l.initial)
	}
	l.f.Close()
}

// nickelLightLevel reads [PowerOptions] FrontLightLevel (and FrontLightState).
func nickelLightLevel() (int, bool) {
	f, err := os.Open(nickelConf)
	if err != nil {
		return 0, false
	}
	defer f.Close()
	section, level, on, found := "", 0, true, false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "[") {
			section = line
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok || section != "[PowerOptions]" {
			continue
		}
		switch k {
		case "FrontLightLevel":
			if n, err := strconv.Atoi(v); err == nil {
				level, found = n, true
			}
		case "FrontLightState":
			on = v != "false"
		}
	}
	if !on {
		level = 0
	}
	return level, found
}

// sysfsBrightness reads a backlight's level as a percentage, if the kernel offers one.
func sysfsBrightness() (int, bool) {
	dirs, _ := filepath.Glob("/sys/class/backlight/*")
	for _, d := range dirs {
		read := func(name string) int {
			b, _ := os.ReadFile(filepath.Join(d, name))
			n, _ := strconv.Atoi(strings.TrimSpace(string(b)))
			return n
		}
		cur, maxB := read("actual_brightness"), read("max_brightness")
		if cur == 0 {
			cur = read("brightness")
		}
		log.Printf("frontlight: %s: %d of %d", d, cur, maxB)
		if maxB > 0 {
			return cur * 100 / maxB, true
		}
	}
	return 0, false
}

// --- the swipe's gauge ---

// lightGaugeRect sits right of the finger, centered vertically.
func (a *app) lightGaugeRect() image.Rectangle {
	cy := a.c.Bounds().Dy() / 2
	return image.Rect(lightEdge+20, cy-260, lightEdge+130, cy+260)
}

func (a *app) setLight(level int) {
	level = max(0, min(100, level))
	if level == a.light.level && a.lightOverlay != nil {
		return
	}
	if err := a.light.set(level); err != nil {
		log.Print(err)
	}
	a.drawLightGauge()
}

func (a *app) drawLightGauge() {
	a.drawGauge(&a.lightOverlay, a.lightGaugeRect(), a.light.level, 100, fmt.Sprintf("%d%%", a.light.level))
}

func (a *app) hideLightGauge() { a.hideGauge(&a.lightOverlay) }

// drawGauge shows a bar filled n/of in r, labeled, saving what it covers
// in *under the first time.
func (a *app) drawGauge(under **image.Gray, r image.Rectangle, n, of int, label string) {
	if *under == nil {
		*under = image.NewGray(r)
		draw.Draw(*under, r, a.c.Gray, r.Min, draw.Src)
	}
	c := a.c
	c.fill(r, white)
	c.frame(r, 3, black)
	bar := image.Rect(r.Min.X+20, r.Min.Y+20, r.Max.X-20, r.Max.Y-70)
	c.frame(bar, 2, black)
	in := bar.Inset(6)
	c.fill(image.Rect(in.Min.X, in.Max.Y-in.Dy()*n/of, in.Max.X, in.Max.Y), black)
	c.text(a.f.panel, image.Rect(r.Min.X, r.Max.Y-64, r.Max.X, r.Max.Y-10), label, black, alignCenter)
	a.mark(r, false) // black & white only, so the fast waveform is fine
}

// hideGauge puts back what the gauge covered.
func (a *app) hideGauge(under **image.Gray) {
	if *under == nil {
		return
	}
	r := (*under).Bounds()
	draw.Draw(a.c.Gray, r, *under, r.Min, draw.Src)
	*under = nil
	a.mark(r, true)
}

func (a *app) lightInfo() string {
	if a.light == nil {
		return "frontlight: not available"
	}
	return fmt.Sprintf("frontlight: %d%% (started at %d%%, from %s)", a.light.level, a.light.initial, a.light.from)
}
