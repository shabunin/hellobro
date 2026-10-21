package main

// The Network tab's probes: Wi-Fi status, a scan (access points + hosts on
// the local network), and an HTTPS request. They run in goroutines and
// report through netMsg; Wi-Fi itself stays Nickel's business (turn it on
// there before launching).

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"net/http/httptrace"
	"os"
	"os/exec"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	_ "golang.org/x/crypto/x509roots/fallback" // CA roots, in case the Kobo has none Go can find

	"hellobro/kobo"
)

const (
	netWifi = iota
	netScan
	netMDNS
	netHTTP
	numNetPanels
)

// netMsg replaces one Network panel's content.
type netMsg struct {
	panel  int
	status string
	lines  []string
	done   bool // the probe finished (more messages = progress)
}

const httpTestURL = "https://httpbin.org/get"

var sweepPorts = []int{22, 53, 80, 443, 445, 8080}

// wifiIface is the Wi-Fi interface: Nickel exports it to what it starts.
func wifiIface() string { return kobo.EnvOr("INTERFACE", "wlan0") }

// --- Wi-Fi status ---

func wifiStatus(out chan<- netMsg) {
	ifname := wifiIface()
	var lines []string
	add := func(format string, args ...any) { lines = append(lines, fmt.Sprintf(format, args...)) }

	ifc, err := net.InterfaceByName(ifname)
	if err != nil {
		out <- netMsg{netWifi, ifname + " missing", []string{
			fmt.Sprintf("no %s: %v", ifname, err),
			"Turn Wi-Fi on in Kobo before starting brotools.",
		}, true}
		return
	}
	state := "down"
	if ifc.Flags&net.FlagUp != 0 {
		state = "up"
	}
	wpa := wpaStatus(ifname)
	if s := wpa["wpa_state"]; s != "" {
		state = strings.ToLower(s)
	}
	if ssid := wpa["ssid"]; ssid != "" {
		add("network: %s", ssid)
	}
	if b := wpa["bssid"]; b != "" {
		add("access point: %s, %s", b, freqName(wpa["freq"]))
	}
	sig := wpaCLI(ifname, "signal_poll")
	if rssi := keyValues(sig)["RSSI"]; rssi != "" {
		add("signal: %s dBm (%s), link %s Mbit/s", rssi, signalWord(rssi), keyValues(sig)["LINKSPEED"])
	} else if lvl := procWirelessLevel(ifname); lvl != "" {
		add("signal: %s dBm (%s)", lvl, signalWord(lvl))
	}
	if km := wpa["key_mgmt"]; km != "" {
		add("security: %s", km)
	}
	ip4, mask := ipv4(ifc)
	if ip4 == nil {
		add("no IPv4 address")
	} else {
		ones, _ := mask.Size()
		add("address: %s/%d", ip4, ones)
	}
	add("MAC: %s", ifc.HardwareAddr)
	if gw := defaultGateway(); gw != "" {
		add("gateway: %s", gw)
	}
	if dns := nameservers(); len(dns) > 0 {
		add("DNS: %s", strings.Join(dns, ", "))
	}
	if wpa == nil {
		add("(wpa_cli unavailable, so no Wi-Fi details)")
	}
	out <- netMsg{netWifi, ifname + " · " + state, lines, true}
}

// wpaCLI asks Nickel's wpa_supplicant, through wpa_cli. "" if that fails.
func wpaCLI(ifname string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	b, err := exec.CommandContext(ctx, "wpa_cli", append([]string{"-i", ifname}, args...)...).Output()
	if err != nil {
		return ""
	}
	return string(b)
}

func wpaStatus(ifname string) map[string]string {
	s := wpaCLI(ifname, "status")
	if s == "" {
		return nil
	}
	return keyValues(s)
}

func keyValues(s string) map[string]string {
	kv := map[string]string{}
	for _, line := range strings.Split(s, "\n") {
		if k, v, ok := strings.Cut(strings.TrimSpace(line), "="); ok {
			kv[k] = v
		}
	}
	return kv
}

// procWirelessLevel reads the signal level from /proc/net/wireless.
func procWirelessLevel(ifname string) string {
	b, err := os.ReadFile("/proc/net/wireless")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		if len(f) >= 4 && strings.TrimSuffix(f[0], ":") == ifname {
			return strings.TrimSuffix(f[3], ".")
		}
	}
	return ""
}

