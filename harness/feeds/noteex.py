# A notebook's ":" commands: written in; :new a page after it, written
# in; :paper lines; :1 back to the first; :help, "paper" Enter, plain;
# :rename plan; :mv Work; :ls the list (Work: plan); back, :q closes it.
# app: hellobro
import os, sys, math
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

def ex(s, wait=4):
    keys(":"); time.sleep(1.5); keys(s); time.sleep(2); key(ENTER); time.sleep(wait)
def wave(y0):
    return [(W // 6 + (W * 2 // 3) * i / 40, y0 + 50 * math.sin(i / 40 * 4 * math.pi)) for i in range(41)]

start(); time.sleep(3)
gui("notebook"); time.sleep(3)
fullscreen(); time.sleep(4)
stroke(wave(H // 3), pen=True); time.sleep(3)
ex("new")
stroke(wave(H // 2), pen=True); time.sleep(3)
ex("paper lines")
ex("1")
ex("help", 4)                                          # the commands, in a menu
keys("paper"); time.sleep(2); key(ENTER); time.sleep(3)  # :paper on the line
keys("plain"); time.sleep(1); key(ENTER); time.sleep(4)
ex("rename plan")
ex("mv Work")
ex("ls", 5)
key(ESC); time.sleep(4)                                # back to the notebook
ex("q")
exit_app()
