# ink's gallery (cmd/gallery, ink/gallery): Tab rings the controls; the
# switches flipped and a value stepped by touch; a tab held (inverted
# while it's down); the list's page swiped up and back; the About tab;
# Esc quits. The page starts under the header and tabs (144 px down);
# places as ink lays them out at 1.42 px a Dp.
# app: gallery
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

Y = 144                                                # the page's top
start()
key(TAB); time.sleep(1.5)                              # the quit button, ringed
key(TAB); time.sleep(1.5)                              # the Settings tab
tap(W - 60, Y + 190); time.sleep(2)                    # Wi-Fi's switch: on
tap(W - 60, Y + 258); time.sleep(2)                    # Sync's: on
tap(W - 40, Y + 331); time.sleep(2)                    # the page size: ›
hold(W // 2, Y + 106, secs=0.4); time.sleep(3)         # "A list", pressed then let go
stroke([(W // 2, H * 3 // 4 - i * 60) for i in range(12)]); time.sleep(3)   # swiped up: the next page
stroke([(W // 2, H // 4 + i * 60) for i in range(12)]); time.sleep(3)       # back
tap(W * 5 // 6, Y + 106); time.sleep(3)                # "About"
key(ESC); time.sleep(3)                                # it quits
