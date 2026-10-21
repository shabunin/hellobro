package main

import (
	"maps"
	"slices"
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
		{" wifi \r\nnow\n\nbogus\nsettings", []string{"now", "settings", "wifi"}},
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
