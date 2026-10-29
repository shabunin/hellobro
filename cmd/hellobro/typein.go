package main

// Typing from one device into another, both running hellobro. Each
// says it's there over mDNS (_hellobro._tcp, with sharing on), and the
// launcher lists the others it hears: picked, it logs in to that one as
// hellobro-keys, whose server makes a keyboard of what comes (keyfeed,
// uinput), for its hellobro, which takes it for any other keyboard. The
// first time, that device asks on its screen whether this one may type
// there; allowed, our key is kept there for typing only (share's
// command="…"). The window is a terminal running hellobro-keys there:
// while it's in focus every key goes to it, as grabbed (Meta+Shift+G
// gives them back, for a moment). Kitty's protocol carries them whole,
// Super and the releases too.

import (
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"hellobro/keyfeed"
	"hellobro/kobo"
	"hellobro/share"
)

const (
	nearbyWait  = 1500 * time.Millisecond // how long the answers take
	nearbyStale = 20 * time.Second        // a launcher drawn after that looks again
	askKeysWait = time.Minute             // the question about a device stays this long
	askKeysCalm = 2 * time.Second         // keys don't answer it till then: it came unasked, mid-typing maybe
	askKeysOff  = 10 * time.Minute        // after a no, the same address isn't asked again for this long
)

// deviceName is what other devices call this one: HELLOBRO_DEVICE_NAME,
// or the model FBInk knows ("Kobo Elipsa").
func (a *bro) deviceName() string {
	if n := strings.TrimSpace(kobo.EnvOr("HELLOBRO_DEVICE_NAME", "")); n != "" {
		return n
	}
	n, _, _ := strings.Cut(a.Screen().Device, " (")
	if n = strings.TrimSpace(n); n == "" || n == "Kobo" {
		return "Kobo"
	}
	return n
}

// sshClientVersion is our SSH client's: another hellobro shows who's
// asking to type by it (share.PeerName).
var sshClientVersion = "SSH-2.0-hellobro"

// deviceLabel is this device's name, for what's said where there's no
// bro (Alpine's motd).
var deviceLabel = "Kobo"

// --- typing from here ---

type nearbyState struct {
	peers   []share.Peer
	at      time.Time // when it last looked
	looking bool
	refused map[string]time.Time // the addresses told no about typing here, and when
}

// nearbyMax is how many other hellobros the launcher shows: what the
// network says is a stranger's word, and a flood of it isn't tiles.
const nearbyMax = 16

// nearbyLook has another look around, if the last one is old: for a
// launcher being drawn.
func (a *bro) nearbyLook() {
	n := &a.tv.nearby
	if n.looking || time.Since(n.at) < nearbyStale || a.share.srv == nil {
		return
	}
	n.looking, n.at = true, time.Now()
	go func() {
		peers, err := share.Browse(shareIf, nearbyWait)
		if err != nil {
			log.Printf("nearby: %v", err)
		}
		peers = peers[:min(len(peers), nearbyMax)]
		a.Do(func() {
			n.looking, n.at = false, time.Now()
			if !samePeers(peers, n.peers) {
				log.Printf("nearby: %v", peers)
				n.peers = peers
				a.redrawLaunchers()
			}
		})
	}()
}

func samePeers(a, b []share.Peer) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// redrawLaunchers draws the launchers that show again.
func (a *bro) redrawLaunchers() {
	for _, w := range a.wm.wins {
		if t, ok := w.(*termSession); ok && t.launcher() && a.onScreen(t) {
			t.pane.shownConnect = nil
			a.drawPane(t)
		}
	}
}

// typeInto connects t to the device p, to type into it: the device whose
// key it says it has, if it says (an older hellobro doesn't).
func (a *bro) typeInto(t *termSession, p share.Peer) {
	mine := ""
	if s, err := clientKey(); err == nil {
		mine = ", if it shows this one's key, " + shortKey(ssh.FingerprintSHA256(s.PublicKey()))
	}
	a.termNote(t, "the first time, "+p.Name+" asks whether this one may type there: tap allow on it"+mine)
	a.termConnect(t, sshHost{name: p.Name, user: share.KeysUser, addr: p.Addr, cmd: share.KeysUser, key: p.Key})
}

