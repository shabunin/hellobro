package radio

// Bluetooth through BlueZ on D-Bus: the adapter's power and discovery,
// the devices, pairing (with an agent of ours, since Nickel's is frozen),
// connecting, forgetting.

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
)

// BTDevice is a device BlueZ knows: paired, or heard while discovering.
type BTDevice struct {
	Path                       dbus.ObjectPath
	Name, Addr, Icon           string
	RSSI                       int16 // 0 if not heard lately
	Paired, Trusted, Connected bool
}

// Kind is what the device is, in a word.
func (d BTDevice) Kind() string {
	switch {
	case strings.HasPrefix(d.Icon, "input-keyboard"):
		return "keyboard"
	case strings.HasPrefix(d.Icon, "input-mouse"), strings.HasPrefix(d.Icon, "input-tablet"):
		return "mouse"
	case strings.HasPrefix(d.Icon, "input-gaming"):
		return "gamepad"
	case strings.HasPrefix(d.Icon, "audio"):
		return "audio"
	case strings.HasPrefix(d.Icon, "phone"):
		return "phone"
	case strings.HasPrefix(d.Icon, "computer"):
		return "computer"
	}
	return ""
}

// BTStatus is the adapter.
type BTStatus struct {
	Running     bool // bluetoothd is there
	Powered     bool
	Discovering bool
	Name, Addr  string
	Err         string
}

// Ask is what pairing needs from the user. Kind:
//   - "confirm": does Passkey show on the device too? ("" or an error)
//   - "display": type Passkey (or PIN) on the device; Entered counts the keys
//   - "pin", "passkey": type what the device shows, answered as text
//   - "authorize": let the device in?
type Ask struct {
	Kind    string
	Device  string // its name
	Passkey string
	Entered int
}

// AgentUI shows an Ask and waits for the answer: the text typed, or ok
// for yes/no; done is false when it's been canceled. Show-only asks
// (display) return at once; Cancel takes whatever's up down.
type AgentUI interface {
	Ask(q Ask) (answer string, ok bool)
	Cancel()
}

// Bluetooth is what the app asks of Bluetooth. Calls block.
type Bluetooth interface {
	Status() (BTStatus, []BTDevice)
	SetPower(on bool) error
	Discover(on bool) error
	Pair(d BTDevice) error // then trusts and connects it
	Connect(d BTDevice) error
	Disconnect(d BTDevice) error
	Forget(d BTDevice) error
	// Restore puts the power back as it was, and drops the agent.
	Restore()
	// Keep makes the power as it is now the one Restore puts back.
	Keep()
}

// NewBluetooth is BlueZ, or the stand-in.
func NewBluetooth(ui AgentUI) Bluetooth {
	if os.Getenv("HELLOBRO_FAKE_RADIO") != "" {
		return newFakeBT(ui)
	}
	return &bluez{ui: ui}
}

type bluez struct {
	mu      sync.Mutex
	bus     *dbus.Conn
	adapter dbus.ObjectPath
	agent   bool
	ui      AgentUI
	initial *bool // the power before we first changed it
}

const agentPath = dbus.ObjectPath("/hellobro/agent")

// conn is the system bus with BlueZ on it, connected on first use.
func (b *bluez) conn() (*dbus.Conn, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.bus != nil && b.bus.Connected() {
		return b.bus, nil
	}
	bus, err := dbus.ConnectSystemBus()
	if err != nil {
		return nil, err
	}
	var names []string
	if err := bus.BusObject().Call("org.freedesktop.DBus.ListNames", 0).Store(&names); err != nil {
		bus.Close()
		return nil, err
	}
	if !slices.Contains(names, "org.bluez") {
		bus.Close()
		return nil, errors.New("bluetoothd isn't running: Bluetooth is off in Kobo's settings")
	}
	b.bus = bus
	return bus, nil
}

