# The notebook's AI diary (notediary.go), against a stand-in model
# (ai.conf in APP_EXTRA, its base the stand-in's): turned on in the
# settings; a line written, ending in a question mark: answered under it,
# by hand. A second line, unfinished: the model waits. A third: the
# answer's long, and goes on on a page put in after.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

def word(x, y, n=4, h=26):                           # a scribbled word: loops along a line
    return [(x + 9 * i + 14 * math.cos(i / 3.0), y - h * abs(math.sin(i / 3.0))) for i in range(n * 9)]
def line(x, y, words=4, ask=True):
    for k in range(words):
        stroke(word(x, y), pen=True, pressure=lambda t: 1800); time.sleep(0.2)
        x += 36 * 4 + 40
    if ask:                                          # a question mark
        stroke([(x + 12 * math.cos(a / 8.0 * math.pi), y - 60 + 14 * math.sin(a / 8.0 * math.pi)) for a in range(-8, 5)] + [(x, y - 40), (x, y - 20)], pen=True); time.sleep(0.2)
        stroke([(x, y - 4), (x + 1, y - 3)], pen=True)

start()
gui("notebook"); time.sleep(4)
fullscreen(); time.sleep(4)
keys(":settings\n"); time.sleep(3)
for i in range(5): keys("j"); time.sleep(0.3)
key(28); time.sleep(3)                               # Enter: the diary on
key(1); time.sleep(3)                                # Esc: back to the page
line(W // 8, H // 8, words=3); time.sleep(14)        # asked after 3 s; answered, written
line(W // 8, H // 8 + 330, words=2, ask=False); time.sleep(8)   # it waits
line(W // 8, H // 8 + 430, words=3); time.sleep(40)  # a long answer, onto a new page
exit_app()
