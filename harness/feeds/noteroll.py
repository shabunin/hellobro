# A notebook as a roll (noteroll.go), with the AI diary on against a
# stand-in model (ai.conf in APP_EXTRA): "Pages" and "AI diary" set in
# its settings (which fit, landscape too); a question written near the
# screen's foot, answered under it; a finger swiped up scrolls on, the
# page grown for it; another question under the answer, and the long
# answer (formulas, a diagram, a plot, a mind map) grows the page as it
# goes, the roll following it. Swiped up and down it scrolls; on a
# blank screen at the foot, the next page; swiped across, pages turn.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

def word(x, y, n=4, h=26):                           # a scribbled word: loops along a line
    return [(x + 9 * i + 14 * math.cos(i / 3.0), y - h * abs(math.sin(i / 3.0))) for i in range(n * 9)]
def line(x, y, words=4):
    for k in range(words):
        stroke(word(x, y), pen=True, pressure=lambda t: 1800); time.sleep(0.2)
        x += 36 * 4 + 40
    stroke([(x + 12 * math.cos(a / 8.0 * math.pi), y - 60 + 14 * math.sin(a / 8.0 * math.pi)) for a in range(-8, 5)] + [(x, y - 40), (x, y - 20)], pen=True); time.sleep(0.2)
    stroke([(x, y - 4), (x + 1, y - 3)], pen=True)
def swipe(dx, dy):                                   # a finger across the page's middle
    x, y = W // 2 - dx // 2, H // 2 - dy // 2
    stroke([(x + dx * i / 10, y + dy * i / 10) for i in range(11)]); time.sleep(4)

start()
gui("notebook"); time.sleep(4)
fullscreen(); time.sleep(4)
keys(":settings\n"); time.sleep(3)
for i in range(5): keys("j"); time.sleep(0.3)
key(28); time.sleep(3)                               # Enter: Pages, a roll
keys("j"); time.sleep(0.5); key(28); time.sleep(3)   # the AI diary on
key(1); time.sleep(3)                                # Esc: back to the page
line(W // 8, H - 260, words=2); time.sleep(18)       # asked after 3 s; the answer below
swipe(0, -700)                                       # up: on down, the page grown for it
line(W // 8, H // 3, words=2); time.sleep(10)        # under the answer: the stand-in waits for more
for i in range(3):                                   # three quick taps: asked now, the long one, followed down
    tid[0] += 1
    frame(0, ID=tid[0], TOOL=0, **at(W * 3 // 4, H // 3), P=30); time.sleep(0.05)
    frame(0, ID=-1); time.sleep(0.2)
time.sleep(90)
swipe(0, 700)                                        # down: back up the roll
swipe(0, 700)
swipe(0, -700)                                       # up: on down
swipe(0, -700)
swipe(0, -700)
swipe(0, -700)                                       # a blank screen at the foot: the next page
swipe(-700, 0)                                       # across: the next page
swipe(700, 0)                                        # and back
keys(":settings\n"); time.sleep(4)                   # they fit
key(1); time.sleep(3)
exit_app()
