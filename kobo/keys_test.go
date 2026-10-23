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

// Asleep, the modifiers still reach the Program: Ctrl+Alt+S's Ctrl and
// Alt come up after it went to sleep, and missed, they'd stay down after
// waking. Other keys don't.
func TestModifiersWhileAsleep(t *testing.T) {
	p := &keyProg{}
	a := &App{prog: p, asleep: true}
	a.ctrl[0], a.alt[0] = true, true
	for _, c := range []uint16{keyS, keyLeftAlt, KeyLeftCtrl, KeyRightShift, keyRightMeta, KeyPlayPause} {
		a.onKey(KeyMsg{Code: c, Value: 0})
	}
	want := []uint16{keyLeftAlt, KeyLeftCtrl, KeyRightShift, keyRightMeta}
	if !slices.Equal(p.got, want) {
		t.Errorf("the Program got %v, want %v", p.got, want)
	}
	if a.ctrl[0] || a.alt[0] {
		t.Error("Ctrl or Alt still held")
	}
}
