package main

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"hellobro/radio"
)

func TestParseFolds(t *testing.T) {
	for _, c := range []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"wifi\n", []string{"wifi"}},
		{" wifi \r\nusage\n\nnow\nsettings", []string{"settings", "usage", "wifi"}}, // now: the Music tab's, gone
	} {
		got := slices.Sorted(maps.Keys(parseFolds(c.in)))
		if !slices.Equal(got, c.want) {
			t.Errorf("parseFolds(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestNetText(t *testing.T) {
	for _, c := range []struct {
		ws   radio.WiFiStatus
		want string
	}{
		{radio.WiFiStatus{}, "Wi-Fi off"},
		{radio.WiFiStatus{On: true, State: "SCANNING"}, "no network"},
		{radio.WiFiStatus{On: true, State: "COMPLETED", SSID: "home"}, "home · no address"},
		{radio.WiFiStatus{On: true, State: "COMPLETED", SSID: "home", IP: "192.168.1.20"}, "home · 192.168.1.20"},
	} {
		if got := netText(c.ws); got != c.want {
			t.Errorf("netText(%+v) = %q, want %q", c.ws, got, c.want)
		}
	}
}

// Every panel that folds has commands to fold and open it.
func TestFoldCommands(t *testing.T) {
	for _, f := range foldables {
		for _, id := range []string{"fold." + f.key, "unfold." + f.key} {
			if !slices.ContainsFunc(commands, func(c *command) bool { return c.id == id }) {
				t.Errorf("no command %s", id)
			}
		}
	}
}

func TestWifiFacts(t *testing.T) {
	ws := radio.WiFiStatus{On: true, State: "COMPLETED", SSID: "home", BSSID: "aa:bb", Freq: 5220, Signal: -48,
		IP: "192.168.1.20", Gateway: "192.168.1.1", Interface: "wlan0", Saving: true}
	got := infoText(wifiFacts(ws, ifaceInfo{mac: "d4:35:00:00:00:01", cidr: "192.168.1.20/24"}, []string{"192.168.1.1"}))
	want := `Wi-Fi: on · connected
network: home
access point: aa:bb
band: 5 GHz · channel 44 (5220 MHz)
signal: -48 dBm · good
address: 192.168.1.20/24
gateway: 192.168.1.1
DNS: 192.168.1.1
interface: wlan0 · d4:35:00:00:00:01
new networks: saved (wpa_supplicant's file)
`
	if got != want {
		t.Errorf("wifiFacts:\n%s\nwant:\n%s", got, want)
	}
	if got := infoText(wifiFacts(radio.WiFiStatus{Interface: "wlan0"}, ifaceInfo{}, nil)); !strings.HasPrefix(got, "Wi-Fi: off\naddress: none\n") {
		t.Errorf("off: %q", got)
	}
	for mhz, want := range map[int]string{2412: "2.4 GHz · channel 1 (2412 MHz)", 2462: "2.4 GHz · channel 11 (2462 MHz)", 5745: "5 GHz · channel 149 (5745 MHz)", 0: "?"} {
		if got := channelOf(mhz); got != want {
			t.Errorf("channelOf(%d) = %q, want %q", mhz, got, want)
		}
	}
}
