# Each terminal its font's size: two Kobo shells side by side, Ctrl+=
# three times in the right one, the left one staying; a third window
# takes the size last set; Ctrl+0 back in it.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
tap(tab_x(0), tabs_y); time.sleep(2)                  # workspace 1: the launcher
keys("2"); key(ENTER); time.sleep(3)                  # the Kobo's sh
keys("echo left\n"); time.sleep(1.5)
mod(ENTER); time.sleep(2.5)                           # another beside it
keys("2"); key(ENTER); time.sleep(3)
keys("echo right\n"); time.sleep(1.5)
for _ in range(3): key(13, ctrl=True); time.sleep(2)  # Ctrl+= x3: 32 px, here only
mod(ENTER); time.sleep(2.5)                           # a third: 32 px too
keys("2"); key(ENTER); time.sleep(3)
keys("echo third\n"); time.sleep(1.5)
key(11, ctrl=True); time.sleep(3)                     # Ctrl+0: 26 px, here only
exit_app()
