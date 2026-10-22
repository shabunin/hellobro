# The tabs and keys, as set by the user: run with
# HB_TABS="ssh -shell settings" (Shell hidden, as by default). Settings' Tabs row opens the Tabs dialog: Shell shown, then
# moved to the front. The switcher lists commands; one hides Shell again.
# Then keys.conf, saved while running, gives F2 the Power tab.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

K2 = 3
app = os.path.join(os.path.dirname(os.path.abspath(sys.argv[1])), 'app')

start(); time.sleep(3)                                # SSH, three tabs
key(K2, ctrl=True, alt=True); time.sleep(4)           # Settings, second now
key(TAB); key(TAB); time.sleep(1)                     # the settings panel
key(END); key(UP)
time.sleep(1)                                         # the row before the last: Tabs
key(ENTER); time.sleep(3)                             # the dialog
key(UP); key(UP); time.sleep(1)                       # Shell
key(SPACE); time.sleep(4)                             # shown: four tabs
key(UP, shift=True); time.sleep(4)                    # moved to the front
key(ESC); time.sleep(3)
key(KEY_P, ctrl=True, shift=True); time.sleep(2)      # the switcher
keys("hide sh"); time.sleep(2)                        # a command
key(ENTER); time.sleep(4)                             # Shell hidden again
with open(os.path.join(app, 'keys.conf'), 'a') as f:
    f.write('f2 tab.power\nctrl+alt+1 -\n')
time.sleep(4)                                         # read again
key(F(2)); time.sleep(3)                              # Power
key(KEY_P, ctrl=True, shift=True); time.sleep(1)
keys("tab"); time.sleep(2)                            # the tabs and their keys
key(ESC); time.sleep(2)
exit_app()
