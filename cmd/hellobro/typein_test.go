package main

import (
	"testing"

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
