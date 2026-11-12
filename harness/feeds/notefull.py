# A notebook full screen, left by touch (main.go, workspaces.go): written
# in with the pen (so fingers turn pages), a finger swiped down from the
# top hides its bar, again shows it; two fingers swiped down from the
# top leave full screen. Full screen again: a finger swiped along the
# right edge brings the menu; Esc. Three fingers swiped down from the
# top: the workspaces. A tap on the bar's ‹ All Notebooks, full screen:
# the list.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

def down(x, n=20):
    stroke([(x, 15 + i * 25) for i in range(n)])
def trio(n=20):
    tid[0] += 3
    for k, x in enumerate((W // 4, W // 2, W * 3 // 4)):
        frame(k, ID=tid[0] - k, TOOL=0, **at(x, 15), P=30)
    for i in range(1, n + 1):
        for k, x in enumerate((W // 4, W // 2, W * 3 // 4)):
            frame(k, **at(x, 15 + i * 25), P=30)
        time.sleep(0.02)
    for k in range(3):
        frame(k, ID=-1)
    time.sleep(2)

start(); time.sleep(3)
gui("notebook"); time.sleep(3)
fullscreen(); time.sleep(4)
stroke([(W // 3 + i * 10, H // 2) for i in range(30)], pen=True, pressure=lambda t: 2500); time.sleep(2)
down(W // 2); time.sleep(3)                 # the bar: hidden
down(W // 2); time.sleep(3)                 # shown
pair((W // 3, 15), (W * 2 // 3, 15), 0, 500); time.sleep(4)   # out of full screen
fullscreen(); time.sleep(4)
stroke([(W - 20, H // 2 + 300 - i * 15) for i in range(30)]); time.sleep(3)   # the menu
key(ESC); time.sleep(2)
trio(); time.sleep(4)                       # the workspaces
key(ESC); time.sleep(2)
tap(70, 25); time.sleep(4)                  # the bar's ‹ All Notebooks: still a tap's
exit_app()
