package main

import (
	"crypto/ed25519"
	"testing"

	"golang.org/x/crypto/ssh"

	"hellobro/share"
)

// A window typing into another device grabs the keys when it connects or
// gets the focus; let go, it stays so till the focus comes back; done
// typing, it lets go; a grab ends with the focus going elsewhere.
func TestGrabFor(t *testing.T) {
	typing := &termSession{kind: kindSSH, state: termConnected, target: sshHost{user: share.KeysUser}}
	sh := &termSession{kind: kindShell, state: termConnected}
	var grab, was window
	step := func(name string, focus window, want window) {
		t.Helper()
		grab, was = grabFor(focus, grab, was)
		if grab != want {
			t.Errorf("%s: grab %v, want %v", name, grab, want)
		}
	}
	step("the shell in focus", sh, nil)
	grab = sh // Meta+Shift+G in it
	step("still the shell", sh, sh)
	step("focus to the typing window", typing, typing)
	grab = nil // Meta+Shift+G: let go
	step("let go, still in focus", typing, nil)
	step("focus to the shell", sh, nil)
	step("focus back", typing, typing)
	typing.state = termIdle
	step("done typing", typing, nil)
	typing.state = termConnected
	step("connected again", typing, typing)
}

func TestOwnAddress(t *testing.T) {
	for ip, want := range map[string]bool{"127.0.0.1": true, "::1": true, "203.0.113.7": false, "nonsense": false} {
		if got := ownAddress(ip); got != want {
			t.Errorf("%s: %v, want %v", ip, got, want)
		}
	}
}

// A nearby device is known by its key: its name alone is anyone's.
func TestPeerNote(t *testing.T) {
	a := &bro{}
	a.mem.Devices = map[string]string{"SHA256:elipsa-key": "Kobo Elipsa"}
	for _, c := range []struct {
		p    share.Peer
		want string
	}{
		{share.Peer{Name: "Kobo Elipsa", Key: "SHA256:elipsa-key"}, "nearby: type into it"},
		{share.Peer{Name: "Desk", Key: "SHA256:elipsa-key"}, "nearby: typed into before, as Kobo Elipsa"},
		{share.Peer{Name: "Kobo Elipsa", Key: "SHA256:another-key"}, "NOT the Kobo Elipsa typed into before: another key, …ther-key"},
		{share.Peer{Name: "Libra", Key: "SHA256:libra-key-1"}, "nearby, new: key …ra-key-1"},
		{share.Peer{Name: "Old"}, "nearby: type into it (an older hellobro)"},
	} {
		if got := a.peerNote(c.p); got != c.want {
			t.Errorf("%v: %q, want %q", c.p, got, c.want)
		}
	}
}

// A device's key, as it said it is, is the one it must have.
func TestHostKeyPinned(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(nil)
	s, _ := ssh.NewSignerFromKey(priv)
	fp := ssh.FingerprintSHA256(s.PublicKey())
	if err := hostKeys(sshHost{key: fp}, nil)("x", nil, s.PublicKey()); err != nil {
		t.Errorf("its own key: %v", err)
	}
	if err := hostKeys(sshHost{key: "SHA256:another"}, nil)("x", nil, s.PublicKey()); err == nil {
		t.Errorf("another key taken")
	}
}
