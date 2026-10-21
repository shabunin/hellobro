# AGENTS.md

How to work on this repo as a coding agent. People start at the
[README](README.md). The details are in [docs/](docs/).

## What this is

Go apps for a Kobo Elipsa (N604, sunxi B300, firmware 4.38). While they
run they freeze Nickel and draw an e-ink UI through FBInk.

- `cmd/hellobro/`: the main app (terminal, music, settings, files, relay).
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
MUSIC=dir TIMEOUT=60 harness/run.sh OUT 0 playlists
harness/relay.sh OUT                  # the relay, sshd playing the VPS
```

- Snapshots: `OUT/screen/snap-NNNN-b-after.png`. Look at them, don't guess.
- Log: `OUT/stdout.txt`. App files: `OUT/app`.
- `APP_EXTRA=dir` copies files into the app's dir first (settings,
  relay.conf, keys, keys.conf). `SHARE_PORT` moves file sharing off 2022.
- The harness shows all five tabs, as the feeds' Ctrl+Alt+N count on
  (the app hides Shell by default). `HB_TABS="ssh -shell music settings
  files"` sets the tabs file as you like: `harness/feeds/tabs.py` wants
  that one.
- Feeds `ssh`, `fullscreen`, `stress`, `connect`, `flash`, `zoom`,
  `fontswipe` and `relay` need the test server: `cd server && go run .`
  (port 2222).
- Put outputs in your scratch directory, not the repo.

A UI change isn't done until a harness snapshot shows it, in portrait and
in landscape when the layout is involved.

## On the device

```sh
make push         # scp hellobro.new + launch.sh, then hellobro-update (restarts at once)
make push-log     # the device log
make push-rollback
ssh -p 2022 kobo.local <command>      # a shell on the Kobo, as root
```

- `KOBO_HOST` (default `kobo.local`; or the IP), `KOBO_PORT`,
  `KOBO_JUMP` (through the relay, see [docs/relay.md](docs/relay.md)).
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
- Nickel registers a BlueZ media player, so headphone buttons arrive as
  D-Bus calls to it, not as keys. `audio/avrcp.go` watches for them as a
  bus monitor, but the Kobo's dbus-daemon is 1.6.30: no `BecomeMonitor`
  (it's 1.9.10+), and its policy forbids `eavesdrop='true'`, even for
  root. BlueZ 5.63 also always gives a connection the *first* registered
  player (Nickel's), so a player of our own never gets presses. The user
  chose to leave it (2026-10-21). With godbus `Eavesdrop`, a method reply
  goes to the eavesdrop channel: `Call` blocks forever, so use `Go`.
- e-ink: never overlap an update with one still in flight. Small changes
  use DU, then a GL16 cleanup; a GC16 flash now and then.
- sshd's `permitopen` matches the host name as asked: `127.0.0.1` and
  `localhost` are different.
- `launch.sh` may be running while it's replaced: write a new file and
  `mv` it over the old one, never write into it.

## Style

- gofmt. Match the surrounding code: short names, plain comments written
  as prose ("we", "it"), about why more than what.
- A new key is a command in `commands.go` (with its default keys), not
  a case in a Key function: then keys.conf, the switcher and the hints
  know of it.
- Tests sit next to the code (`*_test.go`). Parsers and logic get table
  tests.
- User-facing text is short and lower-key, like the rest of the UI.
- New behavior goes into [docs/usage.md](docs/usage.md) (or the relevant
  doc), and the README if it's a headline feature.
