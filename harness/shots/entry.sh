#!/bin/sh
/usr/sbin/sshd -e
su bro -s /bin/sh -c '
cd /home/bro
dbus-daemon --session --address=unix:path=/run/bus/bus --fork --nopidfile
mpd /home/bro/.config/mpd/mpd.conf
DBUS_SESSION_BUS_ADDRESS=unix:path=/run/bus/bus mpd-mpris >/tmp/mpd-mpris.log 2>&1 &
hellobro-server -listen :2222 >/tmp/server.log 2>&1 &
'
exec sleep infinity
