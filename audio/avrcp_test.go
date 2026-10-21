package audio

import (
	"os"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

// TestWatchPlayerCalls plays BlueZ on the session bus: a "bluez" calls
// a player's Next, a stranger its Play; only the first is seen, and the
// watch starts without waiting on anything.
func TestWatchPlayerCalls(t *testing.T) {
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		t.Skip("no session bus")
	}
	fake, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Skip(err)
	}
	defer fake.Close()
	const name = "test.hellobro.bluez"
	if r, err := fake.RequestName(name, dbus.NameFlagDoNotQueue); err != nil || r != dbus.RequestNameReplyPrimaryOwner {
		t.Skip("can't own ", name, err)
	}
	mon, err := dbus.SessionBusPrivate()
	if err != nil {
		t.Fatal(err)
	}
	got := make(chan string, 4)
	done := make(chan error)
	go func() { done <- watchPlayerCalls(mon, name, func(m string) { got <- m }) }()
	select {
	case err := <-done:
		if err != nil {
			t.Skip("monitoring not allowed here: ", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the watch is stuck starting")
	}
	time.Sleep(300 * time.Millisecond) // the monitor rule in place
	stranger, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Fatal(err)
	}
	defer stranger.Close()
	stranger.Object(fake.Names()[0], "/player").Go(mprisPlayer+".Play", dbus.FlagNoReplyExpected, nil)
	fake.Object(stranger.Names()[0], "/player").Go(mprisPlayer+".Next", dbus.FlagNoReplyExpected, nil)
	select {
	case m := <-got:
		if m != "Next" {
			t.Fatalf("saw %s, not bluez's Next", m)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("saw nothing")
	}
	select {
	case m := <-got:
		t.Fatalf("also saw %s", m)
	case <-time.After(300 * time.Millisecond):
	}
}