func (b *bluez) objects() (map[dbus.ObjectPath]map[string]map[string]dbus.Variant, error) {
	bus, err := b.conn()
	if err != nil {
		return nil, err
	}
	var objs map[dbus.ObjectPath]map[string]map[string]dbus.Variant
	err = bus.Object("org.bluez", "/").Call("org.freedesktop.DBus.ObjectManager.GetManagedObjects", 0).Store(&objs)
	return objs, err
}

func (b *bluez) Status() (BTStatus, []BTDevice) {
	var st BTStatus
	objs, err := b.objects()
	if err != nil {
		st.Err = err.Error()
		return st, nil
	}
	st.Running = true
	var devs []BTDevice
	paths := make([]string, 0, len(objs))
	for p := range objs {
		paths = append(paths, string(p))
	}
	slices.Sort(paths)
	b.adapter = "" // found again each time: it goes when the chip goes off
	for _, p := range paths {
		ifs := objs[dbus.ObjectPath(p)]
		if a, ok := ifs["org.bluez.Adapter1"]; ok && b.adapter == "" {
			b.adapter = dbus.ObjectPath(p)
			st.Powered, st.Discovering = prop[bool](a, "Powered"), prop[bool](a, "Discovering")
			st.Name, st.Addr = prop[string](a, "Alias"), prop[string](a, "Address")
		}
		if d, ok := ifs["org.bluez.Device1"]; ok {
			dev := BTDevice{Path: dbus.ObjectPath(p), Name: prop[string](d, "Name"), Addr: prop[string](d, "Address"),
				Icon: prop[string](d, "Icon"), RSSI: prop[int16](d, "RSSI"),
				Paired: prop[bool](d, "Paired"), Trusted: prop[bool](d, "Trusted"), Connected: prop[bool](d, "Connected")}
			if a := prop[string](d, "Alias"); dev.Name == "" && a != "" && a != strings.ReplaceAll(dev.Addr, ":", "-") {
				dev.Name = a
			}
			devs = append(devs, dev)
		}
	}
	if b.adapter == "" { // the chip is off: Kobo leaves bluetoothd running without it
		st.Powered, st.Err = false, ""
	}
	sortDevices(devs)
	return st, devs
}

// sortDevices: connected, then paired, then by signal; nameless last.
func sortDevices(devs []BTDevice) {
	rank := func(d BTDevice) int {
		switch {
		case d.Connected:
			return 0
		case d.Paired:
			return 1
		case d.Name != "":
			return 2
		}
		return 3
	}
	slices.SortStableFunc(devs, func(a, b BTDevice) int {
		if ra, rb := rank(a), rank(b); ra != rb {
			return ra - rb
		}
		ar, br := int(a.RSSI), int(b.RSSI)
		if ar == 0 {
			ar = -1000
		}
		if br == 0 {
			br = -1000
		}
		return br - ar
	})
}

func (b *bluez) setAdapter(name string, v any) error {
	bus, err := b.conn()
	if err != nil {
		return err
	}
	if b.adapter == "" {
		b.Status()
	}
	return bus.Object("org.bluez", b.adapter).Call("org.freedesktop.DBus.Properties.Set", 0, "org.bluez.Adapter1", name, dbus.MakeVariant(v)).Err
}

func (b *bluez) SetPower(on bool) error {
	st, _ := b.Status()
	if b.initial == nil {
		was := st.Powered
		b.initial = &was
	}
	log.Printf("bluetooth: power %v", on)
	if !on {
		if b.adapter != "" {
			b.setAdapter("Powered", false)
		}
		b.adapter = ""
		return chipOff()
	}
	if b.adapter == "" {
		if err := chipOn(); err != nil {
			return err
		}
		if len(pidsOf("bluetoothd")) == 0 {
			start("/libexec/bluetooth/bluetoothd", "-n")
		}
		end := time.Now().Add(10 * time.Second) // bluetoothd finds the new adapter
		for b.adapter == "" && time.Now().Before(end) {
			time.Sleep(500 * time.Millisecond)
			b.Status()
		}
		if b.adapter == "" {
			return errors.New("the chip is on, but bluetoothd doesn't see it")
		}
		if len(pidsOf("bluealsa")) == 0 {
			start("/bin/bluealsa", "-S", "-i", "hci0")
		}
	}
	return b.setAdapter("Powered", true)
}

