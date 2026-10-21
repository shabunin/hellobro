# Developing hellobro

Back to the [README](../README.md). Agents: [AGENTS.md](../AGENTS.md).

## Code

- `kobo/`: what the apps share. The screen through FBInk (the update queue
  and e-ink waveforms), touch with its calibration, keyboards and buttons,
  the frontlight, drawing, panels, and the app's frame (header, tabs,
  footer). An app is a `kobo.Program` that fills in the pages.
- `audio/`: Bluetooth audio through the Kobo's bluez-alsa: finding the
  device and its PCM, MP3/WAV decoding, a player with pause, seeking and
  volume.
- `radio/`: Wi-Fi through wpa_supplicant and DHCP, turned on and off as
  Kobo does it (after KOReader's scripts); Bluetooth through BlueZ, with a
  pairing agent.
- `share/`: the SSH server for a computer: SFTP, a shell, commands, and
  pairing by a code on the screen; the mDNS responder that advertises it.
- `cmd/hellobro/`: the terminal (the emulator view, SSH, the local shell),
  the status bar, the switcher, the Music, Settings and Files tabs, sleep
  and light sleep, updates over the air.
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
`ssh.conf` pointing at this PC. brotools only goes there with
`make install-tools` (and its entry, `nm/brotools`). The apps share that directory, so
they share the touch calibration; each logs to its own `NAME.log` there.
`launch.sh APP` freezes Nickel while the app runs (leaving whatever holds
the watchdog alone) and thaws it afterwards; the app thaws it itself if it
ever gets stuck.

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
    SRV=$PWD/server/hellobro-server SRV_DIR=/tmp/srv harness/run.sh /tmp/hb 0 sessions
                                            # two sessions, password + key install, reconnect, shells
    MUSIC=~/Music harness/run.sh /tmp/hb 0 music      # the Music tab
    MUSIC=~/Music harness/run.sh /tmp/hb 0 statbar    # the status bar, the music hotkeys
    MUSIC=/tmp/lib harness/run.sh /tmp/hb 0 playlists  # folders, shuffle, repeat, playlists (/tmp/lib/Music/<folder>/<album>; it writes a playlist there)
    harness/run.sh /tmp/hb 0 radio                     # Wi-Fi/BT (stand-ins), the switcher, Ctrl+Alt+N, sleep
    harness/run.sh /tmp/hb 0 share                     # pairing, scp, make push's update and restart (ssh, scp on the PC)
    MUSIC=~/Music harness/run.sh /tmp/hb 0 doze        # settings, light sleep with music, media keys
    APP_EXTRA=dir harness/run.sh /tmp/hb 1 tmux       # saved host 1 (dir/ssh.conf, dir/id_ed25519): tmux attach, framed and fullscreen
    APP_EXTRA=dir harness/run.sh /tmp/hb 1 btop       # the same, btop
    harness/relay.sh /tmp/hb                           # the relay: sshd as the VPS, both ways through it, an update (with the server)

`MUSIC` is the music folder (default `harness/music`); the sound goes to
`OUT/pcm.raw` (`HELLOBRO_FAKE_PCM`: 48 kHz, 16-bit stereo), at the pace it
would play. Wi-Fi and Bluetooth are stand-ins (`HELLOBRO_FAKE_RADIO`;
`FAKE_RADIO=` for the PC's own, careful), and sleeping is pretended
(`HELLOBRO_FAKE_SLEEP`: never the PC's suspend). File sharing listens on
127.0.0.1 only, with a fixed pairing code, its storage in `OUT/onboard`
and no mDNS (it would claim kobo.local on the real network). The app's log ends up in `OUT/stdout.txt`, snapshots in `OUT/screen`
(`calls.txt` there lists every update with its waveform). The feeds live in
`harness/feeds`, each with an `# app:` line naming the app it drives, and
their shared helpers in `harness/feedlib.py`.

`harness/record.py` records what a terminal program prints, for
`termrender_test.go`:

    harness/record.py /tmp/hx.raw 113 33 5 hx main.go
    HELLOBRO_TERM_RAW=/tmp/hx.raw HELLOBRO_TERM_PNG=/tmp/hx.png CGO_ENABLED=0 go test -run TestRenderRecording ./cmd/hellobro
