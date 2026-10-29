# The notebook's select tool (notesel.go): a new notebook, fullscreen, a
# wave and a circle written with the pen; S, the select tool; a rectangle
# drawn round the circle takes it, dragged down-left it moves. A tap on
# the paper lets go, another marks where to paste (a cross); Ctrl+V puts
# the clipboard's picture there, taken: dragged, it moves. Delete takes
# it away, Ctrl+Z brings it back. Wants APP_EXTRA with a clipboard.png.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

def wave(y0, x0, x1, amp=60, n=60):
    return [(x0 + (x1 - x0) * i / n, y0 + amp * math.sin(i / n * 4 * math.pi)) for i in range(n + 1)]
def circle(cx, cy, r, n=48):
    return [(cx + r * math.cos(i / n * 2 * math.pi), cy + r * math.sin(i / n * 2 * math.pi)) for i in range(n + 1)]
def line(x0, y0, x1, y1, n=16):
    return [(x0 + (x1 - x0) * i / n, y0 + (y1 - y0) * i / n) for i in range(n + 1)]
def pentap(x, y):
    stroke([(x, y), (x + 1, y)], pen=True)

start(); time.sleep(3)
gui("notebook"); time.sleep(3)
fullscreen(); time.sleep(4)
cx, cy, r = (W // 4 if W > H else W // 2), H // 2, min(W, H) // 10   # on the left page of two
stroke(wave(H // 4, W // 5, W * 4 // 5), pen=True, pressure=lambda t: 2000); time.sleep(1)
stroke(circle(cx, cy, r), pen=True, pressure=lambda t: 3000); time.sleep(3)
key(31); time.sleep(2)                                         # S: the select tool
stroke(line(cx - r - 40, cy - r - 40, cx + r + 40, cy + r + 40), pen=True); time.sleep(4)   # round the circle
stroke(line(cx, cy, cx - r * 2, cy + r * 2), pen=True); time.sleep(5)                       # moved
pentap(W * 3 // 4, H * 3 // 4); time.sleep(3)                  # let go
pentap(W * 3 // 4, H * 3 // 4 - 200); time.sleep(3)            # the paste mark
key(47, ctrl=True); time.sleep(5)                              # Ctrl+V: there, taken
px, py = W * 3 // 4 + 60, H * 3 // 4 - 160
stroke(line(px, py, px - W // 3, py - H // 3), pen=True); time.sleep(5)  # dragged up-left
key(111); time.sleep(4)                                        # Delete
key(44, ctrl=True); time.sleep(4)                              # Ctrl+Z: back
fullscreen(); time.sleep(3)
exit_app()
