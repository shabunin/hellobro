package kobo

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// Events lost with a lift among them leave no finger down: the slots are
// read again, and if the device can't say (a file here), all lifted.
func TestTouchDropped(t *testing.T) {
	var b []byte
	ev := func(typ, code uint16, val int32) {
		var e [16]byte
		binary.LittleEndian.PutUint16(e[8:], typ)
		binary.LittleEndian.PutUint16(e[10:], code)
		binary.LittleEndian.PutUint32(e[12:], uint32(val))
		b = append(b, e[:]...)
	}
	ev(evAbs, absMTSlot, 0)
	ev(evAbs, absMTTrackingID, 7)
	ev(evAbs, absMTPositionX, 100)
	ev(evAbs, absMTPositionY, 200)
	ev(evSyn, synReport, 0)
	ev(evSyn, synDropped, 0) // the lift is lost
	ev(evAbs, absMTPositionX, 101)
	ev(evSyn, synReport, 0)
	path := filepath.Join(t.TempDir(), "events")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	out := make(chan TouchEv, 8)
	(&touchDevice{f: f}).run(out)
	close(out)
	var kinds []int
	for e := range out {
		kinds = append(kinds, e.Kind)
	}
	if len(kinds) != 2 || kinds[0] != TouchDown || kinds[1] != TouchUp {
		t.Errorf("events %v, want down then up", kinds)
	}
}
