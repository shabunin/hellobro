# The clipboard, in a Kobo sh: the pen drawn across output selects
# and copies it, a finger held still and then moved does too; Ctrl+Shift+V
# pastes it back. The switcher pastes and copies the Kobo's SSH key, and
# a program copying (OSC 52, from printf) fills the clipboard.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

K2, KV = 3, 47
Y = lambda row: GY + row * RH + RH // 2      # a terminal row's middle, roughly
GY, RH = int(os.environ.get('GY', 121)), int(os.environ.get('RH', 33))

start(); time.sleep(3)
key(K2, ctrl=True, alt=True); time.sleep(4)           # workspace 2, a Kobo sh
keys("clear; echo hello clipboard; echo second line\n"); time.sleep(3)
stroke([(30 + i * 40, Y(0)) for i in range(12)], pen=True); time.sleep(3)   # the pen across "hello clipboard"
keys("echo "); key(KV, ctrl=True, shift=True); keys("\n"); time.sleep(3)
tid[0] += 1                                            # a finger, held, then moved: the second line
frame(0, ID=tid[0], TOOL=0, **at(30, Y(1)), P=30); time.sleep(0.9)
for i in range(1, 12):
    frame(0, **at(30 + i * 40, Y(1)), P=30); time.sleep(0.02)
frame(0, ID=-1); time.sleep(3)
keys("echo "); key(KV, ctrl=True, shift=True); keys("\n"); time.sleep(3)
keys("echo ")
key(KEY_P, ctrl=True, shift=True); time.sleep(1)
keys("paste the kobo"); time.sleep(1); key(ENTER); time.sleep(1)   # the key, typed in
keys("\n"); time.sleep(3)
key(KEY_P, ctrl=True, shift=True); time.sleep(1)
keys("copy the kobo"); time.sleep(1); key(ENTER); time.sleep(3)
keys("printf '\\033]52;c;%s\\a' $(printf osc-works | base64)\n"); time.sleep(3)
exit_app()
