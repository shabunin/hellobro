# The notebook: a new one beside the launcher, written in with the pen
# (a wave, pressed harder along it; a circle), then fullscreen: more,
# the eraser (E) through the wave, back to the pen (P), the pen's
# button held through the circle (erasing it), Ctrl+Z (the circle back).
# A finger swiped leftward turns to a new page, written on; rightward
# back. The log says "notebook ... kept"; OUT/app/notebooks has it.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

def wave(y0, x0, x1, amp=60, n=60):
    return [(x0 + (x1 - x0) * i / n, y0 + amp * math.sin(i / n * 4 * math.pi)) for i in range(n + 1)]
def circle(cx, cy, r, n=48):
    return [(cx + r * math.cos(i / n * 2 * math.pi), cy + r * math.sin(i / n * 2 * math.pi)) for i in range(n + 1)]
def stylus(on):                                    # the pen's side button
    touch.write(ev(1, 0x14b, 1 if on else 0) + SYN); time.sleep(0.05)

start(); time.sleep(3)
gui("notebook")                                    # beside the launcher
stroke(wave(H * 3 // 4, W * 5 // 8, W - 120), pen=True, pressure=lambda t: 200 + int(3800 * t)); time.sleep(3)
fullscreen(); time.sleep(4)
stroke(wave(H // 3, 200, W - 200), pen=True, pressure=lambda t: 2000); time.sleep(1)
stroke(circle(W // 2, H // 2 + 150, 120), pen=True, pressure=lambda t: 3000); time.sleep(1)
stroke([(300 + i * 8, 300 + i * 9) for i in range(40)], pen=True, pressure=lambda t: 1500); time.sleep(3)
key(18); time.sleep(1)                             # E: the eraser
stroke([(W // 2 - 40 + i, H // 3 - 200 + i * 12) for i in range(35)], pen=True); time.sleep(3)
key(25); time.sleep(1)                             # P: the pen
stylus(True)
stroke([(W // 2 + 120, H // 2 + 50 + i * 10) for i in range(20)], pen=True); time.sleep(3)
stylus(False)
key(44, ctrl=True); time.sleep(3)                  # Ctrl+Z: the circle back
stroke([(W * 3 // 4 - i * 25, H // 2) for i in range(20)]); time.sleep(4)   # leftward: a new page
stroke(circle(W // 2, H // 2, 200), pen=True, pressure=lambda t: 2500); time.sleep(3)
stroke([(W // 4 + i * 25, H // 2) for i in range(20)]); time.sleep(4)       # rightward: back
fullscreen(); time.sleep(4)
exit_app()
