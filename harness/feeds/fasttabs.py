# Changing tabs with "Changing tabs" on (APP_EXTRA's settings:
# fast-tabs=on): each tab goes up with DU, and once the switching stops a
# GL16 brings the grays back. calls.txt shows the waveforms.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start(); time.sleep(2)
for n in (3, 4, 5, 6, 7):                             # Ctrl+Alt+2..6 in quick turns
    key(n, ctrl=True, alt=True); time.sleep(0.3)
time.sleep(3)                                         # Power: cleaned
key(5, ctrl=True, alt=True); time.sleep(3)            # Settings, alone
exit_app()
