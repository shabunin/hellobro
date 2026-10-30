# The workspaces' tiles show what's in the windows (workspaces.go): a
# shell on 1 counting, a launcher on 2. From 2, a tap on the bar puts
# the tiles up: 1's shell drawn from its cells, the launcher as it was
# last on screen. A few seconds on, 1's tile shows the count gone up.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start(); time.sleep(3)
keys("2"); time.sleep(3)                                     # the Kobo's sh
keys("ls /; i=0; while true; i=$((i+1)); do echo tick $i; sleep 1; done"); key(ENTER); time.sleep(2)
mod(3); time.sleep(2)                                        # workspace 2: a launcher
mod(ENTER); time.sleep(3)
tap(W // 2, 20); time.sleep(3)                               # the tiles
time.sleep(5)                                                # 1's count goes on
key(ESC); time.sleep(2)
exit_app()
