// Package radio runs the Kobo's Wi-Fi and Bluetooth while Nickel is
// frozen: Wi-Fi through wpa_supplicant (wpa_cli) and a DHCP client, and
// turned on and off as Nickel does it (the modules, the chip's power), the
// way KOReader's scripts do; Bluetooth through BlueZ on D-Bus, with an
// agent for pairing. HELLOBRO_FAKE_RADIO=1 puts stand-ins in their place,
// for the harness.
package radio

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// Network is a Wi-Fi network: one heard in a scan, one wpa_supplicant
// knows, or both.
type Network struct {
	SSID     string
	BSSID    string
	Signal   int    // dBm; 0 if not heard
	Freq     int    // MHz
	Security string // "WPA2", "WPA", "WPA3", "WEP", "EAP", "" for open
	ID       int    // wpa_supplicant's network id, -1 if unknown to it
	Current  bool
	Hidden   bool // to look for by name (scan_ssid)
}

func (n Network) Known() bool { return n.ID >= 0 }

// Bars is the signal as 0 to 4 bars.
func (n Network) Bars() int {
	switch {
	case n.Signal == 0:
		return 0
	case n.Signal >= -55:
		return 4
	case n.Signal >= -67:
		return 3
	case n.Signal >= -75:
		return 2
	}
	return 1
}

// WiFiStatus is how Wi-Fi is doing.
type WiFiStatus struct {
	On        bool   // wpa_supplicant runs
	State     string // wpa_state: COMPLETED, SCANNING, DISCONNECTED…
	SSID      string
	BSSID     string
	Freq      int
	Signal    int // dBm
	IP        string
	Gateway   string
	Saving    bool   // networks added are saved (update_config); else they're for now
	Interface string // wlan0
}

// WiFi is what the app asks of Wi-Fi. Calls block; run them off the UI.
type WiFi interface {
	Status() WiFiStatus
	Scan() ([]Network, error) // what's heard, merged with what's known, best first
	// Connect joins n (with password, when it's new and secured), then gets
	// an address. progress hears how it goes.
	Connect(n Network, password string, progress func(string)) error
	Disconnect() error
	Forget(n Network) error
	SetPower(on bool, progress func(string)) error
	// Down turns Wi-Fi off but leaves the chip powered, its driver loaded:
	// for while Bluetooth is on. The Elipsa's RTL8821CS is one chip for
	// both, and cutting its power takes Bluetooth with it. SetPower(false)
	// after it powers the chip off too.
	Down(progress func(string)) error
	// Restore puts the power back as it was before the first SetPower:
	// what Nickel expects when it wakes.
	Restore()
	// Keep makes the power as it is now the one Restore puts back.
	Keep()
}

// NewWiFi is the Kobo's Wi-Fi, or the stand-in.
func NewWiFi() WiFi {
	if os.Getenv("HELLOBRO_FAKE_RADIO") != "" {
		return newFakeWiFi()
	}
	return &wpaWiFi{iface: kobEnv("INTERFACE", "wlan0")}
}

// kobEnv is one of the variables Nickel sets (INTERFACE, WIFI_MODULE,
// PLATFORM): ours, as Nickel started us, or else Nickel's own.
func kobEnv(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	for _, pid := range pidsOf("nickel") {
		b, _ := os.ReadFile("/proc/" + pid + "/environ")
		for _, kv := range bytes.Split(b, []byte{0}) {
			if k, v, ok := strings.Cut(string(kv), "="); ok && k == name && v != "" {
				return v
			}
		}
	}
	return def
}

type wpaWiFi struct {
	iface   string
	ctrlDir string // wpa_supplicant's -C, if it said
	initial *bool  // the power before we first changed it
	kept    bool   // off, but Down left the chip powered

	// What we last saw of wpa_supplicant. The bar asks for the status
	// every 15 s, and reading all of /proc for it, a few times over, was
	// most of what an idle hellobro did: while the same process runs, a
	// look at its comm is enough, and what its command line says (saving,
	// the recipe) is read again only for a new one, or now and then.
	mu     sync.Mutex
	pid    string
	fresh  bool // a new pid: saving and remember want reading again
	save   bool
	readAt time.Time
}

// readEvery is how often Status reads the daemons' command lines again
// anyway: dhcpcd may come up after wpa_supplicant.
const readEvery = 5 * time.Minute

const ctrlDirDefault = "/var/run/wpa_supplicant"

