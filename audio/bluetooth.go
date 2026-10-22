// Package audio plays sound on the Kobo: over Bluetooth, the only way it
// has, through bluez-alsa. It finds the devices and the PCM to play into,
// decodes MP3 and WAV, and runs a player with pause, seeking and volume.
package audio

// Bluetooth audio through the Kobo's own stack: BlueZ knows the devices,
// bluez-alsa (bluealsa) turns a connected A2DP sink into a PCM that we
// open over D-Bus and write samples into. Pairing stays Nickel's job.
//
// Kobo doesn't document which bluez-alsa it ships, so this probes: D-Bus
// names, the PCM listing of v4 (ObjectManager) and v2/v3 (Manager1), and
// Open with or without a mode. What it finds goes to the log.

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/godbus/dbus/v5"
)

const (
	uuidAudioSink = "0000110b-0000-1000-8000-00805f9b34fb"
	bluealsaPath  = "/org/bluealsa"
)

// Device is a paired or connected Bluetooth device.
type Device struct {
	Path                       dbus.ObjectPath
	Name, Addr                 string
	Paired, Connected, IsAudio bool
}

// PCM is one of bluealsa's audio streams.
type PCM struct {
	service  string
	Path     dbus.ObjectPath
	Device   dbus.ObjectPath
	Mode     string // "sink": we play into it
	Codec    string
	Format   uint16
	Channels int
	Rate     int
	openArgs int // Open's input args: 0 (v3+), 1 (v2: a mode)
}

func (p PCM) String() string {
	return fmt.Sprintf("%s %s, %d Hz, %d ch, format %#04x", p.Mode, p.Codec, p.Rate, p.Channels, p.Format)
}

// Report is a snapshot of the Bluetooth side.
type Report struct {
	Lines   []string // for people: the adapter, bluealsa, what went wrong
	Devices []Device
	PCMs    []PCM
	Bus     *dbus.Conn
}

// Sink is the PCM to play into: a playback PCM, of a connected device if
// there is one.
func (r Report) Sink() (PCM, bool) {
	var found []PCM
	for _, p := range r.PCMs {
		if p.Mode == "sink" && (p.Format == 0x8210 || p.Format == 0) && p.Codec != "CVSD" { // 16-bit; not a headset's phone line
			found = append(found, p)
		}
	}
	for _, p := range found {
		for _, d := range r.Devices {
			if d.Path == p.Device && d.Connected {
				return p, true
			}
		}
	}
	if len(found) > 0 {
		return found[0], true
	}
	return PCM{}, false
}

// DeviceOf is the device a PCM belongs to, if it's known.
func (r Report) DeviceOf(p PCM) (Device, bool) {
	for _, d := range r.Devices {
		if d.Path == p.Device {
			return d, true
		}
	}
	return Device{}, false
}

var introspected sync.Once // the interfaces go to the log once

