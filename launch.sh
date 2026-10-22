#!/bin/sh
# Starts one of the apps next to it (hellobro, brotools) with Nickel
# frozen, so Nickel can't draw over it.
#
# usage: launch.sh APP
#
# NickelMenu runs this with cmd_output (see nm/hellobro), which makes
# Nickel wait for us; we SIGSTOP it meanwhile, and SIGCONT it only once
# we're gone. Nickel then sees us finished and runs the rest of the menu
# entry, which switches views so it repaints the whole screen.
# Freezing (unlike killing, as KOReader does) leaves Wi-Fi and Bluetooth up.
cd "$(dirname "$0")" || exit 0
app=${1:-hellobro}
case $app in hellobro | brotools) ;; *) exit 0 ;; esac
exec </dev/null >/dev/null 2>&1 # Nickel reads our output, and it'll be frozen: never block on it
PATH=$PATH:/sbin:/usr/sbin:/bin:/usr/bin # for wpa_cli
say() { echo "$(date '+%Y/%m/%d %H:%M:%S') launch.sh: $*" >>"$app.log"; }

[ -s "$app.log" ] && [ "$(wc -c <"$app.log")" -gt 1000000 ] && mv "$app.log" "$app-old.log"

# A process feeding the hardware watchdog must keep running.
watchdog=""
for fd in /proc/[0-9]*/fd/*; do
	case "$(readlink "$fd" 2>/dev/null)" in
	/dev/watchdog*)
		pid=${fd#/proc/}
		pid=${pid%%/*}
		watchdog="$watchdog $pid"
		say "watchdog held by $pid ($(cat /proc/$pid/comm 2>/dev/null))"
		;;
	esac
done

# sickel watches over nickel, so it gets frozen too.
frozen=""
for name in nickel sickel; do
	for pid in $(pidof $name); do
		case " $watchdog " in
		*" $pid "*) say "not freezing $name ($pid): it feeds the watchdog" ;;
		*) frozen="$frozen $pid" ;;
		esac
	done
done

thaw() {
	[ -n "$frozen" ] || return
	say "thawing$frozen"
	# Detached, after a moment: by then we've exited and Nickel finds us done.
	(sleep 0.5 2>/dev/null || sleep 1; kill -CONT $frozen) &
	frozen=""
}
trap 'thaw; exit 0' INT TERM HUP
trap thaw EXIT

if [ -n "$frozen" ]; then
	say "freezing$frozen"
	kill -STOP $frozen
fi
export HELLOBRO_FROZEN="$frozen" # so the app can thaw them itself if it gets stuck
"./$app"
status=$?
say "$app exited ($status)"
# A new build (hellobro-update) that crashed before it got going: the one
# before comes back, and runs.
if [ "$app" = hellobro ] && [ "$status" -ne 0 ] && [ -f update-trial ] && [ -f hellobro.old ]; then
	say "the new $app failed its trial ($(cat update-trial)): back to the one before"
	mv -f hellobro hellobro.bad && mv -f hellobro.old hellobro && rm -f update-trial
	"./$app"
	say "$app exited ($?)"
fi
# The Alpine jails (alpineinit.go) go with the app, and their image lets
# go of the storage, which Nickel may hand to a computer over USB.
for pid in $(pidof hellobro); do
	if tr '\0' ' ' <"/proc/$pid/cmdline" | grep -q -- --alpine-init; then
		say "ending the Alpine jail $pid"
		kill -KILL "$pid"
	fi
done
if grep -q " /tmp/hellobro-alpine " /proc/mounts; then
	say "unmounting the Alpine image"
	umount -l /tmp/hellobro-alpine
fi
exit 0
