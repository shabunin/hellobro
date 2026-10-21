# hello bro

A Go app for the Kobo Elipsa (N604, sunxi): it takes over the screen from
Nickel, draws its own e-ink UI through [FBInk](https://github.com/NiLuJe/FBInk),
and gives Nickel back when you leave. Launched from NickelMenu.

Tabs (tap, or F1–F7 on a Bluetooth keyboard):

- **Keys**: a text box for a Bluetooth keyboard, plus the last key's code.
- **Finger** / **Stylus**: drawing canvases; the stylus draws with pressure.
- **Net**: Wi-Fi status, a scan of nearby networks and LAN hosts, an mDNS
  browser, and an HTTPS test.
- **Sound**: Bluetooth audio through bluealsa: a test tone and an MP3/WAV
  player, with media keys.
- **SSH**: an SSH client with an xterm-256color terminal (tmux, helix, htop,
  Claude Code…), mouse taps and scrolling.
- **Info**: screen, touch and input device details.

Anywhere: swipe the left edge for the frontlight. Esc, the exit button or the
power button leave; it also leaves after 3 minutes idle (20 with an SSH
session).

## Terminal

- `ssh.conf` next to the binary lists saved hosts, one per line:
  `name  user@host[:port]`. The prompt also takes `ssh -p PORT user@host`.
- The Kobo's key is made on first use; `id_ed25519.pub` next to the binary
  goes in a server's `~/.ssh/authorized_keys`. Otherwise it asks for a
  password (Tab shows it while typing). Host keys are trusted on first use
  and kept in `known_hosts`.
- F11: fullscreen. Ctrl+= / Ctrl+- / Ctrl+0, or a swipe along the right
  edge: font size (remembered in `term-font`).
- While connected, every other key goes to the remote, Esc included.

E-ink notes: changed cells only are redrawn; small black & white changes use
the fast DU waveform and get a GL16 cleanup within a few seconds, bigger ones
use GL16, and after a few screenfuls of change the terminal gets one GC16
flash. Updates never overlap a refresh still in flight (on sunxi that mixes
both up).

## Building

The build runs in a podman container with Go and an ARM toolchain
(`tools/Containerfile`), since FBInk is linked in with cgo:

    make hellobro          # static ARM binary
    make install           # copy to a mounted Kobo (KOBO=/run/media/$USER/KOBOeReader)

`make install` copies `hellobro`, `hellobro.sh`, `bin/fbink` and the
NickelMenu entry (`nm/hellobro`), and writes a first `ssh.conf` pointing at
this PC. `hellobro.sh` freezes Nickel while the app runs (leaving whatever
holds the watchdog alone) and thaws it afterwards; the app thaws it itself
if it ever gets stuck.

`third_party/fbink` (header and static library) and `bin/fbink` are
prebuilt from FBInk v1.25.0-543-g886f25f1 with image and TrueType support.
To rebuild them: `./build-fbink.sh path/to/FBInk-checkout`.

`make hellobro-nocgo` builds without the container; every screen update then
runs `bin/fbink`, which is slow.

`go test` (on the PC, `CGO_ENABLED=0`) covers the SSH target parser;
`termrender_test.go` renders a recorded terminal stream to a PNG with the
app's own drawing code (see the comment at its top).
