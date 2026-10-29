# The select tool's dots and buttons (notesel.go): a new notebook,
# fullscreen, a circle and a line through it; the circle tapped, taken;
# its bottom right dot dragged out (bigger, the top left staying); the
# dot over it dragged a quarter round (turned); Ctrl+D (a duplicate,
# down and right, taken); Ctrl+C (copied); a tap lets go, another marks
# a place, Ctrl+V pastes the ink there; Ctrl+Z takes it back. Then all
# of it taken by a rectangle, and the "turn" button (found on the screen).
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

OUT = os.path.dirname(os.path.abspath(sys.argv[1]))
def circle(cx, cy, r, n=48):
    return [(cx + r * math.cos(i / n * 2 * math.pi), cy + r * math.sin(i / n * 2 * math.pi)) for i in range(n + 1)]
def line(x0, y0, x1, y1, n=16):
    return [(x0 + (x1 - x0) * i / n, y0 + (y1 - y0) * i / n) for i in range(n + 1)]
def pentap(x, y):
    stroke([(x, y), (x + 1, y)], pen=True)
def word(y0, y1):
    # buttons under the frame: black runs of their top frames, left to right
    d = os.path.join(OUT, "screen")
    w, h = map(int, open(os.path.join(d, "dims")).read().split("x"))
    raw = open(os.path.join(d, "screen.raw"), "rb").read()
    best = []
    for y in range(y0, y1):
        row, runs, x = raw[y * w:(y + 1) * w], [], 0
        while x < w:
            if row[x] < 0x40:
                s = x
                while x < w and row[x] < 0x40: x += 1
                if 50 < x - s < 300: runs.append(((s + x) // 2, y + 20))
            x += 1
        if len(runs) >= 5 and len(runs) > len(best): best = runs
    return best

start(); time.sleep(3)
gui("notebook"); time.sleep(3)
fullscreen(); time.sleep(4)
land = W > H
cx, cy, r = (W // 4 if land else W // 2), H // 3, (90 if land else 150)
k = 0.62 if land else 0.99                         # about the page's scale
stroke(circle(cx, cy, r), pen=True, pressure=lambda t: 2500); time.sleep(1)
stroke(line(cx - 2 * r, cy + r // 2, cx + 2 * r, cy + r // 2), pen=True, pressure=lambda t: 2500); time.sleep(3)
key(31); time.sleep(2)                             # S
pentap(cx, cy - r); time.sleep(4)                  # the circle, taken
pad = 10 + 4
x0, y0, x1, y1 = cx - r - pad, cy - r - pad, cx + r + pad, cy + r + pad
stroke(line(x1, y1, x1 + 80, y1 + 80), pen=True); time.sleep(5)          # bigger
kk = (math.hypot(x1 - x0, y1 - y0) + math.hypot(80, 80)) / math.hypot(x1 - x0, y1 - y0)
x1, y1 = x0 + (x1 - x0) * kk, y0 + (y1 - y0) * kk
mx, my = (x0 + x1) / 2, (y0 + y1) / 2
top = (mx, y0 - 56)                                # the turning dot
R = my - top[1]
stroke([(mx + R * math.sin(i / 12 * math.pi / 2), my - R * math.cos(i / 12 * math.pi / 2)) for i in range(13)], pen=True); time.sleep(5)  # a quarter round
key(32, ctrl=True); time.sleep(4)                  # Ctrl+D
key(46, ctrl=True); time.sleep(3)                  # Ctrl+C
pentap(W // 8 + 20, H - 200); time.sleep(3)        # let go
pentap(W // 8 + 20, H - 400); time.sleep(3)        # the paste mark
key(47, ctrl=True); time.sleep(5)                  # Ctrl+V: the ink, there
key(44, ctrl=True); time.sleep(4)                  # Ctrl+Z
key(44, ctrl=True); time.sleep(4)                  # and the duplicate
key(44, ctrl=True, shift=True); time.sleep(4)      # redo: the duplicate back
stroke(line(cx - 3 * r, cy - 2 * r, cx + 3 * r, cy + 2.5 * r), pen=True); time.sleep(4)  # all of it, taken
bs = word(int(cy), H - 60)
tap(*bs[3]); time.sleep(5)                         # its "turn" button
fullscreen(); time.sleep(3)
exit_app()
