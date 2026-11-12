# The Wi-Fi window's details: i puts them up, c copies them, Esc
# closes; then the details button, Tab to it and Enter, and a tap
# outside closes them.
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
for _ in range(4): key(TAB); time.sleep(0.5)         # the switch, scan, another network, details
key(ENTER); time.sleep(3)                             # the details again
tap(W // 2, H - 60); time.sleep(2)                    # outside: closed
exit_app()
