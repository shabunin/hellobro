# Tab through a window's controls, as on a desktop: the dashed ring moves,
# Shift+Tab goes back, Enter presses a button. In Wi-Fi: its switch,
# scan, another network; back to scan, Enter: it scans. Sharing beside
# it: Tab to its switch, Enter: sharing on. Settings under it: ↓ goes
# down its dropdowns, the list following. Mod+Left goes to the window
# beside, Tab goes on in it, and a touch takes the ring away.
# FAKE_SNAP_AREA=1 snapshots the ring's small updates.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
gui("wi-fi")
for _ in range(3):
    key(TAB); time.sleep(0.8)                         # the switch, scan, another network
key(TAB, shift=True); time.sleep(1)                   # back: scan
key(ENTER); time.sleep(3)                             # pressed: scanning
mod(48); gui("sharing")                               # Mod+B: beside it
key(TAB); time.sleep(1)                               # its switch
key(ENTER); time.sleep(3)                             # pressed: sharing on
mod(47); gui("settings")                              # Mod+V: under it
for _ in range(8):
    key(DOWN); time.sleep(0.5)                        # down its dropdowns
time.sleep(1)
mod(105); time.sleep(2)                               # Mod+Left: Wi-Fi
key(TAB); time.sleep(1)                               # on from scan
tap(W // 4, H // 2); time.sleep(2)                    # a touch: no ring
exit_app()
