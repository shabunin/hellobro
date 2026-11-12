# The AI models' window (aimodels.go), against a stand-in that lists
# models (ai.conf in APP_EXTRA, its base the stand-in's): from the
# notebook's settings, the diary on, the AI model's row: the window, the
# list fetched; full screen. MODELS_TAPS (x,y pairs) are tapped then:
# a row, a chip, a head; MODELS_FIND typed in the search.
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
keys("jj"); time.sleep(1); key(28); time.sleep(5)    # the model's row: its window
fullscreen(); time.sleep(4)
for p in os.environ.get("MODELS_TAPS", "").split():
    x, y = (int(v) for v in p.split(","))
    tap(x, y); time.sleep(3)
if os.environ.get("MODELS_FIND"):
    keys(os.environ["MODELS_FIND"]); time.sleep(3)
exit_app()
