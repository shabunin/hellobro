# The PDF select tool and full screen (bookcut.go, bookwin.go's place):
# the tool taken, then full screen without the bar: it's put down, so
# taps turn the page on five times and back (5 / 88 on the bar after), and a quick flick down from the
# top (about 110 px, as on the Kobo) leaves full screen. Then with the
# bar shown full screen (the style panel's row), the tool in hand: the
# flick still leaves it; full screen again, two fingers down from the
# top, the first already moving (a cut begun) before the second comes
# down: the workspaces' tiles, Esc. HELLOBRO_BOOKS: a directory with a
# PDF of a few pages first by name.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

def flick(x, y0, dy=110, n=6):                       # fast, as a finger flicks
    stroke([(x, y0 + dy * i // n) for i in range(n + 1)]); time.sleep(4)

start()
gui("books"); time.sleep(4)
key(ENTER); time.sleep(6)
tap(416, 1052); time.sleep(3)                       # the select tool: in hand (the book's under the launcher)
fullscreen(); time.sleep(5)                          # no bar: put down
for _ in range(5): tap(W * 4 // 5, H // 2); time.sleep(4)  # on, five pages
tap(W // 6, H // 2); time.sleep(4)                   # back: 5 / 88, each turn a page
flick(W // 2, 30); time.sleep(2)                     # out of full screen

key(31); time.sleep(3)                               # s: the style panel
for _ in range(4): key(DOWN); time.sleep(0.5)
key(ENTER); time.sleep(3); key(ESC); time.sleep(4)   # Bar, fullscreen: shown
fullscreen(); time.sleep(5)
tap(*barbtn(3)); time.sleep(3)                      # the tool, from the bar full screen
flick(W // 2, 100); time.sleep(2)                    # over the page: out all the same
fullscreen(); time.sleep(5)
# two fingers, the first moving a little before the second is down
tid[0] += 1; a = tid[0]
frame(0, ID=a, TOOL=0, **at(W // 2 - 150, 90), P=30); time.sleep(0.02)
for i in range(1, 4):
    frame(0, **at(W // 2 - 150, 90 + 10 * i), P=30); time.sleep(0.03)
tid[0] += 1; b = tid[0]
frame(1, ID=b, TOOL=0, **at(W // 2 + 150, 90), P=30); time.sleep(0.02)
for i in range(1, 16):
    frame(0, **at(W // 2 - 150, 120 + 20 * i), P=30)
    frame(1, **at(W // 2 + 150, 90 + 20 * i), P=30)
    time.sleep(0.012)
frame(0, ID=-1); frame(1, ID=-1); time.sleep(5)      # the tiles
key(ESC); time.sleep(4)
exit_app()
