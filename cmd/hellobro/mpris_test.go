package main

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

// A jail's bus is dialed with no symlink followed: one there, absolute,
// would be the Kobo's own path.
func TestDialBusIn(t *testing.T) {
	root, err := os.MkdirTemp("", "bus") // short: a socket's path has 108 bytes
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	dir := filepath.Join(root, filepath.Dir(jailBus))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	l, err := net.Listen("unix", filepath.Join(root, jailBus))
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	c, err := dialBusIn(root)
	if err != nil {
		t.Fatalf("the socket: %v", err)
	}
	c.Close()

	// The bus a symlink to the socket, then its directory one.
	evil, err := os.MkdirTemp("", "bus")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(evil)
	edir := filepath.Join(evil, filepath.Dir(jailBus))
	os.MkdirAll(edir, 0o755)
	os.Symlink(filepath.Join(root, jailBus), filepath.Join(evil, jailBus))
	if c, err := dialBusIn(evil); err == nil {
		c.Close()
		t.Error("a symlinked bus dialed")
	}
	os.RemoveAll(filepath.Join(evil, "run"))
	os.Symlink(filepath.Join(root, "run"), filepath.Join(evil, "run"))
	if c, err := dialBusIn(evil); err == nil {
		c.Close()
		t.Error("a bus under a symlinked directory dialed")
	}
	// Not a socket.
	os.Remove(filepath.Join(evil, "run"))
	os.MkdirAll(edir, 0o755)
	os.WriteFile(filepath.Join(evil, jailBus), nil, 0o644)
	if c, err := dialBusIn(evil); err == nil {
		c.Close()
		t.Error("a file dialed")
	}
}
