# A notebook's paper, a tool (notepaper.go), and its settings
# (notesettings.go): a new notebook fullscreen; the paper's button: its
# choices under it; narrow dots, narrow squares, narrow lines, dots
# picked; the button again puts them away. The sliders: its settings;
# a finger stepped (Enter); Esc: the page. :settings again, q: the page;
# fullscreen off; :export png, dots and all. snap.txt says which
# snapshot is which.
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

KEY_Q, KEY_SEMI = 16, 39
start(); time.sleep(3)
gui("notebook"); time.sleep(5)
fullscreen(); time.sleep(5); note("notebook")
bar = buttons(0, 60)                                   # ‹ All Notebooks, pen, eraser, select, text, undo, redo, paste, paper, export, settings
tap(*bar[8]); time.sleep(4); note("paper's choices")
ch = buttons(62, 160)                                  # plain, lines, narrow lines, squares, narrow squares, dots, narrow dots
for k, what in ((6, "narrow dots"), (4, "narrow squares"), (2, "narrow lines"), (5, "dots")):
    tap(*ch[k]); time.sleep(4); note(what)
tap(*bar[8]); time.sleep(4); note("put away")
tap(*bar[10]); time.sleep(4); note("settings")
key(ENTER); time.sleep(3); note("a finger stepped")
key(ESC); time.sleep(4); note("the page again")
key(KEY_SEMI, shift=True); time.sleep(2); keys("settings"); key(ENTER); time.sleep(4); note(":settings")
key(KEY_Q); time.sleep(4); note("q: the page")
fullscreen(); time.sleep(4); note("a window")
key(KEY_SEMI, shift=True); time.sleep(2); keys("export png"); key(ENTER); time.sleep(10); note("exported")  # the dots in the PNG too
exit_app()
