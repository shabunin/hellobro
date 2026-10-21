package radio

// Stand-ins for the harness (HELLOBRO_FAKE_RADIO=1): a few networks, one
// of them known and joined, a password that works ("bropassword"), and
// Bluetooth devices, one of which turns up while discovering and pairs
// with a passkey to confirm.

import (
	"errors"
	"log"
	"slices"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

type fakeWiFi struct {
	mu   sync.Mutex
	on   bool
	cur  string
	nets []Network
	next int
}

func newFakeWiFi() *fakeWiFi {
	return &fakeWiFi{on: true, cur: "HomeNet", next: 2, nets: []Network{
		{SSID: "HomeNet", BSSID: "aa:bb:cc:00:00:01", Signal: -48, Freq: 5180, Security: "WPA2", ID: 0},
		{SSID: "Cafe Free WiFi", BSSID: "aa:bb:cc:00:00:02", Signal: -71, Freq: 2437, Security: "", ID: -1},
		{SSID: "Neighbours 2.4", BSSID: "aa:bb:cc:00:00:03", Signal: -80, Freq: 2412, Security: "WPA2", ID: -1},
		{SSID: "Office", Signal: 0, ID: 1, Hidden: true},
		{SSID: "BroLab", BSSID: "aa:bb:cc:00:00:04", Signal: -60, Freq: 2462, Security: "WPA2", ID: -1},
	}}
}

func (f *fakeWiFi) Status() WiFiStatus {
	f.mu.Lock()
	defer f.mu.Unlock()
	st := WiFiStatus{On: f.on, Interface: "wlan0", Saving: true}
	if !f.on {
		return st
	}
	st.State = "DISCONNECTED"
	for _, n := range f.nets {
		if n.SSID == f.cur && f.cur != "" {
			st.State, st.SSID, st.BSSID, st.Freq, st.Signal = "COMPLETED", n.SSID, n.BSSID, n.Freq, n.Signal
			st.IP, st.Gateway = "192.168.2.31", "192.168.2.1"
		}
	}
	return st
}

func (f *fakeWiFi) Scan() ([]Network, error) {
	time.Sleep(1500 * time.Millisecond)
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.on {
		return nil, errors.New("Wi-Fi is off")
	}
	var heard, known []Network
	for _, n := range f.nets {
		if n.Signal != 0 {
			h := n
			h.ID, h.Current = -1, false
			heard = append(heard, h)
		}
		if n.ID >= 0 {
			known = append(known, Network{SSID: n.SSID, ID: n.ID, Current: n.SSID == f.cur})
		}
	}
	return merge(heard, known, WiFiStatus{}), nil
}

func (f *fakeWiFi) Connect(n Network, password string, progress func(string)) error {
	progress("joining " + n.SSID + "…")
	time.Sleep(1500 * time.Millisecond)
	f.mu.Lock()
	defer f.mu.Unlock()
	i := slices.IndexFunc(f.nets, func(m Network) bool { return m.SSID == n.SSID })
	if i < 0 {
		f.nets = append(f.nets, Network{SSID: n.SSID, ID: -1, Hidden: true, Security: n.Security})
		i = len(f.nets) - 1
	}
	if f.nets[i].ID < 0 {
		if f.nets[i].Security != "" && password != "bropassword" {
			return errors.New("couldn't join " + n.SSID + ": wrong password?")
		}
		f.nets[i].ID, f.next = f.next, f.next+1
	}
	if f.nets[i].Signal == 0 {
		return errors.New("couldn't join " + n.SSID + ": not around")
	}
	f.cur = n.SSID
	progress("connected to " + n.SSID + ", 192.168.2.31")
	return nil
}

func (f *fakeWiFi) Disconnect() error {
	f.mu.Lock()
	f.cur = ""
	f.mu.Unlock()
	return nil
}

func (f *fakeWiFi) Forget(n Network) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.nets {
		if f.nets[i].SSID == n.SSID {
			f.nets[i].ID = -1
			if f.cur == n.SSID {
				f.cur = ""
			}
		}
	}
	return nil
}

func (f *fakeWiFi) SetPower(on bool, progress func(string)) error {
	progress("turning Wi-Fi " + map[bool]string{true: "on", false: "off"}[on] + "…")
	time.Sleep(2 * time.Second)
	f.mu.Lock()
	f.on = on
	if on {
		f.cur = "HomeNet"
	}
	f.mu.Unlock()
	return nil
}

func (f *fakeWiFi) Restore() {}

type fakeBT struct {
	mu          sync.Mutex
	ui          AgentUI
	powered     bool
	discovering bool
	since       time.Time
	devs        []BTDevice
}

func newFakeBT(ui AgentUI) *fakeBT {
	return &fakeBT{ui: ui, powered: true, devs: []BTDevice{
		{Path: "/fake/kbd", Name: "NuPhy Air75 V2-3", Addr: "11:22:33:44:55:66", Icon: "input-keyboard", Paired: true, Trusted: true, Connected: true, RSSI: -50},
		{Path: "/fake/dac", Name: "FiiO BTR3K", Addr: "40:ED:98:1A:3E:E3", Icon: "audio-headphones", Paired: true, Trusted: true},
	}}
}

func (f *fakeBT) Status() (BTStatus, []BTDevice) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.discovering && time.Since(f.since) > 2*time.Second && !slices.ContainsFunc(f.devs, func(d BTDevice) bool { return d.Path == "/fake/spk" }) {
		f.devs = append(f.devs, BTDevice{Path: "/fake/spk", Name: "JBL Flip 5", Addr: "AA:00:11:22:33:44", Icon: "audio-card", RSSI: -62},
			BTDevice{Path: "/fake/anon", Addr: "5C:01:02:03:04:05", RSSI: -88})
	}
	devs := slices.Clone(f.devs)
	sortDevices(devs)
	return BTStatus{Running: true, Powered: f.powered, Discovering: f.discovering, Name: "Kobo", Addr: "00:1A:7D:DA:71:13"}, devs
}

func (f *fakeBT) SetPower(on bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.powered = on
	if !on {
		f.discovering = false
		for i := range f.devs {
			f.devs[i].Connected = false
		}
	}
	return nil
}

func (f *fakeBT) Discover(on bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.powered {
		return errors.New("Bluetooth is off")
	}
	f.discovering, f.since = on, time.Now()
	return nil
}

func (f *fakeBT) set(p dbus.ObjectPath, do func(d *BTDevice)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.devs {
		if f.devs[i].Path == p {
			do(&f.devs[i])
		}
	}
}

func (f *fakeBT) Pair(d BTDevice) error {
	time.Sleep(time.Second)
	if _, ok := f.ui.Ask(Ask{Kind: "confirm", Device: d.Name, Passkey: "123456"}); !ok {
		return errors.New("pairing failed: the codes didn't match, or it was turned down")
	}
	log.Printf("fake bluetooth: paired %s", d.Name)
	f.set(d.Path, func(d *BTDevice) { d.Paired, d.Trusted = true, true })
	return f.Connect(d)
}

func (f *fakeBT) Connect(d BTDevice) error {
	time.Sleep(time.Second)
	f.set(d.Path, func(d *BTDevice) { d.Connected = true })
	return nil
}

func (f *fakeBT) Disconnect(d BTDevice) error {
	f.set(d.Path, func(d *BTDevice) { d.Connected = false })
	return nil
}

func (f *fakeBT) Forget(d BTDevice) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.devs = slices.DeleteFunc(f.devs, func(x BTDevice) bool { return x.Path == d.Path })
	return nil
}

func (f *fakeBT) Restore() {}
