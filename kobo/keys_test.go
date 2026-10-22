package kobo

import (
	"slices"
	"testing"
)

// keyProg is a Program that only hears keys.
type keyProg struct {
	Program
	got []uint16
}

func (p *keyProg) Key(m KeyMsg) bool { p.got = append(p.got, m.Code); return true }

// Asleep or dozing, the modifiers still reach the Program: Ctrl+Alt+S's
// Ctrl and Alt come up after it went to sleep, and missed, they'd stay
// down after waking. Other keys don't, but the media keys while dozing.
func TestModifiersWhileAsleep(t *testing.T) {
	for _, dozing := range []bool{false, true} {
		p := &keyProg{}
		a := &App{prog: p, asleep: !dozing, dozing: dozing}
		a.ctrl[0], a.alt[0] = true, true
		for _, c := range []uint16{keyS, keyLeftAlt, KeyLeftCtrl, KeyRightShift, keyRightMeta, KeyPlayPause} {
			a.onKey(KeyMsg{Code: c, Value: 0})
		}
		want := []uint16{keyLeftAlt, KeyLeftCtrl, KeyRightShift, keyRightMeta}
		if dozing {
			want = append(want, KeyPlayPause)
		}
		if !slices.Equal(p.got, want) {
			t.Errorf("dozing %v: the Program got %v, want %v", dozing, p.got, want)
		}
		if a.ctrl[0] || a.alt[0] {
			t.Errorf("dozing %v: Ctrl or Alt still held", dozing)
		}
	}
}
