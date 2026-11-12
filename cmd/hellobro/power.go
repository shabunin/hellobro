package main

// The Battery and What uses it windows, a powertop of sorts. Battery:
// what it has, what it's giving now, how fast it goes, the busiest
// processes, and its last hours on a graph. What uses
// it: the CPU (how busy, how fast, how deep it sleeps), the processes
// that keep it busy, the wakeups and whose they are, the radios and the
// light. It samples every few seconds while it shows; the battery's
// history is a sample a minute while hellobro runs, kept in
// battery-history.

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"hellobro/kobo"
)

const (
	powerEvery = 5 * time.Second
	powerCalm  = 30 * time.Second // and this often, untouched a minute: each sample costs (powerSample)
	histEvery  = time.Minute
	histTxt    = "battery-history"
	histKeep   = 48 * time.Hour
	graphSpan  = 12 * time.Hour
	clkTck     = 100 // /proc's ticks a second
)

type powerState struct {
	bat      battery
	use      usage
	prev     *usageCounts // the last sample, for what changed since
	at       time.Time    // of the last sample
	hist     []batSample
	histAt   time.Time
	plugged  bool // charging, or full, at the last sample
	histErr  bool
	loaded   bool
	sampling bool // a sample is being read
}

// battery is the battery as sysfs has it now.
type battery struct {
	ok              bool
	pct             int
	ma              int // the current, in mA: drawn, or taken in while charging
	mv              int
	temp            int // tenths of a degree
	status, health  string
	toEmpty, toFull time.Duration
}

func readBattery(dir string) battery {
	var b battery
	pct, err := readInt(dir + "/capacity")
	if err != nil {
		return b
	}
	b.ok, b.pct = true, int(pct)
	str := func(f string) string { s, _ := os.ReadFile(dir + "/" + f); return strings.TrimSpace(string(s)) }
	num := func(f string) int64 { n, _ := readInt(dir + "/" + f); return n }
	b.status, b.health = str("status"), str("health")
	// The Kobo's gauge (ricoh61x) gives tenths of a mA: its time to empty
	// times this is a full battery, as it should be.
	b.ma = int(abs64(num("current_now")) / 10)
	b.mv = int(num("voltage_now") / 1000)
	b.temp = int(num("temp"))
	b.toEmpty = time.Duration(num("time_to_empty_now")) * time.Second
	b.toFull = time.Duration(num("time_to_full_now")) * time.Second
	return b
}

func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

func (b battery) charging() bool { return b.status == "Charging" || b.status == "Full" }

// --- the history ---

type batSample struct {
	at          time.Time
	pct, ma, mv int // mv: 0 if not known (older lines)
	charging    bool
}

func (s batSample) String() string {
	c := "d"
	if s.charging {
		c = "c"
	}
	return fmt.Sprintf("%d %d %d %s %d", s.at.Unix(), s.pct, s.ma, c, s.mv)
}

// parseHistory reads battery-history: "unix pct mA c|d mV" a line, the
// ones since after kept. The voltage is there to check the percent by:
// the gauge's percent drifts (it once said 12% of a battery near full),
// the voltage doesn't. Older lines have no voltage.
func parseHistory(s string, after time.Time) []batSample {
	var h []batSample
	for _, line := range strings.Split(s, "\n") {
		f := strings.Fields(line)
		if len(f) != 4 && len(f) != 5 {
			continue
		}
		t, err1 := strconv.ParseInt(f[0], 10, 64)
		pct, err2 := strconv.Atoi(f[1])
		ma, err3 := strconv.Atoi(f[2])
		if err1 != nil || err2 != nil || err3 != nil || time.Unix(t, 0).Before(after) {
			continue
		}
		s := batSample{at: time.Unix(t, 0), pct: pct, ma: ma, charging: f[3] == "c"}
		if len(f) == 5 {
			s.mv, _ = strconv.Atoi(f[4])
		}
		h = append(h, s)
	}
	return h
}

