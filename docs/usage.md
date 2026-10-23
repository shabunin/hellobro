# Using hellobro

Back to the [README](../README.md). From afar: [relay.md](relay.md).

It starts on a "hello bro" screen while it gets going, and says "bye
bro!" as it hands the Kobo back; both have its version and where it's
from (github.com/shabunin/hellobro) at the foot.

## Windows

hellobro is a tiling window manager, after [i3](https://i3wm.org): the
screen is split between windows, side by side or one above another,
tabbed or stacked, or floating over them, on workspaces. A window is:

- **a terminal**: an SSH session (an xterm-256color terminal: tmux,
  helix, htop, Claude Code…, mouse taps and scrolling), a shell on the
  Kobo itself (`/bin/sh`, as root, in a PTY), or Alpine Linux in a jail
  ([The Alpine shell](#the-alpine-shell)). A new one is a **launcher**
  till it runs something: **1** Alpine, **2** the Kobo's sh, **3** and
  on the saved hosts, or `user@host` typed in ([Terminal](#terminal)).
  A saved host's files can be mounted too ([Host files](#host-files)).
- **a settings' window**, one of each, opened from a launcher's
  "Settings, in a window" row or the switcher:
  - **Wi-Fi** and **Bluetooth**, without going back to Nickel: on and
    off, networks (join, with a password; leave; forget; one by name),
    and devices (scan, pair, connect, disconnect, forget);
  - **Sharing** with a computer over SFTP and SSH (how to reach the
    Kobo, pairing, who's connected and sending what);
  - **Settings**, hellobro's own, in groups: Light; Sleep; Keyboard
    (the Bluetooth window when it's gone); At start (Nickel, the radios); Sharing (file sharing, the relay, mDNS,
    CPU profiles); Terminal (notifications, the status bar); Alpine;
    Keys;
  - **Battery**: the charge, the current it gives (mA, W), the gauge's
    time left, temperature, how fast it went down lately (measured from
    its history, a sample a minute while hellobro runs, kept in
    `battery-history`), the processes that keep the CPU busy (percent
    of one core, the busiest five), and the last 12 hours on a graph;
  - **What uses it**, a powertop of sorts, every 5 s: how busy the CPU
    is, at which speeds and how deep it sleeps; the wakeups a second and
    whose; Wi-Fi, Bluetooth, the light, sharing and the relay; and the
    processes that keep the CPU busy (percent of one core).

A window's title strip has its name (white on black when it has the
focus), a **+** for a launcher next to it, and at its end **float**
(floats it over the others; floating, it's **tile**, which puts it
back), **fullscreen**, and **close** (a terminal's shell or connection
ends with it). In a narrow window they're only their icons. A tap on a window gives it the focus; the keys go to the
one that has it. Full screen, a swipe in from the top (or bottom) edge
of the screen, or Meta+F, puts it back.

New windows open next to the one in focus, along its container: side
by side, or one above another after Meta+V (split vertically; Meta+B,
horizontally, side by side). Meta+E turns a container round, Meta+W
tabs it (a row of tabs, one window shown), Meta+S stacks it (a title
each, one above another, one window shown). Meta+F gives the window the
whole screen (again, or a swipe in from the top or bottom edge, gives it
back); Meta+Shift+Space (or its strip's float) floats it over the others (drag
it by its strip; a floating window is put back with the same key, or
tile).

**Workspaces** are in the bar at the top, numbered, each with its
windows' titles after its number (`1 Alpine · Wi-Fi · Battery`), the one
shown in white: Meta+1…Meta+0 go to one (made as you go there; an empty
one goes once you leave it), Meta+Shift+1…0 send the window in focus to
one, and a tap on one goes there. An empty workspace lists the keys (as
they are, keys.conf's too), and a tap on it opens a window.

Meta+Shift+Minus puts the window away (i3's scratchpad), Meta+Minus
brings it back, floating, and puts it away again.

The windows are kept as they're left, in `hellobro.json`, and come back
when hellobro starts (an update's restart too): the workspaces, the
layouts, the floating windows' places, the settings' windows, and the
terminals, which start again: a shell anew, an SSH session connecting to
its host. A launcher, or a terminal running what was typed in, comes
back as a launcher.

## The bar and the keys' focus

One bar at the top, as i3's: the workspaces at its left, and at its
right where the Kobo is on the network, the Wi-Fi's name and its address
(`HomeNet · 192.168.1.20`; or `no network`, `Wi-Fi off`), the battery
and the time. It looks again every 15 s. In a binding mode (Meta+R,
say) it says which (`grab` too, while the keys are grabbed), and what plays, and for a moment what was copied.

A swipe up or down along the right edge of the screen opens the menu:
rotate the screen, touch calibration, sleep, lock, the switcher, and
quit, back to Nickel. Tap one, or type to narrow and Enter; Esc closes
it.

Meta and the arrows (or h j k l, as vim has them) move the focus
between the windows; with Shift, the window itself moves. Meta+A
focuses the container around the window (its whole tabbed group, say,
to move it all), Meta+Tab and Meta+Shift+Tab the next and the previous
window on the workspace.

Meta+R is the resize mode: the arrows (or h j k l) make the window in
focus wider or narrower, taller or shorter; Enter, Esc or Meta+R again
ends it. In a mode only its keys count.

In a settings' window, Tab moves the keys' focus through its controls,
as on a desktop: the buttons and the lists; Shift+Tab goes back. A
dashed frame shows where it is. Enter or Space presses the button in
focus; a list takes ↑ ↓, PgUp/PgDn, Home/End and Enter. The frame shows
once Tab is used, and a touch takes it away. The terminals keep Tab for
the shell.

A question over the page (a saved host's menu, read-only or
read-write, restart into a new build) has its answers as buttons: the
arrows or Tab move between them, Enter picks the one in black, a
button's letter picks it, Esc closes the question.

## The switcher

Meta+D or Meta+Space, anywhere, is a switcher in the manner of rofi (or
dmenu, which i3 has there): the windows (and where they are), the
workspaces, the saved hosts (a new terminal to one), a new Alpine or
Kobo shell, the settings' windows, and the commands, each with its
keys: everything hellobro does on a key, a button or a window's letter.
Among them: the window manager's (focus, move, layouts, fullscreen,
floating); disconnecting the terminal in focus; the player's play,
pause, stop, tracks and volume; Wi-Fi or Bluetooth on and off, looking
for networks or devices, joining a network by name, its details;
sharing, pairing, the relay; sleep, the light, the screen's way
round; and every setting ("Setting: …", with its value now: Enter steps
it, as a tap on its row does). Typing narrows it (fuzzy); Up/Down and
Enter, or 1–9 before typing.

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

The bar says what was copied or pasted, for a moment.

## Keys

The keys are hellobro's commands', i3's with Meta (the Win key). Settings,
Keyboard, "Mod key" makes it Alt instead, as i3's `set $mod Mod1`: then
Alt+Enter opens a window, and the Win key does what Alt did. The hints
say which; keys.conf still says meta for the Mod key either way.

| Key | |
| --- | --- |
| Meta+Enter | a new window: a launcher |
| Meta+D, Meta+Space | the switcher |
| Meta+Shift+Q | close the window in focus |
| Meta+arrows, Meta+H/J/K/L | focus the window there |
| Meta+Shift+arrows, Meta+Shift+H/J/K/L | move the window there |
| Meta+Tab, Meta+Shift+Tab | the next, the previous window |
| Meta+1…0, Meta+Shift+1…0 | go to a workspace, send the window there |
| Meta+B, Meta+V | split: the next window beside, below |
| Meta+E, Meta+W, Meta+S | side by side ⇄ one above another, tabbed, stacked |
| Meta+F | fullscreen (or its strip's fullscreen; a swipe in from the top edge leaves it) |
| Meta+Shift+Space | floating |
| Meta+A | focus the container around |
| Meta+Shift+Minus, Meta+Minus | put the window away, bring it back |
| Meta+R | resize mode: arrows, then Enter or Esc |
| Meta+Shift+C | read keys.conf again |
| Meta+Shift+E | quit, back to Nickel (i3's exit) |

To change one: Settings, the Keys row (or "keys" in the switcher),
lists the commands with their keys. Pick one and press its new key, with
Ctrl, Alt or Meta (an F-key alone will do): it takes the place of the
ones it had, and a command that had that key lets it go. Backspace
leaves it no key, Delete puts its defaults back, Esc leaves it be.

It all goes into `keys.conf`, which can be edited too. It's written the
first time hellobro runs, every command in it with its default keys,
commented out (a `keys.conf` of an older hellobro with nothing but its
defaults is written again). A line is a key and a command, hellobro's
or any of [i3's](https://i3wm.org/docs/userguide.html#list_of_commands),
as i3's `bindsym`:

```
meta+t       layout tabbed; focus parent  # i3's, any line of them
f1           workspace number 1
f2           workspace number 2
meta+o       [class="hellobro" instance="wifi"] focus
meta+x       exec htop                    # in a new Alpine window
ctrl+shift+n launcher
f12          -                            # not ours: the terminal gets F12
none         focus prev                   # no key at all

[resize]                                  # a binding mode: mode resize
x            mode default
```

A command named there loses its default keys (in its mode); the others
keep theirs. `KEY -` frees a key, so it goes on to the terminal (or the
window). `[name]` starts a binding mode's keys, `[default]` goes back:
`mode NAME` enters one, and in it only its keys count. Keys are a-z,
0-9, f1-f12, tab, space, enter, esc, backspace, up, down, left, right,
pgup, pgdn, home, end, insert, delete, the marks (`- = [ ] ; ' , . /`,
and `minus` for `-`), brightnessup and brightnessdown (a keyboard's
light keys), and any other by its Linux code (`key183`), with any of
ctrl, shift, alt and meta. Saved (over SFTP, say), it takes effect at
once; the log says what it didn't understand. The workspaces' corners,
the hints and an empty workspace show the keys as they are. The ids of an older
hellobro that have a match now are read as it (`tab.1` as `workspace
number 1`, `session.new` as `launcher`).

## i3's IPC

hellobro speaks i3's IPC, so what's written for i3 works: `i3-msg`,
`swaymsg`, i3ipc's Python and its scripts, i3blocks-like loops. Every
Alpine shell has the socket at `/run/hellobro/ipc.sock`, with `I3SOCK`
and `SWAYSOCK` set to it, and `hellobro-msg`, which works as `i3-msg`:

```sh
hellobro-msg workspace 2
hellobro-msg '[instance="launcher"] kill'
hellobro-msg -t get_tree | jq '..|.name? // empty'
hellobro-msg -t subscribe -m '["window"]'   # what the windows do, as it happens
hellobro-msg notify "the build is done"     # a note in the corner
apk add i3wm && i3-msg -t get_workspaces    # i3's own, if you like
```

`notify TEXT` is hellobro's, not i3's: a note in the corner, and the
light a moment if it's off, as a terminal's notification (Settings,
Terminal: off, the light, the note's seconds). From a script, a cron
job, mpd's song change: `make && hellobro-msg notify "make: done"`.
It's a command like any: `meta+n notify hi bro` in keys.conf works too.

Commands, the tree, workspaces, outputs, marks, the version, binding
modes and state, `keys.conf` as the config, ticks and sync; and the
events: workspace, window, mode, binding, tick and shutdown. A window's
class is `terminal` or `hellobro`, its instance `launcher`, `alpine`,
`sh`, `ssh` or the settings' window (`wifi`, `bluetooth`, `sharing`,
`settings`, `battery`, `usage`).

What comes over the socket is a jail's, so it never runs as root on the
Kobo: `exec` opens a new Alpine window, and of hellobro's own commands
only what can't reach out of the jail runs (the music, the light, the
font, the screen's way round, sleep, opening a launcher, an Alpine
window or a settings' window, a notification); the Kobo's shell, SSH, the clipboard,
sharing, the radios, the settings and `exit` are refused. On the Kobo
itself (root), the socket is `/tmp/hellobro-ipc/ipc.sock`.

## The light

A swipe up or down the left edge sets it (and keys: see `keys.conf`).
When nothing's touched or typed for 30 s, the light goes off by itself,
and the next touch or key brings it back as it was; that touch or key
does what it does as well. The terminal's output doesn't count, only
you. Settings, **Light off when idle**: after 10 s, 30 s, 1 min, 5 min,
or never. Sleeping meanwhile wakes with the light as it was before.

hellobro starts with the light as you last set it (remembered in
`hellobro.json`); until you set it once, it takes Nickel's. Settings,
**Light at start**: as you last set it, or as Nickel had it. Leaving
gives Nickel its own level back.

## Sleep

The power button, Shift+Meta+S, the menu (a swipe along the right edge), or "Sleep"
in the switcher put the Kobo to sleep with hellobro still up, as Nickel would (a sleep
screen, the light off, suspend to RAM); the power button wakes it, and
it's all as it was. A player that plays is paused first (Bluetooth goes off),
Wi-Fi and Bluetooth go off and come back on waking, and the SSH sessions
reconnect once Wi-Fi is back. Nickel holds the power button's input
device (frozen, all the while hellobro is up), so hellobro reads the
button's GPIO instead (`/sys/kernel/debug/gpio`); the presses still queue
up for Nickel, which may see them when hellobro quits (and go to sleep
once).

It also sleeps by itself after a while untouched (Settings, Sleep): on
battery after 30 minutes by default, plugged in never, each one 5 min to
2 hours or never. Whatever runs goes down with it: SSH sessions, the
relay, a copy to or from the Kobo. A toast says so a minute before, and
a touch or a key keeps it awake. Music doesn't: it's paused, as for the
power button. The while starts again when it's plugged in or out
(unplugged after a night on power, it sleeps after the battery's while
from then, not at once). hellobro never quits by itself: sleep is what
an idle Kobo gets. Left awake untouched, the Kobo lasts about 12 hours
from full.

At 10% on battery a toast says to plug it in (once on the way down),
and the lock screen says so too. Nothing else happens: it doesn't
sleep or quit for it.

## Lock

Shift+Meta+X (or Lock in the menu) locks the screen, for a bag or a hand: touch does nothing
and keys go nowhere, but the music's keys, the power button (sleep),
and Shift+Meta+X again, which unlocks. The light goes off till then
(notifications don't flash it). The lock screen says hello bro, what
plays and whether it's paused, a low battery, how to unlock, the
version and where it's from; it changes only when one of them does. The windows go on behind it, as they are when it
unlocks; asleep and awake, it's still locked. Locked, touches and
keys don't count as use: the light stays off and sleeping when idle
still comes. With no keyboard (or the keyboard gone while it's locked),
the lock screen lists swipes instead, the touch screen's way out: ↑ unlocks, → unlocks and opens Bluetooth full screen
(to connect the keyboard again), ↓ goes back to Nickel. A swipe is long (a quarter
of the screen's short side) and straight, so a brush in a bag isn't
one; the keyboard back, they're gone. keys.conf: `lock`.

Unlocked, a keyboard gone for 15 s brings up the Bluetooth window, full
screen, to connect it again; when it's back the window goes (or, if it
was open already, goes back to its place). A keyboard back within the
15 s, napping, say, shows nothing, and asleep or just awake it waits:
Bluetooth comes back on with the keyboard. Settings, **The keyboard
gone**: after 15 s, 30 s or 1 min, or nothing comes up.

## Terminal

- A launcher (Meta+Enter, or a window's +) asks what to run: **1**
  Alpine, **2** the Kobo's sh (Enter alone: Alpine), **3** and on a
  saved host, or a host typed in. Esc closes it.
- Type `user@host[:port] [command]` and Enter; the prompt also takes
  `ssh -p PORT [-J user@jumphost] user@host [command]`. A host typed
  there is saved once it connects (in `hellobro.json`), and listed under
  the prompt.
- A tap on a saved host, or its number, opens its menu: **connect**
  (Enter), **mount its files** ([Host files](#host-files)), **edit**,
  **forget**. Edit puts its line in the prompt, to change and save with
  Enter: `name  user@host[:port]  [via user@jumphost[:port]]
  [command]`; the command (say `tmux new -A -s kobo`) runs instead of a
  login shell, and `via` goes through a jump host, as `ssh -J` does
  ([relay.md](relay.md)). An `ssh.conf` next to hellobro, with lines
  like that, is taken in at start (the names it doesn't have yet) and
  kept as `ssh.conf.old`.
- The Kobo's key is made on first use; `id_ed25519.pub` next to the binary
  goes in a server's `~/.ssh/authorized_keys`. Otherwise it asks for a
  password (Tab shows it while typing), and then offers to install the key
  there itself. Host keys are trusted on first use and kept in `known_hosts`.
- Keepalives every 15 s notice a dead connection; a dropped one (not one the
  remote closed) is made again, after 2, 4, 8… up to 30 s, with the password
  typed earlier (kept in memory only). Esc stops it.
- Each terminal is a window, with its own size: the remote (or the
  shell) is told when it changes. The title strip's `disconnect` or
  `kill` ends what runs, and the window is a launcher again; close closes it.
- Ctrl+= / Ctrl+- / Ctrl+0: font size, all the terminals' (remembered
  in `hellobro.json`).
- A status bar along the bottom, under the windows, fullscreen too: what's playing, and the
  battery (charge, temperature), CPU and memory use, every 5 s.
  Ctrl+Shift+B hides or shows it (remembered in `status-bar`); a tap on it
  plays or pauses.
- While connected, every other key goes to the remote, Esc included, but
  for hellobro's own (the commands' keys: Meta+Shift+E quits, say). A
  program there that wants one of them gets it if keys.conf frees it:
  `ctrl+shift+v -`.
- **Grab** (Meta+Shift+G) gives every key to the window in focus,
  hellobro's own too, till Meta+Shift+G again: for tmux, a desktop or
  another hellobro on the other end, with keys of their own. The bar
  says `grab` while it's on.
- A program that asks for kitty's keyboard protocol (`CSI > flags u`:
  kitty's own, foot's, neovim, helix, hellobro-keys…) gets the keys as
  kitty says them: the modifiers apart, Super among them, and releases
  and repeats if it wants them. A modifier tapped for one of hellobro's
  keys reaches it too, as in kitty; the key itself doesn't.
- A session calls out with the bell, or with a message as programs send a
  desktop notification (OSC 9 or 777: `make; printf '\e]9;make done\a'`).
  The light comes on a moment if it's off; the session's title strip
  blinks if it shows, and if not, a note in the lower right corner says
  which and what (`Alpine: make done`). A bell within 2 s of a key typed
  into it doesn't count (that's the shell's answer to the key), and a
  session calls out at most every 5 s. Settings, **Terminal
  notifications** and the three under it: the bell too or messages only,
  how long the light stays on, how often the strip blinks, how long the
  note stays (each can be off).

## Host files

A saved host's menu (in a launcher) has **mount its files**: it asks
**read-only** or **read-write** each time, logs in as connecting does
(the Kobo's key, or a password asked on the page), and the host's home
is then a folder:

- `/mnt/hosts/NAME` in the Alpine sessions ([The Alpine shell](#the-alpine-shell)),
  NAME being the host's;
- `/tmp/hellobro-hosts/NAME` on the Kobo itself (the Kobo shell).

```sh
ls /mnt/hosts/pc/Music
cp -r /mnt/hosts/pc/books/new /mnt/onboard/books/
rsync -av --progress /mnt/hosts/pc/Music/Heroes/ /mnt/onboard/Music/Heroes/
mpv /mnt/hosts/pc/Music/song.flac      # plays straight from the host
```

The host needs nothing but its sshd (SFTP). Files show as root's, as
you are in Alpine; what may be done is the host user's to say.
Read-only, nothing can change on the host. The saved hosts list says
which are mounted; the menu unmounts.

The mount is hellobro's (a process of its own, `hellobro --mount`), as
the jail can't mount; it goes away before the Kobo sleeps and comes back
on waking, and goes when hellobro does. If the host goes
quiet (keepalives, as for sessions), it's unmounted, and says so: what
was using it gets an error, not a hang. Sessions started before an
update see `/mnt/hosts` from their next start.

## The Alpine shell

A launcher's **1** (or Enter) is **Alpine Linux, in a jail**: a small
Linux of its own, with its own tools and `apk add` for more (git,
python3, vim, tmux…), kept away from the Kobo's system. (**2** is the
Kobo's own `/bin/sh`, as root.)

- The first Alpine session sets it up, in the session: a disk image
  (`.adds/hellobro/alpine.img`, ext2), as big as you say there, in MB
  (Enter: 1024). It's made once and doesn't grow: 256 MB at least, and
  at most 4095 MB, as FAT's files stop at 4 GB, with 256 MB left free
  on the storage. Then Alpine's minirootfs
  (3.24.3, 3 MB, needs Wi-Fi). It's fetched over plain HTTP, as the Kobo
  has no CA certificates, and checked against the SHA-256 pinned in
  `alpine.go`; inside, apk goes over HTTPS with Alpine's own.
- In the jail you're root, as the storage is FAT and only root writes
  there, but root over the jail only:
  - `/mnt/onboard` is the Kobo's storage, writable, but its `.adds` and
    `.kobo` are read-only: what's there runs as root outside (the
    launcher, NickelMenu, `KoboRoot.tgz`).
  - `/mnt/hosts` has the hosts mounted from a launcher ([Host
    files](#host-files)); it's read-only itself, the mounts in it are as
    they were mounted.
  - Its own processes only (no Nickel, no hellobro), its own `/dev` (no
    screen, keys, frontlight or disks), its own `/tmp`; `/sys` and
    `/proc/sys` read-only.
  - No mount, modules, raw devices, mknod, reboot, ptrace, clock or
    network setup, and no way to get them back (no_new_privs). The
    network itself works.
  - The window manager's socket, `/run/hellobro/ipc.sock`, and
    `hellobro-msg` ([i3's IPC](#i3s-ipc)).
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
  headphones at a time.
- Each session is a jail of its own on the same image: when its shell
  exits, everything it started goes too (tmux's server included).
- From a computer: `ssh -t -p 2022 kobo.local hellobro-alpine`, or
  `… hellobro-alpine COMMAND`. A plain `ssh kobo.local` is still the
  Kobo's root shell (`make push` needs it).
- The image is mounted at `/tmp/hellobro-alpine` while hellobro runs and
  let go of when it quits, so USB still works. ext2 has no journal: after
  a crash, check it on a computer (`e2fsck -f alpine.img`, copied over
  with hellobro not running).
- Settings, **… Alpine: remove it** (or "Alpine: remove it…" in the
  switcher) deletes the image and all in it, once you type `yes delete
  it bro` in the question (a keyboard is needed). The Alpine sessions
  have to be closed first. The next Alpine window starts over, asking
  the size anew.

## Music

hellobro has no player of its own: the music is a Linux player's, in the
Alpine shell, and hellobro is its remote control, over MPRIS (the D-Bus
interface players have for that). In an Alpine shell:

```sh
apk add dbus mpv mpv-mpris     # or cmus, or mpd and mpd-mpris
```

Then a new Alpine shell has a session bus (`/run/user/0/bus`, in
`DBUS_SESSION_BUS_ADDRESS`), and a player started in it is seen there.
Sound needs "Alpine: system bus access" on ([The Alpine
shell](#the-alpine-shell)); mplayer has no MPRIS.

- Anywhere: Ctrl+Shift+Space plays or pauses, Ctrl+Shift+< and > go to
  the previous and next track; so do media keys, and the volume keys (a
  headset's, a keyboard's) set the player's volume. A tap on the status
  bar plays or pauses. They go to the player that's playing, or else one
  that's paused.
- What it plays shows in the bar and the status bar, and in light
  sleep. While it plays, hellobro doesn't quit by itself.
- Headphones' buttons work only when BlueZ sends them as keys, which it
  does while no media player is registered with it. On the Kobo, Nickel
  has one registered, so BlueZ sends the presses there as D-Bus calls,
  and Nickel, frozen, never answers them; the Elipsa's D-Bus (1.6.30)
  has no monitor mode and doesn't allow eavesdropping, so they're lost.
  Settings, "Nickel while hellobro runs", closed: hellobro closes Nickel
  as it starts, as KOReader does, and Nickel's player goes with it, so
  the buttons come as keys ([Closing Nickel](#closing-nickel)).
- Each Alpine shell has its own bus, and its players go with it: closing
  the shell stops the music.
- Music sent over SFTP (`scp song.mp3 kobo.local:Music/`) is in
  `/mnt/onboard/Music` in the jail.

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
  confirm, or to type on a keyboard, shown in the window), then gets trusted
  and connected. Paired ones connect, disconnect, and are forgotten with f.
  Pairing goes through hellobro's own agent while Nickel is frozen.
  Kobo turns the Bluetooth chip itself off, not only the adapter; turning
  it on powers the chip (rfkill), attaches it (`rtk_hciattach`, the
  Realtek UART chip), and starts bluetoothd and bluealsa if they aren't
  running, as [bluetooth.koplugin](https://github.com/Tharavol/bluetooth.koplugin)
  does on the Sage; off undoes it.
- The Settings window: a tap (or ←/→, Enter) changes one; ↑ ↓ and
  PgUp/PgDn go through them. They're kept in `settings`.
- When hellobro quits, the radios' power goes back to how Kobo had it,
  since Nickel wakes up thinking so; networks and devices added stay.
  "Wi-Fi and Bluetooth at start", turned on, has hellobro turn both on
  as it starts and leave them on when it quits (or restarts into a new
  build). Nickel may still turn them off soon after, if you were in
  hellobro a while: its idle timers run out while it's frozen. On
  the first opening of their windows, what's found of the setup (Kobo's variables,
  processes, modules, rfkill) goes to `hellobro.log`.

## Sharing with a computer

With file sharing on (Settings: **Sharing**, `o` or `turn on`), hellobro is an
SSH server on port 2022, and with mDNS on the Kobo is `kobo.local` and
shows up as an SFTP server in a file manager's Network. Only paired
computers get in, with their key; there are no passwords:

1. Pair: `pair` in Settings, Sharing (or `p`) shows a code for 5
   minutes. On the computer, `ssh -p 2022 kobo.local`: once the computer
   has shown it holds its key, it asks for the code, and that key is in.
   (No key yet? `ssh-keygen` makes one.) Check the host key fingerprint
   against the one Sharing shows, the first time.
2. Then, from that computer:
   - a file manager: `sftp://kobo.local:2022/mnt/onboard` (Files on
     Fedora: Other Locations, or Ctrl+L);
   - `scp -P 2022 -r "Some Album" kobo.local:Music/` (relative paths are
     under `/mnt/onboard`), `sftp -P 2022 kobo.local`;
   - `sshfs -p 2022 kobo.local:/mnt/onboard ~/kobo`, to mount it;
   - `ssh -p 2022 kobo.local`, a shell on the Kobo (as root: Kobo has no
     other user), or a command.

Paired keys are in
`.adds/hellobro/authorized_keys` (`forget paired` empties it), the Kobo's
host key in `ssh_host_ed25519_key` there. A key whose line starts with
OpenSSH's `command="…"` runs that and nothing else, whatever it asks
for (no SFTP either): `command="hellobro-keys" ssh-ed25519 AAAA… kobo`,
a key that may only type. The server only runs while
hellobro does, with sharing on, and Wi-Fi up. Sleeping when idle comes
even with a computer connected.

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
Meta+] / Meta+[, and Meta+\ for on/off; in brotools Ctrl+Alt+Up/Down
and Ctrl+Alt+L). Fullscreen (Meta+F in
hellobro): swipe in from the top or bottom edge to leave it. The exit
button leaves, and so does Esc in brotools, Meta+Shift+E in hellobro (where Esc
is the terminal's, or closes what's open);
the power button sleeps. brotools also leaves after 3 minutes idle;
hellobro doesn't, it sleeps instead (Sleep).

## Closing Nickel

Settings, "Nickel while hellobro runs": frozen (the default) or closed.
Closed, from the next start, `launch.sh` ends Nickel and its helpers as
KOReader does (after a `sync`), and starts it again, as the Kobo boots
it, when hellobro quits. What Nickel registered with BlueZ goes with it,
its media player too, so the headphones' buttons are no longer sent to
it. Wi-Fi carries on; if Bluetooth goes (its chip's link was Nickel's
child), hellobro turns it on again a few seconds in. Quitting turns both
radios off and Nickel turns them on as its settings say, so leaving
takes as long as Nickel takes to start, and Nickel's idle timers start
afresh. Updates (a restart into a new build) keep Nickel closed and the
radios on.

## E-ink

Changed cells only are redrawn; small black & white changes use
the fast DU waveform and get a GL16 cleanup within a few seconds, bigger ones
use GL16, and after a few screenfuls of change the terminal gets one GC16
flash. Updates never overlap a refresh still in flight (on sunxi that mixes
both up).

DU can't show gray, so what it draws is sent rounded to black and white:
crisp, if without smooth edges till the GL16 cleanup puts them in. (Sent
the grays, DU left them faint, and the cleanup, sending the same pixels,
didn't touch them.)

## Files on the Kobo

Everything hellobro keeps is next to it, in `.adds/hellobro/` on the
Kobo's storage:

| File | What it is |
| --- | --- |
| `relay.conf` | where the relay goes ([relay.md](relay.md)) |
| `id_ed25519`, `id_ed25519.pub` | the Kobo's own key, for SSH and the relay |
| `known_hosts` | host keys trusted on first use |
| `authorized_keys` | computers paired for file sharing |
| `ssh_host_ed25519_key` | the Kobo's host key, for file sharing |
| `settings`, `status-bar` | settings |
| `hellobro.json` | the saved SSH hosts, and how it was left: the windows, the light, the font size |
| `ssh.conf.old` | the hosts as kept before, taken in once |
| `keys.conf` | the keys ([Keys](#keys)) |
| `clipboard` | what's copied ([Copy and paste](#copy-and-paste)) |
| `wifi-recipe` | how Kobo starts Wi-Fi, as seen |
| `touch-calibration.conf` | the touch calibration (shared with brotools) |
| `hellobro.log` | the log (`make push-log`) |
| `hellobro.old`, `hellobro.bad` | the build before an update, a build that failed its trial |
