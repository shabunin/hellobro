package main

import (
	"slices"
	"testing"
)

func TestLibView(t *testing.T) {
	es := []libEntry{
		{key: "/b/zed.epub", rel: "zed.epub", name: "Zed", at: 5},
		{key: "/b/Fiction/alice.epub", rel: "Fiction/alice.epub", name: "Alice", at: 9},
		{key: "/b/Fiction/SF/dune.epub", rel: "Fiction/SF/dune.epub", name: "Dune"},
		{key: "/b/art.pdf", rel: "art.pdf", name: "art"},
		{key: "/b/Poems/a.epub", rel: "Poems/a.epub", name: "Odes"},
	}
	show := func(items []libItem) []string {
		var s []string
		for _, it := range items {
			n := it.name
			if it.dir {
				n = "[" + n + " " + it.note + " → " + it.key + "]"
			}
			s = append(s, n)
		}
		return s
	}
	for _, c := range []struct {
		tab  int
		dir  string
		want []string
	}{
		{libRecent, "", []string{"Alice", "Zed", "art", "Dune", "Odes"}},
		{libAll, "", []string{"Alice", "art", "Dune", "Odes", "Zed"}},
		{libFolders, "", []string{"[Empty empty → Empty]", "[Fiction 2 → Fiction]", "[Poems 1 → Poems]", "art", "Zed"}},
		{libFolders, "Fiction", []string{"[‹ all folders Fiction → ]", "[SF 1 → Fiction/SF]", "Alice"}},
		{libFolders, "Fiction/SF", []string{"[‹ Fiction Fiction/SF → Fiction]", "[Old empty → Fiction/SF/Old]", "Dune"}},
		{libFolders, "Gone", []string{"[‹ all folders Gone → ]"}},
	} {
		if got := show(libView(es, []string{"Empty", "Fiction", "Fiction/SF/Old"}, c.tab, c.dir)); !slices.Equal(got, c.want) {
			t.Errorf("tab %d %q: %q, want %q", c.tab, c.dir, got, c.want)
		}
	}
}