func (a *bro) loadHistory() {
	ps := &a.pw
	ps.loaded = true
	path := kobo.AppFile(histTxt)
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	ps.hist = parseHistory(string(b), time.Now().Add(-histKeep))
	if n := strings.Count(string(b), "\n"); n > len(ps.hist)+60 { // an hour's worth gone stale: write it short
		var w strings.Builder
		for _, s := range ps.hist {
			w.WriteString(s.String() + "\n")
		}
		if err := os.WriteFile(path+".new", []byte(w.String()), 0o644); err == nil {
			os.Rename(path+".new", path)
		}
	}
}

// histTick adds the battery to its history every minute, whatever the tab.
func (a *bro) histTick() {
	ps := &a.pw
	if time.Since(ps.histAt) < histEvery {
		return
	}
	ps.histAt = time.Now()
	if !ps.loaded {
		a.loadHistory()
	}
	if a.stat.bat == "" {
		a.stat.bat = batteryDir()
	}
	b := readBattery(a.stat.bat)
	if !b.ok {
		return
	}
	s := batSample{at: time.Now(), pct: b.pct, ma: b.ma, mv: b.mv, charging: b.charging()}
	ps.plugged = s.charging
	a.batteryCheck(b.pct, s.charging)
	ps.hist = append(ps.hist, s)
	if cut := slices.IndexFunc(ps.hist, func(s batSample) bool { return time.Since(s.at) < histKeep }); cut > 0 {
		ps.hist = ps.hist[cut:]
	}
	f, err := os.OpenFile(kobo.AppFile(histTxt), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err == nil {
		_, err = f.WriteString(s.String() + "\n")
		f.Close()
	}
	if err != nil && !ps.histErr {
		ps.histErr = true
		log.Printf("power: %v", err)
	}
}

// drainRate is how fast the battery went (%/h, up while charging) over
// the run of samples up to the last one, of the same kind, at most 3 h
// back: once that's 20 min or a percent and more.
func drainRate(h []batSample) (rate float64, over time.Duration, ok bool) {
	if len(h) < 2 {
		return 0, 0, false
	}
	last := h[len(h)-1]
	first := last
	for i := len(h) - 2; i >= 0; i-- {
		s := h[i]
		if s.charging != last.charging || last.at.Sub(s.at) > 3*time.Hour || first.at.Sub(s.at) > 15*time.Minute {
			break // a change, too far back, or a gap: hellobro was off
		}
		first = s
	}
	over = last.at.Sub(first.at)
	moved := last.pct - first.pct
	if over < 20*time.Minute && kobo.Abs(moved) < 1 || over < 5*time.Minute {
		return 0, over, false
	}
	return float64(moved) / over.Hours(), over, true
}

// --- what uses it ---

// usageCounts are the system's counters at a moment.
type usageCounts struct {
	at          time.Time
	idle, total uint64            // /proc/stat
	freq        map[int]uint64    // cpufreq time in state, by kHz
	cstate      map[string]uint64 // cpu0's time in its idle states, µs
	procs       map[int]procTicks
	irqs        map[string]uint64
}

type procTicks struct {
	name  string
	ticks uint64
}

// usage is what changed between two samples.
type usage struct {
	ok      bool
	busy    float64 // the CPU, 0–1
	cores   string  // online, as sysfs says it
	load    string
	freqs   []ranked
	cstates []ranked
	procs   []ranked // percent of one core
	wakeups float64  // a second
	irqs    []ranked // a second
}

type ranked struct {
	name string
	v    float64
}

func countUsage() *usageCounts {
	c := &usageCounts{at: time.Now(), freq: map[int]uint64{}, cstate: map[string]uint64{}, procs: map[int]procTicks{}}
	c.idle, c.total, _ = cpuTimes()
	if b, err := os.ReadFile("/sys/devices/system/cpu/cpu0/cpufreq/stats/time_in_state"); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if f := strings.Fields(line); len(f) == 2 {
				k, _ := strconv.Atoi(f[0])
				n, _ := strconv.ParseUint(f[1], 10, 64)
				c.freq[k] = n
			}
		}
	}
	states, _ := filepath.Glob("/sys/devices/system/cpu/cpu0/cpuidle/state*")
	for _, d := range states {
		name, _ := os.ReadFile(d + "/name")
		if t, err := readInt(d + "/time"); err == nil {
			c.cstate[strings.TrimSpace(string(name))] = uint64(t)
		}
	}
	pids, _ := filepath.Glob("/proc/[0-9]*/stat")
	for _, p := range pids {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if pid, pt, ok := parseProcStat(string(b)); ok {
			c.procs[pid] = pt
		}
	}
	b, _ := os.ReadFile("/proc/interrupts")
	c.irqs = parseInterrupts(string(b))
	return c
}

