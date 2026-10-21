package main

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

// Every page fills the window exactly: the renderer doesn't clear rows a
// taller frame left behind, and a wider one wraps.
func TestLayoutFillsWindow(t *testing.T) {
	for _, sz := range [][2]int{{84, 48}, {120, 30}, {60, 20}} {
		for p := range pages {
			m := newModel("bro@x", "xterm")
			m.w, m.h, m.page = sz[0], sz[1], p
			lines := strings.Split(m.render(), "\n")
			if len(lines) != m.h {
				t.Errorf("%dx%d %s: %d lines", m.w, m.h, pages[p], len(lines))
			}
			for i, l := range lines {
				if w := lipgloss.Width(l); w > m.w {
					t.Errorf("%dx%d %s: line %d is %d wide", m.w, m.h, pages[p], i, w)
				}
			}
		}
	}
}
