# The sleep screen as the calendar's agenda: the calendar synced in the
# background (calendar.conf in APP_EXTRA, a stand-in CalDAV server and an
# .ics subscribed to), never opened; then sleep (Mod+Shift+S), pretended.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

KEY_S = 31
start(); time.sleep(8)                         # synced meanwhile
mod(KEY_S, shift=True); time.sleep(6)          # the agenda, then awake
exit_app()
