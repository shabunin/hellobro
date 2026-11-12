# A notebook zoomed by hand: a wave written with the pen (fingers turn
# pages from then on), two fingers pinched apart round it (200%), one
# finger moving the page, a word written zoomed, then pinched back
# together (the fit), and a swipe turning the page.
# app: hellobro
import os, sys, math
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

def wave(x0, y0, w=400):
    return [(x0 + w * i / 40, y0 + 40 * math.sin(i / 40 * 4 * math.pi)) for i in range(41)]
def pinch(c, d0, d1, n=20):
    """Two fingers either side of c, d0 apart, going to d1 apart."""
    ids = []
    for slot, s in enumerate((-1, 1)):
        tid[0] += 1; ids.append(tid[0])
        frame(slot, ID=tid[0], TOOL=0, **at(c[0] + s * d0 // 2, c[1]), P=30); time.sleep(0.02)
    for i in range(1, n + 1):
        d = d0 + (d1 - d0) * i // n
        for slot, s in enumerate((-1, 1)):
            frame(slot, **at(c[0] + s * d // 2, c[1]), P=30)
        time.sleep(0.03)
    time.sleep(0.3)
    for slot in (0, 1):
        frame(slot, ID=-1)
    time.sleep(1.5)

start(); time.sleep(3)
gui("notebook"); time.sleep(3)
fullscreen(); time.sleep(3)
stroke(wave(200, 500), pen=True); time.sleep(2)
stroke(wave(200, 1200, 800), pen=True); time.sleep(3)
pinch((400, 600), 200, 400); time.sleep(3)                 # apart: 200%
stroke([(900, 1400 - i * 30) for i in range(21)]); time.sleep(3)  # a finger, up: the page moves
stroke(wave(300, 900), pen=True); time.sleep(3)              # written zoomed
pinch((700, 900), 500, 150); time.sleep(3)                 # together: the fit
stroke([(1100 - i * 40, 1000) for i in range(21)]); time.sleep(3)  # a swipe: the next page
exit_app()