// cli asks wpa_supplicant, through its control socket (what wpa_cli
// does; the Kobo has no wpa_cli): args as wpa_cli takes them, the reply,
// or an error.
func (w *wpaWiFi) cli(args ...string) (string, error) {
	cmd := strings.ToUpper(args[0])
	if len(args) > 1 {
		cmd += " " + strings.Join(args[1:], " ")
	}
	dir := w.ctrl()
	if dir == "" {
		dir = ctrlDirDefault
	}
	local := fmt.Sprintf("/tmp/hellobro-wpa-%d-%d", os.Getpid(), ctrlSeq.Add(1))
	os.Remove(local)
	defer os.Remove(local)
	conn, err := net.DialUnix("unixgram", &net.UnixAddr{Name: local, Net: "unixgram"},
		&net.UnixAddr{Name: filepath.Join(dir, w.iface), Net: "unixgram"})
	if err != nil {
		return "", fmt.Errorf("wpa_supplicant: %v", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(8 * time.Second))
	if _, err := conn.Write([]byte(cmd)); err != nil {
		return "", fmt.Errorf("wpa_supplicant %s: %v", args[0], err)
	}
	buf := make([]byte, 64<<10)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			return "", fmt.Errorf("wpa_supplicant %s: %v", args[0], err)
		}
		reply := string(buf[:n])
		if strings.HasPrefix(reply, "<") { // an event, not our reply
			continue
		}
		s := strings.TrimSpace(reply)
		if strings.HasPrefix(s, "FAIL") || s == "UNKNOWN COMMAND" {
			return s, fmt.Errorf("wpa_supplicant %s: %s", strings.Join(args, " "), s)
		}
		return s, nil
	}
}

var ctrlSeq atomic.Int64

// ctrl is wpa_supplicant's control folder, from its command line.
func (w *wpaWiFi) ctrl() string {
	if w.ctrlDir != "" {
		return w.ctrlDir
	}
	for _, argv := range cmdlines("wpa_supplicant") {
		for i, a := range argv {
			if a == "-C" && i+1 < len(argv) {
				w.ctrlDir = argv[i+1]
			} else if strings.HasPrefix(a, "-C") && len(a) > 2 {
				w.ctrlDir = a[2:]
			}
		}
	}
	return w.ctrlDir
}

func (w *wpaWiFi) running() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.pid != "" && isProc(w.pid, "wpa_supplicant") {
		return true
	}
	pids := pidsOf("wpa_supplicant")
	if len(pids) == 0 {
		w.pid = ""
		return false
	}
	w.pid, w.fresh = pids[0], true
	return true
}

func (w *wpaWiFi) Status() WiFiStatus {
	st := WiFiStatus{Interface: w.iface, On: w.running()}
	if !st.On {
		return st
	}
	out, err := w.cli("status")
	if err != nil {
		st.State = err.Error()
		return st
	}
	kv := keyValues(out)
	st.State, st.SSID, st.BSSID = kv["wpa_state"], unescape(kv["ssid"]), kv["bssid"]
	st.Freq, _ = strconv.Atoi(kv["freq"])
	st.IP = kv["ip_address"]
	if sig, err := w.cli("signal_poll"); err == nil {
		st.Signal, _ = strconv.Atoi(keyValues(sig)["RSSI"])
	}
	if st.IP == "" {
		st.IP = ipv4Of(w.iface)
	}
	st.Gateway = defaultGateway(w.iface)
	w.mu.Lock()
	if w.fresh || time.Since(w.readAt) > readEvery {
		remember()
		w.save = saving(w.pid)
		w.fresh, w.readAt = false, time.Now()
	}
	st.Saving = w.save
	w.mu.Unlock()
	return st
}

// saving says whether wpa_supplicant (pid) writes networks back to its
// file.
func saving(pid string) bool {
	argv := cmdline(pid)
	for i, a := range argv {
		if a == "-c" && i+1 < len(argv) {
			b, _ := os.ReadFile(argv[i+1])
			return bytes.Contains(b, []byte("update_config=1"))
		}
	}
	return false
}

func (w *wpaWiFi) known() []Network {
	out, err := w.cli("list_networks")
	if err != nil {
		return nil
	}
	var ns []Network
	for _, line := range strings.Split(out, "\n")[1:] {
		f := strings.Split(line, "\t")
		if len(f) < 2 {
			continue
		}
		id, err := strconv.Atoi(f[0])
		if err != nil {
			continue
		}
		n := Network{ID: id, SSID: unescape(f[1])}
		if len(f) >= 4 {
			n.Current = strings.Contains(f[3], "[CURRENT]")
		}
		ns = append(ns, n)
	}
	return ns
}

