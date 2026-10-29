package main

import (
	"image"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestMenuLayout(t *testing.T) {
	// 1000 wide, 290 at least, 10 apart: 3 to a row, 326 each.
	heads, tiles, h := menuLayout(1000, []int{4, 1}, 290, 100, 30, 10)
	wantHeads := []image.Rectangle{image.Rect(0, 0, 1000, 30), image.Rect(0, 260, 1000, 290)}
	if !slices.Equal(heads, wantHeads) {
		t.Errorf("heads %v, want %v", heads, wantHeads)
	}
	want := [][]image.Rectangle{
		{image.Rect(0, 30, 326, 130), image.Rect(336, 30, 662, 130), image.Rect(672, 30, 998, 130), image.Rect(0, 140, 326, 240)},
		{image.Rect(0, 290, 326, 390)},
	}
	for i := range want {
		if !slices.Equal(tiles[i], want[i]) {
			t.Errorf("group %d: %v, want %v", i, tiles[i], want[i])
		}
	}
	if h != 390 {
		t.Errorf("height %d, want 390", h)
	}
	// Narrower than a tile: one to a row, as wide as it is.
	if _, tiles, _ := menuLayout(200, []int{2}, 290, 100, 30, 10); tiles[0][1] != image.Rect(0, 140, 200, 240) {
		t.Errorf("narrow: %v", tiles[0])
	}
}

func TestMenuStep(t *testing.T) {
	// Two groups: three in a row, one under; then two.
	_, tiles, _ := menuLayout(1000, []int{4, 2}, 290, 100, 30, 10)
	rs := append(slices.Clone(tiles[0]), tiles[1]...)
	for _, c := range []struct{ sel, dx, dy, want int }{
		{-1, 0, 1, 0}, // nothing lit: the first
		{9, 0, 1, 0},  // a tile gone: the first
		{0, 1, 0, 1},  // →
		{2, 1, 0, 3},  // → off a row's end: the next row
		{3, -1, 0, 2}, // ← back
		{5, 1, 0, 5},  // the last stays
		{0, -1, 0, 0}, // the first too
		{2, 0, 1, 3},  // ↓ from the right: the row below has one, at the left
		{1, 0, 1, 3},  // ↓ from the middle: the nearest across
		{3, 0, 1, 4},  // ↓ into the next group
		{4, 0, 1, 4},  // ↓ from the bottom row: stays
		{5, 0, -1, 3}, // ↑ from the next group
		{3, 0, -1, 0}, // ↑ to the row above, the nearest across
		{1, 0, -1, 1}, // ↑ from the top: stays
	} {
		if got := menuStep(rs, c.sel, c.dx, c.dy); got != c.want {
			t.Errorf("from %d by %d,%d: %d, want %d", c.sel, c.dx, c.dy, got, c.want)
		}
	}
	if got := menuStep(nil, 0, 1, 0); got != -1 {
		t.Errorf("no tiles: %d, want -1", got)
	}
}

// TestLaunchFound: what's typed narrows the tiles, best first, and finds
// every notebook, not only the last ones; an @ is a host, not a search.
func TestLaunchFound(t *testing.T) {
	defer func(r string) { shareRoot = r }(shareRoot)
	shareRoot = t.TempDir()
	os.MkdirAll(filepath.Join(shareRoot, "Notebooks", "Work"), 0o755)
	for _, n := range []string{"Work/wifi plan", "shopping"} {
		os.WriteFile(filepath.Join(shareRoot, "Notebooks", n+noteExt), []byte("x"), 0o644)
	}
	a := &bro{}
	gs := []launchGroup{{"Settings", []launchTile{{name: "Wi-Fi", note: "networks"}, {name: "Bluetooth", note: "keyboards"}}},
		{"Read and write", []launchTile{{name: "shopping", note: "notebook", icon: "notebook"}}}}
	names := func(q string) []string {
		var ns []string
		for _, tl := range a.launchFound(&termSession{}, gs, q)[0].tiles {
			ns = append(ns, tl.name)
		}
		return ns
	}
	if got := names("wifi"); len(got) != 2 || !slices.Contains(got, "Wi-Fi") || got[0] != "wifi plan" { // a run of it first
		t.Errorf("wifi: %q", got)
	}
	if got := names("shop"); len(got) != 1 || got[0] != "shopping" {
		t.Errorf("shop, once: %q", got)
	}
	if got := names("zzz"); len(got) != 0 {
		t.Errorf("zzz: %q", got)
	}
	if q := launchQuery(&termSession{input: "bro@host"}); q != "" {
		t.Errorf("a host looked for: %q", q)
	}
}
