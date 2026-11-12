# The AI model's choices are the API's list (notediary.go: modelsWant),
# against a stand-in that lists models (ai.conf in APP_EXTRA, its base
# the stand-in's): the notebook's settings, the diary on, the model's
# dropdown open: as ai.conf says, then those that read pictures,
# cheapest first, with their prices.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
gui("notebook"); time.sleep(4)
fullscreen(); time.sleep(4)
keys(":settings\n"); time.sleep(4)                   # the list fetched as the settings show
for i in range(8): keys("j"); time.sleep(0.3)
key(28); time.sleep(2); key(103); time.sleep(0.5); key(28); time.sleep(3)   # the diary on
keys("jj"); time.sleep(1); key(28); time.sleep(4)    # its model: the list
keys("jjj"); time.sleep(2)
key(1); time.sleep(3)                                # Esc: as it was
exit_app()
