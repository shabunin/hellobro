# Two Shell panes, one with output that never stops (a status line
# redrawn ten times a second, as mplayer's position), typing in the
# other: the busy one, not in focus, shows its output once a second and
# waits while keys come, so the panel isn't kept busy under the typing.
# The log's "blits:" lines say how busy it was.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
key(3, ctrl=True, alt=True); time.sleep(3)            # the Shell tab
key(ENTER); time.sleep(3)                             # a shell
keys("while :; do printf '\\rpos %s ' $(date +%S.%N | cut -c1-5); sleep 0.1; done\n"); time.sleep(4)
key(KEY_T, ctrl=True, shift=True); time.sleep(2)      # a second session, in focus
key(ENTER); time.sleep(3)
for ch in "echo typing along while the other one runs":
    keys(ch); time.sleep(0.25)
key(ENTER); time.sleep(5)                             # then the busy one shows once a second
exit_app()
