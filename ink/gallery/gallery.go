// Package gallery is the pages of every ink control, as the apps would
// have them: for the tests to drive, and to see on the Kobo (cmd/gallery).
package gallery

import (
	"fmt"
	"image"

	"hellobro/ink"
	"hellobro/ink/layout"
	"hellobro/ink/theme"
	"hellobro/ink/widget"
)

// The options the dropdowns offer.
var (
	pageSizes = []string{"the screen, upright", "the screen, across", "A4", "A5", "Letter", "a square"}
	pauses    = []string{"3 s", "2 s", "5 s", "10 s", "off"}
	models    = []string{"as ai.conf says", "claude-haiku-5-5", "claude-sonnet-5-5", "claude-opus-5-5", "claude-fable-5-2",
		"gpt-5.5", "gpt-5.5-mini", "gemini-3-pro", "gemini-3-flash", "qwen-4-coder", "kimi-k3", "glm-5", "deepseek-v4", "grok-5"}
	actions = []string{"rename…", "duplicate", "export as PDF", "move to a folder…", "forget it…"}
	tabs    = []string{"controls", "text", "lists", "tiles", "popups", "about"}
	tabName = []string{"Controls", "Text", "Lists", "Tiles", "Popups", "About"}
)

// Gallery is its state.
type Gallery struct {
	Th *theme.Theme

	tab  widget.Enum
	back widget.Clickable
	Said string // what was done last, at the foot
	Quit bool   // quit was pressed

	// Controls
	page               layout.List
	wifi, sync         widget.Bool
	size, pause, model widget.Select
	erase, confirm     widget.Bool
	finger             widget.Enum
	light, letters     widget.Float
	// Text
	name, search widget.Editor
	rename, clr  widget.Clickable
	// Lists, tiles
	list  layout.List
	rows  []widget.Clickable
	tiles []widget.Clickable
	// Popups
	ask, toastBtn widget.Clickable
	asking        bool
	yes, no       widget.Clickable
	menu          widget.Choice
	toast         widget.Toast
}

// New is the gallery, in th.
func New(th *theme.Theme) *Gallery {
	g := &Gallery{Th: th, page: layout.List{Axis: layout.Vertical}, list: layout.List{Axis: layout.Vertical}}
	g.tab.Value = "controls"
	g.finger.Value = "turns"
	g.light = widget.Float{Value: 30, Min: 0, Max: 100, Step: 1}
	g.letters = widget.Float{Value: 18, Min: 10, Max: 36, Step: 1}
	g.rows = make([]widget.Clickable, 40)
	g.tiles = make([]widget.Clickable, 12)
	g.name.SetText("Tom Riddle's diary")
	return g
}

// Layout lays it out: a header, the tabs, the tab's page, a line saying
// what was done; a dialog or a toast over it all.
func (g *Gallery) Layout(gtx ink.Ctx) ink.Dims {
	th := g.Th
	for _, e := range gtx.Events(ink.Root) {
		if k, ok := e.(ink.Key); ok {
			switch k.Name {
			case "Esc":
				if g.asking {
					g.asking = false
				} else {
					g.Quit = true
				}
			case "PageDown":
				g.list.ScrollBy(1)
				g.page.ScrollBy(1)
			case "PageUp":
				g.list.ScrollBy(-1)
				g.page.ScrollBy(-1)
			}
		}
	}
	if g.back.Clicked(gtx) {
		g.Quit, g.Said = true, "quit"
	}
	g.tab.Update(gtx)
	ink.Fill(gtx.Ops, image.Rectangle{Max: gtx.Constraints.Max}, th.Bg)
	d := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx ink.Ctx) ink.Dims {
			h := theme.Header(th, "the controls")
			h.Back, h.BackText = &g.back, "quit"
			return h.Layout(gtx)
		}),
		layout.Rigid(func(gtx ink.Ctx) ink.Dims {
			return layout.UniformInset(th.Pad).Layout(gtx, theme.Tabs(th, &g.tab, tabs, tabName).Layout)
		}),
		layout.Flexed(1, func(gtx ink.Ctx) ink.Dims {
			return layout.UniformInset(th.Pad).Layout(gtx, func(gtx ink.Ctx) ink.Dims {
				switch g.tab.Value {
				case "text":
					return g.textPage(gtx)
				case "lists":
					return g.listPage(gtx)
				case "tiles":
					return g.tilePage(gtx)
				case "popups":
					return g.popupPage(gtx)
				case "about":
					return g.aboutPage(gtx)
				}
				return g.controlsPage(gtx)
			})
		}),
		layout.Rigid(func(gtx ink.Ctx) ink.Dims {
			return layout.Inset{Left: th.Pad * 2, Bottom: th.Pad}.Layout(gtx, theme.Small(th, g.Said).Layout)
		}),
	)
	if g.asking {
		yes, no := theme.Button(th, &g.yes, "forget it"), theme.Button(th, &g.no, "keep it")
		yes.Strong = true
		theme.Dialog(th, "Forget the notebook?", "Its pages go, and its answers. This can't be undone.", no, yes).Layout(gtx)
	}
	theme.Toast(th, &g.toast).Layout(gtx)
	return d
}

