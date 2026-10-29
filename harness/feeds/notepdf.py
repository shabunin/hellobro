# A PDF's page copied to a notebook, written on, exported (bookcut.go,
# noteexport.go): the first book, a PDF, in fullscreen; the select tool,
# "copy page"; a new notebook, fullscreen, Ctrl+V: the page, taken; pen,
# a circle and a wave over it; Ctrl+E exports: OUT's Notebooks/NAME.pdf,
# Clips/latest.png.
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
tap(W // 2 + 150, H - 75); time.sleep(3)          # the select tool
tap(W - 110, H - 160); time.sleep(6)              # copy page, at the page's foot on the right
fullscreen(); time.sleep(3)
gui("notebook"); time.sleep(3)
fullscreen(); time.sleep(4)
key(47, ctrl=True); time.sleep(6)                 # Ctrl+V: the page, taken
key(25); time.sleep(2)                            # P: the pen
cx = W // 4 if W > H else W // 2
stroke(circle(cx, H // 3, 120), pen=True, pressure=lambda t: 2500); time.sleep(2)
stroke(wave(H // 2, cx - 250, cx + 250), pen=True, pressure=lambda t: 2500); time.sleep(3)
key(18, ctrl=True); time.sleep(20)                # Ctrl+E
fullscreen(); time.sleep(3)
exit_app()
