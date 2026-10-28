# The notebook in landscape, fullscreen: two pages side by side. A finger
# writes on the left one and the right one (no pen seen: fingers write),
# Right turns to the next two (blank), Left back. Then out of
# fullscreen: beside the launcher, one page.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

def circle(cx, cy, r, n=48):
    return [(cx + r * math.cos(i / n * 2 * math.pi), cy + r * math.sin(i / n * 2 * math.pi)) for i in range(n + 1)]

start(); time.sleep(3)
gui("notebook"); fullscreen(); time.sleep(4)
stroke(circle(W // 4, H // 2, 150)); time.sleep(3)
stroke([(W * 3 // 4 - 200 + i * 20, H // 2 + 80 * math.sin(i / 5)) for i in range(21)]); time.sleep(3)
key(RIGHT); time.sleep(4)
key(LEFT); time.sleep(4)
fullscreen(); time.sleep(4)
exit_app()
