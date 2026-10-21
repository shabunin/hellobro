package main

// Bluetooth audio through the Kobo's own stack: BlueZ knows the devices,
// bluez-alsa (bluealsa) turns a connected A2DP sink into a PCM that we
// open over D-Bus and write samples into. Pairing stays Nickel's job.
//
// Kobo doesn't document which bluez-alsa it ships, so this probes: D-Bus
// names, the PCM listing of v4 (ObjectManager) and v2/v3 (Manager1), and
// Open with or without a mode. What it finds goes to the log.

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/godbus/dbus/v5"
)

const (
	uuidAudioSink = "0000110b-0000-1000-8000-00805f9b34fb"
	bluealsaPath  = "/org/bluealsa"
)

type btDevice struct {
	path                       dbus.ObjectPath
	name, addr                 string
	paired, connected, isAudio bool
}

type bluealsaPCM struct {
	service  string
	path     dbus.ObjectPath
	device   dbus.ObjectPath
	mode     string // "sink": we play into it
	codec    string
	format   uint16
	channels int
	rate     int
	openArgs int // Open's input args: 0 (v3+), 1 (v2: a mode)
}

func (p bluealsaPCM) String() string {
	return fmt.Sprintf("%s %s, %d Hz, %d ch, format %#04x", p.mode, p.codec, p.rate, p.channels, p.format)
}

// btReport is a snapshot of the Bluetooth side, for the Sound tab.
type btReport struct {
	lines   []string
	devices []btDevice
	pcms    []bluealsaPCM
	bus     *dbus.Conn
}

// probeBluetooth finds the bus with BlueZ, the devices, and bluealsa's PCMs.
func probeBluetooth() btReport {
	var r btReport
	add := func(format string, args ...any) { r.lines = append(r.lines, fmt.Sprintf(format, args...)) }

	for _, pid := range pidsOf("bluetoothd", "bluealsa") {
		cmd, _ := os.ReadFile(filepath.Join("/proc", pid, "cmdline"))
		log.Printf("bluetooth: process %s: %s", pid, strings.ReplaceAll(strings.TrimRight(string(cmd), "\x00"), "\x00", " "))
	}
	bus, which, names, err := btBus()
	if err != nil {
		add("no D-Bus with BlueZ: %v", err)
		add("Turn Bluetooth on in Kobo, and connect the device there.")
		return r
	}
	r.bus = bus
	log.Printf("bluetooth: %s bus names: %v", which, names)

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
			dev := btDevice{path: dbus.ObjectPath(p), name: prop[string](d, "Alias"), addr: prop[string](d, "Address"),
				paired: prop[bool](d, "Paired"), connected: prop[bool](d, "Connected")}
			for _, u := range prop[[]string](d, "UUIDs") {
				dev.isAudio = dev.isAudio || strings.EqualFold(u, uuidAudioSink)
			}
			if dev.paired || dev.connected {
				r.devices = append(r.devices, dev)
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
		pcms, api, err := bluealsaPCMs(bus, svc)
		if err != nil {
			add("%s: %v", svc, err)
			continue
		}
		add("%s (%s API): %d PCMs", svc, api, len(pcms))
		r.pcms = append(r.pcms, pcms...)
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
func bluealsaPCMs(bus *dbus.Conn, svc string) ([]bluealsaPCM, string, error) {
	var xml string
	if err := bus.Object(svc, bluealsaPath).Call("org.freedesktop.DBus.Introspectable.Introspect", 0).Store(&xml); err == nil {
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

	var pcms []bluealsaPCM
	for p, props := range found {
		log.Printf("bluetooth: PCM %s: %v", p, props)
		pcm := bluealsaPCM{service: svc, path: p, device: prop[dbus.ObjectPath](props, "Device"),
			mode: prop[string](props, "Mode"), codec: prop[string](props, "Codec"),
			format: prop[uint16](props, "Format"), channels: int(prop[byte](props, "Channels")),
			rate: int(prop[uint32](props, "Rate"))}
		if pcm.rate == 0 {
			pcm.rate = int(prop[uint32](props, "Sampling")) // v3's name
		}
		if pcm.mode == "" {
			pcm.mode = "sink" // v2 PCMs have no mode; A2DP from the Kobo is playback
		}
		pcm.openArgs = openArgCount(bus, svc, p)
		pcms = append(pcms, pcm)
	}
	sort.Slice(pcms, func(i, j int) bool { return pcms[i].path < pcms[j].path })
	return pcms, api, nil
}

// openArgCount reads how many input arguments PCM1.Open takes.
func openArgCount(bus *dbus.Conn, svc string, p dbus.ObjectPath) int {
	var xml string
	if bus.Object(svc, p).Call("org.freedesktop.DBus.Introspectable.Introspect", 0).Store(&xml) != nil {
		return 0
	}
	log.Printf("bluetooth: %s introspection:\n%s", p, xml[:min(len(xml), 4000)])
	_, open, ok := strings.Cut(xml, `<method name="Open">`)
	if !ok {
		return 0
	}
	open, _, _ = strings.Cut(open, "</method>")
	return strings.Count(open, `direction="in"`)
}

// openPCM asks bluealsa for the PCM's sample pipe and its control socket.
func openPCM(bus *dbus.Conn, pcm bluealsaPCM) (data, ctrl *os.File, err error) {
	var args []any
	if pcm.openArgs > 0 {
		args = append(args, "sink")
	}
	var dataFD, ctrlFD dbus.UnixFD
	if err := bus.Object(pcm.service, pcm.path).Call("org.bluealsa.PCM1.Open", 0, args...).Store(&dataFD, &ctrlFD); err != nil {
		return nil, nil, fmt.Errorf("PCM1.Open: %w", err)
	}
	return os.NewFile(uintptr(dataFD), "bluealsa-pcm"), os.NewFile(uintptr(ctrlFD), "bluealsa-ctrl"), nil
}

func connectDevice(bus *dbus.Conn, d btDevice) error {
	return bus.Object("org.bluez", d.path).Call("org.bluez.Device1.Connect", 0).Err
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
