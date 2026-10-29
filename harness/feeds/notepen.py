# The notebook's pens (notepen.go): a new notebook, fullscreen, a wave in
# the plain pen; "pen" tapped again: its choices; bold, gray, a wave; then
# the marker across the first wave (under it); then fine and black, a
# third. The page touched puts the choices away. Finds the choices on the
# screen (OUT/screen/screen.raw).
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

OUT = os.path.dirname(os.path.abspath(sys.argv[1]))
def wave(y0, x0, x1, amp=60, n=60):
    return [(x0 + (x1 - x0) * i / n, y0 + amp * math.sin(i / n * 4 * math.pi)) for i in range(n + 1)]
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
def choose(*words):
    tap(*pen); time.sleep(3)                       # its choices, out till the page is touched
    bs = buttons(62, 140)                          # under the strip: fine medium bold, black gray marker
    for word in words:
        tap(*bs[["fine", "medium", "bold", "black", "gray", "marker"].index(word)]); time.sleep(3)

start(); time.sleep(3)
gui("notebook"); time.sleep(3)
fullscreen(); time.sleep(4)
pen = buttons(0, 40)[1]                            # the strip's second button (the first: ‹ All Notebooks)
stroke(wave(H // 4, W // 5, W * 4 // 5), pen=True, pressure=lambda t: 2000); time.sleep(2)
choose("bold", "gray")
stroke(wave(H // 2, W // 5, W * 4 // 5), pen=True, pressure=lambda t: 2000); time.sleep(3)
choose("marker")
stroke([(W // 5 + i * 20, H // 4) for i in range((W * 3 // 5) // 20)], pen=True); time.sleep(3)
choose("fine", "black")
stroke(wave(H * 3 // 4, W // 5, W * 4 // 5), pen=True, pressure=lambda t: 2000); time.sleep(3)
fullscreen(); time.sleep(3)
exit_app()
