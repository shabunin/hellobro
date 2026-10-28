# The reader (bookwin.go): the shelf, a book's pages turned by keys and
# taps, bigger text, another font, the contents, back to the shelf.
# HELLOBRO_BOOKS: a directory of EPUBs (Alice first by name, with
# pictures); HELLOBRO_FONTS: one of fonts in four styles, as the Kobo's
# (Name-Regular, -Bold, -Italic, -BoldItalic).
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
gui("books"); time.sleep(3)                          # the shelf, through the switcher
key(ENTER); time.sleep(4)                            # the first book, where it starts
for _ in range(3): key(SPACE); time.sleep(2.5)       # on
tap(W * 3 // 4, H // 2); time.sleep(2.5)             # a tap on the right: on
tap(W // 6, H // 2); time.sleep(2.5)                 # on the left: back
keys("t"); time.sleep(2.5)                           # the contents
for _ in range(4): key(DOWN); time.sleep(0.6)
key(ENTER); time.sleep(4)                            # a chapter
keys("++"); time.sleep(5)                            # bigger
keys("f"); time.sleep(5)                             # the next font
keys("--"); time.sleep(4)
key(ESC); time.sleep(3)                              # the shelf, the book read first
exit_app()
