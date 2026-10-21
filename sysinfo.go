package main

// The System tab: what this Kobo is and runs, gathered from /proc, /sys
// and the tools it has, the way one would by hand over a shell.

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	sysDevice = iota
	sysSystem
	sysStorage
	sysSoftware
	numSysPanels
)

type sysState struct {
	lines [numSysPanels][]string
	busy  bool
	at    time.Time
}

func (a *app) startSys() {
	if a.sys.busy {
		return
	}
	a.sys.busy = true
	if a.tab == tabSystem && a.calib == nil {
		a.drawSystem()
	}
	info := a.scr.info
	go func() {
		r := sysReport(info)
		a.call <- func() {
			a.sys = sysState{lines: r, at: time.Now()}
			log.Printf("system report:\n\t%s\n\t%s\n\t%s\n\t%s", joinLines(r[0]), joinLines(r[1]), joinLines(r[2]), joinLines(r[3]))
			if a.tab == tabSystem && a.calib == nil {
				a.drawSystem()
			}
		}
	}()
}

func (a *app) drawSystem() {
	for i, p := range a.L.sys {
		status := ""
		if i == sysDevice {
			switch {
			case a.sys.busy:
				status = "looking…"
			case !a.sys.at.IsZero():
				status = "as of " + a.sys.at.Format("15:04:05")
			}
		}
		a.drawPanel(p, status)
		a.drawText(p, a.f.small, a.sys.lines[i], "…", true)
	}
}

func sysReport(s screenInfo) [numSysPanels][]string {
	return [numSysPanels][]string{deviceLines(s), systemLines(), storageLines(), softwareLines(s)}
}

// --- device ---

func deviceLines(s screenInfo) []string {
	lines := []string{s.device}
	// serial, ?, firmware, ?, ?, model ID; the serial stays private.
	if b, err := os.ReadFile("/mnt/onboard/.kobo/version"); err == nil {
		f := strings.Split(strings.TrimSpace(string(b)), ",")
		if len(f) >= 6 {
			lines = append(lines, "firmware "+f[2], "model ID "+f[5])
		}
	} else {
		lines = append(lines, "no /mnt/onboard/.kobo/version: not a Kobo?")
	}
	if hw := cpuinfo()["Hardware"]; hw != "" {
		lines = append(lines, "SoC "+hw)
	}
	lines = append(lines, fmt.Sprintf("screen %d×%d px, codename %s", s.screenW, s.screenH, s.codename))
	if b, err := os.ReadFile("/proc/device-tree/model"); err == nil {
		lines = append(lines, "device tree: "+strings.Trim(string(b), "\x00\n "))
	}
	return lines
}

// --- system ---

func whoami() string {
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	return "uid " + strconv.Itoa(os.Getuid())
}

