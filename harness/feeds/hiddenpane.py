# A Shell session with output that never stops (a status line twenty times
# a second, as mplayer's), left running behind the Power tab: what that
# costs hellobro with nothing of it on screen (hellobro-profile meanwhile).
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
key(3, ctrl=True, alt=True); time.sleep(3)            # the Shell tab
key(ENTER); time.sleep(3)                             # a shell
if os.environ.get('QUIET') != '1':
    keys("while :; do printf '\\rA: %s ' $(date +%S.%N | cut -c1-5); sleep 0.05; done\n")
time.sleep(4)
key(5, ctrl=True, alt=True)                           # the Power tab
time.sleep(int(os.environ.get('HOLD', '60')))
key(3, ctrl=True, alt=True); time.sleep(3)            # back: the line is as new as ever
exit_app()
