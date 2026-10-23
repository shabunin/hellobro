# A window's buttons: Bluetooth beside the launcher; a tap on its float
# (FLOAT_X, FLOAT_Y) floats it, a tap on tile (TILE_X, TILE_Y, where its
# strip is then) puts it back; fullscreen (FULL_X, FULL_Y), and a swipe
# in from the top edge out. Then three windows side by side: too narrow
# for the words, their buttons are icons only. The log says fullscreen
# true and false.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

def at(name):
    x = os.environ.get(name + "_X")
    if x:
        tap(int(x), int(os.environ[name + "_Y"])); time.sleep(4)

start(); time.sleep(3)
gui("bluetooth"); time.sleep(3)
at("FLOAT")
at("TILE")
at("FULL")
stroke([(W // 2, 10 + 30 * i) for i in range(12)]); time.sleep(4)
KEY_B = 48
mod(KEY_B); mod(ENTER); time.sleep(3); mod(ENTER); time.sleep(4)   # side by side: narrow
exit_app()
