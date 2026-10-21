# hello bro

A Go app for the Kobo Elipsa (N604, sunxi): it takes over the screen from
Nickel, draws its own e-ink UI through [FBInk](https://github.com/NiLuJe/FBInk),
and gives Nickel back when you leave. Launched from NickelMenu.

Tabs (tap, or F1–F9 on a Bluetooth keyboard; Alt+F1–F9 from a terminal):

- **Keys**: a text box for a Bluetooth keyboard, plus the last key's code.
- **Finger** / **Stylus**: drawing canvases; the stylus draws with pressure.
- **Net**: Wi-Fi status, a scan of nearby networks and LAN hosts, an mDNS
  browser, and an HTTPS test.
- **Sound**: Bluetooth audio through bluealsa: a test tone and an MP3/WAV
  player, with media keys.
- **SSH**: an SSH client with an xterm-256color terminal (tmux, helix, htop,
  Claude Code…), mouse taps and scrolling; several sessions at once.
- **Shell**: the same terminal with `/bin/sh` on the Kobo itself, in a PTY;
  several too.
- **System**: the device and firmware, user, kernel, uptime, CPU, memory,
  IP addresses, storage, battery, temperatures, add-ons, running SSH/telnet
  servers, and which developer tools are installed (or busybox applets).
- **Info**: screen, touch and input device details.

Anywhere: swipe the left edge for the frontlight. Esc, the exit button or the
power button leave; it also leaves after 3 minutes idle (20 with a
terminal session).

## Terminal

- `ssh.conf` next to the binary lists saved hosts, one per line:
  `name  user@host[:port]  [command]`; the command (say `tmux new -A -s
  kobo`) runs instead of a login shell. The prompt also takes
  `ssh -p PORT user@host [command]`, and a host typed there is added to
  `ssh.conf` once it connects.
- The Kobo's key is made on first use; `id_ed25519.pub` next to the binary
  goes in a server's `~/.ssh/authorized_keys`. Otherwise it asks for a
  password (Tab shows it while typing), and then offers to install the key
  there itself. Host keys are trusted on first use and kept in `known_hosts`.
- Keepalives every 15 s notice a dead connection; a dropped one (not one the
  remote closed) is made again, after 2, 4, 8… up to 30 s, with the password
  typed earlier (kept in memory only). Esc stops it.
- Sessions: Ctrl+Shift+T or the `+` button adds one, Ctrl+PgUp/PgDn (or
  Ctrl+Shift+Left/Right) or the numbered buttons switch, Esc on an idle one
  or `close` drops it.
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

`make test` runs the tests that work on the PC: the SSH target parser,
the server's layout, and `termrender_test.go`, which draws a recorded
terminal stream to a PNG with the app's own code when given one.

## The test server

`server/` is an SSH server made with [wish](https://github.com/charmbracelet/wish)
and Bubble Tea, to try the terminal against: a dashboard of the PC, a todo
list, a chat shared by everyone connected, a showcase of borders, colors,
attributes, blocks and braille, with mouse support.

    make server && (cd server && ./hellobro-server)    # listens on :2222

It lets anyone in, with any key or password, so keep it on the home network.
With `-password PW` it takes only that password or a key in
`.ssh/authorized_keys`, and handles hellobro's key-install command, to try
that flow (`harness/feeds/sessions.py` does).
On Fedora, open the port with `sudo firewall-cmd --add-port=2222/tcp`. Its
host key is made in `server/.ssh` on first start.

## Testing on the PC

`harness/` runs the ARM binary under `qemu-arm` (qemu-user) as if on the
Kobo. A fake FBInk keeps the screen as a file and saves PNG snapshots of
big updates. Scripted touch and keyboard input comes through FIFOs, and
fake input devices, Nickel settings and frontlight stand in for the rest:

    make hellobro
    harness/run.sh /tmp/hb 0 demo           # a tour of the tabs, in portrait
    harness/run.sh /tmp/hb 1 ssh            # landscape, a session with the server
    SRV=$PWD/server/hellobro-server SRV_DIR=/tmp/srv harness/run.sh /tmp/hb 0 sessions
                                            # two sessions, password + key install, reconnect, shells, System

The app's log ends up in `OUT/stdout.txt`, snapshots in `OUT/screen`
(`calls.txt` there lists every update with its waveform). The feeds live in
`harness/feeds`, with their shared helpers in `harness/feedlib.py`.

`harness/record.py` records what a terminal program prints, for
`termrender_test.go`:

    harness/record.py /tmp/hx.raw 113 33 5 hx main.go
    HELLOBRO_TERM_RAW=/tmp/hx.raw HELLOBRO_TERM_PNG=/tmp/hx.png CGO_ENABLED=0 go test -run TestRenderRecording .
