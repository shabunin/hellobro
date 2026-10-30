# The bar's system and music (portrait), for a Linux player: the battery,
# CPU and RAM at the bar's right; play from the keys, what plays in the
# bar, next and previous; Meta+Shift+B hides the bar and shows it again;
# a tap on what plays pauses it; fullscreen has no bar; workspace 2 has it
# too. Wants MPRIS=1 (the stand-in mpv). NOTE_X: where what plays is.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

COMMA, DOT, KEY_B = 51, 52, 48
NOTE_X = int(os.environ.get("NOTE_X", "0")) or W * 2 // 5

start(); time.sleep(6)                                # the launcher, the bar with the battery, CPU, RAM
key(SPACE, ctrl=True, shift=True); time.sleep(6)      # play: it's in the bar
key(DOT, ctrl=True, shift=True); time.sleep(4)        # next track
key(COMMA, ctrl=True, shift=True); time.sleep(4)      # back to the first
mod(KEY_B, shift=True); time.sleep(3)                 # the bar hidden: the windows up to the top
mod(KEY_B, shift=True); time.sleep(3)                 # back
tap(NOTE_X, 20); time.sleep(4)                        # a tap on what plays: paused
fullscreen(); time.sleep(3)                           # no bar
fullscreen(); time.sleep(3)
tab(2); time.sleep(3)                                 # workspace 2, a Kobo sh: the bar there too
exit_app()
