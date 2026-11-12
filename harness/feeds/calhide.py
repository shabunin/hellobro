# Which calendars show (calpick.go): synced with the stand-in CalDAV
# server and a subscribed .ics (calendar.conf in APP_EXTRA), the
# calendars' dialog (c), one unticked: its events gone from the week;
# done; the agenda without it.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start(); time.sleep(3)
gui("calendar"); fullscreen(); time.sleep(6)   # the week, synced
keys("c"); time.sleep(2)                       # the calendars
TAPS = [tuple(int(v) for v in p.split(",")) for p in os.environ.get("CALHIDE_TAPS", "").split()]
for x, y in TAPS:                              # unticked; the last: done
    tap(x, y); time.sleep(2)
key(ESC); time.sleep(1)
keys("4"); time.sleep(2)                       # the agenda
exit_app()