func (g *Gallery) controlsPage(gtx ink.Ctx) ink.Dims {
	th := g.Th
	if g.wifi.Update(gtx) {
		g.Said = "Wi-Fi " + onOff(g.wifi.Value)
	}
	if g.sync.Update(gtx) {
		g.Said = "sync " + onOff(g.sync.Value)
	}
	if g.size.Changed(gtx, len(pageSizes)) {
		g.Said = "page size: " + pageSizes[g.size.Value]
	}
	if g.pause.Changed(gtx, len(pauses)) {
		g.Said = "asks after " + pauses[g.pause.Value]
	}
	if g.model.Changed(gtx, len(models)) {
		g.Said = "model: " + models[g.model.Value]
	}
	if g.erase.Update(gtx) {
		g.Said = "the pen's button erases: " + onOff(g.erase.Value)
	}
	if g.confirm.Update(gtx) {
		g.Said = "asks before forgetting: " + onOff(g.confirm.Value)
	}
	if g.finger.Update(gtx) {
		g.Said = "a finger " + g.finger.Value
	}
	if g.light.Changed(gtx) {
		g.Said = fmt.Sprintf("light %d%%", int(g.light.Value))
	}
	if g.letters.Changed(gtx) {
		g.Said = fmt.Sprintf("letters %d px", int(g.letters.Value))
	}
	sub := func(w theme.ToggleStyle, s string) theme.ToggleStyle { w.Sub = s; return w }
	rows := []ink.Widget{
		theme.Section(th, "Switches"),
		theme.Toggle(th, &g.wifi, "Wi-Fi").Layout,
		sub(theme.Toggle(th, &g.sync, "Sync notebooks"), "over SFTP, when the Wi-Fi's on").Layout,
		theme.Section(th, "Dropdowns"),
		theme.Select(th, &g.size, "Page size (new pages)", pageSizes...).Layout,
		theme.Select(th, &g.pause, "AI asks after a pause", pauses...).Layout,
		theme.Select(th, &g.model, "AI model", models...).Layout,
		theme.Section(th, "Checkboxes, a choice of one"),
		theme.CheckBox(th, &g.erase, "The pen's button erases").Layout,
		theme.CheckBox(th, &g.confirm, "Ask before forgetting a notebook").Layout,
		theme.Radio(th, &g.finger, "turns", "A finger turns pages").Layout,
		theme.Radio(th, &g.finger, "writes", "A finger writes, as the pen").Layout,
		theme.Radio(th, &g.finger, "pans", "A finger pans the page").Layout,
		theme.Section(th, "Values"),
		theme.Slider(th, &g.light, "Light", fmt.Sprintf("%d%%", int(g.light.Value))).Layout,
		theme.Slider(th, &g.letters, "Letters' size", fmt.Sprintf("%d px", int(g.letters.Value))).Layout,
		theme.Progress(th, g.light.Frac(), fmt.Sprintf("a download, as far as the light: %d%%", int(g.light.Value))).Layout,
	}
	return theme.List(th, &g.page).Layout(gtx, len(rows), func(gtx ink.Ctx, i int) ink.Dims { return rows[i](gtx) })
}

