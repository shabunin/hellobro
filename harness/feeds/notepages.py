# The notebook's pages, small (notepages.go): a new notebook, fullscreen,
# three pages written on (a line, a circle, a wave); O, the overview;
# Left twice, the first chosen; "later" (it's second now); "copy page"
# (a copy after it, chosen); Delete (the copy gone); "new after" (a blank
# one); Ctrl+Z (gone again); Enter opens the page chosen; O again, the
# first tile tapped twice: chosen, then opened. Finds the strip's buttons
# on the screen (OUT/screen/screen.raw).
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

OUT = os.path.dirname(os.path.abspath(sys.argv[1]))
def circle(cx, cy, r, n=48):
    return [(cx + r * math.cos(i / n * 2 * math.pi), cy + r * math.sin(i / n * 2 * math.pi)) for i in range(n + 1)]
def wave(y0, x0, x1, amp=60, n=40):
    return [(x0 + (x1 - x0) * i / n, y0 + amp * math.sin(i / n * 4 * math.pi)) for i in range(n + 1)]
def line(x0, y0, x1, y1, n=16):
    return [(x0 + (x1 - x0) * i / n, y0 + (y1 - y0) * i / n) for i in range(n + 1)]
def strip():
    # the strip's buttons, left to right: the black runs of their frames' tops
    d = os.path.join(OUT, "screen")
    w, h = map(int, open(os.path.join(d, "dims")).read().split("x"))
    raw = open(os.path.join(d, "screen.raw"), "rb").read()
    best = []
    for y in range(0, 80):
        row, runs, x = raw[y * w:(y + 1) * w], [], 0
        while x < w:
            if row[x] < 0x40:
                s = x
                while x < w and row[x] < 0x40: x += 1
                if 30 < x - s < 400: runs.append(((s + x) // 2, y + 20))
            x += 1
        if len(runs) > len(best): best = runs
    return best
def button(i):
    tap(*strip()[i]); time.sleep(4)
OPEN, NEW_BEFORE, NEW_AFTER, COPY, DELETE, EARLIER, LATER = range(7)

start(); time.sleep(3)
gui("notebook"); time.sleep(3)
fullscreen(); time.sleep(4)
land = W > H
pen = lambda pts: (stroke(pts, pen=True, pressure=lambda t: 2500), time.sleep(2))
if land:
    pen(line(W // 4, H // 3, W // 4, H * 2 // 3)); pen(circle(W * 3 // 4, H // 2, 150))
    key(RIGHT); time.sleep(4); pen(wave(H // 2, W // 10, W * 2 // 5))
else:
    pen(line(W // 2, H // 3, W // 2, H * 2 // 3)); key(RIGHT); time.sleep(4)
    pen(circle(W // 2, H // 2, 200)); key(RIGHT); time.sleep(4)
    pen(wave(H // 2, W // 5, W * 4 // 5))
time.sleep(2)
key(24); time.sleep(5)                             # O: the pages, small
key(LEFT); time.sleep(2); key(LEFT); time.sleep(3) # the first
button(LATER)
button(COPY)
key(111); time.sleep(4)                            # Delete
button(NEW_AFTER)
key(44, ctrl=True); time.sleep(4)                  # Ctrl+Z
key(ENTER); time.sleep(5)                          # open it
key(24); time.sleep(5)
tap(180, 300); time.sleep(3)                       # the first tile: chosen
tap(180, 300); time.sleep(5)                       # again: opened
fullscreen(); time.sleep(3)
exit_app()
