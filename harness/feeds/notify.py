# A terminal calls out: an OSC 9 message on the Shell tab, which shows,
# blinks its title strip; a bell from it while Settings is up puts a
# note in the corner for a few seconds.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
key(3, ctrl=True, alt=True); time.sleep(3)            # Shell
keys("printf '\\033]9;build done\\007'\n"); time.sleep(4)   # shows: the strip blinks
keys("sleep 4; printf '\\a'\n"); time.sleep(0.5)
key(5, ctrl=True, alt=True); time.sleep(7)            # Settings: the bell comes, a note
exit_app()