// The Elipsa's and Sage's Realtek chip hangs off a serial line: rfkill
// powers it, rtk_hciattach loads its firmware and makes it hci0, as
// Kobo does it (and bluetooth.koplugin's on.sh, which found out how).
const (
	rtkAttach = "/sbin/rtk_hciattach"
	hci0      = "/sys/class/bluetooth/hci0"
)

// btRfkill is the Bluetooth chip's power switch.
func btRfkill() string {
	rf, _ := filepath.Glob("/sys/class/rfkill/rfkill*")
	for _, r := range rf {
		if t, _ := os.ReadFile(r + "/type"); strings.TrimSpace(string(t)) == "bluetooth" {
			return r + "/state"
		}
	}
	return ""
}

func chipOn() error {
	rf := btRfkill()
	if rf == "" {
		return errors.New("no Bluetooth power switch (rfkill)")
	}
	if exists(hci0) && len(pidsOf("rtk_hciattach")) > 0 {
		return nil // it's there; only bluetoothd hadn't caught up
	}
	was, _ := os.ReadFile(rf)
	stopAttach()
	// A chip that was running needs a few seconds off to reset; from off,
	// a second does.
	os.WriteFile(rf, []byte("0"), 0)
	if strings.TrimSpace(string(was)) == "1" {
		time.Sleep(4 * time.Second)
	} else {
		time.Sleep(time.Second)
	}
	if err := os.WriteFile(rf, []byte("1"), 0); err != nil {
		return fmt.Errorf("powering the chip: %v", err)
	}
	log.Print("bluetooth: chip powered, attaching it")
	start(rtkAttach, "-n", "-s", "115200", "/dev/ttyS1", "rtk_h5")
	end := time.Now().Add(15 * time.Second) // it syncs, loads firmware, speeds up
	for !exists(hci0) {
		if time.Now().After(end) {
			return errors.New("the chip didn't come up (rtk_hciattach)")
		}
		time.Sleep(500 * time.Millisecond)
	}
	log.Print("bluetooth: hci0 is there")
	return nil
}

func chipOff() error {
	stopAttach()
	if rf := btRfkill(); rf != "" {
		return os.WriteFile(rf, []byte("0"), 0)
	}
	return nil
}

// stopAttach ends rtk_hciattach and waits for it to be gone: on its way
// out it puts the serial line back, which would undo a new one's attach.
func stopAttach() {
	killAll(syscall.SIGTERM, "rtk_hciattach")
	for i := 0; i < 20 && len(pidsOf("rtk_hciattach")) > 0; i++ {
		time.Sleep(250 * time.Millisecond)
	}
}

func (b *bluez) Discover(on bool) error {
	bus, err := b.conn()
	if err != nil {
		return err
	}
	if b.adapter == "" {
		b.Status()
	}
	if b.adapter == "" {
		return errors.New("Bluetooth is off")
	}
	if on {
		b.ensureAgent()
		return bus.Object("org.bluez", b.adapter).Call("org.bluez.Adapter1.StartDiscovery", 0).Err
	}
	return bus.Object("org.bluez", b.adapter).Call("org.bluez.Adapter1.StopDiscovery", 0).Err
}

func (b *bluez) device(d BTDevice, method string, timeout time.Duration) error {
	bus, err := b.conn()
	if err != nil {
		return err
	}
	call := bus.Object("org.bluez", d.Path).Go("org.bluez.Device1."+method, 0, make(chan *dbus.Call, 1))
	select {
	case c := <-call.Done:
		return btError(c.Err)
	case <-time.After(timeout):
		return fmt.Errorf("%s took too long", strings.ToLower(method))
	}
}

