package audio

// Headphones' buttons (AVRCP), when Nickel has a media player registered
// with BlueZ: BlueZ then sends the presses to that player as D-Bus calls
// (Play, Pause, Next…) instead of as keys from the "(AVRCP)" input
// device, and Nickel, frozen while we run, never takes them. So we watch
// the bus for them, as a monitor (root may), and play them ourselves.

import (
	"fmt"
	"log"

	"github.com/godbus/dbus/v5"
)

const mprisPlayer = "org.mpris.MediaPlayer2.Player" // what BlueZ calls a registered player on

// WatchPlayerCalls calls cmd with each command BlueZ sends a registered
// media player: Play, Pause, PlayPause, Stop, Next, Previous. cmd runs
// on the watcher's goroutine. It runs until the bus goes.
func WatchPlayerCalls(cmd func(member string)) error {
	conn, err := dbus.SystemBusPrivate()
	if err != nil {
		return err
	}
	if err := conn.Auth(nil); err != nil {
		conn.Close()
		return err
	}
	if err := conn.Hello(); err != nil {
		conn.Close()
		return err
	}
	var bluez string // its unique name: only its calls count; "" till bluetoothd runs
	conn.BusObject().Call("org.freedesktop.DBus.GetNameOwner", 0, "org.bluez").Store(&bluez)
	ch := make(chan *dbus.Message, 16)
	conn.Eavesdrop(ch)
	rules := []string{
		"type='method_call',interface='" + mprisPlayer + "'",
		"type='signal',interface='org.freedesktop.DBus',member='NameOwnerChanged',arg0='org.bluez'", // bluetoothd restarted
	}
	if err := conn.BusObject().Call("org.freedesktop.DBus.Monitoring.BecomeMonitor", 0, rules, uint32(0)).Err; err != nil {
		conn.Close()
		return fmt.Errorf("BecomeMonitor: %w", err)
	}
	log.Printf("audio: watching BlueZ's calls to media players (bluez is %s)", bluez)
	go func() {
		defer conn.Close()
		for msg := range ch {
			sender, _ := msg.Headers[dbus.FieldSender].Value().(string)
			member, _ := msg.Headers[dbus.FieldMember].Value().(string)
			iface, _ := msg.Headers[dbus.FieldInterface].Value().(string)
			if msg.Type == dbus.TypeSignal && member == "NameOwnerChanged" && len(msg.Body) == 3 {
				if name, _ := msg.Body[0].(string); name == "org.bluez" {
					bluez, _ = msg.Body[2].(string)
					log.Printf("audio: bluez is %q now", bluez)
				}
				continue
			}
			if msg.Type != dbus.TypeMethodCall || iface != mprisPlayer || sender != bluez || bluez == "" {
				continue
			}
			cmd(member)
		}
		log.Print("audio: the bus watch ended")
	}()
	return nil
}
