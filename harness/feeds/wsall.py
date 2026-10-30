# The workspaces' tiles, all live (workspaces.go): a shell counting on
# 1, a book on 2, the battery on 3. From 3 a tap on the bar puts them
# up, the whole screen, no line to type in: the shell's count goes on
# in its tile, the book shows its page. "n" makes 4 with a launcher;
# Meta+O, then "1", goes back to 1. The log says "workspace number 4",
# then "workspace number 1".
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

KEY_O = 24
start(); time.sleep(3)
keys("2"); time.sleep(3)                                     # the Kobo's sh
keys("i=0; while true; do i=$((i+1)); echo tick $i; sleep 1; done"); key(ENTER); time.sleep(2)
mod(3); time.sleep(2)                                        # 2: a book
gui("books"); time.sleep(3)
key(ENTER); time.sleep(5)
mod(4); time.sleep(2)                                        # 3: the battery
gui("battery"); time.sleep(4)
tap(W // 2, 20); time.sleep(4)                               # the tiles
time.sleep(7)                                                # the count goes on
keys("n"); time.sleep(4)                                     # 4, a launcher
mod(KEY_O); time.sleep(4)
keys("1"); time.sleep(4)                                     # back to 1
exit_app()
