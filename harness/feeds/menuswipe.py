# The menu swipe: up along the right edge opens the ≡ menu (Esc closes
# it), down too; a stroke mid-screen doesn't. With the keyboard gone, the
# menu's Lock (tapped at LOCK_X, LOCK_Y) locks with the swipes as the way
# out, and a swipe up unlocks. The log says "menu swipe" twice, then
# "lock: locked", "lock: swiped ↑, unlock".
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
import feedlib
from feedlib import *

start(); time.sleep(3)
stroke([(W - 50, 1300 - i * 15) for i in range(21)]); time.sleep(3)   # up 300 px: the menu
key(ESC); time.sleep(2)
stroke([(700, 1300 - i * 15) for i in range(21)]); time.sleep(2)      # mid-screen: no menu
stroke([(W - 50, 600 + i * 15) for i in range(21)]); time.sleep(3)    # down: the menu
key(ESC); time.sleep(2)
feedlib.kbd.close(); time.sleep(4)                                     # no keyboard
stroke([(W - 50, 1300 - i * 15) for i in range(21)]); time.sleep(3)   # the menu, by touch
if os.environ.get("LOCK_X"):
    tap(int(os.environ["LOCK_X"]), int(os.environ["LOCK_Y"])); time.sleep(4)  # Lock
    stroke([(W // 2, H // 2 + 250 - i * 25) for i in range(21)]); time.sleep(4)  # up: unlocked
feedlib.kbd = open(os.path.join(feedlib.d, 'event4'), 'wb', buffering=0); time.sleep(4)
key(ESC); time.sleep(1)
exit_app()
