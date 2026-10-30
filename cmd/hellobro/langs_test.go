package main

import (
	"slices"
	"testing"
)

func TestLangValues(t *testing.T) {
	vs, shown := langValues(false), langValues(true)
	if len(vs) != 31 || len(shown) != 31 || vs[0] != "en" || shown[0] != "English" {
		t.Fatalf("%d values, the first %q (%q)", len(vs), vs[0], shown[0])
	}
	for _, v := range []string{"en ru", "en uk", "en ru uk", "en de", "ru el", "en ru uk de el"} {
		if !slices.Contains(vs, v) {
			t.Errorf("no %q", v)
		}
	}
	if i := slices.Index(vs, "en ru uk"); shown[i] != "English, Russian, Ukrainian" {
		t.Errorf("en ru uk shown as %q", shown[i])
	}
}
