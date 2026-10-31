# A PDF's cut kept through the workspaces' tiles: the first book, a PDF,
# full screen; the select tool, a rectangle; Meta+O, Esc: the rectangle
# and its buttons are still there. snap.txt says which snapshot is
# which. HELLOBRO_BOOKS: a directory with a PDF first by name.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

OUT = os.path.dirname(os.path.abspath(sys.argv[1]))
notes = open(os.path.join(OUT, "snap.txt"), "w", buffering=1)
def note(what):
    n = len([f for f in os.listdir(os.path.join(OUT, "screen")) if f.endswith("after.png")])
    notes.write("%d %s\n" % (n, what))

KEY_O = 24
start()
for _ in range(2): gui("fullscreen books")           # the bar: hidden for a PDF, hidden, shown
gui("books"); time.sleep(4)
key(ENTER); time.sleep(6)
fullscreen(); time.sleep(5)
tap(*barbtn(3)); time.sleep(3)                       # the select tool
half = int(min(W, H) * 0.2)
tap(W // 2 - half, int(H * 0.30)); time.sleep(3)
tap(W // 2 + half, int(H * 0.42)); time.sleep(4); note("cut")
mod(KEY_O); time.sleep(8); note("tiles")
key(ESC); time.sleep(4); note("back")
exit_app()
