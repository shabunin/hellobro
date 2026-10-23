# Battery (with a history from APP_EXTRA's battery-history, if any) and
# What uses it, side by side (the launcher closed first), sampled every
# 5 s. Meta+Right moves the focus to What uses it, Meta+F gives it the
# screen and back; then the same for Battery. The battery is the
# harness's stand-in.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start(); time.sleep(3)
mod(16, shift=True); time.sleep(2)                    # the launcher closed
gui("battery"); mod(48); gui("what uses")             # Meta+B: beside it
time.sleep(10)                                        # two samples in
mod(LEFT); time.sleep(2)                              # the focus: Battery
mod(RIGHT); time.sleep(2)                             # What uses it
fullscreen(); time.sleep(7)                           # it alone
fullscreen(); time.sleep(3)
mod(LEFT); time.sleep(2)                              # Battery
fullscreen(); time.sleep(5)                           # alone
fullscreen(); time.sleep(3)
exit_app()
