# A book's notes. HELLOBRO_BOOKS has notes.epub alone: its first chapter
# starts "One", then "It began with a note¹ and a second one² in the
# first lines, then the next chapter by name." (the notes noterefs to a
# notes file in the spine, "the next chapter" a link to the second), in
# the default style. The book opened; a tap on note 1: its box; a tap
# elsewhere: gone; note 2, Enter: go there, the notes; ‹ back under the
# page: back; the chapter's link: there at once; ‹ back.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start(); time.sleep(3)
gui("books"); time.sleep(5)
key(ENTER); time.sleep(8)                              # the book
fullscreen(); time.sleep(5)
N1, N2, NEXT = ((441, 143), (731, 143), (1180, 150)) if land else ((408, 150), (712, 150), (1200, 160))
BACK = (150, H - 62)                                   # under the page, at the left
tap(*N1); time.sleep(4)                                # note 1: its box
tap(W // 2, H // 3); time.sleep(3)                     # elsewhere: gone
tap(*N2); time.sleep(4)                                # note 2
key(ENTER); time.sleep(5)                              # go there: the notes
tap(*BACK); time.sleep(5)                              # ‹ back
tap(*NEXT); time.sleep(5)                              # a chapter's link: there at once
tap(*BACK); time.sleep(5)
fullscreen(); time.sleep(3)
exit_app()
