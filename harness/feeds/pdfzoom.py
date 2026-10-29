# A PDF's zoom and its gestures (bookzoom.go): the first book, a PDF,
# fullscreen; two fingers pinched apart round the page's top left: the
# zoom shows while they move, the page drawn again zoomed when they
# lift; a finger dragged up and left: the page moves with it; → a
# screenful on from there; - a step out, + a step in. Esc: the shelf;
# the PDF again: zoomed still (each PDF's own).
# HELLOBRO_BOOKS: a directory with a PDF first by name.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

tid = [100]
def pinch(cx, cy, d0, d1):
    """Two fingers round (cx, cy), from d0 px apart to d1."""
    tid[0] += 2
    for i in range(21):
        h = (d0 + (d1 - d0) * i // 20) // 2
        if i == 0:
            frame(0, ID=tid[0] - 1, TOOL=0, **at(cx - h, cy - h), P=30)
            frame(1, ID=tid[0], TOOL=0, **at(cx + h, cy + h), P=30)
        else:
            frame(0, **at(cx - h, cy - h), P=30)
            frame(1, **at(cx + h, cy + h), P=30)
        time.sleep(0.04)
    time.sleep(1)
    frame(0, ID=-1); frame(1, ID=-1)

start()
gui("books"); time.sleep(4)
key(ENTER); time.sleep(6)
fullscreen(); time.sleep(5)
pinch(W // 3, H // 3, 200, 700); time.sleep(12)    # zoomed in round there
stroke([(W // 2 + 300 - i * 30, H // 2 + 300 - i * 20) for i in range(21)]); time.sleep(10)  # moved
key(RIGHT); time.sleep(8)                          # a screenful on
key(MINUS); time.sleep(8)                          # a step out
key(13, shift=True); time.sleep(8)                 # + a step in
key(1); time.sleep(5)                              # the shelf
key(ENTER); time.sleep(8)                          # the PDF: zoomed still
fullscreen(); time.sleep(3)
exit_app()