func signalWord(dbm string) string {
	n, err := strconv.Atoi(dbm)
	switch {
	case err != nil:
		return "?"
	case n >= -55:
		return "excellent"
	case n >= -67:
		return "good"
	case n >= -75:
		return "fair"
	}
	return "weak"
}

func freqName(mhz string) string {
	n, _ := strconv.Atoi(mhz)
	switch {
	case n == 0:
		return "? GHz"
	case n < 3000:
		return fmt.Sprintf("2.4 GHz (%d MHz)", n)
	}
	return fmt.Sprintf("5 GHz (%d MHz)", n)
}

func ipv4(ifc *net.Interface) (net.IP, net.IPMask) {
	addrs, _ := ifc.Addrs()
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil {
			return n.IP.To4(), n.Mask
		}
	}
	return nil, nil
}

// defaultGateway reads /proc/net/route (addresses are little-endian hex).
func defaultGateway() string {
	b, err := os.ReadFile("/proc/net/route")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n")[1:] {
		f := strings.Fields(line)
		if len(f) < 3 || f[1] != "00000000" {
			continue
		}
		gw, err := strconv.ParseUint(f[2], 16, 32)
		if err != nil {
			continue
		}
		ip := make(net.IP, 4)
		binary.LittleEndian.PutUint32(ip, uint32(gw))
		return fmt.Sprintf("%s via %s", ip, f[0])
	}
	return ""
}

func nameservers() []string {
	b, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		return nil
	}
	var ns []string
	for _, line := range strings.Split(string(b), "\n") {
		if f := strings.Fields(line); len(f) >= 2 && f[0] == "nameserver" {
			ns = append(ns, f[1])
		}
	}
	return ns
}

// --- scan ---

func scan(out chan<- netMsg) {
	ifname := wifiIface()
	var lines []string
	progress := func(status string, more ...string) {
		out <- netMsg{netScan, status, append(slices.Clone(lines), more...), false}
	}

	progress("scanning Wi-Fi…")
	aps, err := wifiScan(ifname)
	if err != nil {
		lines = append(lines, "Wi-Fi networks: "+err.Error())
	} else {
		lines = append(lines, fmt.Sprintf("Wi-Fi networks: %d", len(aps)))
	}
	const maxAPs = 8
	for i, ap := range aps {
		if i == maxAPs {
			lines = append(lines, fmt.Sprintf("   … and %d more", len(aps)-maxAPs))
			break
		}
		lines = append(lines, "   "+ap)
	}

	ifc, err := net.InterfaceByName(ifname)
	var ip4 net.IP
	var mask net.IPMask
	if err == nil {
		ip4, mask = ipv4(ifc)
	}
	if ip4 == nil {
		lines = append(lines, "Local network: no IPv4 address, nothing to sweep")
		out <- netMsg{netScan, "done", lines, true}
		return
	}
	// Sweep the subnet, but no more than the /24 around us.
	if ones, _ := mask.Size(); ones < 24 {
		mask = net.CIDRMask(24, 32)
	}
	subnet := &net.IPNet{IP: ip4.Mask(mask), Mask: mask}
	start := time.Now()
	hosts := sweep(subnet, ip4, func(done, total, found int) {
		progress(fmt.Sprintf("sweeping %s: %d%%", subnet, done*100/total), fmt.Sprintf("Local network: %d hosts so far…", found))
	})
	lines = append(lines, fmt.Sprintf("Local network %s: %d hosts (%.1fs)", subnet, len(hosts), time.Since(start).Seconds()))
	lines = append(lines, hosts...)
	out <- netMsg{netScan, "done", lines, true}
}