func systemLines() []string {
	host, _ := os.Hostname()
	lines := []string{"hostname " + host}

	id := fmt.Sprintf("uid=%d(%s) gid=%d", os.Getuid(), whoami(), os.Getgid())
	if g, err := user.LookupGroupId(strconv.Itoa(os.Getgid())); err == nil {
		id += "(" + g.Name + ")"
	}
	if gs, err := os.Getgroups(); err == nil && len(gs) > 0 {
		var names []string
		for _, n := range gs {
			name := strconv.Itoa(n)
			if g, err := user.LookupGroupId(name); err == nil {
				name += "(" + g.Name + ")"
			}
			names = append(names, name)
		}
		id += " groups=" + strings.Join(names, ",")
	}
	lines = append(lines, id)

	var u syscall.Utsname
	if syscall.Uname(&u) == nil {
		lines = append(lines, fmt.Sprintf("%s %s %s", utsString(u.Sysname[:]), utsString(u.Release[:]), utsString(u.Machine[:])),
			"   "+utsString(u.Version[:]))
	}

	if b, err := os.ReadFile("/proc/uptime"); err == nil {
		if f := strings.Fields(string(b)); len(f) > 0 {
			secs, _ := strconv.ParseFloat(f[0], 64)
			up := (time.Duration(secs) * time.Second).Round(time.Minute)
			load, _ := os.ReadFile("/proc/loadavg")
			l := strings.Fields(string(load))
			if len(l) >= 4 {
				lines = append(lines, fmt.Sprintf("up %s, load %s %s %s, tasks %s", fmtDuration(up), l[0], l[1], l[2], l[3]))
			}
		}
	}

	ci := cpuinfo()
	model := ci["model name"]
	if model == "" {
		model = ci["Processor"]
	}
	cpu := fmt.Sprintf("%d × %s", runtime.NumCPU(), model)
	if on, err := os.ReadFile("/sys/devices/system/cpu/online"); err == nil {
		all, _ := os.ReadFile("/sys/devices/system/cpu/present")
		cpu += fmt.Sprintf(" (online %s of %s)", strings.TrimSpace(string(on)), strings.TrimSpace(string(all)))
	}
	if cur := readInt("/sys/devices/system/cpu/cpu0/cpufreq/scaling_cur_freq"); cur > 0 {
		cpu += fmt.Sprintf(", %d MHz", cur/1000)
		if mx := readInt("/sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_max_freq"); mx > 0 {
			cpu += fmt.Sprintf(" (max %d)", mx/1000)
		}
		if g, err := os.ReadFile("/sys/devices/system/cpu/cpu0/cpufreq/scaling_governor"); err == nil {
			cpu += ", " + strings.TrimSpace(string(g))
		}
	}
	lines = append(lines, "CPU "+cpu)

	mi := meminfo()
	mem := fmt.Sprintf("memory %s, %s available", fmtBytes(mi["MemTotal"]), fmtBytes(mi["MemAvailable"]))
	if mi["SwapTotal"] > 0 {
		mem += fmt.Sprintf(", swap %s", fmtBytes(mi["SwapTotal"]))
	}
	lines = append(lines, mem)

	if ifs, err := net.Interfaces(); err == nil {
		var addrs []string
		for _, i := range ifs {
			if i.Flags&net.FlagLoopback != 0 || i.Flags&net.FlagUp == 0 {
				continue
			}
			as, _ := i.Addrs()
			for _, ad := range as {
				if n, ok := ad.(*net.IPNet); ok && n.IP.To4() != nil {
					addrs = append(addrs, i.Name+" "+n.IP.String())
				}
			}
		}
		if len(addrs) == 0 {
			addrs = []string{"none (Wi-Fi off?)"}
		}
		lines = append(lines, "IPv4 "+strings.Join(addrs, ", "))
	}
	return lines
}

func utsString[T int8 | uint8](b []T) string {
	var s []byte
	for _, c := range b {
		if c == 0 {
			break
		}
		s = append(s, byte(c))
	}
	return string(s)
}

func cpuinfo() map[string]string {
	m := map[string]string{}
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return m
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if k, v, ok := strings.Cut(sc.Text(), ":"); ok {
			if k = strings.TrimSpace(k); m[k] == "" {
				m[k] = strings.TrimSpace(v)
			}
		}
	}
	return m
}

// meminfo is /proc/meminfo, in bytes.
func meminfo() map[string]int64 {
	m := map[string]int64{}
	b, _ := os.ReadFile("/proc/meminfo")
	for _, line := range strings.Split(string(b), "\n") {
		if f := strings.Fields(line); len(f) >= 2 {
			n, _ := strconv.ParseInt(f[1], 10, 64)
			m[strings.TrimSuffix(f[0], ":")] = n * 1024
		}
	}
	return m
}

func readInt(path string) int64 {
	b, _ := os.ReadFile(path)
	n, _ := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	return n
}

func fmtBytes(n int64) string {
	switch {
	case n >= 10<<30:
		return fmt.Sprintf("%.0f GB", float64(n)/(1<<30))
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.0f MB", float64(n)/(1<<20))
	}
	return fmt.Sprintf("%d kB", n>>10)
}

func fmtDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	s := fmt.Sprintf("%d:%02d", int(d.Hours())%24, int(d.Minutes())%60)
	switch {
	case days == 1:
		s = "1 day " + s
	case days > 1:
		s = fmt.Sprintf("%d days %s", days, s)
	}
	return s
}

// --- storage & power ---

func storageLines() []string {
	var lines []string
	mounts := map[string][2]string{} // mount point: device, type
	if b, err := os.ReadFile("/proc/mounts"); err == nil {
		for _, l := range strings.Split(string(b), "\n") {
			if f := strings.Fields(l); len(f) >= 3 {
				mounts[f[1]] = [2]string{f[0], f[2]}
			}
		}
	}
	for _, mp := range []string{"/", "/mnt/onboard", "/mnt/sd", "/tmp"} {
		m, ok := mounts[mp]
		var st syscall.Statfs_t
		if !ok || syscall.Statfs(mp, &st) != nil {
			continue
		}
		total, free := int64(st.Blocks)*int64(st.Bsize), int64(st.Bavail)*int64(st.Bsize)
		lines = append(lines, fmt.Sprintf("%s  %s free of %s  (%s on %s)", mp, fmtBytes(free), fmtBytes(total), m[1], m[0]))
	}

	bat := ""
	dirs, _ := filepath.Glob("/sys/class/power_supply/*")
	for _, d := range dirs {
		if t, _ := os.ReadFile(d + "/type"); strings.TrimSpace(string(t)) == "Battery" {
			bat = d
			break
		}
	}
	if bat == "" {
		lines = append(lines, "no battery found")
	} else {
		read := func(name string) string {
			b, _ := os.ReadFile(bat + "/" + name)
			return strings.TrimSpace(string(b))
		}
		s := fmt.Sprintf("battery %s%% %s", read("capacity"), strings.ToLower(read("status")))
		if v := readInt(bat + "/voltage_now"); v > 0 {
			s += fmt.Sprintf(", %.2f V", float64(v)/1e6)
		}
		if c := readInt(bat + "/current_now"); c != 0 {
			s += fmt.Sprintf(", %d mA", c/1000)
		}
		if t := readInt(bat + "/temp"); t != 0 {
			s += fmt.Sprintf(", %.1f °C", float64(t)/10)
		}
		if h := read("health"); h != "" {
			s += ", " + strings.ToLower(h)
		}
		lines = append(lines, s)
	}

	zones, _ := filepath.Glob("/sys/class/thermal/thermal_zone*")
	var temps []string
	for _, z := range zones {
		t, _ := os.ReadFile(z + "/type")
		if c := readInt(z + "/temp"); c != 0 {
			deg := float64(c) / 1000
			if c < 1000 { // some drivers report whole degrees
				deg = float64(c)
			}
			temps = append(temps, fmt.Sprintf("%s %.0f °C", strings.TrimSpace(string(t)), deg))
		}
	}
	if len(temps) > 0 {
		if len(temps) > 6 {
			temps = append(temps[:6], "…")
		}
		lines = append(lines, "thermal "+strings.Join(temps, ", "))
	}
	return lines
}

// --- software & tools ---

// devTools are looked for in PATH, with how to ask each for its version.
var devTools = []struct{ name, args string }{
	{"sh", ""}, {"bash", "--version"}, {"busybox", ""}, {"python3", "--version"}, {"python", "--version"},
	{"perl", "-e print$^V"}, {"lua", "-v"}, {"gdb", "--version"}, {"strace", "-V"}, {"ltrace", "-V"},
	{"ssh", "-V"}, {"dropbear", "-V"}, {"scp", ""}, {"rsync", "--version"}, {"curl", "--version"},
	{"wget", "--version"}, {"nc", ""}, {"vi", ""}, {"nano", "--version"}, {"tmux", "-V"}, {"git", "--version"},
	{"gcc", "--version"}, {"make", "--version"}, {"openssl", "version"}, {"sqlite3", "--version"}, {"evtest", ""},
}

