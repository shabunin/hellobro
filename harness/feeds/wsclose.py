# The workspaces' ×: Wi-Fi and Battery on 2, Settings on 3; Meta+O puts
# the tiles up. x on 3 (the arrows there) asks, x again closes its
# window; then 2's × tapped (at CLOSE_XY, "x,y") asks, and again closes
# both. The log says "workspaces: closing 3, 1 windows", then "closing
# 2, 2 windows"; snap.txt says which snapshot is which.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

OUT = os.path.dirname(os.path.abspath(sys.argv[1]))
notes = open(os.path.join(OUT, "snap.txt"), "w", buffering=1)
def note(what):
    n = len([f for f in os.listdir(os.path.join(OUT, "screen")) if f.endswith("after.png")])
    notes.write("%d %s\n" % (n, what))

KEY_O, KEY_X = 24, 45
close = tuple(int(v) for v in os.environ.get("CLOSE_XY", "1330,40").split(","))
start(); time.sleep(3)
mod(3); time.sleep(2)                                         # 2
gui("wi-fi"); gui("battery"); time.sleep(2)
mod(4); time.sleep(2)                                         # 3
gui("settings"); time.sleep(2)
mod(KEY_O); time.sleep(5); note("up")
key(KEY_X); time.sleep(3); note("3 asks")
key(KEY_X); time.sleep(6); note("3 closed")
tap(*close); time.sleep(3); note("2 asks")
tap(*close); time.sleep(6); note("2 closed")
key(ESC); time.sleep(2)
exit_app()
