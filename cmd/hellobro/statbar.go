package main

// What the bar says of the system, at its right, before the clock: the
// battery, its temperature, CPU and memory use, read every few seconds
// (there was a status bar of its own along the bottom for it once).
// Meta+Shift+B (the command statusbar, or its key in keys.conf) hides the
// bar or shows it, remembered in bar; a tap on what's playing in it plays
// or pauses.

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"hellobro/kobo"
)

const (
	statEvery = 5 * time.Second
	barTxt    = "bar"
)

type statBar struct {
	at          time.Time // the last sample
	sys         string    // what it says, as last sampled
	idle, total uint64    // /proc/stat's CPU counters at the last sample
	bat         string    // the battery's sysfs folder, once found
}

// loadBar has the bar hidden if it was. Before anything is laid out.
func (a *bro) loadBar() {
	b, _ := os.ReadFile(kobo.AppFile(barTxt))
	a.BarOff = strings.TrimSpace(string(b)) == "off"
}

// toggleBar hides the bar, or shows it: the windows have its room
// meanwhile.
func (a *bro) toggleBar() {
	state := "off"
	if a.BarOff {
		state = "on"
	}
	os.WriteFile(kobo.AppFile(barTxt), []byte(state+"\n"), 0o644)
	a.SetBarOff(!a.BarOff)
}

// statTick samples the system every few seconds, for the bar.
func (a *bro) statTick() {
	if s := &a.stat; time.Since(s.at) >= statEvery {
		s.sample()
		a.SetSystem(s.sys)
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
		b := strings.TrimSpace(string(c)) + "%"
		if st, _ := os.ReadFile(s.bat + "/status"); strings.TrimSpace(string(st)) == "Charging" {
			b += "↑"
		}
		if t, err := readInt(s.bat + "/temp"); err == nil {
			b += fmt.Sprintf(" %d°C", (t+5)/10) // tenths of a degree
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
	if d := os.Getenv("HELLOBRO_BATTERY"); d != "" { // the harness's stand-in
		return d
	}
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
