# The lock: music playing, Shift+Meta+X in a Kobo sh: the lock screen
# with the track. A tap and an x go nowhere, Meta+D opens nothing; the
# headphones' next and pause keys still work, and the lock screen follows
# them; asleep and awake, it's still locked. Shift+Meta+X again: unlocked, and "echo ok" reaches the shell,
# without the x. The log says "lock: locked", "lock: unlocked". Wants
# MPRIS=1 (the stand-in mpv).
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

KEY_X, NEXT, PLAYPAUSE = 45, 163, 164
start(); time.sleep(3)
tab(2); time.sleep(3)                                 # a Kobo sh
key(SPACE, ctrl=True, shift=True); time.sleep(5)      # play
mod(KEY_X, shift=True); time.sleep(4)                 # locked: the lock screen, the track
tap(W // 2, H // 2); time.sleep(2)                    # nothing
key(KEY_X); time.sleep(1)                             # nowhere
mod(32); time.sleep(2)                                # no switcher
key(NEXT); time.sleep(4)                              # the next track, on the lock screen
key(PLAYPAUSE); time.sleep(4)                         # paused
power(); time.sleep(10)                               # asleep (pretended), awake: locked still
mod(KEY_X, shift=True); time.sleep(4)                 # unlocked
keys("echo ok\n"); time.sleep(3)
exit_app()
