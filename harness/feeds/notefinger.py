# A finger in a notebook (run it with settings' note-finger=writes, and
# =turns, in APP_EXTRA): the pen writes a wave; a finger's wave under it
# writes too, or not; :new, then a finger swiped rightward: a line on
# the new page, or the page before.
# app: hellobro
import os, sys, math
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

def ex(s, wait=4):
    keys(":"); time.sleep(1.5); keys(s); time.sleep(2); key(ENTER); time.sleep(wait)
def wave(y0):
    return [(W // 6 + (W * 2 // 3) * i / 40, y0 + 50 * math.sin(i / 40 * 4 * math.pi)) for i in range(41)]

start(); time.sleep(3)
gui("notebook"); time.sleep(3)
fullscreen(); time.sleep(4)
stroke(wave(H // 3), pen=True); time.sleep(3)
stroke(wave(H // 2)); time.sleep(3)                                     # a finger
ex("new")
stroke([(W // 4 + i * 30, H * 2 // 3) for i in range(21)]); time.sleep(4)  # a finger, rightward
exit_app()
