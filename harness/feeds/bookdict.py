# A word held on a book's page (bookdict.go). HELLOBRO_BOOKS has one
# EPUB alone, in the default style, its second line ending "καλὸς λόγος",
# its fourth starting "L’homme", and full of text to its foot;
# HELLOBRO_DICTS dictionaries with those (cmd/dictconv). L’homme: homme's
# box; καλὸς: over two pages, Enter to the next and round; a word at the
# foot: its box on top; a tap elsewhere closes it.
# app: hellobro
import os, sys
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from feedlib import *

start()
gui("books"); time.sleep(3)
key(ENTER); time.sleep(5)
fullscreen(); time.sleep(4)
hold(150, 310); time.sleep(4)             # L’homme
key(ESC); time.sleep(3)
hold(900, 191); time.sleep(4)             # καλὸς
key(ENTER); time.sleep(3)                 # next
key(ENTER); time.sleep(3)                 # round to the first
key(ESC); time.sleep(3)
hold(300, H - 120); time.sleep(4)         # at the foot: the box on top
tap(W // 2, H - 300); time.sleep(3)       # elsewhere: closed
exit_app()
