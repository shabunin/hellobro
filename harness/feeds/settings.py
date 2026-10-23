# The Settings list on a short screen (landscape): not all its rows fit,
# so the hint under it pages to the rest and back, and ↓ ↑ scroll it.
# Its groups' headings come along. Settings under the launcher (Meta+V),
# half the screen's height.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
mod(47); gui("settings"); time.sleep(1)
hx = 300                                              # the list's hint, under it
tap(hx, H - 50); time.sleep(2)                       # the hint: the rest
tap(hx, H - 50); time.sleep(2)                       # and back to the top (the taps gave it the focus)
for _ in range(22):
    key(DOWN); time.sleep(0.2)
time.sleep(2)                                         # down at Keys
for _ in range(22):
    key(UP); time.sleep(0.2)
time.sleep(2)
exit_app()
