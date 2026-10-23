# The Wi-Fi window's info: i puts the details up, c copies them, Esc
# closes; then the info button, tapped, and a tap outside closes it.
# Wi-Fi alone on its workspace (the launcher closed first).
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

KQ, KI, KC = 16, 23, 46  # Q, I, C

start(); time.sleep(3)
mod(KQ, shift=True); time.sleep(2)                    # the launcher closed
gui("wi-fi"); time.sleep(2)                           # the workspace's only window
key(KI); time.sleep(3)                                # the details
key(KC); time.sleep(2)                                # copied
key(ESC); time.sleep(2)
bw = min(250, (W - 2 * 34 - 36) // 4)                 # its buttons along the top
tap(34 + 3 * (bw + 12) + bw // 2, 152); time.sleep(3) # info, tapped
tap(W // 2, H - 60); time.sleep(2)                    # outside: closed
exit_app()
