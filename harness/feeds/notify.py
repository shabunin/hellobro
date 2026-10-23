# A terminal calls out: an OSC 9 message in a Kobo sh, which shows,
# blinks its title strip; a bell from it while workspace 2 is up puts a
# note in the corner for a few seconds. The log says
# "terminal: Kobo sh: bell".
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
keys("2"); time.sleep(3)                              # the launcher: a Kobo sh
keys("printf '\\033]9;build done\\007'\n"); time.sleep(4)   # shows: the strip blinks
keys("sleep 4; printf '\\a'\n"); time.sleep(0.5)
mod(3); time.sleep(7)                                 # workspace 2: the bell comes, a note
mod(2); time.sleep(2)
exit_app()