func (w *wpaWiFi) Scan() ([]Network, error) {
	if !w.running() {
		return nil, errors.New("Wi-Fi is off")
	}
	if _, err := w.cli("scan"); err != nil {
		log.Printf("wifi: %v", err) // FAIL-BUSY: one's going on already, its results do
	}
	time.Sleep(3500 * time.Millisecond)
	out, err := w.cli("scan_results")
	if err != nil {
		return nil, err
	}
	var heard []Network
	for _, line := range strings.Split(out, "\n") {
		f := strings.Split(line, "\t")
		if len(f) < 5 || strings.Count(f[0], ":") != 5 {
			continue
		}
		n := Network{BSSID: f[0], SSID: unescape(f[4]), ID: -1, Security: security(f[3])}
		n.Freq, _ = strconv.Atoi(f[1])
		n.Signal, _ = strconv.Atoi(f[2])
		if n.Signal > 0 { // a quality, with wext: make it dBm-ish
			n.Signal = n.Signal/2 - 100
		}
		heard = append(heard, n)
	}
	return merge(heard, w.known(), w.Status()), nil
}

// merge makes one row per name: the strongest of what's heard, marked
// known if it is, then the known ones not heard.
func merge(heard, known []Network, st WiFiStatus) []Network {
	byName := map[string]int{}
	var out []Network
	for _, n := range heard {
		if n.SSID == "" {
			continue // hidden: "other network" adds one by name
		}
		if i, ok := byName[n.SSID]; ok {
			if n.Signal > out[i].Signal {
				n.ID, n.Current = out[i].ID, out[i].Current
				out[i] = n
			}
			continue
		}
		byName[n.SSID] = len(out)
		out = append(out, n)
	}
	for _, k := range known {
		i, ok := byName[k.SSID]
		if !ok {
			k.Hidden = true
			byName[k.SSID] = len(out)
			out = append(out, k)
			continue
		}
		out[i].ID = k.ID
		out[i].Current = out[i].Current || k.Current
	}
	for i := range out {
		if st.State == "COMPLETED" && out[i].SSID == st.SSID {
			out[i].Current = true
		}
	}
	slices.SortStableFunc(out, func(a, b Network) int {
		if a.Current != b.Current {
			return b2i(b.Current) - b2i(a.Current)
		}
		if a.Known() != b.Known() && (a.Signal == 0) == (b.Signal == 0) {
			return b2i(b.Known()) - b2i(a.Known())
		}
		return cmpSignal(a.Signal, b.Signal)
	})
	return out
}

func cmpSignal(a, b int) int {
	if a == 0 {
		a = -1000
	}
	if b == 0 {
		b = -1000
	}
	return b - a
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func security(flags string) string {
	switch {
	case strings.Contains(flags, "EAP"):
		return "EAP"
	case strings.Contains(flags, "SAE") && !strings.Contains(flags, "PSK"):
		return "WPA3"
	case strings.Contains(flags, "WPA2"), strings.Contains(flags, "RSN"):
		return "WPA2"
	case strings.Contains(flags, "WPA"):
		return "WPA"
	case strings.Contains(flags, "WEP"):
		return "WEP"
	}
	return ""
}

func (w *wpaWiFi) Connect(n Network, password string, progress func(string)) error {
	if !w.running() {
		return errors.New("Wi-Fi is off")
	}
	id, added := n.ID, false
	if id < 0 {
		if n.Security == "EAP" {
			return errors.New("enterprise (EAP) networks aren't supported here")
		}
		out, err := w.cli("add_network")
		if err != nil {
			return err
		}
		if id, err = strconv.Atoi(strings.TrimSpace(lastLine(out))); err != nil {
			return fmt.Errorf("add_network: %q", out)
		}
		added = true
		if err := w.configure(id, n, password); err != nil {
			w.cli("remove_network", strconv.Itoa(id))
			return err
		}
	}
	progress("joining " + n.SSID + "…")
	if _, err := w.cli("select_network", strconv.Itoa(id)); err != nil {
		return err
	}
	err := w.waitFor("COMPLETED", 25*time.Second)
	w.cli("enable_network", "all") // select_network turned the others off
	if err != nil {
		if added {
			w.cli("remove_network", strconv.Itoa(id))
			return errors.New("couldn't join " + n.SSID + ": wrong password?")
		}
		return fmt.Errorf("couldn't join %s: %v", n.SSID, err)
	}
	if added {
		if _, err := w.cli("save_config"); err != nil {
			log.Printf("wifi: save_config: %v (the network stays until Wi-Fi goes off)", err)
		}
	}
	progress("joined " + n.SSID + ", getting an address…")
	ip, err := w.renewIP()
	if err != nil {
		return err
	}
	progress("connected to " + n.SSID + ", " + ip)
	return nil
}

// configure sets up a new network for n.
func (w *wpaWiFi) configure(id int, n Network, password string) error {
	sid := strconv.Itoa(id)
	set := func(k, v string) error {
		_, err := w.cli("set_network", sid, k, v)
		return err
	}
	if err := set("ssid", fmt.Sprintf("%x", n.SSID)); err != nil { // hex: any bytes
		return err
	}
	if n.Hidden {
		set("scan_ssid", "1")
	}
	switch {
	case n.Security == "":
		return set("key_mgmt", "NONE")
	case n.Security == "WEP":
		set("key_mgmt", "NONE")
		set("wep_tx_keyidx", "0")
		if isHex(password) && (len(password) == 10 || len(password) == 26) {
			return set("wep_key0", password)
		}
		return set("wep_key0", strconv.Quote(password))
	case len(password) < 8 || len(password) > 64:
		return errors.New("a WPA password has 8 to 63 characters")
	case n.Security == "WPA3":
		set("key_mgmt", "SAE")
		set("ieee80211w", "2")
		if err := set("sae_password", strconv.Quote(password)); err != nil {
			return errors.New("this Kobo's wpa_supplicant can't do WPA3")
		}
		return nil
	case len(password) == 64 && isHex(password):
		return set("psk", password)
	}
	return set("psk", strconv.Quote(password))
}

func isHex(s string) bool {
	for _, c := range s {
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return false
		}
	}
	return s != ""
}

