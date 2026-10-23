package share

// Browse: the other hellobros on the network, as their mDNS responders
// answer for _hellobro._tcp. The question goes from a port of our own,
// not 5353, so each answers us alone (RFC 6762 6.7, a "legacy unicast"
// query): no group to join, and nothing heard but the answers.

import (
	"errors"
	"net"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/dns/dnsmessage"
	"golang.org/x/net/ipv4"
)

// Peer is another hellobro heard on the network.
type Peer struct {
	Name string // its instance: "Kobo Elipsa"
	Addr string // where its server is: "192.168.1.20:2022"
}

// Browse asks on iface who's there, listens for wait, and returns who
// answered, but us. It asks twice, for a packet lost.
func Browse(iface string, wait time.Duration) ([]Peer, error) {
	ifc, ip := ifaceIPv4(iface)
	if ip == nil {
		return nil, errors.New("no address on " + iface)
	}
	c, err := net.ListenPacket("udp4", net.JoinHostPort(ip.String(), "0"))
	if err != nil {
		return nil, err
	}
	defer c.Close()
	pc := ipv4.NewPacketConn(c)
	pc.SetMulticastInterface(ifc)
	pc.SetMulticastTTL(255)
	q := dnsmessage.Message{Header: dnsmessage.Header{ID: uint16(time.Now().UnixNano())},
		Questions: []dnsmessage.Question{{Name: dnsmessage.MustNewName(HelloType), Type: dnsmessage.TypePTR, Class: dnsmessage.ClassINET}}}
	b, err := q.Pack()
	if err != nil {
		return nil, err
	}
	var heard browsed
	for _, until := range []time.Time{time.Now().Add(wait / 3), time.Now().Add(wait)} {
		if _, err := pc.WriteTo(b, nil, mdnsGroup); err != nil {
			return nil, err
		}
		heard.read(c, until)
	}
	return heard.peers(ip), nil
}

// browsed is what the answers said: instances, their SRV, addresses.
type browsed struct {
	inst  []string
	srv   map[string]dnsmessage.SRVResource
	addrs map[string]net.IP
}

// read takes answers in till until.
func (h *browsed) read(c net.PacketConn, until time.Time) {
	buf := make([]byte, 9000)
	c.SetReadDeadline(until)
	for {
		n, _, err := c.ReadFrom(buf)
		if err != nil {
			return
		}
		h.take(buf[:n])
	}
}

// take reads one answer.
func (h *browsed) take(p []byte) {
	var m dnsmessage.Message
	if m.Unpack(p) != nil || !m.Header.Response {
		return
	}
	if h.srv == nil {
		h.srv, h.addrs = map[string]dnsmessage.SRVResource{}, map[string]net.IP{}
	}
	for _, r := range append(m.Answers, m.Additionals...) {
		name := strings.ToLower(r.Header.Name.String())
		switch b := r.Body.(type) {
		case *dnsmessage.PTRResource:
			if name == HelloType && !slices.Contains(h.inst, b.PTR.String()) {
				h.inst = append(h.inst, b.PTR.String())
			}
		case *dnsmessage.SRVResource:
			h.srv[name] = *b
		case *dnsmessage.AResource:
			h.addrs[name] = net.IP(b.A[:])
		}
	}
}

// peers are the instances whose server and address came too, but the
// one at self.
func (h *browsed) peers(self net.IP) []Peer {
	var out []Peer
	for _, in := range h.inst {
		srv, ok := h.srv[strings.ToLower(in)]
		if !ok {
			continue
		}
		ip := h.addrs[strings.ToLower(srv.Target.String())]
		if ip == nil || ip.Equal(self) {
			continue
		}
		out = append(out, Peer{Name: strings.TrimSuffix(in, "."+HelloType), Addr: net.JoinHostPort(ip.String(), strconv.Itoa(int(srv.Port)))})
	}
	slices.SortFunc(out, func(a, b Peer) int { return strings.Compare(a.Name, b.Name) })
	return out
}
