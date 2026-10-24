# Kitty's graphics protocol: the Kobo's sh (the PC's, here) runs
# harness/kgshow.py, which shows a PNG at its size and in cells, and a
# zlib'd RGB one.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

here = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
start()
tap(tab_x(0), tabs_y); time.sleep(2)                 # workspace 1: the launcher
keys("2"); key(ENTER); time.sleep(3)                 # the Kobo's sh
keys("clear; python3 " + here + "/kgshow.py\n"); time.sleep(12)
keys("clear\n"); time.sleep(3)
exit_app()
