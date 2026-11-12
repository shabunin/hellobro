// Package demo is a page of every ink control, as a settings window
// would have them: for the tests to drive, and to see on the Kobo.
package demo

import (
	"fmt"
	"image"

	"hellobro/ink"
	"hellobro/ink/layout"
	"hellobro/ink/text"
	"hellobro/ink/theme"
	"hellobro/ink/widget"
)

// Demo is its state.
type Demo struct {
	Th *theme.Theme

	tab     widget.Enum
	back    widget.Clickable
	wifi    widget.Bool
	sync    widget.Bool
	size    widget.Stepper
	sizeI   int
	pause   widget.Stepper
	pauseI  int
	name    widget.Editor
	save    widget.Clickable
	ask     widget.Clickable
	asking  bool
	yes, no widget.Clickable
	list    layout.List
	rows    []widget.Clickable
	Said    string // what was done last, shown at the foot
	Quit    bool   // back was pressed
	sizes   []string
	pauses  []string
}

// New is the demo, in th.
func New(th *theme.Theme) *Demo {
	d := &Demo{Th: th, list: layout.List{Axis: layout.Vertical}}
	d.tab.Value = "settings"
	d.sizes = []string{"the screen, upright", "the screen, across", "A4", "A5", "Letter"}
	d.pauses = []string{"3 s", "2 s", "5 s", "10 s", "off"}
	d.rows = make([]widget.Clickable, 40)
	d.name.SetText("Tom Riddle's diary")
	return d
}

// Layout lays it out: a header, tabs, then the tab's page.
func (d *Demo) Layout(gtx ink.Ctx) ink.Dims {
	th := d.Th
	for _, e := range gtx.Events(ink.Root) {
		if k, ok := e.(ink.Key); ok {
			switch k.Name {
			case "Esc":
				if d.asking {
					d.asking = false
				} else {
					d.Quit = true
				}
			case "PageDown":
				d.list.ScrollBy(1)
			case "PageUp":
				d.list.ScrollBy(-1)
			}
		}
	}
	if d.back.Clicked(gtx) {
		d.Quit = true
		d.Said = "back"
	}
	d.tab.Update(gtx)
	ink.Fill(gtx.Ops, image.Rectangle{Max: gtx.Constraints.Max}, th.Bg)
	page := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx ink.Ctx) ink.Dims {
			h := theme.Header(th, "ink")
			h.Back, h.BackText = &d.back, "All Notebooks"
			return h.Layout(gtx)
		}),
		layout.Rigid(func(gtx ink.Ctx) ink.Dims {
			return layout.Inset{Top: th.Pad, Bottom: th.Pad, Left: th.Pad, Right: th.Pad}.Layout(gtx,
				theme.Tabs(th, &d.tab, []string{"settings", "list", "about"}, []string{"Settings", "A list", "About"}).Layout)
		}),
		layout.Flexed(1, func(gtx ink.Ctx) ink.Dims {
			return layout.UniformInset(th.Pad).Layout(gtx, func(gtx ink.Ctx) ink.Dims {
				switch d.tab.Value {
				case "list":
					return d.listPage(gtx)
				case "about":
					return d.aboutPage(gtx)
				}
				return d.settingsPage(gtx)
			})
		}),
		layout.Rigid(func(gtx ink.Ctx) ink.Dims {
			return layout.Inset{Left: th.Pad * 2, Bottom: th.Pad}.Layout(gtx, theme.Small(th, d.Said).Layout)
		}),
	)
	if d.asking {
		yes, no := theme.Button(th, &d.yes, "forget it"), theme.Button(th, &d.no, "keep it")
		yes.Strong = true
		theme.Dialog(th, "Forget the notebook?", "Its pages go, and its answers. This can't be undone.", no, yes).Layout(gtx)
	}
	return page
}

