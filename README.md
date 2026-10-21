# hello bro

Go apps for the Kobo Elipsa (N604, sunxi): they take over the screen from
Nickel, draw their own e-ink UI through [FBInk](https://github.com/NiLuJe/FBInk),
and give Nickel back when you leave. Each has its own NickelMenu entry.

**hellobro** ("Hello Bro") is a terminal for a Bluetooth keyboard, with
music, Kobo's wireless settings and a way in for a computer. Tabs (tap,
or Ctrl+Alt+1–5, which work
everywhere, fullscreen and in a session too):

- **SSH**: an SSH client with an xterm-256color terminal (tmux, helix, htop,
  Claude Code…), mouse taps and scrolling; several sessions at once.
- **Shell**: the same terminal with `/bin/sh` on the Kobo itself, in a PTY;
  several too.
- **Music**: a player for the music on the Kobo (folders of MP3/WAV, with
  their covers), to Bluetooth headphones or a DAC: by album, by folder
  (subfolders and all) or by playlist, in order or shuffled. It goes on
  playing on the other tabs.
- **Settings**: Wi-Fi and Bluetooth without going back to Nickel: on and
  off, networks (join, with a password; leave; forget; one by name), and
  devices (scan, pair, connect, disconnect, forget); and hellobro's own
  settings: light sleep, sleep with music on, file sharing, mDNS, the
  status bar.
- **Files**: sharing with a computer over SFTP and SSH (how to reach the
  Kobo, pairing, who's connected and sending what), and the Kobo's
  storage: folders to open, files and folders to delete.

Ctrl+Shift+Tab, anywhere, is a switcher in the manner of rofi: the
sessions, the tabs, the saved hosts (a new session to one) and a new
shell, narrowed by typing (fuzzy); Up/Down and Enter, or 1–9 before
typing.

**Sleep**: the power button, Ctrl+Alt+S, the header's `sleep`, or "sleep"
in the switcher put the Kobo to sleep with hellobro still up, as Nickel would (a sleep
screen, the light off, suspend to RAM); the power button wakes it, and
it's all as it was. The music stops where it was (Bluetooth goes off),
Wi-Fi and Bluetooth go off and come back on waking, and the SSH sessions
reconnect once Wi-Fi is back. Nickel holds the power button's input
device (frozen, all the while hellobro is up), so hellobro reads the
button's GPIO instead (`/sys/kernel/debug/gpio`); the presses still queue
up for Nickel, which may see them when hellobro quits (and go to sleep
once).

**Light sleep** is for listening: the light goes off and the screen shows
what's playing (the cover, the track, the album), but the Kobo stays
awake, so the music plays on and the headphones' buttons (or a keyboard's
media keys) play and pause, skip, and set the volume. The power button or
any other key wakes it up, to the page as it was; touching the screen
doesn't (unless there's no keyboard and no power button to be had). With music playing it comes
after a while without input (5 minutes; Settings), and Ctrl+Alt+S dozes
instead of sleeping (Settings: or not). Wi-Fi goes off meanwhile (or not),
and comes back on waking; with Bluetooth on, the Wi-Fi chip keeps its
power (on the Elipsa it's Bluetooth's chip too: cutting it took the
headphones with it). Once the music has stopped for a while (10
minutes), the Kobo goes to sleep properly.

**brotools** ("Bro Tools") is diagnostics. Tabs (tap, or Ctrl+Alt+1–7):

- **Keys**: a text box for a Bluetooth keyboard, plus the last key's code.
- **Finger** / **Stylus**: drawing canvases; the stylus draws with pressure.
- **Net**: Wi-Fi status, a scan of nearby networks and LAN hosts, an mDNS
  browser, and an HTTPS test.
- **Sound**: Bluetooth audio through bluealsa: a test tone and an MP3/WAV
  player, with media keys.
- **System**: the device and firmware, user, kernel, uptime, CPU, memory,
  IP addresses, storage, battery, temperatures, add-ons, running SSH/telnet
  servers, and which developer tools are installed (or busybox applets).
- **Info**: screen, touch and input device details.

In both: swipe the left edge for the frontlight. Fullscreen (F11): swipe
in from the top or bottom edge to leave it. Esc or the exit button leave;
the power button sleeps; they also leave after 3 minutes idle (hellobro: 20
with a session up, 6 hours while music plays).

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

## Terminal

- `ssh.conf` next to hellobro lists saved hosts, one per line:
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
- A status bar along the bottom, fullscreen too: what's playing, and the
  battery (charge, temperature), CPU and memory use, every 5 s.
  Ctrl+Shift+B hides or shows it (remembered in `status-bar`); a tap on it
  plays or pauses.
- While connected, every other key goes to the remote, Esc included, but
  for the music's keys below.

## Music

- Albums are folders with MP3 or WAV files, up to 5 levels under the Kobo's
  storage (`/mnt/onboard`; hidden folders skipped), say
  `Music/Artist/Album/`. The cover is `cover.jpg`, `folder.jpg` or another
  image there (a "front" one before a "back").
- It plays to the Bluetooth device connected in Kobo's settings; a paired
  one that isn't connected gets a "connect" button.
- On any tab: Ctrl+Shift+Space plays or pauses, Ctrl+Shift+< and > go to
  the previous and next track; so do media keys, and the volume keys (a
  headset's, a keyboard's) set the volume. Headphones' buttons come as
  keys, or, while Nickel has a media player registered with BlueZ, as
  BlueZ's calls to it (Play, Next…), which hellobro watches the system bus
  for: Nickel, frozen, would never answer them.
