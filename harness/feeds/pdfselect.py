# The PDF select tool (bookcut.go): the first book, a PDF, in fullscreen;
# the tool from the strip under the page, one corner tapped, then the
# other: a rectangle with a dot at each corner. The bottom-right dot
# tapped (it goes white), then tapped further down and right: bigger.
# Dragged: the top left dot up and left, the right side right, the
# inside down (the whole of it). Copied; a drag elsewhere: a new one.
# The tool put down again. HELLOBRO_BOOKS: a directory with a PDF first
# by name.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

def buttons(x1, y1):                       # where drawCut puts copy, save, × under a cut
    x0 = x1 - (150 + 150 + 74)
    return (x0 + 70, y1 + 42), (x0 + 220, y1 + 42), (x0 + 332, y1 + 42)

start()
gui("books"); time.sleep(4)
key(ENTER); time.sleep(6)
fullscreen(); time.sleep(5)
tool = (W // 2 + 150, H - 75)                        # the fourth of the strip's buttons, over the status bar
tap(*tool); time.sleep(3)
half = int(min(W, H) * 0.2)
x0, y0, x1, y1 = W // 2 - half, int(H * 0.30), W // 2 + half, int(H * 0.42)
tap(x0, y0); time.sleep(3)                           # one corner: a dot
tap(x1, y1); time.sleep(4)                           # the other: the rectangle
tap(x1, y1); time.sleep(3)                           # its dot, picked
x1, y1 = x1 + 60, int(H * 0.50)
tap(x1, y1); time.sleep(4)                           # moved there
def drag(x, y, dx, dy):
    stroke([(x + dx * i // 20, y + dy * i // 20) for i in range(21)]); time.sleep(4)
drag(x0, y0, -100, -80); x0, y0 = x0 - 100, y0 - 80   # the top left corner
drag(x1, (y0 + y1) // 2, 120, 0); x1 += 120          # the right side
drag((x0 + x1) // 2, (y0 + y1) // 2, 0, 200); y0, y1 = y0 + 200, y1 + 200  # the whole
copy, save, close = buttons(x1, y1)
tap(*copy); time.sleep(3)
drag(W // 2 - 250, int(H * 0.78), 500, int(H * 0.1))  # elsewhere: a new one
tap(*tool); time.sleep(4)                            # put down
exit_app()
