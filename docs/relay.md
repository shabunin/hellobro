# From afar: a relay on a VPS

Away from home, the Kobo and the PC can't reach each other directly. A
VPS with a fixed address, that both of them reach, is the meeting point.
Both keep an SSH connection to it open, and it only carries bytes between
them, end to end encrypted: it never sees passwords, files or screens.

```
          ┌──────────────── VPS (fixed address) ────────────────┐
          │  127.0.0.1:2023 ──► the Kobo's file sharing (2022)  │
          │  127.0.0.1:2222 ──► the PC's sshd (22)              │
          └──────▲──────────────────────────────────────▲───────┘
                 │ ssh -R 2023                           │ ssh -R 2222
               Kobo                                      PC
   "home", a saved host: via the VPS        ssh -J relay@VPS -p 2023
   to 127.0.0.1:2222, the PC                root@127.0.0.1, the Kobo
```

Nothing new listens on the VPS's public side, only its own sshd. Both
forwards are on its loopback, and each key can only open the other's.

Below, `VPS` is your VPS's address and `relay` the user both ends log in
as there.

## 1. The VPS (once)

Any Linux with OpenSSH. As root:

```sh
useradd -m -s /usr/sbin/nologin relay
install -d -m 700 -o relay -g relay ~relay/.ssh
```

`~relay/.ssh/authorized_keys` (mode 600, owned by `relay`) gets a line
per key, each limited to its own forwards:

```
restrict,port-forwarding,permitlisten="127.0.0.1:2023",permitopen="127.0.0.1:2222" ssh-ed25519 AAAA… hellobro@kobo
restrict,port-forwarding,permitlisten="127.0.0.1:2222",permitopen="127.0.0.1:2023" ssh-ed25519 AAAA… me@pc
```

- The Kobo's key is `.adds/hellobro/id_ed25519.pub` on the Kobo. It's
  made on first SSH use, or when the relay is turned on. At home:
  `ssh -p 2022 kobo.local cat /mnt/onboard/.adds/hellobro/id_ed25519.pub`.
  Or from the Kobo itself, in an SSH session to the VPS (as root, say),
  in an editor on that file: type the line's start, `restrict,…` up to
  the space before the key, then "Paste the Kobo's SSH key" in the
  switcher (Meta+D) types the key in.
- The PC's key is its `~/.ssh/id_ed25519.pub` (`ssh-keygen -t ed25519`
  if there's none).

In `/etc/ssh/sshd_config`, then `systemctl reload sshd`:

```
ClientAliveInterval 30
ClientAliveCountMax 3
```

With these, a forward left behind by a connection that died (Wi-Fi gone)
is freed within a minute and a half, so the next connection can take it.

## 2. The PC (stays connected)

1. Its sshd on: `sudo systemctl enable --now sshd`.
2. The Kobo's key in `~/.ssh/authorized_keys`. Or type the password the
   first time: hellobro offers to install its key then.
3. The tunnel, kept open by a systemd user unit. Put the VPS's address
   in [`tools/relay/hellobro-tunnel.service`](../tools/relay/hellobro-tunnel.service), then:

   ```sh
   cp tools/relay/hellobro-tunnel.service ~/.config/systemd/user/
   systemctl --user daemon-reload
   systemctl --user enable --now hellobro-tunnel
   loginctl enable-linger $USER    # up from boot, logged in or not
   ```

It reconnects by itself (`Restart=always`, and `ServerAliveInterval`
notices a dead connection). `systemctl --user status hellobro-tunnel`
shows how it is, `journalctl --user -u hellobro-tunnel` why it isn't.

## 3. The Kobo

Two files next to hellobro, in `.adds/hellobro/` (send them over SFTP,
or edit them in a shell):

- a saved host for the PC (type it at a launcher's prompt, or a
  saved host's **edit**; or an `ssh.conf` with this line, which
  hellobro takes in at start):

  ```
  home  bobalus@127.0.0.1:2222  via relay@VPS  tmux new -A -s kobo
  ```

