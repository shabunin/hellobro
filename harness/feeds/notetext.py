# A notebook's text tool: T, a tap on the page, "hello bro", Enter,
# "typed on the Kobo"; Up, End, "!"; Esc. The pen writes under it; the
# select tool (S) takes the text with a tap; Esc. :text, a tap in the
# text: End, " :)", Esc.
# app: hellobro
import os, sys, math
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

def ex(s, wait=4):
    keys(":"); time.sleep(1.5); keys(s); time.sleep(2); key(ENTER); time.sleep(wait)
def wave(y0):
    return [(W // 6 + (W * 2 // 3) * i / 40, y0 + 40 * math.sin(i / 40 * 4 * math.pi)) for i in range(41)]
KEY_T, KEY_S, KEY_UP, KEY_END = 20, 31, 103, 107

start(); time.sleep(3)
gui("notebook"); time.sleep(3)
fullscreen(); time.sleep(4)
key(KEY_T); time.sleep(2)
tap(W // 5, H // 4); time.sleep(3)
keys("hello bro"); time.sleep(2)
key(ENTER); keys("typed on the Kobo"); time.sleep(2)
key(KEY_UP); key(KEY_END); keys("!"); time.sleep(2)
key(ESC); time.sleep(3)
key(25); time.sleep(1)                                      # P: the pen
stroke(wave(H // 2), pen=True); time.sleep(3)
key(KEY_S); time.sleep(2)
tap(W // 5 + 30, H // 4 + 10); time.sleep(3)              # the text, taken
key(ESC); time.sleep(2)
ex("text")
tap(W // 5 + 30, H // 4 + 10); time.sleep(2)
key(KEY_END); keys(" :)"); time.sleep(2)
key(ESC); time.sleep(4)
exit_app()
