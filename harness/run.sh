#!/bin/sh
# Runs hellobro or brotools on the PC under qemu-arm, as if on the Kobo: a fake FBInk
# (fakefbink.py) keeps the screen in OUT/screen and snapshots big updates as
# PNGs, a feed plays touch and keyboard input into FIFOs, and fake devices,
# Nickel settings and frontlight stand in for the rest. The app's log goes
# to OUT/stdout.txt, its files (ssh.conf, keys, known_hosts) to OUT/app.
#
# usage: harness/run.sh OUT ROTA [FEED]
#   ROTA  0-3, how the "device" is held (1, 3: landscape)
#   FEED  a script in harness/feeds, without .py (default: demo); its
#         "# app:" line says which app it's for
# env: APP (the binary, default the feed's app, built in the repo),
#      TIMEOUT (seconds, default 170),
#      INTERFACE (for the Net tab, default: the one with the default route)
#
# The ssh, fullscreen, stress, connect, flash, zoom and fontswipe feeds want
# the server running: (cd server && go run .)
set -eu
H=$(cd "$(dirname "$0")" && pwd); OUT=$1; ROTA=$2; FEED=${3:-demo}
NAME=$(sed -n 's/^# app: //p' "$H/feeds/$FEED.py" | head -n 1)
APP=${APP:-$H/../${NAME:-hellobro}}
NAME=$(basename "$APP")
case $NAME in brotools*) TABS=7 ;; *) TABS=2 ;; esac
IFACE=${INTERFACE:-$(ip route show default 2>/dev/null | awk '{print $5; exit}')}
rm -rf "$OUT"; mkdir -p "$OUT/screen" "$OUT/input" "$OUT/app"
cp "$APP" "$OUT/app/$NAME"; cp "$H/ssh.conf" "$OUT/app/"
touch "$OUT/ntx_io"; mkfifo "$OUT/input/event2" "$OUT/input/event4"
TABS=$TABS python3 -I "$H/feeds/$FEED.py" "$OUT/input" "$ROTA" &
FEEDPID=$!
FAKE_SNAP_AREA=40000 FAKE_SCREEN_DIR=$OUT/screen FAKE_GYRO=$ROTA HELLOBRO_FBINK=$H/fakefbink.py \
	HELLOBRO_INPUT_DIR=$OUT/input HELLOBRO_DEVICES=$H/devices INTERFACE=$IFACE HELLOBRO_MUSIC_DIR=$H/music \
	HELLOBRO_NTX_IO=$OUT/ntx_io HELLOBRO_NICKEL_CONF=$H/nickel.conf \
	timeout "${TIMEOUT:-170}" qemu-arm "$OUT/app/$NAME" > "$OUT/stdout.txt" 2>&1 || echo "app exit: $?"
wait $FEEDPID || true
echo "$(ls "$OUT/screen" | grep -c 'after.png') snapshots in $OUT/screen"
