# The bar fullscreen (toolbar.go's barOff), as each app's own settings
# say: HELLOBRO_BOOKS shelf2's a.pdf then pooh.epub. The PDF fullscreen:
# no bar (the default), a tap at the top for its contents; s, its style
# panel's Bar, fullscreen flipped: the bar. The EPUB fullscreen: its
# bar, and its style panel's row. A new notebook fullscreen: its bar;
# its settings (the sliders) shown, the bar flipped there, Esc: no bar;
# fullscreen off: the bar again. snap.txt says which snapshot is which.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

OUT = os.path.dirname(os.path.abspath(sys.argv[1]))
notes = open(os.path.join(OUT, "snap.txt"), "w", buffering=1)
def note(what):
    n = len([f for f in os.listdir(os.path.join(OUT, "screen")) if f.endswith("after.png")])
    notes.write("%d %s\n" % (n, what))

def buttons(y0, y1):
    # the widest row of black runs (buttons' top frames) between y0 and y1
    d = os.path.join(OUT, "screen")
    w, h = map(int, open(os.path.join(d, "dims")).read().split("x"))
    raw = open(os.path.join(d, "screen.raw"), "rb").read()
    best = []
    for y in range(y0, y1):
        row, runs, x = raw[y * w:(y + 1) * w], [], 0
        while x < w:
            if row[x] < 0x40:
                s = x
                while x < w and row[x] < 0x40: x += 1
                if 40 < x - s < 300: runs.append(((s + x) // 2, y + 20))
            x += 1
        if len(runs) > len(best): best = runs
    return best

KEY_Q, KEY_S = 16, 31
start(); time.sleep(3)
gui("books"); time.sleep(4)
key(ENTER); time.sleep(8)                              # the PDF
fullscreen(); time.sleep(6); note("pdf fullscreen")
tap(W // 2, 40); time.sleep(4); note("pdf contents")
key(ESC); time.sleep(4)
key(KEY_S); time.sleep(4)                              # its style panel
for _ in range(4): key(DOWN); time.sleep(0.5)
key(ENTER); time.sleep(4); note("pdf style: bar shown")
key(ESC); time.sleep(5); note("pdf fullscreen, its bar")
key(KEY_S); time.sleep(3)
for _ in range(4): key(DOWN); time.sleep(0.5)
key(ENTER); time.sleep(3); key(ESC); time.sleep(4)     # hidden again, the default
key(KEY_Q); time.sleep(4)                              # the shelf
key(DOWN); key(ENTER); time.sleep(8); note("epub fullscreen")
key(KEY_S); time.sleep(4); note("epub style"); key(ESC); time.sleep(4)
fullscreen(); time.sleep(4)
mod(3); time.sleep(2)
gui("notebook"); time.sleep(5)
fullscreen(); time.sleep(5); note("notebook fullscreen")
tap(*buttons(0, 60)[10]); time.sleep(4); note("notebook settings")  # ‹ All Notebooks, pen, …, paper, export, settings
key(DOWN); key(DOWN); time.sleep(1); key(ENTER); time.sleep(4); note("settings: bar hidden")
key(ESC); time.sleep(5); note("notebook, no bar")
fullscreen(); time.sleep(5); note("notebook, a window")
exit_app()
