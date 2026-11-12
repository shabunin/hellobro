# Screenshots (shot.go): the quick menu (its Screenshot and Auto-rotate
# tiles), closed; Mod+Shift+P, then a rectangle dragged round a part of
# the launcher (held a moment mid-way, to see its outline): a PNG in
# OUT/onboard/Screenshots. Again with a tap: all of the screen. Again,
# then Esc: none.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

def drag(x0, y0, x1, y1, n=24):
    tid[0] += 1
    frame(0, ID=tid[0], TOOL=0, **at(x0, y0), P=30)
    for i in range(1, n + 1):
        frame(0, **at(x0 + (x1 - x0) * i / n, y0 + (y1 - y0) * i / n), P=30); time.sleep(0.03)
    time.sleep(3)
    frame(0, ID=-1); time.sleep(3)

start(); time.sleep(3)
menu(); time.sleep(3)
key(ESC); time.sleep(2)
mod(25, shift=True); time.sleep(2)             # Mod+Shift+P
drag(W // 10, H // 10, W // 2, H // 3)         # a part
mod(25, shift=True); time.sleep(2)
tap(W // 2, H // 2); time.sleep(3)             # all of it
mod(25, shift=True); time.sleep(2)
key(ESC); time.sleep(3)                     # none
exit_app()
