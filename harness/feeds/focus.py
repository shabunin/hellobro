# Tab through a window's controls, as on a desktop: the dashed ring moves,
# Shift+Tab goes back, Enter presses a button. In Wi-Fi from its list:
# disconnect, forget, round to turn off and scan; back to turn off, on to
# scan, Enter: it scans. Sharing beside it: Tab to pair, Enter: pairing.
# Settings under it: its list pages with PgDn. Meta+Left goes to the
# window beside, whose list has the focus, and a touch takes the ring
# away. FAKE_SNAP_AREA=1 snapshots the ring's small updates.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
gui("wi-fi")
for _ in range(4):
    key(TAB); time.sleep(0.8)                         # disconnect, forget, turn off, scan
key(TAB, shift=True); time.sleep(1)                   # back: turn off
key(TAB); time.sleep(1)                               # scan
key(ENTER); time.sleep(3)                             # pressed: scanning
mod(48); gui("sharing")                               # Meta+B: beside it
key(TAB); time.sleep(1)                               # pair
key(ENTER); time.sleep(3)                             # pressed: pairing
mod(47); gui("settings")                              # Meta+V: under it
key(TAB); time.sleep(1)                               # its list (the only control)
key(PGDN); time.sleep(1.5)                            # paged down
key(PGUP); time.sleep(1)
mod(105); time.sleep(2)                               # Meta+Left: Wi-Fi
key(TAB); time.sleep(1)                               # the ring from its list on
tap(W // 4, H // 2); time.sleep(2)                    # a touch: no ring
exit_app()