// shortKey is the end of a key's fingerprint, enough to tell two apart
// by eye: "…3fQx9kLm".
func shortKey(fp string) string {
	if len(fp) > 8 {
		return "…" + fp[len(fp)-8:]
	}
	return fp
}

// peerNote says on p's tile who it is: a device typed into before (by
// its key), a new one, or one with the name of another typed into, but
// not its key.
func (a *bro) peerNote(p share.Peer) string {
	known := a.mem.Devices
	switch {
	case p.Key == "":
		return "nearby: type into it (an older hellobro)"
	case known[p.Key] == p.Name:
		return "nearby: type into it"
	case known[p.Key] != "":
		return "nearby: typed into before, as " + known[p.Key]
	}
	for _, n := range known {
		if n == p.Name {
			return "NOT the " + p.Name + " typed into before: another key, " + shortKey(p.Key)
		}
	}
	return "nearby, new: key " + shortKey(p.Key)
}

// rememberDevice keeps the device typed into, by its key.
func (a *bro) rememberDevice(key, name string) {
	if a.mem.Devices[key] == name {
		return
	}
	if a.mem.Devices == nil {
		a.mem.Devices = map[string]string{}
	}
	a.mem.Devices[key] = name
	a.mem.save()
	log.Printf("keys: %s is %s", name, key)
}

// typing says t is typing into another device.
func (t *termSession) typing() bool {
	return t != nil && t.kind == kindSSH && t.target.user == share.KeysUser && t.connected()
}

// grabbing says the keys go to the window in focus, all of them.
func (a *bro) grabbing() bool {
	return a.wm.grab != nil && a.wm.grab == a.focusedWin()
}

// grabTyping is the grab as the focus and the connections have it now.
func (a *bro) grabTyping() {
	a.wm.grab, a.wm.typing = grabFor(a.focusedWin(), a.wm.grab, a.wm.typing)
	a.showMode()
}

// grabFor is the grab with w in focus: a window typing into another
// device grabs the keys when it gets the focus (or connects in it), and
// lets go when it's done; any grab ends with the focus going elsewhere.
// typing is the window in focus that was typing, last time we looked:
// once let go (Meta+Shift+G), it's not grabbed again till it's back.
func grabFor(w, grab, typing window) (window, window) {
	t, _ := w.(*termSession)
	switch {
	case t.typing() && typing != w:
		grab = w
	case grab == w && typing == w && !t.typing():
		grab = nil // done typing there
	case grab != w:
		grab = nil
	}
	if t.typing() {
		return grab, w
	}
	return grab, nil
}

// --- typed into, here ---

// askKeys asks on the screen whether who may type here (the share
// server's AskKeys, from its goroutine): no while locked, or no answer.
// A keyboard typing here can do what the keys do, a root shell among
// them, so it's asked carefully: no is what Enter says, the keys wait a
// moment (a tap doesn't), an address told no isn't asked again for a
// while, and this device's own (a program in Alpine) is never asked.
func (a *bro) askKeys(who, addr, key string) bool {
	if ownAddress(addr) {
		log.Printf("keys: %s (%s) asked to type here: no, it's this device", who, addr)
		return false
	}
	answer := make(chan bool, 1)
	var q *askBox
	a.Do(func() {
		n := &a.tv.nearby
		if a.Locked() || a.asking != nil || time.Since(n.refused[addr]) < askKeysOff {
			answer <- false
			return
		}
		no := func() {
			if n.refused == nil {
				n.refused = map[string]time.Time{}
			}
			n.refused[addr] = time.Now()
			answer <- false
		}
		a.choose(who+" wants to type here", "It's at "+addr+" on the network, its key "+shortKey(key)+" (it shows its own as it asks: the same?). Allowed, it may type here from now on, as a keyboard can: any key, a shell too (Settings, Sharing: forget it).",
			askOpt{"allow", "", func() { answer <- true }}, askOpt{"no", "n", no})
		q = a.asking
		q.sel, q.keysAt = 1, time.Now().Add(askKeysCalm)
		a.drawAsk()
	})
	select {
	case ok := <-answer:
		log.Printf("keys: %s (%s) asked to type here: %v", who, addr, ok)
		return ok
	case <-time.After(askKeysWait):
		a.Do(func() {
			if a.asking == q && q != nil {
				a.answer(-1)
			}
		})
		log.Printf("keys: %s (%s) asked to type here: no answer", who, addr)
		return false
	}
}

