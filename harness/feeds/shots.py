# The README's screenshots, in landscape: Wi-Fi, Sharing, Bluetooth and
# Settings tiled two by two, then the switcher over them. No terminal:
# the harness's would show the PC's user and host. snap.txt in OUT says
# which snapshot is which.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

OUT = os.path.dirname(os.path.abspath(sys.argv[1]))
notes = open(os.path.join(OUT, "snap.txt"), "w", buffering=1)
def note(what):
    n = len([f for f in os.listdir(os.path.join(OUT, "screen")) if f.endswith("after.png")])
    notes.write("%d %s\n" % (n, what))

KEY_B, KEY_V, KEY_D = 48, 47, 32
start(); time.sleep(3)
key(ESC); time.sleep(2)                               # the launcher goes
gui("wi-fi"); mod(KEY_B); gui("sharing")              # side by side
mod(LEFT); mod(KEY_V); gui("bluetooth")               # under Wi-Fi
mod(RIGHT); mod(KEY_V); gui("settings")               # under Sharing
mod(LEFT); mod(UP); time.sleep(4)                     # the focus back on Wi-Fi
note("tiling")
mod(KEY_D); time.sleep(1.5); keys("wi"); time.sleep(3)
note("switcher")
key(ESC); time.sleep(2)
exit_app()
