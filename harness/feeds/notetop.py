# A notebook full screen (main.go's topSwipe, notebook.go's noteMove): a
# pen's dot first, so fingers turn pages, not write. A finger flicked
# down from the top, on the bar: the bar hidden; again, on the page now:
# shown; again: hidden. Two fingers down from the top: out of full
# screen, the bar back.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

def flick(x, y0, dy=110, n=6):                       # fast, as a finger flicks
    stroke([(x, y0 + dy * i // n) for i in range(n + 1)]); time.sleep(4)

start()
gui("notebook"); time.sleep(5)
fullscreen(); time.sleep(5)
stroke([(W // 2, H // 2), (W // 2 + 2, H // 2 + 2)], pen=True); time.sleep(3)  # the pen: fingers turn now
flick(W // 2, 30); time.sleep(2)                     # on the bar: hidden
flick(W // 2, 30); time.sleep(2)                     # on the page: shown
flick(W // 2, 30); time.sleep(2)                     # hidden
tid[0] += 1; a = tid[0]                              # two fingers down from the top
frame(0, ID=a, TOOL=0, **at(W // 2 - 150, 40), P=30); time.sleep(0.02)
tid[0] += 1; b = tid[0]
frame(1, ID=b, TOOL=0, **at(W // 2 + 150, 40), P=30); time.sleep(0.02)
for i in range(1, 16):
    frame(0, **at(W // 2 - 150, 40 + 20 * i), P=30)
    frame(1, **at(W // 2 + 150, 40 + 20 * i), P=30)
    time.sleep(0.012)
frame(0, ID=-1); frame(1, ID=-1); time.sleep(5)      # out of full screen
exit_app()