// Probe finds the bus with BlueZ, the devices, and bluealsa's PCMs.
func Probe() Report {
	if r, ok := fakeReport(); ok {
		return r
	}
	var r Report
	add := func(format string, args ...any) { r.Lines = append(r.Lines, fmt.Sprintf(format, args...)) }

	first := false
	introspected.Do(func() { first = true })
	if first {
		for _, pid := range pidsOf("bluetoothd", "bluealsa") {
			cmd, _ := os.ReadFile(filepath.Join("/proc", pid, "cmdline"))
			log.Printf("bluetooth: process %s: %s", pid, strings.ReplaceAll(strings.TrimRight(string(cmd), "\x00"), "\x00", " "))
		}
	}
	bus, which, names, err := btBus()
	if err != nil {
		add("no D-Bus with BlueZ: %v", err)
		add("Turn Bluetooth on in Kobo, and connect the device there.")
		return r
	}
	r.Bus = bus
	if first {
		log.Printf("bluetooth: %s bus names: %v", which, names)
	}

	var objs map[dbus.ObjectPath]map[string]map[string]dbus.Variant
	if err := bus.Object("org.bluez", "/").Call("org.freedesktop.DBus.ObjectManager.GetManagedObjects", 0).Store(&objs); err != nil {
		add("BlueZ: %v", err)
		return r
	}
	paths := make([]string, 0, len(objs))
	for p := range objs {
		paths = append(paths, string(p))
	}
	sort.Strings(paths)
	for _, p := range paths {
		ifs := objs[dbus.ObjectPath(p)]
		if a, ok := ifs["org.bluez.Adapter1"]; ok {
			state := "off"
			if prop[bool](a, "Powered") {
				state = "on"
			}
			add("adapter %s (%s): %s", prop[string](a, "Alias"), prop[string](a, "Address"), state)
		}
		if d, ok := ifs["org.bluez.Device1"]; ok {
			dev := Device{Path: dbus.ObjectPath(p), Name: prop[string](d, "Alias"), Addr: prop[string](d, "Address"),
				Paired: prop[bool](d, "Paired"), Connected: prop[bool](d, "Connected")}
			for _, u := range prop[[]string](d, "UUIDs") {
				dev.IsAudio = dev.IsAudio || strings.EqualFold(u, uuidAudioSink)
			}
			if dev.Paired || dev.Connected {
				r.Devices = append(r.Devices, dev)
			}
		}
	}

	services := []string{}
	for _, n := range names {
		if strings.HasPrefix(n, "org.bluealsa") {
			services = append(services, n)
		}
	}
	if len(services) == 0 {
		add("bluealsa isn't on the %s bus: no audio", which)
	}
	for _, svc := range services {
		pcms, api, err := bluealsaPCMs(bus, svc, first)
		if err != nil {
			add("%s: %v", svc, err)
			continue
		}
		add("%s (%s API): %d PCMs", svc, api, len(pcms))
		r.PCMs = append(r.PCMs, pcms...)
	}
	return r
}

// btBus returns the first bus that has BlueZ on it, with its names.
func btBus() (*dbus.Conn, string, []string, error) {
	var errs []string
	for _, b := range []struct {
		name    string
		connect func(...dbus.ConnOption) (*dbus.Conn, error)
	}{{"system", dbus.ConnectSystemBus}, {"session", dbus.ConnectSessionBus}} {
		conn, err := b.connect()
		if err != nil {
			errs = append(errs, b.name+": "+err.Error())
			continue
		}
		var names []string
		if err := conn.BusObject().Call("org.freedesktop.DBus.ListNames", 0).Store(&names); err != nil {
			errs = append(errs, b.name+": "+err.Error())
			conn.Close()
			continue
		}
		for _, n := range names {
			if n == "org.bluez" {
				return conn, b.name, names, nil
			}
		}
		errs = append(errs, b.name+": no org.bluez")
		conn.Close()
	}
	return nil, "", nil, fmt.Errorf("%s", strings.Join(errs, "; "))
}

