# The notebook's buttons as pictures (noteicons.go), and "clear" asking
# first: a new notebook, fullscreen, a wave; the pen's choices, bold gray
# (its button shows the line it writes), a second wave. "clear" once: it
# asks ("tap again"), and forgets after a few seconds; twice: the page
# cleared, Ctrl+Z back. S and a rectangle round the first wave: the
# selection's buttons; O: the pages' strip. Finds the buttons on the
# screen (OUT/screen/screen.raw).
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

OUT = os.path.dirname(os.path.abspath(sys.argv[1]))
def wave(y0, x0, x1, amp=60, n=60):
    return [(x0 + (x1 - x0) * i / n, y0 + amp * math.sin(i / n * 4 * math.pi)) for i in range(n + 1)]
def line(x0, y0, x1, y1, n=16):
    return [(x0 + (x1 - x0) * i / n, y0 + (y1 - y0) * i / n) for i in range(n + 1)]
def buttons(y0, y1):
    # the widest row of black runs (buttons' top frames) between y0 and y1
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
                if 40 < x - s < 300: runs.append(((s + x) // 2, y + 20))
            x += 1
        if len(runs) > len(best): best = runs
    return best

start(); time.sleep(3)
gui("notebook"); time.sleep(3)
fullscreen(); time.sleep(4)
x0, x1 = W // 8, (W * 3 // 8 if W > H else W * 7 // 8)
stroke(wave(H // 4, x0, x1), pen=True, pressure=lambda t: 2000); time.sleep(2)
bar = buttons(0, 60)                               # pen eraser select undo redo clear paste paper export …
tap(*bar[0]); time.sleep(3)                        # the pen's choices
ch = buttons(62, 140)
tap(*ch[2]); time.sleep(2); tap(*ch[4]); time.sleep(2)   # bold, gray
stroke(wave(H // 2, x0, x1), pen=True, pressure=lambda t: 2000); time.sleep(3)
tap(*bar[5]); time.sleep(2)                        # clear: it asks
time.sleep(5)                                      # and forgets
tap(*bar[5]); time.sleep(1); tap(*bar[5]); time.sleep(4)  # twice: cleared
key(44, ctrl=True); time.sleep(4)                  # Ctrl+Z: back
key(31); time.sleep(2)                             # S: the select tool
stroke(line(x0 - 30, H // 4 - 100, x1 + 30, H // 4 + 100), pen=True); time.sleep(5)
key(24); time.sleep(5)                             # O: the pages, small
key(24); time.sleep(3)
fullscreen(); time.sleep(3)
exit_app()
