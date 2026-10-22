package wm

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

type host struct{ did []string }

func (h *host) Kill(n *Node)          { h.did = append(h.did, "kill "+n.Window.Title()) }
func (h *host) Exec(cmd string) error { h.did = append(h.did, "exec "+cmd); return nil }
func (h *host) Mode(m string) error   { h.did = append(h.did, "mode "+m); return nil }
func (h *host) Command(n *Node, name string, args []string) (bool, error) {
	if name != "hellobro" {
		return false, nil
	}
	h.did = append(h.did, strings.Join(append([]string{name, Name(n)}, args...), " "))
	if len(args) > 0 && args[0] == "fail" {
		return true, errors.New("failed")
	}
	return true, nil
}

func TestRun(t *testing.T) {
	for _, c := range []struct {
		in, cmd string
		want    string
		did     []string
	}{
		{"1:splith[a*, b]", "split v; exec foot", "<1:splith[splitv[a*], b]>", []string{"exec foot"}},
		{"1:splith[a*, b]", "layout tabbed", "<1:splith[tabbed[a*, b]]>", nil},
		{"1:splith[a, splitv[b*, c]]", "layout stacking", "<1:splith[a, stacked[b*, c]]>", nil},
		{"1:splith[a, splitv[b*, c]]", "layout toggle splith tabbed", "<1:splith[a, splith[b*, c]]>", nil},
		{"1:splith[a, splith[b*, c]]", "layout toggle splith tabbed", "<1:splith[a, tabbed[b*, c]]>", nil},
		{"1:splith[a*, b]", "focus right", "<1:splith[a, b*]>", nil},
		{"1:splith[a*, b]", "move right", "<1:splith[b, a*]>", nil},
		{"1:splith[a*, b]", `[title="^b$"] focus`, "<1:splith[a, b*]>", nil},
		{"1:splith[a*, b]", "[class=test] kill", "<1:splith[a*, b]>", []string{"kill a", "kill b"}},
		{"1:splith[a*, b]", "[title=b] fullscreen, focus", "<1:splith[a, b*!]>", nil},
		{"1:splith[a*, b]", "[title=b] mark x; [con_mark=x] floating enable", "<1:splith[a*] float[b]>", nil},
		{"1:splith[a*, b]", "workspace 2; workspace back_and_forth", "<1:splith[a*, b]>", nil},
		{"1:splith[a*, b]", "workspace my mail", "1:splith[a, b] <my mail:splith[]>", nil},
		{"1:splith[a*, b]", `workspace "my mail"`, "1:splith[a, b] <my mail:splith[]>", nil},
		{"1:splith[a*, b] | 3:mail:splith[c]", "workspace number 3", "1:splith[a, b] <3:mail:splith[c*]>", nil},
		{"1:splith[a*, b]", "move container to workspace number 2", "<1:splith[b*]> 2:splith[a]", nil},
		{"1:splith[a*, b]", "move to workspace 2, workspace 2", "1:splith[b] <2:splith[a*]>", nil},
		{"1:splith[a*, b]", "move scratchpad; scratchpad show", "<1:splith[b] float[a*]>", nil},
		{"1:splith[a*, b]", "rename workspace to one", "<one:splith[a*, b]>", nil},
		{"1:splith[a*, b] | 2:splith[c]", "rename workspace 2 to 0", "0:splith[c] <1:splith[a*, b]>", nil},
		{"1:splith[a*, b]", "mode resize", "<1:splith[a*, b]>", []string{"mode resize"}},
		{"1:splith[a*, b]", "hellobro switcher", "<1:splith[a*, b]>", []string{"hellobro a switcher"}},
		{"1:splith[a*, b]", "exec sh -c 'a; b', focus right", "<1:splith[a, b*]>", []string{"exec sh -c 'a"}},
		{"1:splith[a*, b]", `exec "sh -c 'a; b'", focus right`, "<1:splith[a, b*]>", []string{"exec sh -c 'a; b'"}},
		{"1:splith[a*, b]", "[con_id=__focused__] move right", "<1:splith[b, a*]>", nil},
		{"1:splith[a*] | 2:splith[b]", "[workspace=2] move to workspace current", "<1:splith[a*, b]>", nil},
		{"1:splith[a*, b]", "swap container with mark m", "<1:splith[a*, b]>", nil},
		{"1:splith[a*, b]", "[title=b] mark m; swap container with mark m", "<1:splith[b, a*]>", nil},
		{"1:splith[a*, b, c]", "[title=c] mark m; move window to mark m", "<1:splith[b, c, a*]>", nil},
		{"1:splith[a] float[f*]", "move position 10 px 20 px", "<1:splith[a] float[f*]>", nil},
		{"1:splith[a*, b]", "border none; border toggle", "<1:splith[a*, b]>", nil},
	} {
		tr := build(t, c.in)
		h := &host{}
		for _, r := range tr.Run(c.cmd, h) {
			if !r.Success && c.cmd != "swap container with mark m" && !strings.Contains(c.cmd, "'a; b',") {
				t.Errorf("%s: %+v", c.cmd, r)
			}
		}
		check(t, tr)
		if got := tr.DumpAll(); got != c.want {
			t.Errorf("%s on %s: %s, want %s", c.cmd, c.in, got, c.want)
		}
		if !reflect.DeepEqual(h.did, c.did) {
			t.Errorf("%s: did %q, want %q", c.cmd, h.did, c.did)
		}
	}
}

func TestRunResults(t *testing.T) {
	tr := build(t, "1:splith[a*, b]")
	got := tr.Run("focus left; frob; hellobro fail; [title=( ] kill", &host{})
	want := []Result{
		{Success: true},
		{ParseError: true, Error: `unknown command "frob"`},
		{Error: "failed"},
		{ParseError: true, Error: "bad title \"(\": error parsing regexp: missing closing ): `(`"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("results %+v", got)
	}
}

func TestRunMisc(t *testing.T) {
	tr := build(t, "1:splith[a] float[f*]")
	tr.Run("move position 10 px 20 px; resize set 300 px 200 px", &host{})
	if got := tr.FloatRect(tr.Focused()); got != r(10, 20, 310, 220) {
		t.Errorf("floating at %v", got)
	}
	tr.Run("border none; border toggle", &host{})
	if b := tr.Focused().Border; b != BorderNormal {
		t.Errorf("border %v", b)
	}
	tr = build(t, "1:splith[a*, b]")
	tr.Render()
	tr.Run("resize grow width 10 px or 10 ppt", &host{})
	if p := w(tr, "a").Percent; p < 0.59 || p > 0.61 {
		t.Errorf("grown to %v", p)
	}
	tr.Run("[title=b] mark --add x, mark --add y; [con_mark=x] unmark y", &host{})
	if m := tr.Marks(); !reflect.DeepEqual(m, []string{"x"}) {
		t.Errorf("marks %q", m)
	}
}

func TestRunQuoted(t *testing.T) {
	tr := build(t, "1:splith[a*, b]")
	h := &host{}
	got := tr.Run(`hellobro "done, bro; \"really\""; focus left`, h)
	if len(got) != 2 || !got[0].Success || !got[1].Success || !reflect.DeepEqual(h.did, []string{`hellobro a done, bro; "really"`}) {
		t.Errorf("results %+v, did %q", got, h.did)
	}
}
