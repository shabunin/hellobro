package share

// The mDNS responder (RFC 6762, DNS-SD RFC 6763): "kobo.local" for the
// Kobo's Wi-Fi address, and the server as _sftp-ssh._tcp and _ssh._tcp,
// so a computer finds it by name, and a file manager lists it under
// Network. It answers on the Wi-Fi interface, follows the interface
// through Wi-Fi going off and on, and says goodbye when stopped. (It
// doesn't probe for a name already taken: one Kobo per network.)

import (
	"context"
	"log"
	"net"
	"strings"
	"syscall"
	"time"

	"golang.org/x/net/dns/dnsmessage"
	"golang.org/x/net/ipv4"
)

var mdnsGroup = &net.UDPAddr{IP: net.IPv4(224, 0, 0, 251), Port: 5353}

// Advert is what to advertise.
type Advert struct {
	Host     string // "kobo", for kobo.local
	Instance string // the service's name, as listed: "Kobo Elipsa"
	Port     int
	Iface    string   // "wlan0"
	TXT      []string // the services' TXT: "u=root", "path=/mnt/onboard"
}

const (
	servicesEnum = "_services._dns-sd._udp.local."
	hostTTL      = 120
	serviceTTL   = 4500
	flush        = 0x8000 // the class's cache-flush bit: these records are only ours
)

var serviceTypes = []string{"_sftp-ssh._tcp.local.", "_ssh._tcp.local."}

// zone is the records, for one address.
type zone struct {
	host, instance string // "kobo.local.", "Kobo Elipsa"
	ip             [4]byte
	port           uint16
	txt            []string
}

func (z zone) inst(st string) string { return z.instance + "." + st }

// records are all of them, ttl 0 for a goodbye.
func (z zone) records(ttl func(uint32) uint32) []dnsmessage.Resource {
	rr := []dnsmessage.Resource{z.a(ttl)}
	for _, st := range serviceTypes {
		rr = append(rr, z.ptr(servicesEnum, st, ttl), z.ptr(st, z.inst(st), ttl), z.srv(st, ttl), z.txtRec(st, ttl))
	}
	return rr
}

func hdr(name string, ttl uint32, exclusive bool) dnsmessage.ResourceHeader {
	class := dnsmessage.ClassINET
	if exclusive {
		class |= flush
	}
	return dnsmessage.ResourceHeader{Name: dnsmessage.MustNewName(name), Class: class, TTL: ttl}
}

func (z zone) a(ttl func(uint32) uint32) dnsmessage.Resource {
	return dnsmessage.Resource{Header: hdr(z.host, ttl(hostTTL), true), Body: &dnsmessage.AResource{A: z.ip}}
}

func (z zone) ptr(name, to string, ttl func(uint32) uint32) dnsmessage.Resource {
	return dnsmessage.Resource{Header: hdr(name, ttl(serviceTTL), false), Body: &dnsmessage.PTRResource{PTR: dnsmessage.MustNewName(to)}}
}

func (z zone) srv(st string, ttl func(uint32) uint32) dnsmessage.Resource {
	return dnsmessage.Resource{Header: hdr(z.inst(st), ttl(hostTTL), true),
		Body: &dnsmessage.SRVResource{Port: z.port, Target: dnsmessage.MustNewName(z.host)}}
}

func (z zone) txtRec(st string, ttl func(uint32) uint32) dnsmessage.Resource {
	txt := z.txt
	if len(txt) == 0 {
		txt = []string{""}
	}
	return dnsmessage.Resource{Header: hdr(z.inst(st), ttl(serviceTTL), true), Body: &dnsmessage.TXTResource{TXT: txt}}
}

func same(a dnsmessage.Name, b string) bool { return strings.EqualFold(a.String(), b) }

// answer is what to say to the questions: the answers, and the records
// that go with them (a service's SRV, TXT and address).
func (z zone) answer(qs []dnsmessage.Question) (ans, extra []dnsmessage.Resource) {
	keep := func(t uint32) uint32 { return t }
	all := z.records(keep)
	has := func(list []dnsmessage.Resource, r dnsmessage.Resource) bool {
		for _, o := range list {
			if o.Header.Name == r.Header.Name && typeOf(o) == typeOf(r) && o.Body.GoString() == r.Body.GoString() {
				return true
			}
		}
		return false
	}
	for _, q := range qs {
		for _, r := range all {
			t := typeOf(r)
			if same(q.Name, r.Header.Name.String()) && (q.Type == t || q.Type == dnsmessage.TypeALL) && !has(ans, r) {
				ans = append(ans, r)
			}
		}
	}
	for _, r := range ans {
		switch b := r.Body.(type) {
		case *dnsmessage.PTRResource:
			for _, st := range serviceTypes {
				if same(b.PTR, z.inst(st)) {
					for _, x := range []dnsmessage.Resource{z.srv(st, keep), z.txtRec(st, keep), z.a(keep)} {
						if !has(ans, x) && !has(extra, x) {
							extra = append(extra, x)
						}
					}
				}
			}
		case *dnsmessage.SRVResource:
			if x := z.a(keep); !has(ans, x) && !has(extra, x) {
				extra = append(extra, x)
			}
		}
	}
	return ans, extra
}

