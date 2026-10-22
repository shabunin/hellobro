# Typing in a Shell: the rows drawn with the fast waveform get a flash of
# their own (GC16 -f in calls.txt) once the pane is still, the setting's
# 10 s after the first of them, and not before.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
key(3, ctrl=True, alt=True); time.sleep(3)            # the Shell tab
key(ENTER); time.sleep(3)                             # a shell
for ch in "echo faint till the flash":
    keys(ch); time.sleep(0.3)
key(ENTER); time.sleep(14)                            # the flash, a few seconds after the echo
exit_app()
