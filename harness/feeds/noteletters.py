# The AI letters' page (noteletters.go), from a notebook's settings: the
# answers' size, a slider, and this notebook's font, each shown in
# itself at that size (Latin, Cyrillic, Greek). LETTERS_TAPS (x,y
# pairs) are tapped then: a font, the slider's track.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
gui("notebook"); time.sleep(4)
fullscreen(); time.sleep(4)
keys(":settings\n"); time.sleep(4)
for i in range(8): keys("j"); time.sleep(0.3)
key(28); time.sleep(2); key(103); time.sleep(0.5); key(28); time.sleep(3)   # the diary on
keys("jjj"); time.sleep(1); key(28); time.sleep(4)   # past the model: the letters' page
for p in os.environ.get("LETTERS_TAPS", "").split():
    x, y = (int(v) for v in p.split(","))
    tap(x, y); time.sleep(3)
key(1); time.sleep(3)                                # Esc: the settings again
exit_app()
