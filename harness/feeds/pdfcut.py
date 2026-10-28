# Zoom and cutting a part of a PDF's page out (bookpdf.go, bookcut.go):
# the paper (third on the shelf), + twice (150%), screenfuls on, - back;
# then the pen drawn across the abstract, copied (text and picture), saved
# (HELLOBRO_BOOKS/Clips), Esc; then the same with a finger held still a
# moment, then moved, and × to take it away. HELLOBRO_BOOKS as pdf's.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

def held(x0, y0, x1, y1):                 # a finger: still a moment, then across
    tid[0] += 1
    frame(0, ID=tid[0], TOOL=0, **at(x0, y0), P=30)
    for _ in range(8):
        time.sleep(0.1); frame(0, **at(x0 + 1, y0), P=31)
    for i in range(1, 13):
        frame(0, **at(x0 + (x1 - x0) * i / 12, y0 + (y1 - y0) * i / 12), P=30); time.sleep(0.05)
    frame(0, ID=-1); time.sleep(3)

def buttons(x1, y1):                       # where drawCut puts copy, save, × under a cut
    x0 = x1 - (150 + 150 + 74)
    return (x0 + 70, y1 + 42), (x0 + 220, y1 + 42), (x0 + 332, y1 + 42)

start()
gui("books"); time.sleep(4)
key(DOWN); key(DOWN); key(ENTER); time.sleep(6)      # the paper
fullscreen(); time.sleep(5)
keys("+"); time.sleep(3); keys("+"); time.sleep(5)   # 150%
for _ in range(3): key(SPACE); time.sleep(3)         # screenfuls, across then down
keys("-"); time.sleep(3); keys("-"); time.sleep(5)   # the fit again
half = int(min(W, H) * 0.25)                         # on the page, which is narrower in landscape
x0, y0, x1, y1 = W // 2 - half, int(H * 0.30), W // 2 + half, int(H * 0.46)
stroke([(x0 + (x1 - x0) * i / 20, y0 + (y1 - y0) * i / 20) for i in range(21)], pen=True); time.sleep(4)
copy, save, close = buttons(x1, y1)
tap(*copy); time.sleep(3)
tap(*save); time.sleep(6)
key(ESC); time.sleep(3)
held(x0, int(H * 0.55), x1, int(H * 0.65)); time.sleep(2)
copy, save, close = buttons(x1, int(H * 0.65))
tap(*close); time.sleep(3)
exit_app()
