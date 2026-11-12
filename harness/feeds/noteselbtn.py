# The select tool's buttons by what's taken, in a notebook: one held
# (dark while the finger's on it, nothing done as it's no tap), then the
# ribbon tapped (marked: filled) and tapped again (unmarked).
# app: hellobro
import os, sys, math
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

def wave(x0, y0):
    return [(x0 + 400 * i / 40, y0 + 40 * math.sin(i / 40 * 4 * math.pi)) for i in range(41)]
BTN_Y = int(os.environ.get('BTN_Y', 603))
def btn(i): return (int(os.environ.get('BTN_X', 229)) + 94 * i, BTN_Y)   # copy cut duplicate turn mark delete, compact

start(); time.sleep(3)
gui("notebook"); time.sleep(3)
fullscreen(); time.sleep(3)
stroke(wave(200, 500), pen=True); time.sleep(2)          # the pen: fingers turn now
key(25); time.sleep(1)                                   # P: the pen in hand
hold(400, 500, secs=1.0); time.sleep(3)                  # held on the wave: taken
hold(*btn(0), secs=2.5); time.sleep(2)                   # copy, held: dark meanwhile, not done
tap(*btn(4)); time.sleep(3)                              # the ribbon: marked
tap(*btn(4)); time.sleep(3)                              # again: unmarked
exit_app()
