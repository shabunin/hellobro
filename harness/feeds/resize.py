# The resize mode: two windows side by side, Meta+R, wider three times,
# shorter once (nothing: they're side by side), then Enter. While it's on
# the windows are gray boxes ("resize mode", each one's name and size),
# the one in focus framed black; Enter lays them out for real.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

KEY_R = 19
start()
tap(tab_x(0), tabs_y); time.sleep(2)                 # workspace 1: the launcher
keys("2"); time.sleep(3)                             # the Kobo's sh
keys("echo left\n"); time.sleep(1)
mod(ENTER); time.sleep(2)                            # a second window, beside it
keys("2"); time.sleep(3)
keys("echo right\n"); time.sleep(1)
mod(KEY_R); time.sleep(3)                            # resize mode: boxes
for _ in range(3):
    key(LEFT); time.sleep(2)                         # narrower
key(UP); time.sleep(2)
key(ENTER); time.sleep(4)                            # done: laid out
exit_app()
