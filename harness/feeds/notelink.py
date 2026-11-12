# Links between notebooks (notelink.go): a text typed in one, taken
# with the select tool, :mark; a second notebook in its place (All
# Notebooks, n, Enter), :link: the marks (this notebook's only: none;
# the tick taken away, the first's), "link here", a tap: the link. A finger's tap on it: the first notebook
# in its place, the text framed; :back. Held: the first in a window of
# its own (it's open, so it's focused there).
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

def ex(s, wait=4):
    keys(":"); time.sleep(1.5); keys(s); time.sleep(2); key(ENTER); time.sleep(wait)
KEY_T, KEY_S = 20, 31
LINK = (int(os.environ.get('LINK_X', 332)), int(os.environ.get('LINK_Y', 330)))
AT = (W // 3, H // 2)

start(); time.sleep(3)
gui("notebook"); time.sleep(3)
fullscreen(); time.sleep(3)
key(KEY_T); time.sleep(2)
tap(W // 5, H // 4); time.sleep(2)
keys("the budget"); time.sleep(2)
key(ESC); time.sleep(2)
key(KEY_S); time.sleep(2)
tap(W // 5 + 30, H // 4 + 10); time.sleep(3)             # the text, taken
ex("mark")
key(ESC); time.sleep(1)
key(16); time.sleep(3)                                   # q: All Notebooks
key(49); time.sleep(3)                                   # n: a new one, the wizard
key(ENTER); time.sleep(4)                                # the second, plain
ex("link", wait=4)                                       # the marks: this notebook's, none
tap(55, 178); time.sleep(3)                              # all of them
tap(*LINK); time.sleep(3)                                # link here
tap(*AT); time.sleep(3)                                  # put there
tap(AT[0] + 40, AT[1] + 15); time.sleep(5)               # followed
ex("back", wait=5)
hold(AT[0] + 40, AT[1] + 15, secs=1.0); time.sleep(5)   # held: apart
exit_app()
