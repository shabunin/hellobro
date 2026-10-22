# Sessions as panels: two SSH sessions to the test server (:2222) share
# the tab (landscape: side by side; portrait: one above the other);
# Ctrl+Tab moves the focus, and the keys follow it. The second folds to
# its strip and the first has it all; the switcher opens it again. Ctrl+Shift+F
# gives the one in focus the whole screen. Then Ctrl+Tab on the other
# tabs: Settings, Files.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

K3, K4 = 4, 5  # Settings, Files
L = W > H
X0, Y0, X1, Y1 = 16, 160, W - 16, H - 62 - 36   # the page's body, less the status bar

start(); time.sleep(3)
tap(tab_x(0), tabs_y); time.sleep(2)                  # SSH tab
keys("1"); key(ENTER); time.sleep(5)                              # session 1: saved host 1
key(KEY_T, shift=True, ctrl=True); time.sleep(2)      # session 2, beside it
keys("1"); key(ENTER); time.sleep(5)
keys("echo two\n"); time.sleep(2)                     # into session 2, the focused one
key(TAB, ctrl=True); time.sleep(2)                    # the focus: session 1
keys("echo one\n"); time.sleep(2)
# Session 2's − box: its panel's top right.
if L:
    right, top = X1, Y0
else:
    right, top = X1, Y0 + (Y1 - Y0) // 2 + 8
tap(right - 30, top + 26); time.sleep(3)              # session 2 folded
key(KEY_P, ctrl=True, shift=True); time.sleep(1)
keys("ssh 2"); time.sleep(2)
key(ENTER); time.sleep(3)                             # open again, in focus
fullscreen(); time.sleep(3)                               # fullscreen: session 2 alone
key(TAB, ctrl=True); time.sleep(3)                    # session 1, alone
fullscreen(); time.sleep(3)
key(K3, ctrl=True, alt=True); time.sleep(4)           # Settings
key(TAB, ctrl=True); time.sleep(2)                    # Bluetooth
key(TAB, ctrl=True, shift=True); time.sleep(2)        # Wi-Fi again
key(K4, ctrl=True, alt=True); time.sleep(3)           # Files
key(TAB, ctrl=True); time.sleep(2)                    # Sharing in focus
exit_app()
