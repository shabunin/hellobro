# Using hellobro

Back to the [README](../README.md). From afar: [relay.md](relay.md).

It starts on a "hello bro" screen while it gets going, and says "bye
bro!" as it hands the Kobo back.

## Tabs

Tap a tab, or Ctrl+Alt+1–5 (by place; works everywhere, fullscreen and
in a session too):

- **SSH**: an SSH client with an xterm-256color terminal (tmux, helix, htop,
  Claude Code…), mouse taps and scrolling; several sessions at once, side
  by side.
- **Shell**: the same terminal with `/bin/sh` on the Kobo itself, in a PTY;
  several too. Hidden at first: turn it on in Settings, Tabs. Or Alpine
  Linux in a jail instead ([The Alpine shell](#the-alpine-shell)).
- **Music**: a player for the music on the Kobo (folders of MP3/WAV, with
  their covers), to Bluetooth headphones or a DAC: by album, by folder
  (subfolders and all) or by playlist, in order or shuffled. It goes on
  playing on the other tabs.
- **Settings**: Wi-Fi and Bluetooth without going back to Nickel: on and
  off, networks (join, with a password; leave; forget; one by name), and
  devices (scan, pair, connect, disconnect, forget); and hellobro's own
  settings: the light off when idle, light sleep, sleep with music on, file sharing, the relay,
  mDNS, terminal notifications, CPU profiles, the status bar, the tabs.
- **Files**: sharing with a computer over SFTP and SSH (how to reach the
  Kobo, pairing, who's connected and sending what), and the Kobo's
  storage: folders to open, files and folders to delete.
- **Power**: a powertop of sorts. **Battery**: the charge, the current
  it gives (mA, W), the gauge's time left, temperature, how fast it
  went down lately (measured from its history, a sample a minute while
  hellobro runs, kept in `battery-history`), and the last 12 hours on a
  graph. **What uses it**, every 5 s: how busy the CPU is, at which
  speeds and how deep it sleeps; the wakeups a second and whose; Wi-Fi,
  Bluetooth, the light, sharing and the relay; and the processes that
  keep the CPU busy (percent of one core).

Which tabs show, and in what order, is up to you: Settings, the last
row, **Tabs** (or "tabs" in the switcher) opens a list of them. A tap on
one, or Space, shows or hides it; the arrows, or Shift+Up/Down, move it.
Settings itself can't be hidden. It's kept in `tabs`
(`ssh -shell music settings files`: a `-` hides one).

## The header and the panels

The header says where the Kobo is on the network: the Wi-Fi's name and
its address (`HomeNet · 192.168.1.20`; or `no network`, `Wi-Fi off`),
then the battery and the time. It looks again every 15 s.

A panel folds: the − at the end of its title strip folds it to the
strip, and the other panels on the page take the room. The + (or a tap
anywhere on the strip) opens it again. From the keys, Ctrl+Shift+M
folds the panel in focus or opens it, and the switcher can too ("Wi-Fi
panel: minimize", "… open"). They stay folded, kept in `folded` (a
session's fold lasts as long as the session).

One panel on a page has the focus, marked ►: the keys go to it.
Ctrl+Tab moves it to the next panel, Ctrl+Shift+Tab back; a tap on a
panel gives it the focus too. A folded one takes the focus as well, on
its strip: there the keys wait, and Enter (or Ctrl+Shift+M) opens it.
Folding the one in focus moves the focus to an open one.

Ctrl+Shift+F gives the panel in focus the whole screen, as a tiling
window manager does, on any tab: Ctrl+Tab then goes on to the next
panel, which has the screen in turn. Ctrl+Shift+F again, or a swipe in
from the top or bottom edge, brings the others back. Like every key
here, these are set in `keys.conf` (or Settings → Keys): `fullscreen`,
`panel.next`, `panel.prev`, `panel.fold`. On Music, Now playing in focus takes
↑ ↓ for the volume and Enter to play or pause; the Library has them
otherwise. On Files, the arrows are the list's.

Tab moves the keys' focus through the page's controls, as on a desktop:
the tab strip, the buttons and the lists, panel by panel; Shift+Tab goes
back. A dashed frame shows where it is. Enter or Space presses the
button in focus; a list takes ↑ ↓, PgUp/PgDn, Home/End and Enter; on
the tab strip ← → change tabs. Ctrl+Tab, as before, goes to the next
panel (its list, or its first button). The frame shows once Tab is used,
and a touch takes it away. The terminals keep Tab for the shell.

## The switcher

Ctrl+Shift+P, anywhere, is a switcher in the manner of rofi (or VS
Code's command palette): the sessions, the tabs, the saved hosts (a new
session to one), a new shell, and the commands (Wi-Fi or Bluetooth on
and off, sharing, pairing, the relay, showing or hiding a tab, light
sleep, the light brighter, dimmer, on and off, the screen to portrait or
landscape…), each with its keys. Typing narrows it (fuzzy);
Up/Down and Enter, or 1–9 before typing.

## Copy and paste

- **Copy** in a terminal: draw the pen across the text, or hold a finger
  still a moment and then move it. What's selected shows reversed, and
  it's copied as you lift. A key or a tap takes the highlight away;
  Ctrl+Shift+C copies it again. "Copy the screen's text" in the switcher
  takes all of it.
- **Paste**: Ctrl+Shift+V. Into a terminal goes all of it (as a paste, so
  editors know it's not typed); into a password or a question, the
  switcher or the `ssh>` line, its first line.
- **The Kobo's SSH key**: "Copy the Kobo's SSH key" and "Paste the
  Kobo's SSH key" in the switcher. Pasting types it in, the clipboard
  left as it was: on a server, `echo ` then paste then
  ` >> ~/.ssh/authorized_keys`.
- A program on the other end that copies (OSC 52: tmux, helix, Claude
  Code, vim with a plugin) copies into the clipboard too. It can't read
  the clipboard back.
- The clipboard is the file `clipboard` next to hellobro, so the
  computer gets at it as well: `ssh -p 2022 kobo.local cat
  /mnt/onboard/.adds/hellobro/clipboard`, or write it over SFTP and
  paste on the Kobo.

The header says what was copied or pasted, for a moment.

## Keys

The keys are hellobro's commands'. To change one: Settings, the last
row, **Keys** (or "keys" in the switcher), lists the commands with
their keys. Pick one and press its new key, with Ctrl, Alt or Meta (an
F-key alone will do): it takes the place of the ones it had, and a
command that had that key lets it go. Backspace leaves it no key,
Delete puts its defaults back, Esc leaves it be.

It all goes into `keys.conf`, which can be edited too. It's written the
first time hellobro runs, every command in it with its default keys,
commented out. A line is a key and a command:

```
ctrl+alt+m   tab.music        # a tab of its own
f1           tab.1            # the first tab, whichever it is
ctrl+shift+n session.new
f12          -                # not ours: the terminal gets F12
none         session.prev     # no key at all
```

A command named there loses its default keys; the others keep theirs.
`KEY -` frees a key, so it goes on to the terminal (or the page). Keys
are a-z, 0-9, f1-f12, tab, space, enter, esc, backspace, up, down,
left, right, pgup, pgdn, home, end, insert, delete, the marks
(`- = [ ] ; ' , . /`, and `minus` for `-`), brightnessup and
brightnessdown (a keyboard's light keys), and any other by its Linux
code (`key183`), with any of ctrl, shift, alt and meta. Saved (over
SFTP, say), it takes effect at once; the log says what it didn't
understand. The tabs' corners, the footer and the hints show the keys as
they are.

## The light

A swipe up or down the left edge sets it (and keys: see `keys.conf`).
When nothing's touched or typed for 30 s, the light goes off by itself,
and the next touch or key brings it back as it was; that touch or key
does what it does as well. The terminal's output doesn't count, only
you. Settings, **Light off when idle**: after 10 s, 30 s, 1 min, 5 min,
or never. Sleeping meanwhile wakes with the light as it was before.

## Sleep

The power button, Ctrl+Alt+S, the header's `sleep`, or "Sleep"
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
awake, so the music plays on and a keyboard's media keys play and pause,
skip, and set the volume. The power button or
any other key wakes it up, to the page as it was; touching the screen
doesn't (unless there's no keyboard and no power button to be had). With music playing it comes
after a while without input (5 minutes; Settings), and Ctrl+Alt+S dozes
instead of sleeping (Settings: or not). Wi-Fi goes off once the light
sleep has lasted 3 minutes (Settings: or not), and comes back on waking.
Not before: Wi-Fi coming back (a scan, joining, DHCP) takes the air from
the music, which skips meanwhile, so a short light sleep leaves it be.
With Bluetooth on, the Wi-Fi chip keeps its
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
- Sessions are panels: each has its own, and the open ones share the
  tab (portrait: one above the other; landscape: two abreast), each with
  its own size. Ctrl+Shift+T or the `+` in the focused one's title adds
  one; Ctrl+Tab and Ctrl+Shift+Tab (or Ctrl+PgUp/PgDn,
  Ctrl+Shift+Left/Right, or a tap on it) move the focus, and the keys,
  to the next and previous one, folded ones too. − or Ctrl+Shift+M folds
  one to its strip; Enter on the strip, or picking it in the switcher,
  opens it again. Esc on an idle one or `close` drops it.
- Ctrl+Shift+F: fullscreen, the session in focus alone (Ctrl+Tab goes
  on to the next). Ctrl+= / Ctrl+- / Ctrl+0, or a swipe along the right
  edge: font size (remembered in `term-font`).
- A status bar along the bottom, fullscreen too: what's playing, and the
  battery (charge, temperature), CPU and memory use, every 5 s.
  Ctrl+Shift+B hides or shows it (remembered in `status-bar`); a tap on it
  plays or pauses.
- While connected, every other key goes to the remote, Esc included, but
  for the music's keys below.
- A session calls out with the bell, or with a message as programs send a
  desktop notification (OSC 9 or 777: `make; printf '\e]9;make done\a'`).
  The light comes on a moment if it's off; the session's title strip
  blinks if it shows, and if not, a note in the lower right corner says
  which and what (`SSH 2: make done`). A bell within 2 s of a key typed
  into it doesn't count (that's the shell's answer to the key), and a
  session calls out at most every 5 s. Settings, **Terminal
  notifications** and the three under it: the bell too or messages only,
  how long the light stays on, how often the strip blinks, how long the
  note stays (each can be off).

## The Alpine shell

Settings, **Shell tab runs**: the Kobo's own `/bin/sh`, as root (the
default), or **Alpine Linux, in a jail**: a small Linux of its own, with
its own tools and `apk add` for more (git, python3, vim, tmux…), kept
away from the Kobo's system.

- The first Alpine session sets it up, in the session: a 1 GB disk image
  (`.adds/hellobro/alpine.img`, ext2), and Alpine's minirootfs
  (3.24.3, 3 MB, needs Wi-Fi). It's fetched over plain HTTP, as the Kobo
  has no CA certificates, and checked against the SHA-256 pinned in
  `alpine.go`; inside, apk goes over HTTPS with Alpine's own.
- In the jail you're root, as the storage is FAT and only root writes
  there, but root over the jail only:
  - `/mnt/onboard` is the Kobo's storage, writable, but its `.adds` and
    `.kobo` are read-only: what's there runs as root outside (the
    launcher, NickelMenu, `KoboRoot.tgz`).
  - Its own processes only (no Nickel, no hellobro), its own `/dev` (no
    screen, keys, frontlight or disks), its own `/tmp`; `/sys` and
    `/proc/sys` read-only.
  - No mount, modules, raw devices, mknod, reboot, ptrace, clock or
    network setup, and no way to get them back (no_new_privs). The
    network itself works.
- System bus access: off by default. Settings, **… Alpine: system bus
  access** gives the jail the Kobo's D-Bus, for what talks to its
  services. Sound is the first of those: the Kobo has no sound card,
  only Bluetooth, through its bluealsa (3.1) on that bus. With the bus
  the jail also gets the bluez-alsa 3.1.0 ALSA plugins
  (from Alpine 3.15, as Alpine's own 4.x doesn't speak to the Kobo's;
  fetched once, pinned hash) and an `/etc/asound.conf` making them the
  default (written if there's none). Then mplayer, mpv, aplay… play to
  the headphones connected last. The price: root on the bus can also
  pair, unpair and connect Bluetooth devices and change the Wi-Fi
  (dhcpcd, wpa_supplicant): settings, not the Kobo's system, but mind it
  if what runs in the jail faces the network. One program has the
  headphones at a time: the Music tab lets go of them while paused, and
  says so if the jail has them.
- Each session is a jail of its own on the same image: when its shell
  exits, everything it started goes too (tmux's server included).
- From a computer: `ssh -t -p 2022 kobo.local hellobro-alpine`, or
  `… hellobro-alpine COMMAND`. A plain `ssh kobo.local` is still the
  Kobo's root shell (`make push` needs it).
- The image is mounted at `/tmp/hellobro-alpine` while hellobro runs and
  let go of when it quits, so USB still works. ext2 has no journal: after
  a crash, check it on a computer (`e2fsck -f alpine.img`, copied over
  with hellobro not running). Removing `alpine.img` starts over.

## Music

- Albums are folders with MP3 or WAV files, up to 5 levels under the Kobo's
  storage (`/mnt/onboard`; hidden folders skipped), say
  `Music/Artist/Album/`. The cover is `cover.jpg`, `folder.jpg` or another
  image there (a "front" one before a "back").
- It plays to the Bluetooth device connected in Kobo's settings; a paired
  one that isn't connected gets a "connect" button.
- On any tab: Ctrl+Shift+Space plays or pauses, Ctrl+Shift+< and > go to
  the previous and next track; so do media keys, and the volume keys (a
  headset's, a keyboard's) set the volume.
- Headphones' buttons work only when BlueZ sends them as keys, which it
  does while no media player is registered with it. On the Kobo, Nickel
  has one registered, so BlueZ sends the presses there as D-Bus calls,
  and Nickel, frozen, never answers them. hellobro tries to watch the bus
  for them, but the Elipsa's D-Bus (1.6.30) has no monitor mode and
  doesn't allow eavesdropping, so on the Elipsa they're lost.
- The library has three views (tap them, or v / 1-3):
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
  a new secured one asks for its password (`show` reveals it: tap it, or
  Tab to it and Enter).
  "other network…" (or n) joins a hidden one by name. The address comes
  from the DHCP client already running (Kobo's), or one started for it. Networks are saved
  when wpa_supplicant's file allows it. `turn off`/`turn on` (or o) unload
  and load the driver and power the chip, as Kobo does; s scans, f forgets.
  Turning it on starts wpa_supplicant and the DHCP client the way Kobo ran
  them, as seen while Wi-Fi was on (kept in `wifi-recipe`). `info` (or i)
  shows the details: the network, access point, band and channel,
  signal, address, gateway, DNS, the interface and its MAC; c copies
  them, Esc closes.
- Bluetooth: `scan` (s) looks for 40 s; a new device pairs (with a code to
  confirm, or to type on a keyboard, shown in the panel), then gets trusted
  and connected. Paired ones connect, disconnect, and are forgotten with f.
  Pairing goes through hellobro's own agent while Nickel is frozen.
  Kobo turns the Bluetooth chip itself off, not only the adapter; turning
  it on powers the chip (rfkill), attaches it (`rtk_hciattach`, the
  Realtek UART chip), and starts bluetoothd and bluealsa if they aren't
  running, as [bluetooth.koplugin](https://github.com/Tharavol/bluetooth.koplugin)
  does on the Sage; off undoes it.
- Ctrl+Tab moves between the panels: Wi-Fi, Bluetooth, and the
  settings, where a tap (or ←/→, Enter) changes one; ↑ ↓ and PgUp/PgDn
  go through them. They're kept in `settings`.
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

From a release (see the README's Install): send its bare `hellobro` as
`hellobro.new`, then `ssh -p 2022 kobo.local hellobro-update`. From the
source, `make push` (with this computer paired)
sends the new build as `hellobro.new` and asks hellobro to take it
(`hellobro-update`). It checks it's an ARM build that runs and says it's
hellobro, keeps the running one as `hellobro.old`, and restarts into the
new one ("hello bro, starting the new hellobro…"): the same process, so
Nickel stays frozen. `make push-rollback`
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

In both: swipe the left edge for the frontlight (in hellobro also
Ctrl+Alt+Up/Down, and Ctrl+Alt+L for on/off). Fullscreen (Ctrl+Shift+F
in hellobro): swipe in from the top or bottom edge to leave it. Esc or the exit button leave;
the power button sleeps; they also leave after 3 minutes idle (hellobro: 20
with a session up, 6 hours while music plays).

## E-ink

Changed cells only are redrawn; small black & white changes use
the fast DU waveform and get a GL16 cleanup within a few seconds, bigger ones
use GL16, and after a few screenfuls of change the terminal gets one GC16
flash. Updates never overlap a refresh still in flight (on sunxi that mixes
both up).

DU can't show gray, so what it draws is sent rounded to black and white:
crisp, if without smooth edges till the GL16 cleanup puts them in. (Sent
the grays, DU left them faint, and the cleanup, sending the same pixels,
didn't touch them.) Settings, "Terminal: clean redraw of typed text", adds
a small flash of what was drawn fast every 10 s, 30 s or 1 min, in SSH and
Shell sessions alike; it's off at first.

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
| `settings`, `status-bar`, `term-font`, `tabs`, `folded` | settings |
| `keys.conf` | the keys ([Keys](#keys)) |
| `clipboard` | what's copied ([Copy and paste](#copy-and-paste)) |
| `music-state` | volume, shuffle, repeat, what was playing |
| `wifi-recipe` | how Kobo starts Wi-Fi, as seen |
| `touch-calibration.conf` | the touch calibration (shared with brotools) |
| `hellobro.log` | the log (`make push-log`) |
| `hellobro.old`, `hellobro.bad` | the build before an update, a build that failed its trial |
