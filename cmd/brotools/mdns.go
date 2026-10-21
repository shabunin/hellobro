package main

// An mDNS / DNS-SD browser (RFC 6762/6763): join the 224.0.0.251 group,
// ask what service types exist, then who offers them and where.
// It doubles as a test that multicast gets through the Kobo's Wi-Fi.

import (
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

var mdnsGroup = &net.UDPAddr{IP: net.IPv4(224, 0, 0, 251), Port: 5353}

const servicesName = "_services._dns-sd._udp.local."

type mdnsInstance struct {
	name, typ string // "Living Room._googlecast._tcp.local.", "_googlecast._tcp.local."
	target    string // host name, from SRV
	port      uint16
	txt       []string
}

type mdnsBrowser struct {
	conn      *net.UDPConn
	types     map[string]bool
	instances map[string]*mdnsInstance
	addrs     map[string][]string // host name -> IPv4s
	packets   int
	senders   map[string]bool
}

func mdnsBrowse(out chan<- netMsg) {
	ifname := wifiIface()
	fail := func(err error) {
		out <- netMsg{netMDNS, "failed", []string{err.Error()}, true}
	}
	ifc, err := net.InterfaceByName(ifname)
	if err != nil {
		fail(err)
		return
	}
	conn, err := net.ListenMulticastUDP("udp4", ifc, mdnsGroup)
	if err != nil {
		fail(fmt.Errorf("joining %s on %s: %w", mdnsGroup.IP, ifname, err))
		return
	}
	defer conn.Close()
	b := &mdnsBrowser{conn: conn, types: map[string]bool{}, instances: map[string]*mdnsInstance{},
		addrs: map[string][]string{}, senders: map[string]bool{}}
	progress := func(what string) {
		out <- netMsg{netMDNS, what, b.report(ifname), false}
	}
	start := time.Now()

	// 1. Which service types are out there?
	progress("asking for service types…")
	b.query(dnsmessage.TypePTR, servicesName)
	b.listen(1500 * time.Millisecond)
	b.query(dnsmessage.TypePTR, servicesName) // again: multicast over Wi-Fi gets lost
	b.listen(1000 * time.Millisecond)

	// 2. Who offers them? Answers usually carry SRV, TXT and A records too.
	progress(fmt.Sprintf("asking %d service types…", len(b.types)))
	b.query(dnsmessage.TypePTR, sortedKeys(b.types)...)
	b.listen(2500 * time.Millisecond)

	// 3. Fill in what's still missing.
	var srv, a []string
	for _, in := range b.instances {
		if in.target == "" {
			srv = append(srv, in.name)
		} else if len(b.addrs[in.target]) == 0 {
			a = append(a, in.target)
		}
	}
	if len(srv)+len(a) > 0 {
		progress("asking for addresses…")
		b.query(dnsmessage.TypeSRV, srv...)
		b.query(dnsmessage.TypeA, a...)
		b.listen(1500 * time.Millisecond)
	}
	out <- netMsg{netMDNS, fmt.Sprintf("%d services · %.1fs", len(b.instances), time.Since(start).Seconds()), b.report(ifname), true}
}

// query multicasts questions, a few per packet.
func (b *mdnsBrowser) query(t dnsmessage.Type, names ...string) {
	for len(names) > 0 {
		n := min(len(names), 12)
		bld := dnsmessage.NewBuilder(nil, dnsmessage.Header{})
		bld.StartQuestions()
		for _, name := range names[:n] {
			dn, err := dnsmessage.NewName(name)
			if err != nil {
				continue
			}
			bld.Question(dnsmessage.Question{Name: dn, Type: t, Class: dnsmessage.ClassINET})
		}
		names = names[n:]
		if msg, err := bld.Finish(); err == nil {
			b.conn.WriteToUDP(msg, mdnsGroup)
		}
	}
}

// listen collects records from every mDNS packet for d.
func (b *mdnsBrowser) listen(d time.Duration) {
	deadline := time.Now().Add(d)
	buf := make([]byte, 9000)
	for {
		b.conn.SetReadDeadline(deadline)
		n, from, err := b.conn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		var p dnsmessage.Parser
		h, err := p.Start(buf[:n])
		if err != nil || !h.Response {
			continue // other hosts' questions
		}
		b.packets++
		b.senders[from.IP.String()] = true
		p.SkipAllQuestions()
		var rrs []dnsmessage.Resource
		if ans, err := p.AllAnswers(); err == nil {
			rrs = append(rrs, ans...)
		}
		p.SkipAllAuthorities()
		if add, err := p.AllAdditionals(); err == nil {
			rrs = append(rrs, add...)
		}
		for _, rr := range rrs {
			b.record(rr)
		}
	}
}

func (b *mdnsBrowser) record(rr dnsmessage.Resource) {
	name := strings.ToLower(rr.Header.Name.String())
	instance := func(n string) *mdnsInstance {
		key := strings.ToLower(n)
		in := b.instances[key]
		if in == nil {
			typ := ""
			if _, t, ok := strings.Cut(n, "._"); ok { // until a PTR says better
				typ = "_" + t
			}
			in = &mdnsInstance{name: n, typ: strings.ToLower(typ)}
			b.instances[key] = in
		}
		return in
	}
	switch body := rr.Body.(type) {
	case *dnsmessage.PTRResource:
		ptr := body.PTR.String()
		if name == servicesName {
			b.types[strings.ToLower(ptr)] = true
		} else {
			if _, base, ok := strings.Cut(name, "._sub."); ok { // a subtype: file it under the type
				name = base
			}
			b.types[name] = true
			instance(ptr).typ = name
		}
	case *dnsmessage.SRVResource:
		in := instance(rr.Header.Name.String())
		in.target, in.port = strings.ToLower(body.Target.String()), body.Port
	case *dnsmessage.TXTResource:
		instance(rr.Header.Name.String()).txt = body.TXT
	case *dnsmessage.AResource:
		ip := net.IP(body.A[:]).String()
		for _, have := range b.addrs[name] {
			if have == ip {
				return
			}
		}
		b.addrs[name] = append(b.addrs[name], ip)
	}
}

// report lists one service per line: "EPSON L3270  ipp  192.168.1.30:631".
func (b *mdnsBrowser) report(ifname string) []string {
	lines := []string{fmt.Sprintf("multicast %s on %s: %d replies from %d hosts, %d service types",
		mdnsGroup.IP, ifname, b.packets, len(b.senders), len(b.types))}
	ins := make([]*mdnsInstance, 0, len(b.instances))
	for _, in := range b.instances {
		ins = append(ins, in)
	}
	sort.Slice(ins, func(i, j int) bool {
		if a, b := strings.ToLower(ins[i].name), strings.ToLower(ins[j].name); a != b {
			return a < b
		}
		return ins[i].typ < ins[j].typ
	})
	for _, in := range ins {
		label := in.name
		if suffix := "." + in.typ; len(label) > len(suffix) && strings.EqualFold(label[len(label)-len(suffix):], suffix) {
			label = label[:len(label)-len(suffix)]
		}
		typ := strings.TrimPrefix(strings.TrimSuffix(in.typ, "._tcp.local."), "_")
		typ = strings.TrimSuffix(typ, ".local.")
		line := fmt.Sprintf("   %s  ·  %s", unescapeDNS(label), typ)
		if in.target != "" {
			host := strings.TrimSuffix(in.target, ".")
			if ips := b.addrs[in.target]; len(ips) > 0 {
				host = ips[0]
			}
			line += fmt.Sprintf("  ·  %s:%d", host, in.port)
		}
		lines = append(lines, line)
	}
	return lines
}

// unescapeDNS turns "Living\032Room" back into "Living Room".
func unescapeDNS(s string) string {
	var out strings.Builder
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '\\' && i+3 < len(s) && isDigit(s[i+1]) && isDigit(s[i+2]) && isDigit(s[i+3]):
			out.WriteByte((s[i+1]-'0')*100 + (s[i+2]-'0')*10 + s[i+3] - '0')
			i += 3
		case s[i] == '\\' && i+1 < len(s):
			out.WriteByte(s[i+1])
			i++
		default:
			out.WriteByte(s[i])
		}
	}
	return out.String()
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
