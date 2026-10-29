# The launcher's find> line: "blue" narrows the tiles to Bluetooth,
# Enter opens it; a new window, "sett": Settings and what else matches,
# ↓ the next, Esc clears it, "bro@" turns it to ssh>, Esc twice closes.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start(); time.sleep(3)
keys("blue"); time.sleep(3)                            # the first window's launcher
key(ENTER); time.sleep(4)                              # Bluetooth
mod(ENTER); time.sleep(3)                              # a new window
keys("sett"); time.sleep(3)
key(DOWN); time.sleep(2)
key(ESC); time.sleep(2)                                # cleared
keys("bro@"); time.sleep(3)                            # ssh>
key(ESC); time.sleep(2); key(ESC); time.sleep(3)       # cleared, closed
exit_app()
