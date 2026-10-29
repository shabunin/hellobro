# The launcher as tiles: the arrows light one (Alpine first), Enter
# opens it; a second launcher beside the first is narrower, a swipe up
# scrolls it; a tap on Books opens the shelf in its place.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start(); time.sleep(3)                                # the launcher
key(RIGHT); time.sleep(2)                             # the Kobo shell lit
key(DOWN); time.sleep(2); key(DOWN); time.sleep(2)    # down to Read and write, then Settings
key(UP); time.sleep(2)
mod(ENTER); time.sleep(4)                             # another, beside it: narrower
stroke([(W * 3 // 4, H * 2 // 3 - i * 40) for i in range(12)]); time.sleep(4)   # swipe up: on
stroke([(W * 3 // 4, H // 3 + i * 40) for i in range(12)]); time.sleep(4)       # and back
key(16, shift=True, meta=True); time.sleep(3)         # closed: the first again
keys("bro@"); time.sleep(2)                           # typed into the ssh> line
for _ in range(4):
    key(14); time.sleep(0.5)                          # and taken back
time.sleep(2)
if not land:
    tap(194, 466); time.sleep(5)                      # Books: the shelf in its place
exit_app()
