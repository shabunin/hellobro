# The on-screen keyboard for text fields (the setting's first): with the
# keyboard there, two fingers up from the screen's foot show it anyway,
# full screen too, and two down onto it hide it. With the keyboard gone
# it doesn't come by itself; a tap on the launcher's find line brings
# it, two fingers down put it away, and the keyboard back stays as it
# is. APP_EXTRA a settings file with kbd-gone=off. The log says "two-finger swipe: on-screen keyboard true", false,
# and "on-screen keyboard: true" for the find line.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
import feedlib
from feedlib import *

start(); time.sleep(3)
up = lambda: pair((W // 2 - 150, H - 30), (W // 2 + 150, H - 30), 0, -400)
down = lambda: pair((W // 2 - 150, H // 2), (W // 2 + 150, H // 2), 0, H // 2 - 200)
up(); time.sleep(3)                                       # shown
down(); time.sleep(3)                                     # hidden
fullscreen(); time.sleep(3)
up(); time.sleep(3)                                       # shown, full screen
down(); time.sleep(3)
fullscreen(); time.sleep(3)
feedlib.kbd.close(); time.sleep(14)                       # no keyboard: still none on the screen
tap(W // 2, H - 185); time.sleep(3)                     # the find line (Sizes for fingers): it shows
down(); time.sleep(3)
feedlib.kbd = open(os.path.join(feedlib.d, 'event4'), 'wb', buffering=0); time.sleep(4)
exit_app()
