package book

import "testing"

func TestHyphenate(t *testing.T) {
	h := hyphenatorFor("en-US")
	for word, want := range map[string]string{
		"hyphenation":   "hy-phen-ation",
		"considering":   "con-sid-er-ing",
		"Rabbit":        "Rab-bit",
		"waistcoat":     "waist-coat",
		"table":         "ta-ble", // by hand
		"project":       "project",
		"Alice":         "Al-ice", // TeX's too
		"curiouser":     "cu-ri-ouser",
		"conversations": "con-ver-sa-tions",
		"telescope":     "tele-scope",
		"it's":          "it's",
		"mêlée":         "mêlée",
	} {
		if got := h.hyphenated(word); got != want {
			t.Errorf("%s: %s, want %s", word, got, want)
		}
	}
	if hyphenatorFor("ru") != nil || hyphenatorFor("") == nil || hyphenatorFor("en") == nil {
		t.Error("languages")
	}
}
