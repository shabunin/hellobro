package radio

import "testing"

func TestUnescape(t *testing.T) {
	for in, want := range map[string]string{
		`plain`:            "plain",
		`caf\xc3\xa9`:      "café",
		`a\\b`:             `a\b`,
		`q\"x`:             `q"x`,
		`trailing\x4`:      `trailing\x4`,
		`\x41\x42 and \x4`: `AB and \x4`,
	} {
		if got := unescape(in); got != want {
			t.Errorf("unescape(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSecurity(t *testing.T) {
	for flags, want := range map[string]string{
		"[WPA2-PSK-CCMP][ESS]":     "WPA2",
		"[WPA-PSK-TKIP][ESS]":      "WPA",
		"[RSN-SAE-CCMP][ESS]":      "WPA3",
		"[WPA2-PSK+SAE-CCMP][ESS]": "WPA2",
		"[WPA2-EAP-CCMP][ESS]":     "EAP",
		"[WEP][ESS]":               "WEP",
		"[ESS]":                    "",
	} {
		if got := security(flags); got != want {
			t.Errorf("security(%q) = %q, want %q", flags, got, want)
		}
	}
}

func TestMerge(t *testing.T) {
	heard := []Network{
		{SSID: "A", Signal: -70, ID: -1}, {SSID: "B", Signal: -50, ID: -1},
		{SSID: "A", Signal: -40, ID: -1}, {SSID: "", Signal: -30, ID: -1},
	}
	known := []Network{{SSID: "A", ID: 3}, {SSID: "Gone", ID: 4}}
	got := merge(heard, known, WiFiStatus{State: "COMPLETED", SSID: "B"})
	want := []string{"B", "A", "Gone"}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i, n := range got {
		if n.SSID != want[i] {
			t.Fatalf("order %v, want %v", got, want)
		}
	}
	if !got[0].Current || got[1].ID != 3 || got[1].Signal != -40 || !got[2].Hidden {
		t.Errorf("got %+v", got)
	}
}
