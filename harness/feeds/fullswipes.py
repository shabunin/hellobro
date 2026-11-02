# Full screen, the edges are the window's: a swipe up the right edge
# (the menu's), in from the right (a workspace's), up from the bottom:
# nothing. Up the left edge is the light's still, and two fingers across
# go to the next workspace (Battery on workspace 2) and back. A swipe
# down from the top leaves full screen.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start(); time.sleep(3)
mod(3); time.sleep(2); gui("battery"); time.sleep(3)                       # Mod+2: Battery there
mod(2); time.sleep(3)                                                      # Mod+1: back
fullscreen(); time.sleep(3)                                                # the launcher, full screen
stroke([(W - 30, H * 2 // 3 - i * 15) for i in range(21)]); time.sleep(2)  # the right edge, up: no menu
stroke([(30, H * 2 // 3 - i * 20) for i in range(21)]); time.sleep(2)      # the left edge, up: the light
pair((W // 2, H // 3), (W // 2, H // 2), -W // 3, 0); time.sleep(3)        # two fingers left: workspace 2
pair((W // 2, H // 3), (W // 2, H // 2), W // 3, 0); time.sleep(3)         # and right: back, full screen
stroke([(W - 10 - i * 20, H // 2) for i in range(21)]); time.sleep(2)      # in from the right
stroke([(W // 2, H - 20 - i * 15) for i in range(21)]); time.sleep(2)      # up from the bottom
stroke([(W // 2, 20 + i * 15) for i in range(21)]); time.sleep(3)          # down from the top: out
stroke([(30, H * 2 // 3 - i * 20) for i in range(21)]); time.sleep(3)      # the light again
menu(); key(ESC); time.sleep(2)                                            # and the menu
exit_app()
