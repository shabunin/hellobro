# A PDF's cut, by the pen, and its buttons: copy held (dark while the
# finger's on it, then done), save slid off (dark, then light again,
# not done), × tapped. HELLOBRO_BOOKS: a directory with a PDF first.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

def slide(a, b, secs=1.0, n=10):
    tid[0] += 1
    frame(0, ID=tid[0], TOOL=0, **at(*a), P=30); time.sleep(secs)
    for i in range(1, n + 1):
        frame(0, **at(a[0] + (b[0] - a[0]) * i // n, a[1] + (b[1] - a[1]) * i // n), P=30); time.sleep(0.03)
    frame(0, ID=-1); time.sleep(1)

start()
gui("books"); time.sleep(4)
key(ENTER); time.sleep(6)
fullscreen(); time.sleep(5)
x0, y0, x1, y1 = 400, 500, 1000, 800
stroke([(x0 + (x1 - x0) * i / 20, y0 + (y1 - y0) * i / 20) for i in range(21)], pen=True); time.sleep(4)
bx = x1 - (168 + 168 + 168 + 88)                     # compact: 156 wide (× 76), 76 tall, 12 apart
hold(bx + 78, y1 + 50, secs=0.4); time.sleep(3)      # copy
slide((bx + 246, y1 + 50), (bx + 246, y1 + 300)); time.sleep(3)  # save, slid off
tap(bx + 542, y1 + 50); time.sleep(3)                # ×
exit_app()
