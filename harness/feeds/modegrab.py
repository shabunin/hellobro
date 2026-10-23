# A mode's keys are its own, grab or not: the Kobo's sh runs cat -v,
# grabbed (Meta+Shift+G), and the resize mode comes from the menu. In it,
# x, the arrows, Shift and Ctrl+C reach nothing: cat shows none of them.
# Esc ends it, and y goes to cat again (the grab still on).
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

KEY_G, KEY_C = 34, 46
start()
tap(tab_x(0), tabs_y); time.sleep(2)                 # workspace 1: the launcher
keys("2"); time.sleep(3)                             # the Kobo's sh
keys("cat -v\n"); time.sleep(1)
mod(KEY_G, shift=True); time.sleep(2)                # grab
menu(); keys("5"); time.sleep(2)                     # Go to…: the switcher, by touch
keys("resize"); time.sleep(1); key(ENTER); time.sleep(3)  # resize mode
keys("x"); key(LEFT); key(RIGHT, shift=True); key(KEY_C, ctrl=True); time.sleep(3)
key(ESC); time.sleep(3)                              # done
keys("y\n"); time.sleep(2)                           # cat has y, and only y
key(KEY_C, ctrl=True); time.sleep(1)
mod(KEY_G, shift=True); time.sleep(1)                # grab off
exit_app()
