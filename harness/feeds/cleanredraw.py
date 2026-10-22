# Typing in a Shell: the rows drawn with the fast waveform get a flash of
# their own (GC16 -f in calls.txt) every 10 s (the setting) while
# there are any: the first 10 s after the first of them.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
key(3, ctrl=True, alt=True); time.sleep(3)            # the Shell tab
key(ENTER); time.sleep(3)                             # a shell
for ch in "echo faint till the flash, typed slowly":
    keys(ch); time.sleep(0.6)                         # 23 s of typing: flashes in it
key(ENTER); time.sleep(14)                            # and one for the echo
exit_app()
