# Light sleep while music plays: the settings panel (light sleep after
# 1 min idle), Ctrl+Alt+S dozing instead of sleeping, the media keys
# (next, volume) while dozing, a key to wake, dozing by itself after a
# minute without input, a tap that doesn't wake it, the power button that
# does (and dozes it again, and wakes it), fullscreen left by a swipe in
# from the top; then paused, the power button sleeps (pretended). Wants
# MUSIC=a folder of albums.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

K1, K4, KS, KX = 2, 5, 31, 45
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
tap(W // 2, H // 2); time.sleep(3)                    # a tap doesn't wake it
power(); time.sleep(4)                                # the power button does
power(); time.sleep(5)                                # dozes again: the music's on
power(); time.sleep(4)                                # awake
key(K1, ctrl=True, alt=True); time.sleep(3)           # the SSH tab
fullscreen(); time.sleep(3)                               # fullscreen
stroke([(W // 2, 20 + 18 * i) for i in range(16)]); time.sleep(3)  # in from the top: framed again
key(SPACE, ctrl=True, shift=True); time.sleep(2)      # paused
power(); time.sleep(10)                               # sleeps properly (pretended), wakes
exit_app()
