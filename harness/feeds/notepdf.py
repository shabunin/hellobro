# Writing on a PDF (notepdf.go) and exporting (noteexport.go): the first
# book, a PDF, in fullscreen; "write" (the pen, the last button under
# the page): its notebook, beside it, on that page; fullscreen; a circle
# and a wave written over the PDF's page; Right, the next PDF page, a
# line; Ctrl+E exports: OUT's Notebooks/NAME.pdf, Clips/latest.png.
# HELLOBRO_BOOKS: a directory with a PDF first by name.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

def circle(cx, cy, r, n=48):
    return [(cx + r * math.cos(i / n * 2 * math.pi), cy + r * math.sin(i / n * 2 * math.pi)) for i in range(n + 1)]
def wave(y0, x0, x1, amp=50, n=40):
    return [(x0 + (x1 - x0) * i / n, y0 + amp * math.sin(i / n * 4 * math.pi)) for i in range(n + 1)]

start()
gui("books"); time.sleep(4)
key(ENTER); time.sleep(6)
fullscreen(); time.sleep(5)
tap(W // 2 + 200, H - 75); time.sleep(10)         # write: the notebook, beside
fullscreen(); time.sleep(10)                      # its page drawn, a moment after
land = W > H
cx = W // 4 if land else W // 2
stroke(circle(cx, H // 3, 120), pen=True, pressure=lambda t: 2500); time.sleep(2)
stroke(wave(H // 2, cx - 250, cx + 250), pen=True, pressure=lambda t: 2500); time.sleep(3)
key(RIGHT); time.sleep(10)
stroke([(cx - 200 + i * 20, H * 2 // 3) for i in range(21)], pen=True, pressure=lambda t: 2500); time.sleep(3)
key(18, ctrl=True); time.sleep(20)                # Ctrl+E
fullscreen(); time.sleep(3)
exit_app()