- `relay.conf`, where the Kobo keeps its own way in open (an example is
  written the first time the relay is turned on):

  ```
  relay@VPS  2023
  ```

Then in Settings turn **Relay** on. File sharing has to be on too.
Settings, Sharing shows whether it's up. It reconnects by itself: after 5 s, then
up to a minute between tries.

While the relay is on, the Kobo stays reachable, and that costs battery:

- hellobro doesn't auto-exit.

A deep sleep (the power button) drops the relay till the Kobo wakes.
So does sleeping when idle (Settings, Sleep: on battery after 30 min
untouched, by default), relay or not; a toast says so a minute before.
To keep it reachable all night, leave it on its charger ("plugged in"
is never, by default), or set "on battery" to never.
hellobro has to be running for any of it.

## Using it

**On the Kobo**, the PC: `home` in the switcher (Meta+D) or a
launcher.

**On the PC**, the Kobo (the pairing from home still holds):

```sh
ssh -J relay@VPS -p 2023 root@127.0.0.1
make push KOBO_HOST=127.0.0.1 KOBO_PORT=2023 KOBO_JUMP=relay@VPS
```

Write `127.0.0.1`, not `localhost`: `permitopen` only allows the name
exactly as given. Or, in `~/.ssh/config`:

```
Host kobo-afar
    HostName 127.0.0.1
    Port 2023
    User root
    ProxyJump relay@VPS
```

then `ssh kobo-afar`, `scp -r Album kobo-afar:Music/`,
`make push KOBO_HOST=kobo-afar`, and so on.

### Off your LAN

On another network (a café's, a phone's hotspot) the PC can't reach the
Kobo straight: `kobo.local` doesn't resolve, its old address has no
route, and behind a hotspot it's out of reach anyway. The Kobo can still
reach out: `home` works, and so does the relay, so from the PC go
through `kobo-afar` (or `-J`) as above. When the Kobo changes networks
the relay notices within a minute (keepalives) and comes back by itself.

Updates from afar are safe: a new build is on trial until it's running
with file sharing up. One that hangs for 45 s, or crashes, is put aside
as `hellobro.bad` and the build before comes back by itself, so a bad
push can't leave the Kobo out of reach.

## When it doesn't work

| What you see | Look at |
| --- | --- |
| Sharing: `Relay down (… permission denied …)` | The Kobo's key isn't in `~relay/.ssh/authorized_keys`, or the file's mode/owner is wrong (sshd's log: `journalctl -u sshd` on the VPS). |
| `Relay down (listening on the server's port 2023: …)` | A dead connection still holds the port. Wait 90 s (`ClientAliveInterval`), or check `ss -ltnp` on the VPS. A connection made before the keepalives were set up is never let go: kill its `sshd: relay` process (the one as old as the Kobo's last good relay). |
| PC: `ssh kobo-afar` hangs (`timed out during banner exchange`) | The VPS's port 2023 is held by a dead connection, so ours can't take it (Sharing says `Relay down`). As above. On the VPS, `ps -o pid,lstart,args -u relay \| grep sshd` shows the sessions with their start times. |
| PC: `stdio forwarding failed` | Nothing listens on the VPS's 2023: the relay is off or down on the Kobo, or the port asked for isn't 2023. |
| `Relay: say where in relay.conf` | `relay.conf` is still the example. |
| PC: `administratively prohibited: open failed` | `localhost` instead of `127.0.0.1`, or a port that doesn't match `permitopen`. |
| Kobo: `home` says `through VPS to 127.0.0.1:2222: … refused` | The PC's tunnel is down: `systemctl --user status hellobro-tunnel`. |
| Kobo: `host key CHANGED` | The VPS was reinstalled: remove its line from `.adds/hellobro/known_hosts`. |

## Testing it on the PC

`harness/relay.sh OUT` plays every part on one machine: OpenSSH's sshd
as the VPS (on 127.0.0.1:2299, its keys locked down as above), a tunnel
from it to `hellobro-server`, and the Kobo under the harness. It goes
both ways through the "VPS" and does a full update with restart. It
needs the test server running: `cd server && go run .`.
