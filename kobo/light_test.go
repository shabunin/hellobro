package kobo

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Idle, the light goes off; a touch or key brings it back as it was;
// sleeping meanwhile keeps the level from before.
func TestLightIdle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ntx_io")
	os.WriteFile(path, nil, 0o644)
	t.Setenv("HELLOBRO_NTX_IO", path) // a stand-in: set takes any level
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	a := &App{light: &frontlight{f: f, level: 40}, LightIdle: 30 * time.Second, inputAt: time.Now()}
	a.lightIdle()
	if a.light.level != 40 {
		t.Fatal("off too soon")
	}
	a.inputAt = time.Now().Add(-31 * time.Second)
	a.lightIdle()
	if a.light.level != 0 || a.lightIdled != 40 {
		t.Fatalf("idle: light %d, idled %d", a.light.level, a.lightIdled)
	}
	a.lightBack()
	if a.light.level != 40 || a.lightIdled != 0 {
		t.Fatalf("back: light %d", a.light.level)
	}
	// Off by hand: nothing to bring back.
	a.light.set(0)
	a.lightIdle()
	a.lightBack()
	if a.light.level != 0 {
		t.Errorf("off by hand came back: %d", a.light.level)
	}
	// Idled, then asleep: what's saved for waking is the level from before.
	a.light.set(25)
	a.lightIdle()
	if l := a.lightLevel(); l != 25 || a.lightIdled != 0 {
		t.Errorf("for sleep: %d, idled %d", l, a.lightIdled)
	}
	// Never, with LightIdle 0.
	a.light.set(25)
	a.LightIdle = 0
	a.lightIdle()
	if a.light.level != 25 {
		t.Error("LightIdle 0 turned it off")
	}
}

// The level the reader chose is set at the next start, over Nickel's,
// which is still the one put back on leaving.
func TestLightKept(t *testing.T) {
	dir := t.TempDir()
	ntx := filepath.Join(dir, "ntx_io")
	os.WriteFile(ntx, nil, 0o644)
	t.Setenv("HELLOBRO_NTX_IO", ntx)
	conf := filepath.Join(dir, "nickel.conf")
	os.WriteFile(conf, []byte("[PowerOptions]\nFrontLightLevel=60\n"), 0o644)
	was := nickelConf
	nickelConf = conf
	defer func() { nickelConf = was }()
	save := filepath.Join(dir, "light")

	l := openFrontlight(save)
	if l.level != 60 || l.kept != -1 {
		t.Fatalf("first start: %d, kept %d", l.level, l.kept)
	}
	l.set(0)
	l.keep()
	l.close()

	l = openFrontlight(save)
	defer l.close()
	if l.level != 0 || l.initial != 60 {
		t.Fatalf("next start: %d, Nickel's %d", l.level, l.initial)
	}
}
