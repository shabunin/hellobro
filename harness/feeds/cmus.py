# For a screenshot: the Shell tab, cmus in it (CMUS: the command, say a
# container with music in it and a session bus that HELLOBRO_MPRIS_BUS
# names), the fifth track playing, then quit.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
key(3, ctrl=True, alt=True); time.sleep(3)            # the Shell tab
key(ENTER); time.sleep(4)                             # a shell
keys("exec " + os.environ.get("CMUS", "cmus") + "\n"); time.sleep(12)
key(TAB); time.sleep(1)                               # the tracks
for _ in range(4):
    key(DOWN); time.sleep(0.3)
key(ENTER); time.sleep(12)                            # play
keys("q"); time.sleep(1); keys("y"); time.sleep(4)
exit_app()
