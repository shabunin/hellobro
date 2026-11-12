package gallery

import (
	"image"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

func start(t *testing.T, size image.Point) (*Demo, *inktest.T) {
	d := New(theme.New())
	it := inktest.New(size, inktest.ElipsaMetric, d.Layout)
	if u := it.Updates; len(u) != 1 || !u[0].Full {
		t.Fatalf("the first frame: %v", u)
	}
	return d, it
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestSettings(t *testing.T) {
	for _, size := range []image.Point{inktest.Elipsa, {1872, 1404}} {
		name := "portrait"
		if size.X > size.Y {
			name = "landscape"
		}
		t.Run(name, func(t *testing.T) {
			d, it := start(t, size)
			save(t, it, name+"-settings")
			// A tap on the switch's row flips it; held, it shows inverted.
			p, err := it.Find("Wi-Fi")
			must(t, err)
			it.Press(p)
			if !d.wifi.Pressed() {
				t.Error("not pressed")
			}
			pressed := it.Updates
			save(t, it, name+"-pressed")
			it.Release(p)
			if !d.wifi.Value || d.Said != "Wi-Fi on" {
				t.Errorf("Wi-Fi %v, said %q", d.wifi.Value, d.Said)
			}
			for _, u := range pressed {
				if u.Gray && u.R.Dy() > it.W.Size().Y/2 {
					t.Errorf("a press updated %v in grays", u.R)
				}
			}
			// The stepper: › on, ‹ back, the value on.
			must(t, it.TapText("›"))
			if d.Said != "page size: the screen, across" {
				t.Errorf("› said %q", d.Said)
			}
			must(t, it.TapText("‹"))
			must(t, it.TapText("‹"))
			if d.Said != "page size: Letter" {
				t.Errorf("‹‹ said %q", d.Said)
			}
			// The keys: Tab to the editor, type, Enter.
			for range 20 {
				it.Key("Tab", 0)
				if d.name.Focused() {
					break
				}
			}
			if !d.name.Focused() {
				t.Fatal("Tab never got to the name")
			}
			if it.W.Ring().Empty() {
				t.Error("no focus ring")
			}
			it.Key("End", 0)
			for range len("diary") {
				it.Key("Back", 0)
			}
			it.Type("notes")
			it.Key("Enter", 0)
			if d.Said != "named it: Tom Riddle's notes" {
				t.Errorf("typed: %q", d.Said)
			}
			save(t, it, name+"-typed")
			// The dialog: a tap under it does nothing; its buttons do.
			must(t, it.TapText("forget the notebook…"))
			if !d.asking {
				t.Fatal("no dialog")
			}
			save(t, it, name+"-dialog")
			before := d.wifi.Value
			wp, _ := it.Find("Page size (new pages)")
			it.Tap(wp)
			if d.Said == "page size: the screen, across" || d.wifi.Value != before {
				t.Error("a tap went under the dialog")
			}
			must(t, it.TapText("keep it"))
			if d.asking || d.Said != "kept" {
				t.Errorf("asking %v, said %q", d.asking, d.Said)
			}
		})
	}
}

func TestListPages(t *testing.T) {
	d, it := start(t, inktest.Elipsa)
	must(t, it.TapText("A list"))
	if d.tab.Value != "list" {
		t.Fatalf("tab %q", d.tab.Value)
	}
	save(t, it, "portrait-list")
	first, n := d.list.First(), d.list.Count()
	if first != 0 || n < 5 || n >= len(d.rows) {
		t.Fatalf("page: %d from %d", n, first)
	}
	// A swipe up: the next page, and no row opened.
	p, _ := it.Find("Notebook 3")
	d.Said = ""
	it.Swipe(p, p.Sub(image.Pt(0, 600)))
	if d.list.First() != n {
		t.Errorf("swiped: from %d, want %d", d.list.First(), n)
	}
	if d.Said != "" {
		t.Errorf("a swipe said %q", d.Said)
	}
	save(t, it, "portrait-list2")
	// Back down, and a tap opens one; held, it's held.
	p, _ = it.Find("Notebook " + strconv.Itoa(n+2))
	it.Swipe(p, p.Add(image.Pt(0, 600)))
	if d.list.First() != 0 {
		t.Errorf("swiped back: from %d", d.list.First())
	}
	must(t, it.TapText("Notebook 2"))
	if d.Said != "opened notebook 2" {
		t.Errorf("tap said %q", d.Said)
	}
	p, _ = it.Find("Notebook 4")
	it.Hold(p, time.Second)
	if d.Said != "held notebook 4" {
		t.Errorf("hold said %q", d.Said)
	}
	it.Key("PageDown", 0)
	if d.list.First() != n {
		t.Errorf("PageDown: from %d", d.list.First())
	}
}

// Only what changed goes to the panel: a switch flipped is a small
// black-and-white update, not the window.
func TestDamage(t *testing.T) {
	d, it := start(t, inktest.Elipsa)
	if u := it.Frame(); len(u) != 0 {
		t.Errorf("nothing changed, yet %v", u)
	}
	p, _ := it.Find("Sync notebooks")
	it.Tap(p)
	if !d.sync.Value {
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
	_, it := start(t, inktest.Elipsa)
	must(t, it.TapText("About"))
	save(t, it, "portrait-about")
	found := false
	for _, s := range it.W.Texts() {
		if strings.HasPrefix(s, "An immediate-mode UI") {
			found = true
		}
	}
	if !found {
		t.Errorf("the words: %q", it.W.Texts())
	}
	if !it.Key("Esc", 0) {
		t.Error("Esc not taken")
	}
}
