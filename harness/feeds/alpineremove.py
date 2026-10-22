# Removing Alpine: Settings, "… Alpine: remove it" (the 19th row), Enter.
# The question's yes waits for "yes delete it bro": half of it and Enter
# do nothing, all of it and Enter delete the image. Then the switcher's
# "Alpine: remove it…" finds none. Run it with a stand-in image and
# shell=alpine (APP_EXTRA), which goes back to kobo.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
key(4, ctrl=True, alt=True); time.sleep(3)               # Settings
for _ in range(3):
    key(TAB, ctrl=True); time.sleep(0.5)                 # the settings panel
for _ in range(18):
    key(DOWN); time.sleep(0.2)
time.sleep(2)
key(ENTER); time.sleep(3)                                # the question
keys("yes delete"); time.sleep(2)
key(ENTER); time.sleep(2)                                # not yet
keys(" it bro"); time.sleep(3)
key(ENTER); time.sleep(4)                                # gone
key(KEY_P, ctrl=True, shift=True); time.sleep(1)
keys("remove"); time.sleep(2)
key(ENTER); time.sleep(3)                                # there's none
exit_app()