func (d *Demo) settingsPage(gtx ink.Ctx) ink.Dims {
	th := d.Th
	if d.wifi.Update(gtx) {
		d.Said = fmt.Sprintf("Wi-Fi %s", onOff(d.wifi.Value))
	}
	if d.sync.Update(gtx) {
		d.Said = fmt.Sprintf("sync %s", onOff(d.sync.Value))
	}
	if n := d.size.Stepped(gtx); n != 0 {
		d.sizeI = (d.sizeI + n%len(d.sizes) + len(d.sizes)) % len(d.sizes)
		d.Said = "page size: " + d.sizes[d.sizeI]
	}
	if n := d.pause.Stepped(gtx); n != 0 {
		d.pauseI = (d.pauseI + n%len(d.pauses) + len(d.pauses)) % len(d.pauses)
		d.Said = "asks after " + d.pauses[d.pauseI]
	}
	if d.save.Clicked(gtx) || d.name.Submitted(gtx) {
		d.Said = "named it: " + d.name.Text()
	}
	if d.ask.Clicked(gtx) {
		d.asking = true
	}
	if d.yes.Clicked(gtx) {
		d.asking, d.Said = false, "forgotten"
	}
	if d.no.Clicked(gtx) {
		d.asking, d.Said = false, "kept"
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(theme.Toggle(th, &d.wifi, "Wi-Fi").Layout),
		layout.Rigid(func(gtx ink.Ctx) ink.Dims {
			t := theme.Toggle(th, &d.sync, "Sync notebooks")
			t.Sub = "over SFTP, when the Wi-Fi's on"
			return t.Layout(gtx)
		}),
		layout.Rigid(theme.Stepper(th, &d.size, "Page size (new pages)", d.sizes[d.sizeI]).Layout),
		layout.Rigid(theme.Stepper(th, &d.pause, "AI asks after a pause", d.pauses[d.pauseI]).Layout),
		layout.Rigid(layout.Spacer{Height: th.Pad * 2}.Layout),
		layout.Rigid(func(gtx ink.Ctx) ink.Dims {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx ink.Ctx) ink.Dims {
					return layout.Inset{Left: th.Pad * 1.5, Right: th.Pad}.Layout(gtx, theme.Body(th, "Name").Layout)
				}),
				layout.Flexed(1, func(gtx ink.Ctx) ink.Dims {
					return framed(gtx, th, d.name.Focused(), func(gtx ink.Ctx) ink.Dims {
						return d.name.Layout(gtx, th.Face(gtx, th.TextSize, text.Regular), th.Fg, "a name")
					})
				}),
				layout.Rigid(layout.Spacer{Width: th.Pad}.Layout),
				layout.Rigid(theme.Button(th, &d.save, "rename").Layout),
			)
		}),
		layout.Rigid(layout.Spacer{Height: th.Pad * 2}.Layout),
		layout.Rigid(func(gtx ink.Ctx) ink.Dims {
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(layout.Spacer{Width: th.Pad * 1.5}.Layout),
				layout.Rigid(theme.Button(th, &d.ask, "forget the notebook…").Layout))
		}),
	)
}

// framed is w in a frame, padded, thicker in focus.
func framed(gtx ink.Ctx, th *theme.Theme, focus bool, w ink.Widget) ink.Dims {
	gtx.Constraints.Min = image.Pt(gtx.Constraints.Max.X, gtx.Dp(th.Touch))
	return layout.Background(gtx, func(gtx ink.Ctx) ink.Dims {
		r := image.Rectangle{Max: gtx.Constraints.Min}
		t := max(gtx.Dp(th.Border), 1)
		if focus {
			t++
		}
		ink.Frame(gtx.Ops, r, t, th.Fg)
		return ink.Dims{Size: r.Size()}
	}, func(gtx ink.Ctx) ink.Dims {
		gtx.Constraints.Min.Y = gtx.Dp(th.Touch)
		return layout.Inset{Left: th.Pad, Right: th.Pad}.Layout(gtx, func(gtx ink.Ctx) ink.Dims {
			return layout.W.Layout(gtx, w)
		})
	})
}

func (d *Demo) listPage(gtx ink.Ctx) ink.Dims {
	th := d.Th
	for i := range d.rows {
		if d.rows[i].Clicked(gtx) {
			d.Said = fmt.Sprintf("opened notebook %d", i+1)
		}
		if d.rows[i].LongPressed(gtx) {
			d.Said = fmt.Sprintf("held notebook %d", i+1)
		}
	}
	return theme.List(th, &d.list).Layout(gtx, len(d.rows), func(gtx ink.Ctx, i int) ink.Dims {
		r := theme.Row(th, &d.rows[i], fmt.Sprintf("Notebook %d", i+1))
		r.Sub = fmt.Sprintf("%d pages", (i*7)%23+1)
		r.Value = fmt.Sprintf("Nov %d", 12-i%12)
		r.More = true
		return r.Layout(gtx)
	})
}

func (d *Demo) aboutPage(gtx ink.Ctx) ink.Dims {
	th := d.Th
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(theme.Title(th, "ink").Layout),
		layout.Rigid(layout.Spacer{Height: th.Pad}.Layout),
		layout.Rigid(theme.Body(th, "An immediate-mode UI for e-ink, after Gio: the program lays its UI out from its state each frame; ink paints it, compares it with the frame before, and sends the panel only what changed: black and white at once, grays after.").Layout),
		layout.Rigid(layout.Spacer{Height: th.Pad}.Layout),
		layout.Rigid(theme.Small(th, "Tab and the arrows move the focus; Enter presses; a swipe turns a list's page.").Layout),
	)
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}
