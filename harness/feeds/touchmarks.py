# Touch marks (Settings, Touch, on: APP_EXTRA with a settings file
# saying touch-marks=on, and FAKE_SNAP_AREA=1000 for their small
# updates): a ring where a finger came down, a dot where it lifted after
# moving, gone 3 s later.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start(); time.sleep(4)
tap(W // 3, H // 3); time.sleep(0.5)                                      # a ring
tap(W // 2, H // 2); time.sleep(0.5)                                      # another
stroke([(W // 3 + 20 * i, 2 * H // 3) for i in range(21)]); time.sleep(1)  # a ring, then a dot 400 px on
time.sleep(5)                                                              # all gone
exit_app()