func (b *bluez) Pair(d BTDevice) error {
	b.ensureAgent()
	log.Printf("bluetooth: pairing %s %s", d.Name, d.Addr)
	if err := b.device(d, "Pair", 60*time.Second); err != nil {
		b.ui.Cancel()
		return err
	}
	b.ui.Cancel() // a passkey shown is done with
	bus, _ := b.conn()
	if err := bus.Object("org.bluez", d.Path).Call("org.freedesktop.DBus.Properties.Set", 0, "org.bluez.Device1", "Trusted", dbus.MakeVariant(true)).Err; err != nil {
		log.Printf("bluetooth: trust %s: %v", d.Addr, err)
	}
	return b.Connect(d)
}

func (b *bluez) Connect(d BTDevice) error {
	log.Printf("bluetooth: connecting %s %s", d.Name, d.Addr)
	return b.device(d, "Connect", 30*time.Second)
}

func (b *bluez) Disconnect(d BTDevice) error {
	log.Printf("bluetooth: disconnecting %s %s", d.Name, d.Addr)
	return b.device(d, "Disconnect", 15*time.Second)
}

func (b *bluez) Forget(d BTDevice) error {
	bus, err := b.conn()
	if err != nil {
		return err
	}
	log.Printf("bluetooth: removing %s %s", d.Name, d.Addr)
	return btError(bus.Object("org.bluez", b.adapter).Call("org.bluez.Adapter1.RemoveDevice", 0, d.Path).Err)
}

func (b *bluez) Keep() {
	st, _ := b.Status()
	b.initial = &st.Powered
}

func (b *bluez) Restore() {
	if b.bus == nil {
		return
	}
	bus := b.bus
	if b.agent {
		bus.Object("org.bluez", "/org/bluez").Call("org.bluez.AgentManager1.UnregisterAgent", 0, agentPath)
		b.agent = false
	}
	if b.adapter != "" {
		bus.Object("org.bluez", b.adapter).Call("org.bluez.Adapter1.StopDiscovery", 0)
	}
	if b.initial != nil {
		st, _ := b.Status()
		if st.Powered != *b.initial {
			log.Printf("bluetooth: putting the power back as Kobo had it: %v", *b.initial)
			if err := b.SetPower(*b.initial); err != nil {
				log.Printf("bluetooth: %v", err)
			}
		}
	}
	bus.Close()
	b.bus = nil
}

// btError makes BlueZ's errors readable.
func btError(err error) error {
	var de dbus.Error
	if !errors.As(err, &de) {
		if e, ok := err.(*dbus.Error); ok {
			de = *e
		} else {
			return err
		}
	}
	msg := strings.TrimPrefix(de.Name, "org.bluez.Error.")
	if len(de.Body) > 0 {
		if s, ok := de.Body[0].(string); ok && s != "" {
			msg += ": " + s
		}
	}
	switch {
	case strings.HasPrefix(msg, "AuthenticationFailed"), strings.HasPrefix(msg, "AuthenticationRejected"):
		return errors.New("pairing failed: the codes didn't match, or it was turned down")
	case strings.HasPrefix(msg, "AuthenticationCanceled"):
		return errors.New("pairing canceled")
	case strings.HasPrefix(msg, "AuthenticationTimeout"), strings.HasPrefix(msg, "ConnectionAttemptFailed"):
		return errors.New("no answer: is it on, and in pairing mode?")
	case strings.HasPrefix(msg, "AlreadyExists"):
		return errors.New("already paired")
	case strings.HasPrefix(msg, "InProgress"):
		return errors.New("busy with it already")
	}
	return errors.New(msg)
}

// --- the agent ---

