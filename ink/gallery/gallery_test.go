package gallery

import (
	"image"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"hellobro/ink/inktest"
	"hellobro/ink/theme"
)

// save writes what shows to $INK_OUT/name.png, if INK_OUT is set: to
// look at.
func save(t *testing.T, it *inktest.T, name string) {
	if dir := os.Getenv("INK_OUT"); dir != "" {
		if err := it.Save(filepath.Join(dir, name+".png")); err != nil {
			t.Fatal(err)
		}
	}
}

var sizes = map[string]image.Point{"portrait": inktest.Elipsa, "landscape": {1872, 1404}}

func start(t *testing.T, size image.Point) (*Gallery, *inktest.T) {
	g := New(theme.New())
	it := inktest.New(size, inktest.ElipsaMetric, g.Layout)
	if u := it.Updates; len(u) != 1 || !u[0].Full {
		t.Fatalf("the first frame: %v", u)
	}
	return g, it
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestControls(t *testing.T) {
	for name, size := range sizes {
		t.Run(name, func(t *testing.T) {
			g, it := start(t, size)
			save(t, it, name+"-controls")
			// A switch: held, it's inverted, quickly; let go, flipped.
			p, err := it.Find("Wi-Fi")
			must(t, err)
			it.Press(p)
			for _, u := range it.Updates {
				if u.Gray {
					t.Errorf("a press updated %v in grays", u.R)
				}
			}
			it.Release(p)
			if !g.wifi.Value || g.Said != "Wi-Fi on" {
				t.Errorf("Wi-Fi %v, said %q", g.wifi.Value, g.Said)
			}
			// A dropdown: a tap opens it under its box, a tap on an
			// option picks it and closes it.
			must(t, it.TapText("the screen, upright"))
			if !g.size.IsOpen() {
				t.Fatal("not open")
			}
			save(t, it, name+"-select")
			box, _ := it.W.FindText("the screen, upright")
			must(t, it.TapText("A5"))
			if g.size.IsOpen() || g.size.Value != 3 || g.Said != "page size: A5" {
				t.Errorf("open %v, value %d, said %q", g.size.IsOpen(), g.size.Value, g.Said)
			}
			// Open, a tap outside closes it, and goes nowhere else.
			must(t, it.TapText("A5"))
			before := g.wifi.Value
			wp, _ := it.Find("Wi-Fi")
			it.Tap(wp)
			if g.size.IsOpen() || g.wifi.Value != before {
				t.Errorf("open %v; Wi-Fi flipped through it: %v", g.size.IsOpen(), g.wifi.Value != before)
			}
			// By keys: ↓ in focus opens it, ↓ ↓ Enter picks, Esc closes.
			for range 30 {
				it.Key("Tab", 0)
				if g.pause.Field.Focused() {
					break
				}
			}
			if !g.pause.Field.Focused() {
				t.Fatal("Tab never got to the pause's dropdown")
			}
			it.Key("Enter", 0)
			if !g.pause.IsOpen() {
				t.Fatal("Enter didn't open it")
			}
			it.Key("Down", 0)
			it.Key("Down", 0)
			save(t, it, name+"-select-keys")
			it.Key("Enter", 0)
			if g.pause.IsOpen() || g.pause.Value != 2 || !g.pause.Field.Focused() {
				t.Errorf("open %v, value %d, focus back %v", g.pause.IsOpen(), g.pause.Value, g.pause.Field.Focused())
			}
			it.Key("Down", 0)
			if !g.pause.IsOpen() || !it.Key("Esc", 0) || g.pause.IsOpen() || g.Quit {
				t.Errorf("↓ then Esc: open %v, quit %v", g.pause.IsOpen(), g.Quit)
			}
			// More options than fit: paged, its bar there.
			must(t, it.TapText("as ai.conf says"))
			if !g.model.IsOpen() {
				t.Fatal("not open")
			}
			if n := g.model.List.Count(); n > 0 && n < len(models) { // paged: they don't all fit
				save(t, it, name+"-select-long")
				p, _ := it.Find(models[1])
				it.Swipe(p.Add(image.Pt(0, 200)), p.Sub(image.Pt(0, 200)))
				if g.model.List.First() == 0 || !g.model.IsOpen() {
					t.Errorf("swiped: from %d, open %v", g.model.List.First(), g.model.IsOpen())
				}
			}
			it.Key("Esc", 0)
			_ = box
		})
	}
}

func TestChecksRadiosSliders(t *testing.T) {
	g, it := start(t, inktest.Elipsa)
	must(t, it.TapText("The pen's button erases"))
	if !g.erase.Value {
		t.Error("checkbox not ticked")
	}
	must(t, it.TapText("A finger pans the page"))
	if g.finger.Value != "pans" {
		t.Errorf("radio: %q", g.finger.Value)
	}
	// The slider: a touch at its far end is the most.
	must(t, it.TapText("Pickers"))
	p, err := it.Find("Light")
	must(t, err)
	w := it.W.Size().X
	q := image.Pt(w-30, p.Y+60)
	it.Swipe(image.Pt(w/2, q.Y), q)
	if g.light.Value < 90 {
		t.Errorf("light %v after a drag to the end", g.light.Value)
	}
	save(t, it, "portrait-values")
}

func TestText(t *testing.T) {
	g, it := start(t, inktest.Elipsa)
	must(t, it.TapText("Text"))
	p, err := it.Find("find a notebook, a book…")
	must(t, err)
	it.Tap(p)
	it.Type("riddle")
	if g.search.Text() != "riddle" || g.Said != "looking for “riddle”" {
		t.Errorf("typed %q, said %q", g.search.Text(), g.Said)
	}
	save(t, it, "portrait-text")
	// The ×: a tap empties it.
	r, _ := it.W.FindText("riddle")
	it.Tap(image.Pt(it.W.Size().X-60, r.Min.Y+r.Dy()/2))
	if g.search.Text() != "" {
		t.Errorf("× left %q", g.search.Text())
	}
}

func TestListPages(t *testing.T) {
	g, it := start(t, inktest.Elipsa)
	must(t, it.TapText("Lists"))
	save(t, it, "portrait-list")
	first, n := g.list.First(), g.list.Count()
	if first != 0 || n < 5 || n >= len(g.rows) {
		t.Fatalf("page: %d from %d", n, first)
	}
	p, _ := it.Find("Notebook 3")
	g.Said = ""
	it.Swipe(p, p.Sub(image.Pt(0, 600)))
	if g.list.First() != n || g.Said != "" {
		t.Errorf("swiped: from %d, want %d; said %q", g.list.First(), n, g.Said)
	}
	p, _ = it.Find("Notebook " + strconv.Itoa(n+2))
	it.Swipe(p, p.Add(image.Pt(0, 600)))
	if g.list.First() != 0 {
		t.Errorf("swiped back: from %d", g.list.First())
	}
	must(t, it.TapText("Notebook 2"))
	if g.Said != "opened notebook 2" {
		t.Errorf("tap said %q", g.Said)
	}
	p, _ = it.Find("Notebook 4")
	it.Hold(p, time.Second)
	if g.Said != "held notebook 4" {
		t.Errorf("hold said %q", g.Said)
	}
}

func TestTiles(t *testing.T) {
	for name, size := range sizes {
		g, it := start(t, size)
		must(t, it.TapText("Grid"))
		save(t, it, name+"-grid")
		must(t, it.TapText("Bluetooth"))
		if g.Said != "tile: Bluetooth" {
			t.Errorf("%s: said %q", name, g.Said)
		}
	}
}

func TestPopups(t *testing.T) {
	for name, size := range sizes {
		t.Run(name, func(t *testing.T) {
			g, it := start(t, size)
			must(t, it.TapText("Popups"))
			// The menu: its actions under it; one picked, a toast says it.
			must(t, it.TapText("Notebook"))
			if !g.menu.IsOpen() {
				t.Fatal("the menu's not open")
			}
			save(t, it, name+"-menu")
			must(t, it.TapText("export as PDF"))
			if g.Said != "the menu: export as PDF" {
				t.Errorf("said %q", g.Said)
			}
			if _, err := it.Find("export as PDF"); err != nil {
				t.Error("no toast")
			}
			save(t, it, name+"-toast")
			it.Wait(4 * time.Second)
			if _, err := it.Find("export as PDF"); err == nil {
				t.Error("the toast stayed")
			}
			// The dialog: a tap under it does nothing; its buttons do.
			must(t, it.TapText("forget the notebook…"))
			if !g.asking {
				t.Fatal("no dialog")
			}
			save(t, it, name+"-dialog")
			p, _ := it.Find("keep it")
			it.Tap(image.Pt(20, it.W.Size().Y-20))
			if !g.asking {
				t.Error("a tap under it closed it")
			}
			it.Tap(p)
			if g.asking || g.Said != "kept" {
				t.Errorf("asking %v, said %q", g.asking, g.Said)
			}
		})
	}
}

// Only what changed goes to the panel: a switch flipped is a small
// update, not the window.
func TestDamage(t *testing.T) {
	g, it := start(t, inktest.Elipsa)
	if u := it.Frame(); len(u) != 0 {
		t.Errorf("nothing changed, yet %v", u)
	}
	p, _ := it.Find("Sync notebooks")
	it.Tap(p)
	if !g.sync.Value {
		t.Fatal("not flipped")
	}
	total := 0
	for _, u := range it.Updates {
		total += u.R.Dx() * u.R.Dy()
	}
	all := it.W.Size().X * it.W.Size().Y
	if total == 0 || total > all/8 {
		t.Errorf("flipping a switch updated %d of %d: %v", total, all, it.Updates)
	}
}

func TestAbout(t *testing.T) {
	g, it := start(t, inktest.Elipsa)
	must(t, it.TapText("About"))
	save(t, it, "portrait-about")
	if !it.Key("Esc", 0) || !g.Quit {
		t.Error("Esc didn't quit")
	}
}

// A touch on a popup's row keeps the keys in the popup: Esc closes it,
// and doesn't quit.
func TestPopupKeepsKeys(t *testing.T) {
	g, it := start(t, image.Pt(1872, 1404))
	must(t, it.TapText("as ai.conf says"))
	p, err := it.Find(models[2])
	must(t, err)
	it.Swipe(p.Add(image.Pt(0, 150)), p.Sub(image.Pt(0, 150)))
	if !it.Key("Esc", 0) || g.model.IsOpen() || g.Quit {
		t.Errorf("Esc after a swipe in it: open %v, quit %v", g.model.IsOpen(), g.Quit)
	}
}
