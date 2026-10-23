# Sleep when idle: "on battery" stepped to 5 min through the switcher (30,
# 60, 120, off, 5), a Kobo sh open; the toast a minute before, a key that
# keeps it awake, then sleep (pretended) after 5 min untouched. The log
# says when ("sleep: 5m0s without input"). TIMEOUT=650.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

KEY_X = 45
start(); time.sleep(3)
for _ in range(4):
    mod(32); time.sleep(1.2); keys("on battery"); time.sleep(0.6); key(ENTER); time.sleep(2.5)
tab(2); time.sleep(3)                                 # a Kobo sh: no auto-exit for 20 min
time.sleep(4 * 60 + 10)                               # the toast: a minute to go
key(KEY_X); time.sleep(5)                             # stays awake
time.sleep(5 * 60 + 10)                               # sleeps, wakes (pretended)
exit_app()
