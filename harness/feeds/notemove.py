# All Notebooks: move. A notebook written in; the list; a folder Work
# made; Folders; move on the notebook (m), into Work (Enter), move here
# (m): it's in Work. Work moved too: into Archive, made first, by taps.
# app: hellobro
import os, sys, math
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

PAD = 16
TOP = 40
L, R = PAD + min(230, (W - 2 * PAD) // 5) + 16, W - PAD - min(300, (W - 2 * PAD) // 4) - 16
def tab(k, n=3): return (L + (2 * k + 1) * (R - L) // (2 * n), TOP)
ALLNOTES = (110, 36)
NEWFOLDER = (370, 118)
def wave(y0):
    return [(W // 6 + (W * 2 // 3) * i / 40, y0 + 50 * math.sin(i / 40 * 4 * math.pi)) for i in range(41)]

start(); time.sleep(3)
gui("notebook"); time.sleep(3)
fullscreen(); time.sleep(4)
stroke(wave(H // 3), pen=True); time.sleep(3)
tap(*ALLNOTES); time.sleep(5)
tap(*NEWFOLDER); time.sleep(4); keys("Work"); time.sleep(2); key(ENTER); time.sleep(4)
tap(*NEWFOLDER); time.sleep(4); keys("Archive"); time.sleep(2); key(ENTER); time.sleep(4)
tap(*tab(2)); time.sleep(4)                            # Folders: Archive, Work, the notebook
key(108); key(108); time.sleep(1)                      # the notebook
key(50); time.sleep(4)                                 # m: move
key(108); time.sleep(1); key(ENTER); time.sleep(4)     # into Work
key(50); time.sleep(4)                                 # m: here
key(14); time.sleep(4)                                 # Backspace: up
key(108); time.sleep(1); key(50); time.sleep(4)        # Work: move
key(ENTER); time.sleep(4)                              # into Archive
tap(80, 118); time.sleep(4)                            # move here
key(ENTER); time.sleep(4)                              # into Work, in Archive
exit_app()