func (w *wpaWiFi) waitFor(state string, d time.Duration) error {
	end := time.Now().Add(d)
	last := ""
	for time.Now().Before(end) {
		out, _ := w.cli("status")
		last = keyValues(out)["wpa_state"]
		if last == state {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("still %s", strings.ToLower(last))
}

func (w *wpaWiFi) Disconnect() error {
	_, err := w.cli("disconnect")
	return err
}

func (w *wpaWiFi) Forget(n Network) error {
	if n.ID < 0 {
		return errors.New(n.SSID + " isn't saved")
	}
	if _, err := w.cli("remove_network", strconv.Itoa(n.ID)); err != nil {
		return err
	}
	if _, err := w.cli("save_config"); err != nil {
		log.Printf("wifi: save_config: %v", err)
	}
	return nil
}

// --- addresses ---

// renewIP gets an address for the network just joined: from the DHCP
// client that's running, or one started as Kobo starts it. A running
// dhcpcd is left to it: it saw the carrier go and come back, dropped the
// old address and asks for a new one by itself. Telling it to rebind
// (dhcpcd -n) kills Kobo's dhcpcd 6.6.6, and then nobody asks.
func (w *wpaWiFi) renewIP() (string, error) {
	started := false
	if !dhcpRunning() {
		w.releaseIP()
		w.obtainIP()
		started = true
	}
	end := time.Now().Add(30 * time.Second)
	for time.Now().Before(end) {
		if ip := ipv4Of(w.iface); ip != "" {
			return ip, nil
		}
		if !started && !dhcpRunning() { // it went meanwhile
			log.Print("wifi: the DHCP client went: starting one")
			w.obtainIP()
			started = true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return "", errors.New("joined, but got no address (DHCP)")
}

func dhcpRunning() bool { return len(pidsOf("dhcpcd", "udhcpc")) > 0 }

// obtainIP starts the DHCP client, as KOReader's obtain-ip.sh does.
func (w *wpaWiFi) obtainIP() {
	rec := recipe()
	for _, n := range []string{"dhcpcd", "udhcpc"} {
		if argv := rec[n]; argv != nil { // as Kobo starts it
			start(argv...)
			return
		}
	}
	if _, err := os.Stat("/sbin/dhcpcd"); err == nil {
		start("dhcpcd", "-d", "-t", "30", "-w", w.iface) // it waits for the lease, then stays
		return
	}
	hn, _ := os.Hostname()
	if hn == "" || hn == "(none)" {
		hn = "kobo"
	}
	run("udhcpc", "-S", "-i", w.iface, "-s", "/etc/udhcpc.d/default.script", "-x", "hostname:"+hn, "-b", "-q")
}

// releaseIP stops the DHCP clients, keeping resolv.conf as it was.
func (w *wpaWiFi) releaseIP() {
	resolv, _ := os.ReadFile("/etc/resolv.conf")
	if _, err := os.Stat("/sbin/dhcpcd"); err == nil {
		run("dhcpcd", "-d", "-k", w.iface)
		killAll(syscall.SIGTERM, "udhcpc", "default.script")
	} else {
		killAll(syscall.SIGTERM, "udhcpc", "default.script", "dhcpcd")
		run("ifconfig", w.iface, "0.0.0.0")
	}
	for i := 0; i < 20 && len(pidsOf("udhcpc")) > 0; i++ {
		time.Sleep(250 * time.Millisecond)
	}
	if now, _ := os.ReadFile("/etc/resolv.conf"); len(resolv) > 0 && !bytes.Equal(now, resolv) {
		os.WriteFile("/etc/resolv.conf", resolv, 0o644)
	}
}

// --- power ---

func (w *wpaWiFi) SetPower(on bool, progress func(string)) error {
	if w.initial == nil {
		was := w.running()
		w.initial = &was
	}
	if on == w.running() && !(w.kept && !on) {
		return nil
	}
	if on {
		kept := w.kept
		w.kept = false
		return w.powerOn(progress, kept)
	}
	return w.powerOff(progress, false)
}

func (w *wpaWiFi) Down(progress func(string)) error {
	if w.initial == nil {
		was := w.running()
		w.initial = &was
	}
	if !w.running() {
		return nil
	}
	return w.powerOff(progress, true)
}

func (w *wpaWiFi) Keep() {
	on := w.running()
	w.initial = &on
}

func (w *wpaWiFi) Restore() {
	if w.initial == nil || *w.initial == w.running() {
		return
	}
	log.Printf("wifi: putting the power back as Kobo had it: %v", *w.initial)
	if err := w.SetPower(*w.initial, func(s string) { log.Printf("wifi: %s", s) }); err != nil {
		log.Printf("wifi: %v", err)
	}
}

// chip says how this Kobo's Wi-Fi chip is driven, as KOReader's scripts
// tell it apart.
type chip struct {
	module, platform, kmodPath, power, driver string
}

func kobChip() chip {
	c := chip{module: kobEnv("WIFI_MODULE", ""), platform: kobEnv("PLATFORM", ""), power: "module", driver: "wext"}
	c.kmodPath = "/drivers/" + c.platform + "/wifi"
	switch c.module {
	case "moal":
		c.power, c.driver = "ntx_io", "nl80211"
	case "8821cs":
		c.driver = "nl80211" // what Kobo runs on the Elipsa and Sage
	case "wlan_drv_gen4m":
		c.power, c.driver = "wmt", "nl80211"
		c.kmodPath = "/drivers/" + c.platform + "/mt66xx"
	}
	return c
}

// powerOn turns Wi-Fi on; kept, the chip still has its power and its
// driver (Down), so it's only the interface and wpa_supplicant.
func (w *wpaWiFi) powerOn(progress func(string), kept bool) error {
	c := kobChip()
	log.Printf("wifi: turning on: module %q, platform %q, power by %s, chip kept on %v", c.module, c.platform, c.power, kept)
	if c.module == "" || c.platform == "" {
		return errors.New("don't know this Kobo's Wi-Fi chip (WIFI_MODULE, PLATFORM unset)")
	}
	if c.power == "wmt" {
		return errors.New("turning Wi-Fi on isn't done for this Kobo's chip yet; use Kobo's settings")
	}
	if kept && loaded(c.module) {
		return w.start(c, progress)
	}
	progress("powering the chip…")
	switch {
	case c.power == "ntx_io":
		if err := ntxIO(208, 1); err != nil {
			return err
		}
	case loaded("sdio_wifi_pwr"):
	case exists(c.kmodPath + "/sdio_wifi_pwr.ko"):
		if err := run("insmod", c.kmodPath+"/sdio_wifi_pwr.ko"); err != nil {
			return err
		}
	default:
		if err := ntxIO(208, 1); err != nil {
			return err
		}
	}
	time.Sleep(250 * time.Millisecond)
	if !loaded(c.module) {
		progress("loading the driver…")
		var params []string
		if cc := kobConf("WifiRegulatoryDomain"); cc != "" {
			switch c.module {
			case "8821cs":
				params = append(params, "rtw_country_code="+cc)
			case "moal":
				params = append(params, "reg_alpha2="+cc)
			}
		}
		if c.module == "moal" {
			params = append(params, "mod_para=nxp/wifi_mod_para_sd8987.conf")
			run("insmod", c.kmodPath+"/mlan.ko")
			time.Sleep(250 * time.Millisecond)
		}
		ko := c.kmodPath + "/" + c.module + ".ko"
		if !exists(ko) {
			ko = "/drivers/" + c.platform + "/" + c.module + ".ko"
		}
		if err := run("insmod", append([]string{ko}, params...)...); err != nil {
			return err
		}
	}
	if c.module == "moal" {
		time.Sleep(2 * time.Second)
	} else {
		time.Sleep(time.Second)
	}
	return w.start(c, progress)
}

// start brings the interface up, starts wpa_supplicant and waits for a
// network and an address.
func (w *wpaWiFi) start(c chip, progress func(string)) error {
	run("ifconfig", w.iface, "up")
	if c.module == "dhd" {
		run("wlarm_le", "-i", w.iface, "up")
	}
	progress("starting wpa_supplicant…")
	argv := recipe()["wpa_supplicant"] // as Kobo started it, if we saw that
	if argv == nil {
		conf, extra := "/etc/wpa_supplicant/wpa_supplicant.conf", []string{"-s"}
		if exists("/mnt/onboard/.kobo/wpa_supplicant.conf") { // firmware 5
			conf, extra = "/mnt/onboard/.kobo/wpa_supplicant.conf", nil
		}
		argv = append([]string{"wpa_supplicant", "-D", c.driver}, extra...)
		argv = append(argv, "-i", w.iface, "-c", conf, "-C", ctrlDirDefault, "-B")
	}
	if !slices.Contains(argv, "-B") {
		argv = append(argv, "-B")
	}
	if err := run(argv[0], argv[1:]...); err != nil {
		return err
	}
	w.ctrlDir = ""
	progress("looking for a known network…")
	if err := w.waitFor("COMPLETED", 15*time.Second); err != nil {
		return errors.New("Wi-Fi is on, no known network around: pick one")
	}
	progress("getting an address…")
	w.obtainIP()
	end := time.Now().Add(30 * time.Second)
	for time.Now().Before(end) {
		if ip := ipv4Of(w.iface); ip != "" {
			progress("connected, " + ip)
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return errors.New("joined, but got no address (DHCP)")
}

// powerOff turns Wi-Fi off; keep leaves the chip powered and its driver
// in, the interface down (Down).
func (w *wpaWiFi) powerOff(progress func(string), keep bool) error {
	c := kobChip()
	log.Printf("wifi: turning off: module %q, power by %s, chip kept on %v", c.module, c.power, keep)
	progress("turning Wi-Fi off…")
	w.releaseIP()
	w.cli("terminate")
	for i := 0; i < 20 && w.running(); i++ {
		time.Sleep(250 * time.Millisecond)
	}
	if c.module == "dhd" {
		run("wlarm_le", "-i", w.iface, "down")
	}
	run("ifconfig", w.iface, "down")
	if w.kept = keep; keep {
		return nil
	}
	if c.power == "wmt" { // its modules stay; only the chip goes off
		return os.WriteFile("/dev/wmtWifi", []byte("0"), 0)
	}
	if c.module != "" && loaded(c.module) {
		time.Sleep(250 * time.Millisecond)
		run("rmmod", c.module)
	}
	if c.module == "moal" && loaded("mlan") {
		time.Sleep(250 * time.Millisecond)
		run("rmmod", "mlan")
	}
	switch {
	case c.power == "ntx_io":
		time.Sleep(250 * time.Millisecond)
		return ntxIO(208, 0)
	case loaded("sdio_wifi_pwr"):
		time.Sleep(250 * time.Millisecond)
		run("rmmod", "sdio_wifi_pwr")
	}
	if !exists(c.kmodPath + "/sdio_wifi_pwr.ko") {
		time.Sleep(250 * time.Millisecond)
		return ntxIO(208, 0)
	}
	return nil
}

// ntxIO sends an ioctl to the board's ntx_io device: 208 is the Wi-Fi
// chip's power.
func ntxIO(cmd, arg uintptr) error {
	f, err := os.OpenFile("/dev/ntx_io", os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), cmd, arg); e != 0 {
		return fmt.Errorf("ntx_io %d %d: %v", cmd, arg, e)
	}
	log.Printf("wifi: ntx_io %d %d", cmd, arg)
	return nil
}

// kobConf is a setting from Kobo eReader.conf.
func kobConf(key string) string {
	b, _ := os.ReadFile("/mnt/onboard/.kobo/Kobo/Kobo eReader.conf")
	for _, line := range strings.Split(string(b), "\n") {
		if k, v, ok := strings.Cut(strings.TrimSpace(line), "="); ok && k == key {
			return v
		}
	}
	return ""
}

// --- how Kobo does it ---

// StateDir is where what's learned of the device is kept: the app's
// folder. "" keeps nothing.
var StateDir string

const recipeFile = "wifi-recipe"

// remember keeps the command lines Kobo started wpa_supplicant and the
// DHCP client with, to start them the same way after turning Wi-Fi off.
func remember() {
	if StateDir == "" {
		return
	}
	rec := recipe()
	changed := false
	names := []string{"wpa_supplicant", "dhcpcd", "udhcpc"}
	pids := procsNamed(names...)
	for _, n := range names {
		if len(pids[n]) > 0 {
			if argv := cmdline(pids[n][0]); !slices.Equal(rec[n], argv) {
				rec[n], changed = argv, true
			}
		}
	}
	if rec["dhcpcd"] != nil && rec["udhcpc"] != nil {
		if len(pids["dhcpcd"]) > 0 {
			delete(rec, "udhcpc")
		} else if len(pids["udhcpc"]) > 0 {
			delete(rec, "dhcpcd")
		}
	}
	if !changed {
		return
	}
	var b strings.Builder
	for _, argv := range rec {
		b.WriteString(strings.Join(argv, "\x1f") + "\n")
	}
	log.Printf("wifi: remembering how Kobo starts it: %v", rec)
	os.WriteFile(filepath.Join(StateDir, recipeFile), []byte(b.String()), 0o644)
}

// recipe is what remember kept, by program name.
func recipe() map[string][]string {
	rec := map[string][]string{}
	if StateDir == "" {
		return rec
	}
	b, _ := os.ReadFile(filepath.Join(StateDir, recipeFile))
	for _, line := range strings.Split(string(b), "\n") {
		if argv := strings.Split(line, "\x1f"); line != "" {
			rec[filepath.Base(argv[0])] = argv
		}
	}
	return rec
}

// start runs a daemon that goes off on its own: not waited for (but
// reaped), its output dropped, out of our session so it outlives us.
func start(argv ...string) {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = "/" // nothing of ours, nor /mnt/onboard (USB storage wants it free)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	err := cmd.Start()
	log.Printf("radio: started %s: %v", strings.Join(argv, " "), err)
	if err == nil {
		go cmd.Wait()
	}
}

// --- helpers ---

// run runs a command, logging it and what went wrong.
func run(name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	log.Printf("radio: %s %s: %v %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	if err != nil {
		return fmt.Errorf("%s: %v %s", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func loaded(mod string) bool {
	b, _ := os.ReadFile("/proc/modules")
	return bytes.Contains(append([]byte("\n"), b...), []byte("\n"+mod+" "))
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

func keyValues(s string) map[string]string {
	m := map[string]string{}
	for _, line := range strings.Split(s, "\n") {
		if k, v, ok := strings.Cut(line, "="); ok {
			m[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return m
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return lines[len(lines)-1]
}

// unescape undoes wpa_supplicant's printf escapes in names (\xNN, \\, \").
func unescape(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b []byte
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b = append(b, s[i])
			continue
		}
		i++
		switch s[i] {
		case 'x':
			if i+2 < len(s) {
				if n, err := strconv.ParseUint(s[i+1:i+3], 16, 8); err == nil {
					b = append(b, byte(n))
					i += 2
					continue
				}
			}
			b = append(b, '\\', 'x')
		case 'n':
			b = append(b, '\n')
		case 't':
			b = append(b, '\t')
		default:
			b = append(b, s[i])
		}
	}
	return string(b)
}

func ipv4Of(iface string) string {
	ifc, err := net.InterfaceByName(iface)
	if err != nil {
		return ""
	}
	as, _ := ifc.Addrs()
	for _, ad := range as {
		if n, ok := ad.(*net.IPNet); ok && n.IP.To4() != nil {
			return n.IP.String()
		}
	}
	return ""
}

// defaultGateway reads the default route through iface.
func defaultGateway(iface string) string {
	b, _ := os.ReadFile("/proc/net/route")
	for _, line := range strings.Split(string(b), "\n")[1:] {
		f := strings.Fields(line)
		if len(f) >= 3 && f[0] == iface && f[1] == "00000000" {
			g, err := strconv.ParseUint(f[2], 16, 32)
			if err == nil {
				return net.IPv4(byte(g), byte(g>>8), byte(g>>16), byte(g>>24)).String()
			}
		}
	}
	return ""
}

func pidsOf(names ...string) []string {
	var pids []string
	for _, p := range procsNamed(names...) {
		pids = append(pids, p...)
	}
	slices.Sort(pids)
	return pids
}

// procsNamed are the pids of the processes called each of names, from
// one look through /proc.
func procsNamed(names ...string) map[string][]string {
	pids := map[string][]string{}
	dirs, _ := os.ReadDir("/proc")
	for _, d := range dirs {
		if d.Name()[0] < '0' || d.Name()[0] > '9' {
			continue
		}
		comm, err := os.ReadFile(filepath.Join("/proc", d.Name(), "comm"))
		if err != nil {
			continue
		}
		if n := strings.TrimSpace(string(comm)); slices.Contains(names, n) {
			pids[n] = append(pids[n], d.Name())
		}
	}
	return pids
}

// isProc says whether pid is still a process called name.
func isProc(pid, name string) bool {
	comm, err := os.ReadFile("/proc/" + pid + "/comm")
	return err == nil && strings.TrimSpace(string(comm)) == name
}

// cmdlines are the command lines of the processes called name.
func cmdlines(name string) [][]string {
	var out [][]string
	for _, pid := range pidsOf(name) {
		if argv := cmdline(pid); argv != nil {
			out = append(out, argv)
		}
	}
	return out
}

// cmdline is the command line of the process pid, nil if it's gone.
func cmdline(pid string) []string {
	b, err := os.ReadFile("/proc/" + pid + "/cmdline")
	if err != nil || len(b) == 0 {
		return nil
	}
	return strings.Split(strings.TrimRight(string(b), "\x00"), "\x00")
}

func killAll(sig syscall.Signal, names ...string) {
	for _, pid := range pidsOf(names...) {
		if n, err := strconv.Atoi(pid); err == nil {
			syscall.Kill(n, sig)
		}
	}
}

// LogFacts writes what's known of the radios' setup to the log, once:
// for working out what the device does.
func LogFacts() {
	for _, k := range []string{"INTERFACE", "WIFI_MODULE", "PLATFORM", "PRODUCT", "MODEL_NUMBER"} {
		log.Printf("radio: %s=%q", k, kobEnv(k, ""))
	}
	for _, n := range []string{"wpa_supplicant", "dhcpcd", "udhcpc", "bluetoothd", "bluealsa", "hciattach", "rtk_hciattach", "btattach", "nickel"} {
		for _, argv := range cmdlines(n) {
			log.Printf("radio: process %s", strings.Join(argv, " "))
		}
	}
	mods, _ := os.ReadFile("/proc/modules")
	var names []string
	for _, line := range strings.Split(string(mods), "\n") {
		if f := strings.Fields(line); len(f) > 0 {
			names = append(names, f[0])
		}
	}
	log.Printf("radio: modules %s", strings.Join(names, " "))
	c := kobChip()
	if ents, err := os.ReadDir(c.kmodPath); err == nil {
		var ns []string
		for _, e := range ents {
			ns = append(ns, e.Name())
		}
		log.Printf("radio: %s: %s", c.kmodPath, strings.Join(ns, " "))
	}
	if ents, err := os.ReadDir("/drivers/" + c.platform); err == nil {
		var ns []string
		for _, e := range ents {
			ns = append(ns, e.Name())
		}
		log.Printf("radio: /drivers/%s: %s", c.platform, strings.Join(ns, " "))
	}
	rf, _ := filepath.Glob("/sys/class/rfkill/rfkill*")
	for _, r := range rf {
		n, _ := os.ReadFile(r + "/name")
		t, _ := os.ReadFile(r + "/type")
		s, _ := os.ReadFile(r + "/state")
		log.Printf("radio: %s %s %s state %s", filepath.Base(r), strings.TrimSpace(string(n)), strings.TrimSpace(string(t)), strings.TrimSpace(string(s)))
	}
	for _, p := range []string{"/etc/wpa_supplicant/wpa_supplicant.conf", "/mnt/onboard/.kobo/wpa_supplicant.conf"} {
		if b, err := os.ReadFile(p); err == nil {
			var head []string
			for _, l := range strings.Split(string(b), "\n") {
				if strings.Contains(l, "psk") || strings.Contains(l, "password") || strings.Contains(l, "wep_key") {
					continue // no secrets in the log
				}
				if !strings.HasPrefix(strings.TrimSpace(l), "ssid") && !strings.Contains(l, "{") && !strings.Contains(l, "}") && strings.TrimSpace(l) != "" {
					head = append(head, strings.TrimSpace(l))
				}
			}
			log.Printf("radio: %s: %s", p, strings.Join(head, "; "))
		}
	}
	for _, p := range []string{"/sbin/dhcpcd", "/sbin/udhcpc", "/usr/bin/wpa_cli", "/sbin/wpa_cli", "/usr/sbin/wpa_cli", "/etc/udhcpc.d/default.script", "/dev/ntx_io"} {
		log.Printf("radio: %s exists: %v", p, exists(p))
	}
}
