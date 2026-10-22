# The keys in a question and on the panels. A saved host's menu (1): ↓
# to "mount its files…", Enter; read-only or read-write: → to read-write,
# Esc. The menu again: ↓ ↓ to edit, Enter, Esc. Then Settings: Shift+Tab
# to Wi-Fi's − button, ← folds the panel, → opens it, ← folds it again.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
tap(tab_x(0), tabs_y); time.sleep(2)                 # SSH tab: connect screen
keys("1"); time.sleep(2)                             # the host's menu: connect selected
key(DOWN); time.sleep(2)                             # mount its files…
key(ENTER); time.sleep(2)                            # read-only or read-write?
key(RIGHT); time.sleep(2)                            # read-write
key(ESC); time.sleep(1)
keys("1"); time.sleep(1)
key(DOWN); key(DOWN); time.sleep(2)                  # edit
key(ENTER); time.sleep(2)                            # its line
key(ESC); time.sleep(1)
key(4, ctrl=True, alt=True); time.sleep(3)           # Settings
for _ in range(5):
    key(TAB, shift=True)                             # from the list back over its 4 buttons
time.sleep(2)                                        # to Wi-Fi's −
key(LEFT); time.sleep(3)                             # folded
key(RIGHT); time.sleep(3)                            # open
key(LEFT); time.sleep(3)                             # folded again
exit_app()
