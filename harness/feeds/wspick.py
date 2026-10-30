# Leaving the workspaces' tiles (workspaces.go): they cover the bar and
# the status strip, so both are drawn again as they go. A shell on 1, a
# launcher on 2; the tiles up from 2, a tap on 1's: the bar says 1, the
# strip is back. Up again, Esc: the same. Up again, a tap on 1's own.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

KEY_O = 24
start(); time.sleep(3)
keys("2"); time.sleep(3)                                     # the Kobo's sh on 1
mod(3); time.sleep(2); mod(ENTER); time.sleep(3)             # a launcher on 2
mod(KEY_O); time.sleep(4)
tap(W // 4, H // 3); time.sleep(4)                           # 1's tile
mod(KEY_O); time.sleep(4)
key(ESC); time.sleep(4)
mod(KEY_O); time.sleep(4)
tap(W // 4, H // 3); time.sleep(4)                           # 1's tile, the one shown
exit_app()