- The library has three views (tap them, or Tab / v / 1-3):
  - **Albums**: every folder with tracks in it.
  - **Folders**: the folders as they are, from the one all the music is
    in. A folder plays with all its subfolders, in path order.
  - **Playlists**: M3U files (`.m3u`, `.m3u8`). New ones go in
    `Music/Playlists/` (or `Playlists/` with no `Music` folder); ones found
    anywhere in the library show up too, Windows paths and all. Tracks
    that aren't there any more show dimmed, and are skipped.
- A tap (or Enter) opens an album, a folder or a playlist, or plays a
  track: what it's in plays on from there. The bar at the bottom plays
  what's open from the start, in order (play) or shuffled (shuffle); at the
  top of Albums or Folders that's all the music. "+ playlist" adds what's
  open to a playlist, picked or new (type its name). In a playlist: remove,
  ↑ and ↓ move the track selected.
- Under the player's buttons: shuffle (on, the track playing stays and the
  rest come in a random order), repeat (off, all: round again, one: the
  track over) and "+ playlist" for the track playing.
- On the Music tab: Space or k play/pause, Left/Right (j/l) seek 10 s, n/p
  next/previous, s stop, = and − volume, r shuffle, t repeat;
  Up/Down/PgUp/PgDn/Home/End and Enter pick, Shift+Enter plays the row
  selected whole, Esc or Backspace go back up, f shows what's playing in
  its album; a adds the row selected to a playlist, Shift+A what's open;
  in a playlist, Shift+Up/Down move a track, Delete or x takes it off (on
  a playlist itself: deletes the playlist, asked first). Or tap.
- The volume, shuffle and repeat, what's playing (the album, folder or
  playlist, the track, where in it) are kept in `music-state`.

## Settings: Wi-Fi and Bluetooth

- Wi-Fi: tap a network to pick it, again (or Enter) to join or leave it;
  a new secured one asks for its password (Tab or `show` reveals it).
  "other network…" (or n) joins a hidden one by name. Networks are saved
  when wpa_supplicant's file allows it. `turn off`/`turn on` (or o) unload
  and load the driver and power the chip, as Kobo does; s scans, f forgets.
  Turning it on starts wpa_supplicant and the DHCP client the way Kobo ran
  them, as seen while Wi-Fi was on (kept in `wifi-recipe`).
- Bluetooth: `scan` (s) looks for 40 s; a new device pairs (with a code to
  confirm, or to type on a keyboard, shown in the panel), then gets trusted
  and connected. Paired ones connect, disconnect, and are forgotten with f.
  Pairing goes through hellobro's own agent while Nickel is frozen.
  Kobo turns the Bluetooth chip itself off, not only the adapter; turning
  it on powers the chip (rfkill), attaches it (`rtk_hciattach`, the
  Realtek UART chip), and starts bluetoothd and bluealsa if they aren't
  running, as [bluetooth.koplugin](https://github.com/Tharavol/bluetooth.koplugin)
  does on the Sage; off undoes it.
- Tab moves between the panels: Wi-Fi, Bluetooth, and the settings,
  where a tap (or ←/→, Enter) changes one. They're kept in `settings`.
- When hellobro quits, the radios' power goes back to how Kobo had it,
  since Nickel wakes up thinking so; networks and devices added stay. On
  the first visit to the tab, what's found of the setup (Kobo's variables,
  processes, modules, rfkill) goes to `hellobro.log`.

E-ink notes: changed cells only are redrawn; small black & white changes use
the fast DU waveform and get a GL16 cleanup within a few seconds, bigger ones
use GL16, and after a few screenfuls of change the terminal gets one GC16
flash. Updates never overlap a refresh still in flight (on sunxi that mixes
both up).

## Sharing with a computer

With file sharing on (Settings, or `o` on the Files tab), hellobro is an
SSH server on port 2022, and with mDNS on the Kobo is `kobo.local` and
shows up as an SFTP server in a file manager's Network. Only paired
computers get in, with their key; there are no passwords:

1. Pair: `pair` on the Files tab (or Settings, `p`) shows a code for 5
   minutes. On the computer, `ssh -p 2022 kobo.local`: once the computer
   has shown it holds its key, it asks for the code, and that key is in.
   (No key yet? `ssh-keygen` makes one.) Check the host key fingerprint
   against the one the Files tab shows, the first time.
2. Then, from that computer:
   - a file manager: `sftp://kobo.local:2022/mnt/onboard` (Files on
     Fedora: Other Locations, or Ctrl+L);
   - `scp -P 2022 -r "Some Album" kobo.local:Music/` (relative paths are
     under `/mnt/onboard`), `sftp -P 2022 kobo.local`;
   - `sshfs -p 2022 kobo.local:/mnt/onboard ~/kobo`, to mount it;
   - `ssh -p 2022 kobo.local`, a shell on the Kobo (as root: Kobo has no
     other user), or a command.

Music sent over is found without leaving: the Music tab looks again once
a computer that sent files leaves. Paired keys are in
`.adds/hellobro/authorized_keys` (`forget paired` empties it), the Kobo's
host key in `ssh_host_ed25519_key` there. The server only runs while
hellobro does, with sharing on, and Wi-Fi up: light sleep turns Wi-Fi off
(Settings), so it doesn't come while a computer is connected, nor does
the auto-exit.

**Updating over the air**: `make push` (with this computer paired)
sends the new build as `hellobro.new` and asks hellobro to take it
(`hellobro-update`). It checks it's an ARM build that runs and says it's
hellobro, keeps the running one as `hellobro.old`, and restarts into the
new one: the same process, so Nickel stays frozen. `make push-rollback`
goes back to `hellobro.old`, `make push-log` shows the log. A
`hellobro.new` sent any other way gets a question on the screen instead.
`make install` (USB) is still how `launch.sh`, `bin/fbink` and the menu
entry get there. `KOBO_HOST=192.168.x.y` if `kobo.local` doesn't resolve.

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