// ensureAgent registers ours as the default, so pairing asks us: Nickel's
// can't answer while it's frozen. Unregistering gives BlueZ its own back.
func (b *bluez) ensureAgent() {
	if b.agent {
		return
	}
	bus, err := b.conn()
	if err != nil {
		return
	}
	if err := bus.Export(&agent{b: b}, agentPath, "org.bluez.Agent1"); err != nil {
		log.Printf("bluetooth: agent: %v", err)
		return
	}
	mgr := bus.Object("org.bluez", "/org/bluez")
	if err := mgr.Call("org.bluez.AgentManager1.RegisterAgent", 0, agentPath, "KeyboardDisplay").Err; err != nil {
		log.Printf("bluetooth: RegisterAgent: %v", err)
		return
	}
	if err := mgr.Call("org.bluez.AgentManager1.RequestDefaultAgent", 0, agentPath).Err; err != nil {
		log.Printf("bluetooth: RequestDefaultAgent: %v", err)
	}
	b.agent = true
	log.Print("bluetooth: our agent is the default")
}

type agent struct{ b *bluez }

var (
	errRejected = dbus.NewError("org.bluez.Error.Rejected", nil)
	errCanceled = dbus.NewError("org.bluez.Error.Canceled", nil)
)

func (g *agent) name(p dbus.ObjectPath) string {
	bus, err := g.b.conn()
	if err != nil {
		return string(p)
	}
	v, err := bus.Object("org.bluez", p).GetProperty("org.bluez.Device1.Alias")
	if s, ok := v.Value().(string); err == nil && ok {
		return s
	}
	return string(p)
}

func (g *agent) Release() *dbus.Error { log.Print("bluetooth: agent released"); return nil }

func (g *agent) RequestPinCode(dev dbus.ObjectPath) (string, *dbus.Error) {
	log.Printf("bluetooth: agent: PIN for %s", dev)
	pin, ok := g.b.ui.Ask(Ask{Kind: "pin", Device: g.name(dev)})
	if !ok {
		return "", errCanceled
	}
	return pin, nil
}

func (g *agent) DisplayPinCode(dev dbus.ObjectPath, pin string) *dbus.Error {
	log.Printf("bluetooth: agent: show PIN for %s", dev)
	g.b.ui.Ask(Ask{Kind: "display", Device: g.name(dev), Passkey: pin})
	return nil
}

func (g *agent) RequestPasskey(dev dbus.ObjectPath) (uint32, *dbus.Error) {
	log.Printf("bluetooth: agent: passkey for %s", dev)
	s, ok := g.b.ui.Ask(Ask{Kind: "passkey", Device: g.name(dev)})
	if !ok {
		return 0, errCanceled
	}
	var n uint32
	if _, err := fmt.Sscan(s, &n); err != nil {
		return 0, errRejected
	}
	return n, nil
}

func (g *agent) DisplayPasskey(dev dbus.ObjectPath, passkey uint32, entered uint16) *dbus.Error {
	g.b.ui.Ask(Ask{Kind: "display", Device: g.name(dev), Passkey: fmt.Sprintf("%06d", passkey), Entered: int(entered)})
	return nil
}

func (g *agent) RequestConfirmation(dev dbus.ObjectPath, passkey uint32) *dbus.Error {
	log.Printf("bluetooth: agent: confirm %06d for %s", passkey, dev)
	if _, ok := g.b.ui.Ask(Ask{Kind: "confirm", Device: g.name(dev), Passkey: fmt.Sprintf("%06d", passkey)}); !ok {
		return errRejected
	}
	return nil
}

func (g *agent) RequestAuthorization(dev dbus.ObjectPath) *dbus.Error {
	if _, ok := g.b.ui.Ask(Ask{Kind: "authorize", Device: g.name(dev)}); !ok {
		return errRejected
	}
	return nil
}

func (g *agent) AuthorizeService(dev dbus.ObjectPath, uuid string) *dbus.Error {
	return nil // a paired device's services: yes
}

func (g *agent) Cancel() *dbus.Error {
	log.Print("bluetooth: agent: canceled")
	g.b.ui.Cancel()
	return nil
}

// prop reads a D-Bus property, or T's zero value.
func prop[T any](props map[string]dbus.Variant, name string) T {
	var zero T
	if v, ok := props[name]; ok {
		if t, ok := v.Value().(T); ok {
			return t
		}
	}
	return zero
}
