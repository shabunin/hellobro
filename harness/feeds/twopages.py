# Two pages side by side (bookwin.go, bookpdf.go): the first book, a PDF,
# fullscreen; s, its style panel: Pages (the fourth row) to "two side by
# side"; back to it: two pages; → the next two; a part of the right one
# cut with the pen. Esc, Esc: the shelf; the second book, an EPUB: two
# pages too (the setting's the readers'); → on; s: its panel, Pages to
# "two if wide"; back.
# HELLOBRO_BOOKS: a directory with a PDF first by name, then an EPUB.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
gui("books"); time.sleep(4)
key(ENTER); time.sleep(6)
fullscreen(); time.sleep(5)
key(31); time.sleep(3)                             # s: the style panel
for _ in range(3): key(108); time.sleep(1)         # down to Pages
key(ENTER); time.sleep(3)                          # two side by side
key(1); time.sleep(10)                             # Esc: the pages
key(RIGHT); time.sleep(8)                          # the next two
x = W * 3 // 4
stroke([(x - 150 + i * 15, H // 4 + i * 10) for i in range(21)], pen=True); time.sleep(5)  # a cut, on the right page
key(1); time.sleep(2); key(1); time.sleep(5)       # the cut away; the shelf
key(108); time.sleep(1); key(ENTER); time.sleep(8) # the EPUB
key(RIGHT); time.sleep(4)
key(31); time.sleep(3)                             # its panel
key(103); time.sleep(1); key(103); time.sleep(1)   # up, round to Pages
key(ENTER); time.sleep(4)                          # two if wide
key(1); time.sleep(5)
fullscreen(); time.sleep(3)
exit_app()
