package main

import "testing"

func TestFuzzy(t *testing.T) {
	if fuzzy("mus", "Music tab") <= fuzzy("mus", "my unused shell") {
		t.Error("a word's start should beat letters strewn about")
	}
	if fuzzy("xyz", "Music") >= 0 {
		t.Error("no match should be -1")
	}
	if fuzzy("", "anything") != 0 {
		t.Error("an empty query matches all alike")
	}
	if fuzzy("ssh2", "SSH 2 bro@host") < 0 {
		t.Error("spaces in the text are skipped over")
	}
}