// wifiScan asks wpa_supplicant for a fresh scan and lists what it heard,
// strongest first, one line per network name.
func wifiScan(ifname string) ([]string, error) {
	if wpaCLI(ifname, "scan") == "" {
		return nil, errors.New("can't ask wpa_supplicant (wpa_cli failed)")
	}
	time.Sleep(4 * time.Second) // scans take a few seconds; results are cached anyway
	type ap struct {
		ssid, flags string
		level, freq int
	}
	best := map[string]ap{}
	for _, line := range strings.Split(wpaCLI(ifname, "scan_results"), "\n") {
		f := strings.Split(line, "\t") // bssid, frequency, signal level, flags, ssid
		if len(f) < 5 {
			continue
		}
		level, err := strconv.Atoi(f[2])
		if err != nil {
			continue
		}
		freq, _ := strconv.Atoi(f[1])
		name := f[4]
		if name == "" {
			name = "(hidden)"
		}
		if old, ok := best[name]; !ok || level > old.level {
			best[name] = ap{name, f[3], level, freq}
		}
	}
	list := make([]ap, 0, len(best))
	for _, a := range best {
		list = append(list, a)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].level > list[j].level })
	var lines []string
	for _, a := range list {
		band := "2.4G"
		if a.freq >= 3000 {
			band = "5G"
		}
		sec := "open"
		switch {
		case strings.Contains(a.flags, "SAE"):
			sec = "WPA3"
		case strings.Contains(a.flags, "WPA2"):
			sec = "WPA2"
		case strings.Contains(a.flags, "WPA"):
			sec = "WPA"
		case strings.Contains(a.flags, "WEP"):
			sec = "WEP"
		}
		lines = append(lines, fmt.Sprintf("%4d dBm  %-4s  %-4s  %s", a.level, band, sec, a.ssid))
	}
	return lines, nil
}

// sweep finds live hosts by trying a few TCP ports on every address:
// an answer, even a refusal, means something is there. Hosts that only
// answered ARP are picked up from the kernel's neighbour table.
func sweep(subnet *net.IPNet, self net.IP, progress func(done, total, found int)) []string {
	var targets []net.IP
	base := binary.BigEndian.Uint32(subnet.IP.To4())
	ones, bits := subnet.Mask.Size()
	for i := uint32(1); i < 1<<(bits-ones)-1; i++ {
		ip := make(net.IP, 4)
		binary.BigEndian.PutUint32(ip, base+i)
		if !ip.Equal(self) {
			targets = append(targets, ip)
		}
	}

	type host struct {
		open []int
		mac  string
	}
	var mu sync.Mutex
	found := map[string]*host{}
	var done atomic.Int32
	sem := make(chan struct{}, 48) // hosts in flight, each trying all ports at once
	var wg sync.WaitGroup
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	finished := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				mu.Lock()
				n := len(found)
				mu.Unlock()
				progress(int(done.Load()), len(targets), n)
			case <-finished:
				return
			}
		}
	}()
	for _, ip := range targets {
		wg.Add(1)
		sem <- struct{}{}
		go func(ip net.IP) {
			defer func() { <-sem; done.Add(1); wg.Done() }()
			var pw sync.WaitGroup
			for _, port := range sweepPorts {
				pw.Add(1)
				go func(port int) {
					defer pw.Done()
					c, err := net.DialTimeout("tcp", net.JoinHostPort(ip.String(), strconv.Itoa(port)), 500*time.Millisecond)
					if c != nil {
						c.Close()
					}
					if err != nil && !errors.Is(err, syscall.ECONNREFUSED) {
						return
					}
					mu.Lock()
					defer mu.Unlock()
					h := found[ip.String()]
					if h == nil {
						h = &host{}
						found[ip.String()] = h
					}
					if err == nil {
						h.open = append(h.open, port)
					}
				}(port)
			}
			pw.Wait()
		}(ip)
	}
	wg.Wait()
	close(finished)

	for ip, mac := range arpTable() {
		if subnet.Contains(net.ParseIP(ip)) && !net.ParseIP(ip).Equal(self) {
			if found[ip] == nil {
				found[ip] = &host{}
			}
			found[ip].mac = mac
		}
	}
	ips := make([]string, 0, len(found))
	for ip := range found {
		ips = append(ips, ip)
	}
	sort.Slice(ips, func(i, j int) bool {
		return binary.BigEndian.Uint32(net.ParseIP(ips[i]).To4()) < binary.BigEndian.Uint32(net.ParseIP(ips[j]).To4())
	})
	gw := defaultGateway()
	var lines []string
	for _, ip := range ips {
		h := found[ip]
		line := "   " + ip
		if strings.HasPrefix(gw, ip+" ") {
			line += " (gateway)"
		}
		if len(h.open) > 0 {
			ports := make([]string, len(h.open))
			slices.Sort(h.open)
			for i, p := range h.open {
				ports[i] = strconv.Itoa(p)
			}
			line += "  open " + strings.Join(ports, ",")
		}
		if h.mac != "" {
			line += "  " + h.mac
		}
		lines = append(lines, line)
	}
	return lines
}

