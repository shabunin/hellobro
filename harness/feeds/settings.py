# The Settings list on a short screen (landscape): not all its rows fit,
# so a finger scrolls it (a drag, then back), and ↓ ↑ walk the focus down
# to Keys and back, the list following. Enter on a setting opens its
# choices; Esc closes them. Settings under the launcher (Mod+V), half the
# screen's height.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
mod(47); gui("settings"); time.sleep(1)
x = 300
stroke([(x, H - 80 - 20 * i) for i in range(20)]); time.sleep(2)   # dragged up: the rest
stroke([(x, H - 480 + 20 * i) for i in range(20)]); time.sleep(2)  # and back down
for _ in range(22):
    key(DOWN); time.sleep(0.2)
time.sleep(2)                                         # down at Keys
for _ in range(21):
    key(UP); time.sleep(0.2)
time.sleep(2)                                         # the first setting
key(ENTER); time.sleep(2)                             # its choices
key(ESC); time.sleep(2)                               # closed
exit_app()
