package main

import "testing"

// TestExFind: a name, a short one, one begun; a number is a page, /word
// a look; one begun as two are asks which.
func TestExFind(t *testing.T) {
	var ran, got string
	run := func(name string) func(string) error {
		return func(arg string) error { ran, got = name, arg; return nil }
	}
	cmds := []exCmd{{"page p", "N", run("page")}, {"paper", "x", run("paper")}, {"find / f", "word", run("find")},
		{"font", "name", run("font")}, {"export", "", run("export")}, {"quit q", "", run("quit")}}
	for _, c := range []struct{ line, ran, arg string }{
		{"12", "page", "12"}, {"40%", "page", "40%"}, {"p 3", "page", "3"}, {"/rabbit hole", "find", "rabbit hole"},
		{"f fox", "find", "fox"}, {"fo Go", "font", "Go"}, {"ex", "export", ""}, {"q", "quit", ""}, {"paper lines", "paper", "lines"},
	} {
		ran, got = "", ""
		c2, arg, err := exFind(cmds, c.line)
		if err == nil && c2 != nil {
			c2.run(arg)
		}
		if err != nil || ran != c.ran || got != c.arg {
			t.Errorf("%q: ran %q (%q), %v; want %q (%q)", c.line, ran, got, err, c.ran, c.arg)
		}
	}
	for _, line := range []string{"pa", "zz"} {
		if _, _, err := exFind(cmds, line); err == nil {
			t.Errorf("%q: no error", line)
		}
	}
	if h := exHint(cmds, "pa"); h != "page N · paper x" {
		t.Errorf("hint pa: %q", h)
	}
	if h := exHint(cmds, "paper l"); h != "paper x" {
		t.Errorf("hint paper: %q", h)
	}
	if h := exHint(cmds, "12"); h != "page N" {
		t.Errorf("hint 12: %q", h)
	}
	if h := exHint(cmds, "/rab"); h != "find word" {
		t.Errorf("hint /: %q", h)
	}
}
