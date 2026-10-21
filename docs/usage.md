# Using hellobro

Back to the [README](../README.md). From afar: [relay.md](relay.md).

## Tabs

Tap a tab, or Ctrl+Alt+1–5 (works everywhere, fullscreen and in a
session too):

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
  settings: light sleep, sleep with music on, file sharing, the relay,
  mDNS, the status bar.
- **Files**: sharing with a computer over SFTP and SSH (how to reach the
  Kobo, pairing, who's connected and sending what), and the Kobo's
  storage: folders to open, files and folders to delete.

## The switcher

Ctrl+Shift+Tab, anywhere, is a switcher in the manner of rofi: the
sessions, the tabs, the saved hosts (a new session to one) and a new
shell, narrowed by typing (fuzzy); Up/Down and Enter, or 1–9 before
typing.

## Sleep

The power button, Ctrl+Alt+S, the header's `sleep`, or "sleep"
in the switcher put the Kobo to sleep with hellobro still up, as Nickel would (a sleep
screen, the light off, suspend to RAM); the power button wakes it, and
it's all as it was. The music stops where it was (Bluetooth goes off),
Wi-Fi and Bluetooth go off and come back on waking, and the SSH sessions
reconnect once Wi-Fi is back. Nickel holds the power button's input
device (frozen, all the while hellobro is up), so hellobro reads the
button's GPIO instead (`/sys/kernel/debug/gpio`); the presses still queue
up for Nickel, which may see them when hellobro quits (and go to sleep
once).

## Light sleep

Light sleep is for listening: the light goes off and the screen shows
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

## Terminal

- `ssh.conf` next to hellobro lists saved hosts, one per line:
  `name  user@host[:port]  [via user@jumphost[:port]]  [command]`; the
  command (say `tmux new -A -s kobo`) runs instead of a login shell, and
  `via` goes through a jump host, as `ssh -J` does ([relay.md](relay.md)). The
  prompt also takes `ssh -p PORT [-J user@jumphost] user@host [command]`,
  and a host typed there is added to `ssh.conf` once it connects.
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

## Updating over the air

`make push` (with this computer paired)
sends the new build as `hellobro.new` and asks hellobro to take it
(`hellobro-update`). It checks it's an ARM build that runs and says it's
hellobro, keeps the running one as `hellobro.old`, and restarts into the
new one: the same process, so Nickel stays frozen. `make push-rollback`
goes back to `hellobro.old`, `make push-log` shows the log. A
`hellobro.new` sent any other way gets a question on the screen instead.
A new build is on trial till it's going (its first tick, and file
sharing up if it's on): one that hangs for 45 s, or crashes, is put
aside as `hellobro.bad` and `hellobro.old` comes back, by itself, so a
bad push from afar doesn't leave the Kobo out of reach. `make push`
sends `launch.sh` too (the crash half is there); `make install` (USB) is
still how `bin/fbink` and the menu entry get there. `KOBO_HOST=192.168.x.y`
if `kobo.local` doesn't resolve.

## Bro Tools

brotools ("Bro Tools") is diagnostics. Tabs (tap, or Ctrl+Alt+1–7):

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
It's only installed when asked for: `make install-tools`.

## In both apps

In both: swipe the left edge for the frontlight. Fullscreen (F11): swipe
in from the top or bottom edge to leave it. Esc or the exit button leave;
the power button sleeps; they also leave after 3 minutes idle (hellobro: 20
with a session up, 6 hours while music plays).

## E-ink

Changed cells only are redrawn; small black & white changes use
the fast DU waveform and get a GL16 cleanup within a few seconds, bigger ones
use GL16, and after a few screenfuls of change the terminal gets one GC16
flash. Updates never overlap a refresh still in flight (on sunxi that mixes
both up).

## Files on the Kobo

Everything hellobro keeps is next to it, in `.adds/hellobro/` on the
Kobo's storage:

| File | What it is |
| --- | --- |
| `ssh.conf` | saved SSH hosts |
| `relay.conf` | where the relay goes ([relay.md](relay.md)) |
| `id_ed25519`, `id_ed25519.pub` | the Kobo's own key, for SSH and the relay |
| `known_hosts` | host keys trusted on first use |
| `authorized_keys` | computers paired for file sharing |
| `ssh_host_ed25519_key` | the Kobo's host key, for file sharing |
| `settings`, `status-bar`, `term-font` | settings |
| `music-state` | volume, shuffle, repeat, what was playing |
| `wifi-recipe` | how Kobo starts Wi-Fi, as seen |
| `touch-calibration.conf` | the touch calibration (shared with brotools) |
| `hellobro.log` | the log (`make push-log`) |
| `hellobro.old`, `hellobro.bad` | the build before an update, a build that failed its trial |