// ownAddress says whether ip is one of this device's.
func ownAddress(ip string) bool {
	p := net.ParseIP(ip)
	if p == nil {
		return false
	}
	if p.IsLoopback() {
		return true
	}
	addrs, _ := net.InterfaceAddrs()
	for _, ad := range addrs {
		if n, ok := ad.(*net.IPNet); ok && n.IP.Equal(p) {
			return true
		}
	}
	return false
}

// isKeys says whether cmd asks for the keyboard.
func isKeys(cmd string) bool { return strings.TrimSpace(cmd) == share.KeysUser }

// keysSession is a keyboard here (uinput) made of what another device's
// terminal sends: hellobro-keys. Ctrl+] ends it.
type keysSession struct {
	mu   sync.Mutex // the keyboard and the decoder: the keys come on one goroutine, the end on another
	kb   *keyfeed.Keyboard
	dec  keyfeed.Decoder
	r    *io.PipeReader
	say  chan string
	once sync.Once
	done chan struct{}
}

func startKeys(here string) (share.Shell, error) {
	kb, err := keyfeed.NewKeyboard("hellobro keys (ssh)")
	if err != nil {
		return nil, err
	}
	r, w := io.Pipe()
	s := &keysSession{kb: kb, r: r, say: make(chan string, 16), done: make(chan struct{})}
	go func() { // what we say, in order, never in the way of the keys
		for m := range s.say {
			w.Write([]byte(m))
		}
		w.Close()
	}()
	s.say <- keyfeed.KittyOn + "hellobro keys: what you type here goes to " + here + ". Ctrl+] to stop.\r\n"
	log.Print("keys: a keyboard over ssh")
	return s, nil
}

// Write is what's typed.
func (s *keysSession) Write(p []byte) (int, error) {
	s.mu.Lock()
	select {
	case <-s.done:
		s.mu.Unlock()
		return 0, io.EOF
	default:
	}
	if err := s.kb.Send(s.dec.Feed(p)); err != nil {
		log.Printf("keys: %v", err)
	}
	quit := s.dec.Quit
	s.mu.Unlock()
	if quit {
		s.finish()
	}
	return len(p), nil
}

func (s *keysSession) Read(p []byte) (int, error) { return s.r.Read(p) }
func (s *keysSession) Resize(cols, rows int)      {}

func (s *keysSession) Wait() error {
	<-s.done
	return nil
}

func (s *keysSession) Close() error {
	s.finish()
	return nil
}

func (s *keysSession) finish() {
	s.once.Do(func() {
		s.mu.Lock()
		s.kb.Send(s.dec.ReleaseAll())
		s.kb.Close()
		kitty := s.dec.Kitty
		close(s.done)
		s.mu.Unlock()
		if !kitty {
			s.say <- "(a plain terminal: no Super, so hellobro's Mod key is best Alt here: Settings, Keyboard)\r\n"
		}
		s.say <- keyfeed.KittyOff + "bye\r\n"
		close(s.say)
		time.AfterFunc(5*time.Second, func() { s.r.Close() }) // nobody reading: the writer above goes
		log.Print("keys: done")
	})
}
