#!/bin/sh
# The README's terminal screenshots, from a throwaway container instead
# of this PC: user bro on host pc (the PC's own user, host name and
# addresses stay out of the pictures). It runs sshd (port 2290) with
# tmux, btop and ncmpcpp, mpd playing MUSIC with mpd-mpris on a session
# bus in OUT/bus, and the test server (port 2291). A new key for it goes
# in OUT/extra-pc and OUT/extra-wish, with an ssh.conf each.
#
# usage: harness/shots/run.sh OUT MUSIC     then, from the repo:
#   B=HELLOBRO_MPRIS_BUS=unix:path=OUT/bus/bus
#   env $B APP_EXTRA=OUT/extra-pc harness/run.sh OUT/tmux 1 tmux
#   env $B APP_EXTRA=OUT/extra-pc harness/run.sh OUT/btop 1 btop
#   env $B APP_EXTRA=OUT/extra-pc harness/run.sh OUT/ncmpcpp 0 ncmpcpp
#   env $B APP_EXTRA=OUT/extra-wish harness/run.sh OUT/ssh 0 ssh
#   podman rm -f hb-shots      # done
set -eu
H=$(cd "$(dirname "$0")" && pwd); OUT=$(mkdir -p "$1" && cd "$1" && pwd); MUSIC=$(cd "$2" && pwd)
mkdir -p "$OUT/img" "$OUT/bus" "$OUT/extra-pc" "$OUT/extra-wish"
cp "$H/Containerfile" "$H/entry.sh" "$H/mpd.conf" "$H/bashrc" "$OUT/img/"
(cd "$H/../../server" && CGO_ENABLED=0 go build -o "$OUT/img/hellobro-server" .)
[ -f "$OUT/extra-pc/id_ed25519" ] || ssh-keygen -q -t ed25519 -N "" -C hellobro@kobo -f "$OUT/extra-pc/id_ed25519"
cp "$OUT/extra-pc/id_ed25519" "$OUT/extra-pc/id_ed25519.pub" "$OUT/extra-wish/"
cp "$OUT/extra-pc/id_ed25519.pub" "$OUT/img/authorized_keys"
printf 'pc  bro@127.0.0.1:2290\n' > "$OUT/extra-pc/ssh.conf"
printf 'wish-server  bro@127.0.0.1:2291\n' > "$OUT/extra-wish/ssh.conf"
podman build -q --build-arg UID="$(id -u)" -t hb-shots "$OUT/img" >/dev/null
podman network exists hbshots || podman network create hbshots >/dev/null # its own: pasta would show the PC's address
podman rm -f hb-shots >/dev/null 2>&1 || true
# keep-id: bro is our uid in there, so the session bus takes us
podman run -d --name hb-shots --network hbshots --hostname pc --userns=keep-id --user 0 \
	-p 127.0.0.1:2290:22 -p 127.0.0.1:2291:2222 -v "$OUT/bus:/run/bus:Z" -v "$MUSIC:/music:ro,Z" hb-shots >/dev/null
sleep 3
git -C "$H/../.." bundle create -q "$OUT/hb.bundle" main
podman cp "$OUT/hb.bundle" hb-shots:/tmp/hb.bundle
podman exec -u bro hb-shots sh -c '
mpc -q update --wait; mpc -q add /; mpc -q play 5
cd /home/bro && git clone -q -b main /tmp/hb.bundle hellobro 2>/dev/null
printf "syntax on\nset number nowrap tabstop=4\nset laststatus=2\n" > .vimrc
tmux new-session -d -s main -n code -x 113 -y 36 -c /home/bro/hellobro "vim +32 cmd/hellobro/kbd.go"
tmux split-window -v -t main:code -l 14 -c /home/bro/hellobro "sh -c \"git log --format=\\\"%h %<(44,trunc)%s\\\" -12; exec bash\""
tmux split-window -h -t main:code.1 -c /home/bro/hellobro htop
tmux new-window -d -t main -n shell -c /home/bro/hellobro
tmux select-pane -t main:code.0'
echo "hb-shots up: APP_EXTRA=$OUT/extra-pc (or extra-wish), HELLOBRO_MPRIS_BUS=unix:path=$OUT/bus/bus"
