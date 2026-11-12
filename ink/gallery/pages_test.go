package gallery

import (
	"image"
	"strings"
	"testing"

	"hellobro/ink/inktest"
)

// drag holds a finger at a, moves it by d in steps (a frame each), and
// says what the frames were: all black and white while it moved.
func drag(t *testing.T, it *inktest.T, a, d image.Point, steps int) {
	t.Helper()
	it.Press(a)
	for i := 1; i <= steps; i++ {
		it.Move(a.Add(d.Mul(i).Div(steps)))
		for _, u := range it.Updates {
			if u.Gray && i > 1 {
				t.Errorf("move %d: %v in grays", i, u.R)
			}
		}
	}
	it.Release(a.Add(d))
}

func grays(it *inktest.T) int {
	n := 0
	for _, u := range it.Updates {
		if u.Gray {
			n += u.R.Dx() * u.R.Dy()
		}
	}
	return n
}

// The scroll box: the page follows the finger, a pixel at a time, in
// black and white; let go, it's put back in grays.
func TestScrollBox(t *testing.T) {
	for name, size := range sizes {
		t.Run(name, func(t *testing.T) {
			g, it := start(t, size)
			must(t, it.TapText("Scroll"))
			save(t, it, name+"-scroll")
			p, err := it.Find("Moving under the finger")
			must(t, err)
			const mid = "PageDown and PageUp move it nearly a page, so a line stays in sight from the page before."
			m0, err := it.Find(mid)
			must(t, err)
			drag(t, it, p.Add(image.Pt(0, 200)), image.Pt(0, -150), 5)
			if grays(it) == 0 {
				t.Error("let go, nothing went back in grays")
			}
			if g.box.Offset() < 100 || g.box.Offset() > 200 {
				t.Errorf("moved 150 px, it's at %d", g.box.Offset())
			}
			if m1, err := it.Find(mid); err != nil || m1.Y-m0.Y != -g.box.Offset() {
				t.Errorf("a line from %v to %v, moved %d (%v)", m0, m1, g.box.Offset(), err)
			}
			// Too far back: it stops at the start.
			drag(t, it, p, image.Pt(0, 600), 4)
			if g.box.Offset() != 0 || g.box.First() != 0 {
				t.Errorf("back past the start: %d %d", g.box.First(), g.box.Offset())
			}
			// PageDown moves it nearly the box.
			it.Key("PageDown", 0)
			if g.box.Offset() == 0 {
				t.Error("PageDown didn't move it")
			}
			save(t, it, name+"-scroll-moved")
			// The strip, sideways: its covers go left.
			s, err := it.Find("Frankenstein")
			must(t, err)
			drag(t, it, s, image.Pt(-300, 0), 4)
			if g.strip.First() == 0 && g.strip.Offset() == 0 {
				t.Error("the strip didn't move")
			}
		})
	}
}

// The list under the finger: it moves as far as the finger, a tap is
// still a tap, and its end stays at the bottom.
func TestLiveList(t *testing.T) {
	g, it := start(t, inktest.Elipsa)
	must(t, it.TapText("Lists"))
	must(t, it.TapText("Under the finger"))
	if !g.live.Value {
		t.Fatal("not live")
	}
	p, _ := it.Find("Notebook 5")
	drag(t, it, p, image.Pt(0, -200), 8)
	q, err := it.Find("Notebook 5")
	if err != nil || q.Y != p.Y-200 {
		t.Errorf("200 px up: from %v to %v (%v)", p, q, err)
	}
	must(t, it.TapText("Notebook 7"))
	if g.Said != "opened notebook 7" {
		t.Errorf("tap said %q", g.Said)
	}
	for range 10 {
		drag(t, it, image.Pt(700, 1500), image.Pt(0, -1000), 3)
	}
	last, err := it.Find("Notebook 40")
	if err != nil || last.Y < it.W.Size().Y-300 {
		t.Errorf("at the end, the last is at %v (%v)", last, err)
	}
	save(t, it, "portrait-live-end")
}