func typeOf(r dnsmessage.Resource) dnsmessage.Type {
	switch r.Body.(type) {
	case *dnsmessage.AResource:
		return dnsmessage.TypeA
	case *dnsmessage.PTRResource:
		return dnsmessage.TypePTR
	case *dnsmessage.SRVResource:
		return dnsmessage.TypeSRV
	case *dnsmessage.TXTResource:
		return dnsmessage.TypeTXT
	}
	return 0
}

// Advertise answers for ad until stop is called.
func Advertise(ad Advert) (stop func()) {
	quit := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		var r *responder
		check := time.NewTicker(5 * time.Second)
		defer check.Stop()
		for {
			ifc, ip := ifaceIPv4(ad.Iface)
			if r != nil && (ip == nil || !ip.Equal(r.ip) || ifc.Index != r.ifc.Index) {
				log.Printf("mdns: %s changed: answering again", ad.Iface)
				r.close(true)
				r = nil
			}
			if r == nil && ip != nil {
				var err error
				if r, err = respond(ad, ifc, ip); err != nil {
					log.Printf("mdns: %v", err)
					r = nil
				}
			}
			select {
			case <-quit:
				if r != nil {
					r.close(true)
				}
				return
			case <-check.C:
			}
		}
	}()
	return func() {
		close(quit)
		<-done
	}
}

// IPv4 is the interface's address, or "".
func IPv4(iface string) string {
	if _, ip := ifaceIPv4(iface); ip != nil {
		return ip.String()
	}
	return ""
}

func ifaceIPv4(name string) (*net.Interface, net.IP) {
	ifc, err := net.InterfaceByName(name)
	if err != nil || ifc.Flags&net.FlagUp == 0 {
		return nil, nil
	}
	addrs, _ := ifc.Addrs()
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil {
			return ifc, n.IP.To4()
		}
	}
	return nil, nil
}

type responder struct {
	pc  *ipv4.PacketConn
	ifc *net.Interface
	ip  net.IP
	z   zone
}

func respond(ad Advert, ifc *net.Interface, ip net.IP) (*responder, error) {
	lc := net.ListenConfig{Control: func(_, _ string, c syscall.RawConn) error {
		var serr error
		c.Control(func(fd uintptr) {
			serr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
			syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, 0xf /* SO_REUSEPORT */, 1)
		})
		return serr
	}}
	c, err := lc.ListenPacket(context.Background(), "udp4", "0.0.0.0:5353")
	if err != nil {
		return nil, err
	}
	pc := ipv4.NewPacketConn(c)
	if err := pc.JoinGroup(ifc, mdnsGroup); err != nil {
		c.Close()
		return nil, err
	}
	pc.SetMulticastInterface(ifc)
	pc.SetMulticastTTL(255)
	pc.SetControlMessage(ipv4.FlagInterface, true)
	r := &responder{pc: pc, ifc: ifc, ip: ip, z: zone{host: ad.Host + ".local.", instance: ad.Instance, port: uint16(ad.Port), txt: ad.TXT}}
	copy(r.z.ip[:], ip.To4())
	log.Printf("mdns: %s.local is %s, services on port %d", ad.Host, ip, ad.Port)
	go r.read()
	go func() { // announce, twice (RFC 6762 8.3)
		r.announce(false)
		time.Sleep(time.Second)
		r.announce(false)
	}()
	return r, nil
}

func (r *responder) announce(goodbye bool) {
	ttl := func(t uint32) uint32 { return t }
	if goodbye {
		ttl = func(uint32) uint32 { return 0 }
	}
	r.send(dnsmessage.Message{Header: dnsmessage.Header{Response: true, Authoritative: true}, Answers: r.z.records(ttl)}, mdnsGroup)
}

func (r *responder) close(goodbye bool) {
	if goodbye {
		r.announce(true)
	}
	r.pc.Close()
}

func (r *responder) send(m dnsmessage.Message, to *net.UDPAddr) {
	b, err := m.Pack()
	if err != nil {
		log.Printf("mdns: %v", err)
		return
	}
	if _, err := r.pc.WriteTo(b, nil, to); err != nil {
		log.Printf("mdns: sending: %v", err)
	}
}

func (r *responder) read() {
	buf := make([]byte, 9000)
	for {
		n, cm, src, err := r.pc.ReadFrom(buf)
		if err != nil {
			return // closed
		}
		if cm != nil && cm.IfIndex != 0 && cm.IfIndex != r.ifc.Index {
			continue // another interface's
		}
		var m dnsmessage.Message
		if m.Unpack(buf[:n]) != nil || m.Header.Response || len(m.Questions) == 0 {
			continue
		}
		ans, extra := r.z.answer(m.Questions)
		if len(ans) == 0 {
			continue
		}
		from, _ := src.(*net.UDPAddr)
		reply := dnsmessage.Message{Header: dnsmessage.Header{Response: true, Authoritative: true}, Answers: ans, Additionals: extra}
		to := mdnsGroup
		if from != nil && from.Port != 5353 { // a plain DNS client asking (dig -p 5353): answer it alone
			reply.Header.ID, reply.Questions, to = m.Header.ID, m.Questions, from
		}
		r.send(reply, to)
	}
}
