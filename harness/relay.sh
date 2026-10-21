#!/bin/sh
# Runs the relay feed with this PC playing every part: OpenSSH's sshd as
# the VPS (127.0.0.1:2299, its keys locked down as the README says), a
# tunnel from it to hellobro-server (the computer's sshd, which must be
# running: cd server && go run .), and the Kobo under the harness.
#
# usage: harness/relay.sh OUT [ROTA]
set -eu
H=$(cd "$(dirname "$0")" && pwd); OUT=$1; ROTA=${2:-0}
V=$OUT-vps; X=$OUT-extra
rm -rf "$V" "$X"; mkdir -p "$V" "$X"
for k in host kobo pc; do ssh-keygen -q -t ed25519 -N '' -C "$k" -f "$V/$k"; done
cat >"$V/authorized_keys" <<KEYS
restrict,port-forwarding,permitopen="127.0.0.1:2298",permitlisten="127.0.0.1:2297" $(cat "$V/kobo.pub")
restrict,port-forwarding,permitlisten="127.0.0.1:2298",permitopen="127.0.0.1:2297" $(cat "$V/pc.pub")
KEYS
chmod 600 "$V/authorized_keys"
cat >"$V/sshd_config" <<CONF
Port 2299
ListenAddress 127.0.0.1
HostKey $V/host
AuthorizedKeysFile $V/authorized_keys
PidFile $V/sshd.pid
UsePAM no
StrictModes no
PasswordAuthentication no
KbdInteractiveAuthentication no
AllowTcpForwarding yes
X11Forwarding no
PermitTTY no
ClientAliveInterval 30
ClientAliveCountMax 3
CONF
/usr/sbin/sshd -D -e -f "$V/sshd_config" 2>"$V/sshd.log" &
VPS=$!
sleep 1
ssh -F /dev/null -i "$V/pc" -o IdentitiesOnly=yes -o BatchMode=yes -o StrictHostKeyChecking=accept-new \
	-o UserKnownHostsFile="$V/known_hosts" -o ExitOnForwardFailure=yes \
	-N -R 127.0.0.1:2298:127.0.0.1:2222 -p 2299 "$USER@127.0.0.1" >"$V/tunnel.log" 2>&1 </dev/null &
TUNNEL=$!
trap 'kill $TUNNEL $VPS 2>/dev/null || true' EXIT
cp "$V/kobo" "$X/id_ed25519"; cp "$V/kobo.pub" "$X/id_ed25519.pub"
cp "$V/pc.pub" "$X/authorized_keys" # paired with the Kobo
printf 'share=on\nrelay=on\n' >"$X/settings"
echo "$USER@127.0.0.1:2299  2297" >"$X/relay.conf"
echo "home  bro@127.0.0.1:2298  via $USER@127.0.0.1:2299" >"$X/ssh.conf"
APP_EXTRA=$X RELAY_VPS=$USER@127.0.0.1:2299 RELAY_PORT=2297 PC_KEY=$V/pc SHARE_PORT=2296 TIMEOUT=${TIMEOUT:-110} \
	"$H/run.sh" "$OUT" "$ROTA" relay
cat "$OUT/pc.log"