func (g *Gallery) textPage(gtx ink.Ctx) ink.Dims {
	th := g.Th
	if g.rename.Clicked(gtx) || g.name.Submitted(gtx) {
		g.Said = "named it: " + g.name.Text()
	}
	if g.search.Changed(gtx) {
		g.Said = "looking for “" + g.search.Text() + "”"
	}
	search := theme.Editor(th, &g.search, "find a notebook, a book…")
	search.Clear = &g.clr
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(theme.Section(th, "Fields")),
		layout.Rigid(func(gtx ink.Ctx) ink.Dims {
			return layout.Inset{Left: th.Pad * 1.5, Right: th.Pad * 1.5, Top: th.Pad}.Layout(gtx, func(gtx ink.Ctx) ink.Dims {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, theme.Editor(th, &g.name, "a name").Layout),
					layout.Rigid(layout.Spacer{Width: th.Pad}.Layout),
					layout.Rigid(theme.Button(th, &g.rename, "rename").Layout))
			})
		}),
		layout.Rigid(func(gtx ink.Ctx) ink.Dims {
			return layout.Inset{Left: th.Pad * 1.5, Right: th.Pad * 1.5, Top: th.Pad * 1.5, Bottom: th.Pad}.Layout(gtx, search.Layout)
		}),
		layout.Rigid(theme.Section(th, "Words")),
		layout.Rigid(func(gtx ink.Ctx) ink.Dims {
			return layout.UniformInset(th.Pad*1.5).Layout(gtx, func(gtx ink.Ctx) ink.Dims {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(theme.Title(th, "A title").Layout),
					layout.Rigid(layout.Spacer{Height: th.Pad}.Layout),
					layout.Rigid(theme.Body(th, "Body text, wrapped to the room it has: a notebook's settings, a book's details, what a dialog asks. Lines break at spaces, and a word too long for a line goes on in pieces.").Layout),
					layout.Rigid(layout.Spacer{Height: th.Pad}.Layout),
					layout.Rigid(theme.Small(th, "Small text, said aside: where a file is, when it was read.").Layout),
				)
			})
		}),
	)
}

func (g *Gallery) listPage(gtx ink.Ctx) ink.Dims {
	th := g.Th
	for i := range g.rows {
		if g.rows[i].Clicked(gtx) {
			g.Said = fmt.Sprintf("opened notebook %d", i+1)
		}
		if g.rows[i].LongPressed(gtx) {
			g.Said = fmt.Sprintf("held notebook %d", i+1)
		}
	}
	return theme.List(th, &g.list).Layout(gtx, len(g.rows), func(gtx ink.Ctx, i int) ink.Dims {
		r := theme.Row(th, &g.rows[i], fmt.Sprintf("Notebook %d", i+1))
		r.Sub = fmt.Sprintf("%d pages", (i*7)%23+1)
		r.Value = fmt.Sprintf("Nov %d", 12-i%12)
		r.More = true
		return r.Layout(gtx)
	})
}

var tileNames = [][2]string{
	{"Alpine", "a shell in a jail"}, {"Kobo shell", "/bin/sh"}, {"Books", "EPUBs and PDFs"}, {"Notebooks", "all, in folders"},
	{"Wi-Fi", "networks"}, {"Bluetooth", "keyboards, headphones"}, {"Sharing", "SFTP and SSH"}, {"Settings", "the rest"},
	{"Battery", "how long it lasts"}, {"Rotate", "the screen"}, {"Light", "on / off"}, {"Sleep", ""},
}

