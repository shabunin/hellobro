# The terminal tabs' status bar (portrait) and the music hotkeys: play
# from the SSH tab before the Music tab was ever up, next and previous,
# fullscreen with the bar, hiding and showing it, a tap on it to pause.
# Wants MUSIC=a folder of albums.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

COMMA, DOT, KEY_B = 51, 52, 48

start(); time.sleep(6)                                # SSH tab, the bar with the battery, CPU, RAM
key(SPACE, ctrl=True, shift=True); time.sleep(6)      # play: the albums are found first
key(DOT, ctrl=True, shift=True); time.sleep(4)        # next track
key(COMMA, ctrl=True, shift=True); time.sleep(4)      # back to the first
key(F11); time.sleep(4)                               # fullscreen: the bar stays
key(KEY_B, ctrl=True, shift=True); time.sleep(3)      # hidden
key(KEY_B, ctrl=True, shift=True); time.sleep(3)      # back
tap(300, H - 17); time.sleep(3)                       # a tap on it: paused
key(F11); time.sleep(3)
tap(tab_x(1), tabs_y); time.sleep(3)                  # the Shell tab has it too
exit_app()
