# Moves show boxes till they settle: two windows side by side, the right
# one moved left and back (Meta+Shift+arrows) quickly: gray boxes,
# "moving"; a second later laid out for real, once. Then it floats and
# moves down twice: boxes again, then in place.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
tap(tab_x(0), tabs_y); time.sleep(2)                 # workspace 1: the launcher
keys("2"); time.sleep(3)                             # the Kobo's sh
keys("echo left\n"); time.sleep(1)
mod(ENTER); time.sleep(2); keys("2"); time.sleep(3)  # a second window, beside it
keys("echo right\n"); time.sleep(1)
mod(LEFT, shift=True); time.sleep(0.4)               # moved left: boxes
mod(RIGHT, shift=True); time.sleep(0.4)              # and back
time.sleep(3)                                        # settled: the windows again
mod(SPACE, shift=True); time.sleep(3)                # it floats
mod(DOWN, shift=True); time.sleep(0.4)
mod(DOWN, shift=True); time.sleep(3)                 # boxes, then in place
exit_app()
