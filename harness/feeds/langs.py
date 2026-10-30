# Languages to type in (APP_EXTRA a settings file with languages=en ru
# uk): the keyboard's Ctrl+Alt+Space goes to Russian, the bar says so,
# and the find line takes "ghbdtn" as привет; the on-screen keyboard
# (two fingers up) has Cyrillic letters, its language key goes to
# Ukrainian, then English. The log says "typing in ru", "uk", "en".
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start(); time.sleep(3)
key(SPACE, ctrl=True, alt=True); time.sleep(2)              # Russian
keys("ghbdtn"); time.sleep(2)                                # привет, in the find line
pair((W // 2 - 150, H - 30), (W // 2 + 150, H - 30), 0, -400); time.sleep(3)   # the keys
u = W / 15
h = min(max(min(W, H - 46) * 4 // 5 // 15, 56), 84)
tap(int(1.25 * u), H - h // 2); time.sleep(2)                # its language key: Ukrainian
tap(int(1.25 * u), H - h // 2); time.sleep(2)                # English
key(ESC); time.sleep(1)
exit_app()