// parseProcStat reads /proc/PID/stat: the pid, the name, and its user
// and system time.
func parseProcStat(s string) (int, procTicks, bool) {
	open, close := strings.IndexByte(s, '('), strings.LastIndexByte(s, ')')
	if open < 0 || close < open {
		return 0, procTicks{}, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(s[:open]))
	f := strings.Fields(s[close+1:])
	if err != nil || len(f) < 13 {
		return 0, procTicks{}, false
	}
	ut, _ := strconv.ParseUint(f[11], 10, 64)
	st, _ := strconv.ParseUint(f[12], 10, 64)
	return pid, procTicks{s[open+1 : close], ut + st}, true
}

// irqNames are the Kobo's interrupts by what they are, where that's sure.
var irqNames = map[string]string{
	"arch_timer": "timer", "sun4i_timer0": "timer", "elan_ktf": "touch", "kx122": "tilt sensor",
	"ricoh619": "power chip", "dispaly": "screen", "Power": "power button", "Hall": "cover",
	"Rescheduling interrupts": "rescheduling", "Function call interrupts": "function calls",
}

// parseInterrupts reads /proc/interrupts: each source's count, all CPUs'.
func parseInterrupts(s string) map[string]uint64 {
	m := map[string]uint64{}
	for _, line := range strings.Split(s, "\n") {
		label, rest, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		f := strings.Fields(rest)
		var n uint64
		i := 0
		for ; i < len(f); i++ {
			v, err := strconv.ParseUint(f[i], 10, 64)
			if err != nil {
				break
			}
			n += v
		}
		desc := f[i:]
		if _, err := strconv.Atoi(strings.TrimSpace(label)); err == nil && len(desc) > 3 {
			desc = desc[3:] // the chip, its number, edge or level: then the name
		}
		name := strings.Join(desc, " ")
		if name == "" {
			continue
		}
		if friendly, ok := irqNames[name]; ok {
			name = friendly
		}
		m[name] += n
	}
	return m
}

// usageSince is what changed from p to c.
func usageSince(p, c *usageCounts) usage {
	u := usage{ok: true}
	if c.total > p.total {
		u.busy = 1 - float64(c.idle-p.idle)/float64(c.total-p.total)
	}
	secs := c.at.Sub(p.at).Seconds()
	if secs <= 0 {
		return usage{}
	}
	var all float64
	for k, n := range c.freq {
		all += float64(n - min(n, p.freq[k]))
	}
	for k, n := range c.freq {
		if d := float64(n - min(n, p.freq[k])); d > 0 && all > 0 {
			u.freqs = append(u.freqs, ranked{mhz(k), d / all})
		}
	}
	for k, n := range c.cstate {
		if d := float64(n - min(n, p.cstate[k])); d > 0 {
			u.cstates = append(u.cstates, ranked{k, min(1, d/(secs*1e6))})
		}
	}
	for pid, pt := range c.procs {
		if d := pt.ticks - min(pt.ticks, p.procs[pid].ticks); d > 0 && p.procs[pid].name != "" {
			u.procs = append(u.procs, ranked{pt.name, float64(d) / (secs * clkTck) * 100})
		}
	}
	for k, n := range c.irqs {
		if d := float64(n - min(n, p.irqs[k])); d > 0 {
			u.wakeups += d / secs
			u.irqs = append(u.irqs, ranked{k, d / secs})
		}
	}
	for _, s := range [][]ranked{u.freqs, u.cstates, u.procs, u.irqs} {
		slices.SortFunc(s, func(a, b ranked) int {
			if a.v != b.v {
				return map[bool]int{true: -1, false: 1}[a.v > b.v]
			}
			return strings.Compare(a.name, b.name)
		})
	}
	return u
}

