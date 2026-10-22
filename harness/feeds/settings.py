# The Settings list on a short screen (landscape): not all its rows fit,
# so the hint under it pages to the rest and back, and ↓ ↑ scroll it.
# Portrait has room for all: the taps on the hint do nothing there.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
key(5, ctrl=True, alt=True); time.sleep(3)            # Settings
tap(300, H - 101); time.sleep(2)                      # the hint: the rest
tap(300, H - 101); time.sleep(2)                      # and back to the top (the taps gave it the focus)
for _ in range(12):
    key(DOWN); time.sleep(0.2)
time.sleep(2)                                         # down at Keys
for _ in range(12):
    key(UP); time.sleep(0.2)
time.sleep(2)
exit_app()
