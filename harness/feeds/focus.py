# Tab through a page's controls, as on a desktop: the dashed ring moves,
# Shift+Tab goes back, Enter presses a button, ← → on the tab strip change
# tabs, PgDn pages a list, Ctrl+Tab goes to the next panel, and a touch
# takes the ring away. FAKE_SNAP_AREA=1 snapshots the ring's small updates.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
key(4, ctrl=True, alt=True); time.sleep(3)            # Settings
key(TAB); time.sleep(1)                               # Wi-Fi's first button (the list was in focus)...
for _ in range(5):
    key(TAB); time.sleep(0.8)                         # ...and on: scan, other network, info, the list, Bluetooth
key(TAB, shift=True); time.sleep(1)                   # back
key(TAB, ctrl=True); time.sleep(1.5)                  # the next panel: Sharing, its − box
key(TAB); time.sleep(1)                               # on/off
key(TAB); time.sleep(1)                               # pair
key(ENTER); time.sleep(3)                             # pressed: pairing
key(TAB, ctrl=True); time.sleep(1.5)                  # the next panel: the Settings list
key(PGDN); time.sleep(1.5)                            # paged down
key(PGUP); time.sleep(1)
for _ in range(20):
    key(TAB, shift=True); time.sleep(0.3)             # back round to the tab strip
time.sleep(1)
key(RIGHT); time.sleep(2.5)                           # → Power
key(LEFT); time.sleep(2.5)                            # ← Settings
tap(200, 40); time.sleep(2)                           # a touch (the title): no ring
key(5, ctrl=True, alt=True); time.sleep(3)            # Power
key(TAB); time.sleep(1)                               # the strip, Battery has nothing
key(TAB, ctrl=True); time.sleep(1)
exit_app()
