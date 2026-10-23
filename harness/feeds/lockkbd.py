# The lock with the keyboard gone: Shift+Meta+X, then the keyboard goes
# (its FIFO closed): the lock screen lists its swipes. A short swipe up
# and a slant do nothing; a swipe right unlocks and opens Bluetooth. The
# keyboard comes back, locks again and goes: a swipe up unlocks. Locked
# once more, a swipe down goes back to Nickel (the app quits). The log
# says "lock: swiped →, Bluetooth", "lock: swiped ↑, unlock",
# "lock: swiped ↓, back to Nickel".
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
import feedlib
from feedlib import *

KEY_X = 45
def swipe(dx, dy):
    x, y = W // 2, H // 2
    stroke([(x + dx * i // 20, y + dy * i // 20) for i in range(21)]); time.sleep(4)
def gone(): feedlib.kbd.close(); time.sleep(5)
def back(): feedlib.kbd = open(os.path.join(feedlib.d, 'event4'), 'wb', buffering=0); time.sleep(4)
def lock(): mod(KEY_X, shift=True); time.sleep(4)

start(); time.sleep(3)
lock(); gone()                    # the swipes listed
swipe(0, -200)                    # a brush: nothing
swipe(400, -500)                  # a slant: nothing
swipe(500, 0)                     # Bluetooth
back(); lock(); gone()
swipe(0, -500)                    # unlocked
back(); lock(); gone()
swipe(0, 500)                     # back to Nickel
