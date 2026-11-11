# An EPUB full screen and its select tool (main.go's topSwipe,
# bookcut.go): full screen, its bar shown; a finger flicked down from
# the top hides it, again shows it. The select tool from the bar, a
# finger dragged across the text: the rectangle and its buttons (copy,
# save, ×: no zoom); copied (its text and picture), saved (Clips/).
# Two fingers swiped down from the top: out of full screen. A notebook
# beside it, Ctrl+V: the picture on its page. HELLOBRO_BOOKS: a
# directory with an EPUB first by name.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

def flick(x, y0, dy=110, n=6):                       # fast, as a finger flicks
    stroke([(x, y0 + dy * i // n) for i in range(n + 1)]); time.sleep(4)

start()
gui("books"); time.sleep(4)
key(ENTER); time.sleep(8)
fullscreen(); time.sleep(6)                          # an EPUB's bar stays
for _ in range(2): tap(W * 4 // 5, H // 2); time.sleep(5)  # past the cover, to text
flick(W // 2, 30); time.sleep(2)                     # hidden
flick(W // 2, 30); time.sleep(2)                     # shown
tap(*barbtn(3)); time.sleep(3)                       # the select tool
x0, y0, x1, y1 = 120, int(H * 0.25), W - 200, int(H * 0.40)
stroke([(x0 + (x1 - x0) * i // 20, y0 + (y1 - y0) * i // 20) for i in range(21)]); time.sleep(5)
bx = x1 - (150 + 150 + 74)                           # copy, save, × under it
tap(bx + 70, y1 + 42); time.sleep(4)                 # copy
tap(bx + 220, y1 + 42); time.sleep(4)                # save
tid[0] += 1; a = tid[0]                              # two fingers down from the top
frame(0, ID=a, TOOL=0, **at(W // 2 - 150, 40), P=30); time.sleep(0.02)
tid[0] += 1; b = tid[0]
frame(1, ID=b, TOOL=0, **at(W // 2 + 150, 40), P=30); time.sleep(0.02)
for i in range(1, 16):
    frame(0, **at(W // 2 - 150, 40 + 20 * i), P=30)
    frame(1, **at(W // 2 + 150, 40 + 20 * i), P=30)
    time.sleep(0.012)
frame(0, ID=-1); frame(1, ID=-1); time.sleep(5)      # out of full screen
gui("notebook"); time.sleep(5)                       # a notebook beside it
key(47, ctrl=True); time.sleep(5)                    # Ctrl+V
exit_app()
