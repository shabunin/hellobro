# The Power tab: Battery (with a history from APP_EXTRA's
# battery-history, if any) and What uses it, sampled every 5 s. Ctrl+Tab
# moves the focus, Ctrl+Shift+F gives each the screen, Ctrl+Shift+M
# folds the battery. The battery is the harness's stand-in.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

K5 = 6  # Power

start(); time.sleep(3)
key(K5, ctrl=True, alt=True); time.sleep(12)          # Power: two samples in
key(TAB, ctrl=True); time.sleep(2)                    # the focus: What uses it
fullscreen(); time.sleep(7)                           # it alone
key(TAB, ctrl=True); time.sleep(3)                    # the battery alone
fullscreen(); time.sleep(3)
key(KEY_M, ctrl=True, shift=True); time.sleep(7)      # the battery (in focus again) folded
exit_app()