// bluealsaPCMs lists the PCMs, trying the v4 API, then v2/v3's.
func bluealsaPCMs(bus *dbus.Conn, svc string, verbose bool) ([]PCM, string, error) {
	var xml string
	if verbose && bus.Object(svc, bluealsaPath).Call("org.freedesktop.DBus.Introspectable.Introspect", 0).Store(&xml) == nil {
		log.Printf("bluetooth: %s %s introspection:\n%s", svc, bluealsaPath, xml[:min(len(xml), 6000)])
	}

	found := map[dbus.ObjectPath]map[string]dbus.Variant{}
	api := "v4"
	for _, root := range []dbus.ObjectPath{"/", bluealsaPath} {
		var objs map[dbus.ObjectPath]map[string]map[string]dbus.Variant
		if bus.Object(svc, root).Call("org.freedesktop.DBus.ObjectManager.GetManagedObjects", 0).Store(&objs) != nil {
			continue
		}
		for p, ifs := range objs {
			if props, ok := ifs["org.bluealsa.PCM1"]; ok {
				found[p] = props
			}
		}
	}
	if len(found) == 0 {
		api = "v2/v3"
		if err := bus.Object(svc, bluealsaPath).Call("org.bluealsa.Manager1.GetPCMs", 0).Store(&found); err != nil {
			return nil, "", fmt.Errorf("can't list PCMs: %v", err)
		}
	}

	var pcms []PCM
	for p, props := range found {
		if verbose {
			log.Printf("bluetooth: PCM %s: %v", p, props)
		}
		pcm := PCM{service: svc, Path: p, Device: prop[dbus.ObjectPath](props, "Device"),
			Mode: prop[string](props, "Mode"), Codec: prop[string](props, "Codec"),
			Format: prop[uint16](props, "Format"), Channels: int(prop[byte](props, "Channels")),
			Rate: int(prop[uint32](props, "Rate"))}
		if pcm.Rate == 0 {
			pcm.Rate = int(prop[uint32](props, "Sampling")) // v3's name
		}
		if pcm.Mode == "" {
			pcm.Mode = "sink" // v2 PCMs have no mode; A2DP from the Kobo is playback
		}
		pcm.openArgs = openArgCount(bus, svc, p, verbose)
		pcms = append(pcms, pcm)
	}
	sort.Slice(pcms, func(i, j int) bool { return pcms[i].Path < pcms[j].Path })
	return pcms, api, nil
}

// openArgCount reads how many input arguments PCM1.Open takes.
func openArgCount(bus *dbus.Conn, svc string, p dbus.ObjectPath, verbose bool) int {
	var xml string
	if bus.Object(svc, p).Call("org.freedesktop.DBus.Introspectable.Introspect", 0).Store(&xml) != nil {
		return 0
	}
	if verbose {
		log.Printf("bluetooth: %s introspection:\n%s", p, xml[:min(len(xml), 4000)])
	}
	_, open, ok := strings.Cut(xml, `<method name="Open">`)
	if !ok {
		return 0
	}
	open, _, _ = strings.Cut(open, "</method>")
	return strings.Count(open, `direction="in"`)
}

// openPCM asks bluealsa for the PCM's sample pipe and its control socket.
func openPCM(bus *dbus.Conn, pcm PCM) (data, ctrl *os.File, err error) {
	var args []any
	if pcm.openArgs > 0 {
		args = append(args, "sink")
	}
	var dataFD, ctrlFD dbus.UnixFD
	if err := bus.Object(pcm.service, pcm.Path).Call("org.bluealsa.PCM1.Open", 0, args...).Store(&dataFD, &ctrlFD); err != nil {
		if strings.Contains(err.Error(), "busy") { // bluealsa gives a PCM to one client at a time
			return nil, nil, errors.New("the headphones are in use by another program (the Alpine shell?)")
		}
		return nil, nil, fmt.Errorf("PCM1.Open: %w", err)
	}
	return pollable(int(dataFD), "bluealsa-pcm"), pollable(int(ctrlFD), "bluealsa-ctrl"), nil
}

// Connect asks BlueZ to connect a paired device.
func Connect(bus *dbus.Conn, d Device) error {
	return bus.Object("org.bluez", d.Path).Call("org.bluez.Device1.Connect", 0).Err
}

// prop reads a D-Bus property, or T's zero value.
func prop[T any](props map[string]dbus.Variant, name string) T {
	var zero T
	v, ok := props[name]
	if !ok {
		return zero
	}
	if t, ok := v.Value().(T); ok {
		return t
	}
	return zero
}

func pidsOf(names ...string) []string {
	var pids []string
	dirs, _ := os.ReadDir("/proc")
	for _, d := range dirs {
		comm, err := os.ReadFile(filepath.Join("/proc", d.Name(), "comm"))
		if err != nil {
			continue
		}
		for _, n := range names {
			if strings.TrimSpace(string(comm)) == n {
				pids = append(pids, d.Name())
			}
		}
	}
	return pids
}
