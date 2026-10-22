package main

// The header says where we are on the network: the Wi-Fi's name and our
// address, looked at now and then.

import (
	"time"

	"hellobro/radio"
)

const netPoll = 15 * time.Second

type netState struct {
	wifi    radio.WiFi // ours, till the Settings tab starts the radios
	ws      radio.WiFiStatus
	at      time.Time
	polling bool
}

// netTick looks at the Wi-Fi now and then, off the UI.
func (a *bro) netTick() {
	n := &a.net
	if n.polling || time.Since(n.at) < netPoll {
		return
	}
	w := a.rs.wifi
	if !a.rs.started {
		if n.wifi == nil {
			n.wifi = radio.NewWiFi()
		}
		w = n.wifi
	}
	n.polling, n.at = true, time.Now()
	go func() {
		ws := w.Status()
		a.Do(func() {
			n.polling, n.at, n.ws = false, time.Now(), ws
			a.SetNetwork(netText(ws))
		})
	}()
}

// netText is the network for the header: "home-wifi · 192.168.1.20".
func netText(ws radio.WiFiStatus) string {
	switch {
	case !ws.On:
		return "Wi-Fi off"
	case ws.State != "COMPLETED" || ws.SSID == "":
		return "no network"
	case ws.IP == "":
		return ws.SSID + " · no address"
	}
	return ws.SSID + " · " + ws.IP
}
