# The new notebook's wizard (notewizard.go), full screen: a (an AI
# notebook): the prompts in full; the list swiped up to its end, "+ your
# own prompt"; "by hand": a word written on the sheet, "read it to
# text" (the stand-in model's answer comes in as the text, typed);
# "make it": the notebook, its diary on, the prompt its own and saved.
# Wants the stand-in model (ai.conf in APP_EXTRA).
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start(); time.sleep(3)
mod(32); time.sleep(1.2); keys("notebook"); time.sleep(0.6); key(ENTER); time.sleep(3)
fullscreen(); time.sleep(3)
key(30); time.sleep(3)                                               # a: an AI notebook
stroke([(W // 2, H - 300 - i * 40) for i in range(30)]); time.sleep(3)  # the list, to its end
OWN = (W // 2, int(os.environ.get('OWN_Y', H - 200)))
tap(*OWN); time.sleep(3)                                             # + your own prompt
tap(W * 3 // 4, int(os.environ.get('TABS_Y', 200))); time.sleep(3)   # by hand
for k in range(4):                                                   # a word, written
    x = 200 + k * 220
    stroke([(x + 9 * i + 14 * math.cos(i / 3.0), 600 - 40 * abs(math.sin(i / 3.0))) for i in range(36)], pen=True, pressure=lambda t: 2000)
time.sleep(2)
tap(int(os.environ.get('READ_X', 400)), int(os.environ.get('READ_Y', 1300))); time.sleep(8)  # read it to text
tap(W // 2, H - 60); time.sleep(5)                                   # make it
exit_app()
