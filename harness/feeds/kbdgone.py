# The keyboard gone, unlocked: back within 15 s, nothing shows; gone
# longer, the Bluetooth window comes up full screen, and closes when the
# keyboard's back. With Bluetooth open already (not in focus: in focus,
# it's left be), it goes full screen and back to its place. The log says "keyboard: still gone" twice, "wm:
# opened Bluetooth", "wm: closing Bluetooth", fullscreen true and false.
# Then Settings' "The keyboard gone" stepped to nothing (three times from
# the switcher: 30 s, 1 min, nothing): gone 20 s, nothing comes up. The
# log says "settings: kbd-gone = off".
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
import feedlib
from feedlib import *

def gone(s): feedlib.kbd.close(); time.sleep(s)
def back(): feedlib.kbd = open(os.path.join(feedlib.d, 'event4'), 'wb', buffering=0); time.sleep(4)

start(); time.sleep(3)
gone(4); back()                   # a nap: nothing
gone(20)                          # the Bluetooth window, full screen
back()                            # closed
gui("bluetooth"); time.sleep(3)   # open already
mod(105); mod(103); time.sleep(2) # the focus on the launcher, left of it or above
gone(20)                          # full screen
back()                            # back in its place
for _ in range(3):
    mod(32); time.sleep(1.2); keys("keyboard gone"); time.sleep(0.6); key(ENTER); time.sleep(2)
gone(20)                          # nothing
back()
exit_app()
