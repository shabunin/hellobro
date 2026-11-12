# The select tool's rectangles (notesel.go), as AutoCAD's: a new notebook,
# fullscreen, three short lines written with the pen; S, the select tool.
# A rectangle drawn left to right over the first two takes only the one
# wholly in it; let go, one drawn right to left over the same takes both,
# the one it only crosses too. Held a moment mid-way, each, to see its
# outline: whole, then dashed.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

def line(x0, y0, x1, y1, n=16):
    return [(x0 + (x1 - x0) * i / n, y0 + (y1 - y0) * i / n) for i in range(n + 1)]
def pentap(x, y):
    stroke([(x, y), (x + 1, y)], pen=True)
def rect(x0, y0, x1, y1, n=24):
    """The pen down at x0 y0, to x1 y1, and held there a moment."""
    tid[0] += 1
    frame(0, ID=tid[0], TOOL=1, **at(x0, y0), P=0); time.sleep(0.2)
    for i in range(n + 1):
        frame(0, **at(x0 + (x1 - x0) * i / n, y0 + (y1 - y0) * i / n), P=2000); time.sleep(0.03)
    time.sleep(3)
    frame(0, P=0); time.sleep(0.3)
    frame(0, ID=-1); time.sleep(3)

start(); time.sleep(3)
gui("notebook"); time.sleep(3)
fullscreen(); time.sleep(4)
x, y = W // 6, H // 3
stroke(line(x, y, x + 200, y), pen=True, pressure=lambda t: 3000); time.sleep(1)            # wholly in
stroke(line(x + 100, y + 150, x + 600, y + 150), pen=True, pressure=lambda t: 3000); time.sleep(1)  # half in
stroke(line(x, y + 400, x + 300, y + 400), pen=True, pressure=lambda t: 3000); time.sleep(3)  # out
key(31); time.sleep(2)                                         # S: the select tool
rect(x - 40, y - 60, x + 320, y + 220)                         # a window: the first only
pentap(W * 3 // 4, H * 3 // 4); time.sleep(3)                  # let go
rect(x + 320, y + 220, x - 40, y - 60)                         # crossing: both
fullscreen(); time.sleep(3)
exit_app()
