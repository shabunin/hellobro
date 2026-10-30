# Two fingers swiped down from the screen's top put the workspaces' tiles
# up (workspaces.go, wsSwipe): from the bar, Esc; from the middle,
# nothing; full screen, from the top again, Esc. The log says "two-finger
# swipe: the workspaces" twice (once where the screen tells two fingers
# apart in full screen too).
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start(); time.sleep(3)
mod(3); time.sleep(2); mod(ENTER); time.sleep(3)             # 2: a launcher
pair((W // 3, 30), (W * 2 // 3, 30), 0, 300); time.sleep(4)  # from the bar: the tiles
key(ESC); time.sleep(3)
pair((W // 3, H // 2), (W * 2 // 3, H // 2), 0, 300); time.sleep(3)  # from the middle: nothing
fullscreen(); time.sleep(3)
pair((W // 3, 30), (W * 2 // 3, 30), 0, 300); time.sleep(4)  # full screen, from the top
key(ESC); time.sleep(3)
fullscreen(); time.sleep(3)
exit_app()
