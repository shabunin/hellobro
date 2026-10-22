# Removing Alpine: "Alpine: remove it" in the switcher, Enter.
# The question's yes waits for "yes delete it bro": half of it and Enter
# do nothing, all of it and Enter delete the image. Then the switcher's
# "Alpine: remove it…" finds none. Run it with a stand-in image and
# shell=alpine (APP_EXTRA), which stays so.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
gui("settings")
mod(32); time.sleep(1.2); keys("alpine: remove"); time.sleep(1)
key(ENTER); time.sleep(3)                                # the question
keys("yes delete"); time.sleep(2)
key(ENTER); time.sleep(2)                                # not yet
keys(" it bro"); time.sleep(3)
key(ENTER); time.sleep(4)                                # gone
key(KEY_P, ctrl=True, shift=True); time.sleep(1)
keys("remove"); time.sleep(2)
key(ENTER); time.sleep(3)                                # there's none
exit_app()
