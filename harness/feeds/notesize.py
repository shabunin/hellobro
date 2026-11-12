# A notebook's page size (notesize.go): :pagesize landscape makes the
# blank page a landscape one (filling a landscape screen), a line
# written on it; a new page after it is so too; :pagesize 120x80 a
# small custom one; the settings say the size, and step it to A4.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

def word(x, y, n=4, h=26):                           # a scribbled word: loops along a line
    return [(x + 9 * i + 14 * math.cos(i / 3.0), y - h * abs(math.sin(i / 3.0))) for i in range(n * 9)]

start()
gui("notebook"); time.sleep(4)
fullscreen(); time.sleep(4)
keys(":pagesize landscape\n"); time.sleep(4)         # the blank page: landscape
stroke(word(W // 4, H // 2), pen=True, pressure=lambda t: 1800); time.sleep(3)
keys(":new\n"); time.sleep(4)                         # a new page: landscape too
keys(":pagesize 120x80\n"); time.sleep(4)            # a custom one, small
stroke(word(W // 3, H // 2), pen=True, pressure=lambda t: 1800); time.sleep(3)
keys(":settings\n"); time.sleep(3)
for i in range(7): key(108); time.sleep(0.3)
key(28); time.sleep(3)                               # Page size: its dropdown, the custom one last
for i in range(4): key(103); time.sleep(0.3)
key(28); time.sleep(3)                               # A4
key(1); time.sleep(3)
keys(":p 1\n"); time.sleep(4)                         # back to the first: landscape
exit_app()