func mhz(khz int) string {
	if khz >= 1000000 {
		return strconv.FormatFloat(float64(khz)/1e6, 'f', -1, 64) + " GHz"
	}
	return strconv.Itoa(khz/1000) + " MHz"
}

// --- the tab ---

func (a *bro) showingPower() bool { return a.guiShows("battery") || a.guiShows("usage") }

func (a *bro) enterPower() {
	a.powerSample()
}

// powerTick samples and draws, every few seconds while the tab shows.
func (a *bro) powerTick() {
	a.histTick()
	if a.showingPower() && time.Since(a.pw.at) >= a.powerEvery() {
		a.powerSample()
	}
}

// powerEvery is how often the windows sample: often while they're used,
// less once nothing's been touched a minute. A sample reads some 150
// files and draws, a good part of what hellobro does when idle.
func (a *bro) powerEvery() time.Duration {
	if a.IdleFor() > time.Minute {
		return powerCalm
	}
	return powerEvery
}

// powerSample reads the battery and what uses it in the background: that's
// some 150 files in /proc, 60 ms on the Kobo, too long for the UI's thread.
// The page shows the last sample meanwhile, and is drawn again with this one.
func (a *bro) powerSample() {
	ps := &a.pw
	if ps.sampling {
		return
	}
	if !ps.loaded {
		a.loadHistory()
	}
	if a.stat.bat == "" {
		a.stat.bat = batteryDir()
	}
	ps.sampling = true
	dir, prev := a.stat.bat, ps.prev
	go func() {
		at := time.Now()
		bat := readBattery(dir)
		c := countUsage()
		var use usage
		if prev != nil {
			use = usageSince(prev, c)
			cores, _ := os.ReadFile("/sys/devices/system/cpu/online")
			load, _ := os.ReadFile("/proc/loadavg")
			use.cores = strings.TrimSpace(string(cores))
			if f := strings.Fields(string(load)); len(f) > 0 {
				use.load = f[0]
			}
		}
		a.Do(func() {
			ps.sampling = false
			ps.at, ps.bat, ps.prev = at, bat, c
			if prev != nil {
				ps.use = use
			}
			a.drawPower()
		})
	}()
}

// drawPower asks the windows for a frame: a sample came.
func (a *bro) drawPower() {
	a.inkRedraw("battery")
	a.inkRedraw("usage")
}

// powerDevices are the things that draw besides the CPU: the radios and
// the light.
func (a *bro) powerDevices() string {
	var parts []string
	wifi := "Wi-Fi off"
	if ws := a.net.ws; ws.On {
		wifi = "Wi-Fi on"
		if ws.State == "COMPLETED" {
			wifi += " (" + signalWords(ws.Signal) + ")"
		}
	}
	parts = append(parts, wifi)
	bt := "Bluetooth off"
	if _, err := os.Stat("/sys/class/bluetooth/hci0"); err == nil {
		bt = "Bluetooth on"
	}
	parts = append(parts, bt)
	if l, ok := a.Light(); ok {
		light := "light off"
		if l > 0 {
			light = fmt.Sprintf("light %d%%", l)
		}
		parts = append(parts, light)
	}
	if a.share.srv != nil {
		parts = append(parts, "sharing on")
	}
	if a.relayOn() {
		parts = append(parts, "relay on")
	}
	return strings.Join(parts, " · ")
}

func shares(s []ranked, n int, val func(float64) string) string {
	var out []string
	for _, x := range s[:min(n, len(s))] {
		out = append(out, x.name+" "+val(x.v))
	}
	if len(out) == 0 {
		return "—"
	}
	return strings.Join(out, " · ")
}

func pct(v float64) string { return fmt.Sprintf("%d%%", int(v*100+0.5)) }

func hoursMins(d time.Duration) string {
	d = d.Round(time.Minute)
	if d < time.Hour {
		return fmt.Sprintf("%d min", int(d.Minutes()))
	}
	return fmt.Sprintf("%d h %d min", int(d.Hours()), int(d.Minutes())%60)
}

func nonEmpty(ss ...string) []string {
	return slices.DeleteFunc(ss, func(s string) bool { return s == "" })
}
