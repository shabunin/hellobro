# The power button sleeps, music or not: no light sleep. Music playing,
# a press: the player paused, the radios off, asleep (pretended), awake;
# then Shift+Meta+S, the same. The log says "power button: sleeping",
# "sleep: going to sleep" twice. Wants MPRIS=1 (the stand-in mpv).
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

KEY_S = 31
start(); time.sleep(6)
key(SPACE, ctrl=True, shift=True); time.sleep(6)      # play
power(); time.sleep(12)                               # asleep, paused; awake again
key(SPACE, ctrl=True, shift=True); time.sleep(6)      # play again
key(KEY_S, shift=True, meta=True); time.sleep(12)       # the same by keys
exit_app()