func (g *Gallery) tilePage(gtx ink.Ctx) ink.Dims {
	th := g.Th
	for i := range g.tiles {
		if g.tiles[i].Clicked(gtx) {
			g.Said = "tile: " + tileNames[i][0]
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(theme.Section(th, "Tiles, as the launcher's")),
		layout.Rigid(func(gtx ink.Ctx) ink.Dims {
			return layout.UniformInset(th.Pad).Layout(gtx, func(gtx ink.Ctx) ink.Dims {
				return layout.Grid{MinWidth: 200, Gap: th.Pad}.Layout(gtx, len(g.tiles), func(gtx ink.Ctx, i int) ink.Dims {
					t := theme.Tile(th, &g.tiles[i], tileNames[i][0], tileNames[i][1])
					t.Icon = icon(th, i)
					return t.Layout(gtx)
				})
			})
		}),
	)
}

// icon is a plain icon of lines and discs, by place: what ink's paths
// can draw.
func icon(th *theme.Theme, i int) func(gtx ink.Ctx, r image.Rectangle) {
	return func(gtx ink.Ctx, r image.Rectangle) {
		w := max(gtx.Dp(1.5), 2)
		s := r.Dx()
		at := func(x, y int) image.Point { return r.Min.Add(image.Pt(s*x/24, s*y/24)) }
		switch i % 4 {
		case 0: // a box, a prompt
			ink.Frame(gtx.Ops, image.Rectangle{Min: at(3, 5), Max: at(21, 19)}, w, th.Fg)
			ink.Line(gtx.Ops, w, th.Fg, at(7, 9), at(10, 12), at(7, 15))
		case 1: // a page, lines on it
			ink.Frame(gtx.Ops, image.Rectangle{Min: at(5, 3), Max: at(19, 21)}, w, th.Fg)
			for y := 8; y <= 16; y += 4 {
				ink.Line(gtx.Ops, w, th.Fg, at(8, y), at(16, y))
			}
		case 2: // waves out of a dot
			ink.Disc(gtx.Ops, at(12, 18), s/12, th.Fg)
			ink.Ring(gtx.Ops, at(12, 18), s/3, w, th.Fg)
		case 3: // a triangle
			ink.Polygon(gtx.Ops, th.Fg, at(12, 4), at(20, 19), at(4, 19))
		}
	}
}

func (g *Gallery) popupPage(gtx ink.Ctx) ink.Dims {
	th := g.Th
	if g.ask.Clicked(gtx) {
		g.asking = true
	}
	if g.yes.Clicked(gtx) {
		g.asking, g.Said = false, "forgotten"
	}
	if g.no.Clicked(gtx) {
		g.asking, g.Said = false, "kept"
	}
	if i, ok := g.menu.Picked(gtx, len(actions)); ok {
		g.Said = "the menu: " + actions[i]
		g.toast.Show(gtx, actions[i])
	}
	if g.toastBtn.Clicked(gtx) {
		g.toast.Show(gtx, "a toast, for a moment")
	}
	pad := layout.Inset{Left: th.Pad * 1.5, Top: th.Pad, Bottom: th.Pad}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(theme.Section(th, "A menu of actions")),
		layout.Rigid(func(gtx ink.Ctx) ink.Dims {
			return pad.Layout(gtx, func(gtx ink.Ctx) ink.Dims {
				return layout.W.Layout(gtx, theme.Menu(th, &g.menu, "Notebook", actions...).Layout)
			})
		}),
		layout.Rigid(theme.Section(th, "A dialog")),
		layout.Rigid(func(gtx ink.Ctx) ink.Dims {
			return pad.Layout(gtx, func(gtx ink.Ctx) ink.Dims {
				return layout.W.Layout(gtx, theme.Button(th, &g.ask, "forget the notebook…").Layout)
			})
		}),
		layout.Rigid(theme.Section(th, "A toast: a word for a moment")),
		layout.Rigid(func(gtx ink.Ctx) ink.Dims {
			return pad.Layout(gtx, func(gtx ink.Ctx) ink.Dims {
				return layout.W.Layout(gtx, theme.Button(th, &g.toastBtn, "show a toast").Layout)
			})
		}),
	)
}

func (g *Gallery) aboutPage(gtx ink.Ctx) ink.Dims {
	th := g.Th
	return layout.UniformInset(th.Pad*1.5).Layout(gtx, func(gtx ink.Ctx) ink.Dims {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(theme.Title(th, "ink").Layout),
			layout.Rigid(layout.Spacer{Height: th.Pad}.Layout),
			layout.Rigid(theme.Body(th, "An immediate-mode UI for e-ink, after Gio: the program lays its UI out from its state each frame; ink paints it, compares it with the frame before, and sends the panel only what changed: black and white at once, grays after.").Layout),
			layout.Rigid(layout.Spacer{Height: th.Pad}.Layout),
			layout.Rigid(theme.Small(th, "Tab and the arrows move the focus; Enter presses; a swipe turns a list's page; Esc closes what's open, then quits.").Layout),
		)
	})
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}