func softwareLines(s screenInfo) []string {
	lines := []string{fmt.Sprintf("hellobro: %s %s/%s, FBInk %s", runtime.Version(), runtime.GOOS, runtime.GOARCH, s.version)}

	applets := map[string]bool{}
	if bb, err := exec.LookPath("busybox"); err == nil {
		lines = append(lines, firstLine(run(bb, "")))
		if out := run(bb, "--list"); out != "" {
			for _, a := range strings.Fields(out) {
				applets[a] = true
			}
			lines[len(lines)-1] += fmt.Sprintf(", %d applets", len(applets))
		}
	}

	var addons []string
	for _, a := range []struct{ name, dir string }{
		{"KOReader", "/mnt/onboard/.adds/koreader"}, {"NickelMenu", "/mnt/onboard/.adds/nm"},
		{"KFMon", "/mnt/onboard/.adds/kfmon"}, {"Plato", "/mnt/onboard/.adds/plato"},
		{"NickelDBus", "/mnt/onboard/.adds/nickeldbus"},
	} {
		if _, err := os.Stat(a.dir); err == nil {
			name := a.name
			if rev, err := os.ReadFile(a.dir + "/git-rev"); err == nil {
				name += " " + strings.TrimSpace(string(rev))
			}
			addons = append(addons, name)
		}
	}
	if len(addons) > 0 {
		lines = append(lines, "add-ons: "+strings.Join(addons, ", "))
	}
	if servers := runningServers(); len(servers) > 0 {
		lines = append(lines, "running: "+strings.Join(servers, ", "))
	}

	var missing, viaBusybox []string
	for _, t := range devTools {
		if t.name == "busybox" {
			continue
		}
		path, err := exec.LookPath(t.name)
		if err != nil {
			if applets[t.name] {
				viaBusybox = append(viaBusybox, t.name+" (busybox --)")
			} else {
				missing = append(missing, t.name)
			}
			continue
		}
		if real, err := filepath.EvalSymlinks(path); err == nil && filepath.Base(real) == "busybox" {
			viaBusybox = append(viaBusybox, t.name)
			continue
		}
		line := t.name
		if dir := filepath.Dir(path); !slices.Contains([]string{"/bin", "/sbin", "/usr/bin", "/usr/sbin"}, dir) {
			line += " (" + dir + ")"
		}
		if t.args != "" {
			if v := firstLine(run(path, t.args)); v != "" {
				line += "  " + v
			}
		}
		lines = append(lines, line)
	}
	if len(viaBusybox) > 0 {
		lines = append(lines, "busybox applets: "+strings.Join(viaBusybox, " "))
	}
	if len(missing) > 0 {
		lines = append(lines, "not found: "+strings.Join(missing, " "))
	}
	return lines
}

// run runs a tool for a moment and returns what it said (some print
// versions to stderr, some exit non-zero after usage text).
func run(path, args string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, _ := exec.CommandContext(ctx, path, strings.Fields(args)...).CombinedOutput()
	return string(out)
}

func firstLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			if len(l) > 70 {
				l = l[:70] + "…"
			}
			return l
		}
	}
	return ""
}

// runningServers lists the remote-access daemons that are up.
func runningServers() []string {
	var found []string
	procs, _ := filepath.Glob("/proc/[0-9]*/comm")
	seen := map[string]bool{}
	for _, p := range procs {
		b, err := os.ReadFile(p)
		name := strings.TrimSpace(string(b))
		if err != nil || seen[name] {
			continue
		}
		switch name {
		case "dropbear", "sshd", "telnetd", "inetd", "ftpd", "vsftpd", "rsync":
			seen[name] = true
			found = append(found, name)
		}
	}
	return found
}
