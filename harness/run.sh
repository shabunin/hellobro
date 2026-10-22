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
#      TIMEOUT (seconds, default 170), MUSIC (brotools: where the music is,
#      default harness/music; it "plays" to a fake speaker, into OUT/pcm.raw),
#      MPRIS (1: a session bus with a stand-in player on it, harness/mpris,
#      as an Alpine session's; its log in OUT/mpris.txt),
#      INTERFACE (for the Net tab, default: the one with the default route),
#      FAKE_RADIO (default 1: stand-in Wi-Fi and Bluetooth; empty: the real ones),
#      APP_EXTRA (a directory copied into the app's, over what's there:
#      settings, relay.conf, keys, keys.conf),
#      HB_TABS (hellobro's tabs file; default all five shown, as the
#      feeds' Ctrl+Alt+N and tab taps count on; one it leaves out is
#      shown at the end, as the app does with Power)
#
# The ssh, fullscreen, stress, connect, flash, zoom and fontswipe feeds want
# the server running: (cd server && go run .); the relay feed wants
# more, see it.
set -eu
H=$(cd "$(dirname "$0")" && pwd); OUT=$1; ROTA=$2; FEED=${3:-demo}
NAME=$(sed -n 's/^# app: //p' "$H/feeds/$FEED.py" | head -n 1)
APP=${APP:-$H/../${NAME:-hellobro}}
NAME=$(basename "$APP")
HB_TABS=${HB_TABS:-ssh shell settings files power}
case " $HB_TABS " in *" power "* | *" -power "*) ;; *) HB_TABS="$HB_TABS power" ;; esac
case $NAME in brotools*) TABS=7 ;; *) TABS=$(echo "$HB_TABS" | tr ' ' '\n' | grep -vc '^-') ;; esac
IFACE=${INTERFACE:-$(ip route show default 2>/dev/null | awk '{print $5; exit}')}
rm -rf "$OUT"; mkdir -p "$OUT/screen" "$OUT/input" "$OUT/app" "$OUT/onboard/Music"
cp "$APP" "$OUT/app/$NAME"; cp "$H/ssh.conf" "$OUT/app/"
case $NAME in brotools*) ;; *) echo "$HB_TABS" > "$OUT/app/tabs" ;; esac
[ -z "${APP_EXTRA:-}" ] || cp -r "$APP_EXTRA/." "$OUT/app/"
mkdir "$OUT/battery"; for f in capacity=71 status=Discharging health=Good type=Battery current_now=2708 voltage_now=4057300 temp=310 time_to_empty_now=25800 time_to_full_now=3932100; do
	echo "${f#*=}" > "$OUT/battery/${f%%=*}"
done
touch "$OUT/ntx_io"; printf ' gpio-356 (                    |Power               ) in  hi    \n' > "$OUT/gpio"; mkfifo "$OUT/input/event2" "$OUT/input/event4"
MPRIS_ENV=""
if [ "${MPRIS:-}" = 1 ]; then
	(cd "$H/.." && go build -o "$OUT/mpris" ./harness/mpris)
	dbus-daemon --session --address="unix:path=$OUT/bus" --fork --print-pid > "$OUT/bus.pid"
	"$OUT/mpris" "unix:path=$OUT/bus" > "$OUT/mpris.txt" 2>&1 &
	MPRISPID=$!
	MPRIS_ENV="HELLOBRO_MPRIS_BUS=unix:path=$OUT/bus"
fi
TABS=$TABS python3 -I "$H/feeds/$FEED.py" "$OUT/input" "$ROTA" &
FEEDPID=$!
FAKE_SNAP_AREA=${FAKE_SNAP_AREA:-40000} FAKE_SCREEN_DIR=$OUT/screen FAKE_GYRO=$ROTA HELLOBRO_FBINK=$H/fakefbink.py \
	HELLOBRO_INPUT_DIR=$OUT/input HELLOBRO_DEVICES=$H/devices INTERFACE=$IFACE HELLOBRO_MUSIC_DIR=${MUSIC:-$H/music} HELLOBRO_FAKE_PCM=$OUT/pcm.raw HELLOBRO_FAKE_RADIO=${FAKE_RADIO-1} HELLOBRO_FAKE_SLEEP=1 \
	HELLOBRO_NTX_IO=$OUT/ntx_io HELLOBRO_GPIO=$OUT/gpio HELLOBRO_BATTERY=$OUT/battery HELLOBRO_NICKEL_CONF=$H/nickel.conf \
	HELLOBRO_SHARE_ADDR=127.0.0.1:${SHARE_PORT:-2022} HELLOBRO_SHARE_ROOT=$OUT/onboard HELLOBRO_NO_MDNS=1 HELLOBRO_FAKE_PAIR_CODE=246810 \
	env $MPRIS_ENV timeout "${TIMEOUT:-170}" qemu-arm "$OUT/app/$NAME" > "$OUT/stdout.txt" 2>&1 || echo "app exit: $?"
wait $FEEDPID || true
if [ -n "$MPRIS_ENV" ]; then
	kill "$MPRISPID" "$(cat "$OUT/bus.pid")" 2>/dev/null || true
fi
echo "$(ls "$OUT/screen" | grep -c 'after.png') snapshots in $OUT/screen"
