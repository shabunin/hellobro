# ink: the UI library

`ink/` is an immediate-mode UI for e-ink, after [Gio](https://gioui.org):
each frame a window lays its UI out again from its own state, as plain
function calls; ink paints it in software onto a gray image, compares it
with the frame before, and gives the host only what changed: black and
white in the fast waveform (DU), grays in a grayscale one (GL16). The
aim is that every window in hellobro is made of the same controls.

Gio itself can't run on the Kobo: it wants a GPU, and knows nothing of
e-ink refreshes. ink keeps its shape, so its habits carry over:

```go
func (s *page) Layout(gtx ink.Ctx) ink.Dims {
	if s.ok.Clicked(gtx) {
		s.done = true
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(theme.Toggle(th, &s.wifi, "Wi-Fi").Layout),
		layout.Rigid(theme.Select(th, &s.size, "Page size", sizes...).Layout),
		layout.Rigid(theme.Button(th, &s.ok, "ok").Layout))
}
```

## The packages

- `ink`: `Ctx`, `Constraints`, `Dims`, `Dp`; the ops (`Fill`, `Frame`,
  `TextAt`, `Line`, `Disc`, `Ring`, `Polygon`, `Image`, `Draw` for what
  draws itself, `Invert`, `Offset`, `Clip`, `Record`, `Popup` for what
  opens over everything); input areas (`Input`); events (`Pointer`, with
  the pen's pressure and eraser, `Key`, `Text`, `Focus`, `Reveal`); the
  `Window` a host owns (input in, `Frame` out: `Update`s).
- `ink/layout`: `Flex` (`Rigid`, `Flexed`), `Stack`, `Inset`, `Spacer`,
  `Direction`, `Grid`; lists: a paged `List` (a swipe turns the page)
  and `Scroll`, which moves under the finger a pixel at a time and coasts
  on after a flick. Both are `Scroller`s, so a list, a table or a tree
  takes either. `Flick` is a view onto something bigger both ways (a map)
  that moves any way the finger goes.
- `ink/widget`: the controls' state: `Clickable` (tap, Enter, long
  press), `Bool`, `Enum`, `Editor` (a line), `TextArea` (many: titles,
  headings, lists, to-dos, quotes, as Markdown's marks hidden), `Canvas`
  (to draw on: the pen, or a finger), `Choice` (a list opening over the
  rest: a menu), `Select` (a dropdown), `Float` (a slider), `Range`
  (a span, two ends), `Date` (a calendar), `Clock` (hours and minutes),
  `Table` (heads that sort), `Tree` (`Node`s that open), `Toast`; what
  opens over the rest is a `Popover`.
- `ink/theme`: how they look: `Label`/`Body`/`Small`/`Title`,
  `Button`, `Row`, `Toggle`, `CheckBox`, `Radio`, `Select`, `Menu`,
  `Slider` (with ticks), `RangeSlider`, `Date`, `Clock`, `Progress`,
  `Section`, `Tile`, `Editor` (framed, with a ×), `TextArea` (with its
  toolbar of the paragraphs' kinds), `Table` (a search
  underlined in it), `Tree`, `Tabs`, `Header`, `List` with its bar,
  `GridList`, `ScrollBox`, `Dialog`, `Toast`, `Divider`.
- `ink/text`: faces by style and size, widths, wrapping, `Fit`.
- `ink/inkkobo`: runs an ink UI as a Kobo app (a `kobo.Program`), or
  as a pane in one (`Pane`: hellobro's windows); a frame's updates that
  are most of what they span go to the panel as one, so a list scrolled
  doesn't show its labels moved and its values not, for a moment.
- `ink/inktest`: drives a window without a screen: `Tap`, `Hold`,
  `Swipe`, `Key`, `Type`, `Find` by words, `Save` a PNG.
- `ink/gallery`: pages of every control, and two demos made of them, a
  Calendar and a Mail; `cmd/gallery` shows them on the Kobo.

## How it behaves

- Lengths are `Dp` (a 160th of an inch): 1.42 px each on the Elipsa,
  2 with the Sizes for fingers.
- A held control is drawn inverted, and that frame asks for the fast
  waveform (`Ctx.Quick`); letting go puts it back in grays.
- Focus: Tab and Shift+Tab (and the arrows, an editor's ↑ ↓ at its
  ends) go through the focusable areas in the order they were laid out,
  into a list's rows that don't show too: the list moves to show the one
  focused (`Reveal`); round past the last, the list goes back to its
  start (`Reveal.Edge`). A dropdown opens with Enter or Space, not ↓:
  the arrows go on, down a list of settings. Enter and Space press; Esc
  leaves an editor. The ring is the host's to stamp (`Window.Ring`),
  never painted into the frame. A touch hides it.
- Keys nobody in focus takes go to `ink.Root`; a window that asks for
  them there took them (Esc, its letters).
- A pointer goes to the topmost area under it; areas that `Watch` (a
  list) hear of it too, and may take it (`Ctx.Hand`): the others get a
  Cancel.
- A `Scroll` takes a finger once it has gone a little along it (till
  then the row under it is pressed, as ever). Each move is a quick frame,
  in black and white at once; the window keeps what quick frames drew
  and sends it again in grays with the next ordinary frame, the one
  after the finger lifts, even if nothing changed by then.
- A list, a scroll box, a dropdown's options move under the finger by
  default; `NoFling` stops them coasting on. A paged `List` is still
  there for what wants pages.
- Ink drawn (`Ctx.Pen`, a `Canvas`) is a quick frame marked `Pen`: the
  host may send it in its quickest waveform.
- Measuring (`Ctx.Measuring`) lays a widget out for its size only: no
  events, no areas, and its state left alone (a press stays a press).
- A popup (a dropdown's options, a menu) is painted last, under its
  field or over it where there's no room, and takes the keys: ↑ ↓
  Enter, Esc. A touch outside closes it and goes nowhere else.
- A frame is laid out when input came, when it's asked for
  (`Invalidate`, `InvalidateAt`), and at most once a tick otherwise.

## The gallery

`cmd/gallery` is an app of its own beside hellobro, as brotools is: its
page is one ink window (`ink/inkkobo`), the header and its buttons the
Kobo app's as ever. It's where the components are made and tried. Its header switches between the components
(a tab each: Controls, Pickers, Text, Lists, Scroll, Table, Tree,
Grid, Draw, Popups) and the demos: a Calendar (a day, a week of hours,
a month, an agenda; an event opened, a new one added) and a Mail (three
columns across, two upright; search, reply, flag, archive, a new
message). The apps move onto it one window at a
time once the controls they need are in the gallery.

## In hellobro

hellobro's windows move onto ink one at a time, each step a build that
works. In ink now: the settings' windows (Settings, Sharing, Wi-Fi,
Bluetooth, Battery, What uses it, Keys), the questions, facts and
cheat sheets (`over.go`, a card over the page), the switcher and its
menus, the quick menu and the workspaces (tiles, a workspace's windows
painted small into them), the launcher, the books' shelf, a book's
contents, bookmarks and pages small (`libui.go`, a PDF's as a grid),
the reader's style panel, the notebooks' list and settings, and the
book's and the notebook's bars. A window's ink is a pane over its body
(`inkBody`); what's drawn its own way (a page, thumbnails, the pen's
sample) goes in through `ink.Draw` or a kept image.

Still drawn the old way: the window manager's bar and title strips
(`kobo`), an empty workspace, the keyboard on the screen, the notices,
the ex line, the book's note, dictionary and cut buttons, the
notebook's tool choices, overview strip and selection buttons, and the
bodies themselves (terminal cells, book pages, notebook ink), which
stay drawn their own way for speed.

```sh
make gallery              # the ARM build
make push-gallery         # over Wi-Fi: gallery, launch.sh, nm/gallery
harness/run.sh OUT 0 ink  # the harness's feed of it
```

On the Kobo it's "ink Gallery" in NickelMenu, with hellobro closed
(NickelMenu may want Nickel restarted to see a new entry). Its log is
`gallery.log` (`make push-gallery-log`).
