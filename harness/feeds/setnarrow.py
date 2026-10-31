# Settings narrow: Wi-Fi, Battery and Settings side by side, a third
# of the screen each. Its long values are cut short ("…") and its
# labels show whole where they can.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start(); time.sleep(3)
mod(3); time.sleep(2)
gui("wi-fi"); gui("battery"); gui("settings"); time.sleep(4)
for _ in range(14): key(DOWN); time.sleep(0.3)        # down to the keyboard's rows
time.sleep(3)
exit_app()
