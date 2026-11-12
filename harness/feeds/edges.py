# The edges' strips, held alike (main.go: edgeHeld): over the Settings
# window, full screen, a finger up the left edge sets the light, and
# one down the right edge opens the menu (Esc: gone); a tap in either
# strip is the window's: the Settings' rows there are tapped (EDGE_TAPS,
# x,y pairs). The log says which.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start(); time.sleep(2)
gui("settings"); time.sleep(4)
fullscreen(); time.sleep(3)
stroke([(40, H * 2 // 3 - i * 20) for i in range(30)]); time.sleep(3)      # the light, up
stroke([(W - 40, H // 3 + i * 20) for i in range(30)]); time.sleep(3)      # the menu
key(ESC); time.sleep(2)
for p in os.environ.get("EDGE_TAPS", "").split():
    x, y = (int(v) for v in p.split(","))
    tap(x, y); time.sleep(3)
exit_app()
