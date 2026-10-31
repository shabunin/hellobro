# The bar fullscreen (toolbar.go's barOff): HELLOBRO_BOOKS shelf2's
# a.pdf then pooh.epub. The PDF fullscreen: no bar (the default, hidden
# for a PDF), a tap at the top for its contents; the EPUB fullscreen:
# its bar. A new notebook fullscreen: its bar; the setting stepped,
# none; fullscreen off: the bar again. snap.txt says which snapshot is
# which.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

OUT = os.path.dirname(os.path.abspath(sys.argv[1]))
notes = open(os.path.join(OUT, "snap.txt"), "w", buffering=1)
def note(what):
    n = len([f for f in os.listdir(os.path.join(OUT, "screen")) if f.endswith("after.png")])
    notes.write("%d %s\n" % (n, what))

KEY_Q = 16
start(); time.sleep(3)
gui("books"); time.sleep(4)
key(ENTER); time.sleep(8)                              # the PDF
fullscreen(); time.sleep(6); note("pdf fullscreen")
tap(W // 2, 40); time.sleep(4); note("pdf contents")
key(ESC); time.sleep(4)
key(KEY_Q); time.sleep(4)                              # the shelf
key(DOWN); key(ENTER); time.sleep(8); note("epub fullscreen")
fullscreen(); time.sleep(4)
mod(3); time.sleep(2)
gui("notebook"); time.sleep(5)
fullscreen(); time.sleep(5); note("notebook fullscreen")
gui("fullscreen notebooks"); time.sleep(6); note("notebook, no bar")
fullscreen(); time.sleep(5); note("notebook, a window")
exit_app()
