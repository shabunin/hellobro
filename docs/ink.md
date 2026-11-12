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
		layout.Rigid(theme.Stepper(th, &s.size, "Page size", sizes[s.i]).Layout),
		layout.Rigid(theme.Button(th, &s.ok, "ok").Layout))
}
```

## The packages

- `ink`: `Ctx`, `Constraints`, `Dims`, `Dp`; the ops (`Fill`, `Frame`,
  `TextAt`, `Image`, `Draw` for what draws itself, `Invert`, `Offset`,
  `Clip`, `Record`); input areas (`Input`); events (`Pointer`, `Key`,
  `Text`, `Focus`); the `Window` a host owns (input in, `Frame` out:
  `Update`s).
- `ink/layout`: `Flex` (`Rigid`, `Flexed`), `Stack`, `Inset`, `Spacer`,
  `Direction`, and a paged `List`: no pixel scrolling, a swipe turns
  the page.
- `ink/widget`: the controls' state: `Clickable` (tap, Enter, long
  press), `Bool`, `Enum`, `Stepper`, `Editor`.
- `ink/theme`: how they look: `Label`/`Body`/`Small`/`Title`,
  `Button`, `Row`, `Toggle`, `Stepper`, `Tabs`, `Header`, `List` with
  its bar, `Dialog`, `Divider`.
- `ink/text`: faces by style and size, widths, wrapping, `Fit`.
- `ink/inkkobo`: runs an ink UI as a Kobo app (a `kobo.Program`).
- `ink/inktest`: drives a window without a screen: `Tap`, `Hold`,
  `Swipe`, `Key`, `Type`, `Find` by words, `Save` a PNG.
- `ink/gallery`: pages of every control; `cmd/gallery` shows them on the Kobo.

## How it behaves

- Lengths are `Dp` (a 160th of an inch): 1.42 px each on the Elipsa,
  2 with the Sizes for fingers.
- A held control is drawn inverted, and that frame asks for the fast
  waveform (`Ctx.Quick`); letting go puts it back in grays.
- Focus: Tab and the arrows go through the focusable areas in the order
  they were laid out; Enter and Space press. The ring is the host's to
  stamp (`Window.Ring`), never painted into the frame. A touch hides it.
- Keys nobody in focus takes go to `ink.Root`; a window that asks for
  them there took them (Esc, its letters).
- A pointer goes to the topmost area under it; areas that `Watch` (a
  list) hear of it too, and may take it (`Ctx.Hand`): the others get a
  Cancel.
- A frame is laid out when input came, when it's asked for
  (`Invalidate`, `InvalidateAt`), and at most once a tick otherwise.

## The gallery

`cmd/gallery` is an app of its own beside hellobro, as brotools is: its
page is one ink window (`ink/inkkobo`), the header and its buttons the
Kobo app's as ever. It's where the components are made and tried;
hellobro doesn't use ink yet. The apps move onto it one window at a
time once the controls they need are in the gallery.

```sh
make gallery              # the ARM build
make push-gallery         # over Wi-Fi: gallery, launch.sh, nm/gallery
harness/run.sh OUT 0 ink  # the harness's feed of it
```

On the Kobo it's "ink Gallery" in NickelMenu, with hellobro closed
(NickelMenu may want Nickel restarted to see a new entry). Its log is
`gallery.log` (`make push-gallery-log`).
