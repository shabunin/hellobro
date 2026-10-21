package main

// The terminal tabs' status bar, along the bottom, fullscreen too: what's
// playing on the left; the battery, its temperature, CPU and memory use
// on the right. Ctrl+Shift+B shows or hides it (remembered in
// status-bar), a tap on it plays or pauses.

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"hellobro/kobo"
)

const (
	statH      = 34
	statEvery  = 5 * time.Second
	statBarTxt = "status-bar"
)

type statBar struct {
	off         bool            // hidden
	r           image.Rectangle // on screen; empty when hidden
	shown       string          // what's on screen
	at          time.Time       // the last sample
	sys         string          // the right side, as last sampled
	idle, total uint64          // /proc/stat's CPU counters at the last sample
	bat         string          // the battery's sysfs folder, once found
}

func (a *bro) loadStatBar() {
	b, _ := os.ReadFile(kobo.AppFile(statBarTxt))
	a.stat.off = strings.TrimSpace(string(b)) == "off"
}

// layoutStat takes the bar's strip off the bottom of the terminal panel.
func (a *bro) layoutStat() {
	s := &a.stat
	s.r, s.shown = image.Rectangle{}, ""
	if s.off {
		return
	}
	b := &a.termP.Body
	s.r = image.Rect(b.Min.X, b.Max.Y-statH, b.Max.X, b.Max.Y)
	b.Max.Y = s.r.Min.Y
}

func (a *bro) toggleStat() {
	a.stat.off = !a.stat.off
	state := "on"
	if a.stat.off {
		state = "off"
	}
	os.WriteFile(kobo.AppFile(statBarTxt), []byte(state+"\n"), 0o644)
	a.Layout(a.lay)
	if a.term() != nil {
		a.drawTerm()
		a.MarkFlash(a.termP.Outer)
	}
}

// statTick samples the system every few seconds.
func (a *bro) statTick() {
	if s := &a.stat; !s.r.Empty() && time.Since(s.at) >= statEvery {
		s.sample()
		a.drawStat(false)
	}
}

// drawStat draws the bar if what it says changed, or anyway.
func (a *bro) drawStat(force bool) {
	s := &a.stat
	if s.r.Empty() || !isTermTab(a.Tab) || a.Calibrating() || a.GaugeUp() || a.picker != nil {
		return
	}
	if s.at.IsZero() {
		s.sample()
	}
	left := a.nowPlaying()
	if left == "" {
		left = "♪  Ctrl+Shift+Space: play"
	}
	if text := left + "\x00" + s.sys; text != s.shown || force {
		s.shown = text
		c := a.C
		c.Fill(s.r, kobo.White)
		c.Fill(image.Rect(s.r.Min.X, s.r.Min.Y, s.r.Max.X, s.r.Min.Y+2), kobo.Black)
		t := image.Rect(s.r.Min.X+12, s.r.Min.Y+2, s.r.Max.X-12, s.r.Max.Y)
		c.Text(a.F.Small, t, s.sys, kobo.Black, kobo.AlignRight)
		t.Max.X -= kobo.TextWidth(a.F.Small, s.sys) + 32
		c.Text(a.F.Small, t, left, kobo.Black, kobo.AlignLeft)
		a.Mark(s.r, true)
	}
}

// sample reads the battery, CPU and memory.
func (s *statBar) sample() {
	first := s.at.IsZero()
	s.at = time.Now()
	var parts []string
	if s.bat == "" {
		s.bat = batteryDir()
	}
	if c, err := os.ReadFile(s.bat + "/capacity"); err == nil {
		b := "bat " + strings.TrimSpace(string(c)) + "%"
		if st, _ := os.ReadFile(s.bat + "/status"); strings.TrimSpace(string(st)) == "Charging" {
			b += "↑"
		}
		if t, err := readInt(s.bat + "/temp"); err == nil {
			b += fmt.Sprintf(" %d °C", (t+5)/10) // tenths of a degree
		}
		parts = append(parts, b)
	}
	if idle, total, ok := cpuTimes(); ok {
		cpu := "cpu …"
		if !first && total > s.total {
			busy := 1 - float64(idle-s.idle)/float64(total-s.total)
			cpu = fmt.Sprintf("cpu %d%%", int(busy*100+0.5))
		}
		s.idle, s.total = idle, total
		parts = append(parts, cpu)
	}
	if mt, ma := memInfo(); mt > 0 {
		parts = append(parts, fmt.Sprintf("ram %d%%", int(100*(mt-ma)/mt)))
	}
	s.sys = strings.Join(parts, "  ·  ")
}

// batteryDir is the power supply that is a battery.
func batteryDir() string {
	dirs, _ := filepath.Glob("/sys/class/power_supply/*")
	for _, d := range dirs {
		if t, _ := os.ReadFile(d + "/type"); strings.TrimSpace(string(t)) == "Battery" {
			return d
		}
	}
	return "/sys/class/power_supply/battery"
}

// cpuTimes is /proc/stat's idle time (iowait too) and all time, in ticks.
func cpuTimes() (idle, total uint64, ok bool) {
	b, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0, false
	}
	line, _, _ := strings.Cut(string(b), "\n")
	f := strings.Fields(line)
	if len(f) < 6 || f[0] != "cpu" {
		return 0, 0, false
	}
	for i, v := range f[1:min(len(f), 9)] { // user nice system idle iowait irq softirq steal
		n, _ := strconv.ParseUint(v, 10, 64)
		total += n
		if i == 3 || i == 4 {
			idle += n
		}
	}
	return idle, total, true
}

// memInfo is MemTotal and MemAvailable, in kB.
func memInfo() (total, avail int64) {
	b, _ := os.ReadFile("/proc/meminfo")
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		n, _ := strconv.ParseInt(f[1], 10, 64)
		switch f[0] {
		case "MemTotal:":
			total = n
		case "MemAvailable:":
			avail = n
		}
	}
	return total, avail
}

func readInt(path string) (int64, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
}
