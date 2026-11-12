# All Notebooks (notelist.go): a new notebook, written in, fullscreen;
# "‹ All Notebooks" at the bar's left: the list, with back to the
# notebook at its top right. + new folder: "Work" typed, Enter. Folders:
# Work and the notebook. The notebook held: picked, boxes on the rows;
# rename (one picked): "plan". Held again, Work ticked too: no rename
# then; Work unticked, copy; Work opened, paste here: a copy in it.
# Up, plan held, delete: asked, d: to the trash. Back to the notebook.
# app: hellobro
import os, sys, math
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

PAD = 16
TOP = 40
BACK = (W - PAD - min(300, (W - 2 * PAD) // 4) // 2, TOP)
L, R = PAD + min(230, (W - 2 * PAD) // 5) + 16, W - PAD - min(300, (W - 2 * PAD) // 4) - 16
def tab(k, n=3): return (L + (2 * k + 1) * (R - L) // (2 * n), TOP)
ALLNOTES = (110, 36)                                   # the notebook bar's first button
NEWFOLDER = (370, 118)                                 # under the header, the second
ROW0 = int(os.environ.get('ROW0', 200))                # the first row's middle
ROWH = int(os.environ.get('ROWH', 70))
def row(i): return (W // 2, ROW0 + i * ROWH)
TOOLS = [70, 205, 320, 442, 580]                       # rename, copy, cut, delete, cancel (one picked)
def tool(i): return (TOOLS[i], 118)
def wave(y0):
    return [(W // 6 + (W * 2 // 3) * i / 40, y0 + 50 * math.sin(i / 40 * 4 * math.pi)) for i in range(41)]

start(); time.sleep(3)
gui("notebook"); time.sleep(3)
fullscreen(); time.sleep(4)
stroke(wave(H // 3), pen=True); time.sleep(3)
tap(*ALLNOTES); time.sleep(5)                          # the list
tap(*NEWFOLDER); time.sleep(4)                         # + new folder
keys("Work"); time.sleep(2); key(ENTER); time.sleep(4)
tap(*tab(2)); time.sleep(4)                            # Folders: Work, the notebook
hold(*row(1), secs=1.0); time.sleep(3)                 # picked
tap(*tool(0)); time.sleep(3)                           # rename
for _ in range(20): key(14)
keys("plan"); time.sleep(2); key(ENTER); time.sleep(4)
hold(*row(1), secs=1.0); time.sleep(2)                 # plan, picked
tap(*row(0)); time.sleep(3)                            # Work too: no rename
tap(*row(0)); time.sleep(2)                            # Work not
tap(*tool(1)); time.sleep(3)                           # copy
tap(*row(0)); time.sleep(4)                            # Work, opened
tap(47, 118); time.sleep(4)                            # paste here
key(14); time.sleep(4)                                 # up
hold(*row(1), secs=1.0); time.sleep(2)                 # plan
tap(*tool(3)); time.sleep(3)                           # delete: asked
key(32); time.sleep(4)                                 # d
tap(*BACK); time.sleep(5)                              # back to the notebook
exit_app()
