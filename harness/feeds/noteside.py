# A notebook in a narrow window (run with HELLOBRO_TOUCH= : for fingers):
# beside All Books, its tools stand down the left; the pen writes, the
# pen's choices open by it (a tap on the pen, again), the overview (O)
# has its own tools there too, and back.
# app: hellobro
import os, sys, math
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

def wave(x0, x1, y0):
    return [(x0 + (x1 - x0) * i / 40, y0 + 40 * math.sin(i / 40 * 4 * math.pi)) for i in range(41)]

start(); time.sleep(3)
gui("books"); time.sleep(3)
gui("notebook"); time.sleep(4)
PEN = (int(os.environ.get("PEN_X", "0")), int(os.environ.get("PEN_Y", "0")))
if PEN[0]:
    tap(*PEN); time.sleep(2); tap(*PEN); time.sleep(3)  # the pen's choices
    key(ESC); time.sleep(2)
stroke(wave(W * 3 // 4, W - 80, H // 2), pen=True); time.sleep(3)
key(24); time.sleep(4)                                  # the overview
key(24); time.sleep(3)
exit_app()
