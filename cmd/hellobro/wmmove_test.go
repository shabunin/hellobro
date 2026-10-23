package main

import "testing"

// A move where the window is (a step, or to a position) shows boxes till
// the moves settle; one to elsewhere (a workspace, the scratchpad) doesn't.
func TestWmMove(t *testing.T) {
	for line, want := range map[string]bool{
		"move left":                            true,
		"move down 50 px":                      true,
		"move position 10 20":                  true,
		"move position center":                 true,
		"move absolute position 0 0":           true,
		"move window left":                     true,
		"move container to workspace number 2": false,
		"move window to workspace next":        false,
		"move scratchpad":                      false,
		"move workspace to output left":        false,
		"focus left":                           false,
		"move":                                 false,
	} {
		if got := wmMove(line); got != want {
			t.Errorf("wmMove(%q) = %v, want %v", line, got, want)
		}
	}
}
