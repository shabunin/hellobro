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
	"strings"
	"sync"
	"time"

	"hellobro/keyfeed"
	"hellobro/kobo"
	"hellobro/share"
)

const (
	nearbyWait  = 1500 * time.Millisecond // how long the answers take
	nearbyStale = 20 * time.Second        // a launcher drawn after that looks again
	askKeysWait = time.Minute             // the question about a device stays this long
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

// --- typing from here ---

type nearbyState struct {
	peers   []share.Peer
	at      time.Time // when it last looked
	looking bool
}

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

// typeInto connects t to the device p, to type into it.
func (a *bro) typeInto(t *termSession, p share.Peer) {
	a.termNote(t, "the first time, "+p.Name+" asks whether this one may type there: tap Allow on it")
	a.termConnect(t, sshHost{name: p.Name, user: share.KeysUser, addr: p.Addr, cmd: share.KeysUser})
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
func (a *bro) askKeys(who, addr string) bool {
	answer := make(chan bool, 1)
	var q *askBox
	a.Do(func() {
		if a.Locked() || a.asking != nil {
			answer <- false
			return
		}
		a.choose(who+" wants to type here", "It's at "+addr+" on the network. Allowed, it may type here from now on, and do nothing else (Settings, Sharing: forget it).",
			askOpt{"allow", "a", func() { answer <- true }}, askOpt{"no", "n", func() { answer <- false }})
		q = a.asking
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

// isKeys says whether cmd asks for the keyboard.
func isKeys(cmd string) bool { return strings.TrimSpace(cmd) == share.KeysUser }

// keysSession is a keyboard here (uinput) made of what another device's
// terminal sends: hellobro-keys. Ctrl+] ends it.
type keysSession struct {
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
	select {
	case <-s.done:
		return 0, io.EOF
	default:
	}
	if err := s.kb.Send(s.dec.Feed(p)); err != nil {
		log.Printf("keys: %v", err)
	}
	if s.dec.Quit {
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
		s.kb.Send(s.dec.ReleaseAll())
		s.kb.Close()
		if !s.dec.Kitty {
			s.say <- "(a plain terminal: no Super, so hellobro's Mod key is best Alt here: Settings, Keyboard)\r\n"
		}
		s.say <- keyfeed.KittyOff + "bye\r\n"
		close(s.say)
		close(s.done)
		log.Print("keys: done")
	})
}
