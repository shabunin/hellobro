# A part of a PDF's page into a notebook (bookcut.go, notepic.go): the
# paper (third on the shelf), the pen drawn across its abstract, copy;
# a notebook beside it, Ctrl+V (the picture at the top of its page),
# written over with the pen, then paste (the button) again: under the
# first. Kept: the .xopp has two images. HELLOBRO_BOOKS as pdf's.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
gui("books"); time.sleep(4)
key(DOWN); key(DOWN); key(ENTER); time.sleep(6)      # the paper
fullscreen(); time.sleep(5)
half = int(min(W, H) * 0.25)
x0, y0, x1, y1 = W // 2 - half, int(H * 0.30), W // 2 + half, int(H * 0.46)
stroke([(x0 + (x1 - x0) * i / 20, y0 + (y1 - y0) * i / 20) for i in range(21)], pen=True); time.sleep(4)
tap(x1 - 524 + 70, y1 + 42); time.sleep(4)           # copy (of copy, save, zoom, ×)
fullscreen(); time.sleep(4)
gui("notebook"); time.sleep(5)                       # a notebook beside it
key(47, ctrl=True); time.sleep(4)                    # Ctrl+V
fullscreen(); time.sleep(4)
stroke([(200 + i * 20, 330 + 40 * math.sin(i / 3)) for i in range(40)], pen=True, pressure=lambda t: 2500); time.sleep(3)
key(47, ctrl=True); time.sleep(6)                    # again: under it
fullscreen(); time.sleep(5)
exit_app()