// arpTable returns IP → MAC for complete entries of /proc/net/arp.
func arpTable() map[string]string {
	f, err := os.Open("/proc/net/arp")
	if err != nil {
		return nil
	}
	defer f.Close()
	t := map[string]string{}
	sc := bufio.NewScanner(f)
	sc.Scan() // header
	for sc.Scan() {
		// IP address, HW type, Flags, HW address, Mask, Device
		fl := strings.Fields(sc.Text())
		if len(fl) >= 4 && fl[2] != "0x0" && fl[3] != "00:00:00:00:00:00" {
			t[fl[0]] = fl[3]
		}
	}
	return t
}

// --- HTTP ---

func httpTest(out chan<- netMsg) {
	out <- netMsg{netHTTP, "requesting…", []string{"GET " + httpTestURL}, false}

	var start, dnsStart, connStart, tlsStart time.Time
	var dns, conn, tlsT, first time.Duration
	var remote string
	trace := &httptrace.ClientTrace{
		DNSStart:             func(httptrace.DNSStartInfo) { dnsStart = time.Now() },
		DNSDone:              func(httptrace.DNSDoneInfo) { dns = time.Since(dnsStart) },
		ConnectStart:         func(_, _ string) { connStart = time.Now() },
		ConnectDone:          func(_, addr string, _ error) { conn, remote = time.Since(connStart), addr },
		TLSHandshakeStart:    func() { tlsStart = time.Now() },
		TLSHandshakeDone:     func(tls.ConnectionState, error) { tlsT = time.Since(tlsStart) },
		GotFirstResponseByte: func() { first = time.Since(start) },
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(httptrace.WithClientTrace(ctx, trace), "GET", httpTestURL, nil)
	req.Header.Set("User-Agent", "brotools (Kobo Elipsa)")
	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}

	start = time.Now()
	resp, err := client.Do(req)
	if err != nil {
		out <- netMsg{netHTTP, "failed", []string{"GET " + httpTestURL, "error: " + err.Error(), rootsNote()}, true}
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
	total := time.Since(start)

	ms := func(d time.Duration) int64 { return d.Milliseconds() }
	lines := []string{
		fmt.Sprintf("%s in %d ms from %s", resp.Status, ms(total), remote),
		fmt.Sprintf("dns %d ms · connect %d ms · tls %d ms · first byte %d ms", ms(dns), ms(conn), ms(tlsT), ms(first)),
	}
	if st := resp.TLS; st != nil && len(st.PeerCertificates) > 0 {
		c := st.PeerCertificates[0]
		lines = append(lines, fmt.Sprintf("%s, cert %s by %s", tls.VersionName(st.Version), c.Subject.CommonName, c.Issuer.CommonName))
	}
	lines = append(lines, rootsNote())
	var echo struct {
		Origin  string            `json:"origin"`
		Headers map[string]string `json:"headers"`
	}
	if json.Unmarshal(body, &echo) == nil && echo.Origin != "" {
		lines = append(lines, "public IP, as httpbin sees it: "+echo.Origin, "request headers it got:")
		names := slices.Sorted(maps.Keys(echo.Headers))
		for _, k := range names {
			lines = append(lines, fmt.Sprintf("   %s: %s", k, echo.Headers[k]))
		}
	} else {
		lines = append(lines, "")
		for _, l := range strings.Split(strings.TrimSpace(string(body)), "\n") {
			lines = append(lines, strings.ReplaceAll(l, "\t", "  "))
		}
	}
	out <- netMsg{netHTTP, fmt.Sprintf("%d · %d ms", resp.StatusCode, ms(total)), lines, true}
}

// rootsNote says where TLS trust comes from.
func rootsNote() string {
	if _, err := x509.SystemCertPool(); err == nil {
		for _, f := range []string{"/etc/ssl/certs/ca-certificates.crt", "/etc/ssl/cert.pem", "/etc/pki/tls/certs/ca-bundle.crt"} {
			if _, err := os.Stat(f); err == nil {
				return "CA roots: " + f
			}
		}
	}
	return "CA roots: built into brotools (Mozilla's set)"
}
