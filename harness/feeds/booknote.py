# A book's notes. HELLOBRO_BOOKS has notes.epub alone: its first chapter
# starts "One", then "It began with a note¹ and a second one² in the
# first lines, then the next chapter by name." (the notes noterefs to a
# notes file in the spine, "the next chapter" a link to the second), in
# the default style. The book opened; a tap on note 1: its box; a tap
# elsewhere: gone; note 2, Enter: go there, the notes; ‹ back on the bar:
# back; the chapter's link: there at once; ‹ back.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start(); time.sleep(3)
gui("books"); time.sleep(5)
key(ENTER); time.sleep(8)                              # the book
fullscreen(); time.sleep(5)
N1, N2, NEXT = ((441, 207), (731, 207), (1180, 214)) if land else ((408, 214), (712, 214), (1200, 224))  # under the bar
BACK = (205, 36)                                       # on the bar, after ‹ All Books
tap(*N1); time.sleep(4)                                # note 1: its box
tap(W // 2, H // 3); time.sleep(3)                     # elsewhere: gone
tap(*N2); time.sleep(4)                                # note 2
key(ENTER); time.sleep(5)                              # go there: the notes
tap(*BACK); time.sleep(5)                              # ‹ back
tap(*NEXT); time.sleep(5)                              # a chapter's link: there at once
tap(*BACK); time.sleep(5)
fullscreen(); time.sleep(3)
exit_app()
