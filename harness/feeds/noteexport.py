# Exporting a notebook (noteexport.go): a new notebook, fullscreen, a
# circle and a wave; Right, a line on the next page; Ctrl+E asks, Enter
# (the PDF): OUT's Notebooks/NAME.pdf (NAME (notebook).pdf beside
# someone else's); Ctrl+E again, n (the PNG): Clips/NAME pN DATE TIME.png
# and Clips/latest.png; each with a note on the page.
# HELLOBRO_BOOKS: a directory, where Clips/ goes.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

def circle(cx, cy, r, n=48):
    return [(cx + r * math.cos(i / n * 2 * math.pi), cy + r * math.sin(i / n * 2 * math.pi)) for i in range(n + 1)]
def wave(y0, x0, x1, amp=50, n=40):
    return [(x0 + (x1 - x0) * i / n, y0 + amp * math.sin(i / n * 4 * math.pi)) for i in range(n + 1)]

start(); time.sleep(3)
gui("notebook"); time.sleep(3)
fullscreen(); time.sleep(4)
cx = W // 4 if W > H else W // 2
stroke(circle(cx, H // 3, 120), pen=True, pressure=lambda t: 2500); time.sleep(2)
stroke(wave(H // 2, cx - 250, cx + 250), pen=True, pressure=lambda t: 2500); time.sleep(3)
key(RIGHT); time.sleep(3)
stroke([(cx - 200 + i * 20, H * 2 // 3) for i in range(21)], pen=True, pressure=lambda t: 2500); time.sleep(3)
key(18, ctrl=True); time.sleep(3)                 # Ctrl+E: PDF or PNG?
key(ENTER); time.sleep(15)                        # the PDF
key(18, ctrl=True); time.sleep(3)
keys("n"); time.sleep(10)                         # the PNG, page 2
fullscreen(); time.sleep(3)
exit_app()
