# The lock: Shift+Meta+X in a Kobo sh, then a tap (nothing; the hint),
# an x typed (nowhere), Meta+D (no switcher), Shift+Meta+X again:
# unlocked, and "echo ok" reaches the shell, without the x. The log says
# "lock: locked", "lock: unlocked".
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

KEY_X = 45
start(); time.sleep(3)
tab(2); time.sleep(3)                                 # a Kobo sh
mod(KEY_X, shift=True); time.sleep(4)                 # locked: the toast
tap(W // 2, H // 2); time.sleep(2)                    # nothing
key(KEY_X); time.sleep(1)                             # nowhere
mod(32); time.sleep(2)                                # no switcher
time.sleep(9)
tap(W // 2, H // 3); time.sleep(4)                    # the hint again, 10 s on
mod(KEY_X, shift=True); time.sleep(4)                 # unlocked
keys("echo ok\n"); time.sleep(3)
exit_app()
