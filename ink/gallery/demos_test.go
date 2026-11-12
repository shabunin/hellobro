package gallery

import (
	"image"
	"strings"
	"testing"
)

// The calendar: a week of hours, an event opened and closed, a day of
// the month, the agenda, and an event added.
func TestCalendar(t *testing.T) {
	for name, size := range sizes {
		t.Run(name, func(t *testing.T) {
			g, it := start(t, size)
			must(t, it.TapText("Calendar"))
			save(t, it, name+"-calendar-week")
			must(t, it.TapText(startsWith(it, "ink: the"))) // today, Thursday, at 10:00
			if g.cal.open < 0 || g.cal.events[g.cal.open].Title != "ink: the widgets" {
				t.Fatalf("not opened: %d", g.cal.open)
			}
			save(t, it, name+"-calendar-event")
			if !it.Key("Esc", 0) || g.cal.open >= 0 || g.Quit {
				t.Errorf("Esc: open %d, quit %v", g.cal.open, g.Quit)
			}
			must(t, it.TapText("Month"))
			save(t, it, name+"-calendar-month")
			must(t, it.TapText("20")) // a Friday
			if g.cal.view.Value != "day" || g.cal.at.Day() != 20 {
				t.Errorf("tapped the 20th: %s, %v", g.cal.view.Value, g.cal.at)
			}
			save(t, it, name+"-calendar-day")
			must(t, it.TapText("today"))
			must(t, it.TapText("Agenda"))
			save(t, it, name+"-calendar-agenda")
			if _, err := it.Find("Today, Thursday 12 November"); err != nil {
				t.Error(err)
			}
			n := len(g.cal.events)
			must(t, it.TapText("+ new"))
			it.Type("Post the letters")
			save(t, it, name+"-calendar-new")
			must(t, it.TapText("save"))
			if len(g.cal.events) != n+1 || g.cal.adding {
				t.Fatalf("%d events, adding %v", len(g.cal.events), g.cal.adding)
			}
			if !strings.Contains(g.Said, "Post the letters, Thu 12 Nov 13:00") {
				t.Errorf("said %q", g.Said)
			}
			if _, err := it.Find("Post the letters"); err != nil {
				t.Error(err)
			}
		})
	}
}

// Mail: a message read loses its dot; a reply quotes it; sent, it's in
// Sent; archived, it's gone from the inbox; the search narrows it.
func TestMail(t *testing.T) {
	for name, size := range sizes {
		t.Run(name, func(t *testing.T) {
			g, it := start(t, size)
			must(t, it.TapText("Mail"))
			save(t, it, name+"-mail")
			must(t, it.TapText("The book club, Thursday"))
			m := &g.mail
			if m.cur != 0 || m.msgs[0].Unread {
				t.Fatalf("read %d, unread %v", m.cur, m.msgs[0].Unread)
			}
			save(t, it, name+"-mail-read")
			must(t, it.TapText("reply"))
			if !m.composing || m.to.Text() != "Mira Holt" || m.subject.Text() != "Re: The book club, Thursday" ||
				!strings.Contains(m.body.Text(), "> We're on chapter nine") {
				t.Fatalf("the reply: %v %q %q %q", m.composing, m.to.Text(), m.subject.Text(), m.body.Text())
			}
			it.Type("See you there.")
			save(t, it, name+"-mail-reply")
			must(t, it.TapText("send"))
			if m.composing || m.msgs[len(m.msgs)-1].Box != "Sent" || !strings.HasPrefix(m.msgs[len(m.msgs)-1].Body, "See you there.") {
				t.Fatalf("sent: %v %+v", m.composing, m.msgs[len(m.msgs)-1])
			}
			must(t, it.TapText("archive"))
			if m.msgs[0].Box != "Archive" || m.cur != 1 {
				t.Errorf("archived: %s, now reading %d", m.msgs[0].Box, m.cur)
			}
			if _, err := it.Find("The book club, Thursday"); err == nil {
				t.Error("archived, it's still in the list")
			}
			must(t, it.TapText("search"))
			it.Type("beans")
			if is := m.shown(); len(is) != 1 || m.msgs[is[0]].Subject != "Recipe, as promised" {
				t.Errorf("found %v", is)
			}
			save(t, it, name+"-mail-search")
			it.Key("Esc", 0) // the search's
			if !it.Key("Esc", 0) || !g.Quit {
				t.Error("Esc didn't quit")
			}
		})
	}
}

// Draw: a finger only draws when it's let; undo takes the stroke away.
func TestDraw(t *testing.T) {
	g, it := start(t, sizes["portrait"])
	must(t, it.TapText("Draw"))
	p, err := it.Find("undo")
	must(t, err)
	at := p.Add(image.Pt(-600, 400))
	it.Swipe(at, at.Add(image.Pt(300, 200)))
	if g.canvas.Strokes() != 0 {
		t.Fatal("a finger drew")
	}
	must(t, it.TapText("a finger draws"))
	for i := 0; i < 3; i++ {
		it.Press(at)
		for k := 1; k <= 8; k++ {
			it.Move(at.Add(image.Pt(k*40, k*(10+i*15))))
			pen := false
			for _, u := range it.Updates {
				pen = pen || u.Pen
			}
			if k > 1 && !pen {
				t.Errorf("stroke %d move %d: not in the pen's waveform: %v", i, k, it.Updates)
			}
		}
		it.Release(at.Add(image.Pt(320, 8*(10+i*15))))
	}
	save(t, it, "portrait-draw")
	if g.canvas.Strokes() != 3 || g.Said != "3 strokes" {
		t.Fatalf("%d strokes, said %q", g.canvas.Strokes(), g.Said)
	}
	must(t, it.TapText("undo"))
	if g.canvas.Strokes() != 2 {
		t.Errorf("undone: %d", g.canvas.Strokes())
	}
	must(t, it.TapText("clear"))
	if g.canvas.Strokes() != 0 {
		t.Errorf("cleared: %d", g.canvas.Strokes())
	}
}

// The map moves both ways under the finger; a pin's a tap.
func TestMap(t *testing.T) {
	g, it := start(t, sizes["portrait"])
	must(t, it.TapText("Scroll"))
	p, err := it.Find("the library")
	must(t, err)
	drag(t, it, p.Add(image.Pt(40, 60)), image.Pt(-120, -80), 6)
	if pos := g.mapView.Pos; pos.X < 100 || pos.Y < 60 {
		t.Errorf("dragged by (-120, -80), at %v", pos)
	}
	q, err := it.Find("the library")
	must(t, err)
	if d := p.Sub(q); d.X < 100 || d.Y < 60 {
		t.Errorf("the market went from %v to %v", p, q)
	}
	save(t, it, "portrait-map")
	// The pin is left of its name: its middle a touch's half and a bit.
	r, _ := it.W.FindText("the library")
	it.Tap(image.Pt(r.Min.X-37, (r.Min.Y+r.Max.Y)/2))
	if g.Said != "the map: the library" {
		t.Errorf("tapped the pin, said %q", g.Said)
	}
}
