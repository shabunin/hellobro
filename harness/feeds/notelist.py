# All Notebooks (notelist.go): a new notebook, written in, fullscreen;
# "‹ All Notebooks" at the bar's left: the list, with back to the
# notebook at its top right. + new folder: "Work" typed, Enter. Folders:
# Work, empty, and the notebook; F2 on it: renamed "plan". n: a new
# notebook, written in; ‹ All Notebooks again; Delete twice on it: to
# the trash. Enter on Work, + new notebook: one in it, written in; ‹ All
# Notebooks: Work's. Back to the notebook.
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
def wave(y0):
    return [(W // 6 + (W * 2 // 3) * i / 40, y0 + 50 * math.sin(i / 40 * 4 * math.pi)) for i in range(41)]
def write(y0):
    stroke(wave(y0), pen=True); time.sleep(3)

start(); time.sleep(3)
gui("notebook"); time.sleep(3)
fullscreen(); time.sleep(4)
write(H // 3)
tap(*ALLNOTES); time.sleep(5)                          # the list
tap(*NEWFOLDER); time.sleep(4)                         # + new folder
keys("Work"); time.sleep(2); key(ENTER); time.sleep(4)
tap(*tab(2)); time.sleep(4)                            # Folders
key(108); time.sleep(1)                                # the notebook
key(60); time.sleep(3)                                 # F2: rename
for _ in range(20): key(14)
keys("plan"); time.sleep(2); key(ENTER); time.sleep(4)
key(49); time.sleep(5)                                 # n: a new notebook
write(H // 2)
tap(*ALLNOTES); time.sleep(5)
key(108); time.sleep(1)                                # Work, the new one (by name), plan
key(111); time.sleep(3); key(111); time.sleep(4)       # Delete, Delete: to the trash
key(103); time.sleep(1)
key(ENTER); time.sleep(4)                              # Work
tap(130, 118); time.sleep(5)                           # + new notebook, in Work
write(H // 2)
tap(*ALLNOTES); time.sleep(5)                          # Work's
tap(*BACK); time.sleep(5)                              # back to the notebook
fullscreen(); time.sleep(3)
exit_app()
