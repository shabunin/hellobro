# The notebook's AI diary (notediary.go), against a stand-in model
# (ai.conf in APP_EXTRA, its base the stand-in's): turned on in the
# settings, its hand calligraphy; a line written, ending in a question
# mark: answered under it, by hand. A second line, unfinished: the model
# waits. Three taps of a finger: asked at once; the answer's long, and
# goes on on a page put in after. Undone (Ctrl+Z): gone from the talk
# too, so a line written then is asked with the first answer only. The
# settings again: its usage.
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
keys("jjjj"); time.sleep(1); key(28); time.sleep(3)  # its hand: calligraphy
key(1); time.sleep(3)                                # Esc: back to the page
line(W // 8, H // 8, words=3); time.sleep(16)        # asked after 3 s; answered, written
line(W // 8, H // 8 + 400, words=2, ask=False); time.sleep(8)   # it waits
for i in range(3):                                   # three quick taps: asked now
    tid[0] += 1
    frame(0, ID=tid[0], TOOL=0, **at(W * 3 // 4, H * 2 // 3), P=30); time.sleep(0.05)
    frame(0, ID=-1); time.sleep(0.2)
time.sleep(40)                                       # a long answer, onto a new page
key(44, ctrl=True); time.sleep(4)                    # Ctrl+Z: the answer undone
line(W // 8, H // 8 + 520, words=2); time.sleep(16)  # asked: the talk, the first answer only
keys(":settings\n"); time.sleep(4)                    # its usage
exit_app()
