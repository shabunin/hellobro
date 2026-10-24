# tmux's prefix, then Ctrl+arrows held: resizing panes (CSI 1;5D…). The
# Kobo's sh (the PC's, here) runs a tmux of its own socket, no config,
# killed at the end.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

KEY_B = 48
start()
tap(tab_x(0), tabs_y); time.sleep(2)                  # workspace 1: the launcher
keys("2"); key(ENTER); time.sleep(3)                  # the Kobo's sh
keys("tmux -L hbtest -f /dev/null new\n"); time.sleep(4)
key(KEY_B, ctrl=True); keys("%"); time.sleep(3)       # split: two panes side by side
key(KEY_B, ctrl=True)                                 # the prefix, then Ctrl held: ← ← ← ← ←
for _ in range(5): key(LEFT, ctrl=True); time.sleep(0.1)
time.sleep(3)
key(KEY_B, ctrl=True)
for _ in range(3): key(RIGHT, ctrl=True); time.sleep(0.1)
time.sleep(3)
keys("tmux -L hbtest kill-server\n"); time.sleep(3)
exit_app()
