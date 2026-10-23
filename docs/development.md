# Developing hellobro

Back to the [README](../README.md). Agents: [AGENTS.md](../AGENTS.md).

## Code

- `kobo/`: what the apps share. The screen through FBInk (the update queue
  and e-ink waveforms), touch with its calibration, keyboards and buttons,
  the frontlight, drawing, panels, and the app's frame (header, the
  tab bar, footer; or with `Config.Bar`, as hellobro, one bar for both,
  its tabs with what's in them and a menu button). An app is a
  `kobo.Program` that fills in the pages.
- `audio/`: Bluetooth audio through the Kobo's bluez-alsa: finding the
  device and its PCM, MP3/WAV decoding, a player with pause, seeking and
  volume.
- `radio/`: Wi-Fi through wpa_supplicant and DHCP, turned on and off as
  Kobo does it (after KOReader's scripts); Bluetooth through BlueZ, with a
  pairing agent.
- `share/`: the SSH server for a computer: SFTP, a shell, commands, and
  pairing by a code on the screen; the mDNS responder that advertises it.
- `wm/`: the window manager's tree, after i3's: workspaces, splits,
  tabbed and stacked containers, floating windows, the scratchpad,
  focus, moves and resizes, i3's command language, the tree as i3's IPC
  shows it, and as kept between runs. No drawing: tested on its own.
- `cmd/hellobro/`: the windows (`wm.go`: placing, drawing, taps and
  keys; `gui.go`: the settings' windows; `ipc.go`: i3's IPC and
  hellobro-msg), the terminal (the emulator view, SSH, the local shell),
  the status bar, the switcher, the settings' and power windows, sleep
  (and when idle), the Linux players' remote control (MPRIS), the hosts'
  files mounted (FUSE over SFTP), updates over the air.
- `cmd/brotools/`: the diagnostics pages.

## Building

The build runs in a podman container with Go and an ARM toolchain
(`tools/Containerfile`), since FBInk is linked in with cgo:

    make                   # static ARM binaries: hellobro, brotools
    make install           # hellobro to a mounted Kobo (KOBO=/run/media/$USER/KOBOeReader)
    make install-tools     # brotools as well, when it's wanted
    make uninstall-tools   # and off again

`make install` copies `hellobro`, `launch.sh`, `bin/fbink` and its
NickelMenu entry (`nm/hellobro`) to `.adds/hellobro`, and writes a first
`ssh.conf` pointing at this PC (once: hellobro takes it into
`hellobro.json`). brotools only goes there with
`make install-tools` (and its entry, `nm/brotools`). The apps share that directory, so
they share the touch calibration; each logs to its own `NAME.log` there.
`launch.sh APP` freezes Nickel while the app runs (leaving whatever holds
the watchdog alone) and thaws it afterwards; the app thaws it itself if it
ever gets stuck. With `nickel=closed` in `settings` (hellobro only) it
starts a copy of itself in a session of its own (`setsid`), so Nickel's
going doesn't take it along, ends Nickel as KOReader's `koreader.sh`
does, and starts it as `rcS` does when the app has quit
(`HELLOBRO_NICKEL=closed` tells the app).

A release: tag it (`git tag -a v0.2.0 -m v0.2.0`, push the tag), then
`touch cmd/hellobro/main.go && make dist`, so the build takes the tag as
its version. `dist/` gets `hellobro-TAG.zip` (the `.adds` tree to unzip
onto the Kobo: what `make install` copies, without `ssh.conf`), the bare
`hellobro` for updates over Wi-Fi, and `SHA256SUMS`. `gh release create
TAG dist/*` puts them up. brotools isn't in it.

`third_party/fbink` (header and static library) and `bin/fbink` are
prebuilt from FBInk v1.25.0-543-g886f25f1 with image and TrueType support.
To rebuild them: `./build-fbink.sh path/to/FBInk-checkout`.

`make hellobro-nocgo` (or `brotools-nocgo`) builds without the container;
every screen update then runs `bin/fbink`, which is slow.

`make test` runs vet and the tests that work on the PC: the SSH target
parser, the server's layout, and `cmd/hellobro/termrender_test.go`, which
draws a recorded terminal stream to a PNG with the app's own code when
given one.

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

`harness/` runs an app's ARM binary under `qemu-arm` (qemu-user) as if on
the Kobo. A fake FBInk keeps the screen as a file and saves PNG snapshots of
big updates. Scripted touch and keyboard input comes through FIFOs, and
fake input devices, Nickel settings and frontlight stand in for the rest:

    make
    harness/run.sh /tmp/hb 0 demo           # brotools: a tour of the tabs, in portrait
    harness/run.sh /tmp/hb 1 ssh            # hellobro: landscape, a session with the server
    harness/run.sh /tmp/hb 0 tiling         # the window manager: splits, tabbed, workspaces, floating, resize, scratchpad
    HELLOBRO_MSG=path/hellobro-msg harness/run.sh /tmp/hb 1 ipc   # i3's IPC: commands, events, the tree; hellobro-msg and swaymsg
    harness/run.sh /tmp/hb 1 layout && APP_EXTRA=dir harness/run.sh /tmp/hb2 1 restore
                                            # the windows kept, and back (dir: the first run's hellobro.json)
    MPRIS=1 harness/run.sh /tmp/hb 0 statbar           # the status bar, the media keys, a stand-in player
    harness/run.sh /tmp/hb 0 radio                     # Wi-Fi/BT (stand-ins), the switcher, sleep
    harness/run.sh /tmp/hb 0 share                     # pairing, scp, make push's update and restart (ssh, scp on the PC)
    TIMEOUT=650 harness/run.sh /tmp/hb 1 sleepidle    # sleep when idle: the toast, a key keeping it awake, sleep
    APP_EXTRA=dir harness/run.sh /tmp/hb 1 tmux       # saved host 1 (dir/ssh.conf, dir/id_ed25519): tmux attach, framed and fullscreen
    APP_EXTRA=dir harness/run.sh /tmp/hb 1 btop       # the same, btop
    APP_EXTRA=dir HELLOBRO_MPRIS_BUS=addr harness/run.sh /tmp/hb 0 ncmpcpp  # the same, ncmpcpp on mpd, a track playing
    harness/shots/run.sh /tmp/shots MUSIC             # a container for those (user bro on host pc), the README's: see it
    harness/relay.sh /tmp/hb                           # the relay: sshd as the VPS, both ways through it, an update (with the server)

`MPRIS=1` starts a session bus in `OUT/bus` with a stand-in mpv on it
(`harness/mpris`, three tracks; what it's asked goes to `OUT/mpris.txt`).
brotools' sound test plays from `MUSIC` (default `harness/music`) to
`OUT/pcm.raw` (`HELLOBRO_FAKE_PCM`: 48 kHz, 16-bit stereo), at the pace it
would play. Wi-Fi and Bluetooth are stand-ins (`HELLOBRO_FAKE_RADIO`;
`FAKE_RADIO=` for the PC's own, careful), and sleeping is pretended
(`HELLOBRO_FAKE_SLEEP`: never the PC's suspend). File sharing listens on
127.0.0.1 only, with a fixed pairing code, its storage in `OUT/onboard`
and no mDNS (it would claim kobo.local on the real network). The app's log ends up in `OUT/stdout.txt`, snapshots in `OUT/screen`
(`calls.txt` there lists every update with its waveform). The feeds live in
`harness/feeds`, each with an `# app:` line naming the app it drives, and
their shared helpers in `harness/feedlib.py` (`mod()` for Meta and a key,
`gui()` for a settings' window). A few feeds still say Ctrl+Alt+1–4
(`tab()`, from the tabs that were): that's a workspace laid out as the
tab was.

`harness/record.py` records what a terminal program prints, for
`termrender_test.go`:

    harness/record.py /tmp/hx.raw 113 33 5 hx main.go
    HELLOBRO_TERM_RAW=/tmp/hx.raw HELLOBRO_TERM_PNG=/tmp/hx.png CGO_ENABLED=0 go test -run TestRenderRecording ./cmd/hellobro
