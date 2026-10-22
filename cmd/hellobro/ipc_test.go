package main

import (
	"bytes"
	"image"
	"strings"
	"testing"

	"hellobro/wm"
)

func TestIPCFrames(t *testing.T) {
	var b bytes.Buffer
	ipcWrite(&b, ipcGetTree, []byte(`{"a":1}`))
	ipcWrite(&b, ipcEvent|3, nil)
	if !strings.HasPrefix(b.String(), "i3-ipc\x07\x00\x00\x00\x04\x00\x00\x00{") { // little-endian, as the Kobo and the PC are
		t.Errorf("framed: %q", b.String())
	}
	typ, p, err := ipcRead(&b)
	if err != nil || typ != ipcGetTree || string(p) != `{"a":1}` {
		t.Errorf("read: %d %q %v", typ, p, err)
	}
	typ, p, err = ipcRead(&b)
	if err != nil || typ != ipcEvent|3 || len(p) != 0 {
		t.Errorf("event: %x %q %v", typ, p, err)
	}
	if _, _, err := ipcRead(strings.NewReader("i3-ipX\x00\x00\x00\x00\x00\x00\x00\x00")); err == nil {
		t.Error("bad magic read")
	}
	if _, _, err := ipcRead(strings.NewReader("i3-ipc\xff\xff\xff\x7f\x00\x00\x00\x00")); err == nil {
		t.Error("a huge message read")
	}
}

// What comes over the socket is a jail's: of hellobro's own commands it
// runs only what can't reach out of one.
func TestIPCHost(t *testing.T) {
	a := &bro{}
	a.wm.tree = wm.New(image.Rect(0, 0, 1000, 800), wmTabH)
	a.wm.wins = map[*wm.Node]window{}
	h := ipcHost{wmHost{a}}
	for _, c := range []struct {
		name string
		args []string
		ok   bool
	}{
		{"open", []string{"sh"}, false},
		{"open", []string{"ssh", "box"}, false},
		{"paste", nil, false},
		{"paste.key", nil, false},
		{"copy.screen", nil, false},
		{"copy", nil, false},
		{"sharing", nil, false},
		{"relay", nil, false},
		{"quit", nil, false},
		{"exit", nil, false},
		{"wifi", nil, false},
		{"music.play", []string{"now"}, false},
	} {
		known, err := h.Command(nil, c.name, c.args)
		if !known || err == nil || !strings.Contains(err.Error(), "not over IPC") {
			t.Errorf("%s %v: %v %v, want refused", c.name, c.args, known, err)
		}
	}
	if known, err := h.Command(nil, "notify", nil); !known || err == nil || strings.Contains(err.Error(), "not over IPC") {
		t.Errorf("notify: %v %v, want let through (and nothing to say)", known, err)
	}
	if known, _ := h.Command(nil, "frobnicate", nil); known {
		t.Error("frobnicate known")
	}
	for _, s := range []string{"setting.shell", "alpine-remove"} {
		if commandByID(s) != nil && ipcCommands[s] {
			t.Errorf("%s allowed", s)
		}
	}
}