func TestPickers(t *testing.T) {
	for name, size := range sizes {
		t.Run(name, func(t *testing.T) {
			g, it := start(t, size)
			must(t, it.TapText("Pickers"))
			save(t, it, name+"-pickers")
			// The calendar: November, today ringed; › to December, a day.
			must(t, it.TapText("pick a day"))
			if !g.day.IsOpen() {
				t.Fatal("not open")
			}
			if _, err := it.Find("November 2026"); err != nil {
				t.Error(err)
			}
			save(t, it, name+"-date")
			must(t, it.TapText("›"))
			if _, err := it.Find("December 2026"); err != nil {
				t.Fatal(err)
			}
			must(t, it.TapText("24"))
			if g.day.IsOpen() || g.day.Value.Day() != 24 || g.day.Value.Month() != 12 || g.Said != "backup on Thu 24 Dec" {
				t.Errorf("open %v, %v, said %q", g.day.IsOpen(), g.day.Value, g.Said)
			}
			// By keys: Enter opens it on the day, → a day on, Enter.
			it.Key("Enter", 0) // the field kept the focus
			if !g.day.IsOpen() {
				t.Fatal("Enter didn't open it")
			}
			it.Key("Right", 0)
			it.Key("Down", 0)
			it.Key("Enter", 0)
			if g.day.IsOpen() || g.day.Value.Day() != 1 || g.day.Value.Month() != 1 {
				t.Errorf("open %v, %v", g.day.IsOpen(), g.day.Value)
			}
			// The time: an hour, then the minutes.
			must(t, it.TapText("23:30"))
			if !g.alarm.IsOpen() {
				t.Fatal("not open")
			}
			save(t, it, name+"-clock")
			must(t, it.TapText("07"))
			must(t, it.TapText(":45"))
			if g.alarm.IsOpen() || g.Said != "sleeps at 07:45" {
				t.Errorf("open %v, said %q", g.alarm.IsOpen(), g.Said)
			}
			// The range: a finger near its top end takes that end.
			r, err := it.Find("Pages to export")
			must(t, err)
			w := it.W.Size().X
			y := r.Y + 60
			hx := 12 + (w-24)*40/120
			drag(t, it, image.Pt(hx, y), image.Pt(200, 0), 4)
			if g.pages.Low != 12 || g.pages.High <= 40 {
				t.Errorf("pages %v to %v", g.pages.Low, g.pages.High)
			}
		})
	}
}

func TestTable(t *testing.T) {
	for name, size := range sizes {
		t.Run(name, func(t *testing.T) {
			g, it := start(t, size)
			must(t, it.TapText("Table"))
			save(t, it, name+"-table")
			first := func() string { return books[g.shown[0]].Title }
			if first() != "A Study in Scarlet" {
				t.Errorf("by title, first %q", first())
			}
			must(t, it.TapText("Year"))
			if first() != "Pride and Prejudice" || !strings.HasPrefix(g.Said, "sorted by year") {
				t.Errorf("by year, first %q, said %q", first(), g.Said)
			}
			must(t, it.TapText("Year"))
			if first() != "Lolita" {
				t.Errorf("by year the other way, first %q", first())
			}
			p, err := it.Find("search the titles and authors")
			must(t, err)
			it.Tap(p)
			it.Type("woolf")
			if len(g.shown) != 3 || g.Said != "3 of 40 books" {
				t.Errorf("woolf: %d, said %q", len(g.shown), g.Said)
			}
			save(t, it, name+"-table-search")
			must(t, it.TapText("Orlando"))
			if g.Said != "opened Orlando" {
				t.Errorf("said %q", g.Said)
			}
			it.Tap(p)
			it.Type("zz")
			if _, err := it.Find("no book has “woolfzz”"); err != nil || len(g.shown) != 0 {
				t.Errorf("%d shown, %v", len(g.shown), err)
			}
		})
	}
}

func TestTree(t *testing.T) {
	for name, size := range sizes {
		t.Run(name, func(t *testing.T) {
			g, it := start(t, size)
			must(t, it.TapText("Tree"))
			if _, err := it.Find("Nov 12"); err == nil {
				t.Error("Diary's open")
			}
			must(t, it.TapText("Diary"))
			must(t, it.TapText("Work"))
			must(t, it.TapText("Meetings"))
			save(t, it, name+"-tree")
			must(t, it.TapText("Nov 12"))
			if g.Said != "picked nb/diary/1112" {
				t.Errorf("said %q", g.Said)
			}
			must(t, it.TapText("Notebooks"))
			if _, err := it.Find("Diary"); err == nil {
				t.Error("closed, Diary shows")
			}
			// By keys: in focus, → opens it.
			for range 40 {
				it.Key("Tab", 0)
				if g.tree.Row("bk").Focused() {
					break
				}
			}
			it.Key("Right", 0)
			if _, err := it.Find("Fiction"); err != nil {
				t.Error("→ didn't open Books")
			}
		})
	}
}

func TestCovers(t *testing.T) {
	for name, size := range sizes {
		t.Run(name, func(t *testing.T) {
			g, it := start(t, size)
			must(t, it.TapText("Grid"))
			must(t, it.TapText("Emma"))
			if g.Said != "opened Emma" {
				t.Errorf("said %q", g.Said)
			}
			n := g.covers.Count()
			it.Key("PageDown", 0)
			if g.covers.First() != n {
				t.Errorf("PageDown: from %d, want %d", g.covers.First(), n)
			}
			save(t, it, name+"-covers")
		})
	}
}
