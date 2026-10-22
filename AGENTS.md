# AGENTS.md

How to work on this repo as a coding agent. People start at the
[README](README.md). The details are in [docs/](docs/).

## What this is

Go apps for a Kobo Elipsa (N604, sunxi B300, firmware 4.38). While they
run they freeze Nickel and draw an e-ink UI through FBInk.

- `cmd/hellobro/`: the main app (terminal, Alpine, settings, files, relay,
  the Linux players' remote control).
  Its keys are commands (`commands.go`, changed in `keys.conf`); which
  tabs show is `tabs.go`
- `cmd/brotools/`: diagnostics, only installed when asked for
- `kobo/`: screen, input, frame, sleep: what the apps share
- `audio/`, `radio/`, `share/`: Bluetooth audio, Wi-Fi/BT, the SSH/SFTP server
- `server/`: a test SSH server (wish), its own Go module
- `harness/`: runs the ARM binary under qemu-arm with fake hardware

## Rules

- **Install hellobro only.** brotools goes on the Kobo only when the user
  asks for it (`make install-tools`). Never by default.
- **Deploy over Wi-Fi** with `make push`, not USB. If the Kobo can't be
  reached, say what to check (hellobro running, file sharing on, Wi-Fi,
  asleep?). Don't fall back to USB.
- **Keys stay private.** Never copy or commit a private key: not
  `id_ed25519`, not `server/.ssh/`. Only public keys go to the Kobo or a
  server.
- **The VPS address is the user's to fill in.** Leave `VPS` /
  `VPS_ADDRESS` placeholders in docs and units.
- **Careful with `rm`.** Absolute paths only, no relative globs.
- **Don't run `make -B`.** It rebuilds the tracked `bin/fbink` and
  `third_party/fbink/libfbink.a` and leaves the version `-dirty`. If it
  happens: `git checkout -- bin/fbink third_party/fbink/libfbink.a`.
- Work on `main`, small commits, push when a piece works and is tested.

## Build and test

```sh
make test         # vet + unit tests, CGO_ENABLED=0 (cgo test builds fail: libfbink.a is ARM)
make hellobro     # static ARM build in podman (tools/Containerfile); stamps main.version
```

Run single tests the same way: `CGO_ENABLED=0 go test ./cmd/hellobro -run SSH`.

## See it work: the harness

The harness runs the real ARM binary under `qemu-arm`, with a fake
FBInk, input FIFOs, stand-in Wi-Fi/Bluetooth and pretend sleep:

```sh
harness/run.sh OUT ROTA FEED          # ROTA 0-3 (1, 3 landscape); FEED from harness/feeds
MPRIS=1 harness/run.sh OUT 0 doze      # a session bus with a stand-in mpv (harness/mpris)
harness/relay.sh OUT                  # the relay, sshd playing the VPS
```

- Snapshots: `OUT/screen/snap-NNNN-b-after.png`. Look at them, don't guess.
- Log: `OUT/stdout.txt`. App files: `OUT/app`.
- `APP_EXTRA=dir` copies files into the app's dir first (settings,
  relay.conf, keys, keys.conf). `SHARE_PORT` moves file sharing off 2022.
- The harness shows all five tabs, as the feeds' Ctrl+Alt+N count on
  (the app hides Shell by default). `HB_TABS="ssh -shell settings
  files"` sets the tabs file as you like: `harness/feeds/tabs.py` wants
  that one.
- Feeds `ssh`, `fullscreen`, `stress`, `connect`, `flash`, `zoom`,
  `panes`, `foldkeys`, `fontswipe` and `relay` need the test server:
  `cd server && go run .` (port 2222).
- Put outputs in your scratch directory, not the repo.

A UI change isn't done until a harness snapshot shows it, in portrait and
in landscape when the layout is involved.

## On the device

```sh
make push         # scp hellobro.new + launch.sh, then hellobro-update (restarts at once)
make push-log     # the device log
make push-rollback
make push-profile # a CPU profile (PROFILE_SECS, 30) to cpu.pprof; Settings, CPU profiles on first
ssh -p 2022 kobo.local <command>      # a shell on the Kobo, as root
```

- `KOBO_HOST` (default `kobo.local`; or the IP, or a `~/.ssh/config` Host
  such as `kobo-afar`, whose Port it takes), `KOBO_PORT`,
  `KOBO_JUMP` (through the relay, see [docs/relay.md](docs/relay.md)).
- Off the PC's LAN (a phone's hotspot, say) `kobo.local` doesn't
  resolve and its last IP has no route: the Kobo can reach out, nothing
  can reach in. Go through the relay: `make push KOBO_HOST=kobo-afar`.
- **Never run the hellobro binary by hand** on the Kobo, not even
  `--help`: it starts a second app on the same screen and keyboard. New
  builds go through `hellobro-update`. If one slipped out: `ps | grep
  hellobro`, and kill the one not under `launch.sh`.
- The system log is `logread` (busybox): wpa_supplicant and dhcpcd write
  there, not to `hellobro.log`.
- Updates are on trial: a build that doesn't tick within 45 s, or
  crashes, rolls back to `hellobro.old` by itself. Still, read the log
  after every push.
- The app's files live in `/mnt/onboard/.adds/hellobro/`.
- If hellobro hangs, Nickel stays frozen and the user is stuck. Startup
  must never block on anything slow (D-Bus, radios, network): do it in a
  goroutine.

## Hard-won facts

