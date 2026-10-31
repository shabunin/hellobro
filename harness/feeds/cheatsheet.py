# ?: the sheet of keys and commands. The first book (pooh.epub, with
# HELLOBRO_BOOKS shelf2's a.pdf then pooh.epub: the PDF's first), its
# sheet, a key takes it down; then a new notebook's sheet, a tap takes
# it down. snap.txt says which snapshot is which.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

OUT = os.path.dirname(os.path.abspath(sys.argv[1]))
notes = open(os.path.join(OUT, "snap.txt"), "w", buffering=1)
def note(what):
    n = len([f for f in os.listdir(os.path.join(OUT, "screen")) if f.endswith("after.png")])
    notes.write("%d %s\n" % (n, what))

KEY_SLASH = 53
start(); time.sleep(3)
gui("books"); time.sleep(4)
key(ENTER); time.sleep(8)                              # the PDF
key(KEY_SLASH, shift=True); time.sleep(4); note("pdf sheet")
key(SPACE); time.sleep(3); note("pdf back")
mod(3); time.sleep(2)
gui("notebook"); time.sleep(5)
key(KEY_SLASH, shift=True); time.sleep(4); note("notebook sheet")
tap(W // 2, H // 2); time.sleep(3); note("notebook back")
exit_app()
