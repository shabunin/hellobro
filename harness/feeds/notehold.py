# A finger held on a notebook's page where the pen writes (fingers turn
# pages once a pen has): the select tool, what's under it taken (a wave
# written); held and dragged, it moves. Held where nothing is, then
# dragged: the rectangle, what's in it taken. A quick swipe still turns.
# app: hellobro
import os, sys, math
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

def wave(x0, y0):
    return [(x0 + 400 * i / 40, y0 + 40 * math.sin(i / 40 * 4 * math.pi)) for i in range(41)]
def holddrag(a, b, secs=1.0, n=20):
    """A finger down at a, kept still for secs, then dragged to b."""
    tid[0] += 1
    frame(0, ID=tid[0], TOOL=0, **at(*a), P=30); time.sleep(secs)
    for i in range(1, n + 1):
        frame(0, **at(a[0] + (b[0] - a[0]) * i // n, a[1] + (b[1] - a[1]) * i // n), P=30); time.sleep(0.03)
    frame(0, ID=-1); time.sleep(1)

start(); time.sleep(3)
gui("notebook"); time.sleep(3)
fullscreen(); time.sleep(3)
stroke(wave(200, 500), pen=True); time.sleep(2)          # the pen: fingers turn now
stroke(wave(200, 900), pen=True); time.sleep(3)
key(25); time.sleep(1)                                   # P: the pen in hand
hold(400, 500, secs=1.0); time.sleep(3)                  # held on the first wave: taken
key(25); time.sleep(2)                                   # the pen again
holddrag((400, 900), (700, 1200)); time.sleep(3)         # the second, held and moved
key(25); time.sleep(2)
holddrag((200, 1400), (1250, 1700)); time.sleep(3)       # nothing there: a rectangle
exit_app()