- Nickel is SIGSTOPped while we run (`launch.sh`) and holds the
  EVIOCGRAB on the power button (event0), so the button is read from
  `/sys/kernel/debug/gpio` (`kobo/powerkey.go`).
- Wi-Fi and Bluetooth share one RTL8821CS chip. `ntx_io 208 0` (chip
  power off) kills Bluetooth too: with BT on, Wi-Fi only goes `Down()`.
- hellobro has no player: the music is a Linux player's in an Alpine
  session, reached over MPRIS (`mpris.go`). Each session's jail runs a
  session bus at `/run/user/0/bus` (once `dbus` is installed there), which
  hellobro reaches as `/proc/PID/root/run/user/0/bus` of the jail's first
  process. Nothing polls the players: their signals, and a look every
  5 s for new buses. The Kobo's own (system) bus is left alone.
- Nickel registers a BlueZ media player, so with Nickel frozen the
  headphones' buttons go to it and are lost (the Kobo's dbus-daemon,
  1.6.30, can't be eavesdropped on). With Nickel closed BlueZ sends them
  as keys (AVRCP's uinput device), and they reach the player.
- e-ink: never overlap an update with one still in flight. Small changes
  use DU, then a GL16 cleanup; a GC16 flash now and then. DU blits are
  rounded to black and white (`blackWhite`): sent grays, DU leaves them
  faint and the panel counts them drawn, so a cleanup with the same
  pixels changes nothing on screen.
- The Kobo's loopback (`lo`) is down: nothing on the Kobo can reach
  127.0.0.1. The relay hands its connections to the share server in
  process (`share.Server.ServeConn`). The harness can't show this: the
  PC's loopback is up.
- sshd's `permitopen` matches the host name as asked: `127.0.0.1` and
  `localhost` are different.
- Kobo's dhcpcd is 6.6.6, started by Nickel (`dhcpcd -d -z wlan0`). It
  follows a new network by itself (carrier lost: address gone; carrier
  back: a new lease). `dhcpcd -n` (rebind) kills it, and then nothing
  asks for an address: leave a running one alone (`radio.renewIP`).
- sshd's `ClientAliveInterval` only holds for connections made after it
  was set. An older dead connection keeps its `-R` port for good: the
  relay is refused (`tcpip-forward request denied`) till its `sshd`
  process is killed on the VPS.
- The battery's gauge (ricoh61x) gives `current_now` in tenths of a mA
  (2708: 271 mA), not µA: its `time_to_empty_now` times that is a full
  battery. The harness has a stand-in battery (`HELLOBRO_BATTERY`).
- Keys dropped while asleep or dozing must not include the modifiers:
  Ctrl+Alt+S's Ctrl and Alt come up after it dozed, and missed, they
  stayed down (`tv.mods`), so every key after waking was Ctrl+Alt+key.
  `onKey` hands them to the Program anyway (`IsModifier`).
- The Alpine jail (`alpine.go`, `alpineinit.go`): no user namespaces in
  this kernel, so it's root with capabilities dropped (FAT: only uid 0
  writes `/mnt/onboard`). Mount or write nothing in the jail before
  `pivot_root`: its absolute symlinks would be the Kobo's paths then.
  Keep `DAC_READ_SEARCH` out (`open_by_handle_at` goes around the
  read-only `.adds`). Test escapes with `ssh -tt -p 2022 kobo.local
  'hellobro-alpine COMMAND'`; never `echo b` at sysrq-trigger to check it.
- A session off screen reads a trickle of output in batches
  (`termUnseen`): on the Kobo, waking up for each small read costs more
  than parsing it. Floods are still read at once, so the writer never
  blocks on a full pty.
- Closed Nickel (`nickel=closed`): `bluetoothd` and `bluealsa` are
  D-Bus activated (`/usr/share/dbus-1/system-services`), so killing them
  is safe, but `rtk_hciattach` is Nickel's child. Wi-Fi's daemons are
  init's. `/tmp/nickel-hardware-status` must go while Nickel's away
  (writers hang on the FIFO) and come back before it starts.
- `launch.sh` may be running while it's replaced: write a new file and
  `mv` it over the old one, never write into it.

## Style

- gofmt. Match the surrounding code: short names, plain comments written
  as prose ("we", "it"), about why more than what.
- A new key is a command in `commands.go` (with its default keys), not
  a case in a Key function: then keys.conf, the switcher and the hints
  know of it.
- A page's panels are placed with `tile` (`Layout.Arrange`, or
  fullscreen the one in focus alone) and made with `pagePanel` (foldable,
  listed in `fold.go`). A folded panel's inner rects are
  zeroed in its layout and its draw stops after `DrawPanel`, so nothing
  of it is drawn or tapped.
- Each terminal session has its own panel and cells (`termPane`: grid,
  cols, rows, what's drawn); `a.term()` is the one in focus, which the
  keys and the selection go to. `layoutSessions` places a tab's
  sessions; the emulator and the remote are resized only when a pane
  that shows changes size (`t.size`). Ctrl+Tab moves the focus on any
  page through `stepPanel`; the panel in focus gets `focusMark`'s ►.
- A new button or list on a page goes into `controls()` (`focus.go`), so
  Tab reaches it; Enter on a button is a tap on it. The focus ring is
  stamped at blit time (`kobo/ring.go`), never drawn into the canvas.
- Tests sit next to the code (`*_test.go`). Parsers and logic get table
  tests.
- User-facing text is short and lower-key, like the rest of the UI.
- New behavior goes into [docs/usage.md](docs/usage.md) (or the relevant
  doc), and the README if it's a headline feature.
