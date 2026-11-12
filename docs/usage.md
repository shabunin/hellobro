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
  till it runs something: tiles for everything hellobro has
  ([The launcher](#the-launcher)); **1** Alpine, **2** the Kobo's sh,
  **3** and on the saved hosts, or `user@host` typed in ([Terminal](#terminal)).
  A saved host's files can be mounted too ([Host files](#host-files)).
- **a settings' window**, one of each, opened from a launcher's
  Settings tiles or the switcher:
  - **Wi-Fi** and **Bluetooth**, without going back to Nickel: on and
    off, networks (join, with a password; leave; forget; one by name),
    and devices (scan, pair, connect, disconnect, forget);
  - **Sharing** with a computer over SFTP and SSH (how to reach the
    Kobo, pairing, who's connected and sending what);
  - **Settings**, hellobro's own, in groups: Light; Power; Keyboard
    (the Bluetooth window when it's gone); Touch (touch marks, how
    workspaces turn by touch); At start (Nickel, the radios); Sharing (file sharing, the relay, mDNS,
    CPU profiles); Terminal (notifications, the bar); Alpine;
    Keys;
  - **Battery**: the charge, the current it gives (mA, W), the gauge's
    time left, temperature, how fast it went down lately (measured from
    its history, a sample a minute while hellobro runs, kept in
    `battery-history`: time, percent, mA, `c` or `d`, mV; the percent
    can drift a long way, the voltage is there to check it by), the processes that keep the CPU busy (percent
    of one core, the busiest five), and the last 12 hours on a graph;
  - **What uses it**, a powertop of sorts, every 5 s: how busy the CPU
    is, at which speeds and how deep it sleeps; the wakeups a second and
    whose; Wi-Fi, Bluetooth, the light, sharing and the relay; and the
    processes that keep the CPU busy (percent of one core).

    Both sample every 5 s while you use them, every 30 s once nothing's
    been touched for a minute (the graph every 5 min), and redraw only
    the lines that changed:
    a sample reads some 150 files and an update powers the panel, and
    left open they were most of what hellobro did when idle.

A window's title strip has its name (white on black when it has the
focus), a **+** for a launcher next to it, and at its end **float**
(floats it over the others; floating, it's **tile**, which puts it
back), **fullscreen**, and **close** (a terminal's shell or connection
ends with it). In a narrow window they're only their icons. A tap on a window gives it the focus; the keys go to the
one that has it. Full screen, two fingers swiped down from the top of
the screen, a finger swiped down from its top edge (even over a PDF
page that's bigger than the screen and pans; over a book or a
notebook, that shows or hides its bar instead), or Mod+F, puts it back. The edges are the window's there, so no
menu by swiping and no workspace swipes from an edge; two fingers
swiped across still go to the next workspace or the one before, and
the light's swipe up or down the left edge still works (in a notebook,
if its settings say so).

New windows open next to the one in focus, along its container: side
by side, or one above another after Mod+V (split vertically; Mod+B,
horizontally, side by side). Mod+E turns a container round, Mod+W
tabs it (a row of tabs, one window shown), Mod+S stacks it (a title
each, one above another, one window shown). Mod+F gives the window the
whole screen (again, or a swipe in from the top or bottom edge, gives it
back); Mod+Shift+Space (or its strip's float) floats it over the others (drag
it by its strip; a floating window is put back with the same key, or
tile).

**Workspaces** are in the bar at the top, numbered, each with its
windows' titles after its number (`1 Alpine · Wi-Fi · Battery`), the one
shown in white: Mod+1…Mod+0 go to one (made as you go there; an empty
one goes once you leave it), Mod+Shift+1…0 send the window in focus to
one, and a tap on one goes there. An empty workspace lists the keys (as
they are, keys.conf's too), and a tap on it opens a window.

A tap on the bar away from the workspaces, or two fingers swiped down
from the screen's top (full screen, they leave it first), or three
fingers swiped down from the screen's top third (anywhere, full screen
too, over any window), or Mod+O, or the menu's
**Workspaces**, shows them all as tiles over the whole screen, each a
small drawing of its windows as they're laid out, the one shown framed.
What's in the windows shows too, as it is now, and again every few
seconds while they're up: a terminal's output, a book's page, a
notebook, the battery. Tap one to go there; the last tile, **+**, makes
a new workspace (the first free number) and opens a launcher on it.
A workspace with windows has a **×** in its tile's corner: tapped, the
tile asks for it again, and a second tap within a few seconds closes
all its windows (what runs in them ends). Keys: 1…9 and 0 go to that
workspace, **n** makes a new one, **x** is the × of the one the arrows
are on, the arrows and Enter pick one, Esc closes.

Mod+Shift+Minus puts the window away (i3's scratchpad), Mod+Minus
brings it back, floating, and puts it away again.

The windows are kept as they're left, in `hellobro.json`, and come back
when hellobro starts (an update's restart too): the workspaces, the
layouts, the floating windows' places, the settings' windows, and the
terminals, which start again: a shell anew, an SSH session connecting to
its host. A launcher, or a terminal running what was typed in, comes
back as a launcher.

## The bar and the keys' focus

One bar at the top, as i3's, and no other: the workspaces at its left,
and at its right where the Kobo is on the network, the Wi-Fi's name and
its address (`HomeNet · 192.168.1.20`; or `no network`, `Wi-Fi off`), the
battery (charge, temperature), CPU and memory use, and the time
(`71% 31°C · cpu 7% · ram 71% · 14:05`). The network's looked at every
15 s, the rest every 5 s. In a binding mode (Mod+R, say) it says which
(`grab` too, while the keys are grabbed), and what plays (a tap on that
plays or pauses), and for a moment what was copied. Mod+Shift+B hides
it, the windows taking its room, and shows it again (Settings, Terminal,
The bar; remembered in `bar`); a full-screen window has none either.

A swipe up or down along the right edge of the screen opens the menu,
full screen too, over anything,
tiles big enough for a finger, each an icon, a word and its keys: rotate
the screen, auto-rotate (on or off, where it is), a screenshot, touch calibration, the on-screen keyboard, the light,
Wi-Fi, Bluetooth, Settings, sleep, lock, the workspaces, the switcher (Go to…), and quit, back to
Nickel. Tap one, or its number, or the arrows (the one they're on has
a ring) and Enter (or type to narrow); Esc or a tap outside closes it.
**Light**, **Wi-Fi** and **Bluetooth** turn it on or off where they
are, the menu staying up: a tile is dark while it's on, and says how
it is (the light's level, the network joined, on, off, or "a moment…"
while a radio changes). **Keyboard**'s tile is dark while the on-screen
keyboard shows; a tap shows or hides it, the menu going. A finger held on Wi-Fi or Bluetooth opens its
window instead: the networks, the devices.

**Airplane mode** (`airplane` in keys.conf and the switcher) turns
Wi-Fi and Bluetooth off, keeping which were on; again, it turns those
back on, as does turning one on in its window or the menu. It stays
through a restart, and Nickel finds them off when hellobro quits.

Two fingers swiped left anywhere go to the next workspace, swiped
right to the one before, as pages turn (where the screen tells two
fingers apart). Settings, **Workspaces by touch**, one finger as well:
a swipe in from the right edge, at any height, to the next, in from
the left edge to the one before; it's off at first, since a book's or
a notebook's swipe near an edge took it. Settings, Touch marks shows where touches
land: a ring where a finger (or the pen) came down, a dot where it
lifted after moving, each for 3 s; the log has each touch then, to the
millisecond.

Settings, Screen, **Auto-rotate** turns the screen as the Kobo is
held: it asks the Elipsa's tilt sensor (the KX122, over I²C, as FBInk
does) every second, and once a side has been up for two looks, and
nothing has touched the screen for 2 s, it turns. Lying flat it stays
as it is. It only turns to a rotation the touch has been calibrated
for: turn to each once with Rotate (it calibrates then). A Rotate by
hand stays till the Kobo is turned again. It's off at first.

**Screenshot** (the quick menu, the switcher, or Shift+Mod+P): the menu
goes, and the screen is kept as it was then. Drag a rectangle round a
part of it (a finger or the pen; its outline is dashed) to keep that
part, tap anywhere for all of it, or Esc for none. It's a PNG in
`Screenshots` on the Kobo's storage (`hellobro-2026-11-12-190130.png`,
over USB or file sharing), and the clipboard's picture as well, to
paste into a notebook (Ctrl+V).

Settings, Screen, **Text size** (a slider: 85% to 150%) makes the
letters of the menus, lists, settings, the launcher and the buttons
bigger or smaller, at once; a terminal's, a book's and a notebook's
have their own.

Settings, Screen, **Sizes**: **for fingers** (the default) has a
taller bar and title strips, bigger window buttons, and
taller rows in menus, Settings, Wi-Fi and Bluetooth, the books' and
notebooks' lists, and the bars over a notebook's and a book's page;
**compact** fits more on the screen, for a keyboard. It
changes at once (also in the switcher: "setting: sizes"). A
notebook in a window too narrow for all its tools along the top (and
taller than wide) has them down its left side instead, all of them,
the page arrows and counter at the foot; the pen's choices open beside
the pen. Narrow and short, the least needed go. `HELLOBRO_TOUCH=big` or
`small` in `launch.sh`'s environment overrides it.

Mod and the arrows (or h j k l, as vim has them) move the focus
between the windows; with Shift, the window itself moves. Mod+A
focuses the container around the window (its whole tabbed group, say,
to move it all), Mod+Tab and Mod+Shift+Tab the next and the previous
window on the workspace.

Mod+R is the resize mode: the arrows (or h j k l) make the window in
focus wider or narrower, taller or shorter; Enter, Esc or Mod+R again
ends it. In a mode only its keys count, even in a grabbed window: no key, not even Shift or Ctrl, reaches a window. While it lasts the windows are
grey boxes with their sizes, the one in focus framed thick: the
terminals take their new size once, when it ends, not at every step.
Moving windows is the same: while Mod+Shift and the arrows move one
(tiled or floating), the windows are boxes; a second after the last
move, or at once on any other key or a touch, they're laid out for real.

In a settings' window, Tab (or ↓) moves the keys' focus through its
controls, as on a desktop: the buttons, the rows, the dropdowns;
Shift+Tab (or ↑) goes back, and the list follows. A dashed frame shows
where it is. Enter or Space presses the button in focus or opens the
dropdown, whose ↑ ↓ then pick and Esc closes. The frame shows once a key
moved it, and a touch takes it away. The terminals keep Tab for the
shell.

### The on-screen keyboard

Without a keyboard, one on the screen: a laptop's rows under the
windows (numbers, letters, Backspace, Enter, the space bar, the arrows
in a cross at the right), a strip at the left (Hide, Home, PgUp, PgDn,
End, Tab, Shift) and one at the right (Esc, Del, Tab, Mod, Alt, Ctrl,
Shift). Its keys are a keyboard's: the window manager's keys, the
terminals', kitty's, all as from a real one. Shift, Ctrl, Alt and Mod
(the window keys') hold till the next key is up, shown in black;
tapped again they let go. Held Shift, the keys show their capitals and
symbols. A key held goes on, as a keyboard's does.

The keys at the screen's edges (the strips, and the rows' ends) wait
before they're pressed: a tap is a key, a hold presses it (and goes
on), and a finger that moves is the edge's swipe, the menu's or a
workspace's, as it is without the keyboard. The light's swipe is above
the left strip.

Settings, **On-screen keyboard**: for a text field (the first, at
first), shown only while one is typed in, with no keyboard connected: a
name, the launcher's find line, a ":" line, a notebook's text, and
hidden once it's done (never for a terminal); off; auto, shown once no
keyboard has been connected for 10 s (and hidden as one connects); or
always.

Two fingers swiped up from the screen's very foot (the last 100 px)
show it whatever the setting, full screen too; two fingers swiped down
onto it hide it, as does its **Hide ▼** key, atop the left strip. The
menu's **Keyboard** (keys.conf: `keyboard`) does the same, showing it or
hiding it. Either holds till a text field
wants it (it shows again) or a keyboard connects. While it shows, a
keyboard gone doesn't bring up the Bluetooth window.

#### Languages

Settings, **Languages to type in**: a tap (or Enter) puts them up as
tiles, ticked or not: English, Russian, Ukrainian, German, Greek. A tap
(or its number) ticks or unticks one, one at least; Esc or a tap outside
closes it (the switcher has it too: "Languages to type in, ticked").
Ctrl+Alt+Space (keys.conf: `language`), or
the on-screen keyboard's key left of the space bar (it says `EN`, `RU`…),
types in the next one, round; a keyboard's keys and the on-screen ones
alike, with the on-screen letters showing it. The layouts are the usual
ones (ЙЦУКЕН, Ukrainian's with ґ on the `\` key, QWERTZ, Greek's) at a
US keyboard's places. Greek's accents are dead keys, as on any Greek
keyboard: `;` then a vowel is ά, έ, ή…, Shift+`;` then ι or υ is ϊ, ϋ;
an accent and a space is the accent alone. Only what's typed changes: text in
a terminal, a name, the find line, a ":" line, a notebook's text, a
password. The keys' commands stay where they are (Ctrl+C, Mod+D, a
book's letters), and so do keys.conf's names. Not in English, the bar
says which; hellobro starts in the first.

A question over the page (a saved host's menu, read-only or
read-write, restart into a new build) has its answers as buttons: the
arrows or Tab move between them, Enter picks the one in black, a
button's letter picks it, Esc closes the question.

## The switcher

Mod+D or Mod+Space, anywhere, is a switcher in the manner of rofi (or
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

The keys are hellobro's commands', i3's with Mod (the Win key). Settings,
Keyboard, "Mod key" makes it Alt instead, as i3's `set $mod Mod1`: then
Alt+Enter opens a window, and the Win key does what Alt did. The hints
say Mod either way (and Win for the Win key, when Alt is Mod), and
keys.conf says mod (meta, as an older hellobro wrote it, works too).

| Key | |
| --- | --- |
| Mod+Enter | a new window: a launcher |
| Mod+D, Mod+Space | the switcher |
| Mod+Shift+Q | close the window in focus |
| Mod+arrows, Mod+H/J/K/L | focus the window there |
| Mod+Shift+arrows, Mod+Shift+H/J/K/L | move the window there |
| Mod+Tab, Mod+Shift+Tab | the next, the previous window |
| Mod+1…0, Mod+Shift+1…0 | go to a workspace, send the window there |
| Mod+O | the workspaces as tiles, and a new one |
| Mod+B, Mod+V | split: the next window beside, below |
| Mod+Shift+B | hide the bar, show it again |
| Mod+E, Mod+W, Mod+S | side by side ⇄ one above another, tabbed, stacked |
| Mod+F | fullscreen (or its strip's fullscreen; two fingers swiped down from the top leave it, one too but over a book or a notebook: its bar) |
| Mod+Shift+Space | floating |
| Mod+A | focus the container around |
| Mod+Shift+Minus, Mod+Minus | put the window away, bring it back |
| Mod+R | resize mode: arrows, then Enter or Esc |
| Mod+Shift+C | read keys.conf again |
| Mod+Shift+E | quit, back to Nickel (i3's exit) |

To change one: Settings, the Keys row (or "keys" in the switcher),
lists the commands with their keys. Pick one and press its new key, with
Ctrl, Alt or Mod (an F-key alone will do): it takes the place of the
ones it had, and a command that had that key lets it go. Backspace
leaves it no key, Delete puts its defaults back, Esc leaves it be.

It all goes into `keys.conf`, which can be edited too. It's written the
first time hellobro runs, every command in it with its default keys,
commented out (a `keys.conf` of an older hellobro with nothing but its
defaults is written again). A line is a key and a command, hellobro's
or any of [i3's](https://i3wm.org/docs/userguide.html#list_of_commands),
as i3's `bindsym`:

```
mod+t       layout tabbed; focus parent  # i3's, any line of them
f1           workspace number 1
f2           workspace number 2
mod+o       [class="hellobro" instance="wifi"] focus
mod+x       exec htop                    # in a new Alpine window
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
ctrl, shift, alt and mod (meta works too). Saved (over SFTP, say), it takes effect at
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
It's a command like any: `mod+n notify hi bro` in keys.conf works too.

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

A swipe up or down the left edge sets it, full screen too (and keys:
see `keys.conf`). A notebook's settings, **Light by the left edge**,
can turn it off there, so a finger by the edge is the page's.
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

The power button, Shift+Mod+S, the menu (a swipe along the right edge), or "Sleep"
in the switcher put the Kobo to sleep with hellobro still up, as Nickel would (a sleep
screen, the light off, suspend to RAM); the power button wakes it, and
it's all as it was. A player that plays is paused first (Bluetooth goes off),
Wi-Fi and Bluetooth go off and come back on waking, and the SSH sessions
reconnect once Wi-Fi is back. Nickel holds the power button's input
device (frozen, all the while hellobro is up), so hellobro reads the
button's GPIO instead (`/sys/kernel/debug/gpio`); the presses still queue
up for Nickel, which may see them when hellobro quits (and go to sleep
once).

It also sleeps by itself after a while untouched (Settings, Power): on
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

Shift+Mod+X (or Lock in the menu) locks the screen, for a bag or a hand: touch does nothing
and keys go nowhere, but the music's keys, the power button (sleep),
and Shift+Mod+X again, which unlocks. The light goes off till then
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
screen (unless the on-screen keyboard shows), to connect it again; when it's back the window goes (or, if it
was open already, goes back to its place). A keyboard back within the
15 s, napping, say, shows nothing, and asleep or just awake it waits:
Bluetooth comes back on with the keyboard. Settings, **The keyboard
gone**: after 15 s, 30 s or 1 min, or nothing comes up.

## The launcher

A new window (Mod+Enter, a window's **+**, or the first one) is a
launcher: what hellobro has, as tiles to tap, in groups.

- **Terminals**: **Alpine** (1), the **Kobo shell** (2), each saved
  host (3 and on: its menu), the nearby hellobros to type into
  ([Typing into another device](#typing-into-another-device)), and
  **SSH key**, which copies this Kobo's public key.
- **Read and write**: **Books** (All Books), **Notebooks** (All
  Notebooks), **New notebook**, then the last two books read (how far
  in) and the last two notebooks written, to go on with.
- **Settings**: Wi-Fi, Bluetooth, Sharing, Settings, Battery, What
  uses it, each in its window, and **Keys** (a command's keys).
- **This Kobo**: **Rotate** the screen, the on-screen **Keyboard**
  (show / hide), the **Light** on or off, **Sleep**, **Lock**, **Touch**
  (calibrate), and **Nickel** (quit, back to it; it asks first).

A tile opens in the launcher's place; Rotate, Light and the like leave
it there. The numbers (1–9) still pick the terminals; the arrows light
a tile (Alpine is lit at first) and Enter opens it. The tiles are as
big as the window has room for; in a small one only their names show,
and a swipe up or down scrolls them. At the bottom is the **find>**
line: typing goes there, and narrows the tiles to what matches (fuzzy,
as the switcher; every notebook and every book read, not only the last
two), the best first and lit: Enter opens it, the arrows light another,
Esc clears what's typed (and then closes the window). With an `@` in
it, or nothing matching, it's the **ssh>** line for a host not saved
([Terminal](#terminal)): Enter logs in. A tap on it brings the
on-screen keyboard if no keyboard is connected. How the last session
ended is noted above it.

## Terminal

- A launcher (Mod+Enter, or a window's +) asks what to run
  ([The launcher](#the-launcher)): **1** Alpine, **2** the Kobo's sh
  (Enter alone: the tile lit, Alpine at first), **3** and on a saved
  host, or a host typed in. Esc closes it.
- Type `user@host[:port] [command]` and Enter (a name alone logs in
  too, when no tile matches it); the prompt also takes
  `ssh -p PORT [-J user@jumphost] user@host [command]`. A host typed
  there is saved once it connects (in `hellobro.json`), and gets its
  tile.
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
- Ctrl+= / Ctrl+- / Ctrl+0: font size, the terminal in focus's: the
  others keep theirs. A new terminal takes the size last set. Each
  window's is kept with the layout in `hellobro.json`.
- While connected, every other key goes to the remote, Esc included, but
  for hellobro's own (the commands' keys: Mod+Shift+E quits, say). A
  program there that wants one of them gets it if keys.conf frees it:
  `ctrl+shift+v -`.
- **Grab** (Mod+Shift+G) gives every key to the window in focus,
  hellobro's own too, till Mod+Shift+G again: for tmux, a desktop or
  another hellobro on the other end, with keys of their own. The bar
  says `grab` while it's on.
- A program that asks for kitty's keyboard protocol (`CSI > flags u`:
  kitty's own, foot's, neovim, helix, hellobro-keys…) gets the keys as
  kitty says them: the modifiers apart, Super among them, and releases
  and repeats if it wants them. A modifier tapped for one of hellobro's
  keys reaches it too, as in kitty; the key itself doesn't.
- Images: a program that speaks kitty's graphics protocol shows pictures
  in the terminal, in grays: `chafa -f kitty photo.jpg`, `timg -pk
  photo.jpg` (both are in Alpine's apk), yazi's previews, and books with
  read-kitty (below, in the Alpine shell). They're sent
  in the stream (PNG, RGB or RGBA, zlib'd or not); a program that wants
  to hand over a file name instead is told no, and sends them in the
  stream then. They're cells like any text: they scroll away, and
  `clear` clears them. Scaled, they're dithered to the panel's 16
  grays; shown pixel for pixel (a page's text), each pixel is only
  rounded to its nearest gray. The cells' size in pixels is told as programs
  ask for it (`CSI 16 t`, and the pty's pixel size; not in-band resize,
  mode 2048, whose reports can't say the pixels). Unicode placeholders
  too: an image placed virtually (`U=1`) shows where the program writes
  its U+10EEEE cells (ratatui-image's way). fancy-cat (PDF) and
  bookokrat (EPUB, PDF, DjVu) work, their pictures and pages too. No
  animation.
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
- **read-kitty** reads a book or a document a page at a time: `apk add
  mupdf-tools` once, then `read-kitty book.epub` (PDF, EPUB, MOBI, FB2,
  XPS, CBZ: whatever MuPDF opens). Each page is drawn for the window's
  pixels exactly, so text is as sharp as the panel makes it; a book's
  text is laid out for the window, in points as on paper. Pictures open
  too (JPEG, PNG, GIF, TIFF…). Space, Enter, PgDn: on (down a zoomed
  page, then the next one); b, Backspace, PgUp: back; the arrows (or h
  j k l): pages, or zoomed in, about the page; Home, End; a number then
  Enter: that page; + and -: zoom (a book's text bigger, smaller
  instead); w: the page's width, and back; q: quit. Zoomed, mutool
  draws only what the window shows. Where each file was left (and its text's size) is
  kept in `~/.local/state/read-kitty/places`, and a second argument
  goes to that page. For long reading KOReader is still the better
  reader: no terminal, no radio.
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

## Books

hellobro has a reader of its own for EPUBs, set in the Kobo's fonts
(Georgia, Caecilia, Malabar, Kobo Nickel…, each with its bold and
italics) or the book's own, with the book's pictures, headings, italics
and centred text; and for PDFs, drawn by MuPDF (below).

- Open the shelf, **All Books**, from a new window (the **Books** tile)
  or the switcher (**Books**). It has the EPUBs and PDFs anywhere on
  the Kobo but its dot folders, in three tabs: **Recent** (the last read
  first, then the rest by name), **All** (by name) and **Folders** (a
  folder at a time: its folders first, each with how many books are in
  it, then its books; the top row goes up a folder). Tab goes to the
  next tab, Backspace up a folder; the tab and the folder are kept.
  Beside each book is its type, epub or pdf, and how far in you are if
  you've read it.
- Tap a book, or pick it with ↑ ↓ and Enter. It opens where you left it.
- **rename** on a book's row (or F2) types its name in the row, as in
  All Notebooks: Enter or **ok** keeps it. Its file is renamed beside
  it (a `.kepub.epub` stays one), and the name shows from then on, over
  the book's own title; its place and bookmarks go with it.
- The book's lists and panels (its contents, bookmarks, pages and
  style) all have the same header: **‹ All Books** at the top left, the
  shelf, and **back to the book** at the top right. The shelf opened
  from a book has **back to the book** in the same place (or Esc).
- Turn pages by tapping the right two thirds (on) or the left third
  (back), by a swipe across the page (to the left on, to the right
  back), or with Space, → ↓ PgDn, j, l (on) and ← ↑ PgUp, Backspace,
  k, h (back). Home and End go to the chapter's start and end.
- Over the page is a bar, as over a notebook's: **‹ All Books** (the
  shelf; q too), **☰** the contents, **Aa** the style panel, a ribbon
  for a bookmark (a PDF's has the select tool and its zoom too, below),
  and at its right ◀ the page before, which page (of the chapter in an
  EPUB, of all in a PDF; tapped: the contents, or a PDF's pages) and ▶
  the next. Between the tools and ◀, when there's room: the chapter and
  how far in. In a narrow window the least needed buttons go.
- A link in an EPUB, tapped (a note's number, say), shows what it
  points at in a box at the foot of the page: a tap elsewhere (or Esc)
  closes it, **go there** (or Enter) goes to it. Then **‹ back** is
  on the bar, after ‹ All Books: a tap on it comes back.
  A link to a whole chapter goes there at once.
- **s**, or a tap on **Aa**, opens the style panel, the reader's own: the fonts, each shown in itself
  (tap one), the size, the lines' spacing, the margins (± each),
  justified and hyphenation (on / off), **Pages** (this book's own,
  kept with its place: one page, two side by side, or two if wide: two
  when the window is wider than tall, a landscape screen), **Bar,
  fullscreen** (shown or hidden, for all EPUBs), and under them the
  book's own text, from the page it's open at, set as the choices have
  it (a page with little or no text is followed by a bit of Alice). With keys: ↑ ↓ to a row, ← → to change it, Enter to pick or
  switch; Esc, s or "back to the book" (at the top right) leaves it. Every book takes it;
  it's kept in `hellobro.json`.
- **Hold a word** on an EPUB's page, a finger or the pen, a moment:
  without lifting, what the dictionaries say of it shows in a box, at
  the foot of the page, or at its top if the word's down there (a line under the word
  shows which). A tap elsewhere (or Esc) closes it; **next** (or Enter,
  Space) goes on if there's more. The dictionaries are StarDict's (`.ifo`,
  `.idx`, `.dict` or `.dict.dz`, `.syn`), in `Dictionaries/` on the Kobo's
  storage, and all of them are asked, those named for the book's language
  first (`grc-en` for `grc`). Copy one in over file sharing: no restart.
  A word's found in any case and with any accents, whichever way the
  book writes them, and by its other forms, if the dictionary lists them;
  a word cut short (`l’homme`) by what's left. `cmd/dictconv` makes one of
  [Wiktionary's](https://kaikki.org): see [docs/dictionaries.md](dictionaries.md).
- Without the panel, **+** and **-** make the text bigger and smaller,
  **f** (Shift+F back) goes through the fonts.
- A book with fonts of its own (`@font-face` in its CSS) is set in them
  where its CSS asks for them, and the panel's fonts start with **Book's
  own**. Picking one of the Kobo's fonts (in the panel, with **f** or
  `:font`) turns them off for that book, and it's kept with its place;
  **Book's own** turns them back on. TTF, OTF, WOFF and obfuscated fonts
  are read; a WOFF2 isn't, and a font that can't be read leaves its text
  in the font chosen, as do the letters a font hasn't got.
- **t**, Enter, **☰** or a tap at the page's top shows the contents: pick a
  chapter, or swipe up and down for more. Esc or "back to the book"
  leaves them, "‹ All Books" goes to the shelf (on a page, Esc and q do too).
- **b**, a tap on the ribbon or on the page's top right corner
  bookmarks the page (a ribbon hangs at
  its top right then), or takes its bookmark away. The contents' second
  tab (tap it, or Tab; **b** in the contents) lists the bookmarks, how
  far in, the chapter and how the page starts: Enter or a tap opens one,
  **d**, Delete or its × takes it away. They're kept with the book's
  place in `hellobro.json`.
- Where each book was left is kept in `hellobro.json` (at most every 20
  s while reading, and when its window closes), as a place in its
  chapter, so another size or font finds the page again. The screen
  flashes clean every 8 pages.
- In fullscreen (Mod+F) the page has the whole screen: a PDF's has no
  bar then (keys, two fingers to zoom, a tap at the page's top for the
  contents), an EPUB's keeps it. A finger swiped down from the screen's
  top edge shows the bar, or hides it, as the style panel's **Bar,
  fullscreen** (s or Aa) does, for all PDFs or all EPUBs; two fingers
  swiped down from the top leave fullscreen.
- Two pages side by side (**Pages** in the style panel) turn two at a
  time; each chapter starts on the left.

PDFs are on the same shelf and read the same way: a page at a time,
the contents are their outline (if they have one), bookmarks are the
same. A book is opened aside: the window says **opening…** meanwhile
(a damaged PDF, read whole, can take seconds), and the rest of
hellobro goes on; closed before it's open, it's let go. An EPUB's
chapter is laid out aside too: turned into, the page shown stays till
the next is ready (turns meanwhile wait for it), and a window resized
or a font changed says **laying out…** for that moment. Their pages are drawn gray by MuPDF, linked into hellobro (package
`pdf`), with their white margins trimmed, as big as the screen takes the
whole page. **s** or **Aa** has their own panel: **Fit** the whole page
or the page's width, **Zoom** (− +, 100% to 400% of the fit; **+** and
**-** on the page too, and − **100%** + on the bar, right of the
select tool; each PDF its own, kept with its place), **Trim
white margins** on or off, **Pages**
as for EPUBs (each PDF its own): two side by side are each
whole, meeting in the middle as a book's do, turned two at a time, the
next two drawn while you read. A cut works on either. A page
bigger than the screen is read in screenfuls, across then down, a
little of each showing again on the next. Two fingers pinched apart or
together on a page zoom it in or out round the point between them:
while they move, a frame on the page shows what will fill the screen,
with the zoom in it, and the page is drawn again when they lift.
Zoomed bigger than the screen, a finger dragged on the page moves it,
anywhere (it moves in black and white, then is drawn whole when the
finger lifts); a tap, a key or a swipe goes on a screenful from there.
The next page is drawn while
you read this one, so turning to it is at once. PDFs locked with a
password don't open (an empty one does).

A PDF's contents have a third tab, **Pages** (or **o** on a page, Tab
from the bookmarks): its pages small, a screenful at a time, the one
being read marked "reading". They fill in one by one (MuPDF draws them
aside, those showing first, then the next screenful's). Tap one to read
it, or move the frame with the arrows and Enter; a finger swiped up or
down, or PgUp and PgDn, goes a screenful.

To cut a part of a page out, draw a rectangle across it with the
pen, or, on a PDF's, hold a finger still on the page a moment, then
move it (as to select in a terminal; on an EPUB's that looks a word
up). Or take the select tool, the rectangle with
dots at its corners on the bar over the page (black
while it's in hand): drag a finger across the page, and the rectangle
follows it at once; or tap one corner, then the opposite one. Each
corner then has a dot. Drag a dot to move that corner, a side to move
that side, or the inside to move the whole rectangle; a drag anywhere
else draws a new one. Or tap a dot (it fills in black) and tap where
it should be, the corner across staying where it is. Tap the tool
again, or Esc, to put it down; full screen without the bar puts it
down too, so taps turn pages there. Under the rectangle:

- **copy**: its words (whole ones, line by line) into the clipboard,
  which the terminals paste (Ctrl+Shift+V), and its picture, which a
  notebook pastes ([Notebooks](#notebooks));
- **save**: a picture of it at 300 dpi (an EPUB's as the screen has
  it), and its text, in `Clips/` on
  the Kobo's storage (`/mnt/onboard/Clips` in Alpine), named after the
  book and the page (an EPUB's, how far in), and as `Clips/latest.png` and `latest.txt` too: a
  program in Alpine (an LLM's, say) can take the last cut from there;
- **zoom**: the page zoomed so the rectangle fills the screen, as far
  as 400% goes, in its middle (a PDF's, one page at a time only);
- **×**, a tap elsewhere or Esc takes it away, as does turning the page.

A button tapped goes black till it's done, so a slow copy or save
shows it was heard.

A scanned page has no text: copy and save keep it as a picture.

The layout is the `book` package's: lines broken at spaces and after
dashes, justified where the book says, a little of the book's CSS
(alignment, italics, bold, sizes, indents, margins, floated pictures
such as Gutenberg's drop caps). A word that doesn't fit at the end of a
line is hyphenated: by TeX's patterns in a book in English (or one that
doesn't say its language), names left whole; in any book where it has
soft hyphens.

### ":" commands in a book

As in vim: **:** opens a line at the window's foot (and the on-screen
keyboard, if there's no other). A command and Enter runs it; Esc,
Backspace on nothing, or a tap leaves it. A name can be cut short while
it's the only one so begun (`:ex` is export); the line shows what what's
typed could be. `:help` (`:h`) shows them all in a menu, each with what
it does: typing narrows it, Enter runs the one lit, or puts it on the
line if it takes something (`:size `, then the size).

**?** in a book or a notebook puts up a sheet of its keys and its
commands, in columns over the page; any key or a tap takes it down.

| Command | |
|---|---|
| `:12` | page 12: a PDF's, or of the chapter shown in an EPUB |
| `:40%` | 40% into the book |
| `:ch 3` | chapter 3 (of the book's files, in order) |
| `:/word`, `:f word` | the next page with the word on it (any case; a PDF's text across its lines too); `:/` again: the next |
| `:toc`, `:marks`, `:mark` | the contents, the bookmarks; a bookmark here (or not) |
| `:back` | back from a link followed |
| `:s`, `:size 30`, `:font lit` | the style panel; the size; the font that matches best |
| `:z 200`, `:pages` | a PDF's zoom (100–400); its pages |
| `:rename NAME` | as the shelf's rename |
| `:shelf`, `:q` | All Books; close the window |

## Notebooks

A notebook is pages to write on with the pen: sums worked out by hand,
say. "Notebook" in the switcher (Mod+D) opens a new one; the ones
you have are listed there by name (the date and time each was made,
till it's renamed); the launcher has **Notebooks**, **New notebook**
and the last two written.

- **All Notebooks** (the launcher's **Notebooks**, **Notebooks** in the
  switcher, **‹ All Notebooks** at the left of a notebook's bar, or q)
  has the books' header and tabs ([Books](#books)): **Recent** (the
  last written first), **All** (by name) and **Folders** (a folder at a
  time, its folders first; the top row goes up). Opened from a
  notebook, **back to the notebook** is at the top right, where the
  books have "back to the book" (or Esc).
- **+ new notebook** (or n) asks what kind first ([below](#a-new-notebook))
  and makes it in the folder shown; **+ new folder** makes a folder there. The folders are folders
  in `Notebooks/`, four deep at most.
- Hold a row (or Space on it) to pick it: each row gets a box, and a
  tap ticks or unticks one (a folder too: it opens again once picking's
  done). Over the list then: **rename** (one picked only), **copy**,
  **cut**, **delete**, **cancel** (or Esc), and how many are picked.
- **rename** (or F2) types its name in the row: Enter or **ok** keeps
  it, Esc or **cancel** doesn't. Without a keyboard, the one on the
  screen shows for it. A notebook's PDF exports are renamed with it
  (only ones hellobro wrote); a folder's notebooks go with it, open
  ones too.
- **copy** or **cut** (Ctrl+C, Ctrl+X: what's picked, or the row) then
  **paste here** (Ctrl+V) in the folder shown (Folders; the top in
  Recent and All): a copy named as it was, "(copy)" after if that's
  taken, a folder with all in it; cut, it moves, its export and its
  open window going with it, as with a rename. **cancel** forgets it.
- **delete** (or Delete) asks first; then the notebooks go to
  `Notebooks/.trash` (their windows close first), and a folder too,
  with all in it, to be fished out over USB or sftp.
- Tap a notebook, or Enter, to open it in the list's place; one open
  in another window is focused there. ↑ ↓ (or j k) pick, Tab goes to
  the next tab, Backspace up a folder, a swipe up or down pages.

Its buttons are pictures (below by their names): the pen, an eraser,
a dotted rectangle (select), a T (text), arrows round (undo, redo), a clipboard
(paste), a page as its paper is (paper), a tray with an arrow out
(export), sliders (settings). Under the pen, a line as it writes: its
width, its gray.

- The pen writes; a heavier hand, a thicker line. **eraser** (or E)
  takes off each stroke the pen goes over; **pen** (or P) is back to
  writing. The pen's buttons, held: the first draws a marker, the
  second erases (or the other way round, or both erase: **Pen
  buttons** in the settings); its eraser end erases. Which button is
  first is the pen's: the log says `pen button 0x14b` (the first) or
  `0x14c` (the second) as one goes down.
- Tap **pen** while it's in hand for its choices, under it: **fine**,
  **medium**, **bold**; **black**, **gray**, or **marker** (a
  highlighter: wide, light gray, under the ink, yellow in Xournal++).
  The line under the button shows which; the choice is kept.
  Touch the page to put them away. A notebook from Xournal++ shows its
  colors as the grays they are, its highlighters as markers.
- **text** (or T, or `:text`) types on the page from a keyboard (or
  the one on the screen): tap the page and type there, or tap a text to
  go on in it. Enter starts a new line; the arrows, Home and End move
  the cursor; Esc, another tool, a tap off the page or a turned page
  leaves it. A text is undone as one; emptied, it's gone. The select
  tool takes it with a tap (moves it, sizes it, copies, deletes it);
  the eraser leaves it, as in Xournal++, where it's a text as its own
  are (14 pt; theirs show here too, at their size) and can be edited.
- **undo** and **redo** (Ctrl+Z, Ctrl+Shift+Z or Ctrl+Y).
- **paper** is a tool too: tap it for the papers, under it, each with
  its picture: **plain**, **lines**, **squares**, **dots**, and each of
  the three **narrow** (3.3 mm apart, not 5). A tap makes it the
  notebook's, every page's (`:paper dots-narrow` too); its button shows
  which; tap it again, or the page, to put them away. In Xournal++ they're
  its ruled, graph and dotted backgrounds (the narrow ones as wide as
  its own, marked so hellobro knows them again); the PDF and PNG export
  them as on the screen.
- **settings** (the sliders, or `:settings`) shows the notebooks'
  settings over the window, with the books' header: ‹ All Notebooks,
  **Settings**, **back to the notebook** (or Esc, q). **A finger**
  (below), **Pen buttons** (above), **Export** (below), **Light by
  the left edge** (on: a finger up or down it sets the light, full
  screen too; off: full screen, a finger there is the page's), **Bar,
  fullscreen** (below), **Pages (this notebook)** (below), **AI diary
  (this notebook)** and the rest of the AI's (below): a tap on a
  value, or ↑ ↓ and Enter, ← →, steps it. They're the notebooks' own:
  the app's Settings don't have them.
- The arrows turn the page, as do Left, Right, PgUp, PgDn; past the
  last page, if it has something on it, a new page. Once a pen has
  written (or as the settings say, below), a finger swiped across the page turns it too (leftward: the
  next, as pages turn).
- **Page size (new pages)**: **the screen, upright** (as pages have
  always been), **the screen, across** (a landscape page: it fills a
  landscape screen, one page, not two side by side), **A4**, **A5**,
  **Letter**, **a square**; or any size with `:pagesize 200x120` (mm,
  40 to 1000 a side; `:pagesize landscape` and the rest by name too).
  New pages are made so, and the page shown too if nothing's on it yet;
  a page written on keeps its size, so a notebook can have pages of
  several (two of different sizes aren't shown side by side). Each
  page's size is in the `.xopp`, for Xournal++ and the PDF export. A
  page lower than the window shows its top and foot.
- **Pages (this notebook)**: **one by one**, or **a roll: scrolled up
  and down**. A roll shows the page as wide as the window (no wider
  than it is, landscape), and the arrows and keys scroll it a
  screenful at a time (most of one, so a line or two stays in sight),
  as does a finger swiped up or down. Where the page ends, it grows
  for the scroll, if there's something written on the screen or under
  it: write on, there's always more paper. On a blank screen at its
  foot, it's the next page's top (a new page if this one's written on),
  and back past a page's top, the one before's foot. The diary answers
  only what's written under its last answer, so write on down the roll. A finger swiped across still turns
  the pages. In the `.xopp` they're pages, only taller (up to about 17
  screens; then the next page goes on from there): Xournal++ shows
  them so, and the PDF has them as tall pages. The AI diary's answers
  grow the page as they go, and the roll follows them.
- The page counter between the arrows (or O) shows all the pages
  small, a screenful at a time (the arrows, PgUp, PgDn or a finger
  swiped across for more), and a **+ new page** tile after the last.
  Tap a page to choose it (or the arrow keys), again to open it (or
  Enter, or **open**, corners as a frame). The strip has then: **new
  before** and **new after** (a + by a page), **copy page** (a copy
  after it; two pages), **clear page** (words: it asks, tap it again
  within a few seconds; it says "tap again"), **delete** (a bin, or
  Delete; the only page is cleared instead), **earlier** and **later** (arrows: moving it), and
  **undo**: each is undone like a stroke. Each page has a **×** in
  its corner as well: tapped, it asks ("tap × again"), and tapped
  again within a few seconds it deletes that page, undone as delete
  is. The counter again, or O, goes back to the page.
- Till then fingers write: a stylus without a battery (or a Kobo Stylus
  with a flat one) is a finger to the screen. A pen seen once is
  remembered (`pen` in `hellobro.json`). The notebook's settings, **A
  finger**, chooses instead: **writes** (fingers always write, as
  the pen does) or **turns** (fingers only turn pages, so a hand resting
  on the screen writes nothing; only the pen writes). Where fingers
  turn pages, a finger held still on the page for 0.6 s picks up the
  select tool: what's under it is taken (its frame and buttons show),
  and the same finger dragged on moves it; held where nothing is and
  dragged, it draws the rectangle that takes what's in it (below).
  The pen, P, puts the select tool down again. Near the left and
  right edges a finger writes nothing: those are the light's and the
  workspaces' swipes. Full screen, the right edge is still the menu's
  (it's everywhere), and the left edge is the light's unless the settings' **Light by the
  left edge** is off.
- A page is the screen's shape, upright, fitted into the window. Where
  two side by side are nearly as big as one alone would be (landscape,
  fullscreen; a wide, short window) two show, and turn two at a time;
  writing on the second makes it a page.
- Fullscreen, a notebook keeps its bar. Its settings' **Bar,
  fullscreen**, hidden, leaves the page all of the screen: the pen
  writes, keys do the rest (the pages, small, keep it). A finger
  swiped down from the screen's top edge hides the bar, or shows it,
  as that row does (not the pen: it writes there); two fingers leave
  fullscreen.
- **select** (or S) takes what's written, and pictures, to change
  them; the pen and fingers both work with it. Tap a stroke or a
  picture to take it (the ink over a picture first), or draw a
  rectangle round some, as in AutoCAD: drawn left to right (a window,
  its outline whole) it takes all that's wholly inside; right to left
  (crossing, its outline dashed) all it touches too. A dashed frame
  goes round what's taken, with a dot at each corner and one over it:
  - drag inside the frame to move it, onto the page beside it too where
    two show;
  - drag a corner's dot to make it bigger or smaller (the corner across
    stays put, the shape kept, lines as much thicker);
  - drag the dot over it round to turn it: freely, snapping to quarter
    turns near them; pictures only by quarter turns (Xournal++ has no
    other way to keep them);
  - the buttons under it: **copy** (two pages; Ctrl+C), **cut**
    (scissors; Ctrl+X), **duplicate** (two pages and a +; Ctrl+D: a
    copy down and right, taken), **turn** (an arrow round: a quarter,
    clockwise), **mark** (a ribbon; `:mark`: to link to it, below),
    **delete** (a bin; Delete or Backspace).

  What's done stays on the page; undo puts each back. A tap where
  nothing is lets go; another, with nothing taken, puts a cross where
  the next paste goes (tap the cross to take it away). **pen** or
  **eraser** puts the tool down.
- **link** (two links of a chain, or `:link`) puts a link to a part
  marked, in this notebook or another: it shows **Marks**, a tab by
  the settings: each part marked, the last first (40 kept, in
  `note-marks.json`), with its picture, its text if it has some, its
  notebook and page. **link here**, then a tap on the page, puts the
  link there: "→ the budget (plan, p. 3)", framed. A finger's tap on
  a link opens its notebook in this window, at its page, the part
  marked framed for 3 s; held (more than 0.6 s), in a window of its
  own; open already, it's shown where it is. `:back` goes back where
  the link was tapped. **go to** goes there from the list, **forget**
  takes a mark away (links to it still go). The pen writes over a link
  as over any text; the select tool takes it, to move or delete it. In
  Xournal++ it's the text alone.
- What's copied from a notebook is kept as `clipboard.xopp` next to
  hellobro: paste it (Ctrl+V, **paste**) on another page or in another
  notebook, where it was on its page, or at the cross. It comes in
  taken, to be dragged where it should go. Whichever was copied last,
  ink or a PDF's picture, is what pastes.
- **paste** (or Ctrl+V) puts the picture last copied from a PDF on the
  page: at the cross, if there's one, else on the page shown (the left
  one of two) under what's written on it, shrunk to the margins if need
  be, or on the next page if there's no room left there. It comes in
  taken by the select tool: drag it where it should go, then **pen** to
  write. It's under the ink: write over it. The eraser leaves it; undo
  and clear page (in the overview) take it away.
- It's kept a few seconds after the last stroke and on closing, as
  `Notebooks/NAME.xopp` on the Kobo's storage: over USB, or file
  sharing (`sftp kobo.local`). A notebook's window comes back with the
  others.
- A notebook written on a PDF in Xournal++ shows the PDF's pages under
  the ink, if the PDF is where the `.xopp` says (blank a moment the
  first time: MuPDF takes most of a second a page).
- **export** (or Ctrl+E) asks: **PDF** of the pages (lit: Enter or a
  tap), **PNG** of the page shown (n), or cancel. The notebook's
  settings, **Export**, can make it always the one without asking; `:export pdf` and
  `:export png` don't ask either. The PDF is the notebook next to its
  `.xopp`, `Notebooks/NAME DATE TIME.pdf` (each export its own, as
  `2026-10-30 14.05.07`): a picture of each page at 200 dpi, gray, to
  send, print, or hand an LLM. All the pages to the last written on (of
  one on a PDF, only the pages written on). The PNG is the page
  shown alone, at `Clips/NAME pN DATE TIME.png` (each export its own, as
  `2026-10-30 14.05.07`) and `Clips/latest.png` too, as a PDF's cut
  does, for a program in Alpine to take. It's done aside; a note on the
  page says when it's written, and where.
- `.xopp` is [Xournal++](https://xournalpp.github.io/)'s: open it there
  on a computer to go on writing, add text, or export a PDF. Its texts
  show and are edited here too (the text tool). What hellobro doesn't
  draw yet (a picture background) stays as it was when it writes the
  notebook again; only the
  layers become one. A notebook is written whole or not at all (a new
  file, then in the old one's place); if the card won't take it (full,
  say), a note says so, the window keeps it, and it's tried again, even
  closed. A picture over 16 million pixels isn't drawn, but stays in
  the file. Written on a computer meanwhile (Xournal++ over sftp), an
  open notebook is read again within seconds, if it has nothing unkept
  here; if it has, the computer's file is left be and this one is kept
  beside it as `NAME (Kobo).xopp`, to be put together there. The first notebooks, `notebooks/NAME.hbnote` next
  to hellobro, become `.xopp` when they're opened.

### A new notebook

A new notebook (**+ new notebook**, "Notebook" in the switcher, the
launcher's **New notebook**) asks first what it's to be:

- **A notebook** (Enter or n): one to write and draw in, nothing sent
  anywhere.
- **An AI notebook** (a): one that answers ([the AI diary](#the-ai-diary)),
  then **How should it answer?**: the prompts it comes with, each shown
  in full (a diary as Tom Riddle's, a tutor, a helper, brief, a language
  partner, a translator, a programmer), the ones you saved after them
  (**forget** drops one), and **+ your own prompt**. Tap one, then
  **make it**.

**Your own prompt** is typed (**typed**) or written by hand (**by
hand**: write it on the sheet, then **read it to text**: the model reads
it, and it comes back as text to put right before it's kept). Give it a
name to find it again (else its first words are), and **save it for
every notebook** (on) keeps it in `ai-prompts.json`, for the next new
notebook and for **AI voice** in any notebook's settings. Settings, AI
notebooks, **Your prompts** lists them (also "AI notebooks: your
prompts" in the switcher): tap one to read it whole and change it
(**keep it**), **forget** one, or **+ a new one**, typed or by hand. **‹ back**
(or Esc) goes a step back; closing the window makes none.

### The AI diary

A notebook can answer what's written in it, as Tom Riddle's diary did.
Turn **AI diary (this notebook)** on in its settings (each notebook
its own; it's kept in the file). Write, and rest the pen: after about
3 seconds (**AI asks after a pause**: 2, 3, 5 or 10 s, or off) what's
been written since the diary's last answer on that page goes to a
language model as a picture: the handwriting, typed
text, and pictures pasted there (a book's cut, say: sent as sharp as
the screen shows it). If it reads as finished (it ends with a question
mark, a full stop or an exclamation mark) the answer is written under
it, a word at a time; if not, it waits for more, and writing on sends
it all again. While it thinks, a black badge under the question, where the answer
goes, says so: a spinner turning, "thinking", and the seconds so far.
Three quick taps of a finger on the page (once a pen has written, when
a finger turns pages) ask at once, finished or not (**AI asks on
taps**: three, two, or off). With the pause off, only taps ask. A full
page goes on on the next one if it's empty, else on one put in after
it, and the pages turn with it; in a roll (below) the page grows
instead, and the roll follows the answer down.

It answers in Markdown (**AI answers**, below), laid out as it says:
headings, **bold** and *italic*, lists, quotes with a bar, tables in
columns, code in Go Mono in a frame, and formulas in LaTeX (`$...$` in
a line, `$$...$$` on their own) drawn as TeX would: fractions, roots,
powers and indices, sums and integrals with their limits, brackets as
tall as what they hold, matrices, cases, aligned equations, accents,
Greek, in STIX Two Math. A formula is a picture on the page, its LaTeX
kept with it in the `.xopp` (`hellobro-tex`). The text is in the
notebook's font (**AI font**); a letter it hasn't is drawn in Gentium
Book, then Go, so Cyrillic and Greek always show, as in any text in a
notebook. In plain text, `x^2` and `2^{10}` are written as powers (x²,
2¹⁰).

A diagram comes as a ` ```mermaid ` block, in Mermaid's language, and
is drawn: flowcharts (`graph`, `flowchart`: its shapes, `-->` `-.->`
`==>` and labeled edges, subgraphs, any direction), state diagrams
(`stateDiagram-v2`: `[*]`, composite states, choices, forks), sequence
diagrams (`sequenceDiagram`: participants and actors, the arrows,
notes, `loop`/`alt`/`opt`/`par` frames, `autonumber`), class diagrams
(`classDiagram`: members, annotations, generics, UML's arrows and
multiplicities), entity-relationship diagrams (`erDiagram`: attributes
with their keys, crow's feet), Gantt charts (`gantt`: sections, `after`,
lengths, done/active/crit tasks, milestones), timelines (`timeline`,
drawn down the page), mind maps (`mindmap`: its shapes, growing to
both sides), pies, and bar and line charts (`xychart-beta`). Flowcharts,
state, class and ER diagrams are laid out in ranks as Mermaid's own
(dagre) does, their labels in the notebook's font, Markdown and `$...$`
formulas in them too. It's a picture, made smaller where it'd be wider
than the text or higher than a page (a screen, in a roll), its source
kept in the `.xopp` (`hellobro-mermaid`). Other kinds (quadrant
charts, journeys...) stay code.

A graph of functions comes as a ` ```plot ` block, a line each:

```
title The parabola
x: -1 .. 6
y = x^2 - 5x + 6 : f
g(x) = sin 2x
x = 2 : an upright line
points: (2, 0) (3, 0) : roots
xlabel: t, s
```

Formulas as they're written by hand or in code: `2x`, `(x+1)(x-1)`,
`x^2` or `x²`, `sin x`, `sqrt`, `exp`, `ln`, `log10`, `abs`, `pi`. The x
range is −10 to 10 if it's not said; the y range as far as the curves
go (a tan's poles left out) unless `y: -2 .. 2` says. Round numbers on
the axes, a grid, the zero lines darker; each curve its own dashes,
named in a legend.

A notebook that still has the name it was made with (its date and
time) is named by its first question: the model gives a title of one
to three words with its answer, in the language it was asked in, and
the notebook (its file, its exports) is renamed so, in its folder
("named it: …" says so; a number after it if that name's taken). One
named by hand keeps its name.

What it's sent: the picture, and the talk so far as text: each
question as it read it and its answer, the last six, from all the
notebook's pages. The talk is kept on the answers' strokes in the
`.xopp`, so it's there when the notebook is opened again, and an
answer undone (one step; undo while it's still writing stops it, and
takes back what it wrote) or erased is gone from it too (an answer in
a font is text, which the eraser leaves, as for any text: select it
and delete, or undo). Nothing else
of the notebook goes.

Its settings, under the notebook's:

- **AI voice (this notebook)**: how it answers: **a diary, as Tom
  Riddle's**, **a tutor** (works problems step by step, checks yours),
  **a helper** (plain), **brief** (a line at most), **a language
  partner**, **a translator**, **a programmer**, or one of yours
  (**yours: ...**, saved from the new notebook's wizard).
- **AI prompt (this notebook)**: what that voice tells the model, in
  full.
- **AI instructions (written)**: write one to it, by hand: "from now
  on, answer in French", "call me Harry", "be my Latin teacher". It
  says it will, and keeps it on that answer: shown here, and gone if
  that answer's undone or erased.
- **AI model**: a tap lists them: the models that read handwriting on
  OpenCode Go, each with what a question cost and how long it took
  when tried (on 2026-11-11), or **as ai.conf says**.
- **AI font (this notebook)**: a tap lists them: **Gentium Book**
  (first: Latin, Cyrillic and Greek, polytonic too, in all four
  styles; it comes with hellobro), Go, the fonts the books are read in
  (the Kobo's own: Georgia, Amasis and the rest), or **handwriting**
  (the Hershey script, drawn as strokes; plain text only).
- **AI answers**: **Markdown** (code, formulas, tables, diagrams, as above) or
  **plain text**.
- **AI letters' size**: medium (15 pt), small, large, huge.
- **AI asks after a pause**: when the pen rests 3 s (or 2, 5, 10), or
  **off**: only taps ask.
- **AI asks on taps**: **three quick taps**, **two quick taps**, or
  **off**: only the pause asks.
- **AI usage**: what this Kobo's questions to the model cost, over 5
  hours, a week and 30 days, as a share of the Go plan's limits for it
  (a fifth of the month's in 5 hours, half in a week); the line under
  the settings says it in dollars. OpenCode counts it with each answer;
  each is a line in `ai-usage.tsv` in the app's folder.

The model is reached over [OpenCode Go](https://opencode.ai)'s API
with your key, from `ai.conf` in the app's folder
(`/mnt/onboard/.adds/hellobro/`), read on each question:

```
key = YOUR_KEY
model = claude-haiku-5-5   # for "as ai.conf says"
```

`base = URL` points it at another API that speaks Anthropic's messages
or OpenAI's chat completions (models whose names start with `claude-`,
`qwen3.` or `minimax-m3` go by Anthropic's, the rest by OpenAI's;
`protocol = anthropic` or `openai` says which). Without Wi-Fi it says
why, and waits for the next thing written.

### ":" commands in a notebook

The same line as a book's ([":" commands in a book](#-commands-in-a-book)),
and **?** its sheet of keys and commands too.

| Command | |
|---|---|
| `:3`, `:p 3` | page 3 |
| `:new`, `:copy`, `:delete`, `:clear` | a page after this one, a copy of it, this one gone, cleared (all undone with Ctrl+Z) |
| `:pen`, `:eraser`, `:select`, `:text`, `:u`, `:redo` | the tools; undo, redo |
| `:paper lines` | plain, lines, squares, dots, or one of them `-narrow` |
| `:pagesize SIZE` | new pages' size (screen, landscape, a4, a5, letter, square, or W x H in mm), the blank page shown's too |
| `:settings` | the notebook's settings |
| `:mark`, `:link`, `:back` | what's taken marked; the marks, to put a link to one; back where a link was followed from |
| `:o` | the pages' view, on or off |
| `:w`, `:export [pdf\|png]` | written now; the export (asks, unless said) |
| `:e NAME` | another notebook here: its name, or the one it begins; none so named, a new one beside this |
| `:rename NAME`, `:mv FOLDER` | renamed; moved to the folder (made if need be; `:mv /` the top) |
| `:ls`, `:q` | All Notebooks; close the window (`:wq` too: it's written anyway) |

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
  headset's, a keyboard's) set the player's volume. A tap on what plays,
  in the bar, plays or pauses. They go to the player that's playing, or
  else one that's paused.
- What it plays shows in the bar, and in light
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

- Wi-Fi: tap a network (or Enter on it) to open it: its buttons come
  under it, `connect` or `disconnect`, and `forget` for a saved one. A
  new secured one asks for its password (`show` reveals it: tap it, or
  Tab to it and Enter); the answer done, the keys are back on the button.
  "another network…" (or n) joins a hidden one by name. The address comes
  from the DHCP client already running (Kobo's), or one started for it. Networks are saved
  when wpa_supplicant's file allows it. The switch at the top (or o) unloads
  and loads the driver and powers the chip, as Kobo does; s scans, f forgets
  the one open.
  Turning it on starts wpa_supplicant and the DHCP client the way Kobo ran
  them, as seen while Wi-Fi was on (kept in `wifi-recipe`). `details` (or i)
  shows them: the network, access point, band and channel,
  signal, address, gateway, DNS, the interface and its MAC; c copies
  them, Esc closes.
- Bluetooth: `scan` (s) looks for 40 s; a device opens as a network
  does, with `pair`, `connect` or `disconnect`, and `forget`. A new one pairs (with a code to
  confirm, or to type on a keyboard, shown in the window), then gets trusted
  and connected. Paired ones connect, disconnect, and are forgotten with f.
  Pairing goes through hellobro's own agent while Nickel is frozen.
  Kobo turns the Bluetooth chip itself off, not only the adapter; turning
  it on powers the chip (rfkill), attaches it (`rtk_hciattach`, the
  Realtek UART chip), and starts bluetoothd and bluealsa if they aren't
  running, as [bluetooth.koplugin](https://github.com/Tharavol/bluetooth.koplugin)
  does on the Sage; off undoes it.
- The Settings window: a setting is a dropdown of its values: tap it
  (or Enter) and pick one; under each, a line says what it's for. ↑ ↓
  go through them, the list following. A setting whose values go in
  order (**Text size**, an AI answer's letters) is a slider: drag along
  it (it's picked when the finger lifts), or ← → on it. The notebooks'
  settings and the AI notebooks' (the model, the answers, their
  letters, when it asks, **Your prompts**) are here too, as well as in
  a notebook's own. They're kept in `settings`.
- When hellobro quits, the radios' power goes back to how Kobo had it,
  since Nickel wakes up thinking so; networks and devices added stay.
  "Wi-Fi and Bluetooth at start", turned on (and not in airplane mode,
  the menu's), has hellobro turn both on as it starts and leave them on when it quits (or restarts into a new
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

## Typing into another device

Another e-reader running hellobro, on the same Wi-Fi with sharing on,
is a tile in the launcher after the saved hosts: **Kobo Libra**,
"nearby: type into it", numbered after them. Picked (its number, or a tap), this keyboard
types there: a window logs in to it as `hellobro-keys`, and what's typed
in it goes to a keyboard made there (uinput), which that hellobro takes
for any other. While the window is in focus every key goes there,
hellobro's own too, as grabbed (Mod+Shift+G gives them back here, and
again to it); the bar says `typing into …`. Ctrl+] ends it.

The first time, the other device asks on its screen whether this one
may type there (allow / no). A keyboard there can do what any can, a
shell included, so it's asked with care: Enter says no, the keys don't
answer for 2 s (a tap does), an address told no isn't asked again for
10 minutes, and the device itself (a program in its Alpine) is never
asked. The launcher lists 16 devices at most. Allowed, the key is kept there for typing
only, `command="hellobro-keys"`: no shell, no files. Settings, Sharing
there lists it, to forget. Each one says it's there over mDNS
(`_hellobro._tcp`); the launcher looks again each time it's drawn,
every 20 s at most.

A device is known by its key, not its name or address: each says its
server's key in its mDNS answer, and the window logs in only to the one
that has it. Typed into once, it's remembered by that key (in
`hellobro.json`), and its tile says so; a new one says its key's end
("nearby, new: key …3fQx9kLm"), and one with the name of a device
typed into before but another key says **NOT the Kobo Libra typed into
before**. The first time, this window says the end of this device's
key, and the other's question shows the same: if they match, it's this
one asking.

A computer types into hellobro the same way: `ssh -t -p 2022
hellobro-keys@kobo.local`, in kitty, foot or another terminal that
speaks kitty's keyboard protocol for Super and the releases (any other
sends no Super: the Mod key is best Alt then).

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
Mod+] / Mod+[, and Mod+\ for on/off; in brotools Ctrl+Alt+Up/Down
and Ctrl+Alt+L). Fullscreen (Mod+F in
hellobro): swipe down from the top edge to leave it (hellobro: two
fingers, or one but over a book or a notebook, where it shows or hides
the bar); the left edge's
light and the right edge's menu wait till then. The exit
button leaves, and so does Esc in brotools, Mod+Shift+E in hellobro (where Esc
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

A notebook's ink goes in the panel's pen mode (DU, sent as it's written,
without waiting for the pieces before). The strokes' cleanup, the paper's
grays and the smooth edges back under them, waits till the pen has
rested a second and left the air over the page (4 s at most), so a word
written goes without a grayscale update between its letters, nor one
holding up the next. While the pen writes, other grayscale updates
(the bar's stats, the clock) wait too, till it rests a moment or a
finger or a key comes. The log's `blits:` line says how long ink took to
reach the panel; a slow update says where its time went (`fbink: …
took`).

Each update powers the panel, and the battery goes as fast as they
come: a terminal left to change by itself (a spinner while a program
works, output read rather than typed into) kept the panel at it a few
times a second, and took a fifth of the battery an hour. So once a
terminal hasn't been typed into for 5 s (nor, the one in focus,
anything touched), it goes on screen every 2 s, in grayscale at once
so no cleanup follows; a key brings it back at once. Settings, Power,
**A terminal changing by itself**: every 2, 5 or 10 s, or at once
always. The fast updates' cleanup waits till the output stops, not
every 4 s while it doesn't (**… its quick updates cleaned up**).

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
| `hellobro.json` | the saved SSH hosts, and how it was left: the windows (each terminal's font size with it), the light, the font size for new terminals |
| `ssh.conf.old` | the hosts as kept before, taken in once |
| `keys.conf` | the keys ([Keys](#keys)) |
| `clipboard`, `clipboard.png`, `clipboard.xopp` | what's copied, the picture of a PDF's part copied, and ink copied from a notebook ([Copy and paste](#copy-and-paste), [Notebooks](#notebooks)) |
| `notebooks/NAME.hbnote` | the first notebooks, till they're opened: then they're `Notebooks/NAME.xopp` on the Kobo's storage ([Notebooks](#notebooks)) |
| `wifi-recipe` | how Kobo starts Wi-Fi, as seen |
| `touch-calibration.conf` | the touch calibration (shared with brotools) |
| `hellobro.log` | the log (`make push-log`) |
| `hellobro.old`, `hellobro.bad` | the build before an update, a build that failed its trial |
