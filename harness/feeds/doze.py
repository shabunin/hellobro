# Light sleep while music plays: the settings panel (light sleep after
# 1 min idle), Ctrl+Alt+S dozing instead of sleeping, the media keys
# (next, volume) while dozing, a key to wake, dozing by itself after a
# minute without input, a tap to wake; then paused, Ctrl+Alt+S sleeps
# (pretended). Wants MUSIC=a folder of albums.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

K4, KS, KX = 5, 31, 45
NEXT, VOLUP = 163, 115
LEFT = 105

start(); time.sleep(4)
key(SPACE, ctrl=True, shift=True); time.sleep(6)      # play, from the SSH tab
key(K4, ctrl=True, alt=True); time.sleep(4)           # Settings
key(TAB); key(TAB); time.sleep(2)                     # the settings panel
key(LEFT); key(LEFT); time.sleep(2)                   # light sleep after 1 min idle
key(KS, ctrl=True, alt=True); time.sleep(5)           # dozes: the music plays on
key(NEXT); time.sleep(5)                              # the next track, on the doze screen
key(VOLUP); time.sleep(2)
key(KX); time.sleep(4)                                # wakes, the x typed nowhere
time.sleep(66)                                        # dozes by itself
tap(W // 2, H // 2); time.sleep(4)                    # a tap wakes
key(SPACE, ctrl=True, shift=True); time.sleep(2)      # paused
key(KS, ctrl=True, alt=True); time.sleep(8)           # sleeps properly (pretended), wakes
exit_app()
