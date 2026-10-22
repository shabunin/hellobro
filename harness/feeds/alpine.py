# The Alpine shell, as far as a PC shows it: Settings, "Shell tab runs"
# to Alpine (the 14th row), system bus access on (the next), then the
# Shell tab says so, and Enter starts
# a session, which can't be a jail here (no root): it says why and ends.
# On the Kobo: ssh -t -p 2022 kobo.local hellobro-alpine.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
key(5, ctrl=True, alt=True); time.sleep(3)            # Settings
key(TAB, ctrl=True); key(TAB, ctrl=True); time.sleep(2)  # the settings panel
for _ in range(13):
    key(DOWN); time.sleep(0.2)
time.sleep(2)
key(RIGHT); time.sleep(3)                             # Shell tab runs: Alpine
key(DOWN); key(RIGHT); time.sleep(3)                  # … Alpine: system bus access, on
key(3, ctrl=True, alt=True); time.sleep(3)            # the Shell tab
key(ENTER); time.sleep(6)                             # a session: no jail on a PC
exit_app()
