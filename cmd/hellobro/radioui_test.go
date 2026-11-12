package main

import (
	"image"
	"strings"
	"testing"

	"hellobro/ink/inktest"
	"hellobro/kobo"
	"hellobro/radio"
)

// Wi-Fi: the networks as rows, one tapped opens what can be done with
// it; a question takes the keys, a password shows as dots.
func TestWiFiUI(t *testing.T) {
	a := &bro{App: &kobo.App{}}
	r := &a.rs
	r.ws = radio.WiFiStatus{On: true, State: "COMPLETED", SSID: "HomeNet", Signal: -48, Freq: 2437, IP: "192.168.2.25"}
	r.nets = []radio.Network{
		{SSID: "HomeNet", Signal: -48, Freq: 2437, Security: "WPA2", ID: 0, Current: true},
		{SSID: "Cafe", Signal: -70, Freq: 5180, Security: "", ID: -1},
		{SSID: "Office", Signal: 0, Security: "WPA2", ID: 3},
	}
	u := inkUIs["wifi"](a)
	it := inktest.New(image.Pt(900, 1100), inktest.ElipsaMetric, u.Layout)
	inkSave(t, it, "wifi")
	if u.Status() != "HomeNet" {
		t.Errorf("status %q", u.Status())
	}
	for _, s := range []string{"HomeNet · -48 dBm · 2.4 GHz · 192.168.2.25", "Cafe", "open · 5 GHz", "saved, not around"} {
		if _, err := it.Find(s); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := it.Find("disconnect"); err == nil {
		t.Error("a row's actions show before it's tapped")
	}
	must(t, it.TapText("HomeNet"))
	inkSave(t, it, "wifi-open")
	for _, s := range []string{"disconnect", "forget"} {
		if _, err := it.Find(s); err != nil {
			t.Error(err)
		}
	}
	// A question: the pairing's, answered on a channel.
	reply := make(chan radioReply, 1)
	r.prompt = &radioPrompt{panel: panelWiFi, kind: "password", text: "Password for Cafe (WPA2):", reply: reply}
	it.W.Invalidate()
	it.Frame()
	it.Type("secret")
	inkSave(t, it, "wifi-ask")
	if _, err := it.Find("••••••"); err != nil {
		t.Error(err)
	}
	if !it.Key("Enter", 0) {
		t.Error("Enter not taken")
	}
	select {
	case rp := <-reply:
		if !rp.ok || rp.text != "secret" {
			t.Errorf("answered %+v", rp)
		}
	default:
		t.Fatal("not answered")
	}
	if r.prompt != nil {
		t.Error("the question stays")
	}
}

func TestBluetoothUI(t *testing.T) {
	a := &bro{App: &kobo.App{}}
	r := &a.rs
	r.bs = radio.BTStatus{Running: true, Powered: true, Name: "Kobo", Addr: "AA:BB"}
	r.devs = []radio.BTDevice{{Name: "NuPhy Air75", Addr: "11:22", Paired: true, Connected: true}, {Addr: "33:44"}}
	u := inkUIs["bluetooth"](a)
	it := inktest.New(image.Pt(900, 900), inktest.ElipsaMetric, u.Layout)
	inkSave(t, it, "bluetooth")
	if u.Status() != "1 connected" {
		t.Errorf("status %q", u.Status())
	}
	must(t, it.TapText("33:44"))
	if _, err := it.Find("pair"); err != nil {
		t.Error(err)
	}
	if ts := strings.Join(it.W.Texts(), "|"); strings.Contains(ts, "another network") {
		t.Error("Bluetooth has Wi-Fi's buttons")
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
